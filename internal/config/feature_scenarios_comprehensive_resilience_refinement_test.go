package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_Boundary_ParameterValidators tests safety validators on boundary/invalid inputs.
// Reference: docs/features/command-line-options.md & docs/features/value-resolution.md
func TestUnit_Config_Boundary_ParameterValidators(t *testing.T) {
	t.Parallel()

	t.Run("ValidateToolName Null Byte and Malformed Checks", func(t *testing.T) {
		assert.Error(t, ValidateToolName("node\x00bin"))
		assert.Error(t, ValidateToolName("node;sh"))
		assert.Error(t, ValidateToolName("node/bin"))
		assert.NoError(t, ValidateToolName("node-18"))
	})

	t.Run("ValidateHostname Strict Rules", func(t *testing.T) {
		assert.Error(t, ValidateHostname("host_name"))
		assert.Error(t, ValidateHostname("-host"))
		assert.Error(t, ValidateHostname("host-"))
		assert.Error(t, ValidateHostname("host..name"))
		assert.NoError(t, ValidateHostname("node-1.cluster.local"))
	})

	t.Run("ValidateDNSOption Malformed Rules", func(t *testing.T) {
		assert.Error(t, ValidateDNSOption("ndots:5;rm -rf"))
		assert.Error(t, ValidateDNSOption("option with space"))
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.NoError(t, ValidateDNSOption("timeout:2"))
	})

	t.Run("ValidateSecurityOpt Safety Rules", func(t *testing.T) {
		assert.Error(t, ValidateSecurityOpt("no-new-privileges:\x00"))
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
	})

	t.Run("ValidatePort Bounds and Ranges", func(t *testing.T) {
		assert.Error(t, ValidatePort("70000:80"))
		assert.Error(t, ValidatePort("8080:0"))
		assert.Error(t, ValidatePort("8000-8002:8000-8005"))
		assert.NoError(t, ValidatePort("8080:80"))
		assert.NoError(t, ValidatePort("8000-8005:8000-8005"))
	})

	t.Run("ValidateWorkdir Symbols Support", func(t *testing.T) {
		assert.NoError(t, ValidateWorkdir("/var/node_modules/.pnpm/esbuild@0.25.12"))
		assert.NoError(t, ValidateWorkdir("/@scope/pkg+dir"))
		assert.Error(t, ValidateWorkdir("relative/path"))
		assert.Error(t, ValidateWorkdir("/app/with\nnewline"))
	})
}

// TestUnit_Config_Boundary_PrecedenceMatrix tests precedence resolution with distinct P1-P6 inputs.
// Reference: docs/features/argument-priority-logic.md
func TestUnit_Config_Boundary_PrecedenceMatrix(t *testing.T) {
	t.Parallel()

	mockFS := &MockFileSystem{
		Files: map[string][]byte{},
		Dirs: map[string]bool{
			"/app": true,
		},
		WD:      "/app",
		HomeDir: "/home/testuser",
	}

	opts := &CLIOptions{
		Image:       optStr("node:22-alpine"),
		Workdir:     optStr("/cli-workdir"),
		HangTimeout: optStr("45s"),
	}

	toolsCfg := ToolsConfig{
		"node": ToolConfig{
			Image:       "node:20-alpine",
			Workdir:     "/tool-workdir",
			HangTimeout: "30s",
		},
	}

	globalCfg := &CDERunConfig{
		Defaults: ConfigDefaults{
			Workdir:     "/global-workdir",
			HangTimeout: "15s",
		},
	}

	res, err := ResolveWithFS("node", opts, toolsCfg, globalCfg, mockFS)
	require.NoError(t, err)
	assert.Equal(t, "node:22-alpine", res.Image)
	assert.Equal(t, "/cli-workdir", res.Workdir)
	assert.Equal(t, 45*time.Second, res.HangTimeout)
}

func optStr(s string) *string {
	return &s
}
