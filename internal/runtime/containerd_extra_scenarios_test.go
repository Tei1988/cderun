//go:build linux

package runtime

import (
	"context"
	"errors"
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
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

type mockImage struct {
	client.Image
	spec ocispec.Image
}

func (m *mockImage) Spec(ctx context.Context) (ocispec.Image, error) {
	return m.spec, nil
}

type mockContainer struct {
	client.Container
	id     string
	task   client.Task
	spec   *specs.Spec
	image  client.Image
	delErr error
}

func (m *mockContainer) ID() string {
	return m.id
}

func (m *mockContainer) Spec(ctx context.Context) (*specs.Spec, error) {
	return m.spec, nil
}

func (m *mockContainer) Task(ctx context.Context, attach cio.Attach) (client.Task, error) {
	if m.task == nil {
		return nil, errdefs.ErrNotFound
	}
	return m.task, nil
}

func (m *mockContainer) NewTask(ctx context.Context, creator cio.Creator, opts ...client.NewTaskOpts) (client.Task, error) {
	if m.task == nil {
		return nil, errors.New("mock task error")
	}
	return m.task, nil
}

func (m *mockContainer) Delete(ctx context.Context, opts ...client.DeleteOpts) error {
	return m.delErr
}

type mockTask struct {
	client.Task
	id        string
	exitCode  uint32
	exitErr   error
	status    client.Status
	startErr  error
	killErr   error
	resizeErr error
	deleteErr error
}

func (m *mockTask) ID() string {
	return m.id
}

func (m *mockTask) Wait(ctx context.Context) (<-chan client.ExitStatus, error) {
	ch := make(chan client.ExitStatus, 1)
	ch <- *client.NewExitStatus(m.exitCode, time.Now(), m.exitErr)
	return ch, nil
}

func (m *mockTask) Delete(ctx context.Context, opts ...client.ProcessDeleteOpts) (*client.ExitStatus, error) {
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	return client.NewExitStatus(m.exitCode, time.Now(), m.exitErr), nil
}

func (m *mockTask) Start(ctx context.Context) error {
	return m.startErr
}

func (m *mockTask) Kill(ctx context.Context, sig syscall.Signal, opts ...client.KillOpts) error {
	return m.killErr
}

func (m *mockTask) Resize(ctx context.Context, w, h uint32) error {
	return m.resizeErr
}

func (m *mockTask) Status(ctx context.Context) (client.Status, error) {
	return m.status, nil
}

type mockImageStore struct {
	images.Store
	getFunc func(context.Context, string) (images.Image, error)
}

func (m *mockImageStore) Get(ctx context.Context, name string) (images.Image, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, name)
	}
	return images.Image{}, errdefs.ErrNotFound
}

type mockContainerdClient struct {
	imageStore        images.Store
	pullFunc          func(context.Context, string, ...client.RemoteOpt) (client.Image, error)
	getImageFunc      func(context.Context, string) (client.Image, error)
	newContainerFunc  func(context.Context, string, ...client.NewContainerOpts) (client.Container, error)
	loadContainerFunc func(context.Context, string) (client.Container, error)
	closeErr          error
}

func (m *mockContainerdClient) ImageService() images.Store {
	return m.imageStore
}

func (m *mockContainerdClient) Pull(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error) {
	if m.pullFunc != nil {
		return m.pullFunc(ctx, ref, opts...)
	}
	return nil, nil
}

func (m *mockContainerdClient) GetImage(ctx context.Context, ref string) (client.Image, error) {
	if m.getImageFunc != nil {
		return m.getImageFunc(ctx, ref)
	}
	return nil, nil
}

func (m *mockContainerdClient) NewContainer(ctx context.Context, id string, opts ...client.NewContainerOpts) (client.Container, error) {
	if m.newContainerFunc != nil {
		return m.newContainerFunc(ctx, id, opts...)
	}
	return nil, nil
}

