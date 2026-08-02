package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_ValidateWorkdir_Expansion expands testing for ValidateWorkdir.
// Reference: docs/features/security-validations.md (ValidateWorkdir)
func TestUnit_Config_ValidateWorkdir_Expansion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty working directory",
			path:    "",
			wantErr: false,
		},
		{
			name:    "standard valid working directory",
			path:    "/app/bin",
			wantErr: false,
		},
		{
			name:    "relative working directory",
			path:    "relative/path",
			wantErr: true,
			errMsg:  "working directory must be an absolute path",
		},
		{
			name:    "parent traversal in middle",
			path:    "/app/../escape",
			wantErr: true,
			errMsg:  "working directory cannot contain parent directory references",
		},
		{
			name:    "parent traversal at end",
			path:    "/app/..",
			wantErr: true,
			errMsg:  "working directory cannot contain parent directory references",
		},
		{
			name:    "invalid special character",
			path:    "/app$dir",
			wantErr: true,
			errMsg:  "invalid characters in working directory",
		},
		{
			name:    "invalid backslash",
			path:    "/app\\dir",
			wantErr: true,
			errMsg:  "invalid characters in working directory",
		},
		{
			name:    "valid nested path with dots and underscores",
			path:    "/app_1.2-beta/src",
			wantErr: false,
		},
		{
			name:    "root directory slash",
			path:    "/",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkdir(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestUnit_Config_ValidateExposePort_Expansion expands testing for ValidateExposePort.
// Reference: docs/features/command-line-options.md (Port parsing/validation)
func TestUnit_Config_ValidateExposePort_Expansion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		port    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty exposed port",
			port:    "",
			wantErr: false,
		},
		{
			name:    "standard tcp port",
			port:    "80/tcp",
			wantErr: false,
		},
		{
			name:    "standard udp port",
			port:    "53/udp",
			wantErr: false,
		},
		{
			name:    "no protocol port",
			port:    "8080",
			wantErr: false,
		},
		{
			name:    "valid range tcp",
			port:    "80-90/tcp",
			wantErr: false,
		},
		{
			name:    "invalid protocol",
			port:    "80/sctp",
			wantErr: true,
			errMsg:  "invalid protocol",
		},
		{
			name:    "reversed range",
			port:    "90-80/tcp",
			wantErr: true,
			errMsg:  "invalid port range: 90 > 80",
		},
		{
			name:    "start port too high",
			port:    "70000/tcp",
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name:    "end port too high",
			port:    "80-70000/tcp",
			wantErr: true,
			errMsg:  "invalid end port in range",
		},
		{
			name:    "negative port",
			port:    "-80",
			wantErr: true,
			errMsg:  "invalid start port in range: invalid port",
		},
		{
			name:    "invalid characters",
			port:    "80;rm",
			wantErr: true,
			errMsg:  "invalid port: invalid port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExposePort(tt.port)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestUnit_Config_MaskSensitiveEnvList_Expansion expands testing for environment variable masking.
// Reference: docs/features/sensitive-data-protection.md
func TestUnit_Config_MaskSensitiveEnvList_Expansion(t *testing.T) {
	t.Parallel()

	t.Run("nil patterns masks all environment values", func(t *testing.T) {
		input := []string{"DB_PASSWORD=secret123", "PORT=8080", "USER=jules"}
		output := MaskSensitiveEnvList(input, nil)
		expected := []string{"DB_PASSWORD=[REDACTED]", "PORT=[REDACTED]", "USER=[REDACTED]"}
		assert.Equal(t, expected, output)
	})

	t.Run("empty patterns slice masks nothing", func(t *testing.T) {
		input := []string{"DB_PASSWORD=secret123", "PORT=8080", "USER=jules"}
		output := MaskSensitiveEnvList(input, []string{})
		assert.Equal(t, input, output)
	})

	t.Run("case insensitive literal patterns and fast paths", func(t *testing.T) {
		input := []string{"DB_PASSWORD=secret123", "db_password=secret456", "PORT=8080"}
		patterns := []string{"Db_PaSsWoRd"}
		output := MaskSensitiveEnvList(input, patterns)
		expected := []string{"DB_PASSWORD=[REDACTED]", "db_password=[REDACTED]", "PORT=8080"}
		assert.Equal(t, expected, output)
	})

	t.Run("glob matching patterns with lazy uppercase initialization", func(t *testing.T) {
		input := []string{"AWS_ACCESS_KEY=123", "AWS_SECRET_KEY=abc", "USER=jules"}
		patterns := []string{"aws_*"}
		output := MaskSensitiveEnvList(input, patterns)
		expected := []string{"AWS_ACCESS_KEY=[REDACTED]", "AWS_SECRET_KEY=[REDACTED]", "USER=jules"}
		assert.Equal(t, expected, output)
	})

	t.Run("invalid glob pattern fails-closed", func(t *testing.T) {
		input := []string{"AWS_KEY=123", "USER=jules"}
		patterns := []string{"["} // invalid glob bracket
		output := MaskSensitiveEnvList(input, patterns)
		// Fails-closed: masks matching keys to be safe
		expected := []string{"AWS_KEY=[REDACTED]", "USER=[REDACTED]"}
		assert.Equal(t, expected, output)
	})

	t.Run("non-ASCII keys and values matched correctly", func(t *testing.T) {
		input := []string{"ユーザー_TOKEN=secret日本語", "USER=jules"}
		patterns := []string{"ユーザー_*"}
		output := MaskSensitiveEnvList(input, patterns)
		expected := []string{"ユーザー_TOKEN=[REDACTED]", "USER=jules"}
		assert.Equal(t, expected, output)
	})
}
