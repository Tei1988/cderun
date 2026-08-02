package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_TildeExpansion_StartAndInside verifies the tilde expansion behavior
// defined in docs/features/value-resolution.md under various scenarios.
func TestUnit_Config_TildeExpansion_StartAndInside(t *testing.T) {
	t.Parallel()

	// 1. Tilde at the absolute start vs inside a path
	t.Run("tilde at start vs inside", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/work",
			HomeDir: "/home/user",
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Starts with tilde
		res1, err := r.ResolveString("~/documents")
		require.NoError(t, err)
		assert.Equal(t, "/home/user/documents", res1)

		// Inside path (should NOT expand)
		res2, err := r.ResolveString("/path/to/~documents")
		require.NoError(t, err)
		assert.Equal(t, "/path/to/~documents", res2)
	})

	// 2. Missing or empty HOME environment / Home directory handling
	t.Run("empty home directory", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/work",
			HomeDir: "", // Empty HomeDir
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		res, err := r.ResolveString("~/documents")
		require.NoError(t, err)
		// Should join with empty string, resulting in /documents or documents
		assert.Contains(t, res, "documents")
	})

	// 3. Recursive resolution with a tilde in a deep config
	t.Run("recursive slice and map tilde expansion", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/work",
			HomeDir: "/home/user",
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		inputSlice := []any{"~/a", "plain", []any{"~/nested"}}
		resSlice := r.Resolve(inputSlice)
		require.NoError(t, r.Error())

		expectedSlice := []any{"/home/user/a", "plain", []any{"/home/user/nested"}}
		assert.Equal(t, expectedSlice, resSlice)

		inputMap := map[string]any{
			"key1": "~/b",
			"key2": map[string]any{
				"subkey": "~/nested2",
			},
		}
		resMap := r.Resolve(inputMap)
		require.NoError(t, r.Error())

		expectedMap := map[string]any{
			"key1": "/home/user/b",
			"key2": map[string]any{
				"subkey": "/home/user/nested2",
			},
		}
		assert.Equal(t, expectedMap, resMap)
	})
}

// TestUnit_Config_RecursiveResolution_DoubleBraces verifies that recursive resolution
// correctly traverses various config structures as specified in docs/features/value-resolution.md.
func TestUnit_Config_RecursiveResolution_DoubleBraces(t *testing.T) {
	t.Parallel()

	t.Run("nested double braces escaping", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/work",
			HomeDir: "/home/user",
			Env: map[string]string{
				"NESTED_VAR": "outer-val",
			},
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Escaped double braces inside unresolved text should resolve to literal double-braces
		res, err := r.ResolveString("{{ {{env:NESTED_VAR}} }}")
		require.NoError(t, err)
		assert.Equal(t, "{{env:NESTED_VAR}}", res)
	})
}

// TestUnit_Config_NestedExecution_ContextPriority verifies that nested execution
// magic words like {{BASE_HOME}} and {{BASE_PWD}} resolve correctly in both Level 0 (host)
// and Level 1+ (nested container) contexts.
func TestUnit_Config_NestedExecution_ContextPriority(t *testing.T) {
	t.Parallel()

	// Level 0 (Host) Resolution: BASE_HOME/BASE_PWD fall back to local HOME/PWD
	t.Run("Level 0 host execution resolution", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/host/pwd",
			HomeDir: "/host/home",
		}
		// Level 0 HostContext has level = 0 or nil HostContext
		r, err := NewExpressionResolverWithFS(&HostContext{Level: 0}, mfs)
		require.NoError(t, err)

		valHome, err := r.ResolveString("{{BASE_HOME}}")
		require.NoError(t, err)
		assert.Equal(t, "/host/home", valHome)

		valPwd, err := r.ResolveString("{{BASE_PWD}}")
		require.NoError(t, err)
		assert.Equal(t, "/host/pwd", valPwd)
	})

	// Level 1 (Nested) Resolution: BASE_HOME/BASE_PWD resolve to outer host values
	t.Run("Level 1 nested execution resolution", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/container/pwd",
			HomeDir: "/container/home",
		}
		hostCtx := &HostContext{
			Level:      1,
			HomeDir:    "/physical/home",
			WorkingDir: "/physical/pwd",
		}
		r, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		valHome, err := r.ResolveString("{{BASE_HOME}}")
		require.NoError(t, err)
		assert.Equal(t, "/physical/home", valHome)

		valPwd, err := r.ResolveString("{{BASE_PWD}}")
		require.NoError(t, err)
		assert.Equal(t, "/physical/pwd", valPwd)

		// Normal HOME/PWD should still resolve to local (container) home/pwd
		valLocalHome, err := r.ResolveString("{{HOME}}")
		require.NoError(t, err)
		assert.Equal(t, "/container/home", valLocalHome)

		valLocalPwd, err := r.ResolveString("{{PWD}}")
		require.NoError(t, err)
		assert.Equal(t, "/container/pwd", valLocalPwd)
	})
}

