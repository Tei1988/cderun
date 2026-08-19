package config_test

import (
	"testing"

	"cderun/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_ConfigResolution_AdvancedScenarios(t *testing.T) {
	t.Parallel()

	t.Run("DynamicExpressionsAndDefaultFallbacks", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD:      "/workspace/app",
			HomeDir: "/home/developer",
			Env: map[string]string{
				"EXISTING_ENV": "production",
			},
			Files: map[string][]byte{
				"/workspace/app/config.txt": []byte("  db_host=localhost  \n"),
				"/workspace/app/empty.txt":  []byte(""),
			},
		}

		// Fallback environment variable expression
		opts := &config.CLIOptions{
			Image:   ptr("alpine:latest"),
			Workdir: ptr("{{HOME}}/projects/{{env:MISSING_ENV:-default_app}}"),
			Env:     []string{"ENV_VAL={{env:EXISTING_ENV:-staging}}"},
		}

		res, err := config.ResolveWithFS("sh", opts, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/home/developer/projects/default_app", res.Workdir)
		assert.Contains(t, res.Env, "ENV_VAL=production")

		// File expression with whitespace trimming and empty file check
		optsFile := &config.CLIOptions{
			Image: ptr("alpine:latest"),
			Env:   []string{"CFG={{file:config.txt}}", "EMPTY={{file:empty.txt}}"},
		}
		resFile, err := config.ResolveWithFS("sh", optsFile, nil, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, resFile.Env, "CFG=db_host=localhost")
		assert.Contains(t, resFile.Env, "EMPTY=")
	})

	t.Run("OptionBoundaryValidations", func(t *testing.T) {
		t.Parallel()

		mfs := &config.MockFileSystem{
			WD: "/app",
		}

		// Valid shm-size formats
		validShmSizes := []string{"256m", "1g", "1.5g", "512MiB"}
		for _, size := range validShmSizes {
			opts := &config.CLIOptions{
				Image:   ptr("alpine"),
				ShmSize: ptr(size),
			}
			res, err := config.ResolveWithFS("sh", opts, nil, nil, mfs)
			require.NoError(t, err, "expected valid shm-size: %s", size)
			assert.Equal(t, size, res.ShmSize)
		}

		// Invalid shm-size format
		optsInvalidShm := &config.CLIOptions{
			Image:   ptr("alpine"),
			ShmSize: ptr("invalid_shm_size"),
		}
		_, err := config.ResolveWithFS("sh", optsInvalidShm, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shm-size")

		// Valid ulimit formats
		optsValidUlimit := &config.CLIOptions{
			Image:   ptr("alpine"),
			Ulimits: []string{"nofile=1024:2048", "nproc=512"},
		}
		resUlimit, err := config.ResolveWithFS("sh", optsValidUlimit, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, resUlimit.Ulimits, 2)
		assert.Equal(t, "nofile", resUlimit.Ulimits[0].Name)
		assert.Equal(t, int64(1024), resUlimit.Ulimits[0].Soft)
		assert.Equal(t, int64(2048), resUlimit.Ulimits[0].Hard)
		assert.Equal(t, "nproc", resUlimit.Ulimits[1].Name)
		assert.Equal(t, int64(512), resUlimit.Ulimits[1].Soft)
		assert.Equal(t, int64(512), resUlimit.Ulimits[1].Hard)

		// Invalid ulimit format
		optsInvalidUlimit := &config.CLIOptions{
			Image:   ptr("alpine"),
			Ulimits: []string{"invalidtype=100:200"},
		}
		_, err = config.ResolveWithFS("sh", optsInvalidUlimit, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ulimit")

		// Valid restart policies (with remove=false since restart is mutually exclusive with remove)
		validRestarts := []string{"no", "always", "on-failure:5", "unless-stopped"}
		for _, restart := range validRestarts {
			opts := &config.CLIOptions{
				Image:   ptr("alpine"),
				Restart: ptr(restart),
				Remove:  ptr(false),
			}
			res, err := config.ResolveWithFS("sh", opts, nil, nil, mfs)
			require.NoError(t, err, "expected valid restart policy: %s", restart)
			assert.Equal(t, restart, res.Restart)
		}

		// Invalid restart policy
		optsInvalidRestart := &config.CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("invalid_policy"),
			Remove:  ptr(false),
		}
		_, err = config.ResolveWithFS("sh", optsInvalidRestart, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "restart")

		// Valid IPC modes
		validIPCs := []string{"host", "private", "shareable", "none", ""}
		for _, ipc := range validIPCs {
			opts := &config.CLIOptions{
				Image: ptr("alpine"),
				IPC:   ptr(ipc),
			}
			res, err := config.ResolveWithFS("sh", opts, nil, nil, mfs)
			require.NoError(t, err, "expected valid IPC mode: %s", ipc)
			assert.Equal(t, ipc, res.IPC)
		}

		// Invalid IPC mode
		optsInvalidIPC := &config.CLIOptions{
			Image: ptr("alpine"),
			IPC:   ptr("invalid_ipc_mode"),
		}
		_, err = config.ResolveWithFS("sh", optsInvalidIPC, nil, nil, mfs)
		require.Error(t, err)

		// Valid CgroupNS modes
		validCgroupNS := []string{"host", "private", ""}
		for _, cgroupns := range validCgroupNS {
			opts := &config.CLIOptions{
				Image:    ptr("alpine"),
				Cgroupns: ptr(cgroupns),
			}
			res, err := config.ResolveWithFS("sh", opts, nil, nil, mfs)
			require.NoError(t, err, "expected valid CgroupNS mode: %s", cgroupns)
			assert.Equal(t, cgroupns, res.Cgroupns)
		}

		// Invalid CgroupNS mode
		optsInvalidCgroupNS := &config.CLIOptions{
			Image:    ptr("alpine"),
			Cgroupns: ptr("invalid_cgroupns"),
		}
		_, err = config.ResolveWithFS("sh", optsInvalidCgroupNS, nil, nil, mfs)
		require.Error(t, err)

		// Valid PID modes
		validPIDs := []string{"host", "container:mycontainer_1", ""}
		for _, pid := range validPIDs {
			opts := &config.CLIOptions{
				Image: ptr("alpine"),
				Pid:   ptr(pid),
			}
			res, err := config.ResolveWithFS("sh", opts, nil, nil, mfs)
			require.NoError(t, err, "expected valid PID mode: %s", pid)
			assert.Equal(t, pid, res.Pid)
		}

		// Invalid PID modes (empty container target or invalid characters)
		invalidPIDs := []string{"container:", "invalid_pid_mode", "container:mytarget;bad"}
		for _, pid := range invalidPIDs {
			opts := &config.CLIOptions{
				Image: ptr("alpine"),
				Pid:   ptr(pid),
			}
			_, err = config.ResolveWithFS("sh", opts, nil, nil, mfs)
			require.Error(t, err, "expected error for invalid PID mode: %s", pid)
			assert.Contains(t, err.Error(), "pid")
		}

		// Valid PidsLimit (-1, 0, 100) and invalid (-2)
		validPidsLimits := []int{-1, 0, 100}
		for _, limit := range validPidsLimits {
			opts := &config.CLIOptions{
				Image:     ptr("alpine"),
				PidsLimit: ptr(limit),
			}
			resPids, err := config.ResolveWithFS("sh", opts, nil, nil, mfs)
			require.NoError(t, err, "expected valid pids-limit: %d", limit)
			assert.Equal(t, limit, resPids.PidsLimit)
		}

		optsPidsLimitInvalid := &config.CLIOptions{
			Image:     ptr("alpine"),
			PidsLimit: ptr(-2),
		}
		_, err = config.ResolveWithFS("sh", optsPidsLimitInvalid, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pids-limit")

		// Valid CPUShares (1024) and invalid (-1)
		optsCPUSharesValid := &config.CLIOptions{
			Image:     ptr("alpine"),
			CPUShares: ptr(1024),
		}
		resCPU, err := config.ResolveWithFS("sh", optsCPUSharesValid, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, 1024, resCPU.CPUShares)

		optsCPUSharesInvalid := &config.CLIOptions{
			Image:     ptr("alpine"),
			CPUShares: ptr(-1),
		}
		_, err = config.ResolveWithFS("sh", optsCPUSharesInvalid, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cpu-shares")
	})

	t.Run("SensitiveEnvMaskingScenarios", func(t *testing.T) {
		t.Parallel()

		envList := []string{
			"API_KEY=secret_123",
			"USER_TOKEN=abc_456",
			"DB_PASS=super_secret",
			"PUBLIC_CONFIG=normal_value",
		}

		// Custom mask patterns
		maskedCustom := config.MaskSensitiveEnvList(envList, []string{"*KEY*", "*PASS*"})
		assert.Equal(t, []string{
			"API_KEY=[REDACTED]",
			"USER_TOKEN=abc_456",
			"DB_PASS=[REDACTED]",
			"PUBLIC_CONFIG=normal_value",
		}, maskedCustom)

		// Secure-by-default (Mask-all when patterns slice is nil)
		maskedDefault := config.MaskSensitiveEnvList(envList, nil)
		assert.Equal(t, []string{
			"API_KEY=[REDACTED]",
			"USER_TOKEN=[REDACTED]",
			"DB_PASS=[REDACTED]",
			"PUBLIC_CONFIG=[REDACTED]",
		}, maskedDefault)

		// Mode 2: non-nil but empty patterns slice []string{} explicitly disables masking (NO environment variables masked)
		maskedEmptySlice := config.MaskSensitiveEnvList(envList, []string{})
		assert.Equal(t, envList, maskedEmptySlice)
	})
}
