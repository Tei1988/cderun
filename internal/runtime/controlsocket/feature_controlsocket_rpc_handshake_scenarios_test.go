package controlsocket

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

func TestUnit_ControlSocket_Client_Connect_And_Handshake_Failures(t *testing.T) {
	t.Parallel()

	t.Run("Connect to NonExistent Socket", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, filepath.Join(t.TempDir(), "nonexistent.sock"))
		require.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "failed to connect to control socket")
	})

	t.Run("Handshake Rejected By Server", func(t *testing.T) {
		t.Parallel()
		sockPath := filepath.Join(t.TempDir(), "rejection.sock")
		l, err := net.Listen("unix", sockPath)
		require.NoError(t, err)
		defer l.Close()

		done := make(chan struct{})
		go func() {
			defer close(done)
			conn, acceptErr := l.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()

			reqData, readErr := ReadFrame(conn)
			if readErr != nil {
				return
			}
			var req HandshakeRequest
			if err := json.Unmarshal(reqData, &req); err != nil {
				return
			}

			resp := HandshakeResponse{
				Accepted:        false,
				ProtocolVersion: CurrentProtocolVersion,
				Error:           "policy violation: unauthorized client",
			}
			respBytes, _ := json.Marshal(resp)
			_ = WriteFrame(conn, respBytes)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, sockPath)
		require.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "control socket handshake rejected: policy violation: unauthorized client")
		<-done
	})

	t.Run("Handshake Malformed Response", func(t *testing.T) {
		t.Parallel()
		sockPath := filepath.Join(t.TempDir(), "malformed.sock")
		l, err := net.Listen("unix", sockPath)
		require.NoError(t, err)
		defer l.Close()

		done := make(chan struct{})
		go func() {
			defer close(done)
			conn, acceptErr := l.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()

			_, _ = ReadFrame(conn)
			_ = WriteFrame(conn, []byte("NOT_VALID_JSON"))
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, sockPath)
		require.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "malformed handshake response from control socket")
		<-done
	})
}

func TestUnit_ControlSocket_Client_RPC_Error_And_Malformed_Responses(t *testing.T) {
	t.Parallel()

	sockPath := filepath.Join(t.TempDir(), "rpc_edge.sock")
	l, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer l.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, acceptErr := l.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				reqData, readErr := ReadFrame(c)
				if readErr != nil {
					return
				}
				var req HandshakeRequest
				if json.Unmarshal(reqData, &req) != nil {
					return
				}
				resp := HandshakeResponse{
					Accepted:        true,
					ProtocolVersion: CurrentProtocolVersion,
					ServerVersion:   "1.2.3-test",
				}
				respBytes, _ := json.Marshal(resp)
				if WriteFrame(c, respBytes) != nil {
					return
				}

				for {
					fBytes, fErr := ReadFrame(c)
					if fErr != nil {
						return
					}
					var reqFrame RequestFrame
					if json.Unmarshal(fBytes, &reqFrame) != nil {
						return
					}

					var rFrame ResponseFrame
					switch reqFrame.Type {
					case MsgCreateContainer:
						rFrame = ResponseFrame{Success: true, Payload: []byte("INVALID_CONTAINER_RESULT_JSON")}
					case MsgWaitContainer:
						rFrame = ResponseFrame{Success: true, Payload: []byte("{invalid_json}")}
					case MsgStartContainer:
						rFrame = ResponseFrame{Success: false, Error: "failed to start container"}
					case MsgRemoveContainer:
						rFrame = ResponseFrame{Success: false, Error: "failed to remove container"}
					case MsgSignalContainer:
						rFrame = ResponseFrame{Success: false, Error: "failed to signal container"}
					case MsgResizeContainerTTY:
						rFrame = ResponseFrame{Success: false, Error: "failed to resize TTY"}
					default:
						rFrame = ResponseFrame{Success: false, Error: "unsupported RPC"}
					}
					rb, _ := json.Marshal(rFrame)
					if WriteFrame(c, rb) != nil {
						return
					}
				}
			}(conn)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, sockPath)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()

	assert.Equal(t, "1.2.3-test", client.ServerVersion())

	t.Run("CreateContainer Malformed Result Payload", func(t *testing.T) {
		cid, cErr := client.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine:latest"})
		require.Error(t, cErr)
		assert.Empty(t, cid)
		assert.Contains(t, cErr.Error(), "malformed CreateContainer result")
	})

	t.Run("WaitContainer Malformed Result Payload", func(t *testing.T) {
		exitCode, wErr := client.WaitContainer(ctx, "test-id")
		require.Error(t, wErr)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, wErr.Error(), "malformed WaitContainer result")
	})

	t.Run("StartContainer RPC Error Response", func(t *testing.T) {
		sErr := client.StartContainer(ctx, "test-id")
		require.Error(t, sErr)
		assert.Contains(t, sErr.Error(), "failed to start container")
	})

	t.Run("RemoveContainer RPC Error Response", func(t *testing.T) {
		rErr := client.RemoveContainer(ctx, "test-id")
		require.Error(t, rErr)
		assert.Contains(t, rErr.Error(), "failed to remove container")
	})

	t.Run("SignalContainer RPC Error Response", func(t *testing.T) {
		sigErr := client.SignalContainer(ctx, "test-id", "SIGTERM")
		require.Error(t, sigErr)
		assert.Contains(t, sigErr.Error(), "failed to signal container")
	})

	t.Run("ResizeContainerTTY RPC Error Response", func(t *testing.T) {
		resErr := client.ResizeContainerTTY(ctx, "test-id", 24, 80)
		require.Error(t, resErr)
		assert.Contains(t, resErr.Error(), "failed to resize TTY")
	})

	t.Run("Close Nil Client Connection", func(t *testing.T) {
		nilConnClient := &Client{}
		assert.NoError(t, nilConnClient.Close())
	})

	l.Close()
	<-done
}

