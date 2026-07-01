package runtime

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

type timeoutErr struct{}

func (e *timeoutErr) Error() string   { return "timeout" }
func (e *timeoutErr) Timeout() bool   { return true }
func (e *timeoutErr) Temporary() bool { return true }

type nonTimeoutNetErr struct{}

func (e *nonTimeoutNetErr) Error() string   { return "net error" }
func (e *nonTimeoutNetErr) Timeout() bool   { return false }
func (e *nonTimeoutNetErr) Temporary() bool { return false }

type cancelledErr struct{}

func (e *cancelledErr) Error() string { return "timeout" }
func (e *cancelledErr) Cancelled()    {}

type unavailableErr struct{}

func (e *unavailableErr) Error() string { return "unavailable" }
func (e *unavailableErr) Unavailable()  {}

type deadlineExceededErr struct{}

func (e *deadlineExceededErr) Error() string { return "deadline exceeded" }
func (e *deadlineExceededErr) DeadlineExceeded()  {}

func TestUnit_Common_IsTemporaryAuthError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsTemporaryAuthError(nil))
	})

	t.Run("token expired", func(t *testing.T) {
		assert.True(t, IsTemporaryAuthError(errors.New("token expired")))
	})

	t.Run("other error", func(t *testing.T) {
		assert.False(t, IsTemporaryAuthError(errors.New("fatal auth error")))
	})
}

func TestUnit_Common_IsRetryablePullError_Typed(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsRetryablePullError(nil))
	})

	t.Run("DNSError - NotFound", func(t *testing.T) {
		err := &net.DNSError{IsNotFound: true}
		assert.False(t, IsRetryablePullError(err))
	})

	t.Run("DNSError - Other", func(t *testing.T) {
		err := &net.DNSError{IsNotFound: false, IsTemporary: false}
		assert.True(t, IsRetryablePullError(err))
	})

	t.Run("NetError - Timeout", func(t *testing.T) {
		err := &timeoutErr{}
		assert.True(t, IsRetryablePullError(err))
	})

	t.Run("NetError - Non-Timeout", func(t *testing.T) {
		err := &nonTimeoutNetErr{}
		assert.False(t, IsRetryablePullError(err))
	})

	t.Run("String matching still works", func(t *testing.T) {
		assert.True(t, IsRetryablePullError(errors.New("connection refused")))
		assert.True(t, IsRetryablePullError(errors.New("toomanyrequests")))
	})

	t.Run("Wrapped errors", func(t *testing.T) {
		err := &net.DNSError{IsNotFound: true}
		wrappedProperly := fmt.Errorf("wrap: %w", err)
		assert.False(t, IsRetryablePullError(wrappedProperly))

		timeout := &timeoutErr{}
		wrappedTimeout := fmt.Errorf("wrap: %w", timeout)
		assert.True(t, IsRetryablePullError(wrappedTimeout))
	})

	t.Run("errdefs cases", func(t *testing.T) {
		assert.False(t, IsRetryablePullError(&cancelledErr{}))
		assert.True(t, IsRetryablePullError(&unavailableErr{}))
		assert.True(t, IsRetryablePullError(&deadlineExceededErr{}))
	})
}
