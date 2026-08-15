//go:build linux

package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
)

// TestUnit_Runtime_Containerd_ValidateConfig_EdgeCases tests ContainerdRuntime.ValidateConfig on Linux.
// References: docs/features/command-line-options.md
func TestUnit_Runtime_Containerd_ValidateConfig_EdgeCases(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{}

	testCases := []struct {
		name    string
		config  *container.ContainerConfig
		wantErr bool
	}{
		{
			name:    "valid empty config",
			config:  &container.ContainerConfig{},
			wantErr: false,
		},
		{
			name: "unsupported init",
			config: &container.ContainerConfig{
				Init: true,
			},
			wantErr: true,
		},
		{
			name: "unsupported gpus",
			config: &container.ContainerConfig{
				GPUs: "all",
			},
			wantErr: true,
		},
		{
			name: "unsupported restart policy",
			config: &container.ContainerConfig{
				Restart: "always",
			},
			wantErr: true,
		},
		{
			name: "invalid ipc mode",
			config: &container.ContainerConfig{
				IPC: "container:other",
			},
			wantErr: true,
		},
		{
			name: "invalid cgroupns mode",
			config: &container.ContainerConfig{
				Cgroupns: "invalid_cgroupns",
			},
			wantErr: true,
		},
		{
			name: "invalid negative shm size",
			config: &container.ContainerConfig{
				ShmSize: "-1g",
			},
			wantErr: true,
		},
		{
			name: "unsupported security opt",
			config: &container.ContainerConfig{
				SecurityOpt: []string{"invalid-security-opt"},
			},
			wantErr: true,
		},
		{
			name: "unsupported network mode",
			config: &container.ContainerConfig{
				Network: "bridge",
			},
			wantErr: true,
		},
		{
			name: "unsupported volume mount",
			config: &container.ContainerConfig{
				Mounts: []container.Mount{
					{Type: "volume", Source: "myvol", Target: "/vol"},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := rt.ValidateConfig(tc.config)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
