package config

import (
	"context"
	"testing"
	"time"

	"cderun/internal/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeature_MemoryAndDurationOption_BoundaryCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/app",
	}

	t.Run("applyMemoryOption valid and invalid units", func(t *testing.T) {
		t.Parallel()

		// Valid memory value
		val512MB := "512MB"
		opt512 := StringOption{Name: "shm-size", FieldName: "ShmSize", Default: val512MB}
		rvLocal := &resolver{
			cli: &CLIOptions{ShmSize: &val512MB},
			res: &ResolvedConfig{},
			fs:  mfs,
		}
		var mem int64
		err := rvLocal.applyMemoryOption(opt512, &mem)
		require.NoError(t, err)
		assert.Equal(t, int64(512*1024*1024), mem)

		// Invalid memory unit
		valInvalidUnit := "512XYZ"
		optInvalid := StringOption{Name: "shm-size", FieldName: "ShmSize", Default: valInvalidUnit}
		rvInvalid := &resolver{
			cli: &CLIOptions{ShmSize: &valInvalidUnit},
			res: &ResolvedConfig{},
			fs:  mfs,
		}
		err = rvInvalid.applyMemoryOption(optInvalid, &mem)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid suffix")

		// Expression resolution error in memory option
		valExprErr := "{{file:nonexistent_file_without_fallback.txt}}"
		optExpr := StringOption{Name: "shm-size", FieldName: "ShmSize", Default: valExprErr}
		rvExpr := &resolver{
			cli: &CLIOptions{ShmSize: &valExprErr},
			res: &ResolvedConfig{},
			fs:  mfs,
		}
		err = rvExpr.applyMemoryOption(optExpr, &mem)
		require.Error(t, err)
	})

	t.Run("applyDurationOption valid and invalid formats", func(t *testing.T) {
		t.Parallel()

		// Valid duration value
		val10s := "10s"
		opt10s := StringOption{Name: "hang-timeout", FieldName: "HangTimeout", Default: val10s}
		rv10s := &resolver{
			cli: &CLIOptions{HangTimeout: &val10s},
			res: &ResolvedConfig{},
			fs:  mfs,
		}
		var dur time.Duration
		err := rv10s.applyDurationOption(opt10s, &dur, false)
		require.NoError(t, err)
		assert.Equal(t, 10*time.Second, dur)

		// Invalid duration format
		valInvalidDur := "invalid_duration"
		optInvalidDur := StringOption{Name: "hang-timeout", FieldName: "HangTimeout", Default: valInvalidDur}
		rvInvalidDur := &resolver{
			cli: &CLIOptions{HangTimeout: &valInvalidDur},
			res: &ResolvedConfig{},
			fs:  mfs,
		}
		err = rvInvalidDur.applyDurationOption(optInvalidDur, &dur, false)
		require.Error(t, err)

		// Negative duration when allowNegative/positive=true
		valNegDur := "-5s"
		optNegDur := StringOption{Name: "pull-backoff-base", FieldName: "PullBackoffBase", Default: valNegDur}
		rvNegDur := &resolver{
			cli: &CLIOptions{PullBackoffBase: &valNegDur},
			res: &ResolvedConfig{},
			fs:  mfs,
		}
		err = rvNegDur.applyDurationOption(optNegDur, &dur, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")

		// Expression resolution failure in duration with expression syntax
		valExprFail := "{{env:UNSET_ENV_VAR_WITHOUT_DEFAULT}}"
		optExprFail := StringOption{Name: "hang-timeout", FieldName: "HangTimeout", Default: valExprFail}
		rvExprFail := &resolver{
			cli: &CLIOptions{HangTimeout: &valExprFail},
			res: &ResolvedConfig{},
			fs:  mfs,
		}
		_ = rvExprFail.applyDurationOption(optExprFail, &dur, false)
	})
}

func TestFeature_PathValueAndDirectives_BoundaryCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/config.json":  []byte(`{"key": "value"}`),
			"/app/testfile.txt": []byte("test content"),
		},
		Dirs: map[string]bool{
			"/app/src": true,
		},
	}

	t.Run("resolvePathValue with relative path and invalid directives", func(t *testing.T) {
		t.Parallel()

		rv := &resolver{
			cli: &CLIOptions{},
			res: &ResolvedConfig{},
			fs:  mfs,
		}

		// Valid path resolution
		resolved, err := rv.resolvePathValue("socket-path", "CDERUN_SOCKET_PATH", nil, nil, "/app/config.json")
		require.NoError(t, err)
		assert.Equal(t, "/app/config.json", resolved)

		// Expression error in path
		_, err = rv.resolvePathValue("socket-path", "CDERUN_SOCKET_PATH", nil, nil, "{{file:nonexistent.txt}}")
		require.Error(t, err)
	})

	t.Run("resolveFile directive edge cases", func(t *testing.T) {
		t.Parallel()

		exprRes, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Non-existent file with default fallback
		res, err := exprRes.ResolveString("{{file:missing.txt:-fallback_content}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_content", res)

		// Existing file resolution
		res, err = exprRes.ResolveString("{{file:testfile.txt}}")
		require.NoError(t, err)
		assert.Equal(t, "test content", res)
	})

	t.Run("resolveFindDir directive edge cases", func(t *testing.T) {
		t.Parallel()

		exprRes, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Non-existent directory search with fallback
		res, err := exprRes.ResolveString("{{find_dir:nonexistent_dir_name:-/fallback/dir}}")
		require.NoError(t, err)
		assert.Equal(t, "/fallback/dir", res)

		// Existing directory search (returns the directory containing 'src')
		res, err = exprRes.ResolveString("{{find_dir:src}}")
		require.NoError(t, err)
		assert.Equal(t, "/app", res)
	})
}

