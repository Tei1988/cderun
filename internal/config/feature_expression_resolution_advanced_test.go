package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Expression_NestedFallbackAndStickyErrors(t *testing.T) {
	mockFS := &MockFileSystem{
		HomeDir: "/home/testuser",
		WD:      "/workspace/project",
		Env:     map[string]string{"FALLBACK_ENV": "/opt/fallback_dir"},
		Files: map[string][]byte{
			"/workspace/project/oversized.txt": make([]byte, MaxDirectiveFileSize+100),
		},
	}

	hostCtx := &HostContext{
		Level:      1,
		HomeDir:    "/base/home",
		WorkingDir: "/base/pwd",
		UID:        "1001",
		GID:        "1001",
	}

	resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
	require.NoError(t, err)

	t.Run("Multi-level nested fallbacks: find_dir -> env -> file -> default", func(t *testing.T) {
		expr := "{{find_dir:nonexistent:-{{env:NONEXISTENT_ENV:-{{file:nonexistent.txt:-/default/fallback/path}}}}}}"
		res, err := resolver.ResolveString(expr)
		assert.NoError(t, err)
		assert.Equal(t, "/default/fallback/path", res)
		assert.NoError(t, resolver.Error())
	})

	t.Run("Multi-level nested fallbacks: env missing -> fallback_env value", func(t *testing.T) {
		expr := "{{env:MISSING_VAR:-{{env:FALLBACK_ENV:-/default/path}}}}"
		res, err := resolver.ResolveString(expr)
		assert.NoError(t, err)
		assert.Equal(t, "/opt/fallback_dir", res)
		assert.NoError(t, resolver.Error())
	})

	t.Run("MaxDirectiveFileSize rejection", func(t *testing.T) {
		cleanResolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		_, err = cleanResolver.ResolveString("{{file:oversized.txt}}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is too large")
	})

	t.Run("Sticky error isolation across multiple ResolveString invocations", func(t *testing.T) {
		cleanResolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		// Invoking invalid directive sets sticky error
		_, err = cleanResolver.ResolveString("{{BAD_DIRECTIVE:value}}")
		assert.Error(t, err)
		assert.Error(t, cleanResolver.Error())

		// Subsequent calls fail immediately due to sticky error and return unmodified input
		out, err2 := cleanResolver.ResolveString("{{HOME}}")
		assert.Error(t, err2)
		assert.Equal(t, "{{HOME}}", out)
	})

	t.Run("Double brace escaping", func(t *testing.T) {
		cleanResolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		res, err := cleanResolver.ResolveString("{{{{HOME}}}}")
		assert.NoError(t, err)
		assert.Equal(t, "{{HOME}}", res)
	})
}

func TestUnit_Expression_HostContextMagicWords(t *testing.T) {
	uid := 501
	gid := 20
	mockFS := &MockFileSystem{
		HomeDir:  "/Users/localuser",
		WD:       "/Users/localuser/projects/cderun",
		UIDValue: &uid,
		GIDValue: &gid,
	}

	hostCtx := &HostContext{
		Level:      2,
		HomeDir:    "/base/home/runner",
		WorkingDir: "/base/work/cderun",
		UID:        "1000",
		GID:        "1000",
	}

	resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
	require.NoError(t, err)

	cases := []struct {
		name     string
		expr     string
		expected string
	}{
		{"Local HOME", "{{HOME}}", "/Users/localuser"},
		{"Local PWD", "{{PWD}}", "/Users/localuser/projects/cderun"},
		{"Base HOME", "{{BASE_HOME}}", "/base/home/runner"},
		{"Base PWD", "{{BASE_PWD}}", "/base/work/cderun"},
		{"Local UID", "{{UID}}", "501"},
		{"Local GID", "{{GID}}", "20"},
		{"Base UID", "{{BASE_UID}}", "1000"},
		{"Base GID", "{{BASE_GID}}", "1000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := resolver.ResolveString(tc.expr)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, val)
		})
	}
}

