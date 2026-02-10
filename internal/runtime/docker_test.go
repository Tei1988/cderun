package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"cderun/internal/container"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

func TestUnit_Docker_New(t *testing.T) {
	// This should succeed even without docker daemon as it just creates the client
	runtime, err := NewDockerRuntime("/var/run/docker.sock")
	assert.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, "docker", runtime.Name())
}

type mockDockerClient struct {
	imageInspectErr error
	imagePullErr    error
	pullCount       int

	createConfig     *dockercontainer.Config
	createHostConfig *dockercontainer.HostConfig
	createResp       dockercontainer.CreateResponse
	createErr        error

	startID  string
	startErr error

	waitID     string
	waitResp   dockercontainer.WaitResponse
	waitErrOut error

	removeID  string
	removeErr error

	resizeID   string
	resizeOpts dockercontainer.ResizeOptions
	resizeErr  error

	killID     string
	killSignal string
	killErr    error

	attachID   string
	attachResp types.HijackedResponse
	attachErr  error
}

func (m *mockDockerClient) ImageInspect(ctx context.Context, imageID string, options ...client.ImageInspectOption) (image.InspectResponse, error) {
	return image.InspectResponse{}, m.imageInspectErr
}

func (m *mockDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	m.pullCount++
	if m.imagePullErr != nil {
		return nil, m.imagePullErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (dockercontainer.CreateResponse, error) {
	m.createConfig = config
	m.createHostConfig = hostConfig
	return m.createResp, m.createErr
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options dockercontainer.StartOptions) error {
	m.startID = containerID
	return m.startErr
}

func (m *mockDockerClient) ContainerWait(ctx context.Context, containerID string, condition dockercontainer.WaitCondition) (<-chan dockercontainer.WaitResponse, <-chan error) {
	m.waitID = containerID
	respC := make(chan dockercontainer.WaitResponse, 1)
	errC := make(chan error, 1)
	if m.waitErrOut != nil {
		errC <- m.waitErrOut
	} else {
		respC <- m.waitResp
	}
	return respC, errC
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options dockercontainer.RemoveOptions) error {
	m.removeID = containerID
	return m.removeErr
}

func (m *mockDockerClient) ContainerResize(ctx context.Context, containerID string, options dockercontainer.ResizeOptions) error {
	m.resizeID = containerID
	m.resizeOpts = options
	return m.resizeErr
}

func (m *mockDockerClient) ContainerKill(ctx context.Context, containerID string, signal string) error {
	m.killID = containerID
	m.killSignal = signal
	return m.killErr
}

func (m *mockDockerClient) ContainerAttach(ctx context.Context, container string, options dockercontainer.AttachOptions) (types.HijackedResponse, error) {
	m.attachID = container
	return m.attachResp, m.attachErr
}

func TestUnit_Docker_PullImage(t *testing.T) {
	t.Run("retries on toomanyrequests error", func(t *testing.T) {
		mock := &mockDockerClient{
			imagePullErr: errors.New("toomanyrequests: too many requests"),
		}
		runtime := &DockerRuntime{
			client: mock,
			name:   "docker",
			sleepFunc: func(ctx context.Context, d time.Duration) error {
				return nil
			},
		}

		err := runtime.PullImage(context.Background(), "test-image", "always")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image after 3 attempts")
		assert.Equal(t, 3, mock.pullCount)
	})

	t.Run("retries on Rate exceeded error", func(t *testing.T) {
		mock := &mockDockerClient{
			imagePullErr: errors.New("Rate exceeded: please wait"),
		}
		runtime := &DockerRuntime{
			client: mock,
			name:   "docker",
			sleepFunc: func(ctx context.Context, d time.Duration) error {
				return nil
			},
		}

		err := runtime.PullImage(context.Background(), "test-image", "always")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image after 3 attempts")
		assert.Equal(t, 3, mock.pullCount)
	})

	t.Run("non-retryable error", func(t *testing.T) {
		mock := &mockDockerClient{
			imagePullErr: errors.New("some fatal error"),
		}
		runtime := &DockerRuntime{client: mock}
		err := runtime.PullImage(context.Background(), "test-image", "always")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "some fatal error")
		assert.Equal(t, 1, mock.pullCount)
	})

	t.Run("missing policy - inspect error not found", func(t *testing.T) {
		mock := &mockDockerClient{
			imageInspectErr: fmt.Errorf("not found: %w", errdefs.ErrNotFound),
		}
		runtime := &DockerRuntime{client: mock}
		err := runtime.PullImage(context.Background(), "test-image", "missing")
		assert.NoError(t, err)
		assert.Equal(t, 1, mock.pullCount)
	})

	t.Run("missing policy - fatal inspect error", func(t *testing.T) {
		mock := &mockDockerClient{
			imageInspectErr: errors.New("fatal inspect error"),
		}
		runtime := &DockerRuntime{client: mock}
		err := runtime.PullImage(context.Background(), "test-image", "missing")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fatal inspect error")
	})
}

func TestUnit_Docker_CreateContainer(t *testing.T) {
	t.Run("basic and complex config", func(t *testing.T) {
		mock := &mockDockerClient{
			createResp: dockercontainer.CreateResponse{ID: "created-id"},
		}
		runtime := &DockerRuntime{client: mock}

		config := &container.ContainerConfig{
			Image:   "test-image",
			Command: []string{"ls", "-l"},
			Env:     []string{"K=V"},
			Expose:  []string{"80/tcp", "53/udp"},
			Ports:   []string{"8080:80", "5353:53/udp"},
			Mounts: []container.Mount{
				{Type: "bind", Source: "/src", Target: "/dst"},
				{Type: "volume", Source: "myvol", Target: "/data"},
				{Type: "tmpfs", Target: "/cache"},
				{Type: "unknown", Source: "/ext", Target: "/ext"},
			},
			Devices: []container.DeviceMapping{
				{PathOnHost: "/dev/fuse", PathInContainer: "/dev/fuse", CgroupPermissions: "rmw"},
			},
			Memory: 1024 * 1024,
			CPUs:   0.5,
		}

		id, err := runtime.CreateContainer(context.Background(), config)
		assert.NoError(t, err)
		assert.Equal(t, "created-id", id)
		assert.Equal(t, "test-image", mock.createConfig.Image)
		assert.Equal(t, []string{"ls", "-l"}, []string(mock.createConfig.Cmd))
		assert.Equal(t, []string{"K=V"}, mock.createConfig.Env)
		assert.Equal(t, int64(0.5*1e9), mock.createHostConfig.Resources.NanoCPUs)
		assert.Equal(t, int64(1024*1024), mock.createHostConfig.Resources.Memory)
		assert.Len(t, mock.createHostConfig.Mounts, 4)
		assert.Len(t, mock.createHostConfig.Resources.Devices, 1)
		assert.NotNil(t, mock.createConfig.ExposedPorts)
	})

	t.Run("invalid port spec", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		config := &container.ContainerConfig{
			Ports: []string{"invalid"},
		}
		_, err := runtime.CreateContainer(context.Background(), config)
		assert.Error(t, err)
	})

	t.Run("invalid expose spec", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		config := &container.ContainerConfig{
			Expose: []string{"invalid/proto/extra"},
		}
		_, err := runtime.CreateContainer(context.Background(), config)
		assert.Error(t, err)
	})
}