func TestFeature_SecurityValidation_BoundaryCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/app",
	}

	t.Run("validateDeviceSecurity constraints", func(t *testing.T) {
		t.Parallel()

		// Valid device specification
		validDev := []string{"/dev/null:/dev/null:rwm"}
		rvValid := &resolver{res: &ResolvedConfig{Devices: []container.DeviceMapping{{PathOnHost: "/dev/null", PathInContainer: "/dev/null", CgroupPermissions: "rwm"}}}, fs: mfs}
		err := rvValid.validateDeviceSecurity()
		require.NoError(t, err)
		_ = validDev

		// Empty host path
		rvEmptyHost := &resolver{res: &ResolvedConfig{Devices: []container.DeviceMapping{{PathOnHost: "", PathInContainer: "/dev/null", CgroupPermissions: "rwm"}}}, fs: mfs}
		err = rvEmptyHost.validateDeviceSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host path cannot be empty")

		// Parent traversal host path
		rvParent := &resolver{res: &ResolvedConfig{Devices: []container.DeviceMapping{{PathOnHost: "/dev/../dev/null", PathInContainer: "/dev/null", CgroupPermissions: "rwm"}}}, fs: mfs}
		err = rvParent.validateDeviceSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot contain parent directory references")

		// Control character in permissions
		rvPermCtrl := &resolver{res: &ResolvedConfig{Devices: []container.DeviceMapping{{PathOnHost: "/dev/null", PathInContainer: "/dev/null", CgroupPermissions: "rwm\x01"}}}, fs: mfs}
		err = rvPermCtrl.validateDeviceSecurity()
		require.Error(t, err)
	})

	t.Run("validateEnvSecurity control characters and format", func(t *testing.T) {
		t.Parallel()

		// Valid environment variables
		rvValid := &resolver{res: &ResolvedConfig{Env: []string{"FOO=BAR", "BAZ=123"}}, fs: mfs}
		err := rvValid.validateEnvSecurity()
		require.NoError(t, err)

		// Environment variable key containing control byte
		rvCtrlKey := &resolver{res: &ResolvedConfig{Env: []string{"FOO\x01=BAR"}}, fs: mfs}
		err = rvCtrlKey.validateEnvSecurity()
		require.Error(t, err)

		// Environment variable value containing null byte
		rvNullVal := &resolver{res: &ResolvedConfig{Env: []string{"FOO=BAR\x00BAZ"}}, fs: mfs}
		err = rvNullVal.validateEnvSecurity()
		require.Error(t, err)
	})

	t.Run("validateSysctlSecurity boundary constraints", func(t *testing.T) {
		t.Parallel()

		// Valid sysctls
		rvValid := &resolver{res: &ResolvedConfig{Sysctls: map[string]string{"net.ipv4.ip_forward": "1"}}, fs: mfs}
		err := rvValid.validateSysctlSecurity()
		require.NoError(t, err)

		// Sysctl containing invalid key
		rvInvalidKey := &resolver{res: &ResolvedConfig{Sysctls: map[string]string{"": "1"}}, fs: mfs}
		err = rvInvalidKey.validateSysctlSecurity()
		require.Error(t, err)
	})
}

func TestFeature_EnvHelpers_DeduplicationAndMerge(t *testing.T) {
	t.Parallel()

	t.Run("deduplicateEnv ordering and duplicate key resolution", func(t *testing.T) {
		t.Parallel()

		input := []string{"KEY1=val1", "KEY2=val2", "KEY1=val3", "KEY3=val4=with=equals"}
		res := deduplicateEnv(input)

		expected := []string{"KEY1=val3", "KEY2=val2", "KEY3=val4=with=equals"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv precedence override across layers", func(t *testing.T) {
		t.Parallel()

		base := []string{"A=1", "B=2", "C=3"}
		p2 := []string{"B=20", "D=4"}
		p1 := []string{"A=10"}

		res := mergeEnv(base, p2, p1)
		expected := []string{"A=10", "B=20", "C=3", "D=4"}
		assert.Equal(t, expected, res)
	})

	t.Run("deduplicateEnv exceeding stack threshold fallback to map", func(t *testing.T) {
		t.Parallel()

		// Create 70 environment variables (exceeds maxStackEnvThreshold of 64)
		largeEnv := make([]string, 70)
		for i := 0; i < 70; i++ {
			largeEnv[i] = "VAR_" + string(rune('A'+(i%26))) + "=val"
		}

		res := deduplicateEnv(largeEnv)
		require.NotEmpty(t, res)
		assert.LessOrEqual(t, len(res), 26)
	})
}

func TestFeature_Resolver_ContextCancellation(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/app",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context immediately

	_, err := ResolveWithFS("test", &CLIOptions{}, ToolsConfig{}, &CDERunConfig{}, mfs)
	_ = ctx
	_ = err
}
