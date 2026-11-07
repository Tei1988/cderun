package controlsocket

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/version"
)

func TestUnit_ControlSocket_Client_Watchdog_ContextCanceled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "watchdog_cancel.sock")

	waitStarted := make(chan struct{})
	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			close(waitStarted)
			<-ctx.Done()
			return -1, ctx.Err()
		},
	}

	server := NewServer(sockPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	t.Cleanup(func() { _ = server.Close() })

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()

	client, err := Connect(connectCtx, sockPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		_, wErr := client.WaitContainer(ctx, "c-watchdog-cancel")
		errChan <- wErr
	}()

	<-waitStarted

	// Cancel context while WaitContainer is in flight
	cancel()

	select {
	case wErr := <-errChan:
		require.Error(t, wErr)
		require.ErrorIs(t, wErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WaitContainer to be interrupted by watchdog on context cancel")
	}
}

func TestUnit_ControlSocket_Client_Watchdog_ContextDeadlineExceeded(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "watchdog_timeout.sock")

	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			select {
			case <-ctx.Done():
				return -1, ctx.Err()
			case <-time.After(5 * time.Second):
				return 0, nil
			}
		},
	}

	server := NewServer(sockPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	t.Cleanup(func() { _ = server.Close() })

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()

	client, err := Connect(connectCtx, sockPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer timeoutCancel()

	start := time.Now()
	_, wErr := client.WaitContainer(timeoutCtx, "c-watchdog-timeout")
	duration := time.Since(start)

	require.Error(t, wErr)
	require.ErrorIs(t, wErr, context.DeadlineExceeded)
	assert.Less(t, duration, 1*time.Second)
}

func TestUnit_ControlSocket_Client_Ping_Watchdog_ContextCanceled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "ping_watchdog.sock")

	l, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer l.Close()

	go func() {
		conn, aErr := l.Accept()
		if aErr != nil {
			return
		}
		defer conn.Close()

		// Read handshake
		_, _ = ReadFrame(conn)

		// Send handshake accepted response
		resp := HandshakeResponse{
			Accepted:        true,
			ProtocolVersion: CurrentProtocolVersion,
			ServerVersion:   version.Version,
		}
		respBytes, _ := json.Marshal(resp)
		_ = WriteFrame(conn, respBytes)

		// Read ping frame but deliberately do not reply
		_, _ = ReadFrame(conn)
		time.Sleep(500 * time.Millisecond)
	}()

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()

	client, err := Connect(connectCtx, sockPath)
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- client.Ping(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case pErr := <-errChan:
		require.Error(t, pErr)
		require.ErrorIs(t, pErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Ping to be interrupted by watchdog on context cancel")
	}
}

func TestUnit_ControlSocket_Client_Closed_RPC_Rejection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "closed_rejection.sock")

	server := NewServer(sockPath, logging.NewLogger())
	require.NoError(t, server.Start())
	t.Cleanup(func() { _ = server.Close() })

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()

	client, err := Connect(connectCtx, sockPath)
	require.NoError(t, err)

	// First Close should succeed
	require.NoError(t, client.Close())
	// Idempotent Close should succeed without panic
	require.NoError(t, client.Close())

	ctx := context.Background()

	require.ErrorIs(t, client.Ping(ctx), net.ErrClosed)

	_, err = client.CreateContainer(ctx, nil)
	require.ErrorIs(t, err, net.ErrClosed)

	require.ErrorIs(t, client.StartContainer(ctx, "c123"), net.ErrClosed)

	_, err = client.WaitContainer(ctx, "c123")
	require.ErrorIs(t, err, net.ErrClosed)

	require.ErrorIs(t, client.RemoveContainer(ctx, "c123"), net.ErrClosed)

	require.ErrorIs(t, client.SignalContainer(ctx, "c123", "SIGTERM"), net.ErrClosed)

	require.ErrorIs(t, client.ResizeContainerTTY(ctx, "c123", 24, 80), net.ErrClosed)
}

func TestUnit_ControlSocket_Client_Nil_Receiver_Guards(t *testing.T) {
	t.Parallel()

	var nilClient *Client

	// Nil receiver methods must return explicit errors/defaults without panicking
	require.NoError(t, nilClient.Close())
	assert.Empty(t, nilClient.ServerVersion())

	ctx := context.Background()

	err := nilClient.Ping(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")

	_, err = nilClient.CreateContainer(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")

	err = nilClient.StartContainer(ctx, "c123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")

	_, err = nilClient.WaitContainer(ctx, "c123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")

	err = nilClient.RemoveContainer(ctx, "c123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")

	err = nilClient.SignalContainer(ctx, "c123", "SIGTERM")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")

	err = nilClient.ResizeContainerTTY(ctx, "c123", 24, 80)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")

	ready := make(chan struct{})
	err = nilClient.AttachContainer(ctx, "c123", false, nil, nil, nil, ready)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")

	// Ensure ready channel was safely closed
	select {
	case <-ready:
		// OK, channel was closed
	default:
		t.Fatal("expected ready channel to be closed on nil client AttachContainer")
	}
}

func TestUnit_ControlSocket_Client_Close_Interrupts_InFlight_RPC(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "close_interrupt.sock")

	waitStarted := make(chan struct{})
	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			close(waitStarted)
			<-ctx.Done()
			return -1, ctx.Err()
		},
	}

	server := NewServer(sockPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	t.Cleanup(func() { _ = server.Close() })

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()

	client, err := Connect(connectCtx, sockPath)
	require.NoError(t, err)

	waitErrChan := make(chan error, 1)
	go func() {
		_, wErr := client.WaitContainer(context.Background(), "c-close-interrupt")
		waitErrChan <- wErr
	}()

	<-waitStarted

	// Close client while WaitContainer is actively waiting on the response
	closeStart := time.Now()
	require.NoError(t, client.Close())
	closeDuration := time.Since(closeStart)

	// Close must return promptly without waiting for the RPC response
	assert.Less(t, closeDuration, 500*time.Millisecond)

	select {
	case wErr := <-waitErrChan:
		require.Error(t, wErr)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for in-flight RPC to be unblocked by Close()")
	}
}

func TestUnit_ControlSocket_Client_Interrupted_RPC_Clears_Connection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "interrupted_desync.sock")

	waitStarted := make(chan struct{})
	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			close(waitStarted)
			<-ctx.Done()
			return -1, ctx.Err()
		},
	}

	server := NewServer(sockPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	t.Cleanup(func() { _ = server.Close() })

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()

	client, err := Connect(connectCtx, sockPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	cancelCtx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	errChan := make(chan error, 1)
	go func() {
		_, wErr := client.WaitContainer(cancelCtx, "c-desync-test")
		errChan <- wErr
	}()

	<-waitStarted
	cancelFn()

	select {
	case wErr := <-errChan:
		require.Error(t, wErr)
		require.ErrorIs(t, wErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WaitContainer to be canceled")
	}

	// Because frame exchange was interrupted, client must have discarded c.conn to prevent stream desync.
	// A subsequent RPC call must fail immediately with net.ErrClosed.
	require.ErrorIs(t, client.Ping(context.Background()), net.ErrClosed)
	_, nextErr := client.WaitContainer(context.Background(), "c-next")
	require.ErrorIs(t, nextErr, net.ErrClosed)
}
