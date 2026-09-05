package controlsocket

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
)

func TestUnit_ControlSocket_Phase3_SignalContainer(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "signal.sock")

	var receivedID, receivedSig string
	disp := &mockDispatcher{
		signalFunc: func(ctx context.Context, containerID string, sig string) error {
			if containerID == "err-container" {
				return errors.New("signal failed")
			}
			receivedID = containerID
			receivedSig = sig
			return nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. Success branch
	err = client.SignalContainer(ctx, "c-123", "SIGTERM")
	require.NoError(t, err)
	assert.Equal(t, "c-123", receivedID)
	assert.Equal(t, "SIGTERM", receivedSig)

	// 2. Error branch
	err = client.SignalContainer(ctx, "err-container", "SIGKILL")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal failed")
}

func TestUnit_ControlSocket_Phase3_ResizeContainerTTY(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "resize.sock")

	var receivedID string
	var receivedRows, receivedCols uint
	disp := &mockDispatcher{
		resizeFunc: func(ctx context.Context, containerID string, rows, cols uint) error {
			if containerID == "err-container" {
				return errors.New("resize failed")
			}
			receivedID = containerID
			receivedRows = rows
			receivedCols = cols
			return nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. Success branch
	err = client.ResizeContainerTTY(ctx, "c-123", 24, 80)
	require.NoError(t, err)
	assert.Equal(t, "c-123", receivedID)
	assert.Equal(t, uint(24), receivedRows)
	assert.Equal(t, uint(80), receivedCols)

	// 2. Error branch
	err = client.ResizeContainerTTY(ctx, "err-container", 40, 120)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resize failed")
}

func TestUnit_ControlSocket_Phase3_AttachContainer_TTY(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "attach_tty.sock")

	disp := &mockDispatcher{
		attachFunc: func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			assert.True(t, tty)
			if ready != nil {
				close(ready)
			}
			_, err := stdout.Write([]byte("hello raw tty output\n"))
			return err
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	var stdoutBuf bytes.Buffer
	ready := make(chan struct{})

	err = client.AttachContainer(ctx, "c-tty", true, nil, &stdoutBuf, nil, ready)
	require.NoError(t, err)

	select {
	case <-ready:
	default:
		t.Fatal("expected ready channel to be closed")
	}

	assert.Equal(t, "hello raw tty output\n", stdoutBuf.String())
}

func TestUnit_ControlSocket_Phase3_AttachContainer_NonTTY_StdCopy(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "attach_stdcopy.sock")

	disp := &mockDispatcher{
		attachFunc: func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			assert.False(t, tty)
			if ready != nil {
				close(ready)
			}

			_, _ = stdout.Write([]byte("stdout data\n"))
			_, _ = stderr.Write([]byte("stderr data\n"))
			return nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	ready := make(chan struct{})

	err = client.AttachContainer(ctx, "c-stdcopy", false, nil, &stdoutBuf, &stderrBuf, ready)
	require.NoError(t, err)

	select {
	case <-ready:
	default:
		t.Fatal("expected ready channel to be closed")
	}

	assert.Equal(t, "stdout data\n", stdoutBuf.String())
	assert.Equal(t, "stderr data\n", stderrBuf.String())
}

func TestUnit_ControlSocket_Phase3_AttachContainer_Stdin(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "attach_stdin.sock")

	var serverReceivedInput string
	disp := &mockDispatcher{
		attachFunc: func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			if ready != nil {
				close(ready)
			}
			buf := make([]byte, 1024)
			n, err := stdin.Read(buf)
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			serverReceivedInput = string(buf[:n])
			return nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	stdinInput := "ping from client stdin"
	ready := make(chan struct{})

	err = client.AttachContainer(ctx, "c-stdin", true, strings.NewReader(stdinInput), nil, nil, ready)
	require.NoError(t, err)

	assert.Equal(t, stdinInput, serverReceivedInput)
}

func TestUnit_ControlSocket_Phase3_AttachContainer_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "attach_err.sock")

	disp := &mockDispatcher{
		attachFunc: func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			// Return error without closing ready
			return errors.New("failed to attach to container")
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	ready := make(chan struct{})
	err = client.AttachContainer(ctx, "c-err", true, nil, nil, nil, ready)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to attach to container")

	select {
	case <-ready:
	default:
		t.Fatal("expected ready channel to be closed even on error")
	}
}
