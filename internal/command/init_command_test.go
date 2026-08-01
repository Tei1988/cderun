package command

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
)

func TestUnit_Command_InitOption_DryRun(t *testing.T) {
	t.Parallel()

	t.Run("Verify --init flag is parsed and passed to ContainerConfig in DryRun", func(t *testing.T) {
		out := &bytes.Buffer{}
		err := ExecuteContextWithOptions(
			context.Background(),
			[]string{"cderun", "--image", "alpine", "--init", "--dry-run", "--dry-run-format=json", "sh"},
			func(o *rootOptions, cmd *cobra.Command) {
				cmd.SetOut(out)
				mfs := &config.MockFileSystem{}
				o.fs = mfs
				o.configLoader = config.NewConfigLoaderWithFS(mfs)
			},
		)
		require.NoError(t, err)

		var cfg struct {
			Init bool `json:"init"`
		}
		err = json.Unmarshal(out.Bytes(), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.Init)
	})

	t.Run("Verify --cderun-init override flag is hoisted, parsed, and passed in DryRun", func(t *testing.T) {
		out := &bytes.Buffer{}
		err := ExecuteContextWithOptions(
			context.Background(),
			[]string{"cderun", "--image", "alpine", "--dry-run", "--dry-run-format=json", "sh", "--cderun-init"},
			func(o *rootOptions, cmd *cobra.Command) {
				cmd.SetOut(out)
				mfs := &config.MockFileSystem{}
				o.fs = mfs
				o.configLoader = config.NewConfigLoaderWithFS(mfs)
			},
		)
		require.NoError(t, err)

		var cfg struct {
			Init bool `json:"init"`
		}
		err = json.Unmarshal(out.Bytes(), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.Init)
	})
}
