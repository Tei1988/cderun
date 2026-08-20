package controlsocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"cderun/internal/version"
)

// Client represents a connection to a Base Host cderun Control Socket.
type Client struct {
	socketPath    string
	conn          net.Conn
	serverVersion string
}

// Connect establishes a connection to the Control Socket at socketPath and performs the handshake.
func Connect(ctx context.Context, socketPath string) (*Client, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to control socket %s: %w", socketPath, err)
	}

	c := &Client{
		socketPath: socketPath,
		conn:       conn,
	}

	if err := c.handshake(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return c, nil
}

func (c *Client) handshake(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(5 * time.Second)
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set deadline for control socket handshake: %w", err)
	}
	defer func() {
		_ = c.conn.SetDeadline(time.Time{})
	}()

	req := HandshakeRequest{
		ProtocolVersion: CurrentProtocolVersion,
		ClientVersion:   version.Version,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal handshake request: %w", err)
	}

	if err := WriteFrame(c.conn, reqData); err != nil {
		return fmt.Errorf("failed to send handshake request: %w", err)
	}

	respData, err := ReadFrame(c.conn)
	if err != nil {
		return fmt.Errorf("failed to read handshake response: %w", err)
	}

	var resp HandshakeResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("malformed handshake response from control socket: %w", err)
	}

	if !resp.Accepted {
		return fmt.Errorf("control socket handshake rejected: %s (please re-run --mount-cderun-path with a matching binary)", resp.Error)
	}

	c.serverVersion = resp.ServerVersion
	return nil
}

// Ping sends a ping frame and waits for a pong response to verify connection responsiveness.
func (c *Client) Ping(ctx context.Context) error {
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
