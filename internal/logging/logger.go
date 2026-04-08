package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Level int32

const (
	ErrorLevel Level = iota
	WarnLevel
	InfoLevel
	DebugLevel
	TraceLevel
)

func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "error":
		return ErrorLevel
	case "warn", "warning":
		return WarnLevel
	case "debug":
		return DebugLevel
	case "trace":
		return TraceLevel
	default:
		return InfoLevel
	}
}

var (
	levelNames = []string{
		"ERROR",
		"WARN",
		"INFO",
		"DEBUG",
		"TRACE",
	}
	levelLowerNames = []string{
		"error",
		"warn",
		"info",
		"debug",
		"trace",
	}
)

func (l Level) String() string {
	if l < 0 || int(l) >= len(levelNames) {
		return "INFO"
	}
	return levelNames[int(l)]
}

func (l Level) LowerString() string {
	if l < 0 || int(l) >= len(levelLowerNames) {
		return "info"
	}
	return levelLowerNames[int(l)]
}

type Logger struct {
	mu        sync.Mutex
	level     atomic.Int32
	writer    io.Writer
	format    string // "text" or "json"
	timestamp bool
}

func (l *Logger) SetLevel(level Level) {
	l.level.Store(int32(level))
}

func (l *Logger) GetLevel() Level {
	return Level(l.level.Load())
}

func (l *Logger) Enabled(level Level) bool {
	return level <= l.GetLevel()
}

func (l *Logger) DebugEnabled() bool {
	return l.Enabled(DebugLevel)
}

func (l *Logger) TraceEnabled() bool {
	return l.Enabled(TraceLevel)
}

func (l *Logger) GetFormat() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.format
}

func (l *Logger) GetTimestamp() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.timestamp
}

func (l *Logger) GetWriter() io.Writer {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writer
}

func newDefaultLogger() *Logger {
	l := &Logger{
		writer:    os.Stderr,
		format:    "text",
		timestamp: true,
	}
	l.SetLevel(WarnLevel)
	return l
}

var globalLogger = newDefaultLogger()

func Init(level string, format string, timestamp bool) error {
	return globalLogger.Init(level, format, timestamp)
}

func (l *Logger) Init(level string, format string, timestamp bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.SetLevel(ParseLevel(level))
	l.format = strings.ToLower(format)
	l.timestamp = timestamp

	return nil
}

func SetOutput(w io.Writer) {
	globalLogger.SetOutput(w)
}

func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if w == nil {
		w = io.Discard
	}
	l.writer = w
}

func (l *Logger) log(level Level, msg string, args ...any) {
	if level > l.GetLevel() {
		return
	}

	message := fmt.Sprintf(msg, args...)
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Re-check level inside lock
	if level > l.GetLevel() {
		return
	}

	if l.format == "json" {
		entry := map[string]any{
			"level": level.LowerString(),
			"msg":   message,
		}
		if l.timestamp {
			entry["time"] = now.Format(time.RFC3339)
		}
		data, _ := json.Marshal(entry) //nolint:errcheck
		_, _ = fmt.Fprintln(l.writer, string(data))
	} else {
		ts := ""
		if l.timestamp {
			ts = now.Format("2006-01-02 15:04:05") + " "
		}
		_, _ = fmt.Fprintf(l.writer, "%s[%s] %s\n", ts, level.String(), message)
	}
}

func (l *Logger) Error(msg string, args ...any) { l.log(ErrorLevel, msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.log(WarnLevel, msg, args...) }
func (l *Logger) Info(msg string, args ...any)  { l.log(InfoLevel, msg, args...) }
func (l *Logger) Debug(msg string, args ...any) { l.log(DebugLevel, msg, args...) }
func (l *Logger) Trace(msg string, args ...any) { l.log(TraceLevel, msg, args...) }

func Error(msg string, args ...any) { globalLogger.Error(msg, args...) }
func Warn(msg string, args ...any)  { globalLogger.Warn(msg, args...) }
func Info(msg string, args ...any)  { globalLogger.Info(msg, args...) }
func Debug(msg string, args ...any) { globalLogger.Debug(msg, args...) }
func Trace(msg string, args ...any) { globalLogger.Trace(msg, args...) }

func Enabled(level Level) bool {
	return globalLogger.Enabled(level)
}

func DebugEnabled() bool {
	return globalLogger.DebugEnabled()
}

func TraceEnabled() bool {
	return globalLogger.TraceEnabled()
}

// GetGlobalLogger returns the global logger instance.
// Callers should use Init() and SetOutput() methods to configure it,
// as they are thread-safe and properly handle internal state.
func GetGlobalLogger() *Logger {
	return globalLogger
}

func NewLogger() *Logger {
	return newDefaultLogger()
}
