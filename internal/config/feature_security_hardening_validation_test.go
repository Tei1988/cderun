package config

import (
	"bytes"
	"io"
	"testing"

	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEnvSecurity_ControlChars(t *testing.T) {
	t.Parallel()

	t.Run("valid env values with newlines and tabs", func(t *testing.T) {
		t.Parallel()
		rv := &resolver{
			res: &ResolvedConfig{
				Image: "alpine:latest",
				Env: []string{
					"FOO=bar\nbaz\tqux\r\n",
					"KEY=NORMAL_VALUE",
				},
			},
		}
		err := rv.validateEnvSecurity()
		assert.NoError(t, err)
	})

	t.Run("null byte in env value rejected", func(t *testing.T) {
		t.Parallel()
		rv := &resolver{
			res: &ResolvedConfig{
				Image: "alpine:latest",
				Env: []string{
					"FOO=bar\x00baz",
				},
			},
		}
		err := rv.validateEnvSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})

	t.Run("ASCII control char in env value rejected", func(t *testing.T) {
		t.Parallel()
		rv := &resolver{
			res: &ResolvedConfig{
				Image: "alpine:latest",
				Env: []string{
					"FOO=bar\x1fbaz",
				},
			},
		}
		err := rv.validateEnvSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid control character")
	})

	t.Run("ASCII DEL char in env value rejected", func(t *testing.T) {
		t.Parallel()
		rv := &resolver{
			res: &ResolvedConfig{
				Image: "alpine:latest",
				Env: []string{
					"FOO=bar\x7fbaz",
				},
			},
		}
		err := rv.validateEnvSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid control character DEL")
	})

	t.Run("C1 control char in env value rejected", func(t *testing.T) {
		t.Parallel()
		rv := &resolver{
			res: &ResolvedConfig{
				Image: "alpine:latest",
				Env: []string{
					"FOO=bar\u0085baz",
				},
			},
		}
		err := rv.validateEnvSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid C1 control character")
	})
}

func TestValidateSecurity_UnconfinedWarnings(t *testing.T) {
	var buf bytes.Buffer
	logging.Init("warn", "text", false)
	logging.SetOutput(&buf)
	defer logging.SetOutput(io.Discard)

	rv := &resolver{
		cli: &CLIOptions{},
		fs:  RealFileSystem{},
		res: &ResolvedConfig{
			Image:          "alpine:latest",
			Runtime:        "docker",
			PullMaxRetries: 3,
			SecurityOpt: []string{
				"seccomp=unconfined",
				"apparmor=unconfined",
				"label=disable",
				"systempaths=unconfined",
				"no-new-privileges=false",
			},
		},
	}

	err := rv.validateSecurity()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "seccomp=unconfined")
	assert.Contains(t, out, "apparmor=unconfined")
	assert.Contains(t, out, "label=disable")
	assert.Contains(t, out, "systempaths=unconfined")
	assert.Contains(t, out, "no-new-privileges=false")
	assert.Contains(t, out, "disables default container security isolation")
}
