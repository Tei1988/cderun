package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_ResolveEnvValues_ControlCharacterBoundaryValidation(t *testing.T) {
	fs := RealFileSystem{}

	t.Run("accepted whitespace control characters (newline, carriage return, tab)", func(t *testing.T) {
		input := []string{"MULTILINE_VAR=line1\nline2\rline3\tvalue"}
		res, err := resolveEnvValues(input, nil, false, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, input, res)
	})

	t.Run("accepted printable UTF-8 non-ASCII characters", func(t *testing.T) {
		input := []string{"UNICODE_VAR=HELLO_世界_TEST_123"}
		res, err := resolveEnvValues(input, nil, false, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, input, res)
	})

	t.Run("rejected ASCII control characters with exact byte position and error message", func(t *testing.T) {
		// Control char \x01 at byte position 3 ('A'=0, 'B'=1, 'C'=2, \x01=3)
		input := []string{"BAD_VAR=ABC\x01DEF"}
		res, err := resolveEnvValues(input, nil, false, nil, fs)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.EqualError(t, err, "security validation failed for env[0] (value): invalid control character '\\x01' at position 3")
	})

	t.Run("rejected ASCII DEL character (0x7f) with exact byte position and error message", func(t *testing.T) {
		// DEL \x7f at byte position 2 ('X'=0, 'Y'=1, \x7f=2)
		input := []string{"DEL_VAR=XY\x7fZ"}
		res, err := resolveEnvValues(input, nil, false, nil, fs)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.EqualError(t, err, "security validation failed for env[0] (value): invalid control character '\\x7f' at position 2")
	})

	t.Run("rejected C1 Unicode control characters with exact byte position", func(t *testing.T) {
		// U+0080 (Control C1) at byte position 4 ('P'=0, 'A'=1, 'S'=2, 'S'=3, U+0080=4)
		input := []string{"C1_VAR=PASS\u0080FAIL"}
		res, err := resolveEnvValues(input, nil, false, nil, fs)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.EqualError(t, err, "security validation failed for env[0] (value): invalid control character '\\u0080' at position 4")
	})

	t.Run("byte offset position tracking with preceding multibyte UTF-8 characters", func(t *testing.T) {
		// 'A' (1 byte, pos 0) + '世' (3 bytes, pos 1..3) + \x01 (pos 4)
		input := []string{"MULTIBYTE_VAR=A世\x01B"}
		res, err := resolveEnvValues(input, nil, false, nil, fs)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.EqualError(t, err, "security validation failed for env[0] (value): invalid control character '\\x01' at position 4")
	})
}
