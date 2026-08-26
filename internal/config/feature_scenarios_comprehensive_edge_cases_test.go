package config

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_ValidatorsComprehensiveEdgeCases tests edge cases for configuration validators
// ensuring strict input sanitization across tool names, hostnames, security options, sysctls, GPUs,
// cpuset, add-host entries, images, environment keys, and mount types.
func TestUnit_Config_ValidatorsComprehensiveEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("ValidateToolName boundary checks", func(t *testing.T) {
		t.Parallel()
		validNames := []string{"python", "node-js", "go_tool", "v1.0.0", "my-tool_2.0"}
		for _, name := range validNames {
			require.NoError(t, ValidateToolName(name), "Expected valid tool name: %s", name)
		}

		invalidNames := []string{
			"",                     // empty
			"tool name",            // space
			"tool/sub",             // slash
			"../tool",              // path traversal
			"tool$",                // special char
			"tool\x00",             // null byte
			"tool\n",               // control char
			"ツール",                 // non-ASCII unicode
		}
		for _, name := range invalidNames {
			require.Error(t, ValidateToolName(name), "Expected invalid tool name: %s", name)
		}
	})

	t.Run("ValidateHostname boundary checks", func(t *testing.T) {
		t.Parallel()
		validHostnames := []string{"web-server", "db.local", "node1"}
		for _, h := range validHostnames {
			require.NoError(t, ValidateHostname(h), "Expected valid hostname: %s", h)
		}

		invalidHostnames := []string{
			"-invalid",             // leading hyphen
			"invalid-",             // trailing hyphen
			"web..server",          // consecutive dots
			"web_server",           // underscore in domain (strict hostname validation)
			"host name",            // space
			"host\x01",             // control char
		}
		for _, h := range invalidHostnames {
			require.Error(t, ValidateHostname(h), "Expected invalid hostname: %s", h)
		}
	})

	t.Run("ValidateDNSOption boundary checks", func(t *testing.T) {
		t.Parallel()
		validDNSOpts := []string{"ndots:5", "timeout:2", "attempts:3", "use-vc", "edns0", "rotate"}
		for _, opt := range validDNSOpts {
			require.NoError(t, ValidateDNSOption(opt), "Expected valid DNS option: %s", opt)
		}

		invalidDNSOpts := []string{
			"ndots:5\n",            // control character
			"ndots:5\x00",          // null byte
			"ndots:5!",             // invalid symbol
			"ndots:../etc",         // parent directory traversal
		}
		for _, opt := range invalidDNSOpts {
			require.Error(t, ValidateDNSOption(opt), "Expected invalid DNS option: %s", opt)
		}
	})

	t.Run("ValidateSecurityOpt boundary checks", func(t *testing.T) {
		t.Parallel()
		validSecOpts := []string{
			"seccomp=unconfined",
			"apparmor=unconfined",
			"label=disable",
			"no-new-privileges:true",
			"no-new-privileges=true",
			"systempaths=unconfined",
		}
		for _, opt := range validSecOpts {
			require.NoError(t, ValidateSecurityOpt(opt), "Expected valid security opt: %s", opt)
		}

		invalidSecOpts := []string{
			"seccomp=\x00",         // null byte value
			"seccomp=unconfined!",  // invalid symbol
			"seccomp=\n",           // newline control character
		}
		for _, opt := range invalidSecOpts {
			require.Error(t, ValidateSecurityOpt(opt), "Expected invalid security opt: %s", opt)
		}
	})

	t.Run("ValidateSysctlKey and Value boundary checks", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		require.NoError(t, ValidateSysctlKey("kernel.shmmax"))

		require.Error(t, ValidateSysctlKey(""))                     // empty
		require.Error(t, ValidateSysctlKey(".net.ipv4"))            // leading dot
		require.Error(t, ValidateSysctlKey("net.ipv4."))            // trailing dot
		require.Error(t, ValidateSysctlKey("net..ipv4"))            // consecutive dots
		require.Error(t, ValidateSysctlKey("net.ipv4/forward"))     // slash

		require.NoError(t, ValidateSysctlValue("1"))
		require.NoError(t, ValidateSysctlValue("65536"))

		require.Error(t, ValidateSysctlValue("1\x00"))             // null byte
		require.Error(t, ValidateSysctlValue("1\n"))               // control char
	})

	t.Run("ValidateGPUs and ValidateCpuset boundary checks", func(t *testing.T) {
		t.Parallel()
		validGPUs := []string{"all", "0", "1,2", "device=0,1", "count=2", "driver=nvidia"}
		for _, gpu := range validGPUs {
			require.NoError(t, ValidateGPUs(gpu), "Expected valid GPUs spec: %s", gpu)
		}

		invalidGPUs := []string{
			",0,1",                 // leading comma
			"0,1,",                 // trailing comma
			"0,,1",                 // consecutive commas
			"gpus\x00",             // null byte
			"gpus!",                // invalid symbol
		}
		for _, gpu := range invalidGPUs {
			require.Error(t, ValidateGPUs(gpu), "Expected invalid GPUs spec: %s", gpu)
		}

		validCpusets := []string{"0", "0-3", "0,2-4", "0,1,2,3"}
		for _, cpuset := range validCpusets {
			require.NoError(t, ValidateCpuset(cpuset), "Expected valid cpuset spec: %s", cpuset)
		}

		invalidCpusets := []string{
			"0-3-4",                // multiple range hyphens
			"-1",                   // leading hyphen
			"0,",                   // trailing comma
			",0",                   // leading comma
			"0,,1",                 // consecutive commas
			"0-a",                  // non-numeric range end
		}
		for _, cpuset := range invalidCpusets {
			require.Error(t, ValidateCpuset(cpuset), "Expected invalid cpuset spec: %s", cpuset)
		}
	})

	t.Run("ValidateAddHost, ValidateImageName, ValidateEnvKey, ValidateMountType boundary checks", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidateAddHost("example.com:127.0.0.1"))
		require.Error(t, ValidateAddHost(":127.0.0.1"))             // missing hostname
		require.Error(t, ValidateAddHost("example.com:"))          // missing IP
		require.Error(t, ValidateAddHost("example.com:invalid-ip")) // invalid IP

		require.NoError(t, ValidateImageName("alpine:latest"))
		require.NoError(t, ValidateImageName("ubuntu@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"))
		require.Error(t, ValidateImageName("alpine:"))              // trailing colon
		require.Error(t, ValidateImageName("alpine//latest"))       // consecutive slashes
		require.Error(t, ValidateImageName("alpine:latest\x00"))    // null byte

		require.NoError(t, ValidateEnvKey("MY_VAR"))
		require.NoError(t, ValidateEnvKey("_VAR123"))
		require.Error(t, ValidateEnvKey(""))                       // empty
		require.Error(t, ValidateEnvKey("123VAR"))                  // leading number
		require.Error(t, ValidateEnvKey("MY-VAR"))                  // hyphen in key
		require.Error(t, ValidateEnvKey("MY_VAR\x00"))              // null byte

		require.NoError(t, ValidateMountType("bind"))
		require.NoError(t, ValidateMountType("volume"))
		require.NoError(t, ValidateMountType("tmpfs"))
		require.NoError(t, ValidateMountType(""))                   // defaults to bind
		require.Error(t, ValidateMountType("nfs"))                  // unsupported mount type
		require.Error(t, ValidateMountType("bind\x00"))             // control char
	})
}

