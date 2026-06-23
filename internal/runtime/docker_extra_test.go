package runtime

import (
	"errors"
	"testing"
	"time"

	"cderun/internal/logging"
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
	// io.EOF is NOT in the retryable messages, only "unexpected eof" is.
	assert.True(t, IsRetryablePullError(errors.New("unexpected eof")))
	assert.True(t, IsRetryablePullError(errors.New("token expired")))
	assert.False(t, IsRetryablePullError(errors.New("unauthorized")))
	assert.False(t, IsRetryablePullError(errors.New("other error")))
}

func TestUnit_Docker_Options(t *testing.T) {
	t.Run("WithLogger", func(t *testing.T) {
		logger := logging.GetGlobalLogger()
		rt := &DockerRuntime{}
		opt := WithLogger(logger)
		opt(rt)
		assert.Equal(t, logger, rt.logger)
	})

	t.Run("WithAttachCloseWriteGrace", func(t *testing.T) {
		rt := &DockerRuntime{}
		grace := 500 * time.Millisecond
		opt := WithAttachCloseWriteGrace(grace)
		opt(rt)
		assert.Equal(t, grace, rt.attachCloseWriteGrace)

		// Zero or negative should set to 1ms
		opt = WithAttachCloseWriteGrace(0)
		opt(rt)
		assert.Equal(t, 1*time.Millisecond, rt.attachCloseWriteGrace)
	})
}

func TestUnit_Docker_NewDockerRuntimeWithOptions_Error(t *testing.T) {
	t.Run("empty socket path", func(t *testing.T) {
		_, err := NewDockerRuntimeWithOptions("", "docker", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty socket path")
	})
}
