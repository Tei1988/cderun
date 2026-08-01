package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Logging_TextFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger()
	logger.SetOutput(buf)
	_ = logger.Init("info", "text", false)

	logger.log(InfoLevel, "test info message")
	assert.Contains(t, buf.String(), "[INFO] test info message")

	buf.Reset()
	logger.log(DebugLevel, "test debug message")
	assert.Empty(t, buf.String())

	logger.SetLevel(DebugLevel)
	logger.log(DebugLevel, "test debug message")
	assert.Contains(t, buf.String(), "[DEBUG] test debug message")
}

func TestUnit_Logging_JSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger()
	logger.SetOutput(buf)
	_ = logger.Init("info", "json", false)

	logger.log(InfoLevel, "test json message %d", 123)

	var entry map[string]string
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	assert.Equal(t, "info", entry["level"])
	assert.Equal(t, "test json message 123", entry["msg"])
}

func TestUnit_Logging_WithTimestamp(t *testing.T) {
	buf := &bytes.Buffer{}

	t.Run("text format", func(t *testing.T) {
		buf.Reset()
		logger := NewLogger()
		logger.SetOutput(buf)
		_ = logger.Init("info", "text", true)

		logger.log(InfoLevel, "msg")
		// Format: 2006-01-02 15:04:05 [INFO] msg
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \[INFO\] msg`, buf.String())
	})

	t.Run("json format", func(t *testing.T) {
		buf.Reset()
		logger := NewLogger()
		logger.SetOutput(buf)
		_ = logger.Init("info", "json", true)

		logger.log(InfoLevel, "msg")
		var entry map[string]string
		err := json.Unmarshal(buf.Bytes(), &entry)
		require.NoError(t, err)
		assert.NotEmpty(t, entry["time"])
		assert.Equal(t, "info", entry["level"])
		assert.Equal(t, "msg", entry["msg"])
	})
}

func TestUnit_Logging_LevelString(t *testing.T) {
	assert.Equal(t, "ERROR", ErrorLevel.String())
	assert.Equal(t, "WARN", WarnLevel.String())
	assert.Equal(t, "INFO", InfoLevel.String())
	assert.Equal(t, "DEBUG", DebugLevel.String())
	assert.Equal(t, "TRACE", TraceLevel.String())
	assert.Equal(t, "INFO", Level(-1).String())
}

func TestUnit_Logging_LevelParse(t *testing.T) {
	assert.Equal(t, ErrorLevel, ParseLevel("error"))
	assert.Equal(t, WarnLevel, ParseLevel("warn"))
	assert.Equal(t, WarnLevel, ParseLevel("warning"))
	assert.Equal(t, InfoLevel, ParseLevel("info"))
	assert.Equal(t, DebugLevel, ParseLevel("debug"))
	assert.Equal(t, TraceLevel, ParseLevel("trace"))
	assert.Equal(t, InfoLevel, ParseLevel("unknown"))
}

func TestUnit_Logging_InitGlobal(t *testing.T) {
	// Test Init updates globalLogger
	err := Init("debug", "json", false)
	require.NoError(t, err)
	assert.Equal(t, DebugLevel, GetGlobalLogger().GetLevel())
	assert.Equal(t, "json", GetGlobalLogger().GetFormat())
	assert.False(t, GetGlobalLogger().GetTimestamp())
}

func TestUnit_Logging_Wrappers(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf)
	err := Init("trace", "text", false)
	require.NoError(t, err)

	Error("err")
	assert.Contains(t, buf.String(), "[ERROR] err")
	buf.Reset()

	Warn("warn")
	assert.Contains(t, buf.String(), "[WARN] warn")
	buf.Reset()

	Info("info")
	assert.Contains(t, buf.String(), "[INFO] info")
	buf.Reset()

	Debug("debug")
	assert.Contains(t, buf.String(), "[DEBUG] debug")
	buf.Reset()

	Trace("trace")
	assert.Contains(t, buf.String(), "[TRACE] trace")
}

func TestUnit_Logging_OutputNil(t *testing.T) {
	SetOutput(nil)
	assert.Equal(t, io.Discard, GetGlobalLogger().GetWriter())
}

func TestUnit_Logging_SanitizeLogString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no control characters",
			input:    "hello world 123",
			expected: "hello world 123",
		},
		{
			name:     "contains carriage returns, tabs, and newlines (allowed)",
			input:    "line1\n\rline2\tline3",
			expected: "line1\n\rline2\tline3",
		},
		{
			name:     "contains escape sequences and delete",
			input:    "escape \x1b[31mred\x1b[0m and \x7fdelete",
			expected: `escape \x1b[31mred\x1b[0m and \x7fdelete`,
		},
		{
			name:     "contains null byte",
			input:    "null\x00byte",
			expected: `null\x00byte`,
		},
		{
			name:     "control characters in format output",
			input:    "message \x01\x02\x1f",
			expected: `message \x01\x02\x1f`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := SanitizeLogString(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestUnit_Logging_SanitizedLoggerOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger()
	logger.SetOutput(buf)
	_ = logger.Init("info", "text", false)

	logger.log(InfoLevel, "dangerous escape sequence: \x1b[2J")
	assert.Contains(t, buf.String(), `[INFO] dangerous escape sequence: \x1b[2J`)
}
