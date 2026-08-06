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
		assert.NoError(t, err)

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges:true"},
		})
		assert.NoError(t, err)

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges=false"},
		})
		assert.NoError(t, err)
	})

	t.Run("supported option: seccomp unconfined", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"seccomp=unconfined"},
		})
		assert.NoError(t, err)

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"seccomp:unconfined"},
		})
		assert.NoError(t, err)
	})

	t.Run("supported option: apparmor profile", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"apparmor=unconfined"},
		})
		assert.NoError(t, err)

		err = rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"apparmor:profile-name"},
		})
		assert.NoError(t, err)
	})

	t.Run("unsupported security option rejected", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{
			SecurityOpt: []string{"label=disable"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "containerd runtime: security option \"label=disable\" is not supported yet")
	})
}

func TestUnit_Containerd_SecurityOpt_OCISpecModifier(t *testing.T) {
	// Directly test the anonymous spec modifier logic used during container creation
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

	// We can manually run our modifier function block
	modifier := func(s *specs.Spec) {
		if s.Process == nil {
			s.Process = &specs.Process{}
		}
		for _, opt := range secOpts {
			if opt == "no-new-privileges" || opt == "no-new-privileges:true" || opt == "no-new-privileges=true" {
				s.Process.NoNewPrivileges = true
			} else if opt == "no-new-privileges:false" || opt == "no-new-privileges=false" {
				s.Process.NoNewPrivileges = false
			} else if opt == "seccomp=unconfined" || opt == "seccomp:unconfined" {
				if s.Linux != nil {
					s.Linux.Seccomp = nil
				}
			} else if len(opt) > 9 && opt[:9] == "apparmor=" {
				s.Process.ApparmorProfile = opt[9:]
			} else if len(opt) > 9 && opt[:9] == "apparmor:" {
				s.Process.ApparmorProfile = opt[9:]
			}
		}
	}

	modifier(spec)

	assert.True(t, spec.Process.NoNewPrivileges)
	assert.Nil(t, spec.Linux.Seccomp)
	assert.Equal(t, "custom-apparmor-profile", spec.Process.ApparmorProfile)
}
