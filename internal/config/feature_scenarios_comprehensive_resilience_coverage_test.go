package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_ExpandHome(t *testing.T) {
	home := "/home/testuser"

	assert.Empty(t, expandHome("", home))
	assert.Equal(t, "relative/path", expandHome("relative/path", home))
	assert.Equal(t, "/abs/path", expandHome("/abs/path", home))
	assert.Equal(t, home, expandHome("~", home))
	assert.Equal(t, filepath.Join(home, "config.yaml"), expandHome("~/config.yaml", home))
	assert.Equal(t, "~otheruser/config", expandHome("~otheruser/config", home))
}

func TestUnit_ExpressionResolver_SharedStateAndCaching(t *testing.T) {
	r := &ExpressionResolver{}
	s1 := r.getShared()
	require.NotNil(t, s1)

	s2 := r.getShared()
	assert.Same(t, s1, s2)

	r.ensureShared()
	assert.Same(t, s1, r.getShared())
}

func TestUnit_EnvHelpers_MergeAndDeduplicate(t *testing.T) {
	t.Run("deduplicateEnv", func(t *testing.T) {
		input := []string{"KEY1=val1", "KEY2=val2", "KEY1=val3", "KEY3=val4", "INVALID_NO_EQUALS"}
		res := deduplicateEnv(input)
		expected := []string{"KEY1=val3", "KEY2=val2", "KEY3=val4", "INVALID_NO_EQUALS"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv", func(t *testing.T) {
		base := []string{"A=1", "B=2"}
		p2 := []string{"B=20", "C=30"}
		p1 := []string{"A=10", "D=40"}

		res := mergeEnv(base, p2, p1)
		// Expected precedence: p1 overrides p2, p2 overrides base
		expected := []string{"A=10", "B=20", "C=30", "D=40"}
		assert.Equal(t, expected, res)
	})
}

func TestUnit_ResolverOptions_MemoryAndDuration(t *testing.T) {
	t.Run("applyMemoryOption", func(t *testing.T) {
		rv := &resolver{cli: &CLIOptions{}, res: &ResolvedConfig{}, fs: RealFileSystem{}}
		var target int64

		memOpt := StringOption{
			Name:    "memory",
			Default: "128m",
			EnvKey:  "CDERUN_MEMORY",
		}

		// Valid memory strings via env override
		t.Setenv("CDERUN_MEMORY", "512m")
		err := rv.applyMemoryOption(memOpt, &target)
		require.NoError(t, err)
		assert.Equal(t, int64(512*1024*1024), target)

		t.Setenv("CDERUN_MEMORY", "1g")
		err = rv.applyMemoryOption(memOpt, &target)
		require.NoError(t, err)
		assert.Equal(t, int64(1024*1024*1024), target)

		// Invalid memory string
		t.Setenv("CDERUN_MEMORY", "invalid_size")
		err = rv.applyMemoryOption(memOpt, &target)
		require.Error(t, err)
	})

	t.Run("applyDurationOption", func(t *testing.T) {
		rv := &resolver{cli: &CLIOptions{}, res: &ResolvedConfig{}, fs: RealFileSystem{}}
		durOpt := StringOption{
			Name:    "hang-timeout",
			Default: "5s",
			EnvKey:  "CDERUN_HANG_TIMEOUT",
		}

		// Valid duration via env
		t.Setenv("CDERUN_HANG_TIMEOUT", "10s")
		err := rv.applyDurationOption(durOpt, &rv.res.HangTimeout, true)
		require.NoError(t, err)
		assert.Equal(t, 10*time.Second, rv.res.HangTimeout)

		// Negative check when positive required
		t.Setenv("CDERUN_HANG_TIMEOUT", "-5s")
		err = rv.applyDurationOption(durOpt, &rv.res.HangTimeout, true)
		require.Error(t, err)

		// Invalid duration
		t.Setenv("CDERUN_HANG_TIMEOUT", "invalid_dur")
		err = rv.applyDurationOption(durOpt, &rv.res.HangTimeout, false)
		require.Error(t, err)
	})
}

func TestUnit_ResolverValidation_ContainerPath(t *testing.T) {
	// Valid container paths
	assert.NoError(t, validateContainerPath("/app/data", "mounts", 0, "destination", "destination"))
	assert.NoError(t, validateContainerPath("/var/log", "volumes", 1, "target", "target"))

	// Invalid container paths
	assert.Error(t, validateContainerPath("", "mounts", 0, "destination", "destination"))
	assert.Error(t, validateContainerPath("relative/path", "mounts", 0, "destination", "destination"))
	assert.Error(t, validateContainerPath("/app/../etc", "mounts", 0, "destination", "destination"))
}

func TestUnit_ResolverValidation_SecurityCheckers(t *testing.T) {
	t.Run("validateEnvSecurity", func(t *testing.T) {
		rv := &resolver{
			res: &ResolvedConfig{
				Env: []string{"GOOD=123", "BAD=\x00null"},
			},
		}
		err := rv.validateEnvSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})

	t.Run("validateSysctlSecurity", func(t *testing.T) {
		rv := &resolver{
			res: &ResolvedConfig{
				Sysctls: map[string]string{
					"net.ipv4.ip_forward": "1",
					"bad.key\x00":         "1",
				},
			},
		}
		err := rv.validateSysctlSecurity()
		require.Error(t, err)
	})

	t.Run("validateSlicesSecurity", func(t *testing.T) {
		rvValid := &resolver{
			res: &ResolvedConfig{
				Ports: []string{"8080:8080"},
			},
		}
		assert.NoError(t, rvValid.validateSlices())

		rvInvalidGlob := &resolver{
			res: &ResolvedConfig{
				SensitiveEnv: []string{"["},
			},
		}
		assert.Error(t, rvInvalidGlob.validateSlices())
	})
}

func TestUnit_Directives_ResolveFileAndFindDirEdgeCases(t *testing.T) {
	mfs := &MockFileSystem{
		WD:      "/app",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/app/bigfile.txt": make([]byte, MaxDirectiveFileSize+10),
		},
	}

	r, err := NewExpressionResolverWithFS(&HostContext{WorkingDir: "/app"}, mfs)
	require.NoError(t, err)

	t.Run("resolveFile missing file with default", func(t *testing.T) {
		val, err := r.resolveDirective("file:non_existent.txt:-default_val")
		require.NoError(t, err)
		assert.Equal(t, "default_val", val)
	})

	t.Run("resolveFindDir missing target with default", func(t *testing.T) {
		val, err := r.resolveDirective("find_dir:non_existent_marker:-/default/path")
		require.NoError(t, err)
		assert.Equal(t, "/default/path", val)
	})

	t.Run("resolveFile oversized file error", func(t *testing.T) {
		_, err = r.resolveDirective("file:bigfile.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is too large")
	})
}
