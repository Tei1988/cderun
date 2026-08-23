package controlsocket

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type mockDispatcher struct {
	createFunc func(ctx context.Context, config *container.ContainerConfig) (string, error)
	startFunc  func(ctx context.Context, containerID string) error
	waitFunc   func(ctx context.Context, containerID string) (int, error)
	removeFunc func(ctx context.Context, containerID string) error
}

func (m *mockDispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, config)
	}
	return "mock-container-id", nil
}

func (m *mockDispatcher) StartContainer(ctx context.Context, containerID string) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, containerID)
	}
	return nil
}

func (m *mockDispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	if m.waitFunc != nil {
		return m.waitFunc(ctx, containerID)
	}
	return 0, nil
}

func (m *mockDispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	if m.removeFunc != nil {
		return m.removeFunc(ctx, containerID)
	}
	return nil
}

func TestUnit_ControlSocket_RPC_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "rpc_test.sock")

	disp := &mockDispatcher{
		createFunc: func(ctx context.Context, config *container.ContainerConfig) (string, error) {
			if config.Image == "invalid" {
				return "", errors.New("invalid image")
			}
			return "container-abc-123", nil
		},
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			if containerID == "container-abc-123" {
				return 42, nil
			}
			return 1, fmt.Errorf("unknown container %s", containerID)
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. CreateContainer success
	cfg := &container.ContainerConfig{Image: "alpine:latest"}
	cID, err := client.CreateContainer(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, "container-abc-123", cID)

	// 2. CreateContainer error
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{Image: "invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image")

	// 3. StartContainer success
	err = client.StartContainer(ctx, cID)
	require.NoError(t, err)

	// 4. WaitContainer success
	exitCode, err := client.WaitContainer(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, 42, exitCode)

	// 5. RemoveContainer success
	err = client.RemoveContainer(ctx, cID)
	require.NoError(t, err)
}

func TestUnit_ControlSocket_RPC_NoDispatcherError(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "nodisp.sock")

	server := NewServer(socketPath, logging.NewLogger())
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")
}
