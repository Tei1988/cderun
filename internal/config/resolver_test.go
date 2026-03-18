package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mcs(tb testing.TB, ss ...string) []MountConfig {
	tb.Helper()
	var res []MountConfig
	for _, s := range ss {
		m, err := ParseMountFlag(s)
		if err != nil {
			tb.Fatalf("failed to parse mount flag %q: %v", s, err)
		}
		res = append(res, m)
	}
	return res
}

func dcs(tb testing.TB, ss ...string) []DeviceConfig {
	tb.Helper()
	var res []DeviceConfig
	for _, s := range ss {
		d, ok := ParseDeviceConfig(s)
		if !ok {
			tb.Fatalf("failed to parse device config %q", s)
		}
		res = append(res, d)
	}
	return res
}

func ptr[T any](v T) *T {
	return &v
}

func cp(s string) ConfigPath {
	return ConfigPath{Raw: s}
}

func TestUnit_Resolver_Priority_P2CLITakesPriorityOverP4ToolAndP5Global(t *testing.T) {
	t.Parallel()
	// Given: CLI sets TTY to true, Tool and Global set to false
	cli := CLIOptions{
		TTY:    true,
		TTYSet: true,
	}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:20",
			TTY:   ptr(false),
		},
	}
	global := &CDERunConfig{
		Defaults: ConfigDefaults{
			TTY: ptr(false),
		},
	}

	// When: Resolving configuration for "node"
	res, err := Resolve("node", cli, tools, global)

	// Then: TTY should be true (CLI wins)
	require.NoError(t, err)
	assert.True(t, res.TTY)
	assert.Equal(t, "node:20", res.Image)
}

func TestUnit_Resolver_Priority_P1OverrideTakesPriorityOverP2CLI(t *testing.T) {
	t.Parallel()
	// Given: P1 Override sets TTY to false, P2 CLI sets to true
	cli := CLIOptions{
		TTY:          true,
		TTYSet:       true,
		CderunTTY:    false,
		CderunTTYSet: true,
	}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:20",
		},
	}

	// When: Resolving configuration for "node"
	res, err := Resolve("node", cli, tools, nil)

	// Then: TTY should be false (P1 wins)
	require.NoError(t, err)
	assert.False(t, res.TTY)
}

func TestUnit_Resolver_Priority_P3EnvVarPriority(t *testing.T) {
	t.Parallel()
	// Given: Environment variables set TTY and Image
	mfs := &MockFileSystem{
		Env: map[string]string{
			"CDERUN_TTY":   "true",
			"CDERUN_IMAGE": "env-image:latest",
		},
	}
	cli := CLIOptions{}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:20",
			TTY:   ptr(false),
		},
	}

	// When: Resolving configuration with MockFileSystem
	res, err := ResolveWithFS("node", cli, tools, nil, mfs)

	// Then: TTY should be true and Image should be from environment variable
	require.NoError(t, err)
	assert.True(t, res.TTY)
	assert.Equal(t, "env-image:latest", res.Image)
}

func TestUnit_Resolver_Priority_WhenToolValueMatchesFallback(t *testing.T) {
	t.Parallel()
	// Given: Global sets network to host, Tool sets to bridge (fallback value)
	global := &CDERunConfig{
		Defaults: ConfigDefaults{Network: "host"},
	}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image:   "node",
			Network: "bridge",
		},
	}

	// When: Resolving configuration for "node"
	res, err := Resolve("node", CLIOptions{}, tools, global)

	// Then: Network should be bridge (Tool wins)
	require.NoError(t, err)
	assert.Equal(t, "bridge", res.Network)
}

func TestUnit_Resolver_Image_ResolutionFromToolConfig(t *testing.T) {
	t.Parallel()
	// Given: ToolConfig defines image for python
	cli := CLIOptions{}
	tools := ToolsConfig{
		"python": ToolConfig{
			Image: "python:3.11",
		},
	}

	// When: Resolving configuration for "python"
	res, err := Resolve("python", cli, tools, nil)

	// Then: Image should be python:3.11
	require.NoError(t, err)
	assert.Equal(t, "python:3.11", res.Image)
}

func TestUnit_Resolver_Image_ErrorIfNoImageFound(t *testing.T) {
	t.Parallel()
	// Given: No image mapping for "unknown"
	cli := CLIOptions{}

	// When: Resolving configuration for "unknown"
	_, err := Resolve("unknown", cli, nil, nil)

	// Then: Should return an error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no image mapping found")
}

func TestUnit_Resolver_Image_AllowEmptySubcommand(t *testing.T) {
	t.Parallel()
	// Given: Empty subcommand
	cli := CLIOptions{}

	// When: Resolving configuration
	res, err := Resolve("", cli, nil, nil)

	// Then: Should not error, and image should be empty
	require.NoError(t, err)
	assert.Empty(t, res.Image)
}

