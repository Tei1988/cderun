package config

import (
	"bytes"
	"testing"

	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ValidateNetworkName_NSTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "Valid network namespace path",
			input:   "ns:/proc/1/ns/net",
			wantErr: "",
		},
		{
			name:    "Network namespace path with directory traversal",
			input:   "ns:/proc/1/ns/../../etc/passwd",
			wantErr: "contains parent directory references",
		},
		{
			name:    "Network namespace path relative (not abs)",
			input:   "ns:proc/1/ns/net",
			wantErr: "network namespace path must be an absolute path",
		},
		{
			name:    "Network namespace path with control character",
			input:   "ns:/proc/1/ns/net\n",
			wantErr: "invalid character in path or configuration",
		},
		{
			name:    "Container network reference valid",
			input:   "container:my-container",
			wantErr: "",
		},
		{
			name:    "Container network reference traversal attempt",
			input:   "container:../bad",
			wantErr: "contains parent directory references",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateNetworkName(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidatorEntrypoints_ControlCharSanitization(t *testing.T) {
	t.Parallel()

	controlCharInput := "val\x07id"
	invalidUTF8Input := "val\xffid"

	t.Run("ValidateHostname control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateHostname(controlCharInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("ValidateHostname invalid UTF-8", func(t *testing.T) {
		t.Parallel()
		err := ValidateHostname(invalidUTF8Input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid UTF-8 encoding")
	})

	t.Run("ValidateUserName control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateUserName(controlCharInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("ValidateWorkdir control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateWorkdir("/app\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("ValidateDNSOption control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateDNSOption(controlCharInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("ValidateSecurityOpt control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateSecurityOpt(controlCharInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("ValidateSysctlKey control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateSysctlKey(controlCharInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("ValidateCpuset control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateCpuset(controlCharInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("ValidateGPUs control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateGPUs(controlCharInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("ValidateGroupAdd control char", func(t *testing.T) {
		t.Parallel()
		err := ValidateGroupAdd(controlCharInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})
}

func TestUnit_Config_SecurityWarning_CDERunSocket(t *testing.T) {
	logger := logging.GetGlobalLogger()
	origLevel := logger.GetLevel()
	logger.SetLevel(logging.WarnLevel)
	var buf bytes.Buffer
	logging.SetOutput(&buf)
	t.Cleanup(func() {
		logger.SetLevel(origLevel)
		logging.SetOutput(nil)
	})

	fs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/work",
	}

	cli := &CLIOptions{
		Image:  ptr("alpine"),
		Mounts: []string{"source=/tmp/cderun.sock,target=/run/cderun/cderun.sock"},
	}

	_, err := ResolveWithFS("node", cli, nil, nil, fs)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Container socket mounting is enabled")
}
