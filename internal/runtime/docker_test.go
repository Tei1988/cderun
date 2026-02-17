package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"cderun/internal/container"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_Docker_New(t *testing.T) {
	// This should succeed even without docker daemon as it just creates the client
	runtime, err := NewDockerRuntime("/var/run/docker.sock")
	require.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, "docker", runtime.Name())

	runtimeWithName, err := NewDockerRuntimeWithName("/var/run/docker.sock", "custom")
	require.NoError(t, err)
	assert.Equal(t, "custom", runtimeWithName.Name())

	// Test the default sleepFunc
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err = runtimeWithName.sleepFunc(ctx, 1*time.Millisecond)
	assert.NoError(t, err)

	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	err = runtimeWithName.sleepFunc(ctx2, 1*time.Second)
	assert.ErrorIs(t, err, context.Canceled)
}

type mockDockerClient struct {
	imageInspectErr error
	imagePullErr    error
	pullCount       int
	pullReader      io.ReadCloser

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
	if m.pullReader != nil {
		return m.pullReader, nil
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

func TestUnit_Runtime_Docker_IsRetryablePullError(t *testing.T) {
	assert.False(t, isRetryablePullError(nil))
	assert.True(t, isRetryablePullError(errors.New("toomanyrequests")))
	assert.True(t, isRetryablePullError(errors.New("Rate exceeded")))
	assert.False(t, isRetryablePullError(errors.New("other error")))
}

type mockRetryDockerClient struct {
	mockDockerClient
	maxFailures int
}

func (m *mockRetryDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	m.pullCount++
	if m.pullCount <= m.maxFailures {
		return nil, errors.New("toomanyrequests")
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func TestUnit_Runtime_Docker_PullImage(t *testing.T) {
	t.Run("never policy", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.PullImage(context.Background(), "test", "never")
		require.NoError(t, err)
		assert.Equal(t, 0, mock.pullCount)
	})

	t.Run("missing policy - exists", func(t *testing.T) {
		mock := &mockDockerClient{imageInspectErr: nil}
		runtime := &DockerRuntime{client: mock}
		err := runtime.PullImage(context.Background(), "test", "missing")
		require.NoError(t, err)
		assert.Equal(t, 0, mock.pullCount)
	})

	t.Run("missing policy - unexpected error", func(t *testing.T) {
		mock := &mockDockerClient{imageInspectErr: errors.New("boom")}
		runtime := &DockerRuntime{client: mock}
		err := runtime.PullImage(context.Background(), "test", "missing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to inspect image")
	})

	t.Run("non-retryable pull error", func(t *testing.T) {
		mock := &mockDockerClient{imagePullErr: errors.New("fatal error")}
		runtime := &DockerRuntime{client: mock}
		err := runtime.PullImage(context.Background(), "test", "always")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image")
		assert.Equal(t, 1, mock.pullCount)
	})

	t.Run("non-retryable stream error", func(t *testing.T) {
		mock := &mockDockerClient{
			pullReader: io.NopCloser(strings.NewReader("invalid json")),
		}
		runtime := &DockerRuntime{client: mock}
		err := runtime.PullImage(context.Background(), "test", "always")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image (stream)")
	})

	t.Run("retries on toomanyrequests error", func(t *testing.T) {
		mock := &mockDockerClient{
			imagePullErr: errors.New("toomanyrequests: too many requests"),
		}
		runtime := &DockerRuntime{
			client:    mock,
			sleepFunc: func(ctx context.Context, d time.Duration) error { return nil },
		}

		err := runtime.PullImage(context.Background(), "test-image", "always")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image after 3 attempts")
		assert.Equal(t, 3, mock.pullCount)
	})

	t.Run("succeeds after retries", func(t *testing.T) {
		mock := &mockRetryDockerClient{maxFailures: 2}
		runtime := &DockerRuntime{
			client:    mock,
			sleepFunc: func(ctx context.Context, d time.Duration) error { return nil },
		}
		err := runtime.PullImage(context.Background(), "test-image", "always")
		require.NoError(t, err)
		assert.Equal(t, 3, mock.pullCount)
	})

	t.Run("sleep context cancelled", func(t *testing.T) {
		mock := &mockDockerClient{
			imagePullErr: errors.New("toomanyrequests"),
		}
		runtime := &DockerRuntime{
			client: mock,
			sleepFunc: func(ctx context.Context, d time.Duration) error {
				return context.Canceled
			},
		}
		err := runtime.PullImage(context.Background(), "test", "always")
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestUnit_Runtime_Docker_CreateContainer(t *testing.T) {
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
		require.NoError(t, err)
		assert.Equal(t, "created-id", id)
		assert.Equal(t, "test-image", mock.createConfig.Image)
		assert.Equal(t, []string{"ls", "-l"}, []string(mock.createConfig.Cmd))
		assert.Equal(t, []string{"K=V"}, mock.createConfig.Env)
		assert.Equal(t, int64(0.5*1e9), mock.createHostConfig.NanoCPUs)
		assert.Equal(t, int64(1024*1024), mock.createHostConfig.Memory)
		assert.Len(t, mock.createHostConfig.Mounts, 4)
		assert.Len(t, mock.createHostConfig.Devices, 1)
		assert.NotNil(t, mock.createConfig.ExposedPorts)
	})

	t.Run("invalid port spec", func(t *testing.T) {
		runtime := &DockerRuntime{client: &mockDockerClient{}}
		_, err := runtime.CreateContainer(context.Background(), &container.ContainerConfig{
			Ports: []string{"invalid"},
		})
		require.Error(t, err)
	})

	t.Run("invalid expose port", func(t *testing.T) {
		runtime := &DockerRuntime{client: &mockDockerClient{}}
		_, err := runtime.CreateContainer(context.Background(), &container.ContainerConfig{
			Expose: []string{"invalid"},
		})
		require.Error(t, err)
	})
}

type errNotFound struct{ error }

func (e errNotFound) NotFound() {}

type errConflict struct{ error }

func (e errConflict) Conflict() {}

func TestUnit_Runtime_Docker_Lifecycle(t *testing.T) {
	id := "test-id"
	ctx := context.Background()

	t.Run("Start", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.StartContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, mock.startID)
	})

	t.Run("Wait", func(t *testing.T) {
		mock := &mockDockerClient{
			waitResp: dockercontainer.WaitResponse{StatusCode: 0},
		}
		runtime := &DockerRuntime{client: mock}
		code, err := runtime.WaitContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 0, code)
		assert.Equal(t, id, mock.waitID)
	})

	t.Run("Wait error", func(t *testing.T) {
		mock := &mockDockerClient{
			waitErrOut: errors.New("wait error"),
		}
		runtime := &DockerRuntime{client: mock}
		_, err := runtime.WaitContainer(ctx, id)
		require.Error(t, err)
	})

	t.Run("Remove", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.RemoveContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, mock.removeID)
	})

	t.Run("Remove - suppressed errors", func(t *testing.T) {
		t.Run("NotFound", func(t *testing.T) {
			mock := &mockDockerClient{removeErr: errNotFound{errors.New("not found")}}
			runtime := &DockerRuntime{client: mock}
			err := runtime.RemoveContainer(ctx, id)
			assert.NoError(t, err)
		})
		t.Run("Conflict", func(t *testing.T) {
			mock := &mockDockerClient{removeErr: errConflict{errors.New("conflict")}}
			runtime := &DockerRuntime{client: mock}
			err := runtime.RemoveContainer(ctx, id)
			assert.NoError(t, err)
		})
	})

	t.Run("Resize", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.ResizeContainerTTY(ctx, id, 24, 80)
		require.NoError(t, err)
		assert.Equal(t, id, mock.resizeID)
		assert.Equal(t, uint(24), mock.resizeOpts.Height)
		assert.Equal(t, uint(80), mock.resizeOpts.Width)
	})

	t.Run("Signal", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{client: mock}
		err := runtime.SignalContainer(ctx, id, "SIGINT")
		require.NoError(t, err)
		assert.Equal(t, id, mock.killID)
		assert.Equal(t, "SIGINT", mock.killSignal)
	})

	t.Run("Signal - suppressed errors", func(t *testing.T) {
		t.Run("NotFound", func(t *testing.T) {
			mock := &mockDockerClient{killErr: errNotFound{errors.New("not found")}}
			runtime := &DockerRuntime{client: mock}
			err := runtime.SignalContainer(ctx, id, "SIGKILL")
			assert.NoError(t, err)
		})
		t.Run("Conflict", func(t *testing.T) {
			mock := &mockDockerClient{killErr: errConflict{errors.New("conflict")}}
			runtime := &DockerRuntime{client: mock}
			err := runtime.SignalContainer(ctx, id, "SIGKILL")
			assert.NoError(t, err)
		})
	})
}