func TestUnit_Resolver_Mounts_Resolution(t *testing.T) {
	t.Parallel()
	// Given: ToolConfig defines mounts
	cli := CLIOptions{}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image:  "node:20",
			Mounts: mcs(t, "type=bind,source=/host/path,target=/container/path,readonly", "source=.,target=/app"),
		},
	}

	// When: Resolving configuration for "node"
	res, err := Resolve("node", cli, tools, nil)

	// Then: Mounts should be parsed correctly
	require.NoError(t, err)
	require.Len(t, res.Mounts, 2)
	assert.Equal(t, "/host/path", res.Mounts[0].Source)
	assert.Equal(t, "/container/path", res.Mounts[0].Target)
	assert.True(t, res.Mounts[0].ReadOnly)
}

func TestUnit_Resolver_Mounts_WindowsStyleParsing(t *testing.T) {
	t.Parallel()
	// Given: Windows-style mount paths
	cli := CLIOptions{}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:20",
			Mounts: mcs(t,
				`type=bind,source=C:\host\path,target=/container/path`,
				`type=bind,source=D:\data,target=/mnt,readonly`,
				`type=bind,source=Z:\shared folder,target=/app,readonly=false`,
			),
		},
	}

	// When: Resolving configuration for "node"
	res, err := Resolve("node", cli, tools, nil)

	// Then: Mounts should be parsed correctly regardless of platform
	require.NoError(t, err)
	require.Len(t, res.Mounts, 3)
	assert.Equal(t, `C:\host\path`, res.Mounts[0].Source)
	assert.Equal(t, `D:\data`, res.Mounts[1].Source)
	assert.True(t, res.Mounts[1].ReadOnly)
}

func TestUnit_Resolver_Mounts_OverwriteLogic(t *testing.T) {
	t.Parallel()
	// Given: Multiple sources for mounts
	cli := CLIOptions{
		Mounts: []string{"source=/cli/path,target=/cli/target"},
	}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image:  "node",
			Mounts: mcs(t, "source=/tool/path,target=/tool/target"),
		},
	}
	global := &CDERunConfig{
		Defaults: ConfigDefaults{
			Mounts: mcs(t, "source=/global/path,target=/global/target"),
		},
	}

	t.Run("CLI takes priority", func(t *testing.T) {
		res, err := Resolve("node", cli, tools, global)
		require.NoError(t, err)
		assert.Equal(t, "/cli/path", res.Mounts[0].Source)
	})

	t.Run("CDERUN_MOUNT takes priority if CLI is empty", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT": "source=/env/path,target=/env/target"},
		}
		emptyCLI := CLIOptions{}
		res, err := ResolveWithFS("node", emptyCLI, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/env/path", res.Mounts[0].Source)
	})

	t.Run("P1 Overwrites P2", func(t *testing.T) {
		p1CLI := cli
		p1CLI.CderunMounts = []string{"source=/p1/path,target=/p1/target"}
		res, err := Resolve("node", p1CLI, tools, global)
		require.NoError(t, err)
		assert.Equal(t, "/p1/path", res.Mounts[0].Source)
	})
}

func TestUnit_Resolver_Env_StrictResolution(t *testing.T) {
	t.Parallel()
	// Given: Strict mode enabled and some environment variables missing
	mfs := &MockFileSystem{
		Env: map[string]string{
			"EXISTING_VAR": "value",
		},
	}

	t.Run("Default: missing var is empty string", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				Env:   []string{"EXISTING_VAR", "MISSING_VAR"},
			},
		}
		res, err := ResolveWithFS("node", CLIOptions{}, tools, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "EXISTING_VAR=value")
		assert.Contains(t, res.Env, "MISSING_VAR=")
	})

	t.Run("Strict mode from tool config", func(t *testing.T) {
		toolsStrict := ToolsConfig{
			"node": ToolConfig{
				Image:     "node:20",
				Env:       []string{"MISSING_VAR"},
				StrictEnv: ptr(true),
			},
		}
		_, err := ResolveWithFS("node", CLIOptions{}, toolsStrict, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found: MISSING_VAR")
	})

	t.Run("Strict mode from environment variable", func(t *testing.T) {
		mfsStrict := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_STRICT_ENV": "true",
			},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				Env:   []string{"MISSING_VAR"},
			},
		}
		_, err := ResolveWithFS("node", CLIOptions{}, tools, nil, mfsStrict)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found: MISSING_VAR")
	})
}