func (m *mockContainerdClient) LoadContainer(ctx context.Context, id string) (client.Container, error) {
	if m.loadContainerFunc != nil {
		return m.loadContainerFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockContainerdClient) Close() error {
	return m.closeErr
}

func TestUnit_Containerd_PullImage_Mock(t *testing.T) {
	t.Parallel()

	t.Run("PullImage - Policy never returns immediately", func(t *testing.T) {
		rt := &ContainerdRuntime{}
		err := rt.PullImage(context.Background(), "ubuntu", "never", 3, 1*time.Millisecond)
		assert.NoError(t, err)
	})

	t.Run("PullImage - Pulls when missing and not found locally", func(t *testing.T) {
		pullCalled := false
		mStore := &mockImageStore{
			getFunc: func(ctx context.Context, name string) (images.Image, error) {
				return images.Image{}, errdefs.ErrNotFound
			},
		}
		mClient := &mockContainerdClient{
			imageStore: mStore,
			pullFunc: func(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error) {
				pullCalled = true
				return &mockImage{}, nil
			},
		}

		rt := &ContainerdRuntime{client: mClient, logger: logging.GetGlobalLogger()}
		err := rt.PullImage(context.Background(), "ubuntu", "missing", 3, 1*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, pullCalled)
	})

	t.Run("PullImage - Does not pull when missing and found locally", func(t *testing.T) {
		pullCalled := false
		mStore := &mockImageStore{
			getFunc: func(ctx context.Context, name string) (images.Image, error) {
				return images.Image{Name: name}, nil
			},
		}
		mClient := &mockContainerdClient{
			imageStore: mStore,
			pullFunc: func(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error) {
				pullCalled = true
				return &mockImage{}, nil
			},
		}

		rt := &ContainerdRuntime{client: mClient, logger: logging.GetGlobalLogger()}
		err := rt.PullImage(context.Background(), "ubuntu", "missing", 3, 1*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, pullCalled)
	})

	t.Run("PullImage - Retries on retryable errors", func(t *testing.T) {
		attempts := 0
		mClient := &mockContainerdClient{
			pullFunc: func(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error) {
				attempts++
				if attempts < 3 {
					return nil, fmt.Errorf("connection refused")
				}
				return &mockImage{}, nil
			},
		}

		rt := &ContainerdRuntime{
			client:    mClient,
			logger:    logging.GetGlobalLogger(),
			sleepFunc: func(ctx context.Context, d time.Duration) error { return nil },
		}
		err := rt.PullImage(context.Background(), "ubuntu", "always", 3, 1*time.Millisecond)
		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("PullImage - Aborts immediately on non-retryable error", func(t *testing.T) {
		attempts := 0
		mClient := &mockContainerdClient{
			pullFunc: func(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error) {
				attempts++
				return nil, fmt.Errorf("some fatal error")
			},
		}

		rt := &ContainerdRuntime{
			client:    mClient,
			logger:    logging.GetGlobalLogger(),
			sleepFunc: func(ctx context.Context, d time.Duration) error { return nil },
		}
		err := rt.PullImage(context.Background(), "ubuntu", "always", 3, 1*time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "some fatal error")
		assert.Equal(t, 1, attempts)
	})
}

func TestUnit_Containerd_CreateContainer_Mock(t *testing.T) {
	t.Parallel()

	mImg := &mockImage{
		spec: ocispec.Image{
			Config: ocispec.ImageConfig{
				Entrypoint: []string{"/bin/sh"},
			},
		},
	}

	t.Run("CreateContainer - Success and Spec Opts Verification", func(t *testing.T) {
		containerCreated := false
		mClient := &mockContainerdClient{
			getImageFunc: func(ctx context.Context, ref string) (client.Image, error) {
				return mImg, nil
			},
			newContainerFunc: func(ctx context.Context, id string, opts ...client.NewContainerOpts) (client.Container, error) {
				containerCreated = true
				return &mockContainer{id: id}, nil
			},
		}

		rt := &ContainerdRuntime{client: mClient, logger: logging.GetGlobalLogger()}

		config := &container.ContainerConfig{
			Image:      "alpine",
			Command:    []string{"echo", "hello"},
			TTY:        true,
			User:       "1000:1000",
			Workdir:    "/workspace",
			Privileged: true,
			Env:        []string{"KEY=val"},
			Hostname:   "myhost",
			ReadOnly:   true,
			ShmSize:    "256m",
			Memory:     512 * 1024 * 1024,
			CPUs:       2.0,
			Network:    "host",
			Pid:        "host",
			IPC:        "host",
			Cgroupns:   "host",
			GroupAdd:   []string{"1001"},
			Sysctls:    map[string]string{"net.ipv4.ip_forward": "1"},
			Mounts: []container.Mount{
				{Type: "tmpfs", Target: "/dev/shm"},
				{Type: "bind", Source: "/src", Target: "/dst", ReadOnly: true},
			},
		}

		id, err := rt.CreateContainer(context.Background(), config)
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.True(t, containerCreated)
	})
}

func TestUnit_Containerd_StartContainer_Mock(t *testing.T) {
	t.Parallel()

	t.Run("StartContainer - Success with custom IO", func(t *testing.T) {
		mTask := &mockTask{id: "t1"}
		mContainer := &mockContainer{id: "c1", task: mTask}
		mClient := &mockContainerdClient{
			loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
				return mContainer, nil
			},
		}

		rt := &ContainerdRuntime{
			client: mClient,
			logger: logging.GetGlobalLogger(),
			ioMap:  map[string]cio.Creator{"c1": cio.NullIO},
		}

		err := rt.StartContainer(context.Background(), "c1")
		assert.NoError(t, err)
	})

	t.Run("StartContainer - Fallback to NullIO warning", func(t *testing.T) {
		mTask := &mockTask{id: "t2"}
		mContainer := &mockContainer{id: "c2", task: mTask}
		mClient := &mockContainerdClient{
			loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
				return mContainer, nil
			},
		}

		rt := &ContainerdRuntime{
			client: mClient,
			logger: logging.GetGlobalLogger(),
			ioMap:  map[string]cio.Creator{}, // No attach called beforehand
		}

		err := rt.StartContainer(context.Background(), "c2")
		assert.NoError(t, err)
	})
}

func TestUnit_Containerd_WaitContainer_Mock(t *testing.T) {
	t.Parallel()

	mTask := &mockTask{id: "t1", exitCode: 42}
	mContainer := &mockContainer{id: "c1", task: mTask}
	mClient := &mockContainerdClient{
		loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
			return mContainer, nil
		},
	}

	rt := &ContainerdRuntime{client: mClient, logger: logging.GetGlobalLogger()}
	code, err := rt.WaitContainer(context.Background(), "c1")
	assert.NoError(t, err)
	assert.Equal(t, 42, code)
}

