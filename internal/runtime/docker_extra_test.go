package runtime

import (
	"errors"
	"io"
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitDockerRetryablePullErrorExhaustive(t *testing.T) {
	assert.False(t, IsRetryablePullError(nil))
	assert.True(t, IsRetryablePullError(errors.New("toomanyrequests")))
	assert.True(t, IsRetryablePullError(errors.New("rate exceeded")))
	assert.True(t, IsRetryablePullError(errors.New("rate limit")))
	assert.True(t, IsRetryablePullError(errors.New("data limit exceeded")))
	assert.True(t, IsRetryablePullError(errors.New("i/o timeout")))
	assert.True(t, IsRetryablePullError(errors.New("connection refused")))
	assert.True(t, IsRetryablePullError(errors.New("connection reset")))
	assert.True(t, IsRetryablePullError(io.ErrUnexpectedEOF))
	assert.True(t, IsRetryablePullError(errors.New("token expired")))
	assert.False(t, IsRetryablePullError(errors.New("unauthorized")))
	assert.False(t, IsRetryablePullError(errors.New("other error")))
	assert.False(t, IsRetryablePullError(io.EOF))
}

func TestUnit_Docker_toDockerContainerConfig_Pid(t *testing.T) {
	config := &container.ContainerConfig{
		Image: "alpine",
		Pid:   "host",
	}
	_, hostConfig, _, err := toDockerContainerConfig(config)
	require.NoError(t, err)
	assert.Equal(t, "host", string(hostConfig.PidMode))
}

func TestUnit_Docker_toDockerContainerConfig_Ulimits(t *testing.T) {
	config := &container.ContainerConfig{
		Image: "alpine",
		Ulimits: []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
		},
	}
	_, hostConfig, _, err := toDockerContainerConfig(config)
	require.NoError(t, err)
	require.Len(t, hostConfig.Ulimits, 1)
	assert.Equal(t, "nofile", hostConfig.Ulimits[0].Name)
	assert.Equal(t, int64(1024), hostConfig.Ulimits[0].Soft)
	assert.Equal(t, int64(2048), hostConfig.Ulimits[0].Hard)
}
