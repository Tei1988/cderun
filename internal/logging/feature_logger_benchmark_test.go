package logging

import (
	"io"
	"strings"
	"testing"
	"time"
)

func BenchmarkParseLevel(b *testing.B) {
	levels := []string{"warn", "INFO", "Error", "DEBUG", "trace", "WARNING", "invalid"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseLevel(levels[i%len(levels)])
	}
}

func BenchmarkSanitizeLogString_Clean(b *testing.B) {
	msg := "This is a standard log message without any control characters."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeLogString(msg)
	}
}

func BenchmarkSanitizeLogString_StackPath(b *testing.B) {
	msg := "Standard log message with\nnew line and\rreturn and\ttab character."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeLogString(msg)
	}
}

func BenchmarkSanitizeLogString_HeapPath(b *testing.B) {
	msg := strings.Repeat("Long log message with \n multiple new lines and \r returns ", 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeLogString(msg)
	}
}

func BenchmarkWriteFormattedLog_TextStackPath(b *testing.B) {
	logger := NewLogger()
	logger.SetOutput(io.Discard)
	now := time.Now()
	msg := "Container started successfully with ID 1234567890"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.writeFormattedLog(InfoLevel, msg, now)
	}
}

func BenchmarkWriteFormattedLog_JSON(b *testing.B) {
	logger := NewLogger()
	_ = logger.Init("info", "json", true)
	logger.SetOutput(io.Discard)
	now := time.Now()
	msg := "Container started successfully with ID 1234567890"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.writeFormattedLog(InfoLevel, msg, now)
	}
}