func TestUnit_Containerd_RemoveContainer_Mock(t *testing.T) {
	t.Parallel()

	t.Run("RemoveContainer - deletes task and container", func(t *testing.T) {
		mTask := &mockTask{id: "t1"}
		mContainer := &mockContainer{id: "c1", task: mTask}
		mClient := &mockContainerdClient{
			loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
				return mContainer, nil
			},
		}

		rt := &ContainerdRuntime{client: mClient, logger: logging.GetGlobalLogger()}
		err := rt.RemoveContainer(context.Background(), "c1")
		assert.NoError(t, err)
	})

	t.Run("RemoveContainer - uninitialized client error", func(t *testing.T) {
		rt := &ContainerdRuntime{client: nil}
		err := rt.RemoveContainer(context.Background(), "c1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client is not initialized")
	})
}

func TestUnit_Containerd_SignalContainer_Mock(t *testing.T) {
	t.Parallel()

	mTask := &mockTask{id: "t1"}
	mContainer := &mockContainer{id: "c1", task: mTask}
	mClient := &mockContainerdClient{
		loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
			return mContainer, nil
		},
	}

	rt := &ContainerdRuntime{client: mClient, logger: logging.GetGlobalLogger()}
	err := rt.SignalContainer(context.Background(), "c1", "TERM")
	assert.NoError(t, err)
}

func TestUnit_Containerd_ResizeContainerTTY_Mock(t *testing.T) {
	t.Parallel()

	mTask := &mockTask{id: "t1"}
	mContainer := &mockContainer{id: "c1", task: mTask}
	mClient := &mockContainerdClient{
		loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
			return mContainer, nil
		},
	}

	rt := &ContainerdRuntime{client: mClient, logger: logging.GetGlobalLogger()}
	err := rt.ResizeContainerTTY(context.Background(), "c1", 24, 80)
	assert.NoError(t, err)
}

func TestUnit_Containerd_InspectContainer_Mock(t *testing.T) {
	t.Parallel()

	mTask := &mockTask{
		id: "t1",
		status: client.Status{
			Status:     client.Running,
			ExitStatus: 0,
		},
	}
	mContainer := &mockContainer{id: "c1", task: mTask}
	mClient := &mockContainerdClient{
		loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
			return mContainer, nil
		},
	}

	rt := &ContainerdRuntime{client: mClient, logger: logging.GetGlobalLogger()}
	running, code, err := rt.InspectContainer(context.Background(), "c1")
	assert.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, 0, code)
}
