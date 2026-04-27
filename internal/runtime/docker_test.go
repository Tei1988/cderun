package runtime

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var noopSleepFunc = func(ctx context.Context, d time.Duration) error { return nil }

func TestUnit_Docker_New(t *testing.T) {
	runtime, err := NewDockerRuntime("/var/run/docker.sock")
	require.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, "docker", runtime.Name())
}

func TestUnit_Docker_PullImage_Retry(t *testing.T) {
	t.Run("retry on EOF", func(t *testing.T) {
		mock := &mockDockerClient{
			imagePullErr: errors.New("EOF"),
		}

		var sleeps []time.Duration
		runtime := &DockerRuntime{
			client: mock,
			name:   "test",
			sleepFunc: func(ctx context.Context, d time.Duration) error {
				sleeps = append(sleeps, d)
				return nil
			},
		}

		err := runtime.PullImage(context.Background(), "alpine", "always", 3, 1*time.Second)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image after 4 attempts")
		assert.Equal(t, 4, mock.pullCount)
		assert.Len(t, sleeps, 3)
	})
}

type mockDockerClient struct {
	closeErr         error
	closeCalled      bool
	closeCount       int
	imageInspectErr  error
	imageInspectFunc func(ctx context.Context, imageID string, options ...client.ImageInspectOption) (image.InspectResponse, error)
	inspectCount     int
	imagePullErr     error
	imagePullFunc    func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	pullCount        int
	pullReader       io.ReadCloser

	createResp       dockercontainer.CreateResponse
	createErr        error
	startErr         error
	waitResp         dockercontainer.WaitResponse
	waitErrOut       error
	removeErr        error
	resizeErr        error
	killErr          error
	attachResp       types.HijackedResponse
	attachErr        error
	inspectResp      dockercontainer.InspectResponse
	inspectErr       error
}

func (m *mockDockerClient) Close() error {
	m.closeCalled = true
	m.closeCount++
	return m.closeErr
}

func (m *mockDockerClient) ImageInspect(ctx context.Context, imageID string, options ...client.ImageInspectOption) (image.InspectResponse, error) {
	m.inspectCount++
	if m.imageInspectFunc != nil {
		return m.imageInspectFunc(ctx, imageID, options...)
	}
	return image.InspectResponse{}, m.imageInspectErr
}

func (m *mockDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	m.pullCount++
	if m.imagePullFunc != nil {
		return m.imagePullFunc(ctx, ref, options)
	}
	if m.imagePullErr != nil {
		return nil, m.imagePullErr
	}
	if m.pullReader != nil {
		return m.pullReader, nil
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (dockercontainer.CreateResponse, error) {
	return m.createResp, m.createErr
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options dockercontainer.StartOptions) error {
	return m.startErr
}

func (m *mockDockerClient) ContainerWait(ctx context.Context, containerID string, condition dockercontainer.WaitCondition) (<-chan dockercontainer.WaitResponse, <-chan error) {
	resC := make(chan dockercontainer.WaitResponse, 1)
	errC := make(chan error, 1)
	if m.waitErrOut != nil {
		errC <- m.waitErrOut
	} else {
		resC <- m.waitResp
	}
	return resC, errC
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options dockercontainer.RemoveOptions) error {
	return m.removeErr
}

func (m *mockDockerClient) ContainerResize(ctx context.Context, containerID string, options dockercontainer.ResizeOptions) error {
	return m.resizeErr
}

func (m *mockDockerClient) ContainerKill(ctx context.Context, containerID string, signal string) error {
	return m.killErr
}

func (m *mockDockerClient) ContainerAttach(ctx context.Context, container string, options dockercontainer.AttachOptions) (types.HijackedResponse, error) {
	return m.attachResp, m.attachErr
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (dockercontainer.InspectResponse, error) {
	if m.inspectErr != nil {
		return dockercontainer.InspectResponse{}, m.inspectErr
	}
	return m.inspectResp, nil
}

func TestUnit_Docker_InspectContainer(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		mock := &mockDockerClient{
			inspectResp: dockercontainer.InspectResponse{
				ContainerJSONBase: &dockercontainer.ContainerJSONBase{
					State: &dockercontainer.State{
						Running:  true,
						ExitCode: 0,
					},
				},
			},
		}
		runtime := &DockerRuntime{client: mock, sleepFunc: noopSleepFunc}
		running, code, err := runtime.InspectContainer(ctx, "id")
		require.NoError(t, err)
		assert.True(t, running)
		assert.Equal(t, 0, code)
	})
}

func TestUnit_Docker_RemoveContainer_Suppression(t *testing.T) {
	ctx := context.Background()
	t.Run("suppress not found", func(t *testing.T) {
		mock := &mockDockerClient{removeErr: errdefs.ErrNotFound}
		runtime := &DockerRuntime{client: mock, sleepFunc: noopSleepFunc}
		err := runtime.RemoveContainer(ctx, "id")
		require.NoError(t, err)
	})
}

func TestUnit_Docker_Attach_Errors(t *testing.T) {
	t.Run("attach error with ready channel", func(t *testing.T) {
		mock := &mockDockerClient{attachErr: errors.New("attach failed")}
		runtime := &DockerRuntime{client: mock, sleepFunc: noopSleepFunc}
		ready := make(chan struct{})
		err := runtime.AttachContainer(context.Background(), "id", false, nil, nil, nil, ready)
		require.Error(t, err)
		_, ok := <-ready
		assert.False(t, ok)
	})
}

func TestUnit_Docker_PullImage(t *testing.T) {
	t.Run("missing policy - retry inspect not found", func(t *testing.T) {
		mock := &mockDockerClient{}
		count := 0
		mock.imageInspectFunc = func(ctx context.Context, imageID string, options ...client.ImageInspectOption) (image.InspectResponse, error) {
			count++
			if count == 1 {
				return image.InspectResponse{}, errors.New("eof")
			}
			return image.InspectResponse{}, errdefs.ErrNotFound
		}
		runtime := &DockerRuntime{client: mock, sleepFunc: noopSleepFunc}
		err := runtime.PullImage(context.Background(), "test", "missing", 3, 1*time.Second)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		assert.Equal(t, 1, mock.pullCount)
	})

	t.Run("retry on stream error", func(t *testing.T) {
		mock := &mockDockerClient{}
		count := 0
		mock.imagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
			count++
			if count == 1 {
				return nil, errors.New("connection reset")
			}
			return io.NopCloser(strings.NewReader("")), nil
		}
		runtime := &DockerRuntime{client: mock, sleepFunc: noopSleepFunc}
		err := runtime.PullImage(context.Background(), "test", "always", 3, 1*time.Second)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}
