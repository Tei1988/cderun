package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type pathMockFS struct {
	MockFileSystem
	absErr error
}

func (m *pathMockFS) Abs(path string) (string, error) {
	if m.absErr != nil {
		return "", m.absErr
	}
	return m.MockFileSystem.Abs(path)
}

func TestUnit_Path_Resolution(t *testing.T) {
	home := "/home/user"
	baseDir := "/abs/path"
	mfs := &MockFileSystem{
		WD:      baseDir,
		HomeDir: home,
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("ResolvePath", func(t *testing.T) {
		val, err := ResolvePath("./file", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/abs/path/file", val)

		val, err = ResolvePath("../file", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/abs/file", val)

		val, err = ResolvePath("~/.ssh", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".ssh"), val)

		val, err = ResolvePath("~", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, home, val)

		val, err = ResolvePath("/other/abs/path", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/other/abs/path", val)

		val, err = ResolvePath("just-name", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(baseDir, "just-name"), val) // resolved absolute/joined path, no ./ prefix

		// fs.Abs failure case
		mfsErr := &pathMockFS{
			MockFileSystem: MockFileSystem{
				WD: "/wd",
			},
			absErr: assert.AnError,
		}
		hostCtx := &HostContext{Level: 1} // Trigger nested check
		rErr, err := NewExpressionResolverWithFS(hostCtx, mfsErr)
		require.NoError(t, err)
		// ResolvePath calls fs.Abs when r.HostContext.Level > 0 and path is NOT absolute.
		_, err = ResolvePath("some/path", "/wd", rErr)
		require.Error(t, err)
	})

	t.Run("ConfigPath.Resolve", func(t *testing.T) {
		cp := ConfigPath{Raw: "./data", BaseDir: baseDir}
		val, err := cp.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "/abs/path/data", val)

		cp = ConfigPath{Raw: "{{HOME}}/config", BaseDir: baseDir}
		val, err = cp.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, "config"), val)
	})

	t.Run("MountConfig.Resolve", func(t *testing.T) {
		mc := MountConfig{
			Type:     "bind",
			Source:   ConfigPath{Raw: "./data", BaseDir: baseDir},
			Target:   ConfigPath{Raw: "/app/data", BaseDir: baseDir},
			ReadOnly: false,
		}
		mount, err := mc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "bind", mount.Type)
		assert.Equal(t, "/abs/path/data", mount.Source)
		assert.Equal(t, "/app/data", mount.Target)
		assert.False(t, mount.ReadOnly)

		mc = MountConfig{
			Type:     "bind",
			Source:   ConfigPath{Raw: "~/config", BaseDir: baseDir},
			Target:   ConfigPath{Raw: "/root/config", BaseDir: baseDir},
			ReadOnly: true,
		}
		mount, err = mc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, "config"), mount.Source)
		assert.Equal(t, "/root/config", mount.Target)
		assert.True(t, mount.ReadOnly)

		mc = MountConfig{
			Type:   "volume",
			Source: ConfigPath{Raw: "myvol"},
			Target: ConfigPath{Raw: "/data"},
		}
		mount, err = mc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "volume", mount.Type)
		assert.Equal(t, filepath.Join(baseDir, "myvol"), mount.Source)
		assert.Equal(t, "/data", mount.Target)
	})

	t.Run("DeviceConfig.Resolve", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/video0", BaseDir: baseDir},
			Destination: ConfigPath{Raw: "/dev/video0", BaseDir: baseDir},
			Permissions: "rwm",
		}
		mapping, err := dc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "/dev/video0", mapping.PathOnHost)
		assert.Equal(t, "/dev/video0", mapping.PathInContainer)
		assert.Equal(t, "rwm", mapping.CgroupPermissions)
	})

	t.Run("ParseMountFlag", func(t *testing.T) {
		mc, err := ParseMountFlag("type=bind,source=./data,target=/app/data,readonly")
		require.NoError(t, err)
		assert.Equal(t, "bind", mc.Type)
		assert.Equal(t, "./data", mc.Source.Raw)
		assert.Equal(t, "/app/data", mc.Target.Raw)
		assert.True(t, mc.ReadOnly)

		mc, err = ParseMountFlag("source=/host/path,target=/container/path,readonly=false")
		require.NoError(t, err)
		assert.Equal(t, "bind", mc.Type)
		assert.Equal(t, "/host/path", mc.Source.Raw)
		assert.Equal(t, "/container/path", mc.Target.Raw)
		assert.False(t, mc.ReadOnly)

		mc, err = ParseMountFlag("src=/s,dst=/d,readonly=true")
		require.NoError(t, err)
		assert.Equal(t, "/s", mc.Source.Raw)
		assert.Equal(t, "/d", mc.Target.Raw)
		assert.True(t, mc.ReadOnly)

		mc, err = ParseMountFlag("source=./data,target=/app/data,optional")
		require.NoError(t, err)
		assert.True(t, mc.Optional)

		mc, err = ParseMountFlag("source=./data,target=/app/data,optional=true")
		require.NoError(t, err)
		assert.True(t, mc.Optional)

		mc, err = ParseMountFlag("source=./data,target=/app/data,optional=false")
		require.NoError(t, err)
		assert.False(t, mc.Optional)

		_, err = ParseMountFlag("invalid-format")
		require.Error(t, err)

		_, err = ParseMountFlag("type=bind,source=./src")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target is required")

		_, err = ParseMountFlag("type=bind,source=./src,target=/dst,readonly=invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid readonly value")

		_, err = ParseMountFlag("type=bind,src=/s,target=/t,unknown=val")
		require.NoError(t, err) // unknown keys are ignored
	})

	t.Run("Windows Paths", func(t *testing.T) {
		mc, err := ParseMountFlag(`type=bind,source=C:\host\path,target=/container`)
		require.NoError(t, err)
		assert.Equal(t, `C:\host\path`, mc.Source.Raw)
		assert.Equal(t, `/container`, mc.Target.Raw)

		dc, ok := ParseDeviceConfig(`E:\dev\path:/dev/path:rwm`)
		assert.True(t, ok)
		assert.Equal(t, `E:\dev\path`, dc.Source.Raw)
		assert.Equal(t, `/dev/path`, dc.Destination.Raw)
		assert.Equal(t, "rwm", dc.Permissions)
	})

	t.Run("Scheme Preservation", func(t *testing.T) {
		val, err := ResolvePath("unix:///var/run/docker.sock", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "unix:///var/run/docker.sock", val)

		val, err = ResolvePath("unix:////var/run/docker.sock", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "unix:////var/run/docker.sock", val)

		val, err = ResolvePath("http://example.com/path", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "http://example.com/path", val)
	})

	t.Run("Reverse Path Resolution (Nested)", func(t *testing.T) {
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/home/user/project", Target: "/app", Level: 1},
			},
		}
		rn, err := NewExpressionResolver(hostCtx)
		require.NoError(t, err)

		// Inside container /app/src should resolve to host /home/user/project/src
		val, err := ResolvePath("/app/src", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/project/src", val)

		// Path outside mapping should stay same
		val, err = ResolvePath("/tmp/other", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/tmp/other", val)
	})

	t.Run("Reverse Path Resolution (Nested Priority)", func(t *testing.T) {
		hostCtx := &HostContext{
			Level: 2,
			Mounts: []MountMapping{
				{Source: "/home/user/project", Target: "/app", Level: 1},
				{Source: "/home/user/project/src", Target: "/src", Level: 2},
			},
		}
		rn, err := NewExpressionResolver(hostCtx)
		require.NoError(t, err)

		// /src should match level 2 mapping
		val, err := ResolvePath("/src/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/project/src/file", val)

		// /app should match level 1 mapping
		val, err = ResolvePath("/app/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/project/file", val)
	})

	t.Run("Reverse Path Resolution (Specificity vs Level)", func(t *testing.T) {
		hostCtx := &HostContext{
			Level: 2,
			Mounts: []MountMapping{
				{Source: "/tmp", Target: "/tmp", Level: 1},                      // Broad but specific
				{Source: "/var/lib/docker/overlay/diff", Target: "/", Level: 2}, // Higher level but root
			},
		}
		rn, err := NewExpressionResolver(hostCtx)
		require.NoError(t, err)

		// /tmp/file should match /tmp (Level 1) instead of / (Level 2)
		val, err := ResolvePath("/tmp/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/tmp/file", val)

		// /etc/hosts should match / (Level 2)
		val, err = ResolvePath("/etc/hosts", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/var/lib/docker/overlay/diff/etc/hosts", val)
	})

	t.Run("Reverse Path Resolution (Same Specificity)", func(t *testing.T) {
		hostCtx := &HostContext{
			Level: 2,
			Mounts: []MountMapping{
				{Source: "/host/v1", Target: "/app", Level: 1},
				{Source: "/host/v2", Target: "/app", Level: 2},
			},
		}
		rn, err := NewExpressionResolver(hostCtx)
		require.NoError(t, err)

		// Same specificity, higher level wins
		val, err := ResolvePath("/app/file", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/v2/file", val)
	})

	t.Run("Reverse Path Resolution (Partial Segment Match)", func(t *testing.T) {
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/host/app", Target: "/app", Level: 1},
			},
		}
		rn, err := NewExpressionResolver(hostCtx)
		require.NoError(t, err)

		// /apple should NOT match /app
		val, err := ResolvePath("/apple", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/apple", val)

		// /app/le should match /app
		val, err = ResolvePath("/app/le", baseDir, rn)
		require.NoError(t, err)
		assert.Equal(t, "/host/app/le", val)
	})

	t.Run("MountConfig.Resolve target not reverse-resolved in nested", func(t *testing.T) {
		hostCtx := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/Users/user/.config/gcloud", Target: "/root/.config/gcloud", Level: 1},
			},
		}
		rn, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/root/.config/gcloud"},
			Target: ConfigPath{Raw: "/.config/gcloud"},
		}
		mount, err := mc.Resolve(rn)
		require.NoError(t, err)
		// source should be reverse-resolved to host path
		assert.Equal(t, "/Users/user/.config/gcloud", mount.Source)
		// target must NOT be reverse-resolved; it stays as the container-side path
		assert.Equal(t, "/.config/gcloud", mount.Target)
	})
}

