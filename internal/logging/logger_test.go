package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Logging_Logger_TextFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := &Logger{
		Level:     InfoLevel,
		Writer:    buf,
		Format:    "text",
		Timestamp: false,
	}

	logger.log(InfoLevel, "test info message")
	assert.Contains(t, buf.String(), "[INFO] test info message")

	buf.Reset()
	logger.log(DebugLevel, "test debug message")
	assert.Empty(t, buf.String())

	logger.Level = DebugLevel
	logger.log(DebugLevel, "test debug message")
	assert.Contains(t, buf.String(), "[DEBUG] test debug message")
}

func TestUnit_Logging_Logger_JSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := &Logger{
		Level:     InfoLevel,
		Writer:    buf,
		Format:    "json",
		Timestamp: false,
	}

	logger.log(InfoLevel, "test json message %d", 123)

	var entry map[string]string
	err := json.Unmarshal(buf.Bytes(), &entry)
	assert.NoError(t, err)
	assert.Equal(t, "info", entry["level"])
	assert.Equal(t, "test json message 123", entry["msg"])
}

func TestUnit_Logging_Logger_WithTimestamp(t *testing.T) {
	buf := &bytes.Buffer{}

	t.Run("text format", func(t *testing.T) {
		buf.Reset()
		logger := &Logger{
			Level:     InfoLevel,
			Writer:    buf,
			Format:    "text",
			Timestamp: true,
		}
		logger.log(InfoLevel, "msg")
		// Format: 2006-01-02 15:04:05 [INFO] msg
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \[INFO\] msg`, buf.String())
	})

	t.Run("json format", func(t *testing.T) {
		buf.Reset()
		logger := &Logger{
			Level:     InfoLevel,
			Writer:    buf,
			Format:    "json",
			Timestamp: true,
		}
		logger.log(InfoLevel, "msg")
		var entry map[string]string
		err := json.Unmarshal(buf.Bytes(), &entry)
		assert.NoError(t, err)
		assert.NotEmpty(t, entry["time"])
		assert.Equal(t, "info", entry["level"])
		assert.Equal(t, "msg", entry["msg"])
	})
}

func TestUnit_Logging_Level_String(t *testing.T) {
	assert.Equal(t, "ERROR", ErrorLevel.String())
	assert.Equal(t, "WARN", WarnLevel.String())
	assert.Equal(t, "INFO", InfoLevel.String())
	assert.Equal(t, "DEBUG", DebugLevel.String())
	assert.Equal(t, "TRACE", TraceLevel.String())
	assert.Equal(t, "INFO", Level(-1).String())
}

func TestUnit_Logging_Level_Parse(t *testing.T) {
	assert.Equal(t, ErrorLevel, ParseLevel("error"))
	assert.Equal(t, WarnLevel, ParseLevel("warn"))
	assert.Equal(t, WarnLevel, ParseLevel("warning"))
	assert.Equal(t, InfoLevel, ParseLevel("info"))
	assert.Equal(t, DebugLevel, ParseLevel("debug"))
	assert.Equal(t, TraceLevel, ParseLevel("trace"))
	assert.Equal(t, InfoLevel, ParseLevel("unknown"))
}

func TestUnit_Logging_Init_GlobalLogger(t *testing.T) {
	// Test Init updates globalLogger
	err := Init("debug", "json", false)
	assert.NoError(t, err)
	assert.Equal(t, DebugLevel, globalLogger.Level)
	assert.Equal(t, "json", globalLogger.Format)
	assert.Equal(t, false, globalLogger.Timestamp)
}

func TestUnit_Logging_Wrappers_AllLevels(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf)
	err := Init("trace", "text", false)
	assert.NoError(t, err)

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

func TestUnit_Logging_Output_NilGuard(t *testing.T) {
	SetOutput(nil)
	assert.Equal(t, io.Discard, globalLogger.Writer)
}
