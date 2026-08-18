package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_SecurityControlChars_And_UlimitExpressions(t *testing.T) {
	t.Run("validatePathChars rejects C1 control characters and invalid UTF-8", func(t *testing.T) {
		// C0 control character: \x07 (BEL)
		err := validatePathChars("test\x07path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")

		// C1 control characters: U+0080, U+0085 (NEL), U+009F
		errC1 := validatePathChars("test\u0080path")
		require.Error(t, errC1)
		assert.Contains(t, errC1.Error(), "invalid character in path or configuration")

		errC1_NEL := validatePathChars("test\u0085path")
		require.Error(t, errC1_NEL)
		assert.Contains(t, errC1_NEL.Error(), "invalid character in path or configuration")

		errC1_9F := validatePathChars("test\u009Fpath")
		require.Error(t, errC1_9F)
		assert.Contains(t, errC1_9F.Error(), "invalid character in path or configuration")

		// Invalid UTF-8 sequence
		errInvalidUTF8 := validatePathChars("test\xffpath")
		require.Error(t, errInvalidUTF8)
		assert.Contains(t, errInvalidUTF8.Error(), "invalid UTF-8 encoding")

		// Valid string passes
		assert.NoError(t, validatePathChars("valid/path-123_test"))
	})

	t.Run("resolveUlimits with expression resolution", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"LIMIT_SOFT": "1024",
				"LIMIT_HARD": "2048",
			},
		}

		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		raws := []string{"nofile={{env:LIMIT_SOFT}}:{{env:LIMIT_HARD}}"}
		ulimits, err := resolveUlimits(raws, nil, "", nil, nil, expr, mfs)
		require.NoError(t, err)
		require.Len(t, ulimits, 1)
		assert.Equal(t, "nofile", ulimits[0].Name)
		assert.Equal(t, int64(1024), ulimits[0].Soft)
		assert.Equal(t, int64(2048), ulimits[0].Hard)
	})

	t.Run("resolveUlimits with expression error fails safely", func(t *testing.T) {
		mfs := &MockFileSystem{}
		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		raws := []string{"nofile={{env:NONEXISTENT_VAR_FOR_ULIMIT}}"}
		_, err = resolveUlimits(raws, nil, "", nil, nil, expr, mfs)
		require.Error(t, err)
	})
}