func TestUnit_Path_MarshalYAML(t *testing.T) {
	t.Run("ConfigPath", func(t *testing.T) {
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
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/video0"},
			Destination: ConfigPath{Raw: "/dev/video1"},
			Permissions: "rw",
		}
		data, err := yaml.Marshal(dc)
		require.NoError(t, err)
		assert.Equal(t, "/dev/video0:/dev/video1:rw\n", string(data))

		dc = DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/fuse"},
			Destination: ConfigPath{Raw: "/dev/fuse"},
			Permissions: "rwm",
		}
		data, err = yaml.Marshal(dc)
		require.NoError(t, err)
		assert.Equal(t, "/dev/fuse:/dev/fuse\n", string(data))

		dc = DeviceConfig{}
		data, err = yaml.Marshal(dc)
		require.NoError(t, err)
		assert.Equal(t, "null\n", string(data))
	})

	t.Run("omitempty behavior", func(t *testing.T) {
		type TestConfig struct {
			Path    ConfigPath     `yaml:"path,omitempty"`
			Mounts  []MountConfig  `yaml:"mounts,omitempty"`
			Devices []DeviceConfig `yaml:"devices,omitempty"`
		}

		cfg := TestConfig{}
		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		assert.Equal(t, "{}\n", string(data))

		cfg.Path = ConfigPath{Raw: "/foo"}
		data, err = yaml.Marshal(cfg)
		require.NoError(t, err)
		assert.Contains(t, string(data), "path: /foo")
	})
}

