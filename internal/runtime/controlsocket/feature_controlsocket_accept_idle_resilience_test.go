package controlsocket

import (
	"context"
	"net"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
)

type mockListenerWithErrors struct {
	net.Listener
	mu            sync.Mutex
	tempErrors    int
	permError     bool
	closed        bool
	acceptStarted chan struct{}
}

func (m *mockListenerWithErrors) Accept() (net.Conn, error) {
	if m.acceptStarted != nil {
		select {
		case <-m.acceptStarted:
		default:
			close(m.acceptStarted)
		}
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, net.ErrClosed
	}

	if m.tempErrors > 0 {
		m.tempErrors--
		m.mu.Unlock()
		return nil, &net.OpError{
			Op:  "accept",
			Net: "unix",
			Err: syscall.EAGAIN,
		}
	}

	if m.permError {
		m.mu.Unlock()
		return nil, &net.OpError{
			Op:  "accept",
			Net: "unix",
			Err: syscall.EBADF,
		}
	}
	m.mu.Unlock()

	return m.Listener.Accept()
}

func (m *mockListenerWithErrors) Close() error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return m.Listener.Close()
}

func TestUnit_ControlSocket_AcceptLoop_BackoffAndPermanentError_Scenarios(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "accept_resilience.sock")

	logger := logging.NewLogger()
	server := NewServer(socketPath, logger)

	realListener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	acceptStarted := make(chan struct{})
	mockList := &mockListenerWithErrors{
		Listener:      realListener,
		tempErrors:    2,
		acceptStarted: acceptStarted,
	}

	server.listener = mockList
	server.wg.Add(1)
	go server.acceptLoop()

	defer server.Close()

	// Wait for accept loop to start and run through temporary errors
	<-acceptStarted

	// Verify client can connect after temporary accept errors are retried
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)

	err = client.Ping(ctx)
	require.NoError(t, err)

	// Close client connection so handleConn goroutine exits
	_ = client.Close()

	// Set permError before dialing a new connection so mockList.Accept() immediately hits permError
	mockList.mu.Lock()
	mockList.permError = true
	mockList.mu.Unlock()

	// Trigger accept loop to re-enter Accept() by dialing
	var d net.Dialer
	conn, _ := d.DialContext(ctx, "unix", socketPath)
	if conn != nil {
		_ = conn.Close()
	}

	// Wait for accept loop goroutine to finish due to permanent error
	done := make(chan struct{})
	go func() {
		server.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Accept loop exited on permanent error as expected
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for accept loop to exit on permanent error")
	}
}

func TestUnit_ControlSocket_IdleTimeout_ExemptionAndRestoration_Scenarios(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "idle_resilience.sock")

	waitBlocker := make(chan struct{})
	waitStarted := make(chan struct{})

	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			close(waitStarted)
			<-waitBlocker
			return 42, nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetIdleTimeout(100 * time.Millisecond)
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	type waitRes struct {
		exitCode int
		err      error
	}
	waitResChan := make(chan waitRes, 1)

	// 1. Send WaitContainer request to enter long-running operation
	go func() {
		exitCode, err := client.WaitContainer(ctx, "c-idle-test")
		waitResChan <- waitRes{exitCode: exitCode, err: err}
	}()

	<-waitStarted

	// Verify server handler count is active
	require.Eventually(t, func() bool {
		return server.ActiveHandlerCount() == 1
	}, time.Second, 10*time.Millisecond)

	// Keep WaitContainer blocked for longer than the idle timeout (100ms) to ensure exemption
	time.Sleep(150 * time.Millisecond)

	// Unblock WaitContainer and verify successful response
	close(waitBlocker)

	res := <-waitResChan
	require.NoError(t, res.err)
	assert.Equal(t, 42, res.exitCode)

	// Verify active handler count returns to 0
	require.Eventually(t, func() bool {
		return server.ActiveHandlerCount() == 0
	}, time.Second, 10*time.Millisecond)

	// Now that long-running op is finished, connection idle timeout (100ms) is restored.
	// Allow 150ms of inactivity so the idle timeout triggers server-side connection teardown.
	time.Sleep(150 * time.Millisecond)

	// Next RPC attempt should fail due to idle timeout connection closure
	err = client.Ping(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ping")
}
