package command

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/runtime/controlsocket"
)

func TestUnit_Snapshot_ControlSocket_Creation(t *testing.T) {
	t.Parallel()
	mfs := config.RealFileSystem{}
	logger := logging.NewLogger()

	globalCfg := &config.CDERunConfig{}
	toolsCfg := config.ToolsConfig{}

	containerDir, hostDir, ctrlServer, err := createSnapshot(logger, mfs, globalCfg, toolsCfg, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, ctrlServer)

	defer func() {
		_ = ctrlServer.Close()
		_ = cleanupSnapshot(mfs, containerDir)
	}()

	// Verify Control Socket file was created
	socketPath := filepath.Join(containerDir, "cderun.sock")
	_, err = os.Stat(socketPath)
	require.NoError(t, err, "expected cderun.sock to exist in snapshot directory")

	// Read generated .cderun.yaml and verify hostContext.controlSocket is populated
	cderunYAMLPath := filepath.Join(containerDir, ".cderun.yaml")
	data, err := os.ReadFile(cderunYAMLPath)
	require.NoError(t, err)

	var snapshotCfg config.CDERunConfig
	err = yaml.Unmarshal(data, &snapshotCfg)
	require.NoError(t, err)

	require.NotNil(t, snapshotCfg.HostContext)
	expectedHostSocketPath := filepath.Join(hostDir, "cderun.sock")
	assert.Equal(t, expectedHostSocketPath, snapshotCfg.HostContext.ControlSocket)

	// Test client connection and ping over created control socket with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	err = client.Ping(ctx)
	require.NoError(t, err)
}

func TestUnit_Command_MountCderunSocket_MountConfig(t *testing.T) {
	mfs := &config.MockFileSystem{}
	localOpts := defaultOptions()
	localOpts.fs = mfs
	localOpts.configLoader = config.NewConfigLoaderWithFS(mfs)

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "container-123"

	localOpts.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return mockRt, nil
	}

	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-cderun-socket", "--image=alpine", "sh"}, func(o *rootOptions, c *cobra.Command) {
		*o = localOpts
	})
	require.NoError(t, err)

	createdConfig := mockRt.GetCreatedConfig()
	require.NotNil(t, createdConfig)

	// Verify that /run/cderun/cderun.sock is mounted into the container with matching source
	var foundSocketMount bool
	for _, m := range createdConfig.Mounts {
		if m.Target == "/run/cderun/cderun.sock" {
			foundSocketMount = true
			assert.Equal(t, "bind", m.Type)
			assert.False(t, m.ReadOnly)
			assert.True(t, strings.HasSuffix(m.Source, "cderun.sock"), "expected mount source to end with cderun.sock, got: %s", m.Source)
		}
	}
	assert.True(t, foundSocketMount, "expected /run/cderun/cderun.sock to be mounted in container config")
}

func TestUnit_Command_MountCderunSocket_RealFS_Integration(t *testing.T) {
	realFS := config.RealFileSystem{}
	localOpts := defaultOptions()
	localOpts.fs = realFS
	localOpts.configLoader = config.NewConfigLoaderWithFS(realFS)

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "container-real-fs-123"

	var pingErr error
	var socketPathChecked string

	mockRt.WaitFunc = func(ctx context.Context, id string) (int, error) {
		cfg := mockRt.GetCreatedConfig()
		if cfg != nil {
			for _, m := range cfg.Mounts {
				if m.Target == "/run/cderun/cderun.sock" {
					socketPathChecked = m.Source
					break
				}
			}
		}

		if socketPathChecked != "" {
			if _, err := os.Stat(socketPathChecked); err != nil {
				pingErr = err
				return 1, err
			}

			client, err := controlsocket.Connect(ctx, socketPathChecked)
			if err != nil {
				pingErr = err
				return 1, err
			}
			defer client.Close()

			pingErr = client.Ping(ctx)
		}
		return 0, nil
	}

	localOpts.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return mockRt, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := ExecuteContextWithOptions(ctx, []string{"cderun", "--mount-cderun-socket", "--image=alpine", "sh"}, func(o *rootOptions, c *cobra.Command) {
		*o = localOpts
	})
	require.NoError(t, err)
	require.NoError(t, pingErr, "expected control socket ping inside container execution lifecycle to succeed")
	require.NotEmpty(t, socketPathChecked, "expected control socket mount source path to be set")
}
