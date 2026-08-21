//go:build linux

package runtime

import (
	"context"
	"fmt"
	"syscall"
	"testing"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFullImage implements client.Image for testing using struct embedding.
type mockFullImage struct {
	client.Image
	name string
	spec ocispec.Image
}

func (m *mockFullImage) Name() string                                     { return m.name }
func (m *mockFullImage) Spec(ctx context.Context) (ocispec.Image, error) { return m.spec, nil }

// mockFullTask implements client.Task for testing using struct embedding.
type mockFullTask struct {
	client.Task
	id          string
	pid         uint32
	startErr    error
	deleteErr   error
	killErr     error
	resizeErr   error
	exitCode    uint32
	exitErr     error
	statusState client.ProcessStatus
	lastSignal  syscall.Signal
	resized     bool
}

func (m *mockFullTask) ID() string        { return m.id }
func (m *mockFullTask) Pid() uint32       { return m.pid }
func (m *mockFullTask) Start(ctx context.Context) error { return m.startErr }
func (m *mockFullTask) Delete(ctx context.Context, opts ...client.ProcessDeleteOpts) (*client.ExitStatus, error) {
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	return client.NewExitStatus(m.exitCode, time.Now(), m.exitErr), nil
}
func (m *mockFullTask) Kill(ctx context.Context, sig syscall.Signal, opts ...client.KillOpts) error {
	m.lastSignal = sig
	return m.killErr
}
func (m *mockFullTask) Wait(ctx context.Context) (<-chan client.ExitStatus, error) {
	ch := make(chan client.ExitStatus, 1)
	ch <- *client.NewExitStatus(m.exitCode, time.Now(), m.exitErr)
	close(ch)
	return ch, nil
}
func (m *mockFullTask) Resize(ctx context.Context, w, h uint32) error {
	m.resized = true
	return m.resizeErr
}
func (m *mockFullTask) Status(ctx context.Context) (client.Status, error) {
	return client.Status{Status: m.statusState, ExitStatus: m.exitCode}, nil
}

// mockFullContainer implements client.Container for testing using struct embedding.
type mockFullContainer struct {
	client.Container
	id         string
	image      client.Image
	task       *mockFullTask
	taskErr    error
	newTaskErr error
	deleteErr  error
}

func (m *mockFullContainer) ID() string { return m.id }
func (m *mockFullContainer) Delete(ctx context.Context, opts ...client.DeleteOpts) error {
	return m.deleteErr
}
func (m *mockFullContainer) NewTask(ctx context.Context, _ cio.Creator, opts ...client.NewTaskOpts) (client.Task, error) {
	if m.newTaskErr != nil {
		return nil, m.newTaskErr
	}
	return m.task, nil
}
func (m *mockFullContainer) Task(ctx context.Context, attach cio.Attach) (client.Task, error) {
	if m.taskErr != nil {
		return nil, m.taskErr
	}
	if m.task == nil {
		return nil, errdefs.ErrNotFound
	}
	return m.task, nil
}
func (m *mockFullContainer) Image(ctx context.Context) (client.Image, error) { return m.image, nil }

// mockFullImageStore implements images.Store for testing using struct embedding.
type mockFullImageStore struct {
	images.Store
	images map[string]images.Image
}

func (s *mockFullImageStore) Get(ctx context.Context, name string) (images.Image, error) {
	if img, ok := s.images[name]; ok {
		return img, nil
	}
	return images.Image{}, errdefs.ErrNotFound
}

// mockFullClient implements containerdClient for testing.
type mockFullClient struct {
	imageStore       *mockFullImageStore
	containers       map[string]*mockFullContainer
	getImageErr      error
	pullErr          error
	newContainerErr  error
	loadContainerErr error
	closed           bool
}

func newMockFullClient() *mockFullClient {
	return &mockFullClient{
		imageStore: &mockFullImageStore{images: make(map[string]images.Image)},
		containers: make(map[string]*mockFullContainer),
	}
}

func (m *mockFullClient) ImageService() images.Store {
	return m.imageStore
}

func (m *mockFullClient) Pull(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error) {
	if m.pullErr != nil {
		return nil, m.pullErr
	}
	m.imageStore.images[ref] = images.Image{Name: ref}
	return &mockFullImage{name: ref}, nil
}

func (m *mockFullClient) GetImage(ctx context.Context, ref string) (client.Image, error) {
	if m.getImageErr != nil {
		return nil, m.getImageErr
	}
	return &mockFullImage{name: ref}, nil
}

func (m *mockFullClient) NewContainer(ctx context.Context, id string, opts ...client.NewContainerOpts) (client.Container, error) {
	if m.newContainerErr != nil {
		return nil, m.newContainerErr
	}
	c := &mockFullContainer{id: id, task: &mockFullTask{id: id, statusState: client.Running}}
	m.containers[id] = c
	return c, nil
}

func (m *mockFullClient) LoadContainer(ctx context.Context, id string) (client.Container, error) {
	if m.loadContainerErr != nil {
		return nil, m.loadContainerErr
	}
	if c, ok := m.containers[id]; ok {
		return c, nil
	}
	return nil, errdefs.ErrNotFound
}

func (m *mockFullClient) Close() error {
	m.closed = true
	return nil
}

func TestUnit_Containerd_MockClient_Lifecycle(t *testing.T) {
	mc := newMockFullClient()
	logger := logging.GetGlobalLogger()
	rt, err := NewContainerdRuntime("/dummy/socket.sock", WithContainerdClient(mc), WithContainerdLogger(logger))
	require.NoError(t, err)
	defer rt.Close()

	ctx := context.Background()

	// 1. PullImage tests
	t.Run("PullImage Success", func(t *testing.T) {
		err := rt.PullImage(ctx, "alpine:latest", "always", 0, 10*time.Millisecond)
		assert.NoError(t, err)
	})

	t.Run("PullImage Missing Exists", func(t *testing.T) {
		err := rt.PullImage(ctx, "alpine:latest", "missing", 0, 10*time.Millisecond)
		assert.NoError(t, err)
	})

	t.Run("PullImage Error", func(t *testing.T) {
		mc.pullErr = fmt.Errorf("network error")
		err := rt.PullImage(ctx, "ubuntu:latest", "always", 0, 10*time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image")
		mc.pullErr = nil
	})

	// 2. CreateContainer tests
	var containerID string
	t.Run("CreateContainer Success", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		}
		id, err := rt.CreateContainer(ctx, cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		containerID = id
	})

	t.Run("CreateContainer GetImage Failure", func(t *testing.T) {
		mc.getImageErr = fmt.Errorf("image not found")
		cfg := &container.ContainerConfig{Image: "alpine:latest"}
		_, err := rt.CreateContainer(ctx, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get image")
		mc.getImageErr = nil
	})

	// 3. StartContainer & InspectContainer tests
	t.Run("StartContainer and InspectContainer", func(t *testing.T) {
		err := rt.StartContainer(ctx, containerID)
		assert.NoError(t, err)

		running, exitCode, err := rt.InspectContainer(ctx, containerID)
		require.NoError(t, err)
		assert.True(t, running)
		assert.Equal(t, 0, exitCode)
	})

	// 4. Signal & Resize TTY tests
	t.Run("SignalContainer and ResizeContainerTTY", func(t *testing.T) {
		err := rt.SignalContainer(ctx, containerID, "SIGTERM")
		assert.NoError(t, err)

		err = rt.ResizeContainerTTY(ctx, containerID, 24, 80)
		assert.NoError(t, err)
	})

	// 5. WaitContainer tests
	t.Run("WaitContainer Success", func(t *testing.T) {
		exitCode, err := rt.WaitContainer(ctx, containerID)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
	})

	// 6. RemoveContainer tests
	t.Run("RemoveContainer Success", func(t *testing.T) {
		err := rt.RemoveContainer(ctx, containerID)
		assert.NoError(t, err)
	})

	t.Run("RemoveContainer Nonexistent Is Ignored", func(t *testing.T) {
		err := rt.RemoveContainer(ctx, "nonexistent-id")
		assert.NoError(t, err)
	})
}

func TestUnit_Containerd_MockClient_ErrorBranches(t *testing.T) {
	mc := newMockFullClient()
	logger := logging.GetGlobalLogger()
	rt, err := NewContainerdRuntime("/dummy/socket.sock", WithContainerdClient(mc), WithContainerdLogger(logger))
	require.NoError(t, err)
	defer rt.Close()

	ctx := context.Background()

	t.Run("CreateContainer NewContainer Failure", func(t *testing.T) {
		mc.newContainerErr = fmt.Errorf("daemon error")
		cfg := &container.ContainerConfig{Image: "alpine:latest"}
		_, err := rt.CreateContainer(ctx, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create")
		mc.newContainerErr = nil
	})

	t.Run("StartContainer LoadContainer Failure", func(t *testing.T) {
		mc.loadContainerErr = fmt.Errorf("load failed")
		err := rt.StartContainer(ctx, "c1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load failed")
		mc.loadContainerErr = nil
	})

	t.Run("WaitContainer LoadContainer Failure", func(t *testing.T) {
		mc.loadContainerErr = fmt.Errorf("load failed")
		_, err := rt.WaitContainer(ctx, "c1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load failed")
		mc.loadContainerErr = nil
	})

	t.Run("InspectContainer LoadContainer Failure", func(t *testing.T) {
		mc.loadContainerErr = fmt.Errorf("load failed")
		_, _, err := rt.InspectContainer(ctx, "c1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load failed")
		mc.loadContainerErr = nil
	})

	t.Run("SignalContainer Invalid Format", func(t *testing.T) {
		err := rt.SignalContainer(ctx, "c1", "INVALID_SIG;-rm")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signal")
	})
}
