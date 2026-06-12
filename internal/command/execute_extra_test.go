package command

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Root_Execute_AttachFailure(t *testing.T) {
	t.Parallel()

	setupMocks := func(o *rootOptions) {
		o.setupSignals = func(chan os.Signal) {}
		o.stopSignalHandling = func(chan os.Signal) {}
		o.isTerminal = func(fd int) bool { return false }
		o.logger = logging.NewLogger()
		o.logger.Init("debug", "text", false)
	}

	t.Run("attachContainer failure early", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			AttachErr: errors.New("attach failed"),
		}
		opts := &rootOptions{
			runtimeFactory: func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			},
		}
		setupMocks(opts)

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		resolved := &config.ResolvedConfig{}
		cc := &container.ContainerConfig{Image: "alpine"}

		_, err := opts.execute(cmd, resolved, cc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to attach to container: attach failed")
	})

	t.Run("attachContainer context cancellation", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			AttachFunc: func(ctx context.Context, id string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
				// Don't close ready, wait for context cancel
				<-ctx.Done()
				return ctx.Err()
			},
		}
		opts := &rootOptions{
			runtimeFactory: func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			},
		}
		setupMocks(opts)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		cmd := &cobra.Command{}
		cmd.SetContext(ctx)
		resolved := &config.ResolvedConfig{}
		cc := &container.ContainerConfig{Image: "alpine"}

		_, err := opts.execute(cmd, resolved, cc)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
