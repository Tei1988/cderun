package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_DeepResilience_Validators(t *testing.T) {
	t.Parallel()

	t.Run("ValidatePort boundary checks", func(t *testing.T) {
		validPorts := []string{"80:8080", "443:8443/tcp", "8080-8085:9080-9085", "127.0.0.1:80:8080"}
		for _, p := range validPorts {
			assert.NoError(t, ValidatePort(p), "expected valid port: %s", p)
		}

		invalidPorts := []string{"70000:80", "80:70000", "8080-8085:9080-9086", "invalid-port"}
		for _, p := range invalidPorts {
			assert.Error(t, ValidatePort(p), "expected error for invalid port: %s", p)
		}
	})

	t.Run("ValidateGroupAdd and ValidateUserName boundary checks", func(t *testing.T) {
		assert.NoError(t, ValidateGroupAdd("1000"))
		assert.NoError(t, ValidateGroupAdd("wheel"))
		assert.Error(t, ValidateGroupAdd("4294967296")) // exceeds uint32 max
		assert.Error(t, ValidateGroupAdd("invalid group name with spaces!"))

		assert.NoError(t, ValidateUserName("root"))
		assert.NoError(t, ValidateUserName("1000:1000"))
		assert.Error(t, ValidateUserName("bad_user\nname"))
	})

	t.Run("ValidateWorkdir and ValidateExposePort boundary checks", func(t *testing.T) {
		assert.NoError(t, ValidateWorkdir("/app"))
		assert.Error(t, ValidateWorkdir("relative/dir"))
		assert.Error(t, ValidateWorkdir("/app\x00null"))

		assert.NoError(t, ValidateExposePort("8080"))
		assert.NoError(t, ValidateExposePort("8080/tcp"))
		assert.NoError(t, ValidateExposePort("8080-8085/udp"))
		assert.Error(t, ValidateExposePort("0"))
		assert.Error(t, ValidateExposePort("70000"))
	})

	t.Run("ValidateCapability and ValidateAddHost boundary checks", func(t *testing.T) {
		assert.NoError(t, ValidateCapability("SYS_ADMIN"))
		assert.NoError(t, ValidateCapability("CAP_SYS_ADMIN"))
		assert.Error(t, ValidateCapability("CAP_"))
		assert.Error(t, ValidateCapability("SYS__ADMIN"))

		assert.NoError(t, ValidateAddHost("example.com:127.0.0.1"))
		assert.Error(t, ValidateAddHost(":127.0.0.1"))
		assert.Error(t, ValidateAddHost("example.com"))
	})

	t.Run("ValidateSecurityOpt and ValidateSysctl boundary checks", func(t *testing.T) {
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		assert.Error(t, ValidateSecurityOpt("invalid\nopt"))

		assert.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		assert.Error(t, ValidateSysctlKey("invalid key"))

		assert.NoError(t, ValidateSysctlValue("1"))
		assert.Error(t, ValidateSysctlValue("val\x00bad"))
	})

	t.Run("ValidateCpuset, ValidateGPUs, and ValidateMountType boundary checks", func(t *testing.T) {
		assert.NoError(t, ValidateCpuset("0-3"))
		assert.NoError(t, ValidateCpuset("0,1,2"))
		assert.Error(t, ValidateCpuset("-0"))
		assert.Error(t, ValidateCpuset("0--3"))
		assert.Error(t, ValidateCpuset("abc"))

		assert.NoError(t, ValidateGPUs("all"))
		assert.NoError(t, ValidateGPUs("device=0,1"))
		assert.Error(t, ValidateGPUs("-invalid"))
		assert.Error(t, ValidateGPUs("device==0"))

		assert.NoError(t, ValidateMountType("bind"))
		assert.NoError(t, ValidateMountType("volume"))
		assert.NoError(t, ValidateMountType("tmpfs"))
		assert.Error(t, ValidateMountType("unknown_type"))
	})
}

func TestUnit_Config_DeepResilience_ExpressionFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("file expression with fallback", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/existent.txt": []byte("hello_world"),
			},
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		resolved, err := r.ResolveString("val={{file:existent.txt:-fallback}}")
		require.NoError(t, err)
		assert.Equal(t, "val=hello_world", resolved)

		r2, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		resolvedMissing, err := r2.ResolveString("val={{file:nonexistent.txt:-default_val}}")
		require.NoError(t, err)
		assert.Equal(t, "val=default_val", resolvedMissing)
	})

	t.Run("find_dir expression with fallback", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
			Dirs: map[string]bool{
				"/app/target_dir": true,
			},
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		resolved, err := r.ResolveString("dir={{find_dir:target_dir:-/fallback/path}}")
		require.NoError(t, err)
		assert.Equal(t, "dir=/app", resolved)

		r2, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		resolvedMissing, err := r2.ResolveString("dir={{find_dir:missing_dir:-/fallback/path}}")
		require.NoError(t, err)
		assert.Equal(t, "dir=/fallback/path", resolvedMissing)
	})

	t.Run("env expression with nested expression fallback", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
			Env: map[string]string{
				"EXISTING_VAR": "secret_value",
			},
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		resolved, err := r.ResolveString("var={{env:MISSING_VAR:-{{env:EXISTING_VAR}}}}")
		require.NoError(t, err)
		assert.Equal(t, "var=secret_value", resolved)
	})
}

func TestUnit_Config_DeepResilience_Masking(t *testing.T) {
	t.Parallel()

	t.Run("MaskSensitiveEnv with nil or custom patterns", func(t *testing.T) {
		// Mask-all behavior when patterns is nil
		maskedNil := MaskSensitiveEnv("API_KEY", "12345", nil)
		assert.Equal(t, "[REDACTED]", maskedNil)

		// Custom key matching
		maskedCustom := MaskSensitiveEnv("MY_TOKEN", "secret", []string{"*TOKEN*"})
		assert.Equal(t, "[REDACTED]", maskedCustom)

		unmaskedCustom := MaskSensitiveEnv("PUBLIC_VAR", "public", []string{"*TOKEN*"})
		assert.Equal(t, "public", unmaskedCustom)
	})

	t.Run("MaskSensitiveEnvList masking and ordering", func(t *testing.T) {
		input := []string{
			"SECRET_KEY=12345",
			"PUBLIC_VAR=hello",
			"OTHER_VAR=world",
		}

		masked := MaskSensitiveEnvList(input, []string{"*SECRET*"})
		require.Len(t, masked, 3)
		assert.Equal(t, "SECRET_KEY=[REDACTED]", masked[0])
		assert.Equal(t, "PUBLIC_VAR=hello", masked[1])
		assert.Equal(t, "OTHER_VAR=world", masked[2])
	})
}

func TestUnit_Config_DeepResilience_PrecedenceMatrix(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/app",
		Env: map[string]string{
			"CDERUN_IMAGE": "host-env-image:latest",
		},
	}

	cliImage := "cli-image:latest"
	cliOpts := CLIOptions{
		Image: &cliImage,
	}

	cfg, err := ResolveWithFS("node", &cliOpts, nil, nil, mfs)
	require.NoError(t, err)
	// CLI Option (P1) wins over Host Env (P2)
	assert.Equal(t, "cli-image:latest", cfg.Image)
}
