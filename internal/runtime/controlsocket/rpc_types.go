package controlsocket

import "cderun/internal/container"

// MessageType RPC action definitions for container lifecycle dispatch.
const (
	MsgCreateContainer   MessageType = "CreateContainer"
	MsgStartContainer    MessageType = "StartContainer"
	MsgWaitContainer     MessageType = "WaitContainer"
	MsgRemoveContainer   MessageType = "RemoveContainer"
	MsgAttachContainer   MessageType = "AttachContainer"
	MsgSignalContainer   MessageType = "SignalContainer"
	MsgResizeContainerTTY MessageType = "ResizeContainerTTY"
)

// CreateContainerArgs holds payload for CreateContainer RPC request.
type CreateContainerArgs struct {
	Config *container.ContainerConfig `json:"config"`
}

// CreateContainerResult holds response payload for CreateContainer RPC request.
type CreateContainerResult struct {
	ContainerID string `json:"containerId"`
}

// ContainerIDArgs holds container ID for StartContainer, WaitContainer, RemoveContainer RPC requests.
type ContainerIDArgs struct {
	ContainerID string `json:"containerId"`
}

// WaitContainerResult holds response payload for WaitContainer RPC request.
type WaitContainerResult struct {
	ExitCode int `json:"exitCode"`
}

// AttachContainerArgs holds payload for AttachContainer RPC request.
type AttachContainerArgs struct {
	ContainerID string `json:"containerId"`
	TTY         bool   `json:"tty"`
}

// SignalContainerArgs holds payload for SignalContainer RPC request.
type SignalContainerArgs struct {
	ContainerID string `json:"containerId"`
	Signal      string `json:"signal"`
}

// ResizeContainerTTYArgs holds payload for ResizeContainerTTY RPC request.
type ResizeContainerTTYArgs struct {
	ContainerID string `json:"containerId"`
	Rows        uint   `json:"rows"`
	Cols        uint   `json:"cols"`
}
