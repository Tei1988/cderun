package runtime

import (
	"cderun/internal/container"
	"context"
	"io"
	"sync"
)

// MockRuntime is a mock implementation of ContainerRuntime for testing purposes.
type MockRuntime struct {
	mu                  sync.RWMutex
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PulledImage = image
	m.PullPolicy = pullPolicy
	return m.PullErr
}

func (m *MockRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreatedConfig = config
	return m.CreatedContainerID, m.CreateErr
}

func (m *MockRuntime) StartContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StartedContainerID = containerID
	return m.StartErr
}

func (m *MockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.WaitedContainerID = containerID
	return m.ExitCode, m.WaitErr
}

func (m *MockRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RemovedContainerID = containerID
	return m.RemoveErr
}

func (m *MockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AttachedContainerID = containerID
	return m.AttachErr
}

func (m *MockRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ResizedContainerID = containerID
	m.Rows = rows
	m.Cols = cols
	return m.ResizeErr
}

func (m *MockRuntime) GetTTYSize() (uint, uint) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Rows, m.Cols
}

func (m *MockRuntime) GetPulledImage() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.PulledImage
}

func (m *MockRuntime) GetCreatedConfig() *container.ContainerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.CreatedConfig
}

func (m *MockRuntime) GetStartedContainerID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.StartedContainerID
}

func (m *MockRuntime) GetWaitedContainerID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.WaitedContainerID
}

func (m *MockRuntime) GetRemovedContainerID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.RemovedContainerID
}

func (m *MockRuntime) GetAttachedContainerID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.AttachedContainerID
}

func (m *MockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SignaledContainerID = containerID
	m.Signal = sig
	return m.SignalErr
}

func (m *MockRuntime) Name() string {
	return "mock"
}
