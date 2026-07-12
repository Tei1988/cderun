package command

import (
	"context"
	"reflect"
	"testing"

	"cderun/internal/config"
	"cderun/internal/logging"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

func getFieldPointers(o *rootOptions, name string, typ string) (any, any) {
	var fieldName string
	switch typ {
	case "string":
		opt, ok := config.GetStringOption(name)
		if !ok {
			return nil, nil
		}
		fieldName = opt.FieldName
	case "bool":
		opt, ok := config.GetBoolOption(name)
		if !ok {
			return nil, nil
		}
		fieldName = opt.FieldName
	case "int":
		opt, ok := config.GetIntOption(name)
		if !ok {
			return nil, nil
		}
		fieldName = opt.FieldName
	case "float64":
		opt, ok := config.GetFloat64Option(name)
		if !ok {
			return nil, nil
		}
		fieldName = opt.FieldName
	case "[]string":
		opt, ok := config.GetStringSliceOption(name)
		if !ok {
			return nil, nil
		}
		fieldName = opt.FieldName
	}

	v := reflect.ValueOf(o).Elem()
	f := v.FieldByName(fieldName)
	cf := v.FieldByName("Cderun" + fieldName)
	if !f.IsValid() || !cf.IsValid() {
		return nil, nil
	}
	return f.Addr().Interface(), cf.Addr().Interface()
}

func getStringPointers(o *rootOptions, name string) (*string, *string) {
	p2, p1 := getFieldPointers(o, name, "string")
	if p2 == nil {
		return nil, nil
	}
	return p2.(*string), p1.(*string)
}

func getBoolPointers(o *rootOptions, name string) (*bool, *bool) {
	p2, p1 := getFieldPointers(o, name, "bool")
	if p2 == nil {
		return nil, nil
	}
	return p2.(*bool), p1.(*bool)
}

func getIntPointers(o *rootOptions, name string) (*int, *int) {
	p2, p1 := getFieldPointers(o, name, "int")
	if p2 == nil {
		return nil, nil
	}
	return p2.(*int), p1.(*int)
}

func getFloat64Pointers(o *rootOptions, name string) (*float64, *float64) {
	p2, p1 := getFieldPointers(o, name, "float64")
	if p2 == nil {
		return nil, nil
	}
	return p2.(*float64), p1.(*float64)
}

func getStringSlicePointers(o *rootOptions, name string) (*[]string, *[]string) {
	p2, p1 := getFieldPointers(o, name, "[]string")
	if p2 == nil {
		return nil, nil
	}
	return p2.(*[]string), p1.(*[]string)
}

func TestUnit_Flags_DockerCompatibilityMapping(t *testing.T) {
	t.Run("basic and complex Docker flags", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{"cderun",
			"--publish", "8080:80",
			"--publish-all",
			"--expose", "80",
			"--hostname", "myhost",
			"--dns", "8.8.8.8",
			"--add-host", "host:1.2.3.4",
			"--user", "1000:1000",
			"--privileged",
			"--cap-add", "SYS_ADMIN",
			"--cap-drop", "KILL",
			"--entrypoint", "/bin/sh",
			"--pull", "always",
			"--memory", "512m",
			"--cpus", "2.5",
			"--mount", "type=tmpfs,target=/tmp",
			"--device", "/dev/fuse",
			"--image", "alpine",
			"alpine", "ls", "-l"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err, "raw args: %v", args)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, []string{"ls", "-l"}, cfg.Command)
		assert.Equal(t, []string{"8080:80"}, cfg.Ports)
		assert.True(t, cfg.PublishAll)
		assert.Equal(t, []string{"80"}, cfg.Expose)
		assert.Equal(t, "myhost", cfg.Hostname)
		assert.Equal(t, []string{"8.8.8.8"}, cfg.DNS)
		assert.Equal(t, []string{"host:1.2.3.4"}, cfg.AddHosts)
		assert.Equal(t, "1000:1000", cfg.User)
		assert.True(t, cfg.Privileged)
		assert.Equal(t, []string{"SYS_ADMIN"}, cfg.CapAdd)
		assert.Equal(t, []string{"KILL"}, cfg.CapDrop)
		assert.Equal(t, []string{"/bin/sh"}, cfg.Entrypoint)
		assert.Equal(t, "always", cfg.Pull)
		assert.Equal(t, int64(512*1024*1024), cfg.Memory)
		assert.InDelta(t, 2.5, cfg.CPUs, 0.0001)
		require.Len(t, cfg.Mounts, 1)
		assert.Equal(t, "tmpfs", cfg.Mounts[0].Type)
		assert.Equal(t, "/tmp", cfg.Mounts[0].Target)
		require.Len(t, cfg.Devices, 1)
		assert.Equal(t, "/dev/fuse", cfg.Devices[0].PathOnHost)
	})

	t.Run("P1 flags override P2 for Docker-compatible features", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{"cderun",
			"--publish", "8080:80",
			"--user", "initialUser",
			"--privileged=true",
			"--pull", "missing",
			"--memory", "1g",
			"--cpus", "1.0",
			"--image", "alpine",
			"alpine",
			"--cderun-publish", "9090:90",
			"--cderun-user", "override-user",
			"--cderun-privileged=false",
			"--cderun-pull", "always",
			"--cderun-memory", "2g",
			"--cderun-cpus", "2.0",
			"ls", "-l"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err, "raw args: %v", args)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, []string{"ls", "-l"}, cfg.Command)
		assert.Equal(t, []string{"9090:90"}, cfg.Ports)
		assert.Equal(t, "override-user", cfg.User)
		assert.False(t, cfg.Privileged)
		assert.Equal(t, "always", cfg.Pull)
		assert.Equal(t, int64(2*1024*1024*1024), cfg.Memory)
		assert.InDelta(t, 2.0, cfg.CPUs, 0.0001)
	})
}

