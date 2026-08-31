package controlsocket

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
)

func TestUnit_ControlSocket_Framing_EdgeCases(t *testing.T) {
	t.Run("EOF during length header read", func(t *testing.T) {
		buf := bytes.NewReader([]byte{0x00, 0x00}) // short header
		_, err := ReadFrame(buf)
		require.Error(t, err)
	})

	t.Run("Oversized payload header rejection in ReadFrame", func(t *testing.T) {
		buf := new(bytes.Buffer)
		oversizedLen := MaxFrameSize + 100
		_ = binary.Write(buf, binary.BigEndian, oversizedLen)

		_, err := ReadFrame(buf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum allowed size")
	})

	t.Run("Incomplete payload read error", func(t *testing.T) {
		buf := new(bytes.Buffer)
		payloadLen := uint32(100)
		_ = binary.Write(buf, binary.BigEndian, payloadLen)
		buf.Write([]byte("short")) // only 5 bytes written

		_, err := ReadFrame(buf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read complete frame payload")
	})
}

func TestUnit_ControlSocket_Handshake_FailureModes(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun.sock")

	server := NewServer(socketPath, logging.NewLogger())
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	t.Run("Malformed JSON handshake request", func(t *testing.T) {
		conn, err := net.Dial("unix", socketPath)
		require.NoError(t, err)
		defer conn.Close()

		// Write invalid json payload
		err = WriteFrame(conn, []byte("{invalid-json"))
		require.NoError(t, err)

		respBytes, err := ReadFrame(conn)
		require.NoError(t, err)

		resp, err := parseHandshakeResp(respBytes)
		require.NoError(t, err)
		assert.False(t, resp.Accepted)
		assert.Contains(t, resp.Error, "invalid handshake payload")
	})

	t.Run("Client handshake with malformed server response", func(t *testing.T) {
		// Mock server endpoint that sends malformed handshake response
		mockPath := filepath.Join(tmpDir, "mock_malformed.sock")
		listener, err := net.Listen("unix", mockPath)
		require.NoError(t, err)
		defer listener.Close()

		go func() {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			defer c.Close()

			_, _ = ReadFrame(c)
			_ = WriteFrame(c, []byte("{not-json-resp"))
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err = Connect(ctx, mockPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed handshake response")
	})

	t.Run("Client handshake rejected propagation", func(t *testing.T) {
		mockPath := filepath.Join(tmpDir, "mock_rejected.sock")
		listener, err := net.Listen("unix", mockPath)
		require.NoError(t, err)
		defer listener.Close()

		go func() {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			defer c.Close()

			_, _ = ReadFrame(c)
			_ = WriteFrame(c, []byte(`{"accepted":false,"error":"custom rejection message"}`))
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err = Connect(ctx, mockPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom rejection message")
	})
}

func TestUnit_ControlSocket_Client_Ping_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("Ping receives malformed response", func(t *testing.T) {
		mockPath := filepath.Join(tmpDir, "ping_malformed.sock")
		listener, err := net.Listen("unix", mockPath)
		require.NoError(t, err)
		defer listener.Close()

		go func() {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			defer c.Close()

			// Read handshake req & write valid handshake resp
			_, _ = ReadFrame(c)
			_ = WriteFrame(c, []byte(`{"accepted":true,"protocolVersion":1,"serverVersion":"v1.0.0"}`))

			// Read ping frame & write invalid response
			_, _ = ReadFrame(c)
			_ = WriteFrame(c, []byte(`invalid-json-ping-resp`))
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, mockPath)
		require.NoError(t, err)
		defer client.Close()

		err = client.Ping(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed ping response")
	})

	t.Run("Ping receives unsuccessful response", func(t *testing.T) {
		mockPath := filepath.Join(tmpDir, "ping_unsuccessful.sock")
		listener, err := net.Listen("unix", mockPath)
		require.NoError(t, err)
		defer listener.Close()

		go func() {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			defer c.Close()

			_, _ = ReadFrame(c)
			_ = WriteFrame(c, []byte(`{"accepted":true,"protocolVersion":1,"serverVersion":"v1.0.0"}`))

			_, _ = ReadFrame(c)
			_ = WriteFrame(c, []byte(`{"success":false,"error":"internal server error"}`))
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, mockPath)
		require.NoError(t, err)
		defer client.Close()

		err = client.Ping(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "internal server error")
	})

	t.Run("Ping receives unexpected payload", func(t *testing.T) {
		mockPath := filepath.Join(tmpDir, "ping_wrong_payload.sock")
		listener, err := net.Listen("unix", mockPath)
		require.NoError(t, err)
		defer listener.Close()

		go func() {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			defer c.Close()

			_, _ = ReadFrame(c)
			_ = WriteFrame(c, []byte(`{"accepted":true,"protocolVersion":1,"serverVersion":"v1.0.0"}`))

			_, _ = ReadFrame(c)
			_ = WriteFrame(c, []byte(`{"success":true,"payload":"bm90LXBvbmc="}`)) // base64 "not-pong"
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, mockPath)
		require.NoError(t, err)
		defer client.Close()

		err = client.Ping(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected ping response payload")
	})
}

func TestUnit_ControlSocket_Server_Lifecycle_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun_lifecycle.sock")

	t.Run("Double start error", func(t *testing.T) {
		server := NewServer(socketPath, logging.NewLogger())
		err := server.Start()
		require.NoError(t, err)
		defer server.Close()

		err = server.Start()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server is already running")
	})

	t.Run("Double close idempotency", func(t *testing.T) {
		server := NewServer(socketPath+"_double_close", logging.NewLogger())
		err := server.Start()
		require.NoError(t, err)

		err = server.Close()
		require.NoError(t, err)

		// Second close should be a no-op returning nil
		err = server.Close()
		require.NoError(t, err)
	})

	t.Run("Stale socket cleanup on start", func(t *testing.T) {
		stalePath := filepath.Join(tmpDir, "stale.sock")
		// Create a dummy file at stalePath
		err := os.WriteFile(stalePath, []byte("stale"), 0600)
		require.NoError(t, err)

		server := NewServer(stalePath, logging.NewLogger())
		err = server.Start()
		require.NoError(t, err)
		defer server.Close()

		_, err = os.Stat(stalePath)
		assert.NoError(t, err)
	})

	t.Run("Unknown request message type handled gracefully", func(t *testing.T) {
		unknownSock := filepath.Join(tmpDir, "unknown_msg.sock")
		server := NewServer(unknownSock, logging.NewLogger())
		err := server.Start()
		require.NoError(t, err)
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, unknownSock)
		require.NoError(t, err)
		defer client.Close()

		// Send unknown frame manually
		reqFrame := RequestFrame{Type: MessageType("unknown_action")}
		data, _ := json.Marshal(reqFrame)
		err = WriteFrame(client.conn, data)
		require.NoError(t, err)

		respBytes, err := ReadFrame(client.conn)
		require.NoError(t, err)

		var resp ResponseFrame
		err = json.Unmarshal(respBytes, &resp)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Error, "unsupported request message type")
	})
}

func TestUnit_ControlSocket_PreReadinessWritesAndBufferBounding(t *testing.T) {
	outBuf := new(bytes.Buffer)
	gw := &gatedWriter{w: outBuf}

	// 1. Write pre-readiness output below limit
	earlyLog := []byte("early stdout log\n")
	_, err := gw.Write(earlyLog)
	require.NoError(t, err)

	// 2. Write pre-readiness output exceeding MaxPreReadinessBufferSize
	largeData := bytes.Repeat([]byte("X"), MaxPreReadinessBufferSize+100)
	n, err := gw.Write(largeData)
	require.NoError(t, err)
	assert.Equal(t, len(largeData), n)

	// Verify buffer size is capped exactly at MaxPreReadinessBufferSize
	assert.Equal(t, MaxPreReadinessBufferSize, gw.buffer.Len())

	// 3. Enable output delivery after readiness
	err = gw.Enable()
	require.NoError(t, err)

	// Assert the flushed output length matches exact retained byte count
	assert.Equal(t, MaxPreReadinessBufferSize, outBuf.Len())
	assert.Contains(t, outBuf.String(), "early stdout log\n")

	// 4. Post-readiness write delivers directly to destination writer
	postLog := []byte("post readiness log\n")
	_, err = gw.Write(postLog)
	require.NoError(t, err)
	assert.Equal(t, MaxPreReadinessBufferSize+len(postLog), outBuf.Len())
	assert.Contains(t, outBuf.String(), "post readiness log\n")
}
