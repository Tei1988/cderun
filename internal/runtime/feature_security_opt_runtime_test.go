package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_Docker_SecurityOpt_Mapping(t *testing.T) {
	cfg := &container.ContainerConfig{
		Image:       "alpine",
		SecurityOpt: []string{"no-new-privileges", "seccomp=unconfined"},
	}

	_, hostCfg, _, err := toDockerContainerConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"no-new-privileges", "seccomp=unconfined"}, hostCfg.SecurityOpt)
}

func TestUnit_Runtime_Containerd_SecurityOpt_Validation(t *testing.T) {
	rt := &ContainerdRuntime{}

	t.Run("invalid empty AppArmor profile", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:       "alpine",
			SecurityOpt: []string{"apparmor="},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty AppArmor profile is not supported")
	})

	t.Run("unsupported security option", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:       "alpine",
			SecurityOpt: []string{"invalid-opt"},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported security option")
	})
}

func TestUnit_Runtime_Containerd_ApplySecurityOptions(t *testing.T) {
	spec := &specs.Spec{
		Process: &specs.Process{
			NoNewPrivileges: false,
		},
		Linux: &specs.Linux{
			Seccomp: &specs.LinuxSeccomp{},
		},
	}

	opts := []string{"no-new-privileges", "seccomp=unconfined", "apparmor=my-profile"}
	err := applySecurityOptions(spec, opts)
	require.NoError(t, err)

	assert.True(t, spec.Process.NoNewPrivileges)
	assert.Nil(t, spec.Linux.Seccomp)
	assert.Equal(t, "my-profile", spec.Process.ApparmorProfile)
}
