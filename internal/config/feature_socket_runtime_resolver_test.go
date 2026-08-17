package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeature_SocketRuntimeResolver_TrailingSlashes(t *testing.T) {
	tests := []struct {
		name            string
		socketPath      string
		expectedRuntime string
	}{
		{
			name:            "podman with trailing slash",
			socketPath:      "/run/user/1000/podman/podman.sock/",
			expectedRuntime: "podman",
		},
		{
			name:            "containerd with multiple trailing slashes",
			socketPath:      "/run/containerd/containerd.sock///",
			expectedRuntime: "containerd",
		},
		{
			name:            "docker with trailing slash",
			socketPath:      "/var/run/docker.sock/",
			expectedRuntime: "docker",
		},
		{
			name:            "podman standard path",
			socketPath:      "/run/podman/podman.sock",
			expectedRuntime: "podman",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := &MockFileSystem{
				Files: map[string][]byte{},
			}
			cli := &CLIOptions{
				Image:      ptr("alpine"),
				SocketPath: ptr(tt.socketPath),
			}
			cfg, err := ResolveWithFS("test", cli, nil, nil, mfs)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedRuntime, cfg.Runtime)
		})
	}
}
