package runtime

import (
	"context"
	"testing"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

// ConformanceCapabilities describes what features a runtime adapter supports or rejects.
type ConformanceCapabilities struct {
	SupportsVolumes     bool
	SupportsTmpfs       bool
	SupportsPorts       bool
	SupportsGPUs        bool
	SupportsDNSSearch   bool
	SupportsDNSOptions  bool
	SupportsInit        bool
	RequiresCapPrefix   bool // OCI spec requires CAP_ prefix
}

// RunConformanceTests executes the standard L3 ContainerRuntime conformance test suite against
// any ContainerRuntime adapter implementation.
//
// Ref: docs/testing/strategy.md (L3: Conformance Suite)
// Ref: docs/features/multi-runtime-support.md
func RunConformanceTests(t *testing.T, factory func(t *testing.T) ContainerRuntime, caps ConformanceCapabilities) {
	t.Run("Name", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()
		name := rt.Name()
		assert.NotEmpty(t, name, "Runtime Name() must not be empty")
	})

	t.Run("ValidateConfig_ValidBasicConfig", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
			Pull:    "always",
		}
		err := rt.ValidateConfig(cfg)
		assert.NoError(t, err, "Basic valid ContainerConfig must pass ValidateConfig")
	})

	t.Run("ValidateConfig_VolumeMountContract_T51", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Pull:  "missing",
			Mounts: []container.Mount{
				{
					Type:   "volume",
					Source: "my-vol",
					Target: "/data",
				},
			},
		}

		err := rt.ValidateConfig(cfg)
		if caps.SupportsVolumes {
			assert.NoError(t, err, "Volume mounts should be supported by this runtime")
		} else {
			require.Error(t, err, "Unsupported volume mounts must be explicitly rejected (T51 contract)")
			assert.Contains(t, err.Error(), "volume", "Error message should mention volume mount")
		}
	})

	t.Run("ValidateConfig_TmpfsMountContract_T51", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Pull:  "missing",
			Mounts: []container.Mount{
				{
					Type:   "tmpfs",
					Target: "/tmp",
				},
			},
		}

		err := rt.ValidateConfig(cfg)
		if caps.SupportsTmpfs {
			assert.NoError(t, err, "Valid tmpfs mounts must pass ValidateConfig (T51 contract)")
		} else {
			require.Error(t, err, "Unsupported tmpfs mounts must be explicitly rejected")
		}
	})

	t.Run("ValidateConfig_PortsContract", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Pull:  "missing",
			Ports: []string{"8080:80"},
		}

		err := rt.ValidateConfig(cfg)
		if caps.SupportsPorts {
			assert.NoError(t, err, "Ports mapping should be supported by this runtime")
		} else {
			require.Error(t, err, "Unsupported ports mapping must be explicitly rejected")
		}
	})

	t.Run("ValidateConfig_DNSSearchContract", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image:     "alpine:latest",
			Pull:      "missing",
			DNSSearch: []string{"example.com"},
		}

		err := rt.ValidateConfig(cfg)
		if caps.SupportsDNSSearch {
			assert.NoError(t, err, "DNS search should be supported by this runtime")
		} else {
			require.Error(t, err, "Unsupported DNS search must be explicitly rejected")
		}
	})

	t.Run("ValidateConfig_DNSOptionsContract", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image:      "alpine:latest",
			Pull:       "missing",
			DNSOptions: []string{"ndots:5"},
		}

		err := rt.ValidateConfig(cfg)
		if caps.SupportsDNSOptions {
			assert.NoError(t, err, "DNS options should be supported by this runtime")
		} else {
			require.Error(t, err, "Unsupported DNS options must be explicitly rejected")
		}
	})

	t.Run("ValidateConfig_UnsupportedGPUsContract", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Pull:  "missing",
			GPUs:  "all",
		}

		err := rt.ValidateConfig(cfg)
		if caps.SupportsGPUs {
			assert.NoError(t, err, "GPUs should be supported by this runtime")
		} else {
			require.Error(t, err, "Unsupported GPUs option must be explicitly rejected")
		}
	})

	t.Run("ValidateConfig_CapabilitiesNormalizationContract_T45", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Pull:    "missing",
			CapAdd:  []string{"SYS_ADMIN", "NET_ADMIN"},
			CapDrop: []string{"CHOWN"},
		}

		err := rt.ValidateConfig(cfg)
		assert.NoError(t, err, "Capabilities should pass validation when valid names are supplied")

		if caps.RequiresCapPrefix {
			normAdd := normalizeCapabilities(cfg.CapAdd)
			assert.Equal(t, []string{"CAP_SYS_ADMIN", "CAP_NET_ADMIN"}, normAdd)
			normDrop := normalizeCapabilities(cfg.CapDrop)
			assert.Equal(t, []string{"CAP_CHOWN"}, normDrop)
		}
	})

	t.Run("ValidateConfig_FullOptionsContract", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image:       "alpine:latest",
			Command:     []string{"echo", "test"},
			Workdir:     "/workspace",
			User:        "1000:1000",
			Memory:      1024 * 1024 * 128,
			CPUs:        1.0,
			CPUShares:   512,
			CpusetCpus:  "0-1",
			CpusetMems:  "0",
			PidsLimit:   100,
			ShmSize:     "64m",
			Init:        caps.SupportsInit,
			SecurityOpt: []string{"no-new-privileges"},
			Sysctls:     map[string]string{"net.ipv4.ip_forward": "1"},
			Env:         []string{"ENV_A=valA", "ENV_B=valB"},
			Pull:        "missing",
		}

		err := rt.ValidateConfig(cfg)
		assert.NoError(t, err, "Valid full ContainerConfig must pass ValidateConfig")
	})

	t.Run("ValidateConfig_InitContract", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Pull:  "missing",
			Init:  true,
		}

		err := rt.ValidateConfig(cfg)
		if caps.SupportsInit {
			assert.NoError(t, err, "Init option should be supported by this runtime")
		} else {
			require.Error(t, err, "Unsupported Init option must be explicitly rejected")
			assert.Contains(t, err.Error(), "init", "Error message should mention init")
		}
	})

	t.Run("Lifecycle_CreateStartWaitInspectRemove", func(t *testing.T) {
		rt := factory(t)
		defer rt.Close()

		ctx := context.Background()
		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"true"},
			Pull:    "missing",
		}

		err := rt.ValidateConfig(cfg)
		require.NoError(t, err)

		id, err := rt.CreateContainer(ctx, cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		err = rt.StartContainer(ctx, id)
		require.NoError(t, err)

		code, err := rt.WaitContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 0, code)

		isRunning, exitCode, err := rt.InspectContainer(ctx, id)
		require.NoError(t, err)
		assert.False(t, isRunning)
		assert.Equal(t, 0, exitCode)

		err = rt.RemoveContainer(ctx, id)
		assert.NoError(t, err)
	})
}

