package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T {
	return &v
}

func TestUnit_Resolver_Priority_MergeLayers(t *testing.T) {
	t.Parallel()
	t.Run("P1 Override takes priority over P2 CLI", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{
			Image:        "alpine",
			ImageSet:     true,
			CderunImage:  "ubuntu",
			CderunImageSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "ubuntu", res.Image)
	})

	t.Run("P2 CLI takes priority over P3 Env", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{
			Image:    "alpine",
			ImageSet: true,
		}
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_IMAGE": "ubuntu"}}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "alpine", res.Image)
	})

	t.Run("P3 Env takes priority over P4 Tool", func(t *testing.T) {
		t.Parallel()
		tools := ToolsConfig{
			"sh": ToolConfig{Image: "debian"},
		}
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_IMAGE": "ubuntu"}}
		res, err := ResolveWithFS("sh", CLIOptions{}, tools, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "ubuntu", res.Image)
	})

	t.Run("P4 Tool takes priority over P5 Global", func(t *testing.T) {
		t.Parallel()
		tools := ToolsConfig{
			"sh": ToolConfig{Image: "debian"},
		}
		global := &CDERunConfig{Runtime: "docker"}
		res, err := ResolveWithFS("sh", CLIOptions{}, tools, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "debian", res.Image)
	})

	t.Run("P1 boolean flag priority", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{
			TTY:            true,
			TTYSet:         true,
			CderunTTY:      false,
			CderunTTYSet:   true,
			CderunImage:    "alpine",
			CderunImageSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.False(t, res.TTY)
	})
}

func TestUnit_Resolver_Environment_AutoDetection(t *testing.T) {
	t.Parallel()
	t.Run("detects docker socket", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			Files: map[string][]byte{"/var/run/docker.sock": {}},
			Dirs:  map[string]bool{"/var/run": true},
		}
		cli := CLIOptions{CderunImage: "alpine", CderunImageSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
	})

	t.Run("detects podman socket", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			Files: map[string][]byte{"/run/podman/podman.sock": {}},
			Dirs:  map[string]bool{"/run/podman": true},
		}
		cli := CLIOptions{CderunImage: "alpine", CderunImageSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("CDERUN_RUNTIME env takes priority", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_RUNTIME": "podman"},
			Files: map[string][]byte{"/var/run/docker.sock": {}},
		}
		cli := CLIOptions{CderunImage: "alpine", CderunImageSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
	})
}

func TestUnit_Resolver_Mounts_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("Mount resolution priority", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{
			Mounts: []string{"type=bind,source=/cli,target=/mnt"},
			CderunMounts: []string{"type=bind,source=/p1,target=/mnt"},
			CderunImage: "alpine",
			CderunImageSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/p1", res.Mounts[0].Source)
	})

	t.Run("Mounts from CDERUN_MOUNT env", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{Env: map[string]string{
			"CDERUN_MOUNT": "type=bind,source=/env,target=/mnt",
			"CDERUN_IMAGE": "alpine",
		}}
		res, err := ResolveWithFS("sh", CLIOptions{}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/env", res.Mounts[0].Source)
	})
}

func TestUnit_Resolver_Env_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("Env resolution with P1 priority", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{
			Env: []string{"VAR=cli"},
			CderunEnv: []string{"VAR=p1"},
			CderunImage: "alpine",
			CderunImageSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Contains(t, res.Env, "VAR=p1")
	})

	t.Run("Env merging from multiple sources", func(t *testing.T) {
		t.Parallel()
		// resolveEnv only takes from the winning source, it doesn't merge across sources.
		tools := ToolsConfig{"sh": ToolConfig{Image: "alpine", Env: []string{"T1=1", "T2=2"}}}
		global := &CDERunConfig{Defaults: ConfigDefaults{Env: []string{"G1=1", "T1=global"}}}
		res, err := ResolveWithFS("sh", CLIOptions{}, tools, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Contains(t, res.Env, "T1=1")
		assert.Contains(t, res.Env, "T2=2")
		assert.NotContains(t, res.Env, "G1=1")
	})
}

func TestUnit_Resolver_Devices_Advanced(t *testing.T) {
	t.Parallel()
	t.Run("Device resolution priority", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Devices: []DeviceConfig{{
					Source: ConfigPath{Raw: "/dev/global"},
					Destination: ConfigPath{Raw: "/dev/global"},
				}},
			},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:alpine",
				Devices: []DeviceConfig{{
					Source: ConfigPath{Raw: "/dev/tool"},
					Destination: ConfigPath{Raw: "/dev/tool"},
				}},
			},
		}
		res, err := ResolveWithFS("node", CLIOptions{}, tools, global, mfs)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/tool", res.Devices[0].PathOnHost)
	})
}

func TestUnit_Resolver_Misc_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("User resolution", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{User: "root", UserSet: true, CderunImage: "alpine", CderunImageSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "root", res.User)
	})

	t.Run("Privileged resolution", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{Privileged: true, PrivilegedSet: true, CderunImage: "alpine", CderunImageSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.True(t, res.Privileged)
	})
}

func TestUnit_Resolver_Expressions_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("Expression in environment variable", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{WD: "/work"}
		cli := CLIOptions{Env: []string{"MY_DIR={{PWD}}"}, CderunImage: "alpine", CderunImageSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "MY_DIR=/work")
	})
}

func TestUnit_Resolver_Exhaustive_Additional(t *testing.T) {
	t.Parallel()
	t.Run("Pull policy resolution", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{Pull: "always", PullSet: true, CderunImage: "alpine", CderunImageSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "always", res.Pull)
	})

	t.Run("Memory limit resolution", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{Memory: "512MiB", MemorySet: true, CderunImage: "alpine", CderunImageSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, int64(536870912), res.Memory)
	})
}

func TestUnit_Resolver_Exhaustive_Advanced(t *testing.T) {
	t.Parallel()
	t.Run("CapAdd and CapDrop", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{
			CapAdd: []string{"SYS_ADMIN"},
			CapDrop: []string{"ALL"},
			CderunImage: "alpine",
			CderunImageSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, []string{"SYS_ADMIN"}, res.CapAdd)
		assert.Equal(t, []string{"ALL"}, res.CapDrop)
	})
}
