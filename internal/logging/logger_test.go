package logging

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerTextFormat(t *testing.T) {
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

func TestLoggerJSONFormat(t *testing.T) {
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

func TestParseLevel(t *testing.T) {
	assert.Equal(t, ErrorLevel, ParseLevel("error"))
	assert.Equal(t, WarnLevel, ParseLevel("warn"))
	assert.Equal(t, WarnLevel, ParseLevel("warning"))
	assert.Equal(t, InfoLevel, ParseLevel("info"))
	assert.Equal(t, DebugLevel, ParseLevel("debug"))
	assert.Equal(t, TraceLevel, ParseLevel("trace"))
	assert.Equal(t, InfoLevel, ParseLevel("unknown"))
}

func TestInit(t *testing.T) {
	// Test Init updates globalLogger
	err := Init("debug", "json", "", false, false)
	assert.NoError(t, err)
	assert.Equal(t, DebugLevel, globalLogger.Level)
	assert.Equal(t, "json", globalLogger.Format)
	assert.Equal(t, false, globalLogger.Timestamp)
}
