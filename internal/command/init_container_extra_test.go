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
			runtimeFactory: func(name, socket string) (runtime.ContainerRuntime, error) {
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
			runtimeFactory: func(name, socket string) (runtime.ContainerRuntime, error) {
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
}
