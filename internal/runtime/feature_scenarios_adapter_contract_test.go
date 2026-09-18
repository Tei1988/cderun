package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

func TestUnit_Runtime_MockRuntime_ComprehensiveLifecycle(t *testing.T) {
	t.Parallel()

	mock := &MockRuntime{
		CreatedContainerID: "cid-mock-123",
		ExitCode:           0,
	}

	ctx := context.Background()

	// PullImage
	err := mock.PullImage(ctx, "ubuntu:latest", "missing", 3, 0)
	require.NoError(t, err)

	// CreateContainer
	cc := &container.ContainerConfig{
		Image:   "ubuntu:latest",
		Command: []string{"bash"},
	}
	cid, err := mock.CreateContainer(ctx, cc)
	require.NoError(t, err)
	assert.Equal(t, "cid-mock-123", cid)
	assert.Equal(t, cc, mock.GetCreatedConfig())

	// StartContainer
	err = mock.StartContainer(ctx, cid)
	require.NoError(t, err)

	// WaitContainer
	exitCode, err := mock.WaitContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// InspectContainer
	running, code, err := mock.InspectContainer(ctx, cid)
	require.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 0, code)

	// SignalContainer
	err = mock.SignalContainer(ctx, cid, "SIGKILL")
	require.NoError(t, err)
	assert.Equal(t, cid, mock.SignaledContainerID)
	assert.Equal(t, "SIGKILL", mock.Signal)

	// RemoveContainer
	err = mock.RemoveContainer(ctx, cid)
	require.NoError(t, err)
}

func TestUnit_Runtime_DockerAdapter_OptionalMountContractRejection(t *testing.T) {
	t.Parallel()

	mounts := []container.Mount{
		{
			Source:   "/host/dir",
			Target:   "/container/dir",
			Type:     "bind",
			Optional: true, // Conversion contract: optional mount is not supported
		},
	}

	_, err := buildDockerMounts(mounts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker runtime: optional mount is not supported")
}

func TestUnit_Runtime_Containerd_ValidationRules(t *testing.T) {
	t.Parallel()

	ctrd := &ContainerdRuntime{}

	t.Run("valid configuration passes", func(t *testing.T) {
		t.Parallel()

		cc := &container.ContainerConfig{
			Image:   "redis:alpine",
			Command: []string{"redis-server"},
		}
		require.NoError(t, ctrd.ValidateConfig(cc))
	})

	t.Run("unsupported features reject validation", func(t *testing.T) {
		t.Parallel()

		ccInit := &container.ContainerConfig{
			Image: "alpine:latest",
			Init:  true,
		}
		errInit := ctrd.ValidateConfig(ccInit)
		require.Error(t, errInit)
		assert.Contains(t, errInit.Error(), "containerd runtime: init is not supported yet")

		ccRestart := &container.ContainerConfig{
			Image:   "alpine:latest",
			Restart: "always",
		}
		errRestart := ctrd.ValidateConfig(ccRestart)
		require.Error(t, errRestart)
		assert.Contains(t, errRestart.Error(), "containerd runtime: restart policy is not supported yet")
	})
}
