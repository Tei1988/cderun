package command

import (
	"context"
	"os"
	"testing"

	"cderun/internal/config"
	"cderun/internal/logging"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_BuildContainerConfig_NestedPathTranslation(t *testing.T) {
	t.Parallel()

	// Simulate being inside a container (Level 1)
	// Host /host/project is mounted as /project in container.
	// cderun is at /usr/local/bin/cderun.
	// We want to run another cderun (nested).

	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/usr/local/bin/cderun": []byte("binary"),
		},
		ExecPath: "/usr/local/bin/cderun",
	}

	hostCtx := &config.HostContext{
		Level: 1,
		Mounts: []config.MountMapping{
			{Source: "/host/project", Target: "/project", Level: 1},
			{Source: "/host/bin/cderun", Target: "/usr/local/bin/cderun", Level: 1},
		},
	}

	o := defaultOptions()
	o.fs = mfs
	o.logger = logging.NewLogger()

	resolved := &config.ResolvedConfig{
		Image:       "alpine",
		HostContext: hostCtx,
		MountCderun: true,
	}

	t.Run("mount-cderun translates to host path in nested execution", func(t *testing.T) {
		cc, err := o.buildContainerConfig(resolved, []string{"echo", "hi"}, nil)
		require.NoError(t, err)

		// Find the mount for cderun
		var cderunMountFound bool
		for _, m := range cc.Mounts {
			if m.Target == "/usr/local/bin/cderun" {
				cderunMountFound = true
				// The source should be the HOST path (/host/bin/cderun), not the container path (/usr/local/bin/cderun)
				assert.Equal(t, "/host/bin/cderun", m.Source)
			}
		}
		assert.True(t, cderunMountFound, "cderun mount not found in container config")
	})
}

func TestUnit_Command_BuildContainerConfig_MountTools_Exhaustive(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/project",
	}

	o := defaultOptions()
	o.fs = mfs
	o.logger = logging.NewLogger()

	toolsCfg := config.ToolsConfig{
		"node":   config.ToolConfig{Image: "node:20"},
		"python": config.ToolConfig{Image: "python:3.11"},
	}

	t.Run("MountAllTools sorts tools for deterministic order", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:         "alpine",
			MountAllTools: true,
		}
		cc, err := o.buildContainerConfig(resolved, nil, toolsCfg)
		require.NoError(t, err)

		// Mounts: [cderun, node, python]
		require.Len(t, cc.Mounts, 3)
		assert.Equal(t, "/usr/local/bin/cderun", cc.Mounts[0].Target)
		assert.Equal(t, "/usr/local/bin/node", cc.Mounts[1].Target)
		assert.Equal(t, "/usr/local/bin/python", cc.Mounts[2].Target)
	})

	t.Run("MountTools specific tools", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:      "alpine",
			MountTools: []string{"python"},
		}
		cc, err := o.buildContainerConfig(resolved, nil, toolsCfg)
		require.NoError(t, err)

		require.Len(t, cc.Mounts, 2)
		assert.Equal(t, "/usr/local/bin/cderun", cc.Mounts[0].Target)
		assert.Equal(t, "/usr/local/bin/python", cc.Mounts[1].Target)
	})

	t.Run("MountTools error on invalid tool name", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:      "alpine",
			MountTools: []string{"../bad"},
		}
		_, err := o.buildContainerConfig(resolved, nil, toolsCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool name")
	})

	t.Run("MountTools error on missing tool config", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:      "alpine",
			MountTools: []string{"missing"},
		}
		_, err := o.buildContainerConfig(resolved, nil, toolsCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tool \"missing\" not found")
	})
}

func TestUnit_Command_WriteFormatted_Exhaustive(t *testing.T) {
	o := defaultOptions()
	o.ensureHooks()

	data := struct {
		Foo string `json:"foo" yaml:"foo"`
	}{Foo: "bar"}

	t.Run("unsupported format", func(t *testing.T) {
		// Use copy to avoid side effects
		o2 := o
		err := o2.writeFormatted(os.Stdout, "xml", data, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported output format: \"xml\"")
	})

	t.Run("json marshal error", func(t *testing.T) {
		// Explicit copy to isolate hooks in subtest
		o2 := o
		o2.jsonMarshalIndent = func(v any, prefix, indent string) ([]byte, error) {
			return nil, assert.AnError
		}
		err := o2.writeFormatted(os.Stdout, "json", data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("yaml marshal error", func(t *testing.T) {
		// Explicit copy to isolate hooks in subtest
		o2 := o
		o2.yamlMarshal = func(v any) ([]byte, error) {
			return nil, assert.AnError
		}
		err := o2.writeFormatted(os.Stdout, "yaml", data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func TestUnit_Command_Execute_ConfigErrors(t *testing.T) {
	// ExecuteContextWithOptions uses a lot of real logic, we can mock parts of it.

	t.Run("invalid pull policy", func(t *testing.T) {
		args := []string{"cderun", "--pull", "invalid", "alpine", "ls"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			o.fs = &config.MockFileSystem{
				Files: map[string][]byte{
					"/project/.tools.yaml": []byte("alpine:\n  image: alpine"),
				},
				WD: "/project",
			}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull policy \"invalid\"")
	})

	t.Run("dry-run without subcommand", func(t *testing.T) {
		args := []string{"cderun", "--dry-run"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a subcommand")
	})
}

func TestUnit_Command_HandleDiagnosis_FS_Error(t *testing.T) {
	o := defaultOptions()
	mfs := &config.MockFileSystem{
		StatErr: assert.AnError,
	}
	o.fs = mfs
	o.ensureHooks()

	cmd := &cobra.Command{}
	resolved := &config.ResolvedConfig{
		Runtime:    "docker",
		SocketPath: "/var/run/docker.sock",
	}

	err := o.handleDiagnosis(cmd, resolved, nil, nil, nil)
	require.NoError(t, err) // handleDiagnosis itself doesn't return error on Stat failure, it records it in info.Runtime.Status
}

func TestUnit_Command_GetFd_Boundary(t *testing.T) {
	// Not easy to test math.MaxInt case without 32-bit arch or fake object.
	// We can test the basic true/false cases.

	fd, ok := getFd(os.Stdin)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, fd, 0)

	fd, ok = getFd("not a file")
	assert.False(t, ok)
	assert.Equal(t, -1, fd)
}
