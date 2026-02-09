package runtime

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

func TestNewDockerRuntime(t *testing.T) {
	// This should succeed even without docker daemon as it just creates the client
	runtime, err := NewDockerRuntime("/var/run/docker.sock")
	assert.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, "docker", runtime.Name())
}

type mockDockerClient struct {
	dockerClient
	imageInspectErr error
	imagePullErr    error
	pullCount       int
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

type mockOneRetryDockerClient struct {
	mockDockerClient
}

func (m *mockOneRetryDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	m.pullCount++
	if m.pullCount == 1 {
		return nil, errors.New("toomanyrequests: too many requests")
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func TestDockerRuntime_PullImage_Retry(t *testing.T) {
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

	t.Run("succeeds on first attempt", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{
			client: mock,
			name:   "docker",
			sleepFunc: func(ctx context.Context, d time.Duration) error {
				return nil
			},
		}

		err := runtime.PullImage(context.Background(), "test-image", "always")
		assert.NoError(t, err)
		assert.Equal(t, 1, mock.pullCount)
	})

	t.Run("succeeds after one retry", func(t *testing.T) {
		mock := &mockOneRetryDockerClient{}
		runtime := &DockerRuntime{
			client: mock,
			name:   "docker",
			sleepFunc: func(ctx context.Context, d time.Duration) error {
				return nil
			},
		}

		err := runtime.PullImage(context.Background(), "test-image", "always")
		assert.NoError(t, err)
		assert.Equal(t, 2, mock.pullCount)
	})
}
