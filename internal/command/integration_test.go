package command

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func TestIntegration_Execution_AlpineEcho(t *testing.T) {
	t.Parallel()
	t.Run("mount-cderun-path", func(t *testing.T) {
		t.Parallel()
		customPath := "/tmp/custom-cderun"
		stdout, _, exitCode, err := runCderun("--image", "public.ecr.aws/docker/library/alpine:latest", "--mount-socket", "--mount-cderun", "--mount-cderun-path", customPath, "--dry-run", "--dry-run-format", "simple", "echo", "hello")
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "source="+customPath+",target=/usr/local/bin/cderun")
	})
}

func TestIntegration_Config_ToolsYAML(t *testing.T) {
	t.Parallel()

	toolsContent := `
node:
  image: node:20-alpine
  tty: true
  network: host
  env:
    - KEY=VALUE
  mounts:
    - type: bind
      source: /host
      target: /container
`
	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte(toolsContent),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
	assert.True(t, mockRuntime.CreatedConfig.TTY)
	assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
	assert.Contains(t, mockRuntime.CreatedConfig.Env, "KEY=VALUE")
	assert.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
	assert.Equal(t, "bind", mockRuntime.CreatedConfig.Mounts[0].Type)
	assert.Equal(t, "/host", mockRuntime.CreatedConfig.Mounts[0].Source)
	assert.Equal(t, "/container", mockRuntime.CreatedConfig.Mounts[0].Target)
}

func TestIntegration_Priority_EnvOverTools(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
		},
		Env: map[string]string{
			"CDERUN_IMAGE": "env-image:latest",
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)
	assert.Equal(t, "env-image:latest", mockRuntime.CreatedConfig.Image)
}

func TestIntegration_Config_BaseCommandFromTools(t *testing.T) {
	t.Parallel()

	toolsContent := `
node:
  image: node:20-alpine
`
	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte(toolsContent),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Env_PassThrough(t *testing.T) {
	t.Parallel()

	toolsContent := `
node:
  image: node:20-alpine
  env:
    - TOOL_KEY=TOOL_VALUE
    - OVERRIDE_KEY=TOOL_VALUE
    - P1_OVERRIDE_KEY=TOOL_VALUE
    - HOST_KEY
`
	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte(toolsContent),
		},
		Env: map[string]string{
			"HOST_KEY":     "HOST_VALUE",
			"CLI_HOST_KEY": "CLI_HOST_VALUE",
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun",
		"--env", "OVERRIDE_KEY=CLI_VALUE",
		"--env", "P1_OVERRIDE_KEY=CLI_VALUE",
		"--env", "CLI_KEY=CLI_VALUE",
		"--env", "CLI_HOST_KEY",
		"node",
		"--cderun-env=P1_OVERRIDE_KEY=P1_VALUE",
		"app.js",
	}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	envs := mockRuntime.CreatedConfig.Env

	// Verify that --cderun-env replaces all other environment variables (P1 override behavior).
	assert.Len(t, envs, 1)
	assert.Contains(t, envs, "P1_OVERRIDE_KEY=P1_VALUE")
}

func TestIntegration_MountTools_NotFound(t *testing.T) {
	t.Parallel()

	toolsContent := `
node:
  image: node:20
python:
  image: python:3
`
	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte(toolsContent),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-socket", "--mount-tools", "unknown", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool \"unknown\" not found in .tools.yaml")
	assert.Contains(t, err.Error(), "available tools: node, python")
}

func TestIntegration_MountTools_AutoEnable(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-tools", "node", "sh"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)
	require.NotNil(t, mockRuntime.CreatedConfig)

	cderunFound := false
	socketFound := false
	nodeFound := false
	for _, v := range mockRuntime.CreatedConfig.Mounts {
		if v.Target == "/usr/local/bin/cderun" {
			cderunFound = true
		}
		if v.Target == "/usr/local/bin/node" {
			nodeFound = true
		}
		if strings.Contains(v.Target, "docker.sock") {
			socketFound = true
		}
	}
	assert.True(t, cderunFound)
	assert.True(t, nodeFound)
	assert.True(t, socketFound)
}

