package runtime

import (
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_SecurityOpt_Validation(t *testing.T) {
	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}

	t.Run("supported option: no-new-privileges", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges"},
		})
		require.NoError(t, err)

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges:true"},
		})
		require.NoError(t, err)

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges=false"},
		})
		require.NoError(t, err)
	})

	t.Run("supported option: seccomp unconfined", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"seccomp=unconfined"},
		})
		require.NoError(t, err)

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"seccomp:unconfined"},
		})
		require.NoError(t, err)
	})

	t.Run("supported option: apparmor profile", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"apparmor=unconfined"},
		})
		require.NoError(t, err)

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"apparmor:profile-name"},
		})
		require.NoError(t, err)
	})

	t.Run("unsupported security option rejected", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"label=disable"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "containerd runtime: security option \"label=disable\" is not supported yet")
	})

	t.Run("empty apparmor profile rejected", func(t *testing.T) {
		// @jules - verify that empty AppArmor profiles are rejected for both prefixes
		err := rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"apparmor="},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has an empty AppArmor profile")

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"apparmor:"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has an empty AppArmor profile")
	})
}

func TestUnit_Containerd_SecurityOpt_OCISpecModifier(t *testing.T) {
	// @jules - verify that applySecurityOptions production helper works as expected
	spec := &specs.Spec{
		Process: &specs.Process{},
		Linux: &specs.Linux{
			Seccomp: &specs.LinuxSeccomp{},
		},
	}

	// Simulated security options
	secOpts := []string{
		"no-new-privileges=true",
		"seccomp=unconfined",
		"apparmor=custom-apparmor-profile",
	}

	applySecurityOptions(spec, secOpts)

	assert.True(t, spec.Process.NoNewPrivileges)
	assert.Nil(t, spec.Linux.Seccomp)
	assert.Equal(t, "custom-apparmor-profile", spec.Process.ApparmorProfile)

	// @jules - verify that we initialize s.Linux if it is nil when handling seccomp=unconfined
	specNilLinux := &specs.Spec{
		Process: &specs.Process{},
		Linux:   nil,
	}
	applySecurityOptions(specNilLinux, []string{"seccomp=unconfined"})
	assert.NotNil(t, specNilLinux.Linux)
	assert.Nil(t, specNilLinux.Linux.Seccomp)
}
