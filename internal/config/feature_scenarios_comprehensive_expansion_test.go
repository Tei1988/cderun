package config_test

import (
	"testing"

	"cderun/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T {
	return &v
}

func TestUnit_ConfigResolution_ComprehensiveEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("NestedTemplateExpressionsAndBraceEscaping", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD:      "/workspace/app",
			HomeDir: "/home/testuser",
			Env: map[string]string{
				"APP_ENV":  "production",
				"PORT_VAR": "8080",
			},
		}

		// Double braces escaping: {{PWD}} inside path
		opts := &config.CLIOptions{
			Image:   ptr("alpine:latest"),
			Workdir: ptr("{{PWD}}/{{env:APP_ENV}}"),
		}

		res, err := config.ResolveWithFS("sh", opts, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/workspace/app/production", res.Workdir)
	})

	t.Run("CpusetAndGPUsValidationBoundaries", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
		}

		// Invalid cpuset format
		optsInvalidCpuset := &config.CLIOptions{
			Image:      ptr("alpine"),
			CpusetCpus: ptr("0-2; rm -rf /"),
		}
		_, err := config.ResolveWithFS("sh", optsInvalidCpuset, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cpuset")

		// Valid cpuset format
		optsValidCpuset := &config.CLIOptions{
			Image:      ptr("alpine"),
			CpusetCpus: ptr("0-3,5"),
			CpusetMems: ptr("0,1"),
		}
		res, err := config.ResolveWithFS("sh", optsValidCpuset, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "0-3,5", res.CpusetCpus)
		assert.Equal(t, "0,1", res.CpusetMems)

		// Invalid GPUs string
		optsInvalidGPUs := &config.CLIOptions{
			Image: ptr("alpine"),
			GPUs:  ptr("all; exec /bin/sh"),
		}
		_, err = config.ResolveWithFS("sh", optsInvalidGPUs, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gpus")
	})

	t.Run("DNSOptionsAndSearchValidations", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
		}

		// Valid DNS options
		optsValid := &config.CLIOptions{
			Image:      ptr("alpine"),
			DNSOptions: []string{"ndots:5", "timeout:2"},
			DNSSearch:  []string{"example.com", "sub.example.com"},
		}
		res, err := config.ResolveWithFS("sh", optsValid, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"ndots:5", "timeout:2"}, res.DNSOptions)
		assert.Equal(t, []string{"example.com", "sub.example.com"}, res.DNSSearch)

		// Invalid DNS option with non-printable ASCII
		optsInvalid := &config.CLIOptions{
			Image:      ptr("alpine"),
			DNSOptions: []string{"ndots:5\x00"},
		}
		_, err = config.ResolveWithFS("sh", optsInvalid, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("SysctlValidationAndResolution", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
		}

		// Valid sysctl entries
		optsValid := &config.CLIOptions{
			Image:   ptr("alpine"),
			Sysctls: []string{"net.ipv4.ip_forward=1", "net.core.somaxconn=1024"},
		}
		res, err := config.ResolveWithFS("sh", optsValid, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"net.ipv4.ip_forward": "1",
			"net.core.somaxconn":  "1024",
		}, res.Sysctls)

		// Invalid sysctl entry (no equals sign)
		optsInvalid := &config.CLIOptions{
			Image:   ptr("alpine"),
			Sysctls: []string{"net.ipv4.ip_forward"},
		}
		_, err = config.ResolveWithFS("sh", optsInvalid, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sysctl")
	})

	t.Run("PrecedenceResolutionP1ToP6", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD: "/app",
			Env: map[string]string{
				"CDERUN_IMAGE": "env-image:v1",
			},
		}

		// P1 CLI override takes precedence over P3 Environment
		cliOpts := &config.CLIOptions{
			Image: ptr("cli-image:v1"),
		}

		res, err := config.ResolveWithFS("sh", cliOpts, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "cli-image:v1", res.Image)

		// When P1 CLI is nil, P3 Environment wins
		cliOptsNil := &config.CLIOptions{}
		resEnv, err := config.ResolveWithFS("sh", cliOptsNil, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "env-image:v1", resEnv.Image)
	})
}
