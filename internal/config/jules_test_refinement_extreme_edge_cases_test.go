package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docs/features/value-resolution.md: "cderun Expressions ({{...}})" and "Nested Expressions"
// Verify that deeply recursive nesting, complex magic-word combinations, tildes in fallback strings,
// and boundary verification work correctly in extreme combinations.
func TestUnit_Config_Resolver_ExtremeNestedAndEscaped(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"SUB_VAR": "resolved_sub",
			"NESTED":  "{{env:SUB_VAR}}",
		},
	}

	t.Run("resolves nested env fallback with tilde at start of fallback path", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// ~ only expands if at absolute start of string. Here, if UNSET is empty, fallback starts with ~
		val, err := r.ResolveString("{{env:UNSET_VAR:-~/path/{{env:SUB_VAR}}}}")
		require.NoError(t, err)
		assert.Equal(t, "/home/user/path/resolved_sub", val)
	})

	t.Run("resolves recursively and handles inner sticky errors on fallback", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// This should resolve the inner expression, then trigger strict resolution error on UNKNOWN_MAGIC_KEY
		_, err = r.ResolveString("{{env:UNSET_VAR:-{{UNKNOWN_MAGIC_KEY}}}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown directive or magic word")
		require.Error(t, r.Error())
	})

	t.Run("preserves double-brace escaping inside multi-layer templates", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Output should preserve the escaped template inside resolved parts
		val, err := r.ResolveString("Value is {{env:UNSET_VAR:-{{{{escaped_val}}}}}}")
		require.NoError(t, err)
		assert.Equal(t, "Value is {{escaped_val}}", val)
	})

	t.Run("evaluates path boundaries when multiple magic words are mixed but boundaries are violated", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Mixed anchors HOME and PWD: HOME is /home/user, PWD is /work
		// The path is "/home/user/work/file.txt", but we want to traverse out of PWD (/work)
		_, err = ResolvePath("{{HOME}}/../user/work/../../etc/passwd", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})
}

// docs/features/value-resolution.md: "Security Hardening and Constraints"
// Verify parameter and directory checks fail closed when illegal chars, control chars, or traversals are supplied.
func TestUnit_Config_Resolver_DirectivesAndSecurityRestrictions(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/work/valid.txt": []byte("valid content"),
		},
	}

	t.Run("rejects file directive parameter containing control characters", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{file:val\x01id.txt}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("rejects find_dir directive parameter with slash separators", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{find_dir:sub/dir}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file or directory name is allowed")
	})

	t.Run("rejects find_dir directive parameter with backward slash", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{find_dir:sub\\dir}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file or directory name is allowed")
	})

	t.Run("rejects find_dir directive parameter with relative traversal parent", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{find_dir:..}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file or directory name is allowed")
	})
}

// Verify option resolution paths under resolver_options.go and resolver.go
func TestUnit_Config_Resolver_OptionsReflectionAndDrift(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	t.Run("resolves various options with expression evaluations and custom formats", func(t *testing.T) {
		fieldOnce.Do(initFieldInfo)

		// Mock CLI Options
		imageVal := "{{env:IMAGE_NAME:-alpine:latest}}"
		shmSizeVal := "{{env:SHM_SIZE:-1.5g}}"
		networkVal := "net-{{env:ENV_NAME:-prod}}"
		ttyVal := true
		interactiveVal := true
		pidsLimitVal := 100
		cpuSharesVal := 512
		cpusVal := 2.5

		cliOpts := &CLIOptions{
			CderunImage:       &imageVal,
			CderunShmSize:     &shmSizeVal,
			CderunNetwork:     &networkVal,
			CderunTTY:         &ttyVal,
			CderunInteractive: &interactiveVal,
			CderunPidsLimit:   &pidsLimitVal,
			CderunCPUShares:   &cpuSharesVal,
			CderunCPUs:        &cpusVal,
		}

		resConfig := &ResolvedConfig{}
		rv := &resolver{
			cli:        cliOpts,
			res:        resConfig,
			subcommand: "test",
			fs:         mfs,
		}

		// Apply options
		err := rv.applyStringOption(StringOption{Name: "image", Default: "ubuntu"})
		require.NoError(t, err)
		assert.Equal(t, "alpine:latest", resConfig.Image)

		err = rv.applyStringOption(StringOption{Name: "shm-size", Default: "64m"})
		require.NoError(t, err)
		assert.Equal(t, "1.5g", resConfig.ShmSize)

		err = rv.applyStringOption(StringOption{Name: "network", Default: "bridge"})
		require.NoError(t, err)
		assert.Equal(t, "net-prod", resConfig.Network)

		err = rv.applyBoolOption(BoolOption{Name: "tty", Default: false})
		require.NoError(t, err)
		assert.True(t, resConfig.TTY)

		err = rv.applyBoolOption(BoolOption{Name: "interactive", Default: false})
		require.NoError(t, err)
		assert.True(t, resConfig.Interactive)

		err = rv.applyIntOption(IntOption{Name: "pids-limit", Default: 0})
		require.NoError(t, err)
		assert.Equal(t, 100, resConfig.PidsLimit)

		err = rv.applyIntOption(IntOption{Name: "cpu-shares", Default: 0})
		require.NoError(t, err)
		assert.Equal(t, 512, resConfig.CPUShares)

		err = rv.applyFloat64Option(Float64Option{Name: "cpus", Default: 0.0})
		require.NoError(t, err)
		assert.InDelta(t, 2.5, resConfig.CPUs, 1e-9)
	})

	t.Run("resolves duration and memory options correctly with defaults and dynamic values", func(t *testing.T) {
		fieldOnce.Do(initFieldInfo)

		hangTimeoutVal := "15s"
		memoryVal := "256m"

		cliOpts := &CLIOptions{
			CderunHangTimeout: &hangTimeoutVal,
			CderunMemory:      &memoryVal,
		}

		resConfig := &ResolvedConfig{}
		rv := &resolver{
			cli:        cliOpts,
			res:        resConfig,
			subcommand: "test",
			fs:         mfs,
		}

		var hangTimeout time.Duration
		err := rv.applyDurationOption(StringOption{Name: "hang-timeout", Default: "5s"}, &hangTimeout, true)
		require.NoError(t, err)
		assert.Equal(t, 15*time.Second, hangTimeout)

		var memoryBytes int64
		err = rv.applyMemoryOption(StringOption{Name: "memory", Default: "128m"}, &memoryBytes)
		require.NoError(t, err)
		assert.Equal(t, int64(268435456), memoryBytes)
	})
}
