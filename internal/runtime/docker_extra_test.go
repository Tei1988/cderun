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
