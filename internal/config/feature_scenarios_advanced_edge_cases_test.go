package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_StringSliceOptResolution_EdgeCases tests string slice resolution options
// with template expressions, tildes, nil/empty slices, and single/multi-item slices.
// References: docs/features/value-resolution.md
func TestUnit_Config_StringSliceOptResolution_EdgeCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/home/user/project",
		HomeDir: "/home/user",
		Env: map[string]string{
			"PROJECT_NAME": "demo",
			"BUILD_ID":     "42",
		},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("resolveStringSliceOptWithVals with expressions and tildes", func(t *testing.T) {
		t.Parallel()
		vals := []string{
			"~/app/{{env:PROJECT_NAME}}",
			"{{PWD}}/build/{{env:BUILD_ID}}",
			"static/path",
		}
		resolved := resolveStringSliceOptWithVals(vals, r)
		assert.Equal(t, []string{
			"/home/user/app/demo",
			"/home/user/project/build/42",
			"static/path",
		}, resolved)
	})

	t.Run("resolveStringSliceOptWithVals with nil or empty input", func(t *testing.T) {
		t.Parallel()
		resNil := resolveStringSliceOptWithVals(nil, r)
		assert.Nil(t, resNil)

		resEmpty := resolveStringSliceOptWithVals([]string{}, r)
		assert.Empty(t, resEmpty)
	})

	t.Run("resolveStringSliceCommaOpt with expressions and comma-separated tokens", func(t *testing.T) {
		t.Parallel()
		def := OptionDef[[]string]{
			EnvKey: "TEST_ENV_VAR",
		}
		res := resolveStringSliceCommaOpt(
			def,
			true, "FOO=1,BAR=2",
			false, "",
			"subcmd",
			ToolsConfig{},
			&CDERunConfig{},
			r,
			mfs,
		)
		assert.Equal(t, []string{"FOO=1", "BAR=2"}, res)
	})
}

