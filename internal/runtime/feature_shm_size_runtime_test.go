package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Docker_toDockerContainerConfig_ShmSize(t *testing.T) {
	cfg := &container.ContainerConfig{
		ShmSize: "256m",
	}

	_, hostConfig, _, err := toDockerContainerConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, int64(268435456), hostConfig.ShmSize)
}

func TestUnit_Containerd_ShmSize(t *testing.T) {
	// Verify that the config validation works as expected
	cfg := &container.ContainerConfig{
		ShmSize: "512m",
	}

	rt := &ContainerdRuntime{}
	err := rt.ValidateConfig(cfg)
	require.NoError(t, err)

	// Verify that invalid formats are rejected
	cfgInvalid := &container.ContainerConfig{
		ShmSize: "invalid",
	}
	err = rt.ValidateConfig(cfgInvalid)
	require.Error(t, err)

	cfgNegative := &container.ContainerConfig{
		ShmSize: "-256m",
	}
	err = rt.ValidateConfig(cfgNegative)
	require.Error(t, err)

	// Test the custom oci.SpecOpts returned by getShmSizeSpecOpt for creation behavior
	t.Run("creation behavior (not present in s.Mounts)", func(t *testing.T) {
		spec := &specs.Spec{
			Mounts: []specs.Mount{
				{Destination: "/tmp", Type: "tmpfs"},
			},
		}

		opt := getShmSizeSpecOpt(512 * 1024 * 1024) // 512m
		err := opt(context.Background(), nil, nil, spec)
		require.NoError(t, err)

		// Assert that exactly one /dev/shm is added
		var shmMounts []specs.Mount
		for _, m := range spec.Mounts {
			if m.Destination == "/dev/shm" {
				shmMounts = append(shmMounts, m)
			}
		}

		require.Len(t, shmMounts, 1)
		assert.Equal(t, "tmpfs", shmMounts[0].Type)
		assert.Contains(t, shmMounts[0].Options, "size=536870912")
		assert.Contains(t, shmMounts[0].Options, "mode=1777")
	})

	// Test the custom oci.SpecOpts returned by getShmSizeSpecOpt for replacement behavior
	t.Run("replacement behavior (already present in s.Mounts)", func(t *testing.T) {
		spec := &specs.Spec{
			Mounts: []specs.Mount{
				{Destination: "/dev/shm", Type: "tmpfs", Options: []string{"nosuid", "size=64m"}},
			},
		}

		opt := getShmSizeSpecOpt(512 * 1024 * 1024) // 512m
		err := opt(context.Background(), nil, nil, spec)
		require.NoError(t, err)

		// Assert that exactly one /dev/shm exists and the size is updated
		var shmMounts []specs.Mount
		for _, m := range spec.Mounts {
			if m.Destination == "/dev/shm" {
				shmMounts = append(shmMounts, m)
			}
		}

		require.Len(t, shmMounts, 1)
		assert.Contains(t, shmMounts[0].Options, "size=536870912")
		// The old size option should have been replaced
		assert.NotContains(t, shmMounts[0].Options, "size=64m")
		assert.Contains(t, shmMounts[0].Options, "nosuid")
	})

	// Test the custom oci.SpecOpts returned by getShmSizeSpecOpt for non-tmpfs mounts rejection
	t.Run("rejection of non-tmpfs mount", func(t *testing.T) {
		spec := &specs.Spec{
			Mounts: []specs.Mount{
				{Destination: "/dev/shm", Type: "bind", Options: []string{"rbind"}},
			},
		}

		opt := getShmSizeSpecOpt(512 * 1024 * 1024) // 512m
		err := opt(context.Background(), nil, nil, spec)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot set shm-size on a non-tmpfs mount")

		// Verify size= was not applied to the bind mount
		assert.NotContains(t, spec.Mounts[0].Options, "size=536870912")
	})
}
