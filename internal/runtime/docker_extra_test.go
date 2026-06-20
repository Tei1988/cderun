package runtime

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.True(t, IsRetryablePullError(errors.New("unexpected eof")))
	assert.True(t, IsRetryablePullError(errors.New("token expired")))
	assert.False(t, IsRetryablePullError(errors.New("unauthorized")))
	assert.False(t, IsRetryablePullError(errors.New("other error")))
}