func TestUnit_Config_PathSafetyAndValidationInvariants(t *testing.T) {
	t.Run("ValidateNetworkName safety invariants", func(t *testing.T) {
		assert.NoError(t, ValidateNetworkName("bridge"))
		assert.NoError(t, ValidateNetworkName("custom_net-1"))
		assert.Error(t, ValidateNetworkName("net; echo pwned"))
		assert.Error(t, ValidateNetworkName("net\x00null"))
	})

	t.Run("ValidateUserName safety invariants", func(t *testing.T) {
		assert.NoError(t, ValidateUserName("1000:1000"))
		assert.NoError(t, ValidateUserName("appuser"))
		assert.Error(t, ValidateUserName("root; id"))
		assert.Error(t, ValidateUserName("user\nline"))
	})

	t.Run("ValidateAddHost safety invariants", func(t *testing.T) {
		assert.NoError(t, ValidateAddHost("host.docker.internal:127.0.0.1"))
		assert.Error(t, ValidateAddHost("invalid_no_colon"))
		assert.Error(t, ValidateAddHost("host:127.0.0.1; evil"))
	})

	t.Run("ValidateImageName safety invariants", func(t *testing.T) {
		assert.NoError(t, ValidateImageName("alpine:3.18"))
		assert.NoError(t, ValidateImageName("ghcr.io/org/app:v1.0.0"))
		assert.Error(t, ValidateImageName("alpine:latest; rm -rf /"))
		assert.Error(t, ValidateImageName("alpine\x00:latest"))
	})

	t.Run("ValidateWorkdir safety invariants", func(t *testing.T) {
		assert.NoError(t, ValidateWorkdir("/app"))
		assert.NoError(t, ValidateWorkdir("/workspace/src"))
		assert.Error(t, ValidateWorkdir("relative/path"))
		assert.Error(t, ValidateWorkdir("/app; malicious"))
	})

	t.Run("ValidateDNSOption safety invariants", func(t *testing.T) {
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.NoError(t, ValidateDNSOption("timeout:2"))
		assert.Error(t, ValidateDNSOption("ndots:5; malformed"))
	})
}

func TestUnit_Config_PrecedenceMatrixAndFSInvariants(t *testing.T) {
	mockFS := &MockFileSystem{
		HomeDir: "/home/dev",
		WD:      "/projects/app",
		Env: map[string]string{
			"CDERUN_IMAGE": "redis:6-alpine",
		},
	}

	cliOpts := &CLIOptions{
		CderunImage: makeTestStrPtr("redis:7-alpine"),
	}

	globalCfg := &CDERunConfig{
		Engine: "docker",
	}

	res, err := ResolveWithFS("redis", cliOpts, nil, globalCfg, mockFS)
	require.NoError(t, err)
	assert.Equal(t, "redis:7-alpine", res.Image, "P1 CLI option must override P2 environment variable")

	t.Run("Validate path resolution with tilde and relative boundaries", func(t *testing.T) {
		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		absPath, err := ResolvePath("~/data", "/projects/app", resolver)
		assert.NoError(t, err)
		assert.Equal(t, "/home/dev/data", absPath)

		relPath, err := ResolvePath("src/main.go", "/projects/app", resolver)
		assert.NoError(t, err)
		assert.Equal(t, "/projects/app/src/main.go", relPath)
	})

	t.Run("Reverse path resolution tie-breaking", func(t *testing.T) {
		hostCtx := &HostContext{
			Level:      1,
			HomeDir:    "/home/dev",
			WorkingDir: "/projects/app",
			Mounts: []MountMapping{
				{Source: "/data/v1", Target: "/shared/path", Level: 1},
				{Source: "/data/v2", Target: "/shared/path", Level: 2},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		resolvedAbs, err := resolver.applyReverseResolution("/shared/path/file.txt")
		assert.NoError(t, err)
		assert.Equal(t, "/data/v2/file.txt", resolvedAbs, "Reverse resolution must map Target to Source preferring higher level mapping")
	})
}

func makeTestStrPtr(s string) *string {
	return &s
}
