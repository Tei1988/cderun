package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ResolveOptionFieldInfo(t *testing.T) {
	fieldOnce.Do(initFieldInfo)

	t.Run("valid option field info resolution", func(t *testing.T) {
		expectedInfo := fieldInfo["network"]
		res, err := resolveOptionFieldInfo("network", expectedInfo)
		require.NoError(t, err)
		assert.Equal(t, expectedInfo, res)
	})

	t.Run("zero info falls back to fieldInfo map lookup", func(t *testing.T) {
		zero := optionFields{targetIdx: 0, p1ValIdx: 0, p2ValIdx: 0}
		res, err := resolveOptionFieldInfo("network", zero)
		require.NoError(t, err)
		expectedInfo := fieldInfo["network"]
		assert.Equal(t, expectedInfo, res)
	})

	t.Run("unknown option name returns error when fallback lookup fails", func(t *testing.T) {
		zero := optionFields{targetIdx: 0, p1ValIdx: 0, p2ValIdx: 0}
		_, err := resolveOptionFieldInfo("nonexistent_option_name_xyz", zero)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option")
	})
}

func TestUnit_Config_IsDriftOk(t *testing.T) {
	fieldOnce.Do(initFieldInfo)

	t.Run("matches expectedFieldIndices", func(t *testing.T) {
		expected, ok := expectedFieldIndices["network"]
		require.True(t, ok, "expectedFieldIndices should contain 'network' entry")
		assert.True(t, isDriftOk("network", expected))
	})

	t.Run("drift mismatch returns false", func(t *testing.T) {
		mismatched := optionFields{targetIdx: 999, p1ValIdx: -1, p2ValIdx: -1}
		assert.False(t, isDriftOk("network", mismatched))
	})
}
