package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ParameterSafetyValidators_Comprehensive(t *testing.T) {
	t.Run("ValidateCpuset syntax and character rules", func(t *testing.T) {
		validCpusets := []string{"0", "0-3", "0,1,2", "0-3,5,7-8"}
		for _, v := range validCpusets {
			require.NoError(t, ValidateCpuset(v), "Cpuset should be valid: %s", v)
		}

		invalidCpusets := []string{
			"0-3-4",   // multiple hyphens in range
			",0",      // leading comma
			"0,",      // trailing comma
			"0,,1",    // consecutive commas
			"0-",      // trailing hyphen
			"-1",      // leading hyphen
			"a-b",     // non-digits
			"0\x00-1", // control char
		}
		for _, inv := range invalidCpusets {
			require.Error(t, ValidateCpuset(inv), "Cpuset should be invalid: %q", inv)
		}
	})

	t.Run("ValidateGPUs syntax and character rules", func(t *testing.T) {
		validGPUs := []string{"all", "1", "2,3", "device=0,1", "driver=nvidia"}
		for _, v := range validGPUs {
			require.NoError(t, ValidateGPUs(v), "GPUs should be valid: %s", v)
		}

		invalidGPUs := []string{
			",all",    // leading comma
			"all,",    // trailing comma
			"1,,2",    // double comma
			"all\x07", // control char
			"all\xFF", // invalid utf8
		}
		for _, inv := range invalidGPUs {
			require.Error(t, ValidateGPUs(inv), "GPUs should be invalid: %q", inv)
		}
	})

	t.Run("ValidateDNSOption syntax and character rules", func(t *testing.T) {
		validOpts := []string{"ndots:5", "timeout:2", "attempts:3", "rotate", "edns0", "use-vc"}
		for _, v := range validOpts {
			require.NoError(t, ValidateDNSOption(v), "DNS option should be valid: %s", v)
		}

		invalidOpts := []string{
			"ndots:5\x00", // control char
			"bad\xFFopt",  // invalid utf-8
			"opt with space",
		}
		for _, inv := range invalidOpts {
			require.Error(t, ValidateDNSOption(inv), "DNS option should be invalid: %q", inv)
		}
	})

	t.Run("ValidateSecurityOpt syntax and character rules", func(t *testing.T) {
		validOpts := []string{"no-new-privileges:true", "seccomp=unconfined", "apparmor=unconfined", "label=disable"}
		for _, v := range validOpts {
			require.NoError(t, ValidateSecurityOpt(v), "Security opt should be valid: %s", v)
		}

		invalidOpts := []string{
			"seccomp\x1b", // C0 control char
			"label=\x7f",  // DEL control char
			"opt\xFF",     // invalid utf-8
		}
		for _, inv := range invalidOpts {
			require.Error(t, ValidateSecurityOpt(inv), "Security opt should be invalid: %q", inv)
		}
	})

	t.Run("ValidateSysctlKey and ValidateSysctlValue", func(t *testing.T) {
		require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		require.NoError(t, ValidateSysctlValue("1"))

		require.Error(t, ValidateSysctlKey("net.ipv4.ip_forward\x00"))
		require.Error(t, ValidateSysctlValue("1\x1b"))
	})

	t.Run("ValidateMountType", func(t *testing.T) {
		validTypes := []string{"bind", "volume", "tmpfs"}
		for _, vt := range validTypes {
			require.NoError(t, ValidateMountType(vt))
		}

		invalidTypes := []string{"invalid", "bind\x00", "volume\xFF"}
		for _, inv := range invalidTypes {
			require.Error(t, ValidateMountType(inv))
		}
	})

	t.Run("ValidateAddHost", func(t *testing.T) {
		require.NoError(t, ValidateAddHost("myhost:127.0.0.1"))
		require.Error(t, ValidateAddHost(":127.0.0.1"), "Empty hostname should be rejected")
		require.Error(t, ValidateAddHost("myhost:"))
		require.Error(t, ValidateAddHost("myhost"))
		require.Error(t, ValidateAddHost("myhost:127.0.0.1\x00"))
	})

	t.Run("ValidateImageName", func(t *testing.T) {
		require.NoError(t, ValidateImageName("ubuntu:latest"))
		require.NoError(t, ValidateImageName("ghcr.io/org/repo:tag@sha256:abc"))

		invalidImages := []string{
			"ubuntu/",
			"ubuntu:",
			"ubuntu@",
			"ubuntu//tag",
			"ubuntu::tag",
			"ubuntu@@sha",
			"ubuntu/:tag",
			"ubuntu\x00",
		}
		for _, inv := range invalidImages {
			require.Error(t, ValidateImageName(inv), "Image name should be rejected: %q", inv)
		}
	})

	t.Run("ValidateEnvKey", func(t *testing.T) {
		require.NoError(t, ValidateEnvKey("FOO_BAR"))
		require.Error(t, ValidateEnvKey(""), "Empty env key")
		require.Error(t, ValidateEnvKey("123KEY"), "Key starting with digit")
		require.Error(t, ValidateEnvKey("FOO-BAR"), "Key with hyphen")
		require.Error(t, ValidateEnvKey("FOO_BAR\x00"), "Key with control char")
	})

	t.Run("ValidateHostname, ValidateUserName, ValidateWorkdir, ValidateToolName", func(t *testing.T) {
		require.NoError(t, ValidateHostname("my-container.local"))
		require.Error(t, ValidateHostname("my-container\x00"))

		require.NoError(t, ValidateUserName("root"))
		require.Error(t, ValidateUserName("user\x1b"))

		require.NoError(t, ValidateWorkdir("/app/src"))
		require.Error(t, ValidateWorkdir("/app\x00/src"))

		require.NoError(t, ValidateToolName("node"))
		require.NoError(t, ValidateToolName("go-1.21"))
		require.Error(t, ValidateToolName(""))
		require.Error(t, ValidateToolName("node/js"))
		require.Error(t, ValidateToolName("tool\x00"))
	})
}

