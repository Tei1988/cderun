package runtime

import (
	"testing"

	"cderun/internal/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeature_Runtime_DockerAdapterGPUAndConfigParsing(t *testing.T) {
	t.Parallel()

	t.Run("parseGPUs helper function boundary checks", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			input     string
			expectErr bool
		}{
			{"all GPUs", "all", false},
			{"count GPUs", "count=2", false},
			{"device ID list", "device=0,1", false},
			{"empty input", "", false},
			{"invalid format", "invalid_format", true},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				reqs, err := parseGPUs(tt.input)
				if tt.expectErr {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					if tt.input != "" {
						assert.Len(t, reqs, 1)
					} else {
						assert.Nil(t, reqs)
					}
				}
			})
		}
	})

	t.Run("toDockerContainerConfig mapping validations", func(t *testing.T) {
		t.Parallel()

		cfg := &container.ContainerConfig{
			Image:      "alpine:latest",
			Command:    []string{"echo", "hello"},
			Workdir:    "/app",
			Network:    "bridge",
			CPUShares:  512,
			CpusetCpus: "0-1",
			CpusetMems: "0",
			GPUs:       "all",
			Sysctls:    map[string]string{"net.ipv4.ip_forward": "1"},
		}

		dockerConfig, hostConfig, _, err := toDockerContainerConfig(cfg)
		require.NoError(t, err)

		assert.Equal(t, "alpine:latest", dockerConfig.Image)
		assert.Equal(t, []string{"echo", "hello"}, []string(dockerConfig.Cmd))
		assert.Equal(t, "/app", dockerConfig.WorkingDir)

		assert.Equal(t, int64(512), hostConfig.Resources.CPUShares)
		assert.Equal(t, "0-1", hostConfig.Resources.CpusetCpus)
		assert.Equal(t, "0", hostConfig.Resources.CpusetMems)
		assert.Len(t, hostConfig.Resources.DeviceRequests, 1)
		assert.Equal(t, "1", hostConfig.Sysctls["net.ipv4.ip_forward"])
	})
}

func TestFeature_Runtime_ContainerdValidation(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{}

	t.Run("ValidateConfig rejects unsupported features in containerd", func(t *testing.T) {
		t.Parallel()

		cfg := &container.ContainerConfig{
			Image: "alpine",
			GPUs:  "all", // Unsupported in containerd adapter
		}

		err := rt.ValidateConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gpus is not supported")
	})
}
