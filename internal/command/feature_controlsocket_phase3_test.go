package command

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/runtime/controlsocket"
)

func TestUnit_ControlSocket_Phase3_Interactive_AttachAndSignals(t *testing.T) {
	realFS := config.RealFileSystem{}
	logger := logging.NewLogger()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun_phase3.sock")

	var attachCalled bool

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "interactive-container-789"

	mockRt.AttachFunc = func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
		attachCalled = true
		if ready != nil {
			select {
			case ready <- struct{}{}:
			default:
			}
		}

		if stdout != nil {
			_, _ = stdout.Write([]byte("container output msg"))
		}
		if stderr != nil {
			_, _ = stderr.Write([]byte("container error msg"))
		}

		if stdin != nil {
			inBuf := new(bytes.Buffer)
			_, _ = io.Copy(inBuf, stdin)
			assert.Equal(t, "user input data", inBuf.String())
		}
		return nil
	}

	ctrlServer := controlsocket.NewServer(socketPath, logger)
	ctrlServer.SetDispatcher(mockRt)
	require.NoError(t, ctrlServer.Start())
	defer ctrlServer.Close()

	hostCtx := &config.HostContext{
		Level:         1,
		ControlSocket: socketPath,
	}

	opts := defaultOptions()
	opts.fs = realFS
	opts.configLoader = config.NewConfigLoaderWithFS(realFS)

	nestedMockRt := runtime.NewMockRuntime()
	nestedMockRt.CreatedContainerID = "interactive-container-789"
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
		Image:       "alpine:latest",
		Interactive: true,
		TTY:         false,
	}

	rt, cID, cleanup, err := opts.initContainer(ctx, cfg, cc)
	require.NoError(t, err)
	defer cleanup()

	assert.Equal(t, "mock-controlsocket", rt.Name())
	assert.Equal(t, "interactive-container-789", cID)

	// 1. Test AttachContainer dispatch
	stdinBuf := bytes.NewBufferString("user input data")
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	readyChan := make(chan struct{}, 1)

	err = rt.AttachContainer(ctx, cID, false, stdinBuf, stdoutBuf, stderrBuf, readyChan)
	require.NoError(t, err)

	assert.True(t, attachCalled)
	assert.Equal(t, "container output msg", stdoutBuf.String())
	assert.Equal(t, "container error msg", stderrBuf.String())

	// 2. Test SignalContainer dispatch
	err = rt.SignalContainer(ctx, cID, "SIGINT")
	require.NoError(t, err)

	mockRt.WithLockedMock(func(m *runtime.MockRuntime) {
		assert.Equal(t, "interactive-container-789", m.SignaledContainerID)
		assert.Equal(t, "SIGINT", m.Signal)
	})

	// 3. Test ResizeContainerTTY dispatch
	err = rt.ResizeContainerTTY(ctx, cID, 30, 100)
	require.NoError(t, err)

	rows, cols := mockRt.GetTTYSize()
	assert.Equal(t, uint(30), rows)
	assert.Equal(t, uint(100), cols)
}

func TestUnit_ControlSocket_Phase3_Interactive_TTY_Mode(t *testing.T) {
	realFS := config.RealFileSystem{}
	logger := logging.NewLogger()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun_tty.sock")

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "tty-container-101"

	mockRt.AttachFunc = func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
		assert.True(t, tty)
		if ready != nil {
			select {
			case ready <- struct{}{}:
			default:
			}
		}
		if stdout != nil {
			_, _ = stdout.Write([]byte("tty terminal output\n"))
		}
		return nil
	}

	ctrlServer := controlsocket.NewServer(socketPath, logger)
	ctrlServer.SetDispatcher(mockRt)
	require.NoError(t, ctrlServer.Start())
	defer ctrlServer.Close()

	hostCtx := &config.HostContext{
		Level:         1,
		ControlSocket: socketPath,
	}

	opts := defaultOptions()
	opts.fs = realFS
	opts.configLoader = config.NewConfigLoaderWithFS(realFS)

	nestedMockRt := runtime.NewMockRuntime()
	nestedMockRt.CreatedContainerID = "tty-container-101"
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
		Image:       "alpine:latest",
		Interactive: true,
		TTY:         true,
	}

	rt, cID, cleanup, err := opts.initContainer(ctx, cfg, cc)
	require.NoError(t, err)
	defer cleanup()

	stdoutBuf := new(bytes.Buffer)
	readyChan := make(chan struct{}, 1)

	err = rt.AttachContainer(ctx, cID, true, nil, stdoutBuf, nil, readyChan)
	require.NoError(t, err)

	assert.Equal(t, "tty terminal output\n", stdoutBuf.String())
}
