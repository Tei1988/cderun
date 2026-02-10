package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Logging_Level_String(t *testing.T) {
	assert.Equal(t, "ERROR", ErrorLevel.String())
	assert.Equal(t, "WARN", WarnLevel.String())
	assert.Equal(t, "INFO", InfoLevel.String())
	assert.Equal(t, "DEBUG", DebugLevel.String())
	assert.Equal(t, "TRACE", TraceLevel.String())
	assert.Equal(t, "INFO", Level(100).String())
}

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

	buf.Reset()
	logger.Level = TraceLevel
	logger.log(TraceLevel, "test trace message")
	assert.Contains(t, buf.String(), "[TRACE] test trace message")
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

func TestUnit_Logging_Logger_Timestamp(t *testing.T) {
	t.Run("text format with timestamp", func(t *testing.T) {
		buf := &bytes.Buffer{}
		logger := &Logger{
			Level:     InfoLevel,
			Writer:    buf,
			Format:    "text",
			Timestamp: true,
		}
		logger.log(InfoLevel, "test message")
		// Format: 2006-01-02 15:04:05 [INFO] test message
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \[INFO\] test message`, buf.String())
	})

	t.Run("json format with timestamp", func(t *testing.T) {
		buf := &bytes.Buffer{}
		logger := &Logger{
			Level:     InfoLevel,
			Writer:    buf,
			Format:    "json",
			Timestamp: true,
		}
		logger.log(InfoLevel, "test message")
		var entry map[string]string
		err := json.Unmarshal(buf.Bytes(), &entry)
		assert.NoError(t, err)
		assert.NotEmpty(t, entry["time"])
		assert.Equal(t, "test message", entry["msg"])
	})
}

func TestUnit_Logging_ParseLevel(t *testing.T) {
	assert.Equal(t, ErrorLevel, ParseLevel("error"))
	assert.Equal(t, WarnLevel, ParseLevel("warn"))
	assert.Equal(t, WarnLevel, ParseLevel("warning"))
	assert.Equal(t, InfoLevel, ParseLevel("info"))
	assert.Equal(t, DebugLevel, ParseLevel("debug"))
	assert.Equal(t, TraceLevel, ParseLevel("trace"))
	assert.Equal(t, InfoLevel, ParseLevel("unknown"))
}

func TestUnit_Logging_Init(t *testing.T) {
	// Test Init updates globalLogger
	err := Init("trace", "json", true)
	assert.NoError(t, err)
	assert.Equal(t, TraceLevel, globalLogger.Level)
	assert.Equal(t, "json", globalLogger.Format)
	assert.Equal(t, true, globalLogger.Timestamp)
}

func TestUnit_Logging_SetOutput(t *testing.T) {
	t.Run("valid writer", func(t *testing.T) {
		buf := &bytes.Buffer{}
		SetOutput(buf)
		assert.Equal(t, buf, globalLogger.Writer)
	})

	t.Run("nil writer", func(t *testing.T) {
		SetOutput(nil)
		assert.Equal(t, io.Discard, globalLogger.Writer)
	})
}

func TestUnit_Logging_PackageFunctions(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf)
	_ = Init("trace", "text", false)

	Error("error msg")
	assert.Contains(t, buf.String(), "[ERROR] error msg")
	buf.Reset()

	Warn("warn msg")
	assert.Contains(t, buf.String(), "[WARN] warn msg")
	buf.Reset()

	Info("info msg")
	assert.Contains(t, buf.String(), "[INFO] info msg")
	buf.Reset()

	Debug("debug msg")
	assert.Contains(t, buf.String(), "[DEBUG] debug msg")
	buf.Reset()

	Trace("trace msg")
	assert.Contains(t, buf.String(), "[TRACE] trace msg")
}