// TestUnit_Config_SecurityOpt_And_DNSOption_Validation tests ValidateDNSOption and ValidateSecurityOpt.
// References: docs/features/security-validations.md
func TestUnit_Config_SecurityOpt_And_DNSOption_Validation(t *testing.T) {
	t.Parallel()

	testCasesDNS := []struct {
		name    string
		opt     string
		wantErr bool
	}{
		{"valid timeout", "timeout:5", false},
		{"valid attempts", "attempts:3", false},
		{"valid ndots", "ndots:2", false},
		{"valid single token", "rotate", false},
		{"invalid null byte", "ndots:2\x00", true},
		{"invalid control char", "ndots:2\n", true},
		{"invalid non-ascii", "ndots:2_テスト", true},
	}

	for _, tc := range testCasesDNS {
		t.Run("dns_opt_"+tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateDNSOption(tc.opt)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	testCasesSec := []struct {
		name    string
		opt     string
		wantErr bool
	}{
		{"valid no-new-privileges", "no-new-privileges:true", false},
		{"valid apparmor unconfined", "apparmor=unconfined", false},
		{"valid seccomp unconfined", "seccomp=unconfined", false},
		{"valid label disable", "label=disable", false},
		{"invalid null byte", "apparmor=unconfined\x00", true},
		{"invalid control char", "seccomp=unconfined\r", true},
		{"invalid non-ascii", "seccomp=日本語", true},
	}

	for _, tc := range testCasesSec {
		t.Run("security_opt_"+tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateSecurityOpt(tc.opt)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestUnit_Config_Sysctl_Validation_And_Resolution tests sysctl key/value validation and resolution.
// References: docs/features/security-validations.md, docs/features/value-resolution.md
func TestUnit_Config_Sysctl_Validation_And_Resolution(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/home/user/project",
		HomeDir: "/home/user",
		Env: map[string]string{
			"SHMMAX_VAL": "68719476736",
		},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("ValidateSysctlKey edge cases", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		assert.NoError(t, ValidateSysctlKey("kernel.shmmax"))

		require.Error(t, ValidateSysctlKey(""))
		require.Error(t, ValidateSysctlKey("net.ipv4.ip_forward\x00"))
		require.Error(t, ValidateSysctlKey("net.ipv4.ip forward")) // space in key
		require.Error(t, ValidateSysctlKey("net.ipv4.ip_forward\n"))
	})

	t.Run("ValidateSysctlValue edge cases", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateSysctlValue("1"))
		assert.NoError(t, ValidateSysctlValue("68719476736"))

		require.Error(t, ValidateSysctlValue("1\x00"))
		require.Error(t, ValidateSysctlValue("1\n"))
	})

	t.Run("resolveSysctls with expression resolution", func(t *testing.T) {
		t.Parallel()
		p1 := []string{"net.ipv4.ip_forward=1", "kernel.shmmax={{env:SHMMAX_VAL}}"}
		sysctls, err := resolveSysctls(p1, nil, "", ToolsConfig{}, &CDERunConfig{}, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, "1", sysctls["net.ipv4.ip_forward"])
		assert.Equal(t, "68719476736", sysctls["kernel.shmmax"])
	})

	t.Run("resolveSysctls with control character in key before trim", func(t *testing.T) {
		t.Parallel()
		p1 := []string{"net.ipv4.ip_forward\n=1"}
		_, err := resolveSysctls(p1, nil, "", ToolsConfig{}, &CDERunConfig{}, r, mfs)
		require.Error(t, err)
	})
}

// TestUnit_Config_SensitiveDevice_And_Security_Warnings tests host device classification and security validations.
// References: docs/features/security-validations.md
func TestUnit_Config_SensitiveDevice_And_Security_Warnings(t *testing.T) {
	t.Parallel()

	sensitiveDevices := []string{
		"/dev/mem",
		"/dev/kmem",
		"/dev/port",
		"/dev/msr",
		"/dev/sda1",
		"/dev/nvme0n1",
		"/dev/loop0",
		"/dev/mapper/root",
		"/dev/vda",
	}

	for _, dev := range sensitiveDevices {
		t.Run("sensitive_dev_"+dev, func(t *testing.T) {
			t.Parallel()
			assert.True(t, isHighlySensitiveDevice(dev), "device %s should be flagged as highly sensitive", dev)
		})
	}

	safeDevices := []string{
		"/dev/null",
		"/dev/zero",
		"/dev/random",
		"/dev/urandom",
		"/dev/tty",
	}

	for _, dev := range safeDevices {
		t.Run("safe_dev_"+dev, func(t *testing.T) {
			t.Parallel()
			assert.False(t, isHighlySensitiveDevice(dev), "device %s should not be flagged as highly sensitive", dev)
		})
	}
}

// TestUnit_Config_EnvMasking_And_Resolution tests environment variable resolution and sensitive data masking.
// References: docs/features/sensitive-data-protection.md, docs/features/value-resolution.md
func TestUnit_Config_EnvMasking_And_Resolution(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/home/user/project",
		HomeDir: "/home/user",
		Env: map[string]string{
			"API_SECRET":  "super-secret-123",
			"USER_TOKEN":  "token-xyz",
			"NORMAL_VAR":  "hello-world",
			"PUBLIC_KEY":  "pubkey-456",
			"EMPTY_VAR":   "",
			"EXPR_TARGET": "resolved-value",
		},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("MaskSensitiveEnvList with glob patterns", func(t *testing.T) {
		t.Parallel()
		envList := []string{
			"API_SECRET=super-secret-123",
			"USER_TOKEN=token-xyz",
			"NORMAL_VAR=hello-world",
			"PUBLIC_KEY=pubkey-456",
			"EMPTY_VAR=",
		}
		patterns := []string{"*SECRET*", "USER_*", "*KEY*"}

		masked := MaskSensitiveEnvList(envList, patterns)
		assert.Equal(t, []string{
			"API_SECRET=[REDACTED]",
			"USER_TOKEN=[REDACTED]",
			"NORMAL_VAR=hello-world",
			"PUBLIC_KEY=[REDACTED]",
			"EMPTY_VAR=",
		}, masked)
	})

	t.Run("resolveEnv deduplication and precedence in P1 layer", func(t *testing.T) {
		t.Parallel()
		p1 := []string{"FOO=p1_first", "BAR=p1_bar", "FOO=p1_last"}

		resolved, err := resolveEnv(p1, nil, "", "", ToolsConfig{}, &CDERunConfig{}, nil, false, r, mfs)
		require.NoError(t, err)

		// Last-one-wins within P1: FOO=p1_last, BAR=p1_bar
		assert.Equal(t, []string{"FOO=p1_last", "BAR=p1_bar"}, resolved)
	})

	t.Run("resolveEnvValues with strict mode failure", func(t *testing.T) {
		t.Parallel()
		unresolvedEnv := []string{"UNSET_VAR_XYZ_ABSENT"}
		_, err := resolveEnvValues(unresolvedEnv, nil, true, r, mfs)
		require.Error(t, err)
	})

	t.Run("resolveEnvValues with non-strict mode fallback to host lookup or empty", func(t *testing.T) {
		t.Parallel()
		unresolvedEnv := []string{"UNSET_VAR_XYZ_ABSENT"}
		res, err := resolveEnvValues(unresolvedEnv, nil, false, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"UNSET_VAR_XYZ_ABSENT="}, res)
	})
}

// TestUnit_Config_PathValidation_Boundary_Formats tests path and identifier validation boundary values.
// References: docs/features/security-validations.md
func TestUnit_Config_PathValidation_Boundary_Formats(t *testing.T) {
	t.Parallel()

	t.Run("ValidateCpuset boundary checks", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateCpuset("0-3"))
		assert.NoError(t, ValidateCpuset("0,2,4"))
		assert.NoError(t, ValidateCpuset("0-3,5,7-8"))

		require.Error(t, ValidateCpuset("0-3a"))
		require.Error(t, ValidateCpuset("0-3\n"))
		require.Error(t, ValidateCpuset("0-3\x00"))
	})

	t.Run("ValidateGPUs boundary checks", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateGPUs("all"))
		assert.NoError(t, ValidateGPUs("count=2"))
		assert.NoError(t, ValidateGPUs("device=0,1"))
		assert.NoError(t, ValidateGPUs("1"))

		require.Error(t, ValidateGPUs("all; rm -rf /"))
		require.Error(t, ValidateGPUs("count=2\n"))
		require.Error(t, ValidateGPUs("device=0\x00"))
	})

	t.Run("ValidateGroupAdd boundary checks", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateGroupAdd("1000"))
		assert.NoError(t, ValidateGroupAdd("docker"))
		assert.NoError(t, ValidateGroupAdd("audio"))

		require.Error(t, ValidateGroupAdd("docker;bad"))
		require.Error(t, ValidateGroupAdd("1000\n"))
		require.Error(t, ValidateGroupAdd("docker\x00"))
	})

	t.Run("ValidateUserName boundary checks", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateUserName("root"))
		assert.NoError(t, ValidateUserName("1000"))
		assert.NoError(t, ValidateUserName("1000:1000"))
		assert.NoError(t, ValidateUserName("root:Admin"))

		require.Error(t, ValidateUserName("root\x00"))
		require.Error(t, ValidateUserName("root\n"))
		require.Error(t, ValidateUserName("user:group:extra"))
	})

	t.Run("ValidateCapability boundary checks", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateCapability("SYS_ADMIN"))
		assert.NoError(t, ValidateCapability("CAP_SYS_ADMIN"))
		assert.NoError(t, ValidateCapability("NET_ADMIN"))

		require.Error(t, ValidateCapability("SYS_ADMIN\x00"))
		require.Error(t, ValidateCapability("SYS_ADMIN\n"))
		require.Error(t, ValidateCapability("SYS_ADMIN; rm -rf /"))
	})

	t.Run("ValidateWorkdir boundary checks", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateWorkdir("/app"))
		assert.NoError(t, ValidateWorkdir("/workspace/project_1"))

		require.Error(t, ValidateWorkdir("/app\x00"))
		require.Error(t, ValidateWorkdir("/app\n"))
		require.Error(t, ValidateWorkdir("relative/path"))        // workdir must be absolute
		require.Error(t, ValidateWorkdir("/workspace/../etc")) // parent traversal rejected
	})
}

