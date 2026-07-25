package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_Resolver_MemoryParsingBorderCases verifies parsing of extremely
// large sizes, invalid units, and border cases.
func TestUnit_Config_Resolver_MemoryParsingBorderCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{}

	t.Run("1024TiB represents 1PiB and parses successfully", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "1024TiB",
			MemorySet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		// 1024 TiB = 1024 * 1024^4 = 1125899906842624 bytes
		assert.Equal(t, int64(1125899906842624), res.Memory)
	})

	t.Run("1EiB is rejected due to units.RAMInBytes limitations", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "1EiB",
			MemorySet: true,
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "memory", cfgErr.Field)
	})
}
