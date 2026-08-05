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

func TestIntegration_Command_Ulimits(t *testing.T) {
	t.Run("resolves and populates ulimits from CLI into ContainerConfig", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			WD: "/app",
			Env: map[string]string{
				"CDERUN_LOG_LEVEL": "error",
			},
		}

		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "test-ulimit-container"

		args := []string{"cderun", "--image", "alpine", "--ulimit", "nofile=1024:2048", "sh"}

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
		assert.Len(t, cfg.Ulimits, 1)
		assert.Equal(t, "nofile", cfg.Ulimits[0].Name)
		assert.Equal(t, int64(1024), cfg.Ulimits[0].Soft)
		assert.Equal(t, int64(2048), cfg.Ulimits[0].Hard)
	})
}