func TestUnit_Docker_Lifecycle(t *testing.T) {
	id := "test-id"
	ctx := context.Background()

	t.Run("Start", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.StartContainer(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, id, mock.startID)
	})

	t.Run("Wait success", func(t *testing.T) {
		mock := &mockDockerClient{
			waitResp: dockercontainer.WaitResponse{StatusCode: 42},
		}
		runtime := &DockerRuntime{client: mock}
		code, err := runtime.WaitContainer(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, 42, code)
		assert.Equal(t, id, mock.waitID)
	})

	t.Run("Wait error", func(t *testing.T) {
		mock := &mockDockerClient{
			waitErrOut: errors.New("wait error"),
		}
		runtime := &DockerRuntime{client: mock}
		_, err := runtime.WaitContainer(ctx, id)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wait error")
	})

	t.Run("Remove success", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.RemoveContainer(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, id, mock.removeID)
	})

	t.Run("Remove suppression", func(t *testing.T) {
		mock := &mockDockerClient{
			removeErr: fmt.Errorf("not found: %w", errdefs.ErrNotFound),
		}
		runtime := &DockerRuntime{client: mock}
		err := runtime.RemoveContainer(ctx, id)
		assert.NoError(t, err) // Suppressed
	})

	t.Run("Resize", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.ResizeContainerTTY(ctx, id, 24, 80)
		assert.NoError(t, err)
		assert.Equal(t, id, mock.resizeID)
		assert.Equal(t, uint(24), mock.resizeOpts.Height)
		assert.Equal(t, uint(80), mock.resizeOpts.Width)
	})

	t.Run("Signal success", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.SignalContainer(ctx, id, "SIGINT")
		assert.NoError(t, err)
		assert.Equal(t, id, mock.killID)
		assert.Equal(t, "SIGINT", mock.killSignal)
	})

	t.Run("Signal suppression", func(t *testing.T) {
		mock := &mockDockerClient{
			killErr: fmt.Errorf("conflict: %w", errdefs.ErrConflict),
		}
		runtime := &DockerRuntime{client: mock}
		err := runtime.SignalContainer(ctx, id, "SIGINT")
		assert.NoError(t, err) // Suppressed
	})
}

