package command

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func TestIntegration_Root_Execution_DryRun(t *testing.T) {
	t.Parallel()
	t.Run("MountCderunPath_ProvidedPath_CorrectSourceInConfig", func(t *testing.T) {
		t.Parallel()
		customPath := "/tmp/custom-cderun"
		stdout, _, exitCode, err := runCderun("--image", "public.ecr.aws/docker/library/alpine:latest", "--mount-socket", "--mount-cderun", "--mount-cderun-path", customPath, "--dry-run", "--dry-run-format", "simple", "echo", "hello")
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "source="+customPath+",target=/usr/local/bin/cderun")
	})
}

func TestIntegration_Root_Config_ToolsYAML(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte(`
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
`),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
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

func TestIntegration_Root_Config_Priority(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
		},
		Env: map[string]string{
			"CDERUN_IMAGE": "env-image:latest",
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)
	assert.Equal(t, "env-image:latest", mockRuntime.CreatedConfig.Image)
}

func TestIntegration_Root_Polyglot_Symlink(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
		},
	}

	mockRuntime := &runtime.MockRuntime{
		CreatedContainerID: "test-container-id",
		ExitCode:           0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := ExecuteContextWithOptions(ctx, []string{"node", "--version"}, withMockRuntime(mockRuntime, withMockFS(mfs)))

	require.NoError(t, err)
	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "node:20-alpine", cfg.Image)
	assert.Equal(t, []string{"--version"}, cfg.Command)
}

func TestIntegration_Root_Config_BaseCommand(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Root_Env_PassThrough(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte(`
node:
  image: node:20-alpine
  env:
    - TOOL_KEY=TOOL_VALUE
    - OVERRIDE_KEY=TOOL_VALUE
    - P1_OVERRIDE_KEY=TOOL_VALUE
    - HOST_KEY
`),
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
	}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	envs := mockRuntime.CreatedConfig.Env

	// Verify that --cderun-env replaces all other environment variables (P1 override behavior).
	assert.Len(t, envs, 1)
	assert.Contains(t, envs, "P1_OVERRIDE_KEY=P1_VALUE")
}

func TestIntegration_Root_MountTools_Selection(t *testing.T) {
	t.Parallel()

	t.Run("ToolNotFound_UnknownToolName_Error", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte(`
node:
  image: node:20
python:
  image: python:3
`),
			},
		}

		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-socket", "--mount-tools", "unknown", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tool \"unknown\" not found in .tools.yaml")
		assert.Contains(t, err.Error(), "available tools: node, python")
	})

	t.Run("AutoEnable_MountToolsRequested_SocketAndCderunMounted", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}

		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-tools", "node", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
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
	})

	t.Run("SpecificTools_Selection_CorrectMounts", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte(`
node:
  image: node:20
python:
  image: python:3
sh:
  image: alpine
`),
			},
			ExecPath: "/usr/local/bin/cderun",
		}

		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-tools", "node", "--mount-socket", "--socket-path", "/socket", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.NoError(t, err)

		nodeFound := false
		pythonFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Source == "/usr/local/bin/cderun" && v.Target == "/usr/local/bin/node" {
				nodeFound = true
			}
			if v.Source == "/usr/local/bin/cderun" && v.Target == "/usr/local/bin/python" {
				pythonFound = true
			}
		}
		assert.True(t, nodeFound)
		assert.False(t, pythonFound)

		// Test mount-all-tools
		mockRuntime.CreatedConfig = nil
		err = ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-all-tools", "--mount-socket", "--socket-path", "/socket", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.NoError(t, err)

		nodeFound = false
		pythonFound = false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Source == "/usr/local/bin/cderun" && v.Target == "/usr/local/bin/node" {
				nodeFound = true
			}
			if v.Source == "/usr/local/bin/cderun" && v.Target == "/usr/local/bin/python" {
				pythonFound = true
			}
		}
		assert.True(t, nodeFound)
		assert.True(t, pythonFound)
	})
}

func TestIntegration_Root_MountTools_AllWithEmptyConfig(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
	}

	mockRuntime := &runtime.MockRuntime{}
	var outBuf, errBuf bytes.Buffer
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-all-tools", "--mount-socket", "--socket-path", "/socket", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)
	}))
	require.NoError(t, err)
	assert.Contains(t, errBuf.String(), "[WARN] --mount-all-tools specified but no tools defined in .tools.yaml")
}

