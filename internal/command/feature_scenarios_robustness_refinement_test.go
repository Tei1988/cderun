package command

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Command_Robustness_PreprocessArgs_InterleavedHoisting(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	args := []string{
		"cderun",
		"python",
		"script.py",
		"--cderun-image", "python:3.11-slim",
		"--cderun-tty=true",
		"--cderun-env", "FOO=BAR",
		"--",
		"--app-flag",
		"value",
	}

	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"cderun",
		"--cderun-image", "python:3.11-slim",
		"--cderun-tty=true",
		"--cderun-env", "FOO=BAR",
		"python",
		"script.py",
		"--",
		"--app-flag",
		"value",
	}, processed)
}

func TestUnit_Command_Robustness_DryRunJSONStructure(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	opts := defaultOptions()
	cmd := newRootCmd(&opts)
	cmd.SetOut(buf)

	mr := runtime.NewMockRuntime()
	opts.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return mr, nil
	}

	ctx := context.Background()
	cmd.SetArgs([]string{"--dry-run", "--dry-run-format=json", "--image=alpine:latest", "--env=APP_ENV=prod", "--env=SECRET_TOKEN=supersecret", "--sensitive-env=*TOKEN*", "echo", "hello"})

	err := cmd.ExecuteContext(ctx)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(buf.Bytes(), &payload)
	require.NoError(t, err)

	assert.Equal(t, "alpine:latest", payload["image"])
	cmdSlice, ok := payload["command"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"hello"}, cmdSlice)

	envList, ok := payload["env"].([]any)
	require.True(t, ok)
	assert.Contains(t, envList, "APP_ENV=prod")
	assert.Contains(t, envList, "SECRET_TOKEN=[REDACTED]")
}
