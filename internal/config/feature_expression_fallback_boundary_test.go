package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeature_ExpressionFallbackAndBoundary_AdvancedScenarios(t *testing.T) {
	t.Parallel()

	t.Run("expression_resolver_nested_fallbacks_and_sticky_error_isolation", func(t *testing.T) {
		fs := &MockFileSystem{
			WD:      "/workspace/project",
			HomeDir: "/home/testuser",
			Env: map[string]string{
				"EXISTING_ENV": "active_env_value",
			},
			Dirs: map[string]bool{
				"/workspace/project": true,
			},
			Files: map[string][]byte{
				"/workspace/project/empty.txt": []byte("   \n\t "),
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		// 1. Nested fallback: find_dir missing marker falls back to {{PWD}}
		res1, err := resolver.ResolveString("{{find_dir:nonexistent_marker:-{{PWD}}}}")
		require.NoError(t, err)
		assert.Equal(t, "/workspace/project", res1)

		// 2. Nested fallback: file nonexistent falls back to nested env fallback
		res2, err := resolver.ResolveString("{{file:nonexistent.txt:-{{env:UNSET_ENV:-fallback_val}}}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_val", res2)

		// 3. Empty file content fallback to default
		res3, err := resolver.ResolveString("{{file:empty.txt:-default_for_empty}}")
		require.NoError(t, err)
		assert.Equal(t, "default_for_empty", res3)

		// Confirm sticky error was not polluted during successful fallbacks
		assert.NoError(t, resolver.Error())

		// 4. Fallback security check rejection: control character in default value
		_, errControl := resolver.ResolveString("{{env:UNSET_ENV:-\x01invalid}}")
		require.Error(t, errControl)
		assert.Contains(t, errControl.Error(), "security validation failed")
	})

	t.Run("expression_resolver_strict_magic_words_and_directives", func(t *testing.T) {
		fs := &MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/testuser",
		}

		resolver, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		// Valid magic words
		homeRes, err := resolver.ResolveString("{{HOME}}")
		require.NoError(t, err)
		assert.Equal(t, "/home/testuser", homeRes)

		pwdRes, err := resolver.ResolveString("{{PWD}}")
		require.NoError(t, err)
		assert.Equal(t, "/workspace", pwdRes)

		// Unknown magic word (ALL_CAPS heuristic)
		_, errMagic := resolver.ResolveString("{{UNKNOWN_MAGIC_WORD}}")
		require.Error(t, errMagic)
		assert.Contains(t, errMagic.Error(), "unknown directive or magic word")

		// Reset resolver for fresh error state
		resolverFresh, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		// Unknown directive (contains colon)
		_, errDirective := resolverFresh.ResolveString("{{custom_directive:foo}}")
		require.Error(t, errDirective)
		assert.Contains(t, errDirective.Error(), "unknown directive or magic word")
	})

	t.Run("parameter_validator_port_boundaries", func(t *testing.T) {
		// Valid ports
		assert.NoError(t, ValidatePort(""))
		assert.NoError(t, ValidatePort("8080"))
		assert.NoError(t, ValidatePort("8080:80"))
		assert.NoError(t, ValidatePort("127.0.0.1:8080:80"))
		assert.NoError(t, ValidatePort("127.0.0.1:8080:80/tcp"))

		// Invalid ports
		assert.Error(t, ValidatePort("0"))
		assert.Error(t, ValidatePort("65536"))
		assert.Error(t, ValidatePort("-1"))
		assert.Error(t, ValidatePort("abc"))
		assert.Error(t, ValidatePort("127.0.0.1:70000:80"))
		assert.Error(t, ValidatePort("8080:80/invalidproto"))
	})

	t.Run("parameter_validator_workdir_and_path_boundaries", func(t *testing.T) {
		// Valid workdirs (must be absolute paths)
		assert.NoError(t, ValidateWorkdir(""))
		assert.NoError(t, ValidateWorkdir("/workspace"))
		assert.NoError(t, ValidateWorkdir("/app/src"))
		assert.NoError(t, ValidateWorkdir("/app/node_modules/.pnpm/esbuild@0.25.12/node_modules/esbuild"))
		assert.NoError(t, ValidateWorkdir("/app/node_modules/@scope/pkg+dir"))

		// Invalid workdirs (relative paths or control characters)
		assert.Error(t, ValidateWorkdir("app/src"))
		assert.Error(t, ValidateWorkdir("/app/\x00null"))
		assert.Error(t, ValidateWorkdir("/app/\x07bell"))
	})

	t.Run("parameter_validator_dns_security_cpuset_gpus_boundaries", func(t *testing.T) {
		// DNS options
		assert.NoError(t, ValidateDNSOption(""))
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.Error(t, ValidateDNSOption("ndots:5 space"))
		assert.Error(t, ValidateDNSOption("ndots:5\x00"))

		// Security opt
		assert.NoError(t, ValidateSecurityOpt(""))
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		assert.Error(t, ValidateSecurityOpt("seccomp=\x01bad"))

		// Cpuset
		assert.NoError(t, ValidateCpuset("0-3,5"))
		assert.Error(t, ValidateCpuset("invalid_cpuset!"))

		// GPUs
		assert.NoError(t, ValidateGPUs("all"))
		assert.NoError(t, ValidateGPUs("device=0,1"))
		assert.Error(t, ValidateGPUs("invalid gpu format!"))
	})
}
