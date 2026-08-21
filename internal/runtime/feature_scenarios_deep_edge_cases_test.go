package runtime

import (
	"testing"

	"cderun/internal/container"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeature_Runtime_DockerAdapterGPUAndConfigParsing(t *testing.T) {
	t.Parallel()

	t.Run("parseGPUs helper function boundary checks", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name               string
			input              string
			expectErr          bool
			expectedDriver     string
			expectedCount      int
			expectedCaps       [][]string
			expectedDeviceIDs  []string
			expectedReqsLength int
		}{
			{
				name:               "all GPUs",
				input:              "all",
				expectErr:          false,
				expectedDriver:     "nvidia",
				expectedCount:      -1,
				expectedCaps:       [][]string{{"gpu"}},
				expectedDeviceIDs:  nil,
				expectedReqsLength: 1,
			},
			{
				name:               "count GPUs",
				input:              "count=2",
				expectErr:          false,
				expectedDriver:     "nvidia",
				expectedCount:      2,
				expectedCaps:       [][]string{{"gpu"}},
				expectedDeviceIDs:  nil,
				expectedReqsLength: 1,
			},
			{
				name:               "device ID list",
				input:              "device=0,1",
				expectErr:          false,
				expectedDriver:     "nvidia",
				expectedCount:      0,
				expectedCaps:       [][]string{{"gpu"}},
				expectedDeviceIDs:  []string{"0", "1"},
				expectedReqsLength: 1,
			},
			{
				name:               "empty input",
				input:              "",
				expectErr:          false,
				expectedReqsLength: 0,
			},
			{
				name:      "invalid format",
				input:     "invalid_format",
				expectErr: true,
			},
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
					require.Len(t, reqs, tt.expectedReqsLength)
					if tt.expectedReqsLength > 0 {
						req := reqs[0]
						assert.Equal(t, tt.expectedDriver, req.Driver)
						assert.Equal(t, tt.expectedCount, req.Count)
						assert.Equal(t, tt.expectedCaps, req.Capabilities)
						assert.Equal(t, tt.expectedDeviceIDs, req.DeviceIDs)
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
		require.Len(t, hostConfig.DeviceRequests, 1)

		expectedGPUReq := dockercontainer.DeviceRequest{
			Driver:       "nvidia",
			Count:        -1,
			Capabilities: [][]string{{"gpu"}},
		}
		assert.Equal(t, expectedGPUReq, hostConfig.DeviceRequests[0])
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