func TestIntegration_MountTools_Selection(t *testing.T) {
	t.Parallel()

	toolsContent := `
node:
  image: node:20
python:
  image: python:3
sh:
  image: alpine
`
	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte(toolsContent),
		},
		ExecPath: "/bin/cderun",
	}

	mockRuntime := &runtime.MockRuntime{}
	exePath := "/bin/cderun"

	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-tools", "node", "--mount-socket", "--socket-path", "/socket", "sh"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)

	nodeFound := false
	pythonFound := false
	for _, v := range mockRuntime.CreatedConfig.Mounts {
		if v.Source == exePath && v.Target == "/usr/local/bin/node" {
			nodeFound = true
		}
		if v.Source == exePath && v.Target == "/usr/local/bin/python" {
			pythonFound = true
		}
	}
	assert.True(t, nodeFound)
	assert.False(t, pythonFound)

	// Test mount-all-tools
	mockRuntime.CreatedConfig = nil
	err = ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-all-tools", "--mount-socket", "--socket-path", "/socket", "sh"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)

	nodeFound = false
	pythonFound = false
	for _, v := range mockRuntime.CreatedConfig.Mounts {
		if v.Source == exePath && v.Target == "/usr/local/bin/node" {
			nodeFound = true
		}
		if v.Source == exePath && v.Target == "/usr/local/bin/python" {
			pythonFound = true
		}
	}
	assert.True(t, nodeFound)
	assert.True(t, pythonFound)
}

func TestIntegration_MountTools_AllWithEmptyConfig(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/app",
	}

	mockRuntime := &runtime.MockRuntime{}
	var outBuf, errBuf bytes.Buffer
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-all-tools", "--mount-socket", "--socket-path", "/socket", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)
	}))
	require.NoError(t, err)
	// Warnings should be in stderr (errBuf)
	assert.Contains(t, errBuf.String(), "[WARN] --mount-all-tools specified but no tools defined in .tools.yaml")
}

func TestIntegration_Execution_ExcludeToolSubcommand(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Execution_IncludeExplicitToolSubcommand(t *testing.T) {
	t.Parallel()

	toolsContent := `
node:
  image: node:20
`
	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte(toolsContent),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Flags_DockerCompatible(t *testing.T) {
	t.Parallel()

	toolsContent := `
node:
  image: node:20
  ports: ["8080:80"]
  privileged: true
  memory: 1g
  cpus: 1.5
`
	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte(toolsContent),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}))
	require.NoError(t, err)

	assert.Equal(t, []string{"8080:80"}, mockRuntime.CreatedConfig.Ports)
	assert.True(t, mockRuntime.CreatedConfig.Privileged)
	assert.Equal(t, int64(1024*1024*1024), mockRuntime.CreatedConfig.Memory)
	assert.InDelta(t, 1.5, mockRuntime.CreatedConfig.CPUs, 0.0001)
}