func TestUnit_Resolver_AutoDetection_RuntimeAndSocket(t *testing.T) {
	t.Parallel()

	t.Run("SocketPath from CDERUN_SOCKET_PATH", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_SOCKET_PATH": "/custom/socket.sock"},
		}
		res, err := ResolveWithFS("node", CLIOptions{}, ToolsConfig{"node": {Image: "node"}}, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/custom/socket.sock", res.SocketPath)
	})

	t.Run("Runtime auto-detection from SocketPath", func(t *testing.T) {
		cli := CLIOptions{
			SocketPath:    "/run/user/1000/podman/podman.sock",
			SocketPathSet: true,
		}
		res, err := Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)

		cli = CLIOptions{
			SocketPath:    "/var/run/docker.sock",
			SocketPathSet: true,
		}
		res, err = Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
	})
}

func TestUnit_Resolver_TransitiveAutoEnablement(t *testing.T) {
	t.Parallel()

	t.Run("MountTools enables MountCderun and MountSocket", func(t *testing.T) {
		cli := CLIOptions{
			MountTools:    "node",
			MountToolsSet: true,
		}
		tools := ToolsConfig{
			"sh": ToolConfig{Image: "alpine"},
		}
		res, err := Resolve("sh", cli, tools, nil)
		require.NoError(t, err)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("Explicit MountSocket=false prevents transitive enablement", func(t *testing.T) {
		cli := CLIOptions{
			MountCderun:          true,
			MountCderunSet:       true,
			CderunMountSocket:    false,
			CderunMountSocketSet: true,
		}
		res, err := Resolve("sh", cli, ToolsConfig{"sh": {Image: "alpine"}}, nil)
		require.NoError(t, err)
		assert.True(t, res.MountCderun)
		assert.False(t, res.MountSocket)
	})
}

func TestUnit_Resolver_Expression_Resolution(t *testing.T) {
	t.Parallel()
	// Given: MockFileSystem with a version file
	mfs := &MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.go-version": []byte("golang:1.25"),
		},
	}

	// When: Resolving configuration with expression in image flag
	cli := CLIOptions{
		Image:    "{{file:.go-version}}",
		ImageSet: true,
	}
	res, err := ResolveWithFS("go", cli, nil, nil, mfs)

	// Then: Expression should be resolved correctly
	require.NoError(t, err)
	assert.Equal(t, "golang:1.25", res.Image)
}

func TestUnit_Resolver_Devices_Resolution(t *testing.T) {
	t.Parallel()
	// Given: Device configurations in multiple layers
	cli := CLIOptions{
		Devices: []string{"/dev/video0:/dev/video0:rw"},
	}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image:   "node",
			Devices: dcs(t, "/dev/null:/dev/null:r"),
		},
	}

	// When: Resolving configuration
	res, err := Resolve("node", cli, tools, nil)

	// Then: CLI should take priority
	require.NoError(t, err)
	require.Len(t, res.Devices, 1)
	assert.Equal(t, "/dev/video0", res.Devices[0].PathOnHost)
}

func TestUnit_Resolver_Logging_Resolution(t *testing.T) {
	t.Parallel()
	// Given: Logging configuration in Global defaults
	global := &CDERunConfig{
		Logging: LoggingConfig{
			Level: "debug",
		},
	}

	// When: Resolving configuration
	res, err := Resolve("node", CLIOptions{}, ToolsConfig{"node": {Image: "node"}}, global)

	// Then: LogLevel should be from Global config
	require.NoError(t, err)
	assert.Equal(t, "debug", res.LogLevel)
}

func TestUnit_Resolver_Float64_CPUsResolution(t *testing.T) {
	t.Parallel()
	// Given: CPUs set in environment variable
	mfs := &MockFileSystem{
		Env: map[string]string{"CDERUN_CPUS": "1.5"},
	}

	// When: Resolving configuration
	res, err := ResolveWithFS("node", CLIOptions{}, ToolsConfig{"node": {Image: "node"}}, nil, mfs)

	// Then: CPUs should be resolved correctly
	require.NoError(t, err)
	assert.InDelta(t, 1.5, res.CPUs, 0.0001)
}

func TestUnit_Resolver_Misc_WorkdirResolution(t *testing.T) {
	t.Parallel()
	// Given: Workdir set in ToolConfig
	tools := ToolsConfig{
		"node": ToolConfig{
			Image:   "node:20",
			Workdir: "/tool/workdir",
		},
	}

	// When: Resolving configuration
	res, err := Resolve("node", CLIOptions{}, tools, nil)

	// Then: Workdir should be from ToolConfig
	require.NoError(t, err)
	assert.Equal(t, "/tool/workdir", res.Workdir)
}

func TestUnit_Resolver_TransitiveAutoEnablement_MountSocket(t *testing.T) {
	t.Parallel()
	// Given: MountCderun set to true
	cli := CLIOptions{
		MountCderun:    true,
		MountCderunSet: true,
	}

	// When: Resolving configuration
	res, err := Resolve("sh", cli, ToolsConfig{"sh": {Image: "alpine"}}, nil)

	// Then: MountSocket should be auto-enabled
	require.NoError(t, err)
	assert.True(t, res.MountSocket)
}