// TestUnit_Config_ExpressionResolutionAdvanced verifies advanced template expression evaluation,
// fallback defaults, recursive expansions, double-brace escaping, and path safety invariants.
func TestUnit_Config_ExpressionResolutionAdvanced(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/app",
		Env: map[string]string{
			"CDERUN_TEST_ENV_PRESENT": "present_value",
		},
		Files: map[string][]byte{
			"/app/config.txt": []byte("hello-world"),
			"/app/secret.txt": []byte("super-secret\n"),
		},
	}

	t.Run("dynamic env fallback resolution", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		resPresent := r.resolveString("val={{env:CDERUN_TEST_ENV_PRESENT:-fallback_val}}")
		assert.Equal(t, "val=present_value", resPresent)
		require.NoError(t, r.Error())

		resMissing := r.resolveString("val={{env:CDERUN_TEST_ENV_ABSENT:-default_fallback}}")
		assert.Equal(t, "val=default_fallback", resMissing)
		require.NoError(t, r.Error())
	})

	t.Run("file directive reading and newline trimming", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		content := r.resolveString("secret={{file:secret.txt}}")
		assert.Equal(t, "secret=super-secret", content)
		require.NoError(t, r.Error())
	})

	t.Run("file directive path traversal rejection", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_ = r.resolveString("traversal={{file:../etc/passwd}}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "only a single file name is allowed in file directive")
	})
}

