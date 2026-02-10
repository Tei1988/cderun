package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Logging_LoggerTextFormat(t *testing.T) {
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

func TestUnit_Logging_LoggerJSONFormat(t *testing.T) {
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
	err := Init("debug", "json", false)
	assert.NoError(t, err)
	assert.Equal(t, DebugLevel, globalLogger.Level)
	assert.Equal(t, "json", globalLogger.Format)
	assert.Equal(t, false, globalLogger.Timestamp)
}

func TestUnit_Logging_SetOutputNil(t *testing.T) {
	SetOutput(nil)
	assert.Equal(t, io.Discard, globalLogger.Writer)
}
