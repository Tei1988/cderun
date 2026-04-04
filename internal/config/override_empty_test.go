package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Resolver_EmptyListOverride(t *testing.T) {
	// P5 Global Defaults
	global := &CDERunConfig{
		Defaults: ConfigDefaults{
			Env: []string{"GLOBAL=1"},
			Mounts: []MountConfig{
				{
					Type:   "bind",
					Source: ConfigPath{Raw: "/global"},
					Target: ConfigPath{Raw: "/global"},
				},
			},
			Devices: []DeviceConfig{
				{
					Source:      ConfigPath{Raw: "/dev/global"},
					Destination: ConfigPath{Raw: "/dev/global"},
					Permissions: "rwm",
				},
			},
		},
	}

	// P4 Tool Config with explicit empty lists
	tools := ToolsConfig{
		"empty-tool": ToolConfig{
			Image:   "alpine",
			Env:     []string{},      // Explicit empty list
			Mounts:  []MountConfig{}, // Explicit empty list
			Devices: []DeviceConfig{}, // Explicit empty list
		},
	}

	cli := CLIOptions{}

	res, err := Resolve("empty-tool", &cli, tools, global)
	require.NoError(t, err)

	// These assertions ensure that an explicit empty list in tool config (P4)
	// correctly overrides the global defaults (P5).
	assert.Empty(t, res.Env, "Env should be empty (overridden by P4)")
	assert.Empty(t, res.Mounts, "Mounts should be empty (overridden by P4)")
	assert.Empty(t, res.Devices, "Devices should be empty (overridden by P4)")
}

func TestUnit_Resolver_EmptyEnvOverride(t *testing.T) {
	// P5 Global Defaults
	global := &CDERunConfig{
		Defaults: ConfigDefaults{
			Env: []string{"GLOBAL=1"},
			Mounts: []MountConfig{
				{
					Type:   "bind",
					Source: ConfigPath{Raw: "/global"},
					Target: ConfigPath{Raw: "/global"},
				},
			},
			Devices: []DeviceConfig{
				{
					Source:      ConfigPath{Raw: "/dev/global"},
					Destination: ConfigPath{Raw: "/dev/global"},
					Permissions: "rwm",
				},
			},
		},
	}

	tools := ToolsConfig{
		"test-tool": ToolConfig{
			Image: "alpine",
		},
	}

	cli := CLIOptions{}

	// Case 1: CDERUN_ENV is empty string
	fs := &MockFileSystem{
		Env: map[string]string{
			"CDERUN_ENV":    "",
			"CDERUN_MOUNT":  "",
			"CDERUN_DEVICE": "",
		},
	}

	res, err := ResolveWithFS("test-tool", &cli, tools, global, fs)
	require.NoError(t, err)

	assert.Empty(t, res.Env, "Env should be empty (overridden by P3 CDERUN_ENV)")
	assert.Empty(t, res.Mounts, "Mounts should be empty (overridden by P3 CDERUN_MOUNT)")
	assert.Empty(t, res.Devices, "Devices should be empty (overridden by P3 CDERUN_DEVICE)")
}
