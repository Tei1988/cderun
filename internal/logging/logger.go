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

// Enabled returns true if the given level is enabled.
func (l *Logger) Enabled(level Level) bool {
	return level <= l.GetLevel()
}

// DebugEnabled returns true if debug level is enabled.
func (l *Logger) DebugEnabled() bool {
	return l.Enabled(DebugLevel)
}

// TraceEnabled returns true if trace level is enabled.
func (l *Logger) TraceEnabled() bool {
	return l.Enabled(TraceLevel)
}

func newDefaultLogger() *Logger {
	l := &Logger{
		writer:    os.Stderr,
		format:    "text",
		timestamp: true,
	}
	l.SetLevel(ErrorLevel)
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

func (l *Logger) formatJSON(level Level, message string, now time.Time) []byte {
	entry := map[string]any{
		"level": level.LowerString(),
		"msg":   message,
	}
	if l.timestamp {
		entry["time"] = now.Format(time.RFC3339)
	}
	data, _ := json.Marshal(entry) //nolint:errcheck
	return data
}

func (l *Logger) formatText(level Level, message string, now time.Time) string {
	ts := ""
	if l.timestamp {
		ts = now.Format("2006-01-02 15:04:05") + " "
	}
	return fmt.Sprintf("%s[%s] %s\n", ts, level.String(), message)
}

func (l *Logger) log(level Level, msg string, args ...any) {
	if level > l.GetLevel() {
		return
	}

	message := fmt.Sprintf(msg, args...)
	message = SanitizeLogString(message)
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Re-check level inside lock
	if level > l.GetLevel() {
		return
	}

	if l.format == "json" {
		data := l.formatJSON(level, message, now)
		_, _ = fmt.Fprintln(l.writer, string(data))
	} else {
		output := l.formatText(level, message, now)
		_, _ = fmt.Fprint(l.writer, output)
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

// Enabled returns true if the given level is enabled on the global logger.
func Enabled(level Level) bool {
	return globalLogger.Enabled(level)
}

// DebugEnabled returns true if debug level is enabled on the global logger.
func DebugEnabled() bool {
	return globalLogger.DebugEnabled()
}

// TraceEnabled returns true if trace level is enabled on the global logger.
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

const hexChars = "0123456789abcdef"

// SanitizeLogString escapes ASCII control characters (ASCII < 32 and ASCII 127)
// with hex-escaped strings, preserving only tab ('\t') to prevent terminal
// escape sequence or log injection attacks. Carriage returns ('\r') and
// line feeds ('\n') are explicitly hex-escaped as "\x0d" and "\x0a".
func SanitizeLogString(s string) string {
	hasControl := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 32 && c != '\t') || c == 127 {
			hasControl = true
			break
		}
	}
	if !hasControl {
		return s
	}

	// Fast stack-allocated path for common messages (up to 256 bytes)
	// Each control character is escaped to 4 bytes. If len(s) <= 256, the maximum
	// possible size is 4 * 256 = 1024 bytes, which fits in a 1KB stack buffer.
	if len(s) <= 256 {
		var buf [1024]byte
		w := 0
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c == '\t' {
				buf[w] = c
				w++
			} else if c < 32 || c == 127 {
				buf[w] = '\\'
				buf[w+1] = 'x'
				buf[w+2] = hexChars[c>>4]
				buf[w+3] = hexChars[c&0x0f]
				w += 4
			} else {
				buf[w] = c
				w++
			}
		}
		return string(buf[:w])
	}

	var builder strings.Builder
	builder.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\t' {
			builder.WriteByte(c)
		} else if c < 32 || c == 127 {
			builder.WriteString("\\x")
			builder.WriteByte(hexChars[c>>4])
			builder.WriteByte(hexChars[c&0x0f])
		} else {
			builder.WriteByte(c)
		}
	}
	return builder.String()
}
