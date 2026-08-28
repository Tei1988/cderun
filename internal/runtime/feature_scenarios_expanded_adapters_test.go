package runtime

import (
	"context"
	"testing"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

// TestUnit_Runtime_MockRuntime_ExpandedLifecycle verifies MockRuntime operations
// as specified in docs/features/multi-runtime-support.md and docs/testing/strategy.md.
func TestUnit_Runtime_MockRuntime_ExpandedLifecycle(t *testing.T) {
	t.Parallel()

	mock := &MockRuntime{
		CreatedContainerID: "cid-mock-expanded",
		ExitCode:           0,
	}

	ctx := context.Background()

	// CreateContainer
	cc := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"sh"},
	}
	cid, err := mock.CreateContainer(ctx, cc)
	require.NoError(t, err)
	assert.Equal(t, "cid-mock-expanded", cid)

	// StartContainer
	err = mock.StartContainer(ctx, cid)
	require.NoError(t, err)

	// SignalContainer
	err = mock.SignalContainer(ctx, cid, "SIGTERM")
	require.NoError(t, err)

	// ResizeContainerTTY
	err = mock.ResizeContainerTTY(ctx, cid, 24, 80)
	require.NoError(t, err)

	// WaitContainer
	exitCode, err := mock.WaitContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// RemoveContainer
	err = mock.RemoveContainer(ctx, cid)
	require.NoError(t, err)

	// InspectContainer
	info, _, err := mock.InspectContainer(ctx, cid)
	require.NoError(t, err)
	assert.NotNil(t, info)
}

// TestUnit_Runtime_DockerAdapter_ExpandedHelpers verifies Docker adapter configuration mapping
// and optional mount rejection contract as specified in docs/features/direct-container-execution.md.
func TestUnit_Runtime_DockerAdapter_ExpandedHelpers(t *testing.T) {
	t.Parallel()

	t.Run("parseGPUs boundary test", func(t *testing.T) {
		opts, err := parseGPUs("all")
		require.NoError(t, err)
		assert.Len(t, opts, 1)

		opts, err = parseGPUs("count=2")
		require.NoError(t, err)
		assert.Len(t, opts, 1)

		_, err = parseGPUs("invalid_gpu_spec")
		assert.Error(t, err)
	})

	t.Run("parseDockerRestartPolicy test", func(t *testing.T) {
		p, err := parseDockerRestartPolicy("always")
		require.NoError(t, err)
		assert.Equal(t, dockercontainer.RestartPolicyMode("always"), p.Name)

		p, err = parseDockerRestartPolicy("on-failure:3")
		require.NoError(t, err)
		assert.Equal(t, dockercontainer.RestartPolicyMode("on-failure"), p.Name)
		assert.Equal(t, 3, p.MaximumRetryCount)

		_, err = parseDockerRestartPolicy("invalid-policy")
		assert.Error(t, err)
	})

	t.Run("buildDockerMounts optional mount rejection contract", func(t *testing.T) {
		mounts := []container.Mount{
			{
				Source:   "/host/path",
				Target:   "/container/path",
				Optional: true,
			},
		}
		_, err := buildDockerMounts(mounts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "optional mount is not supported")
	})
}

// TestUnit_Runtime_Containerd_ExpandedValidateConfig verifies containerd validation rules
// as specified in docs/features/multi-runtime-support.md.
func TestUnit_Runtime_Containerd_ExpandedValidateConfig(t *testing.T) {
	t.Parallel()

	runtime := &ContainerdRuntime{}

	t.Run("Valid config passes validation", func(t *testing.T) {
		cc := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sh"},
		}
		err := runtime.ValidateConfig(cc)
		assert.NoError(t, err)
	})

	t.Run("Unsupported network mode fails validation", func(t *testing.T) {
		cc := &container.ContainerConfig{
			Image:   "alpine:latest",
			Network: "bridge",
		}
		err := runtime.ValidateConfig(cc)
		assert.Error(t, err)
	})

	t.Run("Unsupported volume mount fails validation", func(t *testing.T) {
		cc := &container.ContainerConfig{
			Image: "alpine:latest",
			Mounts: []container.Mount{
				{
					Type:   "volume",
					Source: "myvol",
					Target: "/data",
				},
			},
		}
		err := runtime.ValidateConfig(cc)
		assert.Error(t, err)
	})
}
