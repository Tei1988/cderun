package logging_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *safeBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *safeBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

func (sb *safeBuffer) Reset() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.buf.Reset()
}

func TestLogger_ParseLevelAndStringBoundaries(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    string
		expected logging.Level
	}{
		{"warn", logging.WarnLevel},
		{"WARN", logging.WarnLevel},
		{"WaRn", logging.WarnLevel},
		{"warning", logging.WarnLevel},
		{"WARNING", logging.WarnLevel},
		{"info", logging.InfoLevel},
		{"INFO", logging.InfoLevel},
		{"error", logging.ErrorLevel},
		{"ERROR", logging.ErrorLevel},
		{"debug", logging.DebugLevel},
		{"DEBUG", logging.DebugLevel},
		{"trace", logging.TraceLevel},
		{"TRACE", logging.TraceLevel},
		{"unknown", logging.InfoLevel},
		{"", logging.InfoLevel},
		{"a", logging.InfoLevel},
		{"ab", logging.InfoLevel},
		{"abc", logging.InfoLevel},
		{"12345678", logging.InfoLevel},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("ParseLevel_%s", tc.input), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, logging.ParseLevel(tc.input))
		})
	}

	// Level String and LowerString boundaries
	assert.Equal(t, "ERROR", logging.ErrorLevel.String())
	assert.Equal(t, "WARN", logging.WarnLevel.String())
	assert.Equal(t, "INFO", logging.InfoLevel.String())
	assert.Equal(t, "DEBUG", logging.DebugLevel.String())
	assert.Equal(t, "TRACE", logging.TraceLevel.String())
	assert.Equal(t, "INFO", logging.Level(-1).String())
	assert.Equal(t, "INFO", logging.Level(100).String())

	assert.Equal(t, "error", logging.ErrorLevel.LowerString())
	assert.Equal(t, "warn", logging.WarnLevel.LowerString())
	assert.Equal(t, "info", logging.InfoLevel.LowerString())
	assert.Equal(t, "debug", logging.DebugLevel.LowerString())
	assert.Equal(t, "trace", logging.TraceLevel.LowerString())
	assert.Equal(t, "info", logging.Level(-1).LowerString())
	assert.Equal(t, "info", logging.Level(100).LowerString())
}

func TestLogger_LevelFilteringAndEnabled(t *testing.T) {
	t.Parallel()

	logger := logging.NewLogger()
	logger.SetLevel(logging.WarnLevel)

	assert.Equal(t, logging.WarnLevel, logger.GetLevel())
	assert.True(t, logger.Enabled(logging.ErrorLevel))
	assert.True(t, logger.Enabled(logging.WarnLevel))
	assert.False(t, logger.Enabled(logging.InfoLevel))
	assert.False(t, logger.DebugEnabled())
	assert.False(t, logger.TraceEnabled())

	logger.SetLevel(logging.DebugLevel)
	assert.True(t, logger.DebugEnabled())
	assert.False(t, logger.TraceEnabled())

	logger.SetLevel(logging.TraceLevel)
	assert.True(t, logger.TraceEnabled())
}

func TestLogger_InitAndGetters(t *testing.T) {
	t.Parallel()

	logger := logging.NewLogger()
	buf := &safeBuffer{}
	logger.SetOutput(buf)

	err := logger.Init("debug", "  JSON  ", false)
	require.NoError(t, err)

	assert.Equal(t, logging.DebugLevel, logger.GetLevel())
	assert.Equal(t, "json", logger.GetFormat())
	assert.False(t, logger.GetTimestamp())
	assert.Equal(t, buf, logger.GetWriter())

	// Test fallback to "text" format for invalid format strings
	err = logger.Init("info", "xml", true)
	require.NoError(t, err)
	assert.Equal(t, "text", logger.GetFormat())
	assert.True(t, logger.GetTimestamp())

	// Test nil output setter fallback to io.Discard
	logger.SetOutput(nil)
	assert.Equal(t, io.Discard, logger.GetWriter())
}

func TestLogger_SanitizeLogStringBoundaries(t *testing.T) {
	t.Parallel()

	// 1. Preserved tab, escaped control bytes
	clean := "clean text \t with tab"
	assert.Equal(t, clean, logging.SanitizeLogString(clean))

	withCtrl := "hello\x00world\r\n\x1f\x7f"
	expectedEscaped := "hello\\x00world\\x0d\\x0a\\x1f\\x7f"
	assert.Equal(t, expectedEscaped, logging.SanitizeLogString(withCtrl))

	// 2. Vectorized hasControlByte boundary scans (len < 8, len = 8, len > 8)
	// Control character at 0th position
	assert.Equal(t, "\\x012345678", logging.SanitizeLogString("\x012345678"))
	// Control character at 7th position
	assert.Equal(t, "0123456\\x0789", logging.SanitizeLogString("0123456\x0789"))
	// Control character at 8th position
	assert.Equal(t, "01234567\\x0890", logging.SanitizeLogString("01234567\x0890"))

	// 3. Stack buffer boundary (exactly 512 bytes vs 513 bytes)
	stackStr := strings.Repeat("a", 510) + "\x00b"
	require.Equal(t, 512, len(stackStr))
	stackResult := logging.SanitizeLogString(stackStr)
	assert.Equal(t, strings.Repeat("a", 510)+"\\x00b", stackResult)

	heapStr := strings.Repeat("a", 511) + "\x00b"
	require.Equal(t, 513, len(heapStr))
	heapResult := logging.SanitizeLogString(heapStr)
	assert.Equal(t, strings.Repeat("a", 511)+"\\x00b", heapResult)
}