// TestUnit_Config_EnvSecurity_ValidationAndNullBytes verifies environment variables validations,
// verifying invalid format keys are rejected and null bytes in keys or values are caught.
func TestUnit_Config_EnvSecurity_ValidationAndNullBytes(t *testing.T) {
	t.Parallel()

	// Test case: Env keys validation
	t.Run("environment keys validation formats", func(t *testing.T) {
		validKeys := []string{"MY_VAR", "var_name", "VAR123", "_VAR"}
		for _, k := range validKeys {
			require.NoError(t, ValidateEnvKey(k), "key %q should be valid", k)
		}

		invalidKeys := []string{"", "123VAR", "MY-VAR", "MY$VAR", "MY VAR", "VALID\x00KEY"}
		for _, k := range invalidKeys {
			assert.Error(t, ValidateEnvKey(k), "key %q should be invalid", k)
		}
	})

	// Test case: Null byte rejection in env values
	t.Run("null byte injection in env values rejected", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Inject null byte into environment value
		envInput := []string{"VALID_KEY=value\x00malicious"}
		_, err = resolveEnvValues(envInput, nil, false, r, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})
}

// TestUnit_Config_PathSecurity_TraversalsAndValidation validates security checks on container targets
// and host sources to ensure path-traversal safety and absolute destination checks.
func TestUnit_Config_PathSecurity_TraversalsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("ValidateWorkdir strictly rejects parent traversal", func(t *testing.T) {
		err := ValidateWorkdir("/app/../escape")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "working directory cannot contain parent directory references")

		err = ValidateWorkdir("../escape")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "working directory must be an absolute path")

		err = ValidateWorkdir("/valid/absolute/path")
		require.NoError(t, err)
	})

	t.Run("Mount targets must not have traversal", func(t *testing.T) {
		// Mock resolver and config
		mfs := &MockFileSystem{WD: "/work"}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Mounts: []string{"type=bind,source=/work,target=/app/../escape"},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot contain parent directory references")
	})

	t.Run("Device destinations must not have traversal", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Devices: []string{"/dev/null:/app/../escape"},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot contain parent directory references")
	})
}

// TestUnit_Config_Resolve_WithPrivilegedAndSocketSettings tests configuration-resolution coverage
// for options involving privileged modes and socket mounting configurations.
func TestUnit_Config_Resolve_WithPrivilegedAndSocketSettings(t *testing.T) {
	t.Parallel()

	t.Run("resolve with privileged and mount socket options", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Privileged: ptr(true),
			CapAdd: []string{"SYS_ADMIN"},
			MountSocket: ptr(true),
			GroupAdd: []string{"1234"},
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.Privileged)
		assert.Contains(t, res.CapAdd, "SYS_ADMIN")
		assert.True(t, res.MountSocket)
		assert.Contains(t, res.GroupAdd, "1234")
	})
}

// TestUnit_Config_PidOption verifies resolution, priority, and security validation
// for the new '--pid' and '--cderun-pid' configuration options.
func TestUnit_Config_PidOption(t *testing.T) {
	t.Parallel()

	// 1. Valid and invalid security validations
	t.Run("pid validation rules", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}

		// Valid cases: empty or "host"
		cliValidEmpty := &CLIOptions{Image: ptr("alpine"), Pid: ptr("")}
		res, err := ResolveWithFS("sh", cliValidEmpty, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "", res.Pid)

		cliValidHost := &CLIOptions{Image: ptr("alpine"), Pid: ptr("host")}
		res, err = ResolveWithFS("sh", cliValidHost, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "host", res.Pid)

		// Invalid cases: anything else
		cliInvalid := &CLIOptions{Image: ptr("alpine"), Pid: ptr("invalid")}
		_, err = ResolveWithFS("sh", cliInvalid, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported pid namespace")

		cliInvalidChars := &CLIOptions{Image: ptr("alpine"), Pid: ptr("host\x00")}
		_, err = ResolveWithFS("sh", cliInvalidChars, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path")
	})

	// 2. Precedence (P1 override takes priority)
	t.Run("P1 takes absolute priority", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Env: map[string]string{
				"CDERUN_PID": "invalid", // would fail validation if used
			},
		}
		cli := &CLIOptions{
			Image:     ptr("alpine"),
			Pid:       ptr("invalid"),    // P2 would fail
			CderunPid: ptr("host"),       // P1 should win
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "host", res.Pid)
	})

	// 3. Precedence (P2 over P3)
	t.Run("P2 takes priority over P3", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Env: map[string]string{
				"CDERUN_PID": "invalid",
			},
		}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Pid:   ptr("host"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "host", res.Pid)
	})
}