func TestUnit_ControlSocket_Client_AttachContainer_Error_Paths(t *testing.T) {
	t.Parallel()

	t.Run("Dial Failure Closes Ready Channel", func(t *testing.T) {
		t.Parallel()
		client := &Client{socketPath: filepath.Join(t.TempDir(), "nonexistent.sock")}
		ready := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := client.AttachContainer(ctx, "test-id", false, nil, nil, nil, ready)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to dial control socket")

		select {
		case <-ready:
		default:
			t.Fatal("expected ready channel to be closed on dial error")
		}
	})

	t.Run("Server Error Response Closes Ready Channel", func(t *testing.T) {
		t.Parallel()
		sockPath := filepath.Join(t.TempDir(), "attach_err.sock")
		l, err := net.Listen("unix", sockPath)
		require.NoError(t, err)
		defer l.Close()

		go func() {
			conn, acceptErr := l.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()

			reqData, _ := ReadFrame(conn)
			var req HandshakeRequest
			_ = json.Unmarshal(reqData, &req)
			resp := HandshakeResponse{Accepted: true, ProtocolVersion: CurrentProtocolVersion}
			respBytes, _ := json.Marshal(resp)
			_ = WriteFrame(conn, respBytes)

			_, _ = ReadFrame(conn)
			errResp := ResponseFrame{Success: false, Error: "attach refused by server"}
			errBytes, _ := json.Marshal(errResp)
			_ = WriteFrame(conn, errBytes)
		}()

		client := &Client{socketPath: sockPath}
		ready := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		aErr := client.AttachContainer(ctx, "test-id", false, nil, nil, nil, ready)
		require.Error(t, aErr)
		assert.Contains(t, aErr.Error(), "attach refused by server")

		select {
		case <-ready:
		default:
			t.Fatal("expected ready channel to be closed on server error response")
		}
	})

	t.Run("Malformed Attach Response Frame Closes Ready Channel", func(t *testing.T) {
		t.Parallel()
		sockPath := filepath.Join(t.TempDir(), "attach_malformed.sock")
		l, err := net.Listen("unix", sockPath)
		require.NoError(t, err)
		defer l.Close()

		go func() {
			conn, acceptErr := l.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()

			reqData, _ := ReadFrame(conn)
			var req HandshakeRequest
			_ = json.Unmarshal(reqData, &req)
			resp := HandshakeResponse{Accepted: true, ProtocolVersion: CurrentProtocolVersion}
			respBytes, _ := json.Marshal(resp)
			_ = WriteFrame(conn, respBytes)

			_, _ = ReadFrame(conn)
			_ = WriteFrame(conn, []byte("NOT_A_VALID_RESPONSE_FRAME"))
		}()

		client := &Client{socketPath: sockPath}
		ready := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		aErr := client.AttachContainer(ctx, "test-id", false, nil, nil, nil, ready)
		require.Error(t, aErr)
		assert.Contains(t, aErr.Error(), "malformed AttachContainer response frame")

		select {
		case <-ready:
		default:
			t.Fatal("expected ready channel to be closed on malformed response frame")
		}
	})

	t.Run("Stdin Reader Error Propagation", func(t *testing.T) {
		t.Parallel()
		sockPath := filepath.Join(t.TempDir(), "attach_stdin_err.sock")
		l, err := net.Listen("unix", sockPath)
		require.NoError(t, err)
		defer l.Close()

		go func() {
			conn, acceptErr := l.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()

			reqData, _ := ReadFrame(conn)
			var req HandshakeRequest
			_ = json.Unmarshal(reqData, &req)
			resp := HandshakeResponse{Accepted: true, ProtocolVersion: CurrentProtocolVersion}
			respBytes, _ := json.Marshal(resp)
			_ = WriteFrame(conn, respBytes)

			_, _ = ReadFrame(conn)
			okResp := ResponseFrame{Success: true}
			okBytes, _ := json.Marshal(okResp)
			_ = WriteFrame(conn, okBytes)

			buf := make([]byte, 1024)
			for {
				_, readErr := conn.Read(buf)
				if readErr != nil {
					return
				}
			}
		}()

		client := &Client{socketPath: sockPath}
		ready := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		expectedErr := errors.New("failing stdin read error")
		errReader := &failingReader{err: expectedErr}

		aErr := client.AttachContainer(ctx, "test-id", false, errReader, nil, nil, ready)
		require.Error(t, aErr)
		assert.ErrorIs(t, aErr, expectedErr)
	})
}