func TestIntegration_Flags_InternalOverrides(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
		},
	}

	t.Run("cderun-tty overrides tty even if placed after subcommand", func(t *testing.T) {
		t.Parallel()
		mr := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tty=true", "node", "--cderun-tty=false", "--version"}, withMockRuntime(mr, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.False(t, mr.CreatedConfig.TTY)
	})

	t.Run("cderun-tty works in polyglot mode", func(t *testing.T) {
		t.Parallel()
		mr := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"node", "--cderun-tty=true", "--version"}, withMockRuntime(mr, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.True(t, mr.CreatedConfig.TTY)
	})

	t.Run("cderun internal overrides before subcommand result in error", func(t *testing.T) {
		t.Parallel()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--cderun-image=alpine:latest", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})

	t.Run("cderun internal overrides after subcommand work correctly", func(t *testing.T) {
		t.Parallel()
		mr := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine:stable", "sh", "--cderun-image=alpine:latest"}, withMockRuntime(mr, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.Equal(t, "alpine:latest", mr.CreatedConfig.Image)
	})

	t.Run("cderun internal overrides for network, remove, workdir and mount", func(t *testing.T) {
		t.Parallel()
		mr := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine", "--network=bridge", "--remove=false", "--workdir=/initial", "--mount=type=bind,source=/h1,target=/c1", "sh", "--cderun-network=host", "--cderun-remove=true", "--cderun-workdir=/override", "--cderun-mount=type=bind,source=/h2,target=/c2"}, withMockRuntime(mr, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.Equal(t, "host", mr.CreatedConfig.Network)
		assert.True(t, mr.CreatedConfig.Remove)
		assert.Equal(t, "/override", mr.CreatedConfig.Workdir)
		assert.Len(t, mr.CreatedConfig.Mounts, 1)
		assert.Equal(t, "/h2", mr.CreatedConfig.Mounts[0].Source)
	})

	t.Run("cderun internal overrides for runtime, socket and mounting", func(t *testing.T) {
		t.Parallel()
		mr := &runtime.MockRuntime{}
		mfs2 := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine", "sh", "--cderun-runtime=docker", "--cderun-socket-path=/var/run/custom.sock", "--cderun-mount-socket=true", "--cderun-mount-cderun=true", "--cderun-mount-tools=node"}, withMockRuntime(mr, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs2
			o.configLoader = config.NewConfigLoaderWithFS(mfs2)
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)

		socketFound := false
		cderunFound := false
		nodeFound := false
		for _, v := range mr.CreatedConfig.Mounts {
			if v.Source == "/var/run/custom.sock" {
				socketFound = true
			}
			if v.Target == "/usr/local/bin/cderun" {
				cderunFound = true
			}
			if v.Target == "/usr/local/bin/node" {
				nodeFound = true
			}
		}
		assert.True(t, socketFound)
		assert.True(t, cderunFound)
		assert.True(t, nodeFound)
	})

	t.Run("cderun internal override can turn off remove", func(t *testing.T) {
		t.Parallel()
		mr := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine", "--remove=true", "sh", "--cderun-remove=false"}, withMockRuntime(mr, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		}))
		require.NoError(t, err)
		assert.False(t, mr.CreatedConfig.Remove)
	})

	t.Run("cderun internal overrides for dry-run", func(t *testing.T) {
		t.Parallel()
		var outBuf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine", "sh", "echo", "hello", "--cderun-dry-run", "--cderun-dry-run-format=simple"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.exitFunc = func(code int) {}
			cmd.SetOut(&outBuf)
		})
		require.NoError(t, err)
		assert.Contains(t, outBuf.String(), "Image: alpine")
		assert.Contains(t, outBuf.String(), "Command: echo hello")
	})
}

func TestIntegration_Flags_ConfigPaths(t *testing.T) {
	t.Parallel()

	t.Run("--config flag overrides hierarchical search", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/.cderun.yaml":   []byte("runtime: podman"),
				"/app/custom/my.yaml": []byte("runtime: docker\ndefaults:\n  network: host"),
			},
			Dirs: map[string]bool{"/app/custom": true},
		}
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--config", "custom/my.yaml", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		}))
		require.NoError(t, err)

		// Check the network mode returned in mockRuntime.CreatedConfig.Network
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
	})

	t.Run("--cderun-config overrides standard config flag", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/config1.yaml": []byte(`defaults:
  network: net1`),
				"/app/config2.yaml": []byte(`defaults:
  network: net2`),
			},
		}
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--config", "config1.yaml", "--image", "alpine", "sh", "--cderun-config", "config2.yaml"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		}))
		require.NoError(t, err)
		assert.Equal(t, "net2", mockRuntime.CreatedConfig.Network)
	})

	t.Run("CDERUN_CONFIG env var works", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/env-config.yaml": []byte(`defaults:
  network: env-net`),
			},
			Env: map[string]string{
				"CDERUN_CONFIG": "env-config.yaml",
			},
		}
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		}))
		require.NoError(t, err)
		assert.Equal(t, "env-net", mockRuntime.CreatedConfig.Network)
	})

	t.Run("Missing config file results in error", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
		}
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--config", "non-existent.yaml", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load cderun config")
	})

	t.Run("--tool-config flag works", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/custom-tools.yaml": []byte(`node:
  image: node:custom`),
			},
		}
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tool-config", "custom-tools.yaml", "node", "--version"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		}))
		require.NoError(t, err)
		assert.Equal(t, "node:custom", mockRuntime.CreatedConfig.Image)
	})

	t.Run("--cderun-tool-config flag works", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/override-tools.yaml": []byte(`node:
  image: node:override`),
			},
		}
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "--version", "--cderun-tool-config", "override-tools.yaml"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		}))
		require.NoError(t, err)
		assert.Equal(t, "node:override", mockRuntime.CreatedConfig.Image)
	})

	t.Run("CDERUN_TOOL_CONFIG env var works", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/env-tools.yaml": []byte(`node:
  image: node:env`),
			},
			Env: map[string]string{
				"CDERUN_TOOL_CONFIG": "env-tools.yaml",
			},
		}
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "--version"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		}))
		require.NoError(t, err)
		assert.Equal(t, "node:env", mockRuntime.CreatedConfig.Image)
	})
}
