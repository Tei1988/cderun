package config

import (
	"bytes"
	"testing"

	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Path_ValidateCpuset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"0", false},
		{"0-3", false},
		{"0,1", false},
		{"0-3,5,7-9", false},
		{"", false},
		{"0-3; rm -rf", true},
		{"0-3\x01", true},
		{"0-3\x00", true},
		{"0-3\n", true},
		{"0-3a", true},
		{"abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateCpuset(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_Path_ValidateGPUs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"all", false},
		{"count=2", false},
		{"device=0,1", false},
		{"device=NVIDIA-GeForce-RTX-3080", false},
		{"", false},
		{"all; rm -rf", true},
		{"all\x01", true},
		{"all\x00", true},
		{"all\n", true},
		{"device=0,1; rm", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateGPUs(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_Resolver_Cpuset_GPUs_Validation(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("Invalid cpuset-cpus is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			CpusetCpus: ptr("0-3; injection"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters in cpuset")
	})

	t.Run("Invalid cpuset-mems with control char is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			CpusetMems: ptr("0\x00"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("Invalid cpuset-mems is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			CpusetMems: ptr("0a"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters in cpuset")
	})

	t.Run("Invalid GPUs is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			GPUs:  ptr("all; injection"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters in gpus option")
	})

	t.Run("Valid cpuset and GPUs are accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			CpusetCpus: ptr("0-3"),
			CpusetMems: ptr("0,1"),
			GPUs:       ptr("device=0"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "0-3", res.CpusetCpus)
		assert.Equal(t, "0,1", res.CpusetMems)
		assert.Equal(t, "device=0", res.GPUs)
	})
}

func TestUnit_Config_Resolver_CgroupnsHostWarning(t *testing.T) {
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	t.Run("Host cgroupns mode emits security warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Cgroupns: ptr("host"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Container is running with host cgroup namespace enabled")
	})

	t.Run("Private cgroupns mode does not emit host cgroupns warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Cgroupns: ptr("private"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.NotContains(t, logOutput, "Container is running with host cgroup namespace enabled")
	})
}
