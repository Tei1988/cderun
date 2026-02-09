package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
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

func (l Level) String() string {
	switch l {
	case ErrorLevel:
		return "ERROR"
	case WarnLevel:
		return "WARN"
	case DebugLevel:
		return "DEBUG"
	case TraceLevel:
		return "TRACE"
	default:
		return "INFO"
	}
}

type Logger struct {
	mu        sync.Mutex
	Level     Level
	Writer    io.Writer
	Format    string // "text" or "json"
	Timestamp bool
}

var (
	globalLogger = &Logger{
		Level:     WarnLevel,
		Writer:    os.Stderr,
		Format:    "text",
		Timestamp: true,
	}
)

func Init(level string, format string, timestamp bool) error {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	globalLogger.Level = ParseLevel(level)
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
	l.mu.Lock()
	defer l.mu.Unlock()

	if level > l.Level {
		return
	}

	message := fmt.Sprintf(msg, args...)
	now := time.Now()

	if l.Format == "json" {
		entry := map[string]interface{}{
			"level": strings.ToLower(level.String()),
			"msg":   message,
		}
		if l.Timestamp {
			entry["time"] = now.Format(time.RFC3339)
		}
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
