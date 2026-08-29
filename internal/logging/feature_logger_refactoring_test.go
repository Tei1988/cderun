package logging

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Logging_RefactoredSanitizeLogString(t *testing.T) {
	t.Parallel()

	t.Run("isControlByte and hasControlByte checks", func(t *testing.T) {
		assert.True(t, isControlByte('\n'))
		assert.True(t, isControlByte('\r'))
		assert.True(t, isControlByte(127))
		assert.False(t, isControlByte('\t'))
		assert.False(t, isControlByte('A'))

		assert.True(t, hasControlByte("hello\nworld"))
		assert.False(t, hasControlByte("hello\tworld"))
		assert.False(t, hasControlByte("hello world"))
	})

	t.Run("SanitizeLogString stack vs builder path boundary", func(t *testing.T) {
		shortInput := "short\x01test\nmessage"
		longInput := strings.Repeat("x", 260) + "\x01" + strings.Repeat("y", 10)

		sanitizedShort := SanitizeLogString(shortInput)
		sanitizedLong := SanitizeLogString(longInput)

		assert.Equal(t, "short\\x01test\\x0amessage", sanitizedShort)
		assert.Contains(t, sanitizedLong, "\\x01")
		assert.Equal(t, 271+3, len(sanitizedLong)) // 271 bytes original; \x01 expands to \x01 (4 chars instead of 1 byte, +3 length = 274)
	})
}

func TestUnit_Logging_RefactoredWriteFormattedLog(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := NewLogger()
	logger.SetOutput(buf)
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

	t.Run("text output path in writeFormattedLog", func(t *testing.T) {
		buf.Reset()
		_ = logger.Init("info", "text", false)
		logger.writeFormattedLog(InfoLevel, "test formatted text", now)
		assert.Equal(t, "[INFO] test formatted text\n", buf.String())
	})

	t.Run("json output path in writeFormattedLog", func(t *testing.T) {
		buf.Reset()
		_ = logger.Init("info", "json", false)
		logger.writeFormattedLog(InfoLevel, "test formatted json", now)
		assert.Contains(t, buf.String(), `"level":"info"`)
		assert.Contains(t, buf.String(), `"msg":"test formatted json"`)
	})
}
