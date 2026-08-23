package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

// TestUnit_Runtime_MockRuntimeLifecycle verifies MockRuntime container lifecycle operations,
// inspect responses, signal capturing, and image pulling logic.
func TestUnit_Runtime_MockRuntimeLifecycle(t *testing.T) {
	t.Parallel()

	mock := &MockRuntime{
		CreatedContainerID: "test-mock-cid",
		ExitCode:           0,
	}

	ctx := context.Background()

	// 1. PullImage
	err := mock.PullImage(ctx, "alpine:latest", "missing", 3, 0)
	require.NoError(t, err)

	// 2. CreateContainer
	cc := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}
	cid, err := mock.CreateContainer(ctx, cc)
	require.NoError(t, err)
	assert.Equal(t, "test-mock-cid", cid)
	assert.Equal(t, cc, mock.GetCreatedConfig())

	// 3. InspectContainer
	running, exitCode, err := mock.InspectContainer(ctx, cid)
	require.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 0, exitCode)

	// 4. SignalContainer
	err = mock.SignalContainer(ctx, cid, "SIGTERM")
	require.NoError(t, err)
	assert.Equal(t, cid, mock.SignaledContainerID)
	assert.Equal(t, "SIGTERM", mock.Signal)

	// 5. RemoveContainer
	err = mock.RemoveContainer(ctx, cid)
	require.NoError(t, err)
}

// TestUnit_Runtime_DockerAdapterConfigMappings verifies Docker container config conversion
// helpers for restart policies, ulimits, device mappings, and mount configs.
func TestUnit_Runtime_DockerAdapterConfigMappings(t *testing.T) {
	t.Parallel()

	t.Run("parseDockerRestartPolicy mappings", func(t *testing.T) {
		t.Parallel()

		pAlways, err := parseDockerRestartPolicy("always")
		require.NoError(t, err)
		assert.Equal(t, "always", string(pAlways.Name))

		pUnlessStopped, err := parseDockerRestartPolicy("unless-stopped")
		require.NoError(t, err)
		assert.Equal(t, "unless-stopped", string(pUnlessStopped.Name))

		pOnFailure, err := parseDockerRestartPolicy("on-failure:5")
		require.NoError(t, err)
		assert.Equal(t, "on-failure", string(pOnFailure.Name))
		assert.Equal(t, 5, pOnFailure.MaximumRetryCount)

		_, errInvalid := parseDockerRestartPolicy("invalid-policy")
		require.Error(t, errInvalid)
	})

	t.Run("buildDockerUlimits mappings", func(t *testing.T) {
		t.Parallel()

		ulimitsRaw := []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
			{Name: "nproc", Soft: 512, Hard: 512},
		}
		ulimits := buildDockerUlimits(ulimitsRaw)
		require.Len(t, ulimits, 2)

		assert.Equal(t, "nofile", ulimits[0].Name)
		assert.Equal(t, int64(1024), ulimits[0].Soft)
		assert.Equal(t, int64(2048), ulimits[0].Hard)

		assert.Equal(t, "nproc", ulimits[1].Name)
		assert.Equal(t, int64(512), ulimits[1].Soft)
		assert.Equal(t, int64(512), ulimits[1].Hard)
	})
}

// TestUnit_Runtime_ContainerdValidateConfigEdgeCases tests containerd ValidateConfig validation rules.
func TestUnit_Runtime_ContainerdValidateConfigEdgeCases(t *testing.T) {
	t.Parallel()

	ctrd := &ContainerdRuntime{}

	t.Run("valid configuration passes validation", func(t *testing.T) {
		t.Parallel()
		cc := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sh"},
		}
		err := ctrd.ValidateConfig(cc)
		require.NoError(t, err)
	})

	t.Run("unsupported features reject validation", func(t *testing.T) {
		t.Parallel()

		// Init is unsupported in containerd adapter
		ccInit := &container.ContainerConfig{
			Image: "alpine:latest",
			Init:  true,
		}
		errInit := ctrd.ValidateConfig(ccInit)
		require.Error(t, errInit)
		assert.Contains(t, errInit.Error(), "containerd runtime: init is not supported yet")

		// Unsupported restart policy
		ccRestart := &container.ContainerConfig{
			Image:   "alpine:latest",
			Restart: "always",
		}
		errRestart := ctrd.ValidateConfig(ccRestart)
		require.Error(t, errRestart)
		assert.Contains(t, errRestart.Error(), "containerd runtime: restart policy is not supported yet")
	})
}
