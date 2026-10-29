package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime/controlsocket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_HasParentTraversal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "simple relative path",
			input:    "foo/bar",
			expected: false,
		},
		{
			name:     "absolute unix path",
			input:    "/usr/local/bin",
			expected: false,
		},
		{
			name:     "double dots inside filename",
			input:    "/tmp/a..b/file",
			expected: false,
		},
		{
			name:     "filename ending with double dots",
			input:    "/var/log/app..",
			expected: false,
		},
		{
			name:     "windows drive path with double dots in filename",
			input:    `C:\data\a..b`,
			expected: false,
		},
		{
			name:     "parent traversal at start of path",
			input:    "../relative/path",
			expected: true,
		},
		{
			name:     "parent traversal in middle with forward slash",
			input:    "foo/../bar",
			expected: true,
		},
		{
			name:     "parent traversal in middle with backslash",
			input:    `foo\..\bar`,
			expected: true,
		},
		{
			name:     "parent traversal following windows drive colon",
			input:    "C:..\\system32",
			expected: true,
		},
		{
			name:     "parent traversal at end of path",
			input:    "/var/log/..",
			expected: true,
		},
		{
			name:     "standalone parent traversal",
			input:    "..",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasParentTraversal(tt.input))
		})
	}
}

func TestUnit_Runtime_SleepFunc(t *testing.T) {
	t.Run("returns ctx.Err() immediately when context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := SleepFunc(ctx, time.Hour)
		require.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("completes successfully on timer expiration", func(t *testing.T) {
		ctx := context.Background()
		err := SleepFunc(ctx, 5*time.Millisecond)
		require.NoError(t, err)
	})
}

func TestUnit_Runtime_IsRetryablePullError_Comprehensive(t *testing.T) {
	retryableMessages := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"deadline exceeded",
		"unexpected eof",
		"i/o timeout",
		"tls handshake timeout",
		"client.timeout exceeded",
		"rate limit exceeded",
		"toomanyrequests",
		"rate exceeded",
		"rate limit",
		"data limit exceeded",
	}

	for _, msg := range retryableMessages {
		t.Run("retryable message: "+msg, func(t *testing.T) {
			err := fmt.Errorf("pull error: %s occurred", msg)
			assert.True(t, IsRetryablePullError(err))
		})
	}

	authKeywords := []string{
		"token expired",
		"expired token",
		"refresh token",
		"reauthenticate",
		"token refresh",
	}

	for _, kw := range authKeywords {
		t.Run("auth refresh keyword: "+kw, func(t *testing.T) {
			err := fmt.Errorf("auth error: %s required", kw)
			assert.True(t, IsTemporaryAuthError(err))
			assert.True(t, IsRetryablePullError(err))
		})
	}

	t.Run("non-retryable errors", func(t *testing.T) {
		assert.False(t, IsRetryablePullError(nil))
		assert.False(t, IsRetryablePullError(errors.New("image not found")))
		assert.False(t, IsRetryablePullError(errors.New("unauthorized access")))
		assert.False(t, IsRetryablePullError(&net.DNSError{IsNotFound: true}))
	})
}

func TestUnit_Runtime_CLIArgBuilder_EdgeCases(t *testing.T) {
	t.Run("AddJoinedFlag with empty values and omitEmpty", func(t *testing.T) {
		builder := NewCLIArgBuilder()
		builder.AddJoinedFlag("--flag1", "", true)  // omitted
		builder.AddJoinedFlag("--flag2", "", false) // kept as --flag2=
		argv := builder.Build()

		assert.Equal(t, []string{"--flag2="}, argv)
	})

	t.Run("AddRepeatJoinedFlag with empty and non-empty slice", func(t *testing.T) {
		builder := NewCLIArgBuilder()
		builder.AddRepeatJoinedFlag("--env", nil)
		builder.AddRepeatJoinedFlag("--env", []string{})
		builder.AddRepeatJoinedFlag("--label", []string{"a=1", "b=2"})

		argv := builder.Build()
		assert.Equal(t, []string{"--label=a=1", "--label=b=2"}, argv)
	})

	t.Run("VerifyStructure with expectedFlagCount matching vs mismatching", func(t *testing.T) {
		builder := NewCLIArgBuilder()
		builder.AddJoinedFlag("--net", "host", false)
		builder.AddPositionals("ubuntu:latest")
		argv := builder.Build()

		require.NoError(t, builder.VerifyStructure(argv, 1))

		errMismatch := builder.VerifyStructure(argv, 2)
		require.Error(t, errMismatch)
		assert.Contains(t, errMismatch.Error(), "expected 2 flags before '--', got 1")
	})

	t.Run("VerifyStructure control character exception for multiline strings", func(t *testing.T) {
		builder := NewCLIArgBuilder()
		builder.AddJoinedFlag("--cmd", "line1\nline2\rline3\tline4", false)
		argv := builder.Build()

		require.NoError(t, builder.VerifyStructure(argv, 1))

		// Disallowed control character (e.g. ASCII Bell \x07)
		builderWithBadChar := NewCLIArgBuilder()
		builderWithBadChar.AddJoinedFlag("--bad", "val\x07ue", false)
		argvBad := builderWithBadChar.Build()

		errBad := builderWithBadChar.VerifyStructure(argvBad, 1)
		require.Error(t, errBad)
		assert.Contains(t, errBad.Error(), "contains control character")
	})
}

