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
)

func TestUnit_ControlSocket_RPC_Deadline_And_MalformedPayload_Scenarios(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "ctrl.sock")

	disp := &dummyDispatcher{
		waitFn: func(ctx context.Context, containerID string) (int, error) {
			select {
			case <-ctx.Done():
				return -1, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return 0, nil
			}
		},
	}

	srv := NewServer(sockPath, nil)
	srv.SetDispatcher(disp)
	require.NoError(t, srv.Start())
	t.Cleanup(func() { _ = srv.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, sockPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	t.Run("WaitContainer RPC deadline timeout", func(t *testing.T) {
		t.Parallel()

		tightCtx, tightCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer tightCancel()

		_, err := client.WaitContainer(tightCtx, "c123")
		assert.Error(t, err)
	})

	t.Run("malformed raw payloads for remaining RPC types", func(t *testing.T) {
		t.Parallel()

		rawConn, err := net.Dial("unix", sockPath)
		require.NoError(t, err)
		defer rawConn.Close()

		// Perform handshake
		hsReq, _ := json.Marshal(HandshakeRequest{ProtocolVersion: CurrentProtocolVersion, ClientVersion: "test"})
		require.NoError(t, WriteFrame(rawConn, hsReq))
		hsRespBytes, err := ReadFrame(rawConn)
		require.NoError(t, err)
		var hsResp HandshakeResponse
		require.NoError(t, json.Unmarshal(hsRespBytes, &hsResp))
		require.True(t, hsResp.Accepted)

		badCases := []struct {
			name    string
			msgType MessageType
			errSub  string
		}{
			{"StartContainer bad payload", MsgStartContainer, "malformed StartContainer args"},
			{"WaitContainer bad payload", MsgWaitContainer, "malformed WaitContainer args"},
			{"RemoveContainer bad payload", MsgRemoveContainer, "malformed RemoveContainer args"},
			{"SignalContainer bad payload", MsgSignalContainer, "malformed SignalContainer args"},
			{"ResizeContainerTTY bad payload", MsgResizeContainerTTY, "malformed ResizeContainerTTY args"},
		}

		for _, bc := range badCases {
			req, _ := json.Marshal(RequestFrame{
				Type:    bc.msgType,
				Payload: []byte("{invalid-json}"),
			})
			require.NoError(t, WriteFrame(rawConn, req))

			respBytes, err := ReadFrame(rawConn)
			require.NoError(t, err)
			var resp ResponseFrame
			require.NoError(t, json.Unmarshal(respBytes, &resp))
			assert.False(t, resp.Success, bc.name)
			assert.Contains(t, resp.Error, bc.errSub, bc.name)
		}
	})
}
