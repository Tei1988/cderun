package runtime

import (
	"context"
	"testing"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

// TestComprehensiveAdapter_MockRuntimeLifecycle tests MockRuntime behavior and call tracking.
// Ref: docs/features/multi-runtime-support.md, docs/testing/strategy.md
func TestComprehensiveAdapter_MockRuntimeLifecycle(t *testing.T) {
	t.Parallel()

	mock := NewMockRuntime()
	mock.CreatedContainerID = "mock-cid-12345"
	mock.ExitCode = 0

	ctx := context.Background()

	// Create
	cc := &container.ContainerConfig{
		Image:   "golang:1.22",
		Command: []string{"go", "version"},
	}
	cid, err := mock.CreateContainer(ctx, cc)
	require.NoError(t, err)
	assert.Equal(t, "mock-cid-12345", cid)

	// Start
	err = mock.StartContainer(ctx, cid)
	require.NoError(t, err)

	// Signal
	err = mock.SignalContainer(ctx, cid, "SIGKILL")
	require.NoError(t, err)

	// TTY Resize
	err = mock.ResizeContainerTTY(ctx, cid, 30, 100)
	require.NoError(t, err)

	// Inspect
	info, state, err := mock.InspectContainer(ctx, cid)
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.NotNil(t, state)

	// Wait
	code, err := mock.WaitContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, 0, code)

	// Remove
	err = mock.RemoveContainer(ctx, cid)
	require.NoError(t, err)
}

// TestComprehensiveAdapter_DockerConversionHelpers tests Docker adapter mapping functions.
// Ref: docs/features/direct-container-execution.md
func TestComprehensiveAdapter_DockerConversionHelpers(t *testing.T) {
	t.Parallel()

	t.Run("buildDockerUlimits mappings", func(t *testing.T) {
		t.Parallel()

		ulimits := []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
			{Name: "nproc", Soft: 512, Hard: 1024},
		}

		dockUlimits := buildDockerUlimits(ulimits)
		require.Len(t, dockUlimits, 2)
		assert.Equal(t, "nofile", dockUlimits[0].Name)
		assert.Equal(t, int64(1024), dockUlimits[0].Soft)
		assert.Equal(t, int64(2048), dockUlimits[0].Hard)

		assert.Nil(t, buildDockerUlimits(nil))
	})

	t.Run("parseDockerRestartPolicy valid modes", func(t *testing.T) {
		t.Parallel()

		modes := map[string]string{
			"no":             "no",
			"always":         "always",
			"unless-stopped": "unless-stopped",
			"on-failure":     "on-failure",
		}

		for input, expected := range modes {
			policy, err := parseDockerRestartPolicy(input)
			require.NoError(t, err, "input: %s", input)
			assert.Equal(t, dockercontainer.RestartPolicyMode(expected), policy.Name)
		}
	})

	t.Run("parseGPUs options", func(t *testing.T) {
		t.Parallel()

		opts, err := parseGPUs("device=0,1")
		require.NoError(t, err)
		assert.Len(t, opts, 1)

		optsAll, err := parseGPUs("all")
		require.NoError(t, err)
		assert.Len(t, optsAll, 1)
	})
}

// TestComprehensiveAdapter_ContainerdValidation tests containerd ValidateConfig rules.
// Ref: docs/features/multi-runtime-support.md
func TestComprehensiveAdapter_ContainerdValidation(t *testing.T) {
	t.Parallel()

	crt := &ContainerdRuntime{}

	t.Run("Valid basic config passes", func(t *testing.T) {
		t.Parallel()

		cc := &container.ContainerConfig{
			Image:   "alpine:3.19",
			Command: []string{"echo", "hello"},
		}
		assert.NoError(t, crt.ValidateConfig(cc))
	})

	t.Run("Unsupported port mapping fails", func(t *testing.T) {
		t.Parallel()

		cc := &container.ContainerConfig{
			Image: "alpine:3.19",
			Ports: []string{"8080:80"},
		}
		err := crt.ValidateConfig(cc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "port mapping is not supported")
	})

	t.Run("Unsupported restart policy fails", func(t *testing.T) {
		t.Parallel()

		cc := &container.ContainerConfig{
			Image:   "alpine:3.19",
			Restart: "always",
		}
		err := crt.ValidateConfig(cc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "restart policy is not supported")
	})
}
