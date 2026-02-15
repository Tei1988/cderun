package command

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

const (
	testImage = "public.ecr.aws/docker/library/alpine:latest"
)

func TestIntegration_Command_Root_BasicExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("cderun alpine echo hello", func(t *testing.T) {
		// Note: since alpine has no entrypoint, the subcommand 'echo' is stripped
		// and the container command becomes 'hello-cderun'.
		// This will fail unless the image has 'echo' as entrypoint.
		// To fix this, we'll use a tool definition in .tools.yaml or just test that it fails/executes correctly.
		// Actually, the requirement is to use alpine as image and echo hello.
		// Let's use a temporary .tools.yaml for this test to be realistic.
		setupTestDir(t)

		err := os.WriteFile(".tools.yaml", []byte("echo:\n  image: "+testImage+"\n  entrypoint: [\"echo\"]"), 0o644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("echo", "hello-cderun")
		skipIfDockerBroken(t, err)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "hello-cderun")
	})

	t.Run("volume mounting", func(t *testing.T) {
		tmpDir := setupTestDir(t)

		err := os.WriteFile(".tools.yaml", []byte("cat:\n  image: "+testImage+"\n  entrypoint: [\"cat\"]"), 0o644)
		require.NoError(t, err)

		hostFile := filepath.Join(tmpDir, "hello.txt")
		err = os.WriteFile(hostFile, []byte("hello-from-host"), 0o644)
		require.NoError(t, err)

		// Use --log-level info to get more info in CI if it fails
		stdout, stderr, exitCode, err := runCderun("--log-level", "info", "--mount", "type=bind,source="+hostFile+",target=/hello.txt", "cat", "/hello.txt")
		skipIfDockerBroken(t, err)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode, "cat failed with exit code %d, stderr: %s", exitCode, stderr)
		assert.Contains(t, stdout, "hello-from-host", "stdout was empty, stderr was: %s\nCheck if host path %s is accessible to Docker.", stderr, hostFile)
	})

	t.Run("environment variables", func(t *testing.T) {
		setupTestDir(t)

		err := os.WriteFile(".tools.yaml", []byte("env:\n  image: "+testImage+"\n  entrypoint: [\"env\"]"), 0o644)
		require.NoError(t, err)

		t.Setenv("HOST_VAR", "host-value")
		stdout, _, exitCode, err := runCderun("-e", "EXPLICIT_VAR=explicit-value", "-e", "HOST_VAR", "env")
		skipIfDockerBroken(t, err)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "EXPLICIT_VAR=explicit-value")
		assert.Contains(t, stdout, "HOST_VAR=host-value")
	})

	t.Run("port mapping", func(t *testing.T) {
		_, _, exitCode, err := runCderun("--image", testImage, "-p", "8081:8000", "--entrypoint", "echo", "echo", "port-test")
		skipIfDockerBroken(t, err)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
	})

	t.Run("cderun expressions", func(t *testing.T) {
		tmpDir := setupTestDir(t)

		err := os.WriteFile(".tools.yaml", []byte("mytool:\n  image: "+testImage+"\n  env:\n    - MY_PWD={{PWD}}"), 0o644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("mytool", "env")
		skipIfDockerBroken(t, err)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "MY_PWD="+tmpDir)
	})

	t.Run("relative path and tilde expansion", func(t *testing.T) {
		tmpDir := setupTestDir(t)

		subDir := filepath.Join(tmpDir, "subdir")
		err := os.MkdirAll(subDir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(".tools.yaml", []byte("mytool:\n  image: "+testImage+"\n  mounts:\n    - type: bind\n      source: ./subdir\n      target: /mnt"), 0o644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("mytool", "ls", "-d", "/mnt")
		skipIfDockerBroken(t, err)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "/mnt")
	})

	t.Run("mount-cderun-path", func(t *testing.T) {
		customPath := "/tmp/custom-cderun"
		stdout, _, exitCode, err := runCderun("--image", testImage, "--mount-socket", "--mount-cderun", "--mount-cderun-path", customPath, "--dry-run", "--dry-run-format", "simple", "echo", "hello")
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "source="+customPath+",target=/usr/local/bin/cderun")
	})
}

