package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ExpressionResolverDirectivesAndMagicWords(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace/project",
		HomeDir: "/home/runner",
		Env: map[string]string{
			"API_KEY":      "secret123",
			"INVALID_ENV":  "val\x01with_control",
			"SAFE_DEFAULT": "my_default",
		},
		Files: map[string][]byte{
			"/workspace/project/config.env":  []byte("  KEY=VALUE  \n"),
			"/workspace/project/empty.txt":   []byte("  \n"),
			"/workspace/project/.marker.txt": []byte("marker"),
		},
	}

	hostCtx := &HostContext{
		Level:      1,
		WorkingDir: "/host/workspace/project",
		HomeDir:    "/host/home/user",
		UID:        "1001",
		GID:        "1001",
		Mounts: []MountMapping{
			{
				Source: "/host/workspace/project",
				Target: "/workspace/project",
				Level:  1,
			},
			{
				Source: "/host/home/user",
				Target: "/home/runner",
				Level:  1,
			},
		},
	}

	t.Run("without_host_context_cloning", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		cloned := resolver.WithoutHostContext()
		require.NotNil(t, cloned)
		assert.Nil(t, cloned.HostContext)
		assert.Equal(t, resolver.Home, cloned.Home)
		assert.Equal(t, resolver.Pwd, cloned.Pwd)

		// Calling WithoutHostContext on nil returns nil
		var nilResolver *ExpressionResolver
		assert.Nil(t, nilResolver.WithoutHostContext())
	})

	t.Run("magic_words_base_context_and_defaults", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		// Level > 0 hostCtx magic words
		res, err := resolver.ResolveString("{{BASE_HOME}}:{{BASE_PWD}}:{{BASE_UID}}:{{BASE_GID}}")
		require.NoError(t, err)
		assert.Equal(t, "/host/home/user:/host/workspace/project:1001:1001", res)

		// Without hostCtx (nil hostCtx falls back to resolver execution host values)
		resolverNoHost, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		resNoHost, err := resolverNoHost.ResolveString("{{BASE_HOME}}:{{BASE_PWD}}:{{BASE_UID}}:{{BASE_GID}}")
		require.NoError(t, err)
		assert.Equal(t, "/home/runner:/workspace/project:1000:1000", resNoHost)
	})

	t.Run("escaped_expressions_syntax", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Escaped expression {{{{env:KEY}}}} -> {{env:KEY}}
		res, err := resolver.ResolveString("{{{{env:API_KEY}}}}")
		require.NoError(t, err)
		assert.Equal(t, "{{env:API_KEY}}", res)
	})

	t.Run("file_and_find_dir_directive_cases", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Existing file resolution
		resFile, err := resolver.ResolveString("{{file:config.env}}")
		require.NoError(t, err)
		assert.Equal(t, "KEY=VALUE", resFile)

		// Existing find_dir resolution
		resDir, err := resolver.ResolveString("{{find_dir:.marker.txt}}")
		require.NoError(t, err)
		assert.Equal(t, "/workspace/project", resDir)
	})

	t.Run("env_directive_validation_and_fallback", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Valid env key
		val, err := resolver.ResolveString("{{env:API_KEY}}")
		require.NoError(t, err)
		assert.Equal(t, "secret123", val)

		// Unset env key with default
		valDefault, err := resolver.ResolveString("{{env:MISSING_VAR:-default_val}}")
		require.NoError(t, err)
		assert.Equal(t, "default_val", valDefault)

		// Invalid env key fails validation
		_, errBadKey := resolver.ResolveString("{{env:BAD-KEY!}}")
		assert.Error(t, errBadKey)

		// Env value with control chars fails validation
		_, errBadVal := resolver.ResolveString("{{env:INVALID_ENV}}")
		assert.Error(t, errBadVal)
	})
}

func TestUnit_Config_ReverseResolutionAndOverlayFS(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace/app",
		HomeDir: "/home/runner",
		Files: map[string][]byte{
			"/workspace/app/main.go": []byte("package main"),
		},
	}

	hostCtx := &HostContext{
		Level:      1,
		WorkingDir: "/host/workspace/app",
		HomeDir:    "/host/home/user",
		UID:        "1000",
		GID:        "1000",
		Mounts: []MountMapping{
			{
				Source: "/host/workspace/app",
				Target: "/workspace/app",
				Level:  1,
			},
			{
				Source: "/host/overlay_upper",
				Target: "/",
				Level:  1,
			},
		},
	}

	t.Run("apply_reverse_resolution_exact_mount_match", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		res, err := resolver.ResolveString("{{find_dir:main.go}}")
		require.NoError(t, err)
		assert.Equal(t, "/host/workspace/app", res)
	})

	t.Run("apply_reverse_resolution_base_host_path_bypass", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		// Target on base host that does not exist on execution host scratch space
		baseHostPath := "/host/home/user/some_file.txt"
		res, err := resolver.applyReverseResolution(baseHostPath)
		require.NoError(t, err)
		assert.Equal(t, baseHostPath, res)
	})
}

func TestUnit_Config_ValidationHelpersAndMasking(t *testing.T) {
	t.Parallel()

	t.Run("validate_port_and_env_key", func(t *testing.T) {
		t.Parallel()

		// Valid ports
		assert.NoError(t, ValidatePort("8080"))

		// Invalid ports
		assert.Error(t, ValidatePort("0"))
		assert.Error(t, ValidatePort("65536"))
		assert.Error(t, ValidatePort("abc"))

		// Valid / Invalid env keys
		assert.NoError(t, ValidateEnvKey("MY_VAR"))
		assert.Error(t, ValidateEnvKey("MY-VAR!"))
	})

	t.Run("validate_sysctl_key_and_value", func(t *testing.T) {
		t.Parallel()

		// Valid sysctls
		assert.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		assert.NoError(t, ValidateSysctlValue("1"))

		// Invalid sysctls
		assert.Error(t, ValidateSysctlKey("net.ipv4.ip_forward!"))
		assert.Error(t, ValidateSysctlValue("1\x01"))
	})

	t.Run("mask_sensitive_env_list", func(t *testing.T) {
		t.Parallel()

		envs := []string{
			"PATH=/usr/bin",
			"SECRET_KEY=supersecret",
			"API_TOKEN=12345",
			"PUBLIC_VAR=hello",
			"ALREADY_REDACTED=[REDACTED]",
		}

		// Explicit patterns
		patterns := []string{"*SECRET*", "*TOKEN*"}
		masked := MaskSensitiveEnvList(envs, patterns)
		assert.Len(t, masked, 5)
		assert.Equal(t, "PATH=/usr/bin", masked[0])
		assert.Equal(t, "SECRET_KEY=[REDACTED]", masked[1])
		assert.Equal(t, "API_TOKEN=[REDACTED]", masked[2])
		assert.Equal(t, "PUBLIC_VAR=hello", masked[3])
		assert.Equal(t, "ALREADY_REDACTED=[REDACTED]", masked[4])

		// Nil patterns defaults to mask-all behavior
		maskedAll := MaskSensitiveEnvList(envs, nil)
		assert.Equal(t, "PATH=[REDACTED]", maskedAll[0])

		// Empty list returns nil/empty fast path
		assert.Empty(t, MaskSensitiveEnvList(nil, nil))
	})
}
