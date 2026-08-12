package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