func TestIntegration_Command_Root_SymlinkExecution(t *testing.T) {
	// Use a temporary directory for this test
	setupTestDir(t)
	setupTestOptions(t)

	// Create a temporary .tools.yaml for image mapping
	toolsContent := `
node:
  image: node:20-alpine
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	// Prepare mock runtime
	mockRuntime := &runtime.MockRuntime{
		CreatedContainerID: "test-container-id",
		ExitCode:           0,
	}
	setupMockRuntime(t, mockRuntime)

	_, _, _, err = runCderunRawWithOptions(context.Background(), testOptions, []string{"node", "--version"})

	require.NoError(t, err)
	assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
	assert.Equal(t, []string{"--version"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Command_Root_ToolsYAML(t *testing.T) {
	// Use a temporary directory for this test
	setupTestDir(t)
	setupTestOptions(t)

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
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "node", "app.js")
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

func TestIntegration_Command_Root_Priority_EnvOverTools(t *testing.T) {
	t.Setenv("CDERUN_IMAGE", "env-image:latest")

	// Use a temporary directory for this test
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20-alpine
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "node", "app.js")
	require.NoError(t, err)
	// Should use image from environment variable (P3 > P4)
	assert.Equal(t, "env-image:latest", mockRuntime.CreatedConfig.Image)
}

func TestIntegration_Command_Root_BaseCommandFromTools(t *testing.T) {
	// Use a temporary directory for this test
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20-alpine
  command: ["node", "--no-warnings"]
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "node", "app.js")
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
	assert.Equal(t, []string{"node", "--no-warnings", "app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Command_Root_EnvPassThrough(t *testing.T) {
	// Use a temporary directory for this test
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20-alpine
  env:
    - TOOL_KEY=TOOL_VALUE
    - OVERRIDE_KEY=TOOL_VALUE
    - P1_OVERRIDE_KEY=TOOL_VALUE
    - HOST_KEY
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	t.Setenv("HOST_KEY", "HOST_VALUE")
	t.Setenv("CLI_HOST_KEY", "CLI_HOST_VALUE")

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	// Execute with CLI overrides and P1 overrides
	_, _, _, err = runCderunWithOptions(context.Background(), testOptions,
		"--env", "OVERRIDE_KEY=CLI_VALUE",
		"--env", "P1_OVERRIDE_KEY=CLI_VALUE",
		"--env", "CLI_KEY=CLI_VALUE",
		"--env", "CLI_HOST_KEY",
		"node",
		"--cderun-env=P1_OVERRIDE_KEY=P1_VALUE",
		"app.js",
	)
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	envs := mockRuntime.CreatedConfig.Env
	// Now using overwrite logic: P1 overrides everything else
	assert.Len(t, envs, 1)
	assert.Contains(t, envs, "P1_OVERRIDE_KEY=P1_VALUE")
}

func TestIntegration_Command_Root_MountToolsNotFound(t *testing.T) {
	// Setup tools config
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20
python:
  image: python:3
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "--mount-socket", "--mount-tools", "unknown", "--image", "alpine", "sh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool \"unknown\" not found in .tools.yaml")
	assert.Contains(t, err.Error(), "available tools: node, python")
}

func TestIntegration_Command_Root_MountTools_AutoEnable(t *testing.T) {
	// Setup tools config
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	// No --mount-socket, no --mount-cderun
	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "--image", "alpine", "--mount-tools", "node", "sh")
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
	assert.True(t, cderunFound, "cderun should be auto-mounted")
	assert.True(t, nodeFound, "node should be mounted")
	assert.True(t, socketFound, "socket should be auto-mounted")
}

func TestIntegration_Command_Root_MountTools_Logic(t *testing.T) {
	// Setup tools config
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20
python:
  image: python:3
sh:
  image: alpine
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable failed: %v", err)
	}

	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "--mount-tools", "node", "--mount-socket", "--socket-path", "/socket", "sh")
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)

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
	assert.True(t, nodeFound, "node should be mounted")
	assert.False(t, pythonFound, "python should NOT be mounted")

	// Test mount-all-tools
	mockRuntime.CreatedConfig = nil
	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "--mount-all-tools", "--mount-socket", "--socket-path", "/socket", "sh")
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
	assert.True(t, nodeFound, "node should be mounted")
	assert.True(t, pythonFound, "python should be mounted")
}

func TestIntegration_Command_Root_MountAllTools_EmptyConfig(t *testing.T) {
	// Setup empty tools config
	setupTestDir(t)
	setupTestOptions(t)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	stdout, stderr, _, err := runCderunWithOptions(context.Background(), testOptions, "--mount-all-tools", "--mount-socket", "--socket-path", "/socket", "--image", "alpine", "sh")
	require.NoError(t, err)
	// Verify warning appears specifically in stderr
	assert.Contains(t, stderr, "[WARN] --mount-all-tools specified but no tools defined in .tools.yaml")
	_ = stdout // Silence lint if necessary
}

func TestIntegration_Command_Root_ExcludeToolSubcommand(t *testing.T) {
	// Setup tools config
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "node", "app.js")
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Command_Root_IncludeExplicitToolSubcommand(t *testing.T) {
	// Setup tools config
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20
  command: ["node", "--no-warnings"]
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "node", "app.js")
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	assert.Equal(t, []string{"node", "--no-warnings", "app.js"}, mockRuntime.CreatedConfig.Command)
}

func TestIntegration_Command_Flags_ToolsYAML_DockerCompatible(t *testing.T) {
	setupTestDir(t)
	setupTestOptions(t)

	toolsContent := `
node:
  image: node:20
  ports: ["8080:80"]
  privileged: true
  memory: 1g
  cpus: 1.5
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "node", "app.js")
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
	assert.Equal(t, []string{"8080:80"}, mockRuntime.CreatedConfig.Ports)
	assert.True(t, mockRuntime.CreatedConfig.Privileged)
	assert.Equal(t, int64(1024*1024*1024), mockRuntime.CreatedConfig.Memory)
	assert.InDelta(t, 1.5, mockRuntime.CreatedConfig.CPUs, 0.0001)
}

