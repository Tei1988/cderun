package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveOciRuntime_LegacyEngineRejection(t *testing.T) {
	t.Parallel()

	legacyEngines := []string{"docker", "podman", "containerd", "nerdctl", "DOCKER", " Podman "}

	for _, eng := range legacyEngines {
		eng := eng
		t.Run("Flag_Runtime_"+eng, func(t *testing.T) {
			t.Parallel()
			cli := &CLIOptions{
				Image:   ptrVal("alpine"),
				Runtime: ptrVal(eng),
			}
			_, err := Resolve("git", cli, nil, nil)
			require.Error(t, err)

			var invErr *InvalidConfigError
			require.True(t, errors.As(err, &invErr), "expected InvalidConfigError, got %T: %v", err, err)
			assert.Equal(t, "runtime", invErr.Field)
			assert.Contains(t, invErr.Err.Error(), "container engine specification has moved to '--engine'")
		})

		t.Run("Env_CDERUN_RUNTIME_"+eng, func(t *testing.T) {
			t.Parallel()
			mockFS := &MockFileSystem{
				Env: map[string]string{
					"CDERUN_RUNTIME": eng,
				},
			}

			cli := &CLIOptions{
				Image: ptrVal("alpine"),
			}

			_, err := ResolveWithFS("git", cli, nil, nil, mockFS)
			require.Error(t, err)

			var invErr *InvalidConfigError
			require.True(t, errors.As(err, &invErr), "expected InvalidConfigError, got %T: %v", err, err)
			assert.Equal(t, "runtime", invErr.Field)
			assert.Contains(t, invErr.Err.Error(), "container engine specification has moved to '--engine'")
		})
	}
}

func TestResolveOciRuntime_ExpressionLegacyEngineRejection(t *testing.T) {
	t.Parallel()

	mockFS := &MockFileSystem{
		Env: map[string]string{
			"MY_ENG": "containerd",
		},
	}

	cli := &CLIOptions{
		Image:   ptrVal("alpine"),
		Runtime: ptrVal("{{env:MY_ENG}}"),
	}

	_, err := ResolveWithFS("git", cli, nil, nil, mockFS)
	require.Error(t, err)

	var invErr *InvalidConfigError
	require.True(t, errors.As(err, &invErr), "expected InvalidConfigError, got %T: %v", err, err)
	assert.Equal(t, "runtime", invErr.Field)
	assert.Contains(t, invErr.Err.Error(), "container engine specification has moved to '--engine'")
}

func TestResolveOciRuntime_PrecedenceLayers(t *testing.T) {
	t.Parallel()

	t.Run("P1_Override_OciRuntime_Over_Runtime", func(t *testing.T) {
		t.Parallel()
		cli := &CLIOptions{
			Image:            ptrVal("alpine"),
			CderunOciRuntime: ptrVal("crun"),
			CderunRuntime:    ptrVal("runc"),
		}
		res, err := Resolve("git", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "crun", res.OciRuntime)
	})

	t.Run("P2_CLI_OciRuntime_Over_Runtime", func(t *testing.T) {
		t.Parallel()
		cli := &CLIOptions{
			Image:      ptrVal("alpine"),
			OciRuntime: ptrVal("kata-runtime"),
			Runtime:    ptrVal("crun"),
		}
		res, err := Resolve("git", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "kata-runtime", res.OciRuntime)
	})

	t.Run("P3_Env_CDERUN_OCI_RUNTIME_Over_CDERUN_RUNTIME", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_OCI_RUNTIME": "gvisor",
				"CDERUN_RUNTIME":     "runc",
			},
		}

		cli := &CLIOptions{
			Image: ptrVal("alpine"),
		}

		res, err := ResolveWithFS("git", cli, nil, nil, mockFS)
		require.NoError(t, err)
		assert.Equal(t, "gvisor", res.OciRuntime)
	})

	t.Run("P4_ToolConfig_OciRuntime_Over_Runtime", func(t *testing.T) {
		t.Parallel()
		cli := &CLIOptions{
			Image: ptrVal("alpine"),
		}
		tools := ToolsConfig{
			"git": ToolConfig{
				OciRuntime: "youki",
				Runtime:    "crun",
			},
		}
		res, err := Resolve("git", cli, tools, nil)
		require.NoError(t, err)
		assert.Equal(t, "youki", res.OciRuntime)
	})

	t.Run("P5_GlobalConfig_Defaults_And_Root_Runtime", func(t *testing.T) {
		t.Parallel()
		cli := &CLIOptions{
			Image: ptrVal("alpine"),
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				OciRuntime: "crun",
				Runtime:    "runc",
			},
		}
		res, err := Resolve("git", cli, nil, global)
		require.NoError(t, err)
		assert.Equal(t, "crun", res.OciRuntime)
	})

	t.Run("Expression_Resolution_Valid_OciRuntime", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			Env: map[string]string{
				"CUSTOM_OCI": "crun",
			},
		}

		cli := &CLIOptions{
			Image:      ptrVal("alpine"),
			OciRuntime: ptrVal("{{env:CUSTOM_OCI}}"),
		}
		res, err := ResolveWithFS("git", cli, nil, nil, mockFS)
		require.NoError(t, err)
		assert.Equal(t, "crun", res.OciRuntime)
	})
}

func TestEngineClassification_Methods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		engine     string
		family     EngineFamily
		isAPIBased bool
		isCLIBased bool
	}{
		{
			engine:     "docker",
			family:     EngineFamilyDockerCompat,
			isAPIBased: true,
			isCLIBased: false,
		},
		{
			engine:     "Podman",
			family:     EngineFamilyDockerCompat,
			isAPIBased: true,
			isCLIBased: false,
		},
		{
			engine:     "containerd",
			family:     EngineFamilyContainerd,
			isAPIBased: true,
			isCLIBased: false,
		},
		{
			engine:     "nerdctl",
			family:     EngineFamilyNerdctl,
			isAPIBased: false,
			isCLIBased: true,
		},
		{
			engine:     "custom-unknown",
			family:     EngineFamilyDockerCompat,
			isAPIBased: true,
			isCLIBased: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("Engine_"+tt.engine, func(t *testing.T) {
			t.Parallel()
			cfg := &ResolvedConfig{
				Engine: tt.engine,
			}
			assert.Equal(t, tt.family, cfg.EngineFamily())
			assert.Equal(t, tt.isAPIBased, cfg.IsAPIBased())
			assert.Equal(t, tt.isCLIBased, cfg.IsCLIBased())
		})
	}
}

func TestSocketAndEngineAutoDetectionHelpers(t *testing.T) {
	t.Parallel()

	t.Run("DeduceEngineFromSocketPath", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "podman", deduceEngineFromSocketPath("/run/user/1000/podman/podman.sock"))
		assert.Equal(t, "containerd", deduceEngineFromSocketPath("/run/containerd/containerd.sock"))
		assert.Equal(t, "docker", deduceEngineFromSocketPath("/var/run/docker.sock"))
		assert.Equal(t, "docker", deduceEngineFromSocketPath("/var/run/custom.sock"))
	})

	t.Run("DefaultSocketPathForEngine", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "/run/podman/podman.sock", defaultSocketPathForEngine("podman"))
		assert.Equal(t, "/run/containerd/containerd.sock", defaultSocketPathForEngine("containerd"))
		assert.Equal(t, "/run/containerd/containerd.sock", defaultSocketPathForEngine("nerdctl"))
		assert.Equal(t, "/var/run/docker.sock", defaultSocketPathForEngine("docker"))
		assert.Equal(t, "/var/run/docker.sock", defaultSocketPathForEngine("unknown"))
	})
}
