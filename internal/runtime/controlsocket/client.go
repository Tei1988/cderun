package controlsocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"cderun/internal/container"
	"cderun/internal/version"

	"github.com/docker/docker/pkg/stdcopy"
)

// Client represents a connection to a Base Host cderun Control Socket.
type Client struct {
	socketPath    string
	conn          net.Conn
	serverVersion string
	mu            sync.Mutex
}

// Connect establishes a connection to the Control Socket at socketPath and performs the handshake.
func Connect(ctx context.Context, socketPath string) (*Client, error) {
	c := &Client{
		socketPath: socketPath,
	}

	conn, err := c.dialAndHandshake(ctx)
	if err != nil {
		return nil, err
	}
	c.conn = conn

	return c, nil
}

func (c *Client) dialAndHandshake(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to control socket %s: %w", c.socketPath, err)
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(5 * time.Second)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to set deadline for control socket handshake: %w", err)
	}

	req := HandshakeRequest{
		ProtocolVersion: CurrentProtocolVersion,
		ClientVersion:   version.Version,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to marshal handshake request: %w", err)
	}

	if err := WriteFrame(conn, reqData); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to send handshake request: %w", err)
	}

	respData, err := ReadFrame(conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}

	_ = conn.SetDeadline(time.Time{})

	var resp HandshakeResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("malformed handshake response from control socket: %w", err)
	}

	if !resp.Accepted {
		_ = conn.Close()
		return nil, fmt.Errorf("control socket handshake rejected: %s (please re-run --mount-cderun-path with a matching binary)", resp.Error)
	}

	if c.serverVersion == "" {
		c.serverVersion = resp.ServerVersion
	}

	return conn, nil
}

// Ping sends a ping frame and waits for a pong response to verify connection responsiveness.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("failed to set deadline for ping: %w", err)
		}
		defer func() {
			_ = c.conn.SetDeadline(time.Time{})
		}()
	}

	reqFrame := RequestFrame{Type: MsgPing}
	data, err := json.Marshal(reqFrame)
	if err != nil {
		return err
	}

	if err := WriteFrame(c.conn, data); err != nil {
		return fmt.Errorf("failed to send ping frame: %w", err)
	}

	respBytes, err := ReadFrame(c.conn)
	if err != nil {
		return fmt.Errorf("failed to read ping response: %w", err)
	}

	var resp ResponseFrame
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return fmt.Errorf("malformed ping response: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("ping failed: %s", resp.Error)
	}
	if !bytes.Equal(resp.Payload, []byte("pong")) {
		return fmt.Errorf("unexpected ping response payload %q, expected %q", string(resp.Payload), "pong")
	}
	return nil
}

// ServerVersion returns the server version string obtained during handshake.
func (c *Client) ServerVersion() string {
	return c.serverVersion
}

// Close closes the underlying network connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// sendRPC sends a RequestFrame to the server and returns the ResponseFrame payload or error.
func (c *Client) sendRPC(ctx context.Context, msgType MessageType, reqPayload []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var deadlinePtr *time.Time
	if deadline, ok := ctx.Deadline(); ok {
		deadlinePtr = &deadline
		if err := c.conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("failed to set deadline for RPC %s: %w", msgType, err)
		}
		defer func() {
			_ = c.conn.SetDeadline(time.Time{})
		}()
	}

	reqFrame := RequestFrame{
		Type:     msgType,
		Deadline: deadlinePtr,
		Payload:  reqPayload,
	}
	data, err := json.Marshal(reqFrame)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s request: %w", msgType, err)
	}

	if err := WriteFrame(c.conn, data); err != nil {
		return nil, fmt.Errorf("failed to send %s frame: %w", msgType, err)
	}

	respBytes, err := ReadFrame(c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s response: %w", msgType, err)
	}

	var resp ResponseFrame
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("malformed %s response frame: %w", msgType, err)
	}

	if !resp.Success {
		return nil, errors.New(resp.Error)
	}

	return resp.Payload, nil
}

// CreateContainer invokes CreateContainer RPC over Control Socket.
func (c *Client) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	args := CreateContainerArgs{Config: config}
	payload, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal CreateContainer args: %w", err)
	}

	resBytes, err := c.sendRPC(ctx, MsgCreateContainer, payload)
	if err != nil {
		return "", err
	}

	var res CreateContainerResult
	if err := json.Unmarshal(resBytes, &res); err != nil {
		return "", fmt.Errorf("malformed CreateContainer result: %w", err)
	}
	return res.ContainerID, nil
}