func TestUnit_Config_SecurityPathTraversalValidation(t *testing.T) {
	t.Run("ValidateNetworkName ns:<path> traversal checks", func(t *testing.T) {
		require.NoError(t, ValidateNetworkName("bridge"))
		require.NoError(t, ValidateNetworkName("host"))
		require.NoError(t, ValidateNetworkName("ns:/var/run/netns/test"))

		// Path traversal in ns: namespace
		require.Error(t, ValidateNetworkName("ns:/var/run/netns/../other"))
		require.Error(t, ValidateNetworkName("ns:netns/../../etc/passwd"))
		require.Error(t, ValidateNetworkName("ns:/netns\x00"))
	})

	t.Run("resolveDevicePath parent directory traversal validation", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{"/dev/null": []byte("")},
			Dirs:  map[string]bool{"/dev": true},
			WD:    "/",
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Colon-separated device spec with parent traversal in host path
		_, err = resolveDevicePath("/dev/../etc/passwd:/dev/target", "/", r)
		require.Error(t, err, "Device path with parent traversal must be rejected")
		assert.Contains(t, err.Error(), "parent directory references")

		// Single device path with parent traversal
		_, err = resolveDevicePath("/dev/../dev/null", "/", r)
		require.Error(t, err, "Single device path with parent traversal must be rejected")
		assert.Contains(t, err.Error(), "parent directory references")

		// Valid device path without traversal
		devSpec, err := resolveDevicePath("/dev/null:/dev/null", "/", r)
		require.NoError(t, err)
		assert.Equal(t, "/dev/null:/dev/null", devSpec)
	})
}

func TestUnit_Config_ExpressionFallbacks_Masking_Precedence(t *testing.T) {
	t.Run("Expression fallback syntax", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Files: map[string][]byte{
				"/work/version.txt": []byte("v1.2.3"),
			},
			Dirs: map[string]bool{"/work": true},
		}

		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Environment fallback when variable is un-set
		t.Setenv("TEST_UNSET_VAR_99", "")
		res, err := r.ResolveString("{{env:TEST_UNSET_VAR_99:-default_value}}")
		require.NoError(t, err)
		assert.Equal(t, "default_value", res)

		// File fallback when file does not exist
		res, err = r.ResolveString("{{env:UNSET_FILE_VAR:-{{file:version.txt}}}}")
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", res)
	})

	t.Run("Sensitive environment variable masking", func(t *testing.T) {
		env := []string{
			"PATH=/usr/bin",
			"SECRET_KEY=supersecret123",
			"AWS_SECRET_ACCESS_KEY=myawskey",
			"NORMAL_VAR=hello",
			"DB_PASSWORD=adminpass",
		}
		masked := MaskSensitiveEnvList(env, nil)
		require.Len(t, masked, 5)
		assert.Equal(t, "PATH=[REDACTED]", masked[0])
		assert.Equal(t, "SECRET_KEY=[REDACTED]", masked[1])
		assert.Equal(t, "AWS_SECRET_ACCESS_KEY=[REDACTED]", masked[2])
		assert.Equal(t, "NORMAL_VAR=[REDACTED]", masked[3])
		assert.Equal(t, "DB_PASSWORD=[REDACTED]", masked[4])
	})

	t.Run("Precedence resolution matrix across layers", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/project",
			Dirs: map[string]bool{
				"/project": true,
			},
		}

		globalCfg := &CDERunConfig{
			Defaults: ConfigDefaults{
				Network: "bridge",
				Workdir: "/global/work",
				Env:     []string{"GLOBAL=1", "OVERRIDE=global"},
			},
		}

		projectToolCfg := ToolConfig{
			Image:   "ubuntu:latest",
			Network: "host",
			Workdir: "/project/work",
			Env:     []string{"PROJECT=1", "OVERRIDE=project"},
		}

		cliOpts := &CLIOptions{
			Env: []string{"CLI=1", "OVERRIDE=cli"},
		}

		opts, err := ResolveWithFS("test", cliOpts, ToolsConfig{"test": projectToolCfg}, globalCfg, mfs)
		require.NoError(t, err)

		// Verification of CLI overriding Project overriding Global
		assert.Equal(t, "host", opts.Network)
		assert.Equal(t, "/project/work", opts.Workdir)

		// Environment variable merging and precedence assertion
		envMap := make(map[string]string)
		for _, e := range opts.Env {
			k, v, ok := parseEnvKV(e)
			if ok {
				envMap[k] = v
			}
		}

		assert.Equal(t, "cli", envMap["OVERRIDE"])
	})
}

func parseEnvKV(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
