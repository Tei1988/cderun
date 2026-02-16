package command

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Command_Signals_GetSignalName(t *testing.T) {
	setupNoOverlay(t)
	assert.Equal(t, "SIGINT", getSignalName(os.Interrupt))
	assert.Equal(t, "SIGINT", getSignalName(syscall.SIGINT))

	// Platform-specific signals
	// On Unix, SIGTERM is supported. On Windows, it might not be.
	// We check for some standard ones.

	t.Run("Other signals", func(t *testing.T) {
		// Just ensure it doesn't panic and returns a string
		assert.NotEmpty(t, getSignalName(syscall.Signal(99)))
	})
}
