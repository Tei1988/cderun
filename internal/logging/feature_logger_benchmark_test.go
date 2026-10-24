package logging

import (
	"io"
	"testing"
	"time"
)

func BenchmarkLogger_ParseLevel_FastASCII(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseLevel("info")
		_ = ParseLevel("error")
		_ = ParseLevel("trace")
	}
}

func BenchmarkLogger_ParseLevel_MixedCase_FastASCII(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseLevel("INFO")
		_ = ParseLevel("Error")
		_ = ParseLevel("TRACE")
	}
}

func BenchmarkSanitizeLogString_HeapCapacityPreAllocated(b *testing.B) {
	// Create a > 512 byte log string with multiple control characters
	raw := make([]byte, 600)
	for i := range raw {
		if i%50 == 0 {
			raw[i] = '\n'
		} else {
			raw[i] = 'a'
		}
	}
	s := string(raw)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeLogString(s)
	}
}

func BenchmarkLogger_WriteFormattedLog_TextStackBuffer(b *testing.B) {
	logger := NewLogger()
	logger.SetOutput(io.Discard)
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	msg := "Container finished execution with exit code 0"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.writeFormattedLog(InfoLevel, msg, now)
	}
}

func BenchmarkLogger_WriteFormattedLog_JSONMode(b *testing.B) {
	logger := NewLogger()
	_ = logger.Init("info", "json", true)
	logger.SetOutput(io.Discard)
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	msg := "Container finished execution with exit code 0"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.writeFormattedLog(InfoLevel, msg, now)
	}
}
