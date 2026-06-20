package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Errors_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("RegistryMismatchError", func(t *testing.T) {
		err := &RegistryMismatchError{
			ExpectedRegistry: "expected.com/repo",
			ActualRegistry:   "actual.com/repo"}
		assert.Equal(t, "container registry mismatch: expected \"expected.com/repo\", got \"actual.com/repo\"", err.Error())
	})

	t.Run("ImageNotFoundError", func(t *testing.T) {
		err := &ImageNotFoundError{
			Tool: "my-tool"}
		assert.Equal(t, "no image mapping found for tool: \"my-tool\"", err.Error())
	})

	t.Run("RuntimeInitError", func(t *testing.T) {
		baseErr := errors.New("base error")
		err := &RuntimeInitError{
			Runtime: "docker",
			Err:     baseErr}
		assert.Equal(t, "failed to initialize runtime \"docker\": base error", err.Error())
		require.Equal(t, baseErr, err.Unwrap())

		errNoBase := &RuntimeInitError{
			Runtime: "podman"}
		assert.Equal(t, "failed to initialize runtime \"podman\"", errNoBase.Error())
		require.NoError(t, errNoBase.Unwrap())
	})

	t.Run("InvalidConfigError", func(t *testing.T) {
		baseErr := errors.New("invalid format")
		err := &InvalidConfigError{
			Field: "memory",
			Value: "invalid",
			Err:   baseErr}
		// Error() already covered but let's be sure
		assert.Equal(t, "invalid memory value \"invalid\": invalid format", err.Error())
		require.Equal(t, baseErr, err.Unwrap())
	})
}
