package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_SecurityOpt_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_SECURITY_OPT": "no-new-privileges,seccomp=unconfined",
		},
	}

	cli := &CLIOptions{
		Image: ptr("alpine"),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, []string{"no-new-privileges", "seccomp=unconfined"}, res.SecurityOpt)
}

func TestUnit_Config_DNSOptions_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_DNS_SEARCH": "example.com,mycompany.com",
			"CDERUN_DNS_OPTION": "ndots:5,timeout:2",
		},
	}

	cli := &CLIOptions{
		Image: ptr("alpine"),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com", "mycompany.com"}, res.DNSSearch)
	assert.Equal(t, []string{"ndots:5", "timeout:2"}, res.DNSOptions)
}

func TestUnit_Config_ShmSizeOption(t *testing.T) {
	t.Parallel()

	t.Run("resolve valid shm-size from CLI", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		cli := &CLIOptions{Image: ptr("alpine"), ShmSize: ptr("512m")}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "512m", res.ShmSize)
	})

	t.Run("resolve from cderun P1 override with priority", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		cli := &CLIOptions{
			Image:         ptr("alpine"),
			ShmSize:       ptr("128m"), // P2
			CderunShmSize: ptr("1g"),   // P1
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "1g", res.ShmSize)
	})

	t.Run("validation failures", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}

		// Invalid size format
		cliInvalid := &CLIOptions{Image: ptr("alpine"), ShmSize: ptr("invalid")}
		_, err := ResolveWithFS("sh", cliInvalid, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")

		// Negative size
		cliNegative := &CLIOptions{Image: ptr("alpine"), ShmSize: ptr("-100")}
		_, err = ResolveWithFS("sh", cliNegative, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")

		// Security characters violation
		cliSecurity := &CLIOptions{Image: ptr("alpine"), ShmSize: ptr("256m\x00")}
		_, err = ResolveWithFS("sh", cliSecurity, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed")
	})
}

func TestUnit_Resolver_Init_Precedence(t *testing.T) {
	t.Parallel()

	t.Run("P1 cderun-init takes highest priority over P2 init", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			Init:       ptr(false),
			CderunInit: ptr(true),
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("P2 init takes priority over Env and Configs", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_INIT": "true",
			},
		}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Init:  ptr(false),
		}
		tools := ToolsConfig{
			"sh": ToolConfig{Init: ptr(true)},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Init: ptr(true)},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.False(t, res.Init)
	})

	t.Run("CDERUN_INIT env takes priority over tool and global config", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_INIT": "true",
			},
		}
		cli := &CLIOptions{Image: ptr("alpine")}
		tools := ToolsConfig{
			"sh": ToolConfig{Init: ptr(false)},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Init: ptr(false)},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("Tool config takes priority over global config", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}
		tools := ToolsConfig{
			"sh": ToolConfig{Init: ptr(true)},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Init: ptr(false)},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("Global config fallback is used when others are empty", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Init: ptr(true)},
		}

		res, err := ResolveWithFS("sh", cli, nil, global, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("Default fallback is false", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.False(t, res.Init)
	})
}

func TestUnit_Config_IPC_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_IPC": "host",
		},
	}

	cli := &CLIOptions{
		Image: ptr("alpine"),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "host", res.IPC)
}

func TestUnit_Config_IPC_Validation(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
	}

	t.Run("empty container IPC reference is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			IPC:   ptr("container:"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported ipc namespace")
	})

	t.Run("valid container IPC reference is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			IPC:   ptr("container:my-container-id"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "container:my-container-id", res.IPC)
	})
}

func TestUnit_Config_ResourceLimits_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_CGROUPNS":    "private",
			"CDERUN_GPUS":        "all",
			"CDERUN_PIDS_LIMIT":  "50",
			"CDERUN_CPU_SHARES":  "1024",
			"CDERUN_CPUSET_CPUS": "0-2",
			"CDERUN_CPUSET_MEMS": "0",
		},
	}

	cli := &CLIOptions{
		Image: ptr("alpine"),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)

	assert.Equal(t, "private", res.Cgroupns)
	assert.Equal(t, "all", res.GPUs)
	assert.Equal(t, 50, res.PidsLimit)
	assert.Equal(t, 1024, res.CPUShares)
	assert.Equal(t, "0-2", res.CpusetCpus)
	assert.Equal(t, "0", res.CpusetMems)
}

func TestUnit_Config_ResourceLimits_Validation(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
	}

	t.Run("negative pids limit is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     ptr("alpine"),
			PidsLimit: ptr(-5),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pids limit cannot be less than -1")
	})

	t.Run("negative cpu shares is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     ptr("alpine"),
			CPUShares: ptr(-10),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cpu shares cannot be negative")
	})
}

func TestUnit_Config_RestartPolicy_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_RESTART": "always",
		},
	}

	cli := &CLIOptions{
		Image:  ptr("alpine"),
		Remove: ptr(false),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "always", res.Restart)
}

func TestUnit_Config_RestartPolicy_Validation(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
	}

	t.Run("invalid restart policy is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("invalid-policy"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported restart policy")
	})

	t.Run("suffix on non-on-failure policy is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("always:3"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not support retry suffix")
	})

	t.Run("multiple suffixes on on-failure policy are rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("on-failure:3:4"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "supports at most one retry suffix")
	})

	t.Run("malformed suffix on on-failure policy is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("on-failure:3abc"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "retry suffix must be a non-negative integer")
	})

	t.Run("negative suffix on on-failure policy is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("on-failure:-3"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "retry suffix must be a non-negative integer")
	})

	t.Run("restart policy with remove enabled is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("always"),
			Remove:  ptr(true),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the --restart policy cannot be used when --remove is enabled")
	})
}

func TestUnit_Config_MountSocketPath_EnvPrecedence(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_MOUNT_CDERUN_SOCKET_PATH": "/run/override/docker.sock",
			"CDERUN_MOUNT_SOCKET_PATH":        "/run/standard/docker.sock",
		},
	}

	cli := &CLIOptions{
		Image: ptr("alpine"),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "/run/override/docker.sock", res.MountSocketPath)
}