type mockErrorReader struct{}

func (m *mockErrorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("stream error")
}
func (m *mockErrorReader) Close() error { return nil }

type mockConn struct {
	net.Conn
	closed     bool
	writeError error
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}
func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.writeError != nil {
		return 0, m.writeError
	}
	return len(b), nil
}
func (m *mockConn) CloseWrite() error {
	return nil
}


type mockDockerClientWithReader struct {
	mockDockerClient
	pullReader io.ReadCloser
}

func (m *mockDockerClientWithReader) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	m.pullCount++
	return m.pullReader, nil
}

func TestUnit_Docker_PullImage_StreamErrors(t *testing.T) {
	mock := &mockDockerClientWithReader{
		pullReader: io.NopCloser(&mockErrorReader{}),
	}
	runtime := &DockerRuntime{
		client: mock,
		sleepFunc: func(ctx context.Context, d time.Duration) error {
			return nil
		},
	}
	err := runtime.PullImage(context.Background(), "test-image", "always")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream error")
}

func TestUnit_Docker_AttachContainer(t *testing.T) {
	t.Run("TTY mode", func(t *testing.T) {
		conn := &mockConn{}
		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(strings.NewReader("output data")),
			},
		}
		runtime := &DockerRuntime{client: mock}

		stdout := &strings.Builder{}
		err := runtime.AttachContainer(context.Background(), "test-id", true, nil, stdout, nil)
		assert.NoError(t, err)
		assert.Equal(t, "output data", stdout.String())
		assert.True(t, conn.closed)
	})

	t.Run("Non-TTY mode (multiplexed)", func(t *testing.T) {
		conn := &mockConn{}
		// Create multiplexed stream
		var buf strings.Builder
		w := stdcopy.NewStdWriter(&buf, stdcopy.Stdout)
		_, _ = w.Write([]byte("stdout data"))
		w = stdcopy.NewStdWriter(&buf, stdcopy.Stderr)
		_, _ = w.Write([]byte("stderr data"))

		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(strings.NewReader(buf.String())),
			},
		}
		runtime := &DockerRuntime{client: mock}

		stdout := &strings.Builder{}
		stderr := &strings.Builder{}
		err := runtime.AttachContainer(context.Background(), "test-id", false, nil, stdout, stderr)
		assert.NoError(t, err)
		assert.Equal(t, "stdout data", stdout.String())
		assert.Equal(t, "stderr data", stderr.String())
		assert.True(t, conn.closed)
	})

	t.Run("with stdin", func(t *testing.T) {
		conn := &mockConn{}
		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(strings.NewReader("")),
			},
		}
		runtime := &DockerRuntime{client: mock}

		stdin := strings.NewReader("input data")
		err := runtime.AttachContainer(context.Background(), "test-id", true, stdin, nil, nil)
		assert.NoError(t, err)
		assert.True(t, conn.closed)
	})

	t.Run("context cancelled", func(t *testing.T) {
		conn := &mockConn{}
		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(strings.NewReader("never ending output...")),
			},
		}
		runtime := &DockerRuntime{client: mock}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := runtime.AttachContainer(ctx, "test-id", true, nil, nil, nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
		assert.True(t, conn.closed)
	})

	t.Run("stdin write error", func(t *testing.T) {
		conn := &mockConn{writeError: errors.New("write error")}
		// Use a reader that doesn't immediately return EOF to ensure stdin error is caught
		pr, _ := io.Pipe()
		defer pr.Close()

		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(pr),
			},
		}
		runtime := &DockerRuntime{client: mock}

		stdin := strings.NewReader("input data")
		err := runtime.AttachContainer(context.Background(), "test-id", true, stdin, nil, nil)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "write error")
		}
	})
}
