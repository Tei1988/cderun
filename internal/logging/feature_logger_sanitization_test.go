package logging

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Logger_SanitizeLogString_StackAndHeapBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Clean string without control bytes returns original reference", func(t *testing.T) {
		clean := "Hello, World! Standard log message with tabs\t123"
		sanitized := SanitizeLogString(clean)
		assert.Equal(t, clean, sanitized)
	})

	t.Run("Stack buffer path boundary exactly 512 bytes with control chars", func(t *testing.T) {
		// Create a string of length 512 containing control chars (\n, \r)
		prefix := strings.Repeat("a", 510)
		input := prefix + "\n\r"
		require.Len(t, input, 512)

		sanitized := SanitizeLogString(input)
		expectedSuffix := "\\x0a\\x0d"
		assert.True(t, strings.HasSuffix(sanitized, expectedSuffix))
		assert.True(t, strings.HasPrefix(sanitized, prefix))
	})

	t.Run("Heap builder path boundary 513 bytes with control chars", func(t *testing.T) {
		// Create a string of length 513 containing control chars
		prefix := strings.Repeat("b", 511)
		input := prefix + "\n\r"
		require.Len(t, input, 513)

		sanitized := SanitizeLogString(input)
		expectedSuffix := "\\x0a\\x0d"
		assert.True(t, strings.HasSuffix(sanitized, expectedSuffix))
		assert.True(t, strings.HasPrefix(sanitized, prefix))
	})

	t.Run("Preserves tabs while escaping null byte and DEL 0x7f", func(t *testing.T) {
		input := "tab\tbyte\x00del\x7f"
		sanitized := SanitizeLogString(input)
		assert.Equal(t, "tab\tbyte\\x00del\\x7f", sanitized)
	})
}

func TestUnit_Logger_FormattingOutputModes(t *testing.T) {
	t.Parallel()

	t.Run("Text format output contains timestamp and level tag", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger()
		logger.SetOutput(&buf)
		require.NoError(t, logger.Init("info", "text", true))

		now := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)
		logger.writeFormattedLog(InfoLevel, "Execution complete", now)

		out := buf.String()
		assert.Contains(t, out, "2026-05-10 14:30:00 [INFO] Execution complete\n")
	})

	t.Run("JSON format output contains JSON keys and level lower name", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger()
		logger.SetOutput(&buf)
		require.NoError(t, logger.Init("warn", "json", true))

		now := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)
		logger.writeFormattedLog(WarnLevel, "Resource usage high", now)

		out := buf.String()
		assert.Contains(t, out, `"level":"warn"`)
		assert.Contains(t, out, `"msg":"Resource usage high"`)
		assert.Contains(t, out, `"time":"2026-05-10T14:30:00Z"`)
	})

	t.Run("Unrecognized or empty format defaults to text format", func(t *testing.T) {
		logger := NewLogger()
		require.NoError(t, logger.Init("info", "  invalid_format  ", true))
		assert.Equal(t, "text", logger.GetFormat())
	})
}
