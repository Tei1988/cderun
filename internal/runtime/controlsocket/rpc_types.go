package controlsocket

import "cderun/internal/container"

// MessageType RPC action definitions for container lifecycle dispatch.
const (
	MsgCreateContainer MessageType = "CreateContainer"
	MsgStartContainer  MessageType = "StartContainer"
	MsgWaitContainer   MessageType = "WaitContainer"
	MsgRemoveContainer MessageType = "RemoveContainer"
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
