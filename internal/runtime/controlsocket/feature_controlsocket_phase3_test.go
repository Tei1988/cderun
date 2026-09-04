package controlsocket

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type phase3Dispatcher struct {
	mu           sync.Mutex
	lastSignal   string
	lastTTYRows  uint
	lastTTYCols  uint
	attachCalled bool
}

func (m *phase3Dispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	return "c-phase3-123", nil
}

func (m *phase3Dispatcher) StartContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *phase3Dispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	return 0, nil
}

func (m *phase3Dispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *phase3Dispatcher) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.lastSignal = sig
	m.mu.Unlock()
	if sig == "INVALID" {
		return errors.New("invalid signal")
	}
	return nil
}

func (m *phase3Dispatcher) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.mu.Lock()
	m.lastTTYRows = rows
	m.lastTTYCols = cols
	m.mu.Unlock()
	if rows == 0 || cols == 0 {
		return errors.New("invalid terminal size")
	}
	return nil
}

func (m *phase3Dispatcher) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	m.mu.Lock()
	m.attachCalled = true
	m.mu.Unlock()

	if containerID == "err-container" {
		return errors.New("failed to attach to container")
	}

	if ready != nil {
		close(ready)
	}

	if tty {
		if stdin != nil {
			input, _ := io.ReadAll(stdin)
			_, _ = stdout.Write([]byte("TTY_ECHO: " + string(input)))
		} else {
			_, _ = stdout.Write([]byte("TTY_OUTPUT"))
		}
	} else {
		if stdin != nil {
			input, _ := io.ReadAll(stdin)
			_, _ = stdout.Write([]byte("STDOUT_ECHO: " + string(input)))
		} else {
			_, _ = stdout.Write([]byte("STDOUT_LINE\n"))
		}
		if stderr != nil {
			_, _ = stderr.Write([]byte("STDERR_LINE\n"))
		}
	}

	return nil
}

func TestUnit_ControlSocket_Phase3_SignalAndResize(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "phase3_sig_resize.sock")

	disp := &phase3Dispatcher{}
	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. SignalContainer success
	err = client.SignalContainer(ctx, "c-phase3-123", "SIGTERM")
	require.NoError(t, err)
	disp.mu.Lock()
	assert.Equal(t, "SIGTERM", disp.lastSignal)
	disp.mu.Unlock()

	// 2. SignalContainer error
	err = client.SignalContainer(ctx, "c-phase3-123", "INVALID")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signal")

	// 3. ResizeContainerTTY success
	err = client.ResizeContainerTTY(ctx, "c-phase3-123", 24, 80)
	require.NoError(t, err)
	disp.mu.Lock()
	assert.Equal(t, uint(24), disp.lastTTYRows)
	assert.Equal(t, uint(80), disp.lastTTYCols)
	disp.mu.Unlock()

	// 4. ResizeContainerTTY error
	err = client.ResizeContainerTTY(ctx, "c-phase3-123", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid terminal size")
}

func TestUnit_ControlSocket_Phase3_Attach_NonTTY(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "phase3_attach_nontty.sock")

	disp := &phase3Dispatcher{}
	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("hello phase 3\n")
	ready := make(chan struct{})

	err = client.AttachContainer(ctx, "c-phase3-123", false, stdin, &stdout, &stderr, ready)
	require.NoError(t, err)

	select {
	case <-ready:
	default:
		t.Fatal("ready channel should be closed")
	}

	assert.Equal(t, "STDOUT_ECHO: hello phase 3\n", stdout.String())
	assert.Equal(t, "STDERR_LINE\n", stderr.String())
}

func TestUnit_ControlSocket_Phase3_Attach_TTY(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "phase3_attach_tty.sock")

	disp := &phase3Dispatcher{}
	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	var stdout bytes.Buffer
	stdin := strings.NewReader("tty payload")
	ready := make(chan struct{})

	err = client.AttachContainer(ctx, "c-phase3-123", true, stdin, &stdout, nil, ready)
	require.NoError(t, err)

	select {
	case <-ready:
	default:
		t.Fatal("ready channel should be closed")
	}

	assert.Equal(t, "TTY_ECHO: tty payload", stdout.String())
}

func TestUnit_ControlSocket_Phase3_Attach_Error(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "phase3_attach_err.sock")

	disp := &phase3Dispatcher{}
	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	ready := make(chan struct{})
	err = client.AttachContainer(ctx, "err-container", false, nil, nil, nil, ready)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to attach to container")

	select {
	case <-ready:
	default:
		t.Fatal("ready channel should be closed even on error")
	}
}

type errReader struct{}

func (e *errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("failing stdin read")
}

func TestUnit_ControlSocket_Phase3_Attach_StdinErrorPropagation(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "phase3_stdin_err.sock")

	disp := &phase3Dispatcher{}
	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	err = client.AttachContainer(ctx, "c-phase3-123", true, &errReader{}, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failing stdin read")
}

func TestUnit_ControlSocket_Phase3_DialAndHandshake_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "silent_listener.sock")

	l, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer l.Close()

	connAccepted := make(chan struct{})
	clientDone := make(chan struct{})

	go func() {
		conn, err := l.Accept()
		close(connAccepted)
		if err == nil {
			defer conn.Close()
			<-clientDone
		}
	}()

	client := &Client{socketPath: socketPath}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.dialAndHandshake(ctx)
	close(clientDone)
	<-connAccepted

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