func TestUnit_Runtime_MockRuntime_Scenarios(t *testing.T) {
	t.Run("WithLockedMock executes closure with lock", func(t *testing.T) {
		mock := NewMockRuntime()
		executed := false
		mock.WithLockedMock(func(m *MockRuntime) {
			m.ExitCode = 42
			m.PulledImage = "test:latest"
			executed = true
		})
		assert.True(t, executed)
		assert.Equal(t, 42, mock.GetExitCode())
		assert.Equal(t, "test:latest", mock.GetPulledImage())
	})

	t.Run("ResetCreatedConfig clears config", func(t *testing.T) {
		mock := NewMockRuntime()
		cfg := &container.ContainerConfig{Image: "alpine"}
		_, err := mock.CreateContainer(context.Background(), cfg)
		require.NoError(t, err)

		assert.NotNil(t, mock.GetCreatedConfig())
		mock.ResetCreatedConfig()
		assert.Nil(t, mock.GetCreatedConfig())
	})

	t.Run("SignalContainer validates signal format", func(t *testing.T) {
		mock := NewMockRuntime()

		require.NoError(t, mock.SignalContainer(context.Background(), "c123", "SIGTERM"))
		require.NoError(t, mock.SignalContainer(context.Background(), "c123", "9"))

		err := mock.SignalContainer(context.Background(), "c123", "SIGKILL; rm -rf /")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signal")
	})

	t.Run("PruneContainers and InspectContainer hooks", func(t *testing.T) {
		mock := NewMockRuntime()
		mock.InspectFunc = func(ctx context.Context, containerID string) (bool, int, error) {
			return true, 0, nil
		}
		mock.PruneFunc = func(ctx context.Context) ([]string, error) {
			return []string{"c1", "c2"}, nil
		}

		running, exitCode, err := mock.InspectContainer(context.Background(), "c1")
		require.NoError(t, err)
		assert.True(t, running)
		assert.Equal(t, 0, exitCode)

		pruned, err := mock.PruneContainers(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []string{"c1", "c2"}, pruned)
	})
}

func TestUnit_Runtime_ControlSocketRuntimeAdapter_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "ctrl_adapter_edge.sock")

	server := controlsocket.NewServer(socketPath, logging.NewLogger())
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)

	t.Run("NewControlSocketRuntimeAdapter accepts nil logger and falls back to global logger", func(t *testing.T) {
		mock := NewMockRuntime()
		adapter := NewControlSocketRuntimeAdapter(mock, client, nil)
		require.NotNil(t, adapter)
		assert.Equal(t, "mock-controlsocket", adapter.Name())
	})

	t.Run("Close joins errors from client and underlying runtime", func(t *testing.T) {
		mock := NewMockRuntime()
		mock.CloseErr = errors.New("underlying close failed")

		adapter := NewControlSocketRuntimeAdapter(mock, client, logging.NewLogger())
		err := adapter.Close()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "underlying close failed")
	})

	t.Run("Delegation methods forward calls to underlying runtime", func(t *testing.T) {
		mock := NewMockRuntime()
		adapter := NewControlSocketRuntimeAdapter(mock, client, logging.NewLogger())

		require.NoError(t, adapter.PullImage(ctx, "ubuntu:22.04", "missing", 3, time.Second))
		assert.Equal(t, "ubuntu:22.04", mock.GetPulledImage())

		require.NoError(t, adapter.ValidateConfig(&container.ContainerConfig{Image: "ubuntu:22.04"}))

		_, _, err := adapter.InspectContainer(ctx, "c100")
		require.NoError(t, err)
	})
}
