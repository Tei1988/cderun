package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainerConfigInitialization(t *testing.T) {
	config := ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"sh"},
		Args:        []string{"-c", "echo hello"},
		TTY:         true,
		Interactive: true,
		Remove:      true,
		Network:     "bridge",
		Volumes: []VolumeMount{
			{
				HostPath:      "/tmp",
				ContainerPath: "/data",
				ReadOnly:      false,
			},
		},
		Env:     []string{"FOO=BAR"},
		Workdir: "/workspace",
		User:    "1000",
	}

	assert.Equal(t, "alpine:latest", config.Image)
	assert.Equal(t, []string{"sh"}, config.Command)
	assert.Equal(t, []string{"-c", "echo hello"}, config.Args)
	assert.True(t, config.TTY)
	assert.True(t, config.Interactive)
	assert.True(t, config.Remove)
	assert.Equal(t, "bridge", config.Network)
	assert.Len(t, config.Volumes, 1)
	assert.Equal(t, "/tmp", config.Volumes[0].HostPath)
	assert.Equal(t, "/data", config.Volumes[0].ContainerPath)
	assert.False(t, config.Volumes[0].ReadOnly)
	assert.Equal(t, []string{"FOO=BAR"}, config.Env)
	assert.Equal(t, "/workspace", config.Workdir)
	assert.Equal(t, "1000", config.User)
}

func TestVolumeMount(t *testing.T) {
	mount := VolumeMount{
		HostPath:      "/etc/hosts",
		ContainerPath: "/etc/hosts",
		ReadOnly:      true,
	}

	assert.Equal(t, "/etc/hosts", mount.HostPath)
	assert.Equal(t, "/etc/hosts", mount.ContainerPath)
	assert.True(t, mount.ReadOnly)
}
