package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ShmSizeOption(t *testing.T) {
	t.Parallel()

	t.Run("resolve valid shm-size from CLI", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		cli := &CLIOptions{Image: ptr("alpine"), ShmSize: ptr("512m")}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "512m", res.ShmSize)
	})

	t.Run("resolve from cderun P1 override with priority", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		cli := &CLIOptions{
			Image:         ptr("alpine"),
			ShmSize:       ptr("128m"), // P2
			CderunShmSize: ptr("1g"),   // P1
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "1g", res.ShmSize)
	})

	t.Run("validation failures", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}

		// Invalid size format
		cliInvalid := &CLIOptions{Image: ptr("alpine"), ShmSize: ptr("invalid")}
		_, err := ResolveWithFS("sh", cliInvalid, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")

		// Negative size
		cliNegative := &CLIOptions{Image: ptr("alpine"), ShmSize: ptr("-100")}
		_, err = ResolveWithFS("sh", cliNegative, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")

		// Security characters violation
		cliSecurity := &CLIOptions{Image: ptr("alpine"), ShmSize: ptr("256m\x00")}
		_, err = ResolveWithFS("sh", cliSecurity, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed")
	})
}
