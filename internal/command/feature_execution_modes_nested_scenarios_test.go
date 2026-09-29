package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
)

func TestUnit_Command_WrapperMode_ArgumentHoistingAndIsolation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		rawArgs  []string
		expected []string
	}{
		{
			name:     "Space-separated P1 override hoisting",
			rawArgs:  []string{"cderun", "node", "--cderun-image", "node:20-alpine", "app.js", "--port", "8080"},
			expected: []string{"cderun", "--cderun-image", "node:20-alpine", "node", "app.js", "--port", "8080"},
		},
		{
			name:     "Equals-separated P1 override hoisting",
			rawArgs:  []string{"cderun", "python3", "--cderun-image=python:3.12-slim", "main.py"},
			expected: []string{"cderun", "--cderun-image=python:3.12-slim", "python3", "main.py"},
		},
		{
			name:     "Hoisting with double dash separator",
			rawArgs:  []string{"cderun", "node", "--cderun-image", "node:18", "--", "--version"},
			expected: []string{"cderun", "--cderun-image", "node:18", "node", "--", "--version"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := defaultOptions()
			cmd := newRootCmd(&o)

			processed, err := preprocessArgs(cmd, tc.rawArgs)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, processed)
		})
	}
}

func TestUnit_Command_SymlinkMode_CleanedExecutables(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	symlinkPath := filepath.Join(tmpDir, "python3")

	// Create dummy binary / symlink target
	require.NoError(t, os.WriteFile(symlinkPath, []byte("#!/bin/sh\n"), 0755))

	o := defaultOptions()
	cmd := newRootCmd(&o)

	// Simulate invocation via symlink path
	rawArgs := []string{symlinkPath, "--cderun-image=python:3.12", "script.py"}
	processed, err := preprocessArgs(cmd, rawArgs)
	require.NoError(t, err)

	// In symlink mode, binary name "python3" is prepended as subcommand, P1 override is hoisted
	expected := []string{"cderun", "--cderun-image=python:3.12", "python3", "script.py"}
	assert.Equal(t, expected, processed)
}

func TestUnit_Command_AdHocMode_ContainerConfigAssembly(t *testing.T) {
	t.Parallel()

	mockFS := &config.MockFileSystem{}
	mockFS.WD = "/workspace"

	o := defaultOptions()
	o.fs = mockFS

	res := &config.ResolvedConfig{
		Image:   "alpine:latest",
		Workdir: "/workspace",
		Env:     []string{"APP_ENV=test", "FOO=BAR"},
	}

	passthroughCmd := []string{"sh", "-c", "echo hello"}

	cfg, err := o.buildContainerConfig(res, passthroughCmd, nil)
	require.NoError(t, err)

	assert.Equal(t, "alpine:latest", cfg.Image)
	assert.Equal(t, "/workspace", cfg.Workdir)
	assert.Equal(t, passthroughCmd, cfg.Command)
	assert.Contains(t, cfg.Env, "APP_ENV=test")
	assert.Contains(t, cfg.Env, "FOO=BAR")
}

func TestUnit_Command_NestedExecution_ControlSocketAndMounts(t *testing.T) {
	t.Parallel()

	mockFS := &config.MockFileSystem{}
	mockFS.WD = "/app"

	o := defaultOptions()
	o.fs = mockFS

	res := &config.ResolvedConfig{
		Image:             "node:20",
		MountCderunSocket: true,
		SocketPath:        "/var/run/cderun-control.sock",
		MountSocketPath:   "/var/run/cderun-control.sock",
		MountSocket:       true,
	}

	cfg, err := o.buildContainerConfig(res, []string{"node", "index.js"}, nil)
	require.NoError(t, err)

	// Verify control socket bind mount
	var socketMountFound bool
	for _, m := range cfg.Mounts {
		if m.Source == "/var/run/cderun-control.sock" {
			socketMountFound = true
			assert.Equal(t, "bind", m.Type)
			break
		}
	}
	assert.True(t, socketMountFound, "Control socket mount should be configured")
}

func TestUnit_Command_NestedExecution_MountCderunPathIntegration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cderun")
	require.NoError(t, os.WriteFile(binPath, []byte("fake-cderun-binary"), 0755))

	mockFS := &config.MockFileSystem{}
	mockFS.WD = "/app"

	o := defaultOptions()
	o.fs = mockFS

	res := &config.ResolvedConfig{
		Image:           "alpine:3.18",
		MountCderunPath: binPath,
		MountCderun:     true,
	}

	cfg, err := o.buildContainerConfig(res, []string{"echo", "nested"}, nil)
	require.NoError(t, err)

	var binMountFound bool
	for _, m := range cfg.Mounts {
		if m.Source == binPath && m.Target == "/usr/local/bin/cderun" {
			binMountFound = true
			assert.Equal(t, "bind", m.Type)
			assert.True(t, m.ReadOnly)
			break
		}
	}
	assert.True(t, binMountFound, "cderun binary bind mount should be configured when MountCderunPath is specified")
}
