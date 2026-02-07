package runtime

import (
	"cderun/internal/container"
	"context"
	"io"
)

// MockRuntime is a mock implementation of ContainerRuntime for testing purposes.
type MockRuntime struct {
	PulledImage         string
	PullPolicy          string
	CreatedContainerID  string
	CreatedConfig       *container.ContainerConfig
	StartedContainerID  string
	WaitedContainerID   string
	RemovedContainerID  string
	AttachedContainerID string
	ResizedContainerID  string
	SignaledContainerID string
	Rows, Cols          uint
	Signal              string
	ExitCode            int
	PullErr             error
	CreateErr           error
	StartErr            error
	WaitErr             error
	RemoveErr           error
	AttachErr           error
	ResizeErr           error
	SignalErr           error
}

func (m *MockRuntime) PullImage(ctx context.Context, image string, pullPolicy string) error {
	m.PulledImage = image
	m.PullPolicy = pullPolicy
	return m.PullErr
}

func (m *MockRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	m.CreatedConfig = config
	return m.CreatedContainerID, m.CreateErr
}

func (m *MockRuntime) StartContainer(ctx context.Context, containerID string) error {
	m.StartedContainerID = containerID
	return m.StartErr
}

func (m *MockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	m.WaitedContainerID = containerID
	return m.ExitCode, m.WaitErr
}

func (m *MockRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	m.RemovedContainerID = containerID
	return m.RemoveErr
}

func (m *MockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	m.AttachedContainerID = containerID
	if m.AttachErr == nil && ready != nil {
		close(ready)
	}
	return m.AttachErr
}

func (m *MockRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.ResizedContainerID = containerID
	m.Rows = rows
	m.Cols = cols
	return m.ResizeErr
}

func (m *MockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.SignaledContainerID = containerID
	m.Signal = sig
	return m.SignalErr
}

func (m *MockRuntime) Name() string {
	return "mock"
}
