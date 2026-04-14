package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockContainerdClient struct {
	pullFunc         func(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error)
	getImageFunc     func(ctx context.Context, ref string) (client.Image, error)
	newContainerFunc func(ctx context.Context, id string, opts ...client.NewContainerOpts) (client.Container, error)
	loadContainerFunc func(ctx context.Context, id string) (client.Container, error)
	imageServiceFunc func() images.Store
	closeFunc        func() error
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

func (m *mockContainerdClient) ImageService() images.Store {
	if m.imageServiceFunc != nil {
		return m.imageServiceFunc()
	}
	return nil
}

func (m *mockContainerdClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestUnit_Containerd_Name(t *testing.T) {
	r := &ContainerdRuntime{}
	assert.Equal(t, "containerd", r.Name())
}

var containerdNoopSleepFunc = func(ctx context.Context, d time.Duration) error { return nil }

func TestUnit_Containerd_PullImage(t *testing.T) {
	t.Run("never policy", func(t *testing.T) {
		r := &ContainerdRuntime{}
		err := r.PullImage(context.Background(), "test", "never", 3, 1*time.Second)
		require.NoError(t, err)
	})

	t.Run("retry pull error", func(t *testing.T) {
		pullCount := 0
		mock := &mockContainerdClient{
			pullFunc: func(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error) {
				pullCount++
				return nil, errors.New("toomanyrequests")
			},
		}
		r := &ContainerdRuntime{
			client:    mock,
			sleepFunc: containerdNoopSleepFunc,
		}
		err := r.PullImage(context.Background(), "test", "always", 3, 1*time.Second)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image after 3 attempts")
		assert.Equal(t, 3, pullCount)
	})
}