func TestUnit_Path_Helpers(t *testing.T) {
	baseDir := "/base"
	r, err := NewExpressionResolver(nil)
	require.NoError(t, err)

	t.Run("resolveVolumePath", func(t *testing.T) {
		val, err := resolveVolumePath("./host:/container", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/base/host:/container", val)

		val, err = resolveVolumePath("named-volume:/container", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "named-volume:/container", val)
	})

	t.Run("resolveDevicePath", func(t *testing.T) {
		val, err := resolveDevicePath("./dev:/dev:rw", baseDir, r)
		require.NoError(t, err)
		assert.Equal(t, "/base/dev:/dev:rw", val)
	})

	t.Run("SplitHostRemainder", func(t *testing.T) {
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

func TestUnit_Path_ParseDeviceConfig_Errors(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		_, ok := ParseDeviceConfig("")
		assert.False(t, ok)
	})

	t.Run("missing components", func(t *testing.T) {
		_, ok := ParseDeviceConfig(":")
		assert.False(t, ok)
	})
}

func TestUnit_Path_Resolve_Errors(t *testing.T) {
	t.Run("ConfigPath.Resolve empty", func(t *testing.T) {
		cp := ConfigPath{}
		val, err := cp.Resolve(nil)
		require.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("MountConfig.Resolve - Source error", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "{{file:missing}}"},
			Target: ConfigPath{Raw: "/target"},
		}
		r, err := NewExpressionResolverWithFS(nil, &MockFileSystem{})
		require.NoError(t, err)
		_, err = mc.Resolve(r)
		require.Error(t, err)
	})

	t.Run("MountConfig.Resolve - Target error", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/source"},
			Target: ConfigPath{Raw: "{{file:missing}}"},
		}
		r, err := NewExpressionResolverWithFS(nil, &MockFileSystem{})
		require.NoError(t, err)
		_, err = mc.Resolve(r)
		require.Error(t, err)
	})

	t.Run("DeviceConfig.Resolve - Source error", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "{{file:missing}}"},
			Destination: ConfigPath{Raw: "/dev/v"},
		}
		r, err := NewExpressionResolverWithFS(nil, &MockFileSystem{})
		require.NoError(t, err)
		_, err = dc.Resolve(r)
		require.Error(t, err)
	})

	t.Run("DeviceConfig.Resolve - Destination error", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/h"},
			Destination: ConfigPath{Raw: "{{file:missing}}"},
		}
		r, err := NewExpressionResolverWithFS(nil, &MockFileSystem{})
		require.NoError(t, err)
		_, err = dc.Resolve(r)
		require.Error(t, err)
	})

	t.Run("ResolvePath empty", func(t *testing.T) {
		val, err := ResolvePath("", "/base", nil)
		require.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("expandHome error", func(t *testing.T) {
		mfs := &customMockFS{
			homeDirErr: assert.AnError,
		}
		_, err := NewExpressionResolverWithFS(nil, mfs)
		require.Error(t, err)
	})

	t.Run("Expression error in ResolvePath", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, &customMockFS{
			MockFileSystem: MockFileSystem{WD: "/base"},
			readFileErr:    assert.AnError,
		})
		require.NoError(t, err)
		// {{file:foo}} will trigger an error when it tries to read the file
		_, err = ResolvePath("{{file:foo}}", "/base", r)
		require.Error(t, err)
	})

	t.Run("Scheme preservation exhaustive", func(t *testing.T) {
		val, err := ResolvePath("ssh://git@github.com/org/repo", "/base", nil)
		require.NoError(t, err)
		assert.Equal(t, "ssh://git@github.com/org/repo", val)
	})

	t.Run("resolveVolumePath - no separator", func(t *testing.T) {
		val, err := resolveVolumePath("my-volume", "/base", nil)
		require.NoError(t, err)
		assert.Equal(t, "my-volume", val)
	})

	t.Run("resolveDevicePath - no separator", func(t *testing.T) {
		val, err := resolveDevicePath("/dev/fuse", "/base", nil)
		require.NoError(t, err)
		assert.Equal(t, "/dev/fuse", val)
	})

	t.Run("resolveVolumePath - ResolvePath error", func(t *testing.T) {
		mfs := &customMockFS{
			homeDirErr: assert.AnError,
		}
		_, err := NewExpressionResolverWithFS(nil, mfs)
		require.Error(t, err)
	})

	t.Run("resolveDevicePath - ResolvePath error", func(t *testing.T) {
		mfs := &customMockFS{
			homeDirErr: assert.AnError,
		}
		_, err := NewExpressionResolverWithFS(nil, mfs)
		require.Error(t, err)
	})
}