func TestIntegration_Root_Execution_SubcommandExclusion(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Root_Execution_IncludeExplicitToolSubcommand(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Root_Flags_DockerCompatible(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte(`
node:
  image: node:20
  ports: ["8080:80"]
  privileged: true
  memory: 1g
  cpus: 1.5
`),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)

	assert.Equal(t, []string{"8080:80"}, mockRuntime.CreatedConfig.Ports)
	assert.True(t, mockRuntime.CreatedConfig.Privileged)
	assert.Equal(t, int64(1024*1024*1024), mockRuntime.CreatedConfig.Memory)
	assert.InDelta(t, 1.5, mockRuntime.CreatedConfig.CPUs, 0.0001)
}

func TestIntegration_Root_Flags_InternalOverrides(t *testing.T) {
	t.Parallel()

	t.Run("CderunTTY_OverrideAfterSubcommand_Priority", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tty=true", "node", "--cderun-tty=false", "--version"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.False(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("cderun-tty works in polyglot mode", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"node", "--cderun-tty=true", "--version"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("cderun internal overrides before subcommand result in error", func(t *testing.T) {
		t.Parallel()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--cderun-image=alpine:latest", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})

	t.Run("cderun internal overrides after subcommand work correctly", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine:stable", "sh", "--cderun-image=alpine:latest"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.Equal(t, "alpine:latest", mockRuntime.CreatedConfig.Image)
	})

	t.Run("cderun internal overrides for network, remove, workdir and mount", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine", "--network=bridge", "--remove=false", "--workdir=/initial", "--mount=type=bind,source=/h1,target=/c1", "sh", "--cderun-network=host", "--cderun-remove=true", "--cderun-workdir=/override", "--cderun-mount=type=bind,source=/h2,target=/c2"}, withMockRuntime(mockRuntime, func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
		assert.True(t, mockRuntime.CreatedConfig.Remove)
		assert.Equal(t, "/override", mockRuntime.CreatedConfig.Workdir)
		assert.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
		assert.Equal(t, "/h2", mockRuntime.CreatedConfig.Mounts[0].Source)
	})

	t.Run("cderun internal overrides for runtime, socket and mounting", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine", "sh", "--cderun-runtime=docker", "--cderun-socket-path=/var/run/custom.sock", "--cderun-mount-socket=true", "--cderun-mount-cderun=true", "--cderun-mount-tools=node"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)

		socketFound := false
		cderunFound := false
		nodeFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
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
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine", "--remove=true", "sh", "--cderun-remove=false"}, withMockRuntime(mockRuntime))
		require.NoError(t, err)
		assert.False(t, mockRuntime.CreatedConfig.Remove)
	})

	t.Run("cderun internal overrides for dry-run", func(t *testing.T) {
		t.Parallel()
		var outBuf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image=alpine", "sh", "echo", "hello", "--cderun-dry-run", "--cderun-dry-run-format=simple"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			cmd.SetOut(&outBuf)
		})
		require.NoError(t, err)
		assert.Contains(t, outBuf.String(), "Image: alpine")
		assert.Contains(t, outBuf.String(), "Command: echo hello")
	})
}

func TestIntegration_Root_Flags_ConfigPaths(t *testing.T) {
	t.Parallel()

	t.Run("ConfigFlag_OverrideSearch_FileLoaded", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.cderun.yaml": []byte("runtime: podman"),
				"/project/custom/my.yaml": []byte(`runtime: docker
defaults:
  network: host`),
			},
			Dirs: map[string]bool{
				"/project/custom": true,
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--config", "custom/my.yaml", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.NoError(t, err)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
	})

	t.Run("--cderun-config overrides standard config flag", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/config1.yaml": []byte(`defaults:
  network: net1`),
				"/project/config2.yaml": []byte(`defaults:
  network: net2`),
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--config", "config1.yaml", "--image", "alpine", "sh", "--cderun-config", "config2.yaml"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.NoError(t, err)
		assert.Equal(t, "net2", mockRuntime.CreatedConfig.Network)
	})

	t.Run("CDERUN_CONFIG env var works", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/env-config.yaml": []byte(`defaults:
  network: env-net`),
			},
			Env: map[string]string{
				"CDERUN_CONFIG": "env-config.yaml",
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.NoError(t, err)
		assert.Equal(t, "env-net", mockRuntime.CreatedConfig.Network)
	})

	t.Run("Missing config file results in error", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--config", "non-existent.yaml", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load cderun config")
	})

	t.Run("--tool-config flag works", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/custom-tools.yaml": []byte(`node:
  image: node:custom`),
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tool-config", "custom-tools.yaml", "node", "--version"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.NoError(t, err)
		assert.Equal(t, "node:custom", mockRuntime.CreatedConfig.Image)
	})

	t.Run("--cderun-tool-config flag works", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/override-tools.yaml": []byte(`node:
  image: node:override`),
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "--version", "--cderun-tool-config", "override-tools.yaml"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.NoError(t, err)
		assert.Equal(t, "node:override", mockRuntime.CreatedConfig.Image)
	})

	t.Run("CDERUN_TOOL_CONFIG env var works", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/env-tools.yaml": []byte(`node:
  image: node:env`),
			},
			Env: map[string]string{
				"CDERUN_TOOL_CONFIG": "env-tools.yaml",
			},
		}
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "--version"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.NoError(t, err)
		assert.Equal(t, "node:env", mockRuntime.CreatedConfig.Image)
	})
}
