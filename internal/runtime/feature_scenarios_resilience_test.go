package runtime

import (
	"bytes"
	"context"
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_ScenariosResilience_MockAndDockerMapping(t *testing.T) {
	t.Parallel()

	t.Run("mock_runtime_attach_and_resize_operations", func(t *testing.T) {
		t.Parallel()

		mock := NewMockRuntime()
		ctx := context.Background()

		cc := &container.ContainerConfig{
			Image:   "ubuntu:latest",
			Command: []string{"bash"},
			TTY:     true,
		}

		id, err := mock.CreateContainer(ctx, cc)
		require.NoError(t, err)

		// Test AttachContainer
		stdin := bytes.NewBufferString("echo hello\n")
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		ready := make(chan struct{})

		attachErr := mock.AttachContainer(ctx, id, true, stdin, stdout, stderr, ready)
		require.NoError(t, attachErr)

		// Test ResizeContainerTTY
		resizeErr := mock.ResizeContainerTTY(ctx, id, 80, 24)
		require.NoError(t, resizeErr)

		// Test InspectContainer
		running, exitCodeInspect, inspectErr := mock.InspectContainer(ctx, id)
		require.NoError(t, inspectErr)
		assert.False(t, running)
		assert.Equal(t, 0, exitCodeInspect)

		// Test WaitContainer
		exitCode, waitErr := mock.WaitContainer(ctx, id)
		require.NoError(t, waitErr)
		assert.Equal(t, 0, exitCode)
	})

	t.Run("docker_adapter_resource_and_security_mapping", func(t *testing.T) {
		t.Parallel()

		cc := &container.ContainerConfig{
			Image:      "alpine:latest",
			ReadOnly:   true,
			Memory:     268435456, // 256MB
			CPUs:       1.5,
			CPUShares:  512,
			CpusetCpus: "0-1",
			CpusetMems: "0",
			CapAdd:     []string{"SYS_PTRACE", "NET_RAW"},
			CapDrop:    []string{"CHOWN"},
			SecurityOpt: []string{
				"no-new-privileges:true",
				"seccomp=unconfined",
			},
			Devices: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/fuse",
					PathInContainer:   "/dev/fuse",
					CgroupPermissions: "rwm",
				},
			},
		}

		containerConfig, hostConfig, _, err := toDockerContainerConfig(cc)
		require.NoError(t, err)
		require.NotNil(t, containerConfig)
		require.NotNil(t, hostConfig)

		// ReadOnly Rootfs
		assert.True(t, hostConfig.ReadonlyRootfs)

		// Resources
		assert.Equal(t, int64(268435456), hostConfig.Memory)
		assert.Equal(t, int64(1500000000), hostConfig.NanoCPUs)
		assert.Equal(t, int64(512), hostConfig.CPUShares)
		assert.Equal(t, "0-1", hostConfig.CpusetCpus)
		assert.Equal(t, "0", hostConfig.CpusetMems)

		// Capabilities
		assert.Equal(t, []string{"SYS_PTRACE", "NET_RAW"}, []string(hostConfig.CapAdd))
		assert.Equal(t, []string{"CHOWN"}, []string(hostConfig.CapDrop))

		// Security Options
		assert.Equal(t, []string{"no-new-privileges:true", "seccomp=unconfined"}, []string(hostConfig.SecurityOpt))

		// Devices
		require.Len(t, hostConfig.Resources.Devices, 1)
		assert.Equal(t, "/dev/fuse", hostConfig.Resources.Devices[0].PathOnHost)
		assert.Equal(t, "/dev/fuse", hostConfig.Resources.Devices[0].PathInContainer)
		assert.Equal(t, "rwm", hostConfig.Resources.Devices[0].CgroupPermissions)
	})

	t.Run("containerd_adapter_validation_rules", func(t *testing.T) {
		t.Parallel()

		rt := &ContainerdRuntime{}

		// ValidateConfig checks unsupported features for containerd
		validCC := &container.ContainerConfig{
			Image: "alpine:latest",
		}
		require.NoError(t, rt.ValidateConfig(validCC))

		// Unsupported network mode or ports
		invalidNetCC := &container.ContainerConfig{
			Image: "alpine:latest",
			Ports: []string{"8080:80"},
		}
		err := rt.ValidateConfig(invalidNetCC)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port mapping is not supported yet")
	})
}
