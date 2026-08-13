package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ValidateCpuset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string is allowed", "", false},
		{"single core", "0", false},
		{"list of cores", "0,1,2", false},
		{"range of cores", "0-3", false},
		{"mixed format", "0-3,7,8-11", false},
		{"invalid character - space", "0-3, 7", true},
		{"invalid character - control character", "0-3\n", true},
		{"invalid character - letters", "0-3a", true},
		{"invalid character - special chars", "0-3;echo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCpuset(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateGPUs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string is allowed", "", false},
		{"all keyword", "all", false},
		{"count parameter", "count=2", false},
		{"device parameter", "device=0,1", false},
		{"mixed standard characters", "device=0-1,2", false},
		{"invalid character - space", "device=0, 1", true},
		{"invalid character - control character", "all\x00", true},
		{"invalid character - semicolon", "all;inject", true},
		{"invalid character - pipe", "all|inject", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGPUs(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_Cpuset_GPUs_Integration(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/workspace",
	}

	t.Run("valid cpuset-cpus is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			CpusetCpus: ptr("0-3,7"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "0-3,7", res.CpusetCpus)
	})

	t.Run("invalid cpuset-cpus is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			CpusetCpus: ptr("0-3;inject"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for \"cpuset-cpus\"")
	})

	t.Run("valid cpuset-mems is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			CpusetMems: ptr("0,1"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "0,1", res.CpusetMems)
	})

	t.Run("invalid cpuset-mems is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			CpusetMems: ptr("0-1\n"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for \"cpuset-mems\"")
	})

	t.Run("valid GPUs is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			GPUs:  ptr("device=0,1"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "device=0,1", res.GPUs)
	})

	t.Run("invalid GPUs is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			GPUs:  ptr("all;inject"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for \"gpus\"")
	})
}