func TestUnit_Command_PreprocessArgs_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("P1 flag before subcommand is an error", func(t *testing.T) {
		args := []string{"cderun", "--cderun-tty", "node", "--version"}
		cmd := newRootCmd(&rootOptions{})
		_, err := preprocessArgs(cmd, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cderun internal override flag \"--cderun-tty\" must be placed after the subcommand")
	})

	t.Run("Hoisting complex P1 flags with values", func(t *testing.T) {
		// cderun node app.js --cderun-image node:20-alpine --cderun-tty --cderun-env KEY=VAL
		args := []string{"cderun", "node", "app.js", "--cderun-image", "node:20-alpine", "--cderun-tty", "--cderun-env", "KEY=VAL"}
		cmd := newRootCmd(&rootOptions{})
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		// Expected: cderun --cderun-image node:20-alpine --cderun-tty --cderun-env KEY=VAL node app.js
		expected := []string{"cderun", "--cderun-image", "node:20-alpine", "--cderun-tty", "--cderun-env", "KEY=VAL", "node", "app.js"}
		assert.Equal(t, expected, processed)
	})

	t.Run("P1 flag with equals sign (no skip next)", func(t *testing.T) {
		args := []string{"cderun", "node", "--cderun-image=alpine", "ls"}
		cmd := newRootCmd(&rootOptions{})
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		assert.Equal(t, []string{"cderun", "--cderun-image=alpine", "node", "ls"}, processed)
	})

	t.Run("shorthand group with value", func(t *testing.T) {
		// actually -p 80:80 node. 'p' takes arg.
		args := []string{"cderun", "-p", "80:80", "node", "ls"}
		cmd := newRootCmd(&rootOptions{})
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)
		assert.Equal(t, []string{"cderun", "-p", "80:80", "node", "ls"}, processed)
	})
}

func TestUnit_Flags_GetStringPointers_Coverage(t *testing.T) {
	o := &rootOptions{}
	for _, opt := range config.StringOptions {
		t.Run(opt.Name, func(t *testing.T) {
			p2, p1 := getStringPointers(o, opt.Name)
			assert.NotNil(t, p2, "P2 field pointer for %s should not be nil", opt.Name)
			assert.NotNil(t, p1, "P1 field pointer for %s should not be nil", opt.Name)
			assert.NotSame(t, p1, p2, "P1 and P2 field pointers for %s should be different", opt.Name)
		})
	}
	p2, p1 := getStringPointers(o, "unknown")
	assert.Nil(t, p2)
	assert.Nil(t, p1)
}

func TestUnit_Flags_GetBoolPointers_Coverage(t *testing.T) {
	o := &rootOptions{}
	for _, opt := range config.BoolOptions {
		t.Run(opt.Name, func(t *testing.T) {
			p2, p1 := getBoolPointers(o, opt.Name)
			assert.NotNil(t, p2, "P2 field pointer for %s should not be nil", opt.Name)
			assert.NotNil(t, p1, "P1 field pointer for %s should not be nil", opt.Name)
			assert.NotSame(t, p1, p2, "P1 and P2 field pointers for %s should be different", opt.Name)
		})
	}
	p2, p1 := getBoolPointers(o, "unknown")
	assert.Nil(t, p2)
	assert.Nil(t, p1)
}

func TestUnit_Flags_GetIntPointers_Coverage(t *testing.T) {
	o := &rootOptions{}
	for _, opt := range config.IntOptions {
		t.Run(opt.Name, func(t *testing.T) {
			p2, p1 := getIntPointers(o, opt.Name)
			assert.NotNil(t, p2, "P2 field pointer for %s should not be nil", opt.Name)
			assert.NotNil(t, p1, "P1 field pointer for %s should not be nil", opt.Name)
			assert.NotSame(t, p1, p2, "P1 and P2 field pointers for %s should be different", opt.Name)
		})
	}
	p2, p1 := getIntPointers(o, "unknown")
	assert.Nil(t, p2)
	assert.Nil(t, p1)
}

func TestUnit_Flags_GetFloat64Pointers_Coverage(t *testing.T) {
	o := &rootOptions{}
	for _, opt := range config.Float64Options {
		t.Run(opt.Name, func(t *testing.T) {
			p2, p1 := getFloat64Pointers(o, opt.Name)
			assert.NotNil(t, p2, "P2 field pointer for %s should not be nil", opt.Name)
			assert.NotNil(t, p1, "P1 field pointer for %s should not be nil", opt.Name)
			assert.NotSame(t, p1, p2, "P1 and P2 field pointers for %s should be different", opt.Name)
		})
	}
	p2, p1 := getFloat64Pointers(o, "unknown")
	assert.Nil(t, p2)
	assert.Nil(t, p1)
}

func TestUnit_Flags_GetStringSlicePointers_Coverage(t *testing.T) {
	o := &rootOptions{}
	for _, opt := range config.StringSliceOptions {
		t.Run(opt.Name, func(t *testing.T) {
			p2, p1 := getStringSlicePointers(o, opt.Name)
			assert.NotNil(t, p2, "P2 field pointer for %s should not be nil", opt.Name)
			assert.NotNil(t, p1, "P1 field pointer for %s should not be nil", opt.Name)
			assert.NotSame(t, p1, p2, "P1 and P2 field pointers for %s should be different", opt.Name)
		})
	}
	p2, p1 := getStringSlicePointers(o, "unknown")
	assert.Nil(t, p2)
	assert.Nil(t, p1)
}
