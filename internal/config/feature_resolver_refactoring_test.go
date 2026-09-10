package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ResolverRefactoring_NeedsResolver(t *testing.T) {
	t.Run("empty slice returns false", func(t *testing.T) {
		assert.False(t, needsResolver(nil, nil))
		assert.False(t, needsResolver([]string{}, nil))
	})

	t.Run("host context level > 0 returns true even without expression markers", func(t *testing.T) {
		global := &CDERunConfig{
			HostContext: &HostContext{Level: 1},
		}
		assert.True(t, needsResolver([]string{"noflags"}, global))
	})

	t.Run("template or tilde markers return true", func(t *testing.T) {
		assert.True(t, needsResolver([]string{"{{HOME}}"}, nil))
		assert.True(t, needsResolver([]string{"~/workspace"}, nil))
		assert.False(t, needsResolver([]string{"plain_value"}, nil))
	})
}

func TestUnit_Config_ResolverRefactoring_GetResolverIfNeeded(t *testing.T) {
	rv := &resolver{
		fs: &MockFileSystem{
			Files: map[string][]byte{},
			WD:    "/test",
		},
	}

	t.Run("no resolver needed returns nil", func(t *testing.T) {
		r, err := rv.getResolverIfNeeded([]string{"plain"})
		require.NoError(t, err)
		assert.Nil(t, r)
	})

	t.Run("resolver needed returns non-nil resolver", func(t *testing.T) {
		r, err := rv.getResolverIfNeeded([]string{"{{PWD}}"})
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.Equal(t, "/test", r.Pwd)
	})
}

func TestUnit_Config_ResolverRefactoring_ResolveStringOptionWithExpressions(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"FOO": "bar",
		},
	}
	rv := &resolver{
		fs: mfs,
	}

	def := OptionDef[string]{
		EnvKey: "FOO",
	}

	t.Run("plain string resolution without expressions", func(t *testing.T) {
		val, err := rv.resolveStringOptionWithExpressions(def, true, "custom", false, "")
		require.NoError(t, err)
		assert.Equal(t, "custom", val)
	})

	t.Run("template expression resolution", func(t *testing.T) {
		val, err := rv.resolveStringOptionWithExpressions(def, true, "{{PWD}}/out", false, "")
		require.NoError(t, err)
		assert.Equal(t, "/workspace/out", val)
	})
}

func TestUnit_Config_ResolverRefactoring_ResolveStringSliceOptionResolver(t *testing.T) {
	rv := &resolver{
		fs: &MockFileSystem{
			WD: "/workspace",
		},
	}

	t.Run("no expression markers returns nil", func(t *testing.T) {
		r, err := rv.resolveStringSliceOptionResolver([]string{"a", "b"})
		require.NoError(t, err)
		assert.Nil(t, r)
	})

	t.Run("expression marker returns resolver", func(t *testing.T) {
		r, err := rv.resolveStringSliceOptionResolver([]string{"a", "{{PWD}}"})
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.Equal(t, "/workspace", r.Pwd)
	})
}

func TestUnit_Config_ResolverRefactoring_MergeEnvSmall(t *testing.T) {
	base := []string{"A=1", "B=2"}
	p2 := []string{"B=20", "C=3"}
	p1 := []string{"C=30", "D=4"}

	res := mergeEnv(base, p2, p1)
	assert.Equal(t, []string{"A=1", "B=20", "C=30", "D=4"}, res)
}

func TestUnit_Config_ResolverRefactoring_DetectRuntimeFromFS(t *testing.T) {
	t.Run("detects docker when docker socket exists", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/var/run/docker.sock": {},
			},
		}
		rt, sock := detectRuntimeFromFS(mfs)
		assert.Equal(t, "docker", rt)
		assert.Equal(t, "/var/run/docker.sock", sock)
	})

	t.Run("detects containerd when containerd socket exists", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/run/containerd/containerd.sock": {},
			},
		}
		rt, sock := detectRuntimeFromFS(mfs)
		assert.Equal(t, "containerd", rt)
		assert.Equal(t, "/run/containerd/containerd.sock", sock)
	})

	t.Run("detects podman when podman socket exists", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/run/podman/podman.sock": {},
			},
		}
		rt, sock := detectRuntimeFromFS(mfs)
		assert.Equal(t, "podman", rt)
		assert.Equal(t, "/run/podman/podman.sock", sock)
	})

	t.Run("returns empty when no sockets exist", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{},
		}
		rt, sock := detectRuntimeFromFS(mfs)
		assert.Equal(t, "", rt)
		assert.Equal(t, "", sock)
	})
}

func TestUnit_Config_ResolverRefactoring_ModularOptions(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_ULIMIT": "nofile=1024:2048",
			"CDERUN_SYSCTL": "net.ipv4.ip_forward=1",
			"CDERUN_DEVICE": "/dev/kvm:/dev/kvm:rwm",
		},
	}

	cli := &CLIOptions{
		Env: []string{"FOO=bar", "BAZ={{PWD}}"},
	}

	res := &ResolvedConfig{}
	rv := &resolver{
		subcommand: "testtool",
		cli:        cli,
		fs:         mfs,
		res:        res,
	}

	t.Run("resolveEnvOptions", func(t *testing.T) {
		err := rv.resolveEnvOptions()
		require.NoError(t, err)
		assert.Contains(t, rv.res.Env, "FOO=bar")
		assert.Contains(t, rv.res.Env, "BAZ=/workspace")
	})

	t.Run("resolveUlimitOptions", func(t *testing.T) {
		err := rv.resolveUlimitOptions()
		require.NoError(t, err)
		require.Len(t, rv.res.Ulimits, 1)
		assert.Equal(t, "nofile", rv.res.Ulimits[0].Name)
		assert.Equal(t, int64(1024), rv.res.Ulimits[0].Soft)
		assert.Equal(t, int64(2048), rv.res.Ulimits[0].Hard)
	})

	t.Run("resolveSysctlOptions", func(t *testing.T) {
		err := rv.resolveSysctlOptions()
		require.NoError(t, err)
		require.NotNil(t, rv.res.Sysctls)
		assert.Equal(t, "1", rv.res.Sysctls["net.ipv4.ip_forward"])
	})

	t.Run("resolveDeviceOptions", func(t *testing.T) {
		err := rv.resolveDeviceOptions()
		require.NoError(t, err)
		require.Len(t, rv.res.Devices, 1)
		assert.Equal(t, "/dev/kvm", rv.res.Devices[0].PathOnHost)
		assert.Equal(t, "/dev/kvm", rv.res.Devices[0].PathInContainer)
		assert.Equal(t, "rwm", rv.res.Devices[0].CgroupPermissions)
	})
}
