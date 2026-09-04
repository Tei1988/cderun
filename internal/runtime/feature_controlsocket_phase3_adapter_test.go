package runtime

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/runtime/controlsocket"
)

func TestUnit_ControlSocket_Phase3_Adapter_Dispatch(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun.sock")

	underlyingMock := NewMockRuntime()

	server := controlsocket.NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(underlyingMock)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	adapter := NewControlSocketRuntimeAdapter(underlyingMock, client, logging.NewLogger())

	// 1. Test SignalContainer dispatch via Control Socket Adapter
	err = adapter.SignalContainer(ctx, "c-phase3-123", "SIGINT")
	require.NoError(t, err)
	assert.Equal(t, "c-phase3-123", underlyingMock.SignaledContainerID)
	assert.Equal(t, "SIGINT", underlyingMock.Signal)

	// 2. Test ResizeContainerTTY dispatch via Control Socket Adapter
	err = adapter.ResizeContainerTTY(ctx, "c-phase3-123", 30, 100)
	require.NoError(t, err)
	rows, cols := underlyingMock.GetTTYSize()
	assert.Equal(t, uint(30), rows)
	assert.Equal(t, uint(100), cols)

	// 3. Test AttachContainer dispatch via Control Socket Adapter
	underlyingMock.AttachFunc = func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
		if ready != nil {
			close(ready)
		}
		_, err := stdout.Write([]byte("adapter stdout output\n"))
		return err
	}

	var stdoutBuf bytes.Buffer
	ready := make(chan struct{})
	err = adapter.AttachContainer(ctx, "c-phase3-123", true, nil, &stdoutBuf, nil, ready)
	require.NoError(t, err)

	select {
	case <-ready:
	default:
		t.Fatal("expected ready channel to be closed")
	}

	assert.Equal(t, "adapter stdout output\n", stdoutBuf.String())
	assert.Equal(t, "c-phase3-123", underlyingMock.GetAttachedContainerID())
}
