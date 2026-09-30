package command

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/runtime/controlsocket"
)

func TestUnit_ControlSocket_NerdctlDispatcherWiring(t *testing.T) {
	t.Parallel()

	realFS := config.RealFileSystem{}
	localOpts := defaultOptions()
	localOpts.fs = realFS
	localOpts.configLoader = config.NewConfigLoaderWithFS(realFS)

	nerdctlRt, err := runtime.NewNerdctlRuntime("/run/cderun/nerdctl.sock")
	require.NoError(t, err)

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "nerdctl-container-001"

	var socketPath string
	var rpcCreateErr error
	var rpcCreatedID string

	mockRt.WaitFunc = func(ctx context.Context, id string) (int, error) {
		cfg := mockRt.GetCreatedConfig()
		if cfg != nil {
			for _, m := range cfg.Mounts {
				if m.Target == "/run/cderun/cderun.sock" {
					socketPath = m.Source
					break
				}
			}
		}

		if socketPath != "" {
			if _, statErr := os.Stat(socketPath); statErr == nil {
				client, connErr := controlsocket.Connect(ctx, socketPath)
				if connErr == nil {
					defer client.Close()
					adapter := runtime.NewControlSocketRuntimeAdapter(nerdctlRt, client, logging.NewLogger())
					childConfig := &container.ContainerConfig{
						Image:   "alpine:latest",
						Command: []string{"echo", "nested-nerdctl"},
					}
					rpcCreatedID, rpcCreateErr = adapter.CreateContainer(ctx, childConfig)
				}
			}
		}
		return 0, nil
	}

	localOpts.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return mockRt, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	execErr := ExecuteContextWithOptions(ctx, []string{"cderun", "--image=alpine:latest", "--mount-cderun-socket", "--engine=nerdctl", "sh"}, func(o *rootOptions, c *cobra.Command) {
		*o = localOpts
	})
	require.NoError(t, execErr)

	// Since mockRt is dispatched, CreateContainer should be received by mockRt via control socket dispatch
	require.NoError(t, rpcCreateErr)
	assert.NotEmpty(t, rpcCreatedID)
	assert.Equal(t, "nerdctl-container-001", rpcCreatedID)
}

func TestUnit_ControlSocket_NerdctlAdapter_RPCDispatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "ctrl_nerdctl.sock")

	logger := logging.NewLogger()
	ctrlServer := controlsocket.NewServer(socketPath, logger)

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "nerdctl-ctrl-123"
	mockRt.ExitCode = 42

	ctrlServer.SetDispatcher(mockRt)
	require.NoError(t, ctrlServer.Start())
	defer ctrlServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	nerdctlRt, err := runtime.NewNerdctlRuntime(socketPath)
	require.NoError(t, err)

	adapter := runtime.NewControlSocketRuntimeAdapter(nerdctlRt, client, logger)

	// Test CreateContainer
	childConfig := &container.ContainerConfig{
		Image:   "node:20-alpine",
		Command: []string{"node", "-e", "console.log('hello')"},
		Env:     []string{"NODE_ENV=production"},
	}
	id, err := adapter.CreateContainer(ctx, childConfig)
	require.NoError(t, err)
	assert.Equal(t, "nerdctl-ctrl-123", id)

	// Test StartContainer
	err = adapter.StartContainer(ctx, id)
	require.NoError(t, err)

	// Test SignalContainer
	err = adapter.SignalContainer(ctx, id, "SIGTERM")
	require.NoError(t, err)

	// Test ResizeContainerTTY
	err = adapter.ResizeContainerTTY(ctx, id, 24, 80)
	require.NoError(t, err)

	// Test WaitContainer
	code, err := adapter.WaitContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 42, code)

	// Test RemoveContainer
	err = adapter.RemoveContainer(ctx, id)
	require.NoError(t, err)

	assert.Equal(t, "node:20-alpine", mockRt.GetCreatedConfig().Image)
	assert.Equal(t, []string{"node", "-e", "console.log('hello')"}, mockRt.GetCreatedConfig().Command)
}
