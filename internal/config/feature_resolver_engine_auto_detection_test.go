package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Resolver_EngineAndSocketInvariants(t *testing.T) {
	t.Run("DeduceEngineFromSocketPath", func(t *testing.T) {
		assert.Equal(t, "podman", deduceEngineFromSocketPath("/run/podman/podman.sock"))
		assert.Equal(t, "containerd", deduceEngineFromSocketPath("/run/containerd/containerd.sock"))
		assert.Equal(t, "docker", deduceEngineFromSocketPath("/var/run/docker.sock"))
		assert.Equal(t, "docker", deduceEngineFromSocketPath("/custom/socket.sock"))
	})

	t.Run("DefaultSocketPathForEngine", func(t *testing.T) {
		assert.Equal(t, "/run/podman/podman.sock", defaultSocketPathForEngine("podman"))
		assert.Equal(t, "/run/containerd/containerd.sock", defaultSocketPathForEngine("containerd"))
		assert.Equal(t, "/var/run/docker.sock", defaultSocketPathForEngine("docker"))
		assert.Equal(t, "/var/run/docker.sock", defaultSocketPathForEngine("nerdctl"))
	})

	t.Run("SocketPathTrailingSlashesResolution", func(t *testing.T) {
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
	})

	t.Run("AutoDetectEngineFromMockFS", func(t *testing.T) {
		mockFS := &MockFileSystem{
			WD:    "/test",
			Files: map[string][]byte{"/run/containerd/containerd.sock": []byte("")},
		}

		img := "golang:alpine"
		cliOpts := &CLIOptions{Image: &img}
		res, err := ResolveWithFS("go", cliOpts, nil, nil, mockFS)
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Equal(t, "/run/containerd/containerd.sock", res.SocketPath)
	})

	t.Run("ValidateToolImageMismatch", func(t *testing.T) {
		mockFS := &MockFileSystem{WD: "/test"}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:18",
			},
		}

		pythonImg := "python:3.10"
		// Different registry / image name should raise error
		cliOpts := &CLIOptions{
			Image: &pythonImg,
		}
		_, err := ResolveWithFS("node", cliOpts, tools, nil, mockFS)
		require.Error(t, err)

		nodeImg := "node:18"
		// Matching image should succeed
		cliOptsOk := &CLIOptions{
			Image: &nodeImg,
		}
		res, err := ResolveWithFS("node", cliOptsOk, tools, nil, mockFS)
		require.NoError(t, err)
		assert.Equal(t, "node:18", res.Image)
	})
}
