package logging

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Logging_ParseLevelAndLowerString_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("ParseLevel edge cases", func(t *testing.T) {
		assert.Equal(t, InfoLevel, ParseLevel(""))
		assert.Equal(t, InfoLevel, ParseLevel("a"))
		assert.Equal(t, InfoLevel, ParseLevel("foo"))
		assert.Equal(t, InfoLevel, ParseLevel("invalid_long_level_string"))

		// 4-char cases
		assert.Equal(t, WarnLevel, ParseLevel("warn"))
		assert.Equal(t, WarnLevel, ParseLevel("WARN"))
		assert.Equal(t, InfoLevel, ParseLevel("info"))
		assert.Equal(t, InfoLevel, ParseLevel("INFO"))

		// 5-char cases
		assert.Equal(t, ErrorLevel, ParseLevel("error"))
		assert.Equal(t, ErrorLevel, ParseLevel("ERROR"))
		assert.Equal(t, DebugLevel, ParseLevel("debug"))
		assert.Equal(t, DebugLevel, ParseLevel("DEBUG"))
		assert.Equal(t, TraceLevel, ParseLevel("trace"))
		assert.Equal(t, TraceLevel, ParseLevel("TRACE"))

		// 7-char cases
		assert.Equal(t, WarnLevel, ParseLevel("warning"))
		assert.Equal(t, WarnLevel, ParseLevel("WARNING"))
	})

	t.Run("Level.LowerString edge cases", func(t *testing.T) {
		assert.Equal(t, "error", ErrorLevel.LowerString())
		assert.Equal(t, "warn", WarnLevel.LowerString())
		assert.Equal(t, "info", InfoLevel.LowerString())
		assert.Equal(t, "debug", DebugLevel.LowerString())
		assert.Equal(t, "trace", TraceLevel.LowerString())

		assert.Equal(t, "info", Level(-1).LowerString())
		assert.Equal(t, "info", Level(100).LowerString())
	})
}

func TestUnit_Logging_LoggerAccessorsAndEnabled(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := NewLogger()
	logger.SetOutput(buf)
	err := logger.Init("debug", "json", true)
	require.NoError(t, err)

	assert.Equal(t, "json", logger.GetFormat())
	assert.True(t, logger.GetTimestamp())
	assert.Equal(t, buf, logger.GetWriter())
	assert.Equal(t, DebugLevel, logger.GetLevel())

	assert.True(t, logger.Enabled(ErrorLevel))
	assert.True(t, logger.Enabled(WarnLevel))
	assert.True(t, logger.Enabled(InfoLevel))
	assert.True(t, logger.Enabled(DebugLevel))
	assert.False(t, logger.Enabled(TraceLevel))

	assert.True(t, logger.DebugEnabled())
	assert.False(t, logger.TraceEnabled())

	logger.SetLevel(TraceLevel)
	assert.True(t, logger.TraceEnabled())
}

func TestUnit_Logging_SanitizeLogString_BoundaryEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("various ASCII control chars in short input", func(t *testing.T) {
		input := "nul:\x00 bel:\x07 esc:\x1b del:\x7f tab:\t"
		expected := "nul:\\x00 bel:\\x07 esc:\\x1b del:\\x7f tab:\t"
		assert.Equal(t, expected, SanitizeLogString(input))
	})

	t.Run("various ASCII control chars in long input (>256 bytes)", func(t *testing.T) {
		prefix := strings.Repeat("x", 250)
		suffix := strings.Repeat("y", 20)
		input := prefix + "\x00\x07\x1b\x7f\t" + suffix
		expected := prefix + "\\x00\\x07\\x1b\\x7f\t" + suffix
		assert.Equal(t, expected, SanitizeLogString(input))
	})
}

func TestUnit_Logging_ConcurrentAccess(t *testing.T) {
	logger := NewLogger()
	logger.SetOutput(io.Discard)

	var wg sync.WaitGroup
	const numGoroutines = 10
	const opsPerGoroutine = 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				if (id+j)%2 == 0 {
					_ = logger.Init("debug", "text", true)
				} else {
					_ = logger.Init("info", "json", false)
				}

				logger.Info("info msg %d %d", id, j)
				logger.Debug("debug msg %d %d", id, j)
				_ = logger.GetFormat()
				_ = logger.GetTimestamp()
				_ = logger.GetLevel()
				_ = logger.GetWriter()
			}
		}(i)
	}

	wg.Wait()
}