func TestIntegration_Command_Root_InternalOverrides(t *testing.T) {
	// NOTE: This test uses the shared package-level testOptions and is not safe for parallel execution.
	// Use a temporary directory for this test
	setupTestDir(t)

	// Create a temporary .tools.yaml for image mapping
	toolsContent := `
node:
  image: node:20-alpine
`
	err := os.WriteFile(".tools.yaml", []byte(toolsContent), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{}
	setupTestOptions(t)
	setupMockRuntime(t, mockRuntime)

	t.Run("cderun-tty overrides tty even if placed after subcommand", func(t *testing.T) {
		_, _, _, err := runCderunRawWithOptions(context.Background(), testOptions, []string{"cderun", "--tty=true", "node", "--cderun-tty=false", "--version"})
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.False(t, mockRuntime.CreatedConfig.TTY, "TTY should be false because --cderun-tty=false overrides --tty=true")
	})

	t.Run("cderun-tty works in polyglot mode", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil
		_, _, _, err := runCderunRawWithOptions(context.Background(), testOptions, []string{"node", "--cderun-tty=true", "--version"})
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.True(t, mockRuntime.CreatedConfig.TTY, "TTY should be true because --cderun-tty=true was provided")
	})

	t.Run("cderun internal overrides before subcommand result in error", func(t *testing.T) {
		_, _, _, err := runCderunRawWithOptions(context.Background(), testOptions, []string{"cderun", "--cderun-image=alpine:latest", "sh"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})

	t.Run("cderun internal overrides after subcommand work correctly", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil
		_, _, _, err := runCderunRawWithOptions(context.Background(), testOptions, []string{"cderun", "--image=alpine:stable", "sh", "--cderun-image=alpine:latest"})
		require.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "alpine:latest", mockRuntime.CreatedConfig.Image)
	})

	t.Run("cderun internal overrides for network, remove, workdir and mount", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil
		_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "--image=alpine", "--network=bridge", "--remove=false", "--workdir=/initial", "--mount=type=bind,source=/h1,target=/c1", "sh", "--cderun-network=host", "--cderun-remove=true", "--cderun-workdir=/override", "--cderun-mount=type=bind,source=/h2,target=/c2")
		require.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
		assert.True(t, mockRuntime.CreatedConfig.Remove)
		assert.Equal(t, "/override", mockRuntime.CreatedConfig.Workdir)

		assert.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
		assert.Equal(t, "/h2", mockRuntime.CreatedConfig.Mounts[0].Source)
	})

	t.Run("cderun internal overrides for runtime, socket and mounting", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil
		setupTestDir(t)
		err := os.WriteFile(".tools.yaml", []byte("node:\n  image: node:20"), 0o644)
		require.NoError(t, err)

		_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "--image=alpine", "sh", "--cderun-runtime=docker", "--cderun-socket-path=/var/run/custom.sock", "--cderun-mount-socket=true", "--cderun-mount-cderun=true", "--cderun-mount-tools=node")
		require.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)

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
		mockRuntime.CreatedConfig = nil
		_, _, _, err = runCderunWithOptions(context.Background(), testOptions, "--image=alpine", "--remove=true", "sh", "--cderun-remove=false")
		require.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.False(t, mockRuntime.CreatedConfig.Remove)
	})

	t.Run("cderun internal overrides for dry-run", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil
		output, _, _, err := runCderunRawWithOptions(context.Background(), testOptions, []string{"cderun", "--image=alpine", "sh", "echo", "hello", "--cderun-dry-run", "--cderun-dry-run-format=simple"})
		require.NoError(t, err)
		assert.Nil(t, mockRuntime.CreatedConfig)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: echo hello")
	})
}