type failingReader struct {
	err error
}

func (f *failingReader) Read(p []byte) (int, error) {
	return 0, f.err
}

func TestUnit_ControlSocket_Server_Lifecycle_And_Protocol_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("Start Called Twice Returns Error", func(t *testing.T) {
		t.Parallel()
		sockPath := filepath.Join(t.TempDir(), "double_start.sock")
		server := NewServer(sockPath, logging.GetGlobalLogger())
		require.NoError(t, server.Start())
		defer server.Close()

		err := server.Start()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server is already running")
	})

	t.Run("Protocol Version Mismatch Handshake Response", func(t *testing.T) {
		t.Parallel()
		sockPath := filepath.Join(t.TempDir(), "proto_mismatch.sock")
		server := NewServer(sockPath, logging.GetGlobalLogger())
		require.NoError(t, server.Start())
		defer server.Close()

		conn, err := net.Dial("unix", sockPath)
		require.NoError(t, err)
		defer conn.Close()

		mismatchedReq := HandshakeRequest{
			ProtocolVersion: 999,
			ClientVersion:   "0.0.1-old",
		}
		reqBytes, _ := json.Marshal(mismatchedReq)
		require.NoError(t, WriteFrame(conn, reqBytes))

		respBytes, err := ReadFrame(conn)
		require.NoError(t, err)

		var resp HandshakeResponse
		require.NoError(t, json.Unmarshal(respBytes, &resp))

		assert.False(t, resp.Accepted)
		assert.Contains(t, resp.Error, "unsupported protocol version: client requested v999")
	})

	t.Run("Unsupported Message Type Response", func(t *testing.T) {
		t.Parallel()
		sockPath := filepath.Join(t.TempDir(), "unsupported_msg.sock")
		server := NewServer(sockPath, logging.GetGlobalLogger())
		require.NoError(t, server.Start())
		defer server.Close()

		conn, err := net.Dial("unix", sockPath)
		require.NoError(t, err)
		defer conn.Close()

		handshakeReq := HandshakeRequest{
			ProtocolVersion: CurrentProtocolVersion,
			ClientVersion:   "1.0.0",
		}
		hBytes, _ := json.Marshal(handshakeReq)
		require.NoError(t, WriteFrame(conn, hBytes))

		_, err = ReadFrame(conn)
		require.NoError(t, err)

		unknownReq := RequestFrame{
			Type:    MessageType("MsgUnknownType"),
			Payload: []byte("{}"),
		}
		uBytes, _ := json.Marshal(unknownReq)
		require.NoError(t, WriteFrame(conn, uBytes))

		respBytes, err := ReadFrame(conn)
		require.NoError(t, err)

		var resp ResponseFrame
		require.NoError(t, json.Unmarshal(respBytes, &resp))

		assert.False(t, resp.Success)
		assert.Contains(t, resp.Error, "unsupported request message type: \"MsgUnknownType\"")
	})
}
