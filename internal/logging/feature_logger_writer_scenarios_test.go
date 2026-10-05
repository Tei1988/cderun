package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingWriter struct {
	err error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	return 0, errors.New("write failure")
}

func TestUnit_Logger_InitAndLevelParsingScenarios(t *testing.T) {
	t.Parallel()

	t.Run("level parsing normalization", func(t *testing.T) {
		assert.Equal(t, WarnLevel, ParseLevel("warning"))
		assert.Equal(t, WarnLevel, ParseLevel("WARN"))
		assert.Equal(t, DebugLevel, ParseLevel("dEbUg"))
		assert.Equal(t, ErrorLevel, ParseLevel("ERROR"))
		assert.Equal(t, TraceLevel, ParseLevel("TRACE"))
		assert.Equal(t, InfoLevel, ParseLevel(""))
		assert.Equal(t, InfoLevel, ParseLevel("invalid"))
		// Untrimmed strings fall through len switch to default InfoLevel
		assert.Equal(t, InfoLevel, ParseLevel("  WARNING  "))
	})

	t.Run("init formatting edge cases", func(t *testing.T) {
		l := NewLogger()
		err := l.Init("debug", "  JSON  ", false)
		require.NoError(t, err)
		assert.Equal(t, "json", l.GetFormat())
		assert.Equal(t, DebugLevel, l.GetLevel())
		assert.False(t, l.GetTimestamp())

		err = l.Init("info", "yaml", true)
		require.NoError(t, err)
		assert.Equal(t, "text", l.GetFormat())
		assert.True(t, l.GetTimestamp())
	})
}

func TestUnit_Logger_LogFormattingAndArgs(t *testing.T) {
	t.Parallel()

	t.Run("no args vs formatted args in text mode", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := NewLogger()
		l.SetOutput(buf)
		_ = l.Init("trace", "text", false)

		l.Info("plain string without format percent")
		assert.Equal(t, "[INFO] plain string without format percent\n", buf.String())

		buf.Reset()
		l.Debug("formatted string %s %d %v", "arg1", 42, true)
		assert.Equal(t, "[DEBUG] formatted string arg1 42 true\n", buf.String())
	})

	t.Run("timestamp rendering in text and json", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := NewLogger()
		l.SetOutput(buf)
		fixedTime := time.Date(2026, 9, 10, 15, 30, 45, 0, time.UTC)

		// Text mode with timestamp
		_ = l.Init("info", "text", true)
		l.writeFormattedLog(InfoLevel, "test text timestamp", fixedTime)
		assert.Equal(t, "2026-09-10 15:30:45 [INFO] test text timestamp\n", buf.String())

		// JSON mode with timestamp
		buf.Reset()
		_ = l.Init("info", "json", true)
		l.writeFormattedLog(InfoLevel, "test json timestamp", fixedTime)

		var res map[string]string
		err := json.Unmarshal(buf.Bytes(), &res)
		require.NoError(t, err)
		assert.Equal(t, "2026-09-10T15:30:45Z", res["time"])
		assert.Equal(t, "info", res["level"])
		assert.Equal(t, "test json timestamp", res["msg"])
	})
}

func TestUnit_Logger_SanitizeLogString_LoopAndBoundaryCases(t *testing.T) {
	t.Parallel()

	t.Run("8-byte chunk scanning boundary checks", func(t *testing.T) {
		// Test control byte placed at positions 0..7 of an 8-byte chunk
		for pos := 0; pos < 8; pos++ {
			chunk := []byte("12345678")
			chunk[pos] = 0x01 // Control byte
			s := string(chunk)
			assert.True(t, hasControlByte(s), "expected position %d to trigger control byte check", pos)
			sanitized := SanitizeLogString(s)
			assert.Contains(t, sanitized, "\\x01")
		}

		// Test control byte in second 8-byte chunk (12th byte)
		str2 := "01234567890\x02123"
		assert.True(t, hasControlByte(str2))
		assert.Contains(t, SanitizeLogString(str2), "\\x02")
	})

	t.Run("512 stack vs 513 builder path with control characters", func(t *testing.T) {
		// Exact 512 bytes with control character
		exact512 := strings.Repeat("a", 510) + "\x03\t"
		assert.Len(t, exact512, 512)
		sanitized512 := SanitizeLogString(exact512)
		assert.True(t, strings.HasPrefix(sanitized512, strings.Repeat("a", 510)))
		assert.True(t, strings.HasSuffix(sanitized512, "\\x03\t"))

		// 513 bytes with control character
		exact513 := strings.Repeat("b", 511) + "\x04\t"
		assert.Len(t, exact513, 513)
		sanitized513 := SanitizeLogString(exact513)
		assert.True(t, strings.HasPrefix(sanitized513, strings.Repeat("b", 511)))
		assert.True(t, strings.HasSuffix(sanitized513, "\\x04\t"))
	})
}

func TestUnit_Logger_FailingWriterResilience(t *testing.T) {
	t.Parallel()

	l := NewLogger()
	l.SetOutput(&failingWriter{err: errors.New("disk write failure")})

	assert.NotPanics(t, func() {
		_ = l.Init("trace", "text", true)
		l.Error("test error message")
		l.Warn("test warn message")

		_ = l.Init("trace", "json", true)
		l.Info("test info message")
		l.Debug("test debug message")
		l.Trace("test trace message")
	})
}

func TestUnit_Logger_GlobalWrappersAndThresholds(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf)
	err := Init("warn", "text", false)
	require.NoError(t, err)

	assert.True(t, Enabled(ErrorLevel))
	assert.True(t, Enabled(WarnLevel))
	assert.False(t, Enabled(InfoLevel))
	assert.False(t, DebugEnabled())
	assert.False(t, TraceEnabled())

	// Under warn level, info/debug/trace should not produce output
	Info("should be ignored")
	Debug("should be ignored")
	Trace("should be ignored")
	assert.Empty(t, buf.String())

	// Warn and Error should produce output
	Warn("warn log")
	assert.Contains(t, buf.String(), "[WARN] warn log")

	buf.Reset()
	Error("error log")
	assert.Contains(t, buf.String(), "[ERROR] error log")

	// Nil output redirection falls back to io.Discard
	SetOutput(nil)
	assert.Equal(t, io.Discard, GetGlobalLogger().GetWriter())
}
