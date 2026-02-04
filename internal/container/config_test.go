package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainerConfigInitialization(t *testing.T) {
	config := ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"sh", "-c", "echo hello"},
		TTY:         true,
		Interactive: true,
		Remove:      true,
		Network:     "bridge",
		Mounts: []Mount{
			{
				Type:     "bind",
				Source:   "/tmp",
				Target:   "/data",
				ReadOnly: false,
			},
		},
		Env:     []string{"FOO=BAR"},
		Workdir: "/workspace",
		User:    "1000",
	}

	assert.Equal(t, "alpine:latest", config.Image)
	assert.Equal(t, []string{"sh", "-c", "echo hello"}, config.Command)
	assert.True(t, config.TTY)
	assert.True(t, config.Interactive)
	assert.True(t, config.Remove)
	assert.Equal(t, "bridge", config.Network)
	assert.Len(t, config.Mounts, 1)
	assert.Equal(t, "bind", config.Mounts[0].Type)
	assert.Equal(t, "/tmp", config.Mounts[0].Source)
	assert.Equal(t, "/data", config.Mounts[0].Target)
	assert.False(t, config.Mounts[0].ReadOnly)
	assert.Equal(t, []string{"FOO=BAR"}, config.Env)
	assert.Equal(t, "/workspace", config.Workdir)
	assert.Equal(t, "1000", config.User)
}

func TestMount(t *testing.T) {
	mount := Mount{
		Type:     "bind",
		Source:   "/etc/hosts",
		Target:   "/etc/hosts",
		ReadOnly: true,
	}

	assert.Equal(t, "bind", mount.Type)
	assert.Equal(t, "/etc/hosts", mount.Source)
	assert.Equal(t, "/etc/hosts", mount.Target)
	assert.True(t, mount.ReadOnly)
}
