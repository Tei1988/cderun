package controlsocket_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/runtime/controlsocket"
)

type dummyServer struct {
	listener net.Listener
	closed   chan struct{}
}

func startDummyServer(t *testing.T, handler func(conn net.Conn)) (string, *dummyServer) {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "ctrl.sock")
	l, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	srv := &dummyServer{
		listener: l,
		closed:   make(chan struct{}),
	}

	go func() {
		defer close(srv.closed)
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()

	t.Cleanup(func() {
		_ = l.Close()
		<-srv.closed
	})

	return socketPath, srv
}

func TestClient_Handshake_RejectionAndMalformed(t *testing.T) {
	t.Parallel()

	t.Run("handshake rejected by server", func(t *testing.T) {
		t.Parallel()
		socketPath, _ := startDummyServer(t, func(conn net.Conn) {
			defer conn.Close()
			_, err := controlsocket.ReadFrame(conn)
			if err != nil {
				return
			}
			resp, _ := json.Marshal(controlsocket.HandshakeResponse{
				Accepted: false,
				Error:    "version mismatch",
			})
			_ = controlsocket.WriteFrame(conn, resp)
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := controlsocket.Connect(ctx, socketPath)
		require.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "handshake rejected")
		assert.Contains(t, err.Error(), "version mismatch")
	})

	t.Run("handshake malformed json", func(t *testing.T) {
		t.Parallel()
		socketPath, _ := startDummyServer(t, func(conn net.Conn) {
			defer conn.Close()
			_, err := controlsocket.ReadFrame(conn)
			if err != nil {
				return
			}
			_ = controlsocket.WriteFrame(conn, []byte("{invalid json}"))
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := controlsocket.Connect(ctx, socketPath)
		require.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "malformed handshake response")
	})
}

func TestClient_Ping_PayloadMismatchAndError(t *testing.T) {
	t.Parallel()

	t.Run("ping failure response", func(t *testing.T) {
		t.Parallel()
		socketPath, _ := startDummyServer(t, func(conn net.Conn) {
			defer conn.Close()
			// Handshake
			req, _ := controlsocket.ReadFrame(conn)
			assert.NotEmpty(t, req)
			resp, _ := json.Marshal(controlsocket.HandshakeResponse{
				Accepted:      true,
				ServerVersion: "1.0.0",
			})
			_ = controlsocket.WriteFrame(conn, resp)

			// Ping
			_, _ = controlsocket.ReadFrame(conn)
			errResp, _ := json.Marshal(controlsocket.ResponseFrame{
				Success: false,
				Error:   "ping internal error",
			})
			_ = controlsocket.WriteFrame(conn, errResp)
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := controlsocket.Connect(ctx, socketPath)
		require.NoError(t, err)
		defer client.Close()

		err = client.Ping(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ping failed: ping internal error")
	})

	t.Run("ping unexpected payload", func(t *testing.T) {
		t.Parallel()
		socketPath, _ := startDummyServer(t, func(conn net.Conn) {
			defer conn.Close()
			req, _ := controlsocket.ReadFrame(conn)
			assert.NotEmpty(t, req)
			resp, _ := json.Marshal(controlsocket.HandshakeResponse{
				Accepted:      true,
				ServerVersion: "1.0.0",
			})
			_ = controlsocket.WriteFrame(conn, resp)

			_, _ = controlsocket.ReadFrame(conn)
			badResp, _ := json.Marshal(controlsocket.ResponseFrame{
				Success: true,
				Payload: []byte("wrong-pong"),
			})
			_ = controlsocket.WriteFrame(conn, badResp)
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := controlsocket.Connect(ctx, socketPath)
		require.NoError(t, err)
		defer client.Close()

		err = client.Ping(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected ping response payload")
	})
}

func TestClient_RPC_MalformedResponseFrames(t *testing.T) {
	t.Parallel()

	socketPath, _ := startDummyServer(t, func(conn net.Conn) {
		defer conn.Close()
		// Handshake
		_, _ = controlsocket.ReadFrame(conn)
		resp, _ := json.Marshal(controlsocket.HandshakeResponse{
			Accepted:      true,
			ServerVersion: "1.0.0",
		})
		_ = controlsocket.WriteFrame(conn, resp)

		// Wait for RPC call
		_, _ = controlsocket.ReadFrame(conn)
		// Send malformed response
		_ = controlsocket.WriteFrame(conn, []byte("not-valid-json"))
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed CreateContainer response frame")
}

func TestClient_AttachContainer_StreamingScenarios(t *testing.T) {
	t.Parallel()

	t.Run("ready channel closed on dial error", func(t *testing.T) {
		t.Parallel()
		nonExistentSocket := filepath.Join(t.TempDir(), "nonexistent.sock")

		// Create dummy client struct pointing to non-existent path
		client, err := controlsocket.Connect(context.Background(), nonExistentSocket)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("non-tty multiplexed stream piping", func(t *testing.T) {
		t.Parallel()
		socketPath, _ := startDummyServer(t, func(conn net.Conn) {
			defer conn.Close()

			// Main connection handshake
			_, _ = controlsocket.ReadFrame(conn)
			resp, _ := json.Marshal(controlsocket.HandshakeResponse{
				Accepted:      true,
				ServerVersion: "1.0.0",
			})
			_ = controlsocket.WriteFrame(conn, resp)

			// Attach connection handshake (secondary conn handled in loop)
			frame, err := controlsocket.ReadFrame(conn)
			if err != nil {
				return
			}

			var reqFrame controlsocket.RequestFrame
			if err := json.Unmarshal(frame, &reqFrame); err != nil {
				return
			}

			if reqFrame.Type == controlsocket.MsgAttachContainer {
				okResp, _ := json.Marshal(controlsocket.ResponseFrame{
					Success: true,
				})
				if err := controlsocket.WriteFrame(conn, okResp); err != nil {
					return
				}

				// Write stdcopy formatted multiplexed output (stdout = stream 1)
				writer := stdcopy.NewStdWriter(conn, stdcopy.Stdout)
				_, _ = writer.Write([]byte("hello multiplexed stdout\n"))
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := controlsocket.Connect(ctx, socketPath)
		require.NoError(t, err)
		defer client.Close()

		ready := make(chan struct{})
		var stdoutBuf, stderrBuf bytes.Buffer

		err = client.AttachContainer(ctx, "c123", false, nil, &stdoutBuf, &stderrBuf, ready)
		require.NoError(t, err)

		<-ready
		assert.Equal(t, "hello multiplexed stdout\n", stdoutBuf.String())
		assert.Empty(t, stderrBuf.String())
	})

	t.Run("tty raw stream piping with stdin input", func(t *testing.T) {
		t.Parallel()
		expectedPayload := []byte("test input\n")
		socketPath, _ := startDummyServer(t, func(conn net.Conn) {
			defer conn.Close()

			// Handshake
			_, _ = controlsocket.ReadFrame(conn)
			resp, _ := json.Marshal(controlsocket.HandshakeResponse{
				Accepted:      true,
				ServerVersion: "1.0.0",
			})
			_ = controlsocket.WriteFrame(conn, resp)

			// Read Attach request
			frame, err := controlsocket.ReadFrame(conn)
			if err != nil {
				return
			}

			var reqFrame controlsocket.RequestFrame
			_ = json.Unmarshal(frame, &reqFrame)

			if reqFrame.Type == controlsocket.MsgAttachContainer {
				okResp, _ := json.Marshal(controlsocket.ResponseFrame{
					Success: true,
				})
				_ = controlsocket.WriteFrame(conn, okResp)

				// Read full expected stdin payload before echoing
				buf := make([]byte, len(expectedPayload))
				if _, err := io.ReadFull(conn, buf); err == nil {
					_, _ = conn.Write(append([]byte("echo: "), buf...))
				}
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := controlsocket.Connect(ctx, socketPath)
		require.NoError(t, err)
		defer client.Close()

		stdinReader := bytes.NewReader(expectedPayload)
		var stdoutBuf bytes.Buffer
		ready := make(chan struct{})

		err = client.AttachContainer(ctx, "c456", true, stdinReader, &stdoutBuf, nil, ready)
		require.NoError(t, err)

		<-ready
		assert.Equal(t, "echo: test input\n", stdoutBuf.String())
	})

	t.Run("attach error response closes ready channel", func(t *testing.T) {
		t.Parallel()
		socketPath, _ := startDummyServer(t, func(conn net.Conn) {
			defer conn.Close()

			// Handshake
			_, _ = controlsocket.ReadFrame(conn)
			resp, _ := json.Marshal(controlsocket.HandshakeResponse{
				Accepted:      true,
				ServerVersion: "1.0.0",
			})
			_ = controlsocket.WriteFrame(conn, resp)

			// Read Attach request
			frame, _ := controlsocket.ReadFrame(conn)
			var reqFrame controlsocket.RequestFrame
			_ = json.Unmarshal(frame, &reqFrame)

			if reqFrame.Type == controlsocket.MsgAttachContainer {
				errResp, _ := json.Marshal(controlsocket.ResponseFrame{
					Success: false,
					Error:   "container not found",
				})
				_ = controlsocket.WriteFrame(conn, errResp)
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := controlsocket.Connect(ctx, socketPath)
		require.NoError(t, err)
		defer client.Close()

		ready := make(chan struct{})
		err = client.AttachContainer(ctx, "nonexistent", false, nil, nil, nil, ready)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "container not found")

		// Verify ready channel was closed
		select {
		case <-ready:
			// Passed
		default:
			t.Fatal("ready channel was not closed on attach error")
		}
	})
}

type errReader struct {
	err error
}

func (e *errReader) Read(p []byte) (int, error) {
	return 0, e.err
}

func TestClient_AttachContainer_StdinReaderErrorPropagation(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("custom stdin read error")
	socketPath, _ := startDummyServer(t, func(conn net.Conn) {
		defer conn.Close()

		// Handshake
		_, _ = controlsocket.ReadFrame(conn)
		resp, _ := json.Marshal(controlsocket.HandshakeResponse{
			Accepted:      true,
			ServerVersion: "1.0.0",
		})
		_ = controlsocket.WriteFrame(conn, resp)

		// Read Attach request
		frame, err := controlsocket.ReadFrame(conn)
		if err != nil {
			return
		}

		var reqFrame controlsocket.RequestFrame
		_ = json.Unmarshal(frame, &reqFrame)

		if reqFrame.Type == controlsocket.MsgAttachContainer {
			okResp, _ := json.Marshal(controlsocket.ResponseFrame{
				Success: true,
			})
			_ = controlsocket.WriteFrame(conn, okResp)
			// Keep server connection open to allow stdin reader goroutine to attempt read
			time.Sleep(100 * time.Millisecond)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	stdinErrReader := &errReader{err: expectedErr}
	ready := make(chan struct{})

	err = client.AttachContainer(ctx, "c789", true, stdinErrReader, nil, nil, ready)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}
