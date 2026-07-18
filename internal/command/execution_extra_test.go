package command

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Root_Execution_Extra(t *testing.T) {
	t.Run("propagate non-zero exit code from container", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			CreatedContainerID: "c1",
			ExitCode:           123,
		}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 123, exitErr.Code)
	})

	emptySubcmdTests := []struct {
		name       string
		args       []string
		wantErrMsg string
	}{
		{
			name:       "empty subcommand returns error in dry-run (T42 regression)",
			args:       []string{"cderun", "--dry-run", ""},
			wantErrMsg: "--dry-run requires a subcommand",
		},
		{
			name: "empty subcommand does not panic in normal run (T42 regression)",
			args: []string{"cderun", ""},
		},
	}

	for _, tt := range emptySubcmdTests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var runErr error
			require.NotPanics(t, func() {
				runErr = ExecuteContextWithOptions(ctx, tt.args, func(o *rootOptions, cmd *cobra.Command) {
					o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
						return &runtime.MockRuntime{}, nil
					}
					o.exitFunc = func(code int) {}
					o.isTerminal = func(fd int) bool { return false }
					cmd.SetOut(io.Discard)
					cmd.SetErr(io.Discard)
				})
			})
			if tt.wantErrMsg != "" {
				assert.ErrorContains(t, runErr, tt.wantErrMsg)
			} else {
				assert.NoError(t, runErr)
			}
		})
	}

	t.Run("propagate internal error with 125 exit code", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			CreateErr: errors.New("runtime error"),
		}

		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "runtime error")
	})
}
