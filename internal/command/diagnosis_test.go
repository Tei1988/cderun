package command

import (
	"bytes"
	"encoding/json"
	"testing"

	"cderun/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Root_HandleDiagnosis(t *testing.T) {
	t.Run("JSON format", func(t *testing.T) {
		setupTestOptions(t)
		out := &bytes.Buffer{}
		opts := testOptions
		opts.stdout = out
		opts.fs = config.RealFileSystem{}

		cmd := newRootCmd(opts)
		rCfg := &config.ResolvedConfig{
			DiagnosisFormat: "json",
		}
		tCfg := config.ToolsConfig{}
		err := opts.handleDiagnosis(cmd, rCfg, tCfg, []string{"global.yaml"}, []string{"tools.yaml"})

		require.NoError(t, err)
		var res map[string]any
		err = json.Unmarshal(out.Bytes(), &res)
		require.NoError(t, err)
		assert.Equal(t, []any{"global.yaml"}, res["configs"].(map[string]any)["global"])
	})
}
