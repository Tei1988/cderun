package config

import (
	"bytes"
	"testing"

	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_DoubleBraceEscaping_Refinement verifies double-brace escaping.
// Reference: docs/features/value-resolution.md (Double-Brace Escaping Syntax)
func TestUnit_Config_DoubleBraceEscaping_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"HOME": "/home/user",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escape HOME magic word",
			input:    "{{ {{HOME}} }}",
			expected: "{{HOME}}",
		},
		{
			name:     "escape file directive",
			input:    "{{ {{file:config}} }}",
			expected: "{{file:config}}",
		},
		{
			name:     "escape unrecognized key",
			input:    "{{ {{unknown}} }}",
			expected: "{{unknown}}",
		},
		{
			name:     "escape with extra inner spacing",
			input:    "{{  {{unknown_with_spacing}}  }}",
			expected: "{{unknown_with_spacing}}",
		},
		{
			name:     "nested escaped structures",
			input:    "{{ {{ {{HOME}} }} }}",
			expected: "{{ {{HOME}} }}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := r.resolveString(tt.input)
			require.NoError(t, r.Error())
			assert.Equal(t, tt.expected, res)
		})
	}
}

// TestUnit_Config_MultipleAnchors_BoundaryChecks_Refinement tests that a path containing
// multiple anchors must satisfy boundary checks for all present anchors.
// Reference: docs/features/value-resolution.md (Multiple Anchors Evaluation)
func TestUnit_Config_MultipleAnchors_BoundaryChecks_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"HOME": "/home/user",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// In this test case, {{HOME}} is /home/user and {{PWD}} is /work.
	// The path string is: {{HOME}}/{{PWD}}/file
	// Which evaluates to: /home/user/work/file
	// This path does not escape /home/user boundary, but escapes /work boundary.
	// Therefore, it must be rejected on PWD boundary check.
	t.Run("escaped multi-anchor path resolution rejection", func(t *testing.T) {
		_, err := ResolvePath("{{HOME}}/{{PWD}}/file", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes anchor boundary")
	})

	// Valid scenario: path within both boundaries if possible (for example, if resolved path lies inside both, which is rare,
	// or simply a path resolving to a sub-folder of both if they are configured hierarchically)
	t.Run("nested hierachical multi-anchor path resolution success", func(t *testing.T) {
		mfsHierarchical := &MockFileSystem{
			WD:      "/home/user/work",
			HomeDir: "/home/user",
			Env: map[string]string{
				"HOME": "/home/user",
			},
		}
		rHierarchical, err := NewExpressionResolverWithFS(nil, mfsHierarchical)
		require.NoError(t, err)

		// Resolved: /home/user/work/subdir/file, which lies inside both /home/user and /home/user/work
		res, err := ResolvePath("{{PWD}}/subdir/file", "/home/user/work", rHierarchical)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/work/subdir/file", res)
	})
}

// TestUnit_Config_validateEnvSecurity_Refinement verifies security checks on environment keys and values.
// Reference: docs/features/value-resolution.md (Null-Byte Injections Guard)
func TestUnit_Config_validateEnvSecurity_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/work"}

	tests := []struct {
		name    string
		env     []string
		wantErr string
	}{
		{
			name:    "invalid character in env key",
			env:     []string{"KEY\x01=val"},
			wantErr: "security validation failed for env[0] (key)",
		},
		{
			name:    "invalid env key start char",
			env:     []string{"-KEY=val"},
			wantErr: "security validation failed for env[0] (key)",
		},
		{
			name:    "null byte in env value",
			env:     []string{"KEY=val\x00injection"},
			wantErr: "security validation failed for env[0] (value): null byte injection detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &CLIOptions{
				Image: ptr("alpine"),
				Env:   tt.env,
			}
			_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestUnit_Config_validateContainerPath_Refinement verifies validations on container targets.
// Reference: docs/features/value-resolution.md (Container Target Path Safety)
func TestUnit_Config_validateContainerPath_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/work"}

	t.Run("empty container mount target path", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"type=bind,source=/src,target="},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target is required")
	})

	t.Run("relative container mount target path", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"type=bind,source=/src,target=app/bin"},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target must be an absolute path")
	})

	t.Run("parent traversal container mount target path", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"type=bind,source=/src,target=/app/../bin"},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")
	})
}

// TestUnit_Config_isHighlySensitiveDevice_And_isHighlyPrivilegedCapability verifies logging warn level emissions.
// Reference: docs/features/logging-debugging.md
func TestUnit_Config_isHighlySensitiveDevice_And_isHighlyPrivilegedCapability(t *testing.T) {
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	t.Run("warning emission for highly sensitive device and highly privileged cap", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Devices:  []string{"/dev/mem:/dev/mem"},
			CapAdd:   []string{"SYS_ADMIN"},
			GroupAdd: []string{"1234"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		// Highly sensitive device warning
		assert.Contains(t, logOutput, "Mounting highly sensitive host device")
		assert.Contains(t, logOutput, "/dev/mem")
		// Highly privileged capability warning
		assert.Contains(t, logOutput, "Highly privileged capability")
		assert.Contains(t, logOutput, "SYS_ADMIN")
	})
}
