package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptrString(s string) *string {
	return &s
}

func TestUnit_Config_EngineOption_ResolutionAndPrecedence(t *testing.T) {
	t.Run("Engine CLI P1 override precedence", func(t *testing.T) {
		cli := &CLIOptions{
			Image:        ptrString("alpine"),
			Engine:       ptrString("podman"),
			CderunEngine: ptrString("containerd"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Equal(t, "containerd", res.Runtime)
	})

	t.Run("Engine CLI P2 flag resolution", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptrString("alpine"),
			Engine: ptrString("podman"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Engine)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("Engine environment variable resolution", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptrString("alpine"),
		}
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_ENGINE": "containerd",
			},
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Equal(t, "containerd", res.Runtime)
	})

	t.Run("Engine global config resolution", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptrString("alpine"),
		}
		global := &CDERunConfig{
			Engine: "podman",
		}
		res, err := ResolveWithFS("sh", cli, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Engine)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("Deprecated Runtime option fallback", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptrString("alpine"),
			Runtime: ptrString("podman"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Engine)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("Engine precedence over deprecated Runtime when both specified", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptrString("alpine"),
			Engine:  ptrString("containerd"),
			Runtime: ptrString("docker"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Equal(t, "containerd", res.Runtime)
	})

	t.Run("Invalid engine value rejection", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptrString("alpine"),
			Engine: ptrString("invalid_engine"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported engine: \"invalid_engine\"")
	})
}

func TestUnit_Config_EngineFamily_Classification(t *testing.T) {
	tests := []struct {
		engine         string
		expectedFamily string
		wantAPIBased   bool
		wantCLIBased   bool
	}{
		{
			engine:         "docker",
			expectedFamily: "docker-compat-api",
			wantAPIBased:   true,
			wantCLIBased:   false,
		},
		{
			engine:         "podman",
			expectedFamily: "docker-compat-api",
			wantAPIBased:   true,
			wantCLIBased:   false,
		},
		{
			engine:         "containerd",
			expectedFamily: "containerd-api",
			wantAPIBased:   true,
			wantCLIBased:   false,
		},
		{
			engine:         "nerdctl",
			expectedFamily: "nerdctl",
			wantAPIBased:   false,
			wantCLIBased:   true,
		},
		{
			engine:         "CUSTOM_CONTAINERD",
			expectedFamily: "containerd-api",
			wantAPIBased:   true,
			wantCLIBased:   false,
		},
	}

	for _, tt := range tests {
		t.Run("Classification for "+tt.engine, func(t *testing.T) {
			cfg := &ResolvedConfig{
				Engine: tt.engine,
			}
			assert.Equal(t, tt.expectedFamily, cfg.EngineFamily())
			assert.Equal(t, tt.wantAPIBased, cfg.IsAPIBased())
			assert.Equal(t, tt.wantCLIBased, cfg.IsCLIBased())
		})
	}
}
