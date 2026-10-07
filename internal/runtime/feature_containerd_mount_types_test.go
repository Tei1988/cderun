//go:build linux

package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_MountTypes_Validation(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}

	tests := []struct {
		name        string
		mounts      []container.Mount
		expectErr   bool
		errContains string
	}{
		{
			name: "type volume is explicitly rejected",
			mounts: []container.Mount{
				{Type: "volume", Source: "data-vol", Target: "/var/lib/data"},
			},
			expectErr:   true,
			errContains: "containerd runtime: volume mount type is not supported",
		},
		{
			name: "type tmpfs is accepted",
			mounts: []container.Mount{
				{Type: "tmpfs", Target: "/tmp/scratch"},
			},
			expectErr: false,
		},
		{
			name: "type bind is accepted",
			mounts: []container.Mount{
				{Type: "bind", Source: "/host/path", Target: "/container/path"},
			},
			expectErr: false,
		},
		{
			name: "empty mount type defaults to bind and is accepted",
			mounts: []container.Mount{
				{Source: "/host/path", Target: "/container/path"},
			},
			expectErr: false,
		},
		{
			name: "unsupported mount type nfs is rejected",
			mounts: []container.Mount{
				{Type: "nfs", Source: "10.0.0.1:/export", Target: "/mnt/nfs"},
			},
			expectErr:   true,
			errContains: "containerd runtime: unsupported mount type \"nfs\"",
		},
		{
			name: "unsupported mount type overlay is rejected",
			mounts: []container.Mount{
				{Type: "overlay", Source: "/lower", Target: "/merged"},
			},
			expectErr:   true,
			errContains: "containerd runtime: unsupported mount type \"overlay\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &container.ContainerConfig{
				Mounts: tt.mounts,
			}
			err := rt.ValidateConfig(cfg)
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Containerd_MountTypes_OCISpecOpts(t *testing.T) {
	t.Parallel()

	cfg := &container.ContainerConfig{
		Mounts: []container.Mount{
			{
				Type:     "tmpfs",
				Target:   "/tmp/cache",
				ReadOnly: false,
			},
			{
				Type:     "tmpfs",
				Target:   "/tmp/ro-cache",
				ReadOnly: true,
			},
			{
				Type:     "bind",
				Source:   "/host/data",
				Target:   "/app/data",
				ReadOnly: false,
			},
			{
				Type:     "bind",
				Source:   "/host/config",
				Target:   "/app/config",
				ReadOnly: true,
			},
			{
				Type:     "",
				Source:   "/host/logs",
				Target:   "/app/logs",
				ReadOnly: false,
			},
		},
	}

	opts := buildContainerdMountSpecOpts(cfg)
	require.Len(t, opts, 5)

	spec := &specs.Spec{}
	for _, opt := range opts {
		err := opt(context.Background(), nil, nil, spec)
		require.NoError(t, err)
	}

	require.Len(t, spec.Mounts, 5)

	// 1. tmpfs rw
	assert.Equal(t, "tmpfs", spec.Mounts[0].Type)
	assert.Equal(t, "tmpfs", spec.Mounts[0].Source)
	assert.Equal(t, "/tmp/cache", spec.Mounts[0].Destination)
	assert.Equal(t, []string{"rw"}, spec.Mounts[0].Options)

	// 2. tmpfs ro
	assert.Equal(t, "tmpfs", spec.Mounts[1].Type)
	assert.Equal(t, "tmpfs", spec.Mounts[1].Source)
	assert.Equal(t, "/tmp/ro-cache", spec.Mounts[1].Destination)
	assert.Equal(t, []string{"ro"}, spec.Mounts[1].Options)

	// 3. bind rw
	assert.Equal(t, "bind", spec.Mounts[2].Type)
	assert.Equal(t, "/host/data", spec.Mounts[2].Source)
	assert.Equal(t, "/app/data", spec.Mounts[2].Destination)
	assert.Equal(t, []string{"rw", "rbind"}, spec.Mounts[2].Options)

	// 4. bind ro
	assert.Equal(t, "bind", spec.Mounts[3].Type)
	assert.Equal(t, "/host/config", spec.Mounts[3].Source)
	assert.Equal(t, "/app/config", spec.Mounts[3].Destination)
	assert.Equal(t, []string{"ro", "rbind"}, spec.Mounts[3].Options)

	// 5. empty type (default bind) rw
	assert.Equal(t, "bind", spec.Mounts[4].Type)
	assert.Equal(t, "/host/logs", spec.Mounts[4].Source)
	assert.Equal(t, "/app/logs", spec.Mounts[4].Destination)
	assert.Equal(t, []string{"rw", "rbind"}, spec.Mounts[4].Options)
}
