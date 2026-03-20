package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnit_Path_Resolution_Complex(t *testing.T) {
	t.Parallel()
	home := "/home/user"
	baseDir := "/abs/path"
	mfs := &MockFileSystem{
		WD:      baseDir,
		HomeDir: home,
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("ResolvePath", func(t *testing.T) {
		t.Parallel()
		val, err := ResolvePath(mfs, "./file", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/abs/path/file", val)

		val, err = ResolvePath(mfs, "../file", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/abs/file", val)

		val, err = ResolvePath(mfs, "~/.ssh", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/.ssh", val)

		val, err = ResolvePath(mfs, "~", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/home/user", val)

		val, err = ResolvePath(mfs, "/other/abs/path", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/other/abs/path", val)

		val, err = ResolvePath(mfs, "just-name", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "just-name", val)
	})

	t.Run("Scheme handling", func(t *testing.T) {
		t.Parallel()
		val, err := ResolvePath(mfs, "unix:///var/run/docker.sock", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "unix:///var/run/docker.sock", val)

		val, err = ResolvePath(mfs, "unix:///var/run/docker.sock", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "unix:///var/run/docker.sock", val)

		val, err = ResolvePath(mfs, "http://example.com/path", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "http://example.com/path", val)
	})

	t.Run("Nested execution reverse resolution", func(t *testing.T) {
		t.Parallel()
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/host/app", Target: "/app", Level: 1},
				{Source: "/host/tmp", Target: "/tmp", Level: 1},
			},
		}
		rn, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		val, err := ResolvePath(mfs, "/app/src", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/app/src", val)

		val, err = ResolvePath(mfs, "/tmp/other", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/tmp/other", val)
	})

	t.Run("Nested execution longest prefix win", func(t *testing.T) {
		t.Parallel()
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/host/src", Target: "/src", Level: 1},
				{Source: "/host/app", Target: "/app", Level: 1},
			},
		}
		rn, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		val, err := ResolvePath(mfs, "/src/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/src/file", val)

		val, err = ResolvePath(mfs, "/app/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/app/file", val)
	})

	t.Run("Nested execution exact match", func(t *testing.T) {
		t.Parallel()
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/host/tmp", Target: "/tmp", Level: 1},
				{Source: "/host/etc", Target: "/etc", Level: 1},
			},
		}
		rn, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		val, err := ResolvePath(mfs, "/tmp/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/tmp/file", val)

		val, err = ResolvePath(mfs, "/etc/hosts", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/etc/hosts", val)
	})

	t.Run("Nested execution no match", func(t *testing.T) {
		t.Parallel()
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/host/app", Target: "/app", Level: 1},
			},
		}
		rn, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		val, err := ResolvePath(mfs, "/other/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/other/file", val)
	})

	t.Run("Nested execution relative path resolution", func(t *testing.T) {
		t.Parallel()
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/host/app", Target: "/app", Level: 1},
			},
		}
		rn, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		val, err := ResolvePath(mfs, "/app/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/app/file", val)
	})

	t.Run("Nested execution prefix ambiguity", func(t *testing.T) {
		t.Parallel()
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/host/apple", Target: "/apple", Level: 1},
				{Source: "/host/app", Target: "/app", Level: 1},
			},
		}
		rn, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		val, err := ResolvePath(mfs, "/apple", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/apple", val)

		val, err = ResolvePath(mfs, "/app/le", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/app/le", val)
	})
}

