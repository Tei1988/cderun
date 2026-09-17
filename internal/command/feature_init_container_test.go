package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Root_InitContainer_ExtraErrors(t *testing.T) {
	t.Parallel()

	t.Run("runtime factory failure", func(t *testing.T) {
		opts := &rootOptions{
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return nil, errors.New("factory failed")
			},
		}
		resolved := &config.ResolvedConfig{Runtime: "docker"}
		_, _, _, err := opts.initContainer(context.Background(), resolved, nil)
		require.Error(t, err)
		var rtErr *config.RuntimeInitError
		require.ErrorAs(t, err, &rtErr)
		assert.Equal(t, "docker", rtErr.Runtime)
	})

	t.Run("runtime close failure on init error", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			PullErr:  errors.New("pull failed"),
			CloseErr: errors.New("close failed"),
		}
		var logBuf bytes.Buffer
		logger := logging.NewLogger()
		logger.Init("debug", "text", false)
		logger.SetOutput(&logBuf)

		opts := &rootOptions{
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			},
			logger: logger,
		}

		resolved := &config.ResolvedConfig{}
		_, _, _, err := opts.initContainer(context.Background(), resolved, &container.ContainerConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pull failed")

		// Verify that the close failure was logged
		assert.Contains(t, logBuf.String(), "failed to close runtime on init failure: close failed")
	})

	t.Run("early validation failure stops execution before image pull", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			ValidateErr: errors.New("invalid container configuration"),
			PullErr:     errors.New("should not be called"),
		}

		opts := &rootOptions{
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			},
			logger: logging.GetGlobalLogger(),
		}

		resolved := &config.ResolvedConfig{}
		_, _, _, err := opts.initContainer(context.Background(), resolved, &container.ContainerConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configuration validation failed")
		assert.Contains(t, err.Error(), "invalid container configuration")

		// Assert that pull was never attempted because validation failed early
		mock.WithLockedMock(func(m *runtime.MockRuntime) {
			assert.Empty(t, m.PulledImage, "PullImage should not have been called")
		})
	})
}

func TestUnit_Root_InitContainer_DebugLogging_WithMasking(t *testing.T) {
	t.Parallel()

	mock := &runtime.MockRuntime{}
	var logBuf bytes.Buffer
	logger := logging.NewLogger()
	logger.Init("debug", "text", false)
	logger.SetOutput(&logBuf)

	opts := &rootOptions{
		runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mock, nil
		},
		logger: logger,
	}

	resolved := &config.ResolvedConfig{
		SensitiveEnv: []string{"PASSWORD"},
	}
	cc := &container.ContainerConfig{
		Image:   "alpine/git:v2.52.0",
		Command: []string{"status"},
		Mounts: []container.Mount{
			{Type: "bind", Source: "/Users/user/master", Target: "/Users/user/master"},
		},
		Env: []string{"USER=jules", "PASSWORD=supersecret"},
	}

	_, _, _, err := opts.initContainer(context.Background(), resolved, cc)
	require.NoError(t, err)

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "ContainerConfig:")
	assert.Contains(t, logOutput, "Image:      alpine/git:v2.52.0")
	assert.Contains(t, logOutput, "Command:    [status]")
	assert.Contains(t, logOutput, "bind /Users/user/master -> /Users/user/master")
	assert.Contains(t, logOutput, "USER=jules")
	assert.Contains(t, logOutput, "PASSWORD=[REDACTED]")
	assert.NotContains(t, logOutput, "supersecret")
}