// TestUnit_Config_ResolvePath_And_AnchorBoundaries tests path resolution with tildes, expressions, and anchors.
// References: docs/features/value-resolution.md
func TestUnit_Config_ResolvePath_And_AnchorBoundaries(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/home/user/project",
		HomeDir: "/home/user",
		Dirs: map[string]bool{
			"/home/user":         true,
			"/home/user/project": true,
		},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("ResolvePath with tilde and relative subpath", func(t *testing.T) {
		t.Parallel()
		res, err := ResolvePath("~/project", mfs.WD, r)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/project", res)
	})

	t.Run("ResolvePath with relative path and baseDir", func(t *testing.T) {
		t.Parallel()
		res, err := ResolvePath("src/main.go", "/home/user/project", r)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/project/src/main.go", res)
	})

	t.Run("ResolvePath with absolute path", func(t *testing.T) {
		t.Parallel()
		res, err := ResolvePath("/etc/hosts", mfs.WD, r)
		require.NoError(t, err)
		assert.Equal(t, "/etc/hosts", res)
	})

	t.Run("HasParentTraversal edge cases", func(t *testing.T) {
		t.Parallel()
		assert.True(t, HasParentTraversal("../foo"))
		assert.True(t, HasParentTraversal("foo/../bar"))
		assert.True(t, HasParentTraversal("foo/.."))
		assert.False(t, HasParentTraversal("foo..bar"))
		assert.False(t, HasParentTraversal(".hidden/file"))
	})
}
