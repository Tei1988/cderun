//go:build linux

package runtime

import (
	"context"
	"testing"

	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_UpdateShmSize(t *testing.T) {
	s := &specs.Spec{
		Mounts: []specs.Mount{
			{
				Destination: "/dev/shm",
				Type:        "tmpfs",
				Source:      "shm",
				Options:     []string{"rw", "size=64m"},
			},
			{
				Destination: "/etc/resolv.conf",
				Type:        "bind",
			},
		},
	}

	opts := UpdateShmSize(1024 * 1024 * 256) // 256MB
	var fn oci.SpecOpts = opts
	err := fn(context.Background(), nil, nil, s)
	require.NoError(t, err)

	assert.Len(t, s.Mounts, 2)
	found := false
	for _, m := range s.Mounts {
		if m.Destination == "/dev/shm" {
			found = true
			assert.Equal(t, "tmpfs", m.Type)
			assert.Contains(t, m.Options, "size=268435456")
		}
	}
	assert.True(t, found)
}
