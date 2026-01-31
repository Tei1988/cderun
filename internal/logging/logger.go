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
		Level:     InfoLevel,
		Writer:    os.Stderr,
		Format:    "text",
		Timestamp: true,
	}

	currentLogFile *os.File
)

func Init(level string, format string, file string, tee bool, timestamp bool) error {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	globalLogger.Level = ParseLevel(level)
	globalLogger.Format = strings.ToLower(format)
	globalLogger.Timestamp = timestamp

	var out io.Writer = os.Stderr

	if file != "" {
		f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file %q: %w", file, err)
		}

		// Close previous log file if it was open
		if currentLogFile != nil {
			currentLogFile.Close()
		}
		currentLogFile = f

		if tee {
			out = io.MultiWriter(os.Stderr, f)
		} else {
			out = f
		}
	} else {
		// If switching to no file, close previous log file
		if currentLogFile != nil {
			currentLogFile.Close()
			currentLogFile = nil
		}
	}

	globalLogger.Writer = out
	return nil
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
		fmt.Fprintln(l.Writer, string(data))
	} else {
		ts := ""
		if l.Timestamp {
			ts = now.Format("2006-01-02 15:04:05") + " "
		}
		fmt.Fprintf(l.Writer, "%s[%s] %s\n", ts, level.String(), message)
	}
}

func Error(msg string, args ...interface{}) { globalLogger.log(ErrorLevel, msg, args...) }
func Warn(msg string, args ...interface{})  { globalLogger.log(WarnLevel, msg, args...) }
func Info(msg string, args ...interface{})  { globalLogger.log(InfoLevel, msg, args...) }
func Debug(msg string, args ...interface{}) { globalLogger.log(DebugLevel, msg, args...) }
func Trace(msg string, args ...interface{}) { globalLogger.log(TraceLevel, msg, args...) }
