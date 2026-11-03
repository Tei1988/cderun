package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_ValidateWorkdir_Scenarios tests boundary conditions for ValidateWorkdir.
// Reference: docs/features/security-validations.md (ValidateWorkdir)
func TestUnit_Config_ValidateWorkdir_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "empty path is valid",
			path:    "",
			wantErr: false,
		},
		{
			name:    "root path is valid",
			path:    "/",
			wantErr: false,
		},
		{
			name:    "simple absolute path is valid",
			path:    "/app",
			wantErr: false,
		},
		{
			name:    "nested workspace path is valid",
			path:    "/workspace/project_1/src",
			wantErr: false,
		},
		{
			name:    "scoped package directory with at sign and plus is valid",
			path:    "/workspace/@scope/pkg+dir",
			wantErr: false,
		},
		{
			name:    "pnpm virtual store path with version and at sign is valid",
			path:    "/node_modules/.pnpm/esbuild@0.25.12/node_modules/esbuild",
			wantErr: false,
		},
		{
			name:    "directory with multiple plus signs is valid",
			path:    "/app/build+test+artifacts",
			wantErr: false,
		},
		{
			name:    "directory with underscores and hyphens is valid",
			path:    "/var/log_app-service/2026_09",
			wantErr: false,
		},
		{
			name:        "relative path without slash rejected",
			path:        "app/workspace",
			wantErr:     true,
			errContains: "working directory must be an absolute path",
		},
		{
			name:        "relative path with dot prefix rejected",
			path:        "./workspace",
			wantErr:     true,
			errContains: "working directory must be an absolute path",
		},
		{
			name:        "parent directory traversal at start rejected",
			path:        "../workspace",
			wantErr:     true,
			errContains: "working directory must be an absolute path",
		},
		{
			name:        "parent directory traversal in middle rejected",
			path:        "/workspace/../etc",
			wantErr:     true,
			errContains: "working directory cannot contain parent directory references",
		},
		{
			name:        "parent directory traversal at end rejected",
			path:        "/app/..",
			wantErr:     true,
			errContains: "working directory cannot contain parent directory references",
		},
		{
			name:        "parent directory traversal after root rejected",
			path:        "/../app",
			wantErr:     true,
			errContains: "working directory cannot contain parent directory references",
		},
		{
			name:        "double slash in path is valid character-wise and absolute",
			path:        "/app//src",
			wantErr:     false,
		},
		{
			name:        "path with exclamation mark rejected",
			path:        "/app!dir",
			wantErr:     true,
			errContains: "invalid characters in working directory",
		},
		{
			name:        "path with dollar sign rejected",
			path:        "/app$dir",
			wantErr:     true,
			errContains: "invalid characters in working directory",
		},
		{
			name:        "path with space rejected",
			path:        "/app dir",
			wantErr:     true,
			errContains: "invalid characters in working directory",
		},
		{
			name:        "path with colon rejected",
			path:        "/app:dir",
			wantErr:     true,
			errContains: "invalid characters in working directory",
		},
		{
			name:        "path with backslash rejected",
			path:        "/app\\dir",
			wantErr:     true,
			errContains: "invalid characters in working directory",
		},
		{
			name:        "path with semicolon injection rejected",
			path:        "/app; rm -rf /",
			wantErr:     true,
			errContains: "invalid characters in working directory",
		},
		{
			name:        "path with null byte rejected by validatePathChars",
			path:        "/app\x00null",
			wantErr:     true,
			errContains: "invalid character in path or configuration",
		},
		{
			name:        "path with newline rejected by validatePathChars",
			path:        "/app\nnewline",
			wantErr:     true,
			errContains: "invalid character in path or configuration",
		},
		{
			name:        "path with carriage return rejected by validatePathChars",
			path:        "/app\rreturn",
			wantErr:     true,
			errContains: "invalid character in path or configuration",
		},
		{
			name:        "path with tab control character rejected by validatePathChars",
			path:        "/app\ttab",
			wantErr:     true,
			errContains: "invalid character in path or configuration",
		},
		{
			name:        "path with arbitrary control character rejected by validatePathChars",
			path:        "/app/\x03dir",
			wantErr:     true,
			errContains: "invalid character in path or configuration",
		},
		{
			name:        "path with invalid UTF-8 sequence rejected by validatePathChars",
			path:        "/app/\xff\xfe",
			wantErr:     true,
			errContains: "invalid character in path or configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateWorkdir(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func BenchmarkValidateWorkdir_ValidPath(b *testing.B) {
	workdir := "/node_modules/.pnpm/esbuild@0.25.12/node_modules/esbuild"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := ValidateWorkdir(workdir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateWorkdir_InvalidChars(b *testing.B) {
	workdir := "/app/invalid$character"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ValidateWorkdir(workdir)
	}
}