// StartContainer invokes StartContainer RPC over Control Socket.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	args := ContainerIDArgs{ContainerID: containerID}
	payload, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal StartContainer args: %w", err)
	}

	_, err = c.sendRPC(ctx, MsgStartContainer, payload)
	return err
}

// WaitContainer invokes WaitContainer RPC over Control Socket.
func (c *Client) WaitContainer(ctx context.Context, containerID string) (int, error) {
	args := ContainerIDArgs{ContainerID: containerID}
	payload, err := json.Marshal(args)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal WaitContainer args: %w", err)
	}

	resBytes, err := c.sendRPC(ctx, MsgWaitContainer, payload)
	if err != nil {
		return 0, err
	}

	var res WaitContainerResult
	if err := json.Unmarshal(resBytes, &res); err != nil {
		return 0, fmt.Errorf("malformed WaitContainer result: %w", err)
	}
	return res.ExitCode, nil
}

// RemoveContainer invokes RemoveContainer RPC over Control Socket.
func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	args := ContainerIDArgs{ContainerID: containerID}
	payload, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal RemoveContainer args: %w", err)
	}

	_, err = c.sendRPC(ctx, MsgRemoveContainer, payload)
	return err
}

// AttachContainer attaches to a container's IO streams over Control Socket.
func (c *Client) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	conn, err := c.dialAndHandshake(ctx)
	if err != nil {
		return fmt.Errorf("failed to dial control socket for attach: %w", err)
	}
	defer conn.Close()

	args := AttachContainerArgs{
		ContainerID: containerID,
		TTY:         tty,
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal AttachContainer args: %w", err)
	}

	reqFrame := RequestFrame{
		Type:    MsgAttachContainer,
		Payload: payload,
	}
	reqBytes, err := json.Marshal(reqFrame)
	if err != nil {
		return fmt.Errorf("failed to marshal AttachContainer request frame: %w", err)
	}

	if err := WriteFrame(conn, reqBytes); err != nil {
		return fmt.Errorf("failed to send AttachContainer request frame: %w", err)
	}

	respBytes, err := ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("failed to read AttachContainer response frame: %w", err)
	}

	var resp ResponseFrame
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return fmt.Errorf("malformed AttachContainer response frame: %w", err)
	}

	if !resp.Success {
		return errors.New(resp.Error)
	}

	// Notify caller that attach connection is established
	if ready != nil {
		select {
		case ready <- struct{}{}:
		default:
		}
	}

	// Start stdin forwarding in background if stdin is provided
	if stdin != nil {
		go func() {
			_, _ = io.Copy(conn, stdin)
			if cw, ok := conn.(interface{ CloseWrite() error }); ok {
				_ = cw.CloseWrite()
			}
		}()
	}

	// Demux/copy output streams from server
	var outputErr error
	if tty {
		if stdout != nil {
			_, outputErr = io.Copy(stdout, conn)
		} else {
			_, outputErr = io.Copy(io.Discard, conn)
		}
	} else {
		if stdout == nil {
			stdout = io.Discard
		}
		if stderr == nil {
			stderr = io.Discard
		}
		_, outputErr = stdcopy.StdCopy(stdout, stderr, conn)
	}

	return outputErr
}

// SignalContainer invokes SignalContainer RPC over Control Socket.
func (c *Client) SignalContainer(ctx context.Context, containerID string, sig string) error {
	args := SignalContainerArgs{
		ContainerID: containerID,
		Signal:      sig,
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal SignalContainer args: %w", err)
	}

	_, err = c.sendRPC(ctx, MsgSignalContainer, payload)
	return err
}

// ResizeContainerTTY invokes ResizeContainerTTY RPC over Control Socket.
func (c *Client) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	args := ResizeContainerTTYArgs{
		ContainerID: containerID,
		Rows:        rows,
		Cols:        cols,
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal ResizeContainerTTY args: %w", err)
	}

	_, err = c.sendRPC(ctx, MsgResizeContainerTTY, payload)
	return err
}