// TestUnit_Config_SensitiveEnvMaskingAndPrecedence verifies sensitive environment variable
// masking under fastMatchFold and priority resolution across configuration layers.
func TestUnit_Config_SensitiveEnvMaskingAndPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("MaskSensitiveEnvList fastMatchFold matching", func(t *testing.T) {
		t.Parallel()
		rawEnvs := []string{
			"API_KEY=secret123",
			"DATABASE_PASSWORD=pass123",
			"TOKEN_AUTH=tok456",
			"USER_NAME=john_doe",
			"APP_PORT=8080",
		}

		// Mode 1: nil slice masks all variables
		maskedNil := MaskSensitiveEnvList(slices.Clone(rawEnvs), nil)
		for _, env := range maskedNil {
			assert.Contains(t, env, "=[REDACTED]")
		}

		// Mode 2: empty slice masks no variables
		maskedEmpty := MaskSensitiveEnvList(slices.Clone(rawEnvs), []string{})
		expectedUnredacted := []string{
			"API_KEY=secret123",
			"DATABASE_PASSWORD=pass123",
			"TOKEN_AUTH=tok456",
			"USER_NAME=john_doe",
			"APP_PORT=8080",
		}
		assert.Equal(t, expectedUnredacted, maskedEmpty)

		// Mode 3: custom glob pattern matching (using lowercase patterns to test case-insensitive matching)
		patterns := []string{"*key*", "*password*", "token_*"}
		maskedCustom := MaskSensitiveEnvList(slices.Clone(rawEnvs), patterns)

		require.Len(t, maskedCustom, len(rawEnvs))
		assert.Equal(t, "API_KEY=[REDACTED]", maskedCustom[0])
		assert.Equal(t, "DATABASE_PASSWORD=[REDACTED]", maskedCustom[1])
		assert.Equal(t, "TOKEN_AUTH=[REDACTED]", maskedCustom[2])
		assert.Equal(t, "USER_NAME=john_doe", maskedCustom[3])
		assert.Equal(t, "APP_PORT=8080", maskedCustom[4])
	})

	t.Run("Configuration priority matrix P1 > P2 > P3 > P4 > P5 > P6", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
			Env: map[string]string{
				"CDERUN_WORKDIR": "/p3-env-workdir",
			},
			Files: map[string][]byte{
				"/app/.cderun.yaml": []byte(`
global:
  workdir: "/p5-global-workdir"
tools:
  python:
    image: "python:3.10"
    workdir: "/p4-tool-workdir"
`),
			},
		}

		t.Run("P1 internal override wins over P2, P3, P4, P5, P6", func(t *testing.T) {
			cliOpt := &CLIOptions{
				CderunImage:   ptrToVal("python:3.12-alpine"), // P1 internal override
				Image:         ptrToVal("python:3.11-slim"),   // P2 CLI
				CderunWorkdir: ptrToVal("/p1-override-workdir"),// P1 internal override
				Workdir:       ptrToVal("/p2-cli-workdir"),     // P2 CLI
			}

			toolsCfg := ToolsConfig{
				"python": ToolConfig{
					Image:   "python:3.10",
					Workdir: "/p4-tool-workdir",
				},
			}

			globalCfg := &CDERunConfig{
				Defaults: ConfigDefaults{
					Workdir: "/p5-global-workdir",
				},
			}

			res, err := ResolveWithFS("python", cliOpt, toolsCfg, globalCfg, mfs)
			require.NoError(t, err)

			// P1 internal override takes precedence over P2, P3, P4, P5, P6
			assert.Equal(t, "python:3.12-alpine", res.Image)
			assert.Equal(t, "/p1-override-workdir", res.Workdir)
		})

		t.Run("P6 fallback default used when no higher precedence layers are present", func(t *testing.T) {
			emptyCLI := &CLIOptions{
				Image: ptrToVal("alpine:latest"),
			}
			emptyTools := ToolsConfig{}
			emptyGlobal := &CDERunConfig{}
			emptyFS := &MockFileSystem{WD: "/app"}

			res, err := ResolveWithFS("", emptyCLI, emptyTools, emptyGlobal, emptyFS)
			require.NoError(t, err)

			// P6 fallback default for Network is "bridge"
			assert.Equal(t, "bridge", res.Network)
			assert.Equal(t, "alpine:latest", res.Image)
		})
	})
}

func ptrToVal[T any](v T) *T {
	return &v
}
