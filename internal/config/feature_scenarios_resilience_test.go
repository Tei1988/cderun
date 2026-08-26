package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ScenariosResilience_ValidatorsAndResolvers(t *testing.T) {
	t.Parallel()

	t.Run("gpus_and_cpuset_syntax_validators", func(t *testing.T) {
		t.Parallel()

		// GPUs validation
		require.NoError(t, ValidateGPUs("all"))
		require.NoError(t, ValidateGPUs("count=2"))
		require.NoError(t, ValidateGPUs("device=0,1"))
		require.Error(t, ValidateGPUs(",all"))
		require.Error(t, ValidateGPUs("all,"))
		require.Error(t, ValidateGPUs("device=0,,1"))
		require.Error(t, ValidateGPUs("all\x00"))

		// Cpuset validation
		require.NoError(t, ValidateCpuset("0-3"))
		require.NoError(t, ValidateCpuset("0,2,4"))
		require.NoError(t, ValidateCpuset("0-3,5,7-8"))
		require.Error(t, ValidateCpuset("0--3"))
		require.Error(t, ValidateCpuset("0-1-2"))
		require.Error(t, ValidateCpuset("0,,"))
		require.Error(t, ValidateCpuset("0-3\x00"))
	})

	t.Run("env_key_and_hostname_validators", func(t *testing.T) {
		t.Parallel()

		// Hostname validation
		require.NoError(t, ValidateHostname("my-host"))
		require.NoError(t, ValidateHostname("web.service.local"))
		require.Error(t, ValidateHostname("-invalid-lead"))
		require.Error(t, ValidateHostname("invalid-trail-"))
		require.Error(t, ValidateHostname("host_name_underscore"))
		require.Error(t, ValidateHostname("host\x00name"))

		// Env Key validation
		require.NoError(t, ValidateEnvKey("FOO_BAR"))
		require.NoError(t, ValidateEnvKey("_PRIVATE"))
		require.Error(t, ValidateEnvKey(""))
		require.Error(t, ValidateEnvKey("FOO=BAR"))
		require.Error(t, ValidateEnvKey("FOO\x00BAR"))
	})

	t.Run("mount_type_validator", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateMountType("bind"))
		require.NoError(t, ValidateMountType("volume"))
		require.NoError(t, ValidateMountType("tmpfs"))
		require.NoError(t, ValidateMountType("")) // empty defaults to bind
		require.Error(t, ValidateMountType("nfs"))
		require.Error(t, ValidateMountType("invalid_type"))
	})

	t.Run("expression_resolver_nested_and_file_traversal", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		targetFile := filepath.Join(tmpDir, "secret.txt")
		err := os.WriteFile(targetFile, []byte("my-secret-content\n"), 0600)
		require.NoError(t, err)

		mfs := &MockFileSystem{
			WD:      tmpDir,
			HomeDir: "/home/testuser",
			Env: map[string]string{
				"CONFIG_NAME": "secret.txt",
			},
			Files: map[string][]byte{
				"secret.txt":                []byte("my-secret-content\n"),
				filepath.Base(targetFile):   []byte("my-secret-content\n"),
				targetFile:                  []byte("my-secret-content\n"),
			},
		}

		hostCtx := &HostContext{
			WorkingDir: mfs.WD,
			HomeDir:    mfs.HomeDir,
		}

		resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		// File reading via template
		resFile, err := resolver.ResolveString("{{file:secret.txt}}")
		require.NoError(t, err)
		assert.Equal(t, "my-secret-content", resFile)

		// Parent traversal in file directive should be rejected
		_, err = resolver.ResolveString("{{file:../outside.txt}}")
		require.Error(t, err)
	})

	t.Run("sensitive_env_fast_match_fold", func(t *testing.T) {
		t.Parallel()

		patterns := []string{"*auth*", "api_key*"}

		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("USER_AUTH_TOKEN", "val123", patterns))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("API_KEY_PROD", "val123", patterns))
		assert.Equal(t, "plain_val", MaskSensitiveEnv("PUBLIC_DATA", "plain_val", patterns))

		// Empty patterns slice = mode 2 unmasked
		unmasked := MaskSensitiveEnvList([]string{"SECRET=123"}, []string{})
		require.Len(t, unmasked, 1)
		assert.Equal(t, "SECRET=123", unmasked[0])
	})

	t.Run("precedence_resolution_layer_matrix", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		// P2 CLI options
		shmSizeVal := "512m"
		pidsLimitVal := 500
		readOnlyVal := true
		opts := CLIOptions{
			ShmSize:     &shmSizeVal,
			PidsLimit:   &pidsLimitVal,
			ReadOnly:    &readOnlyVal,
			CapAdd:      []string{"SYS_ADMIN"},
			SecurityOpt: []string{"no-new-privileges:true"},
		}

		// P5 Defaults
		cfg := &CDERunConfig{
			Defaults: ConfigDefaults{
				ShmSize:     "128m",
				CapAdd:      []string{"NET_ADMIN"},
				SecurityOpt: []string{"seccomp=unconfined"},
			},
		}

		tools := ToolsConfig{
			"app": ToolConfig{
				Image: "ubuntu:22.04",
			},
		}

		resolved, err := ResolveWithFS("app", &opts, tools, cfg, mfs)
		require.NoError(t, err)

		// P2 CLI override should win over P5 defaults
		assert.Equal(t, "512m", resolved.ShmSize)
		assert.Equal(t, 500, resolved.PidsLimit)
		assert.True(t, resolved.ReadOnly)
		assert.Equal(t, []string{"SYS_ADMIN"}, resolved.CapAdd)
		assert.Equal(t, []string{"no-new-privileges:true"}, resolved.SecurityOpt)
	})
}
