package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ScanAnchors_FastPathAndEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []anchorRange
	}{
		{
			name:     "No braces",
			input:    "plain_text_without_expressions",
			expected: nil,
		},
		{
			name:     "Prefix single brace before double brace",
			input:    "prefix { single brace {{HOME}} suffix",
			expected: []anchorRange{{start: 22, end: 30}},
		},
		{
			name:     "Multiple anchors",
			input:    "head {{HOME}} mid {{PWD}} tail",
			expected: []anchorRange{{start: 5, end: 13}, {start: 18, end: 25}},
		},
		{
			name:     "Single brace only",
			input:    "only { single } braces",
			expected: nil,
		},
		{
			name:     "Unclosed double brace opener",
			input:    "unclosed {{HOME without closer",
			expected: nil,
		},
		{
			name:     "Nested braces",
			input:    "nested {{env:{{KEY}}}} value",
			expected: []anchorRange{{start: 7, end: 22}},
		},
		{
			name:     "Escaped quad braces",
			input:    "escaped {{{{literal}}}} anchor",
			expected: []anchorRange{{start: 8, end: 23}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf [32]anchorRange
			got := scanAnchors(tt.input, buf[:0])
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestUnit_Config_MaskSensitiveEnv_PatternOptimizations(t *testing.T) {
	t.Parallel()

	t.Run("Nil patterns masks all non-empty values", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("API_KEY", "secret123", nil))
		assert.Equal(t, "", MaskSensitiveEnv("API_KEY", "", nil))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("API_KEY", "[REDACTED]", nil))
	})

	t.Run("Empty patterns slice masks nothing", func(t *testing.T) {
		t.Parallel()
		emptyPatterns := []string{}
		assert.Equal(t, "secret123", MaskSensitiveEnv("API_KEY", "secret123", emptyPatterns))
	})

	t.Run("Glob, suffix, prefix, and exact pattern matching", func(t *testing.T) {
		t.Parallel()
		patterns := []string{"*TOKEN*", "SECRET_*", "*_KEY", "PASSWORD"}

		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("MY_TOKEN_VAL", "secret1", patterns))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("SECRET_AUTH", "secret2", patterns))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("USER_KEY", "secret3", patterns))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("PASSWORD", "secret4", patterns))
		assert.Equal(t, "public_val", MaskSensitiveEnv("PUBLIC_VAR", "public_val", patterns))
	})
}

func TestUnit_Config_MaskSensitiveEnvList_FastPathFirstRedaction(t *testing.T) {
	t.Parallel()

	patterns := []string{"*PASS*", "TOKEN"}

	t.Run("Redacts first sensitive variable and subsequent matches", func(t *testing.T) {
		t.Parallel()
		env := []string{
			"USER=alice",
			"MY_PASSWORD=secret",
			"PATH=/usr/bin",
			"TOKEN=xyz123",
		}

		res := MaskSensitiveEnvList(env, patterns)
		require.Len(t, res, 4)
		assert.Equal(t, "USER=alice", res[0])
		assert.Equal(t, "MY_PASSWORD=[REDACTED]", res[1])
		assert.Equal(t, "PATH=/usr/bin", res[2])
		assert.Equal(t, "TOKEN=[REDACTED]", res[3])
	})

	t.Run("Nil patterns redacts all variables with values", func(t *testing.T) {
		t.Parallel()
		env := []string{
			"USER=alice",
			"EMPTY=",
			"TOKEN=xyz123",
		}

		res := MaskSensitiveEnvList(env, nil)
		require.Len(t, res, 3)
		assert.Equal(t, "USER=[REDACTED]", res[0])
		assert.Equal(t, "EMPTY=", res[1])
		assert.Equal(t, "TOKEN=[REDACTED]", res[2])
	})

	t.Run("Empty list and no matches return original slice", func(t *testing.T) {
		t.Parallel()
		env := []string{"FOO=bar", "BAZ=qux"}
		res := MaskSensitiveEnvList(env, patterns)
		assert.Equal(t, env, res)

		var emptyEnv []string
		assert.Nil(t, MaskSensitiveEnvList(emptyEnv, patterns))
	})
}
