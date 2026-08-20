package runtime

import (
	"context"
	"testing"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

func TestUnit_Runtime_Robustness_MockLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := NewMockRuntime()

	cc := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	err := mock.ValidateConfig(cc)
	require.NoError(t, err)

	err = mock.PullImage(ctx, cc.Image, "missing", 3, time.Second)
	require.NoError(t, err)

	id, err := mock.CreateContainer(ctx, cc)
	require.NoError(t, err)

	err = mock.StartContainer(ctx, id)
	require.NoError(t, err)

	code, err := mock.WaitContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 0, code)

	err = mock.RemoveContainer(ctx, id)
	require.NoError(t, err)
}

func TestUnit_Runtime_Robustness_DockerAdapterGPUAndRestart(t *testing.T) {
	t.Parallel()

	reqs, err := parseGPUs("all")
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Equal(t, []string{"gpu"}, reqs[0].Capabilities[0])

	policy, err := parseDockerRestartPolicy("on-failure:5")
	require.NoError(t, err)
	assert.Equal(t, dockercontainer.RestartPolicyMode("on-failure"), policy.Name)
	assert.Equal(t, 5, policy.MaximumRetryCount)

	invalidPolicy, err := parseDockerRestartPolicy("invalid-policy")
	require.Error(t, err)
	assert.Empty(t, invalidPolicy.Name)
}

func TestUnit_Runtime_Robustness_ContainerdValidation(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{}

	// Invalid restart policy in containerd
	ccInvalidRestart := &container.ContainerConfig{
		Image:   "alpine:latest",
		Restart: "always",
	}
	err := rt.ValidateConfig(ccInvalidRestart)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restart policy is not supported yet")

	// Invalid GPUs in containerd
	ccInvalidGPUs := &container.ContainerConfig{
		Image: "alpine:latest",
		GPUs:  "all",
	}
	err = rt.ValidateConfig(ccInvalidGPUs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gpus is not supported yet")
}
