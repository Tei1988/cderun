package runtime

import (
	"cderun/internal/container"
	"context"
	"io"
)

// MockRuntime is a mock implementation of ContainerRuntime for testing purposes.
type MockRuntime struct {
	CreatedContainerID string
	CreatedConfig      *container.ContainerConfig
	StartedContainerID string
	WaitedContainerID  string
	RemovedContainerID string
	AttachedContainerID string
	ResizedContainerID  string
	SignaledContainerID string
	Rows, Cols          uint
	Signal              string
	ExitCode           int
}

func (m *MockRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	m.CreatedConfig = config
	return m.CreatedContainerID, nil
}

func (m *MockRuntime) StartContainer(ctx context.Context, containerID string) error {
	m.StartedContainerID = containerID
	return nil
}

func (m *MockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	m.WaitedContainerID = containerID
	return m.ExitCode, nil
}

func (m *MockRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	m.RemovedContainerID = containerID
	return nil
}

func (m *MockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	m.AttachedContainerID = containerID
	return nil
}

func (m *MockRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.ResizedContainerID = containerID
	m.Rows = rows
	m.Cols = cols
	return nil
}

func (m *MockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.SignaledContainerID = containerID
	m.Signal = sig
	return nil
}

func (m *MockRuntime) Name() string {
	return "mock"
}
