package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_EngineOption_Resolution(t *testing.T) {
	imgVal := "alpine"

	t.Run("resolves engine from CLI flag --engine", func(t *testing.T) {
		engineVal := "podman"
		cli := &CLIOptions{Image: &imgVal, Engine: &engineVal}
		res, err := ResolveWithFS("test-tool", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Engine)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("resolves engine from CLI P1 override --cderun-engine", func(t *testing.T) {
		engineVal := "containerd"
		cli := &CLIOptions{Image: &imgVal, CderunEngine: &engineVal}
		res, err := ResolveWithFS("test-tool", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Equal(t, "containerd", res.Runtime)
	})

	t.Run("resolves engine from env var CDERUN_ENGINE", func(t *testing.T) {
		fs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_ENGINE": "podman",
			},
		}
		cli := &CLIOptions{Image: &imgVal}
		res, err := ResolveWithFS("test-tool", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Engine)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("resolves engine from global config engine field", func(t *testing.T) {
		global := &CDERunConfig{
			Engine: "containerd",
		}
		cli := &CLIOptions{Image: &imgVal}
		res, err := ResolveWithFS("test-tool", cli, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Equal(t, "containerd", res.Runtime)
	})

	t.Run("falls back to deprecated runtime flag --runtime", func(t *testing.T) {
		runtimeVal := "podman"
		cli := &CLIOptions{Image: &imgVal, Runtime: &runtimeVal}
		res, err := ResolveWithFS("test-tool", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Engine)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("falls back to deprecated env var CDERUN_RUNTIME", func(t *testing.T) {
		fs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_RUNTIME": "containerd",
			},
		}
		cli := &CLIOptions{Image: &imgVal}
		res, err := ResolveWithFS("test-tool", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Equal(t, "containerd", res.Runtime)
	})

	t.Run("engine option takes precedence when both engine and runtime are specified", func(t *testing.T) {
		engineVal := "containerd"
		runtimeVal := "podman"
		cli := &CLIOptions{
			Image:   &imgVal,
			Engine:  &engineVal,
			Runtime: &runtimeVal,
		}
		res, err := ResolveWithFS("test-tool", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Equal(t, "containerd", res.Runtime)
	})

	t.Run("unsupported engine returns validation error", func(t *testing.T) {
		engineVal := "invalid-engine"
		cli := &CLIOptions{Image: &imgVal, Engine: &engineVal}
		_, err := ResolveWithFS("test-tool", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "unsupported engine") || strings.Contains(err.Error(), "unsupported runtime"), "expected error message to contain 'unsupported engine' or 'unsupported runtime', got: %v", err.Error())
	})
}

func TestUnit_EngineFamily_Helpers(t *testing.T) {
	t.Run("docker and podman are docker-compat-api and API-based", func(t *testing.T) {
		cfgDocker := &ResolvedConfig{Engine: "docker"}
		assert.Equal(t, EngineFamilyDockerCompat, cfgDocker.EngineFamily())
		assert.True(t, cfgDocker.IsAPIBased())
		assert.False(t, cfgDocker.IsCLIBased())

		cfgPodman := &ResolvedConfig{Engine: "podman"}
		assert.Equal(t, EngineFamilyDockerCompat, cfgPodman.EngineFamily())
		assert.True(t, cfgPodman.IsAPIBased())
		assert.False(t, cfgPodman.IsCLIBased())
	})

	t.Run("containerd is containerd-api and API-based", func(t *testing.T) {
		cfgContainerd := &ResolvedConfig{Engine: "containerd"}
		assert.Equal(t, EngineFamilyContainerd, cfgContainerd.EngineFamily())
		assert.True(t, cfgContainerd.IsAPIBased())
		assert.False(t, cfgContainerd.IsCLIBased())
	})

	t.Run("nerdctl is nerdctl family and CLI-based", func(t *testing.T) {
		cfgNerdctl := &ResolvedConfig{Engine: "nerdctl"}
		assert.Equal(t, EngineFamilyNerdctl, cfgNerdctl.EngineFamily())
		assert.False(t, cfgNerdctl.IsAPIBased())
		assert.True(t, cfgNerdctl.IsCLIBased())
	})
}