func TestConformance_MockRuntime(t *testing.T) {
	factory := func(t *testing.T) ContainerRuntime {
		m := NewMockRuntime()
		m.CreatedContainerID = "mock-container-123"
		return m
	}
	caps := ConformanceCapabilities{
		SupportsVolumes:    true,
		SupportsTmpfs:      true,
		SupportsPorts:      true,
		SupportsGPUs:       true,
		SupportsDNSSearch:  true,
		SupportsDNSOptions: true,
		SupportsInit:       true,
		RequiresCapPrefix:  false,
	}
	RunConformanceTests(t, factory, caps)
}

func TestConformance_PodmanRuntime_MockClient(t *testing.T) {
	factory := func(t *testing.T) ContainerRuntime {
		mock := &mockDockerClient{
			createResp: dockercontainer.CreateResponse{ID: "podman-container-456"},
			waitResp:   dockercontainer.WaitResponse{StatusCode: 0},
			inspectResp: dockercontainer.InspectResponse{
				ContainerJSONBase: &dockercontainer.ContainerJSONBase{
					State: &dockercontainer.State{
						Running:  false,
						ExitCode: 0,
					},
				},
			},
		}
		rt := &DockerRuntime{
			logger:       logging.GetGlobalLogger(),
			client:       mock,
			name:         "podman",
			sleepFunc:    noopSleepFunc,
			removeOnExit: make(map[string]bool),
		}
		return rt
	}
	caps := ConformanceCapabilities{
		SupportsVolumes:    true,
		SupportsTmpfs:      true,
		SupportsPorts:      true,
		SupportsGPUs:       true,
		SupportsDNSSearch:  true,
		SupportsDNSOptions: true,
		SupportsInit:       true,
		RequiresCapPrefix:  false,
	}
	RunConformanceTests(t, factory, caps)
}

func TestConformance_DockerRuntime_MockClient(t *testing.T) {
	factory := func(t *testing.T) ContainerRuntime {
		mock := &mockDockerClient{
			createResp: dockercontainer.CreateResponse{ID: "docker-container-123"},
			waitResp:   dockercontainer.WaitResponse{StatusCode: 0},
			inspectResp: dockercontainer.InspectResponse{
				ContainerJSONBase: &dockercontainer.ContainerJSONBase{
					State: &dockercontainer.State{
						Running:  false,
						ExitCode: 0,
					},
				},
			},
		}
		rt := &DockerRuntime{
			logger:       logging.GetGlobalLogger(),
			client:       mock,
			name:         "docker",
			sleepFunc:    noopSleepFunc,
			removeOnExit: make(map[string]bool),
		}
		return rt
	}
	caps := ConformanceCapabilities{
		SupportsVolumes:    true,
		SupportsTmpfs:      true,
		SupportsPorts:      true,
		SupportsGPUs:       true,
		SupportsDNSSearch:  true,
		SupportsDNSOptions: true,
		SupportsInit:       true,
		RequiresCapPrefix:  false,
	}
	RunConformanceTests(t, factory, caps)
}