func TestUnit_Path_UnmarshalYAMLErrors(t *testing.T) {
	t.Run("MountConfig", func(t *testing.T) {
		var mc MountConfig

		// Valid (structure)
		yamlStr := `
type: bind
source: ./data
target: /app/data
read_only: true
`
		err := yaml.Unmarshal([]byte(yamlStr), &mc)
		require.NoError(t, err)
		assert.Equal(t, "bind", mc.Type)
		assert.Equal(t, "./data", mc.Source.Raw)

		// Implicit type (default to bind)
		yamlStr = `
source: ./implicit
target: /app/implicit
`
		err = yaml.Unmarshal([]byte(yamlStr), &mc)
		require.NoError(t, err)
		assert.Equal(t, "bind", mc.Type)
		assert.Equal(t, "./implicit", mc.Source.Raw)

		// Invalid
		err = yaml.Unmarshal([]byte("invalid-mount"), &mc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount config")

		// Missing target
		yamlStr = `
type: bind
source: ./data
`
		err = yaml.Unmarshal([]byte(yamlStr), &mc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target is required")
	})

	t.Run("DeviceConfig", func(t *testing.T) {
		var dc DeviceConfig

		// Valid
		err := yaml.Unmarshal([]byte("/dev/video0:/dev/video0:rwm"), &dc)
		require.NoError(t, err)
		assert.Equal(t, "/dev/video0", dc.Source.Raw)

		// Invalid
		err = yaml.Unmarshal([]byte(":/container:rwm"), &dc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config")
	})
}

func TestUnit_Path_ResolveVolume_Device(t *testing.T) {
	baseDir := "/base"
	r, err := NewExpressionResolver(nil)
	require.NoError(t, err)

	t.Run("ResolveVolume", func(t *testing.T) {
		cp := ConfigPath{Raw: "./host:/container", BaseDir: baseDir}
		val, err := cp.ResolveVolume(r)
		require.NoError(t, err)
		assert.Equal(t, "/base/host:/container", val)

		cp = ConfigPath{Raw: ""}
		val, err = cp.ResolveVolume(r)
		require.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("ResolveDevice", func(t *testing.T) {
		cp := ConfigPath{Raw: "./dev:/dev:rw", BaseDir: baseDir}
		val, err := cp.ResolveDevice(r)
		require.NoError(t, err)
		assert.Equal(t, "/base/dev:/dev:rw", val)

		cp = ConfigPath{Raw: ""}
		val, err = cp.ResolveDevice(r)
		require.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("DeviceConfig.SetBaseDir", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/a"},
			Destination: ConfigPath{Raw: "/dev/b"},
		}
		dc.SetBaseDir("/base")
		assert.Equal(t, "/base", dc.Source.BaseDir)
		assert.Equal(t, "/base", dc.Destination.BaseDir)
	})
}

func TestUnit_Path_SplitHostRemainder_Windows_Invalid(t *testing.T) {
	t.Run("Windows path without separator", func(t *testing.T) {
		_, _, ok := SplitHostRemainder(`C:\only-path`)
		assert.False(t, ok)
	})
}

func TestUnit_Config_ValidateHostname(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Basic hostname", "localhost", false},
		{"Hostname with dots", "example.com", false},
		{"Hostname with hyphen", "my-host", false},
		{"Empty hostname", "", false},
		{"Too long hostname", string(make([]byte, 254)), true},
		{"Invalid characters", "host!", true},
		{"Starts with hyphen", "-host", true},
		{"Ends with hyphen", "host-", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateNetworkName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Basic network", "bridge", false},
		{"Network with dots", "my.net", false},
		{"Network with underscore", "my_net", false},
		{"Empty network", "", false},
		{"Invalid characters", "net!", true},
		{"Starts with hyphen", "-net", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetworkName(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateUserName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Basic user", "root", false},
		{"User and group", "user:group", false},
		{"UID and GID", "1000:1000", false},
		{"User with dollar", "user$", false},
		{"Empty user", "", false},
		{"Invalid characters", "user!", true},
		{"Too many colons", "user:group:extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserName(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}


func TestUnit_Config_ValidateExposePort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Port", "80", false},
		{"Port range", "80-90", false},
		{"Port with proto", "80/tcp", false},
		{"Range with proto", "80-90/udp", false},
		{"Empty port", "", false},
		{"Invalid protocol", "80/http", true},
		{"Invalid range", "80-extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExposePort(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}
