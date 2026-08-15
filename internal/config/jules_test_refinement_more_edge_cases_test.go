package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docs/features/sensitive-data-protection.md: Environment Variable Masking & Pattern Matching
// Verifies ASCII folding logic and case-insensitive behavior of masking with mixed case, special characters, and Unicode variations.
func TestUnit_Config_Masking_ASCIIFoldingAndUnicode(t *testing.T) {
	t.Parallel()

	t.Run("equalFoldASCII behavior with mixed cases and Unicode", func(t *testing.T) {
		// Verify equalFoldASCII through MaskSensitiveEnv exact matching
		got := MaskSensitiveEnv("My_SeCrEt_KeY", "sensitive_val", []string{"my_secret_key"})
		assert.Equal(t, "[REDACTED]", got)

		got2 := MaskSensitiveEnv("My_SeCrEt_KeY", "sensitive_val", []string{"MY_SECRET_KEY"})
		assert.Equal(t, "[REDACTED]", got2)

		// Non-ASCII character match should fallback to standard Upper comparison
		got3 := MaskSensitiveEnv("SÉCRÊT", "value", []string{"sérêt"})
		assert.Equal(t, "value", got3, "SÉCRÊT should not match sérêt")

		got4 := MaskSensitiveEnv("SÉCRÊT_KEY", "value", []string{"sÉcrÊt_key"})
		assert.Equal(t, "[REDACTED]", got4, "case-insensitive Unicode fallback match should succeed")
	})

	t.Run("pattern matching with suffix, prefix, and substring ASCII fast path", func(t *testing.T) {
		// Suffix matching (isSuffix)
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("AUTH_TOKEN", "123", []string{"*_TOKEN"}))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("auth_token", "123", []string{"*_token"}))
		assert.Equal(t, "123", MaskSensitiveEnv("TOKEN_AUTH", "123", []string{"*_TOKEN"}))

		// Prefix matching (isPrefix)
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("API_KEY_MAIN", "secret", []string{"API_*"}))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("api_key_main", "secret", []string{"api_*"}))
		assert.Equal(t, "secret", MaskSensitiveEnv("MY_API_KEY", "secret", []string{"API_*"}))

		// Substring matching (isSubstr)
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("MY_PASSWORD_VAR", "pass", []string{"*_PASSWORD_*"}))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("my_password_var", "pass", []string{"*_password_*"}))
		assert.Equal(t, "pass", MaskSensitiveEnv("PASSWORD", "pass", []string{"*_PASSWORD_*"}))
	})

	t.Run("empty value is preserved in MaskSensitiveEnvList", func(t *testing.T) {
		envList := []string{"SECRET_KEY=", "PUBLIC_KEY=123"}
		got := MaskSensitiveEnvList(envList, []string{"SECRET_KEY"})
		assert.Equal(t, []string{"SECRET_KEY=", "PUBLIC_KEY=123"}, got, "empty values must remain unmasked/unmodified")
	})
}

// docs/features/sensitive-data-protection.md: Fail-Closed Logic
// Verifies fail-closed behavior on invalid glob pattern during pattern matching.
func TestUnit_Config_Masking_FailClosedInvalidGlob(t *testing.T) {
	t.Parallel()

	// An invalid glob pattern (e.g., "[") should fail gracefully and trigger REDACTED (fail-closed)
	got := MaskSensitiveEnv("ANY_KEY", "some_secret", []string{"["})
	assert.Equal(t, "[REDACTED]", got, "invalid glob must trigger REDACTED due to fail-closed logic")
}

// docs/features/value-resolution.md: Security Hardening and Constraints - Null-Byte Injections Guard
// Verifies validatePathChars correctly rejects strings containing ASCII control characters and null bytes.
func TestUnit_Config_ValidatePathChars_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"normal alphabetic path", "/usr/bin/go", false},
		{"path with backspace", "/usr/bin/go\x08", true},
		{"path with escape character", "/usr/bin/go\x1b", true},
		{"path with vertical tab", "/usr/bin/go\x0b", true},
		{"path with null byte at the beginning", "\x00/usr/bin/go", true},
		{"path with null byte in the middle", "/usr/\x00bin/go", true},
		{"path with delete", "path/with\x7fdelete", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathChars(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// docs/features/value-resolution.md: Anchor Boundary Validation
// Verifies behavior when both PWD or HOME directories resolve to empty, unset, or error-prone strings.
func TestUnit_Config_Resolver_BoundaryResolution_DoubleEmptyErrors(t *testing.T) {
	t.Parallel()

	t.Run("both HOME and PWD are empty on ResolvePath", func(t *testing.T) {
		mfsEmpty := &MockFileSystem{
			WD:      "",
			HomeDir: "",
		}

		r, err := NewExpressionResolverWithFS(nil, mfsEmpty)
		require.NoError(t, err)

		_, err = ResolvePath("{{HOME}}/file", "", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchor path is empty")

		_, err = ResolvePath("{{PWD}}/file", "", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchor path is empty")
	})

	t.Run("working directory lookup error disables relative path resolution", func(t *testing.T) {
		mfsError := &MockFileSystem{
			WD:      "/work",
			HomeDir: "/home/user",
			AbsErr:  errors.New("simulated lookup error"),
		}

		// Initialize expression resolver with Level > 0 to force Abs check
		r, err := NewExpressionResolverWithFS(&HostContext{Level: 1}, mfsError)
		require.NoError(t, err)

		_, err = ResolvePath("./some_relative", "", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated lookup error")
	})
}
