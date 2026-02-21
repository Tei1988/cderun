package runtime

import (
	"context"
	"io"
	"sync"

	"cderun/internal/container"
)

// NewMockRuntime creates a new MockRuntime.
func NewMockRuntime() *MockRuntime {
	return &MockRuntime{}
}

// MockRuntime is a mock implementation of ContainerRuntime for testing purposes.
type MockRuntime struct {
	Mu sync.RWMutex
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
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.PulledImage = image
	m.PullPolicy = pullPolicy
	return m.PullErr
}

func (m *MockRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.CreatedConfig = config
	return m.CreatedContainerID, m.CreateErr
}

func (m *MockRuntime) StartContainer(ctx context.Context, containerID string) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.StartedContainerID = containerID
	return m.StartErr
}

func (m *MockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.WaitedContainerID = containerID
	return m.ExitCode, m.WaitErr
}

func (m *MockRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.RemovedContainerID = containerID
	return m.RemoveErr
}

func (m *MockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.AttachedContainerID = containerID
	return m.AttachErr
}

func (m *MockRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.ResizedContainerID = containerID
	m.Rows = rows
	m.Cols = cols
	return m.ResizeErr
}

func (m *MockRuntime) GetTTYSize() (uint, uint) {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	return m.Rows, m.Cols
}

func (m *MockRuntime) GetPulledImage() string {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	return m.PulledImage
}

func (m *MockRuntime) GetCreatedConfig() *container.ContainerConfig {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	return m.CreatedConfig
}

func (m *MockRuntime) GetStartedContainerID() string {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	return m.StartedContainerID
}

func (m *MockRuntime) GetWaitedContainerID() string {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	return m.WaitedContainerID
}

func (m *MockRuntime) GetRemovedContainerID() string {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	return m.RemovedContainerID
}

func (m *MockRuntime) GetAttachedContainerID() string {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	return m.AttachedContainerID
}

func (m *MockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.SignaledContainerID = containerID
	m.Signal = sig
	return m.SignalErr
}

func (m *MockRuntime) Name() string {
	return "mock"
}
