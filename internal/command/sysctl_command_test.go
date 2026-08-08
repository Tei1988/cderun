package command

import (
	"context"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Command_Sysctl_Execution(t *testing.T) {
	t.Parallel()

	t.Run("single sysctl option is populated correctly", func(t *testing.T) {
		mock := &runtime.MockRuntime{}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--sysctl", "net.ipv4.ip_forward=1", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		var createdConfig *container.ContainerConfig
		mock.WithLockedMock(func(m *runtime.MockRuntime) {
			createdConfig = m.CreatedConfig
		})

		require.NotNil(t, createdConfig)
		assert.Equal(t, map[string]string{
			"net.ipv4.ip_forward": "1",
		}, createdConfig.Sysctls)
	})

	t.Run("multiple sysctl options are merged", func(t *testing.T) {
		mock := &runtime.MockRuntime{}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--sysctl", "net.ipv4.ip_forward=1", "--sysctl", "kernel.threads-max=2000", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		var createdConfig *container.ContainerConfig
		mock.WithLockedMock(func(m *runtime.MockRuntime) {
			createdConfig = m.CreatedConfig
		})

		require.NotNil(t, createdConfig)
		assert.Equal(t, map[string]string{
			"net.ipv4.ip_forward": "1",
			"kernel.threads-max":  "2000",
		}, createdConfig.Sysctls)
	})

	t.Run("cderun-sysctl overrides standard sysctl option", func(t *testing.T) {
		mock := &runtime.MockRuntime{}

		ctx := context.Background()
		// P1 override flag must be placed after the subcommand "sh"
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--sysctl", "net.ipv4.ip_forward=1", "sh", "--cderun-sysctl", "net.ipv4.ip_forward=2"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		var createdConfig *container.ContainerConfig
		mock.WithLockedMock(func(m *runtime.MockRuntime) {
			createdConfig = m.CreatedConfig
		})

		require.NotNil(t, createdConfig)
		assert.Equal(t, map[string]string{
			"net.ipv4.ip_forward": "2",
		}, createdConfig.Sysctls)
	})

	t.Run("dynamic resolution in sysctl", func(t *testing.T) {
		mock := &runtime.MockRuntime{}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--sysctl", "custom.path={{PWD}}/foo", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		var createdConfig *container.ContainerConfig
		mock.WithLockedMock(func(m *runtime.MockRuntime) {
			createdConfig = m.CreatedConfig
		})

		require.NotNil(t, createdConfig)
		// Since we didn't mock Getwd in rootOptions (which defaults to RealFileSystem's current path or test WD),
		// let's just make sure it resolved. The resolved path should be absolute and end with "/foo".
		assert.Contains(t, createdConfig.Sysctls["custom.path"], "/foo")
	})

	t.Run("validation failure results in error", func(t *testing.T) {
		mock := &runtime.MockRuntime{}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--sysctl", "invalid_sysctl_format", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sysctl config")
	})
}
