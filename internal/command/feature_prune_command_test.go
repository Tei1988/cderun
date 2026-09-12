package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Command_Prune(t *testing.T) {
	t.Parallel()

	t.Run("cderun --prune success with pruned containers", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.PrunedContainers = []string{"c123", "c456"}

		opts := &rootOptions{
			logger: logging.GetGlobalLogger(),
			fs:     config.RealFileSystem{},
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			},
		}

		cmd := &cobra.Command{}
		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)

		resolved := &config.ResolvedConfig{
			Prune:    true,
			LogLevel: "error",
		}

		err := opts.handlePrune(cmd, resolved)
		require.NoError(t, err)
		assert.Contains(t, outBuf.String(), "Pruned 2 orphan container(s): c123, c456")
	})

	t.Run("cderun --prune when no orphan containers exist", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.PrunedContainers = nil

		opts := &rootOptions{
			logger: logging.GetGlobalLogger(),
			fs:     config.RealFileSystem{},
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			},
		}

		cmd := &cobra.Command{}
		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)

		resolved := &config.ResolvedConfig{
			Prune:    true,
			LogLevel: "error",
		}

		err := opts.handlePrune(cmd, resolved)
		require.NoError(t, err)
		assert.Contains(t, outBuf.String(), "No orphan containers found to prune.")
	})

	t.Run("cderun --prune --dry-run mode", func(t *testing.T) {
		opts := &rootOptions{
			logger: logging.GetGlobalLogger(),
			fs:     config.RealFileSystem{},
		}

		cmd := &cobra.Command{}
		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)

		resolved := &config.ResolvedConfig{
			Prune:    true,
			DryRun:   true,
			LogLevel: "error",
		}

		err := opts.handlePrune(cmd, resolved)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(outBuf.String(), "Dry-run mode:"))
	})
}
