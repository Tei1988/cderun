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

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/version"
)

func TestUnit_ControlSocket_Server_TimeoutAndDispatcherConfiguration(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "config_test.sock")
	logger := logging.GetGlobalLogger()
	server := NewServer(socketPath, logger)
	require.NotNil(t, server)

	// Verify defaults
	assert.Equal(t, 60*time.Second, server.getIdleTimeout())

	// Set timeouts
	server.SetCloseTimeout(100 * time.Millisecond)
	server.SetIdleTimeout(200 * time.Millisecond)
	assert.Equal(t, 200*time.Millisecond, server.getIdleTimeout())

	// Set dispatcher and parent config
	mockDisp := &mockDispatcher{}
	server.SetDispatcher(mockDisp)
	assert.Equal(t, mockDisp, server.getDispatcher())

	parentCfg := &container.ContainerConfig{Image: "alpine:latest"}
	server.SetParentConfig(parentCfg)
	assert.Equal(t, parentCfg, server.parentConfig)

	// Handler tracking
	assert.Equal(t, 0, server.ActiveHandlerCount())
	seq, done := server.trackHandlerStart("TestRPC")
	assert.Positive(t, seq)
	assert.Equal(t, 1, server.ActiveHandlerCount())
	done()
	assert.Equal(t, 0, server.ActiveHandlerCount())
}

func TestUnit_ControlSocket_Server_HandshakeVersionSkewLogging(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "skew_test.sock")
	logger := logging.GetGlobalLogger()
	server := NewServer(socketPath, logger)
	require.NoError(t, server.Start())
	defer func() { _ = server.Close() }()

	clientConn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer clientConn.Close()

	// Send valid handshake with different client binary version (version skew)
	req := HandshakeRequest{
		ProtocolVersion: CurrentProtocolVersion,
		ClientVersion:   "v9.9.9-skew",
	}
	reqData, err := json.Marshal(req)
	require.NoError(t, err)

	require.NoError(t, WriteFrame(clientConn, reqData))

	respData, err := ReadFrame(clientConn)
	require.NoError(t, err)

	var resp HandshakeResponse
	require.NoError(t, json.Unmarshal(respData, &resp))

	assert.True(t, resp.Accepted)
	assert.Equal(t, CurrentProtocolVersion, resp.ProtocolVersion)
	assert.Equal(t, version.Version, resp.ServerVersion)
	assert.Empty(t, resp.Error)
}

func TestUnit_ControlSocket_Server_RequestContextAndDeadlineBuilding(t *testing.T) {
	t.Parallel()

	logger := logging.GetGlobalLogger()
	server := NewServer("/tmp/dummy.sock", logger)

	// Case 1: No frame deadline set
	frameNoDeadline := &RequestFrame{Type: MsgPing}
	ctx1, cancel1 := server.buildRequestContext(context.Background(), frameNoDeadline)
	defer cancel1()
	assert.NotNil(t, ctx1)
	_, hasDeadline1 := ctx1.Deadline()
	assert.False(t, hasDeadline1)

	// Case 2: Explicit frame deadline set
	futureDeadline := time.Now().Add(5 * time.Second)
	frameWithDeadline := &RequestFrame{Type: MsgPing, Deadline: &futureDeadline}
	ctx2, cancel2 := server.buildRequestContext(context.Background(), frameWithDeadline)
	defer cancel2()
	assert.NotNil(t, ctx2)
	dl2, hasDeadline2 := ctx2.Deadline()
	assert.True(t, hasDeadline2)
	assert.WithinDuration(t, futureDeadline, dl2, 50*time.Millisecond)
}
