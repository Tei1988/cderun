package controlsocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/version"
)

func TestUnit_ControlSocket_Framing(t *testing.T) {
	t.Run("Valid frame write and read", func(t *testing.T) {
		buf := new(bytes.Buffer)
		payload := []byte("hello control socket")

		err := WriteFrame(buf, payload)
		require.NoError(t, err)

		readPayload, err := ReadFrame(buf)
		require.NoError(t, err)
		assert.Equal(t, payload, readPayload)
	})

	t.Run("Empty frame payload", func(t *testing.T) {
		buf := new(bytes.Buffer)
		err := WriteFrame(buf, nil)
		require.NoError(t, err)

		readPayload, err := ReadFrame(buf)
		require.NoError(t, err)
		assert.Empty(t, readPayload)
	})

	t.Run("Oversized payload rejection", func(t *testing.T) {
		buf := new(bytes.Buffer)
		oversized := make([]byte, MaxFrameSize+1)

		err := WriteFrame(buf, oversized)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum allowed size")
	})
}

func TestUnit_ControlSocket_Handshake_Success(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun.sock")

	server := NewServer(socketPath, logging.NewLogger())
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	assert.Equal(t, version.Version, client.ServerVersion())

	err = client.Ping(ctx)
	require.NoError(t, err)
}

func TestUnit_ControlSocket_Handshake_Rejected_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun.sock")

	server := NewServer(socketPath, logging.NewLogger())
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	// Simulate client connecting with mismatched protocol version
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	// Send handshake request with unsupported protocol version 999
	req := HandshakeRequest{
		ProtocolVersion: 999,
		ClientVersion:   "old-v0.1.0",
	}
	reqData, err := reqBytes(req)
	require.NoError(t, err)

	err = WriteFrame(conn, reqData)
	require.NoError(t, err)

	respBytes, err := ReadFrame(conn)
	require.NoError(t, err)

	resp, err := parseHandshakeResp(respBytes)
	require.NoError(t, err)

	assert.False(t, resp.Accepted)
	assert.Contains(t, resp.Error, "unsupported protocol version: client requested v999")
}

func TestUnit_ControlSocket_Server_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun.sock")

	server := NewServer(socketPath, logging.NewLogger())
	err := server.Start()
	require.NoError(t, err)

	// Ensure socket file exists
	_, err = os.Stat(socketPath)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Connect multiple clients
	client1, err := Connect(ctx, socketPath)
	require.NoError(t, err)

	client2, err := Connect(ctx, socketPath)
	require.NoError(t, err)

	// Close server and verify teardown
	err = server.Close()
	require.NoError(t, err)

	_ = client1.Close()
	_ = client2.Close()

	// Socket file should be removed
	_, err = os.Stat(socketPath)
	assert.True(t, os.IsNotExist(err), "expected socket file to be removed on server Close()")
}

func reqBytes(req HandshakeRequest) ([]byte, error) {
	return []byte(fmt.Sprintf(`{"protocolVersion":%d,"clientVersion":%q}`, req.ProtocolVersion, req.ClientVersion)), nil
}

func parseHandshakeResp(data []byte) (HandshakeResponse, error) {
	var resp HandshakeResponse
	err := json.Unmarshal(data, &resp)
	return resp, err
}
