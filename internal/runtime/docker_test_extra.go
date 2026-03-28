package runtime

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnitDockerRetryablePullErrorExhaustive(t *testing.T) {
	assert.False(t, isRetryablePullError(nil))
	assert.True(t, isRetryablePullError(errors.New("toomanyrequests")))
	assert.True(t, isRetryablePullError(errors.New("rate exceeded")))
	assert.True(t, isRetryablePullError(errors.New("rate limit")))
	assert.True(t, isRetryablePullError(errors.New("data limit exceeded")))
	assert.True(t, isRetryablePullError(errors.New("i/o timeout")))
	assert.True(t, isRetryablePullError(errors.New("connection refused")))
	assert.True(t, isRetryablePullError(errors.New("connection reset")))
	assert.True(t, isRetryablePullError(io.EOF))
	assert.True(t, isRetryablePullError(errors.New("token expired")))
	assert.False(t, isRetryablePullError(errors.New("unauthorized")))
	assert.False(t, isRetryablePullError(errors.New("other error")))
}
