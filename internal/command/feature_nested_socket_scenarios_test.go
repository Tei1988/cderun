package command

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
)

func TestUnit_Command_NestedExecution_ControlSocketScenarios(t *testing.T) {
	t.Run("nested_socket_autodetected_from_environment", func(t *testing.T) {
		socketPath := "/var/run/cderun-nested.sock"
		t.Setenv("CDERUN_SOCKET_PATH", socketPath)

		args := []string{
			"cderun",
			"sh",
			"-c", "echo nested",
			"--cderun-image", "alpine:latest",
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
		}

		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Env: map[string]string{
				"CDERUN_SOCKET_PATH": socketPath,
			},
		}

		outBuf := &bytes.Buffer{}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(io.Discard)
		})
		require.NoError(t, err)

		var res map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &res)
		require.NoError(t, err)

		assert.Equal(t, "alpine:latest", res["image"])

		// Assert socket path auto-detection resolution via config resolver
		img := "alpine:latest"
		resolved, err := config.ResolveWithFS("sh", &config.CLIOptions{Image: &img}, config.ToolsConfig{}, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, socketPath, resolved.SocketPath)
	})

	t.Run("mount_cderun_socket_flag_creates_bind_mount", func(t *testing.T) {
		socketPath := "/tmp/cderun-control.sock"
		args := []string{
			"cderun",
			"bash",
			"--cderun-image", "ubuntu:22.04",
			"--cderun-socket-path", socketPath,
			"--cderun-mount-socket",
			"--cderun-dry-run",
			"--cderun-dry-run-format", "json",
		}

		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				socketPath: []byte(""),
			},
		}

		outBuf := &bytes.Buffer{}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(outBuf)
			cmd.SetErr(io.Discard)
		})
		require.NoError(t, err)

		var res map[string]any
		err = json.Unmarshal(outBuf.Bytes(), &res)
		require.NoError(t, err)

		mounts, ok := res["mounts"].([]any)
		require.True(t, ok)

		// Find socket mount and assert that it was added
		foundSocketMount := false
		for _, m := range mounts {
			mMap, isMap := m.(map[string]any)
			if !isMap {
				continue
			}
			if mMap["source"] == socketPath || mMap["target"] == socketPath {
				foundSocketMount = true
				assert.Equal(t, "bind", mMap["type"])
				break
			}
		}
		assert.True(t, foundSocketMount, "expected mount for socket path %s in mounts: %v", socketPath, mounts)
		assert.Equal(t, "ubuntu:22.04", res["image"])
	})
}

func TestUnit_Command_Snapshot_LevelInvariants(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
	}

	t.Run("resolve_snapshot_base_dir_real_filesystem_level0", func(t *testing.T) {
		t.Parallel()

		realFS := config.RealFileSystem{}
		globalCfg := &config.CDERunConfig{
			HostContext: &config.HostContext{
				Level: 0,
			},
		}

		baseDir := resolveSnapshotBaseDir(realFS, globalCfg)
		assert.NotEmpty(t, baseDir)
		assert.Equal(t, realFS.TempDir(), baseDir)
	})

	t.Run("resolve_snapshot_base_dir_mock_filesystem_level0", func(t *testing.T) {
		t.Parallel()

		globalCfg := &config.CDERunConfig{
			HostContext: &config.HostContext{
				Level: 0,
			},
		}

		baseDir := resolveSnapshotBaseDir(mfs, globalCfg)
		assert.NotEmpty(t, baseDir)
		assert.Equal(t, mfs.TempDir(), baseDir)
	})

	t.Run("resolve_host_snapshot_dir_level1", func(t *testing.T) {
		t.Parallel()

		globalCfg := &config.CDERunConfig{
			HostContext: &config.HostContext{
				Level: 1,
			},
		}

		hostCtx := &config.HostContext{
			Level: 1,
			Mounts: []config.MountMapping{
				{
					Source: "/host/workspace",
					Target: "/workspace",
					Level:  1,
				},
			},
		}

		hostDir, err := resolveHostSnapshotDir(mfs, globalCfg, hostCtx, "/workspace/snapshot")
		require.NoError(t, err)
		assert.Equal(t, "/host/workspace/snapshot", hostDir)
	})
}
