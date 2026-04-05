package runtime

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"cderun/internal/container"
)

// NewMockRuntime creates a new MockRuntime.
func NewMockRuntime() *MockRuntime {
	return &MockRuntime{sigChan: make(chan string, 1)}
}

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
	sigChan             chan string
	ExitCode            int
	WaitDelay           time.Duration
	PullErr             error
	CreateErr           error
	StartErr            error
	WaitErr             error
	RemoveErr           error
	AttachErr           error
	ResizeErr           error
	SignalErr           error

	// Custom behavior hooks
	WaitFunc    func(ctx context.Context, containerID string) (int, error)
	AttachFunc  func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error
	InspectFunc func(ctx context.Context, containerID string) (bool, int, error)
}

// WithLockedMock executes the provided function while holding the mock's mutex.
// This is intended for test-only use.
func (m *MockRuntime) WithLockedMock(f func(m *MockRuntime)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f(m)
}

func (m *MockRuntime) PullImage(ctx context.Context, image string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PulledImage = image
	m.PullPolicy = pullPolicy
	_ = maxRetries
	_ = backoffBase
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
	m.WaitedContainerID = containerID
	f := m.WaitFunc
	delay := m.WaitDelay
	m.mu.Unlock()

	if f != nil {
		return f(ctx, containerID)
	}

	if delay > 0 {
		t := time.NewTimer(delay)
		select {
		case <-m.sigChan:
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
		case <-t.C:
		case <-ctx.Done():
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			return 0, ctx.Err()
		}
	} else {
		select {
		case <-m.sigChan:
		default:
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ExitCode, m.WaitErr
}

func (m *MockRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RemovedContainerID = containerID
	return m.RemoveErr
}

func (m *MockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	m.mu.Lock()
	m.AttachedContainerID = containerID
	f := m.AttachFunc
	err := m.AttachErr
	m.mu.Unlock()

	if f != nil {
		return f(ctx, containerID, tty, stdin, stdout, stderr, ready)
	}

	if ready != nil {
		close(ready)
	}
	return err
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

func (m *MockRuntime) GetExitCode() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ExitCode
}

func (m *MockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	if sig != "" && !signalRegex.MatchString(sig) {
		return fmt.Errorf("invalid signal: %s", sig)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SignaledContainerID = containerID
	m.Signal = sig
	select {
	case m.sigChan <- sig:
	default:
	}
	return m.SignalErr
}

func (m *MockRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	m.mu.RLock()
	f := m.InspectFunc
	exitCode := m.ExitCode
	m.mu.RUnlock()

	if f != nil {
		return f(ctx, containerID)
	}
	return false, exitCode, nil
}

func (m *MockRuntime) Name() string {
	return "mock"
}

// ResetCreatedConfig resets the created config to nil.
func (m *MockRuntime) ResetCreatedConfig() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreatedConfig = nil
}
