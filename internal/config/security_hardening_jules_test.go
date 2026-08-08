package config

import (
	"bytes"
	"testing"

	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Resolver_UlimitControlCharacterHardening(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("Ulimit with control character is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Ulimits: []string{"nofile=1024:\x012048"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("Ulimit with null byte is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Ulimits: []string{"nofile=1024:\x002048"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("Valid ulimit is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Ulimits: []string{"nofile=1024:2048"},
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Ulimits, 1)
		assert.Equal(t, "nofile", res.Ulimits[0].Name)
	})
}

func TestUnit_Config_Resolver_HostPIDWarning(t *testing.T) {
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	t.Run("Host PID mode emits security warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image: ptr("alpine"),
			Pid:   ptr("host"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Container is running with host PID namespace enabled")
	})

	t.Run("Private PID mode does not emit host PID warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image: ptr("alpine"),
			Pid:   ptr(""),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.NotContains(t, logOutput, "Container is running with host PID namespace enabled")
	})
}

func TestUnit_Config_Loader_AbsolutePathHardening(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}
	loader := NewConfigLoaderWithFS(mfs)

	t.Run("Config loader path with control character is rejected", func(t *testing.T) {
		_, _, err := loader.LoadCDERunConfigFromPath("/some/path\x01with/control")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for config path")
	})

	t.Run("Config loader path with null byte is rejected", func(t *testing.T) {
		_, _, err := loader.LoadCDERunConfigFromPath("/some/path\x00with/null")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for config path")
	})

	t.Run("Tools loader path with control character is rejected", func(t *testing.T) {
		_, _, err := loader.LoadToolsConfigFromPath("/some/tools\x01with/control")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for config path")
	})
}
