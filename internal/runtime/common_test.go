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

func TestUnit_Common_IsRetryablePullError_Typed(t *testing.T) {
	t.Run("DNSError - NotFound", func(t *testing.T) {
		err := &net.DNSError{IsNotFound: true}
		assert.False(t, IsRetryablePullError(err))
	})

	t.Run("DNSError - Temporary", func(t *testing.T) {
		err := &net.DNSError{IsTemporary: true}
		assert.True(t, IsRetryablePullError(err))
	})

	t.Run("NetError - Timeout", func(t *testing.T) {
		err := &timeoutErr{}
		assert.True(t, IsRetryablePullError(err))
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
}
