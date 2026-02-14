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

type Level int

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
	return levelNames[l]
}

func (l Level) LowerString() string {
	if l < 0 || int(l) >= len(levelLowerNames) {
		return "info"
	}
	return levelLowerNames[l]
}

type Logger struct {
	mu        sync.Mutex
	level     atomic.Int32
	Writer    io.Writer
	Format    string // "text" or "json"
	Timestamp bool
}

func (l *Logger) SetLevel(level Level) {
	l.level.Store(int32(level))
}

func (l *Logger) GetLevel() Level {
	return Level(l.level.Load())
}

var globalLogger = func() *Logger {
	l := &Logger{
		Writer:    os.Stderr,
		Format:    "text",
		Timestamp: true,
	}
	l.SetLevel(WarnLevel)
	return l
}()

func Init(level string, format string, timestamp bool) error {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	globalLogger.SetLevel(ParseLevel(level))
	globalLogger.Format = strings.ToLower(format)
	globalLogger.Timestamp = timestamp

	return nil
}

func SetOutput(w io.Writer) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()
	if w == nil {
		w = io.Discard
	}
	globalLogger.Writer = w
}

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	// Optimization: Check level before locking to avoid contention for filtered logs.
	// We use atomic load to avoid data races.
	if level > l.GetLevel() {
		return
	}

	// Prepare the message and timestamp outside of the lock to reduce critical section time.
	message := fmt.Sprintf(msg, args...)
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Re-check level inside the lock for absolute consistency.
	if level > l.GetLevel() {
		return
	}

	if l.Format == "json" {
		entry := map[string]interface{}{
			"level": level.LowerString(),
			"msg":   message,
		}
		if l.Timestamp {
			entry["time"] = now.Format(time.RFC3339)
		}
		// JSON marshaling and writing are done inside the lock to ensure atomic log lines.
		data, _ := json.Marshal(entry)
		_, _ = fmt.Fprintln(l.Writer, string(data))
	} else {
		ts := ""
		if l.Timestamp {
			ts = now.Format("2006-01-02 15:04:05") + " "
		}
		_, _ = fmt.Fprintf(l.Writer, "%s[%s] %s\n", ts, level.String(), message)
	}
}

func Error(msg string, args ...interface{}) { globalLogger.log(ErrorLevel, msg, args...) }
func Warn(msg string, args ...interface{})  { globalLogger.log(WarnLevel, msg, args...) }
func Info(msg string, args ...interface{})  { globalLogger.log(InfoLevel, msg, args...) }
func Debug(msg string, args ...interface{}) { globalLogger.log(DebugLevel, msg, args...) }
func Trace(msg string, args ...interface{}) { globalLogger.log(TraceLevel, msg, args...) }