func TestUnit_Runtime_Docker_AttachContainer_Multiplexed(t *testing.T) {
	t.Run("non-TTY mode (multiplexed)", func(t *testing.T) {
		// Use stdcopy to create a multiplexed stream
		stdoutBuf := &bytes.Buffer{}
		stderrBuf := &bytes.Buffer{}

		msgStdout := "hello stdout"
		msgStderr := "hello stderr"

		// Create a pipe to simulate the docker hijacked response reader
		pr, pw := io.Pipe()
		go func() {
			sw := stdcopy.NewStdWriter(pw, stdcopy.Stdout)
			_, _ = sw.Write([]byte(msgStdout))
			sw = stdcopy.NewStdWriter(pw, stdcopy.Stderr)
			_, _ = sw.Write([]byte(msgStderr))
			_ = pw.Close()
		}()

		conn := &mockConn{}
		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(pr),
			},
		}
		runtime := &DockerRuntime{client: mock}

		err := runtime.AttachContainer(context.Background(), "test-id", false, nil, stdoutBuf, stderrBuf)
		require.NoError(t, err)
		assert.Equal(t, msgStdout, stdoutBuf.String())
		assert.Equal(t, msgStderr, stderrBuf.String())
		assert.True(t, conn.closed)
	})

	t.Run("attach error", func(t *testing.T) {
		mock := &mockDockerClient{attachErr: errors.New("attach failed")}
		runtime := &DockerRuntime{client: mock}
		err := runtime.AttachContainer(context.Background(), "id", false, nil, nil, nil)
		require.Error(t, err)
	})
}

type mockConn struct {
	net.Conn
	closed bool
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *mockConn) CloseWrite() error {
	return nil
}

func TestUnit_Runtime_Docker_AttachContainer(t *testing.T) {
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
		require.NoError(t, err)
		assert.Equal(t, "output data", stdout.String())
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
		require.NoError(t, err)
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
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
		assert.True(t, conn.closed)
	})
}