func TestLogger_OutputFormattingScenarios(t *testing.T) {
	t.Parallel()

	t.Run("TextFormatWithTimestamp", func(t *testing.T) {
		t.Parallel()
		logger := logging.NewLogger()
		buf := &safeBuffer{}
		logger.SetOutput(buf)

		require.NoError(t, logger.Init("info", "text", true))
		logger.Info("user %s logged in", "alice")

		out := buf.String()
		assert.Contains(t, out, "[INFO] user alice logged in\n")
		// Verify timestamp format YYYY-MM-DD
		nowStr := time.Now().Format("2006-01-02")
		assert.Contains(t, out, nowStr)
	})

	t.Run("TextFormatWithoutTimestamp", func(t *testing.T) {
		t.Parallel()
		logger := logging.NewLogger()
		buf := &safeBuffer{}
		logger.SetOutput(buf)

		require.NoError(t, logger.Init("warn", "text", false))
		logger.Warn("disk space low")

		out := buf.String()
		assert.Equal(t, "[WARN] disk space low\n", out)
	})

	t.Run("JSONFormatWithAndWithoutTimestamp", func(t *testing.T) {
		t.Parallel()

		// With timestamp
		logger := logging.NewLogger()
		buf := &safeBuffer{}
		logger.SetOutput(buf)

		require.NoError(t, logger.Init("debug", "json", true))
		logger.Debug("connecting to %s", "localhost:8080")

		out := strings.TrimSpace(buf.String())
		var jsonMap map[string]any
		err := json.Unmarshal([]byte(out), &jsonMap)
		require.NoError(t, err)

		assert.Equal(t, "debug", jsonMap["level"])
		assert.Equal(t, "connecting to localhost:8080", jsonMap["msg"])
		assert.Contains(t, jsonMap, "time")

		// Without timestamp
		buf.Reset()
		require.NoError(t, logger.Init("error", "json", false))
		logger.Error("connection failed")

		outNoTime := strings.TrimSpace(buf.String())
		var jsonMapNoTime map[string]any
		err = json.Unmarshal([]byte(outNoTime), &jsonMapNoTime)
		require.NoError(t, err)

		assert.Equal(t, "error", jsonMapNoTime["level"])
		assert.Equal(t, "connection failed", jsonMapNoTime["msg"])
		assert.NotContains(t, jsonMapNoTime, "time")
	})

	t.Run("LevelFilteringSuppression", func(t *testing.T) {
		t.Parallel()
		logger := logging.NewLogger()
		buf := &safeBuffer{}
		logger.SetOutput(buf)

		require.NoError(t, logger.Init("error", "text", false))

		logger.Warn("should be suppressed")
		logger.Info("should be suppressed")
		logger.Debug("should be suppressed")
		logger.Trace("should be suppressed")

		assert.Empty(t, buf.String())

		logger.Error("critical failure")
		assert.Equal(t, "[ERROR] critical failure\n", buf.String())
	})
}

func TestLogger_ConcurrencyAndThreadSafety(t *testing.T) {
	t.Parallel()

	logger := logging.NewLogger()
	buf := &safeBuffer{}
	logger.SetOutput(buf)

	var wg sync.WaitGroup
	const goroutines = 10
	const iterations = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				switch j % 5 {
				case 0:
					logger.SetLevel(logging.Level(j % 5))
				case 1:
					_ = logger.Init("debug", "json", j%2 == 0)
				case 2:
					logger.Error("goroutine %d iter %d", id, j)
					logger.Warn("goroutine %d iter %d", id, j)
				case 3:
					logger.Info("goroutine %d iter %d", id, j)
					logger.Debug("goroutine %d iter %d", id, j)
					logger.Trace("goroutine %d iter %d", id, j)
				case 4:
					_ = logger.GetLevel()
					_ = logger.GetFormat()
					_ = logger.GetTimestamp()
					_ = logger.GetWriter()
				}
			}
		}(i)
	}

	wg.Wait()
	assert.NotEmpty(t, buf.String())
}

func TestGlobalLogger_ThreadSafetyAndGetters(t *testing.T) {
	buf := &safeBuffer{}
	global := logging.GetGlobalLogger()
	originalWriter := global.GetWriter()
	originalLevel := global.GetLevel()

	defer func() {
		global.SetOutput(originalWriter)
		global.SetLevel(originalLevel)
	}()

	logging.SetOutput(buf)
	err := logging.Init("debug", "text", false)
	require.NoError(t, err)

	assert.True(t, logging.Enabled(logging.WarnLevel))
	assert.True(t, logging.DebugEnabled())
	assert.False(t, logging.TraceEnabled())

	logging.Error("global error message")
	logging.Warn("global warn message")
	logging.Info("global info message")
	logging.Debug("global debug message")
	logging.Trace("global trace message")

	out := buf.String()
	assert.Contains(t, out, "[ERROR] global error message\n")
	assert.Contains(t, out, "[WARN] global warn message\n")
	assert.Contains(t, out, "[INFO] global info message\n")
	assert.Contains(t, out, "[DEBUG] global debug message\n")
	assert.NotContains(t, out, "[TRACE] global trace message\n")
}
