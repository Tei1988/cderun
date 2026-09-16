package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeature_ExpressionAndValidation_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("expression_resolver_advanced_fallbacks_and_escaping", func(t *testing.T) {
		fs := &MockFileSystem{
			WD:      "/project/sub",
			HomeDir: "/home/user",
			Env: map[string]string{
				"EXISTING_VAR": "value_123",
			},
			Dirs: map[string]bool{
				"/project":     true,
				"/project/sub": true,
			},
			Files: map[string][]byte{
				"/project/sub/valid.txt": []byte("  hello world  "),
				"/project/sub/empty.txt": []byte("   \n\t  "),
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		// 1. Double-brace escaping
		resEscaped, err := resolver.ResolveString(`{{{{env:EXISTING_VAR}}}}`)
		require.NoError(t, err)
		assert.Equal(t, "{{env:EXISTING_VAR}}", resEscaped)

		// 2. Double nested fallback resolution
		resNested, err := resolver.ResolveString("{{env:UNSET_1:-{{env:UNSET_2:-{{env:EXISTING_VAR:-default}}}}}}")
		require.NoError(t, err)
		assert.Equal(t, "value_123", resNested)

		// 3. File fallback on missing file
		resFileMissing, err := resolver.ResolveString("{{file:missing.txt:-fallback_content}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_content", resFileMissing)

		// 4. File fallback on empty file
		resFileEmpty, err := resolver.ResolveString("{{file:empty.txt:-default_content}}")
		require.NoError(t, err)
		assert.Equal(t, "default_content", resFileEmpty)

		// 5. File trimming
		resFileTrim, err := resolver.ResolveString("{{file:valid.txt}}")
		require.NoError(t, err)
		assert.Equal(t, "hello world", resFileTrim)

		// 6. Path traversal error handling in file directive
		_, errTraversal := resolver.ResolveString("{{file:../secret.txt:-fallback}}")
		require.Error(t, errTraversal)
		assert.Contains(t, errTraversal.Error(), "only a single file name is allowed in file directive")

		// Verify sticky error isolation: path traversal set the error
		assert.Error(t, resolver.Error())
	})

	t.Run("expression_resolver_host_context_level_resolution", func(t *testing.T) {
		fs := &MockFileSystem{
			WD:      "/container/workspace",
			HomeDir: "/root",
		}
		hostCtx := &HostContext{
			Level:      1,
			WorkingDir: "/host/project",
			HomeDir:    "/home/hostuser",
		}

		resolver, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		// Level 1 magic word BASE_PWD resolves to host WorkingDir
		resBasePWD, err := resolver.ResolveString("{{BASE_PWD}}")
		require.NoError(t, err)
		assert.Equal(t, "/host/project", resBasePWD)

		// Magic word BASE_HOME resolves to host HomeDir
		resBaseHome, err := resolver.ResolveString("{{BASE_HOME}}")
		require.NoError(t, err)
		assert.Equal(t, "/home/hostuser", resBaseHome)

		// Magic word PWD resolves to current resolver working dir
		resPWD, err := resolver.ResolveString("{{PWD}}")
		require.NoError(t, err)
		assert.Equal(t, "/container/workspace", resPWD)
	})

	t.Run("parameter_validator_network_user_addhost_boundaries", func(t *testing.T) {
		// ValidateNetworkName
		assert.NoError(t, ValidateNetworkName(""))
		assert.NoError(t, ValidateNetworkName("bridge"))
		assert.NoError(t, ValidateNetworkName("my_net-1"))
		assert.NoError(t, ValidateNetworkName("host"))
		assert.Error(t, ValidateNetworkName("net name with spaces"))
		assert.Error(t, ValidateNetworkName("net\x00invalid"))

		// ValidateUserName
		assert.NoError(t, ValidateUserName(""))
		assert.NoError(t, ValidateUserName("root"))
		assert.NoError(t, ValidateUserName("1000:1000"))
		assert.NoError(t, ValidateUserName("user_name-1"))
		assert.Error(t, ValidateUserName("user name"))
		assert.Error(t, ValidateUserName("user\nline"))

		// ValidateAddHost
		assert.NoError(t, ValidateAddHost(""))
		assert.NoError(t, ValidateAddHost("example.com:127.0.0.1"))
		assert.NoError(t, ValidateAddHost("host.local:10.0.0.1"))
		assert.Error(t, ValidateAddHost(":127.0.0.1"))        // Empty hostname
		assert.Error(t, ValidateAddHost("example.com"))       // Missing IP
		assert.Error(t, ValidateAddHost("example.com:invalid")) // Invalid IP
	})

	t.Run("parameter_validator_image_and_tool_boundaries", func(t *testing.T) {
		// ValidateImageName
		assert.NoError(t, ValidateImageName(""))
		assert.NoError(t, ValidateImageName("alpine"))
		assert.NoError(t, ValidateImageName("docker.io/library/alpine:latest"))
		assert.NoError(t, ValidateImageName("localhost:5000/myimage:v1.0"))
		assert.Error(t, ValidateImageName("alpine:"))
		assert.Error(t, ValidateImageName("alpine//latest"))
		assert.Error(t, ValidateImageName("alpine@sha256:"))

		// ValidateToolName
		assert.NoError(t, ValidateToolName("node"))
		assert.NoError(t, ValidateToolName("npm"))
		assert.NoError(t, ValidateToolName("go-1.21"))
		assert.Error(t, ValidateToolName(""))
		assert.Error(t, ValidateToolName("tool with spaces"))
		assert.Error(t, ValidateToolName("../tool"))
	})

	t.Run("sensitive_env_masking_edge_cases", func(t *testing.T) {
		redacted := "[REDACTED]"
		// Mask-all default when sensitiveEnv is nil
		envList := []string{
			"API_KEY=secret_123",
			"DEBUG=true",
			"ALREADY_MASKED=" + redacted,
		}
		masked := MaskSensitiveEnvList(envList, nil)
		require.Len(t, masked, 3)
		assert.Equal(t, "API_KEY="+redacted, masked[0])
		assert.Equal(t, "DEBUG="+redacted, masked[1])
		assert.Equal(t, "ALREADY_MASKED="+redacted, masked[2])

		// Keyword-specific masking when sensitiveEnv is explicitly provided with glob patterns
		customPatterns := []string{"*KEY*", "*TOKEN*"}
		envList2 := []string{
			"API_KEY=secret_123",
			"PUBLIC_VAR=hello",
			"AUTH_TOKEN=bearer_xyz",
		}
		masked2 := MaskSensitiveEnvList(envList2, customPatterns)
		require.Len(t, masked2, 3)
		assert.Equal(t, "API_KEY="+redacted, masked2[0])
		assert.Equal(t, "PUBLIC_VAR=hello", masked2[1])
		assert.Equal(t, "AUTH_TOKEN="+redacted, masked2[2])
	})

	t.Run("resolve_with_fs_full_precedence_matrix", func(t *testing.T) {
		fs := &MockFileSystem{
			WD:      "/app",
			HomeDir: "/home/user",
			Env: map[string]string{
				"CDERUN_IMAGE": "ubuntu:22.04",
			},
		}

		ociRuntimeVal := "crun"
		cliOpts := &CLIOptions{
			OciRuntime: &ociRuntimeVal,
		}

		res, err := ResolveWithFS("sh", cliOpts, nil, nil, fs)
		require.NoError(t, err)

		// CLI override takes precedence for OciRuntime
		assert.Equal(t, "crun", res.OciRuntime)
		// Env var CDERUN_IMAGE supplies Image
		assert.Equal(t, "ubuntu:22.04", res.Image)
	})
}
