package controlsocket

import (
	"encoding/binary"
	"fmt"
	"io"
)

// CurrentProtocolVersion defines the Control Socket protocol version.
const CurrentProtocolVersion uint32 = 1

// MaxFrameSize limits maximum frame payload size (16MB) to prevent memory exhaustion.
const MaxFrameSize uint32 = 16 * 1024 * 1024

// HandshakeRequest is sent by the nested client upon connection.
type HandshakeRequest struct {
	ProtocolVersion uint32 `json:"protocolVersion"`
	ClientVersion   string `json:"clientVersion"`
}

// HandshakeResponse is returned by the Base Host server after processing the handshake request.
type HandshakeResponse struct {
	Accepted        bool   `json:"accepted"`
	ProtocolVersion uint32 `json:"protocolVersion"`
	ServerVersion   string `json:"serverVersion"`
	Error           string `json:"error,omitempty"`
}

// MessageType indicates the type of control protocol payload.
type MessageType string

const (
	MsgPing MessageType = "ping"
	MsgPong MessageType = "pong"
)

// RequestFrame represents a generic request payload frame.
type RequestFrame struct {
	Type    MessageType `json:"type"`
	Payload []byte      `json:"payload,omitempty"`
}

// ResponseFrame represents a generic response payload frame.
type ResponseFrame struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Payload []byte `json:"payload,omitempty"`
}

// ReadFrame reads a 4-byte big-endian length-prefixed payload from r.
func ReadFrame(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length > MaxFrameSize {
		return nil, fmt.Errorf("frame length %d exceeds maximum allowed size %d", length, MaxFrameSize)
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("failed to read complete frame payload of size %d: %w", length, err)
	}
	return buf, nil
}

// WriteFrame writes a payload prefixed with a 4-byte big-endian length header to w.
func WriteFrame(w io.Writer, payload []byte) error {
	length := uint32(len(payload))
	if length > MaxFrameSize {
		return fmt.Errorf("payload length %d exceeds maximum allowed size %d", length, MaxFrameSize)
	}

	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("failed to write frame length header: %w", err)
	}
	if length > 0 {
		if _, err := w.Write(payload); err != nil {
			return fmt.Errorf("failed to write frame payload: %w", err)
		}
	}
	return nil
}
