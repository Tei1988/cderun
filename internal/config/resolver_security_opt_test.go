package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ResolverSecurityOpt(t *testing.T) {
	t.Run("basic security-opt resolution", func(t *testing.T) {
		cli := &CLIOptions{
			Image:       ptr("alpine"),
			SecurityOpt: []string{"no-new-privileges=true", "seccomp=unconfined"},
		}
		global := &CDERunConfig{}
		tools := ToolsConfig{}

		res, err := ResolveWithFS("test", cli, tools, global, RealFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, []string{"no-new-privileges=true", "seccomp=unconfined"}, res.SecurityOpt)
	})

	t.Run("cderun priority override (P1 wins over P2)", func(t *testing.T) {
		cli := &CLIOptions{
			Image:             ptr("alpine"),
			SecurityOpt:       []string{"p2-ignored"},
			CderunSecurityOpt: []string{"no-new-privileges"},
		}
		global := &CDERunConfig{}
		tools := ToolsConfig{}

		res, err := ResolveWithFS("test", cli, tools, global, RealFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, []string{"no-new-privileges"}, res.SecurityOpt)
	})

	t.Run("env var priority overrides (P3 wins over P5)", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
		}
		global := &CDERunConfig{}
		tools := ToolsConfig{}

		// Custom mock FS so that env resolver is happy
		fs := &MockFileSystem{
			Files: map[string][]byte{
				"/.tools.yaml": []byte(`
test:
  image: alpine
  securityOpt: ["p5-yaml"]
`),
			},
			Env: map[string]string{
				"CDERUN_SECURITY_OPT": "no-new-privileges,seccomp:unconfined",
			},
		}

		res, err := ResolveWithFS("test", cli, tools, global, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"no-new-privileges", "seccomp:unconfined"}, res.SecurityOpt)
	})

	t.Run("invalid security opt format triggers validation error", func(t *testing.T) {
		cli := &CLIOptions{
			Image:       ptr("alpine"),
			SecurityOpt: []string{"unsafe_char$"},
		}
		global := &CDERunConfig{}
		tools := ToolsConfig{}

		_, err := ResolveWithFS("test", cli, tools, global, RealFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid security option format")
	})
}
