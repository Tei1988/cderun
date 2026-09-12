package controlsocket

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/version"
)

func TestUnit_ControlSocket_PeerDisconnect_CancelsRequestContext(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "peer_disconnect.sock")

	waitStarted := make(chan struct{})
	ctxCanceled := make(chan error, 1)

	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			close(waitStarted)
			<-ctx.Done()
			ctxCanceled <- ctx.Err()
			return 0, ctx.Err()
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	require.NoError(t, err)

	// Perform Handshake
	hsReq := HandshakeRequest{
		ProtocolVersion: CurrentProtocolVersion,
		ClientVersion:   version.Version,
	}
	hsBytes, err := json.Marshal(hsReq)
	require.NoError(t, err)
	require.NoError(t, WriteFrame(conn, hsBytes))

	_, err = ReadFrame(conn)
	require.NoError(t, err)

	// Send WaitContainer request
	waitArgs := ContainerIDArgs{ContainerID: "test-container-1"}
	payload, err := json.Marshal(waitArgs)
	require.NoError(t, err)

	reqFrame := RequestFrame{
		Type:    MsgWaitContainer,
		Payload: payload,
	}
	reqBytes, err := json.Marshal(reqFrame)
	require.NoError(t, err)
	require.NoError(t, WriteFrame(conn, reqBytes))

	// Wait for handler to start executing WaitContainer
	select {
	case <-waitStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WaitContainer handler to start")
	}

	assert.Equal(t, 1, server.ActiveHandlerCount())

	// Close client connection while WaitContainer is blocking
	require.NoError(t, conn.Close())

	// Verify server handler detects disconnect and cancels request context
	select {
	case err := <-ctxCanceled:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request context cancellation on peer disconnect")
	}

	// Verify active handler count returns to 0
	require.Eventually(t, func() bool {
		return server.ActiveHandlerCount() == 0
	}, 2*time.Second, 10*time.Millisecond)
}

func TestUnit_ControlSocket_ServerClose_CancelsRequestContext(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "server_close_cancel.sock")

	waitStarted := make(chan struct{})
	ctxCanceled := make(chan error, 1)

	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			close(waitStarted)
			<-ctx.Done()
			ctxCanceled <- ctx.Err()
			return 0, ctx.Err()
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = client.WaitContainer(ctx, "c-123")
	}()

	select {
	case <-waitStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WaitContainer handler to start")
	}

	assert.Equal(t, 1, server.ActiveHandlerCount())

	// Close server while WaitContainer is active
	require.NoError(t, server.Close())

	select {
	case err := <-ctxCanceled:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request context cancellation on Server.Close()")
	}

	wg.Wait()
}

func TestUnit_ControlSocket_ServerClose_BoundedWait_LingeringHandler(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "bounded_wait.sock")

	waitStarted := make(chan struct{})

	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			close(waitStarted)
			// Simulates a lingering handler that ignores ctx.Done()
			time.Sleep(2 * time.Second)
			return 0, nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetCloseTimeout(100 * time.Millisecond)
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	go func() {
		_, _ = client.WaitContainer(ctx, "c-lingering")
	}()

	select {
	case <-waitStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WaitContainer handler to start")
	}

	start := time.Now()
	err = server.Close()
	duration := time.Since(start)

	require.NoError(t, err)
	// Must return bounded by the 100ms timeout, well below the 2s handler sleep time
	assert.Less(t, duration, 1*time.Second)
	assert.GreaterOrEqual(t, server.ActiveHandlerCount(), 1)
}

func TestUnit_ControlSocket_DataSentDuringBlockedRequest_NonConsumingDisconnect(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "data_sent_blocked.sock")

	waitStarted := make(chan struct{})
	ctxCanceled := make(chan error, 1)

	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			close(waitStarted)
			<-ctx.Done()
			ctxCanceled <- ctx.Err()
			return 0, ctx.Err()
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	require.NoError(t, err)

	// Handshake
	hsReq := HandshakeRequest{
		ProtocolVersion: CurrentProtocolVersion,
		ClientVersion:   version.Version,
	}
	hsBytes, err := json.Marshal(hsReq)
	require.NoError(t, err)
	require.NoError(t, WriteFrame(conn, hsBytes))

	_, err = ReadFrame(conn)
	require.NoError(t, err)

	// Send WaitContainer request
	waitArgs := ContainerIDArgs{ContainerID: "test-container-2"}
	payload, err := json.Marshal(waitArgs)
	require.NoError(t, err)

	reqFrame := RequestFrame{
		Type:    MsgWaitContainer,
		Payload: payload,
	}
	reqBytes, err := json.Marshal(reqFrame)
	require.NoError(t, err)
	require.NoError(t, WriteFrame(conn, reqBytes))

	// Wait for handler to block
	select {
	case <-waitStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WaitContainer handler to start")
	}

	// Send extra data over the socket while handler is blocked
	pingFrame := RequestFrame{Type: MsgPing}
	pingBytes, err := json.Marshal(pingFrame)
	require.NoError(t, err)
	require.NoError(t, WriteFrame(conn, pingBytes))

	// Disconnect client
	require.NoError(t, conn.Close())

	// Verify server handler context is canceled upon disconnect despite extra data sent
	select {
	case err := <-ctxCanceled:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request context cancellation on disconnect after data sent")
	}
}