func TestUnit_Path_MarshalYAML(t *testing.T) {
	t.Parallel()
	t.Run("ConfigPath", func(t *testing.T) {
		t.Parallel()
		cp := ConfigPath{Raw: "/path"}
		data, err := yaml.Marshal(cp)
		require.NoError(t, err)
		assert.Equal(t, "/path\n", string(data))

		cp = ConfigPath{}
		data, err = yaml.Marshal(cp)
		require.NoError(t, err)
		assert.Equal(t, "null\n", string(data))
	})

	t.Run("MountConfig", func(t *testing.T) {
		t.Parallel()
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/host"},
			Target: ConfigPath{Raw: "/container"},
		}
		data, err := yaml.Marshal(mc)
		require.NoError(t, err)
		assert.Contains(t, string(data), "type: bind")
		assert.Contains(t, string(data), "source: /host")
		assert.Contains(t, string(data), "target: /container")

		mc = MountConfig{}
		data, err = yaml.Marshal(mc)
		require.NoError(t, err)
		assert.Equal(t, "null\n", string(data))
	})

	t.Run("DeviceConfig", func(t *testing.T) {
		t.Parallel()
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/video0"},
			Destination: ConfigPath{Raw: "/dev/video0"},
			Permissions: "rwm",
		}
		data, err := yaml.Marshal(dc)
		require.NoError(t, err)
		assert.Equal(t, "/dev/video0:/dev/video0\n", string(data))

		dc.Permissions = "rw"
		data, err = yaml.Marshal(dc)
		require.NoError(t, err)
		assert.Equal(t, "/dev/video0:/dev/video0:rw\n", string(data))
	})
}

func TestUnit_Path_Parsing_Helpers(t *testing.T) {
	t.Parallel()
	baseDir := "/base"
	mfs := &MockFileSystem{}
	r, _ := NewExpressionResolverWithFS(nil, mfs)

	t.Run("resolveVolumePath", func(t *testing.T) {
		t.Parallel()
		val, err := resolveVolumePath(mfs, "./host:/container", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/base/host:/container", val)

		val, err = resolveVolumePath(mfs, "named-volume:/container", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "named-volume:/container", val)
	})

	t.Run("resolveDevicePath", func(t *testing.T) {
		t.Parallel()
		val, err := resolveDevicePath(mfs, "./dev:/dev:rw", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/base/dev:/dev:rw", val)
	})

	t.Run("SplitHostRemainder", func(t *testing.T) {
		t.Parallel()
		host, rem, ok := SplitHostRemainder("/host:/container")
		assert.True(t, ok)
		assert.Equal(t, "/host", host)
		assert.Equal(t, "/container", rem)

		host, rem, ok = SplitHostRemainder("C:\\host:/container")
		assert.True(t, ok)
		assert.Equal(t, "C:\\host", host)
		assert.Equal(t, "/container", rem)

		_, _, ok = SplitHostRemainder("/no-sep")
		assert.False(t, ok)
	})
}

func TestUnit_Path_UnmarshalYAMLErrors(t *testing.T) {
	t.Parallel()
	t.Run("MountConfig", func(t *testing.T) {
		t.Parallel()
		var mc MountConfig
		err := yaml.Unmarshal([]byte("target: ''"), &mc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target is required")
	})

	t.Run("DeviceConfig", func(t *testing.T) {
		t.Parallel()
		var dc DeviceConfig
		err := yaml.Unmarshal([]byte("["), &dc)
		require.Error(t, err)
	})
}

func TestUnit_Path_ResolveVolume_Device(t *testing.T) {
	t.Parallel()
	baseDir := "/base"
	mfs := &MockFileSystem{}
	r, _ := NewExpressionResolverWithFS(nil, mfs)

	t.Run("ResolveVolume", func(t *testing.T) {
		t.Parallel()
		cp := ConfigPath{Raw: "./host:/container", BaseDir: baseDir}
		val, err := cp.ResolveVolume(mfs, r)
		require.NoError(t, err)
		assert.Equal(t, "/base/host:/container", val)

		cp = ConfigPath{Raw: ""}
		val, err = cp.ResolveVolume(mfs, r)
		require.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("ResolveDevice", func(t *testing.T) {
		t.Parallel()
		cp := ConfigPath{Raw: "./dev:/dev:rw", BaseDir: baseDir}
		val, err := cp.ResolveDevice(mfs, r)
		require.NoError(t, err)
		assert.Equal(t, "/base/dev:/dev:rw", val)

		cp = ConfigPath{Raw: ""}
		val, err = cp.ResolveDevice(mfs, r)
		require.NoError(t, err)
		assert.Empty(t, val)
	})
}

func TestUnit_Path_SplitHostRemainder_Windows_Invalid(t *testing.T) {
	t.Parallel()
	t.Run("Windows path without separator", func(t *testing.T) {
		t.Parallel()
		_, _, ok := SplitHostRemainder("C:\\no-sep")
		assert.False(t, ok)
	})
}
