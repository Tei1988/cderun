package command

import (
	"context"
	"testing"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_Command_ShmSize(t *testing.T) {
	t.Run("resolves and populates shm-size from CLI into ContainerConfig", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			WD: "/app",
			Env: map[string]string{
				"CDERUN_LOG_LEVEL": "error",
			},
		}

		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "test-shm-container"

		args := []string{"cderun", "--image", "alpine", "--shm-size", "256m", "sh"}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			}
		})

		require.NoError(t, err)
		cfg := mockRt.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, int64(268435456), cfg.ShmSize)
	})

	t.Run("resolves and populates shm-size from cderun override flag", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			WD: "/app",
			Env: map[string]string{
				"CDERUN_LOG_LEVEL": "error",
			},
		}

		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "test-shm-container-override"

		args := []string{"cderun", "sh", "--cderun-image", "alpine", "--cderun-shm-size", "512m"}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			}
		})

		require.NoError(t, err)
		cfg := mockRt.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, int64(536870912), cfg.ShmSize)
	})
}
