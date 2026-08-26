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

func TestUnit_ControlSocket_Phase2_NonInteractive_Dispatch(t *testing.T) {
	realFS := config.RealFileSystem{}
	logger := logging.NewLogger()

	// 1. Create a Control Socket server backed by MockRuntime dispatcher
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun.sock")

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "nested-container-456"

	ctrlServer := controlsocket.NewServer(socketPath, logger)
	ctrlServer.SetDispatcher(mockRt)
	require.NoError(t, ctrlServer.Start())
	defer ctrlServer.Close()

	// 2. Setup HostContext with active ControlSocket
	hostCtx := &config.HostContext{
		Level:         1,
		ControlSocket: socketPath,
	}

	opts := defaultOptions()
	opts.fs = realFS
	opts.configLoader = config.NewConfigLoaderWithFS(realFS)

	// Mock underlying runtime for the nested cderun process
	nestedMockRt := runtime.NewMockRuntime()
	nestedMockRt.CreatedContainerID = "nested-container-456"
	opts.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return nestedMockRt, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cfg := &config.ResolvedConfig{
		Runtime:     "docker",
		SocketPath:  "/tmp/mock.sock",
		HostContext: hostCtx,
	}
	cc := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	rt, cID, cleanup, err := opts.initContainer(ctx, cfg, cc)
	require.NoError(t, err)
	defer cleanup()

	assert.Equal(t, "mock-controlsocket", rt.Name())
	assert.Equal(t, "nested-container-456", cID)

	// Verify CreateContainer call reached the Base Host mockRt dispatcher
	createdCfg := mockRt.GetCreatedConfig()
	require.NotNil(t, createdCfg)
	assert.Equal(t, "alpine:latest", createdCfg.Image)

	// Test StartContainer dispatch
	err = rt.StartContainer(ctx, cID)
	require.NoError(t, err)

	// Test WaitContainer dispatch
	mockRt.WaitFunc = func(ctx context.Context, id string) (int, error) {
		return 0, nil
	}
	exitCode, err := rt.WaitContainer(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// Test RemoveContainer dispatch
	err = rt.RemoveContainer(ctx, cID)
	require.NoError(t, err)
}

func TestUnit_ControlSocket_Phase2_FallbackOnConnectError(t *testing.T) {
	realFS := config.RealFileSystem{}

	// Invalid socket path that cannot be connected
	invalidSocketPath := filepath.Join(t.TempDir(), "nonexistent.sock")

	hostCtx := &config.HostContext{
		Level:         1,
		ControlSocket: invalidSocketPath,
	}

	opts := defaultOptions()
	opts.fs = realFS
	nestedMockRt := runtime.NewMockRuntime()
	nestedMockRt.CreatedContainerID = "mock-container-id"

	opts.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return nestedMockRt, nil
	}

	ctx := context.Background()
	cfg := &config.ResolvedConfig{
		Runtime:     "docker",
		SocketPath:  "/tmp/mock.sock",
		HostContext: hostCtx,
	}
	cc := &container.ContainerConfig{
		Image: "alpine:latest",
	}

	rt, cID, cleanup, err := opts.initContainer(ctx, cfg, cc)
	require.NoError(t, err)
	defer cleanup()

	// Should fallback to raw runtime without error
	assert.Equal(t, "mock", rt.Name())
	assert.Equal(t, "mock-container-id", cID)
}

func TestUnit_ControlSocket_Phase2_Diagnostics_Output(t *testing.T) {
	realFS := config.RealFileSystem{}
	opts := defaultOptions()
	opts.fs = realFS

	socketPath := filepath.Join(t.TempDir(), "cderun.sock")
	_ = os.WriteFile(socketPath, []byte{}, 0600)

	globalCfg := &config.CDERunConfig{
		HostContext: &config.HostContext{
			ControlSocket: socketPath,
		},
	}

	resolved := &config.ResolvedConfig{
		Diagnosis:       true,
		DiagnosisFormat: "json",
		Runtime:         "docker",
		SocketPath:      socketPath,
	}

	cmd := &cobra.Command{}
	outBuf := &dirBuffer{}
	cmd.SetOut(outBuf)

	err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil, globalCfg)
	require.NoError(t, err)

	outStr := outBuf.String()
	assert.Contains(t, outStr, "control_socket")
	assert.Contains(t, outStr, socketPath)
}

type dirBuffer struct {
	data []byte
}

func (b *dirBuffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *dirBuffer) String() string {
	return string(b.data)
}
