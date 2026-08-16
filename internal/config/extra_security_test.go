package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ValidateSecurity_EdgeCases(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{WD: "/work"}

	tests := []struct {
		name    string
		cli     *CLIOptions
		wantErr string
	}{
		{
			name: "socket-path with control char",
			cli: &CLIOptions{
				Image:      ptr("alpine"),
				SocketPath: ptr("/var/run/docker.sock\x01"),
			},
			wantErr: "security validation failed for \"socket-path\"",
		},
		{
			name: "mount-socket-path with control char",
			cli: &CLIOptions{
				Image:           ptr("alpine"),
				MountSocketPath: ptr("/var/run/docker.sock\x02"),
			},
			wantErr: "security validation failed for \"mount-socket-path\"",
		},
		{
			name: "mount-cderun-path with control char",
			cli: &CLIOptions{
				Image:           ptr("alpine"),
				MountCderunPath: ptr("/usr/local/bin/cderun\x03"),
			},
			wantErr: "security validation failed for \"mount-cderun-path\"",
		},
		{
			name: "invalid user format (too many colons)",
			cli: &CLIOptions{
				Image: ptr("alpine"),
				User:  ptr("user:group:extra"),
			},
			wantErr: "security validation failed for \"user\": invalid user format",
		},
		{
			name: "invalid user identifier (illegal char)",
			cli: &CLIOptions{
				Image: ptr("alpine"),
				User:  ptr("user!"),
			},
			wantErr: "security validation failed for \"user\": invalid user or group identifier",
		},
		{
			name: "unsupported runtime",
			cli: &CLIOptions{
				Image:  ptr("alpine"),
				Engine: ptr("rkt"),
			},
			wantErr: "security validation failed for \"engine\": unsupported engine: \"rkt\"",
		},
		{
			name: "unsupported log level",
			cli: &CLIOptions{
				Image:    ptr("alpine"),
				LogLevel: ptr("verbose"),
			},
			wantErr: "security validation failed for \"log-level\": unsupported log level: \"verbose\"",
		},
		{
			name: "unsupported log format",
			cli: &CLIOptions{
				Image:     ptr("alpine"),
				LogFormat: ptr("xml"),
			},
			wantErr: "security validation failed for \"log-format\": unsupported log format: \"xml\"",
		},
		{
			name: "unsupported dry-run format",
			cli: &CLIOptions{
				Image:        ptr("alpine"),
				DryRunFormat: ptr("csv"),
			},
			wantErr: "security validation failed for \"dry-run-format\": unsupported dry-run format: \"csv\"",
		},
		{
			name: "unsupported diagnosis format",
			cli: &CLIOptions{
				Image:           ptr("alpine"),
				Diagnosis:       ptr(true),
				DiagnosisFormat: ptr("pdf"),
			},
			wantErr: "security validation failed for \"diagnosis-format\": unsupported diagnosis format: \"pdf\"",
		},
		{
			name: "hostname too long",
			cli: &CLIOptions{
				Image:    ptr("alpine"),
				Hostname: ptr(strings.Repeat("a", 254)),
			},
			wantErr: "security validation failed for \"hostname\": hostname too long",
		},
		{
			name: "invalid hostname (starts with hyphen)",
			cli: &CLIOptions{
				Image:    ptr("alpine"),
				Hostname: ptr("-host"),
			},
			wantErr: "security validation failed for \"hostname\": invalid hostname",
		},
		{
			name: "invalid network name (illegal char)",
			cli: &CLIOptions{
				Image:   ptr("alpine"),
				Network: ptr("net!"),
			},
			wantErr: "security validation failed for \"network\": invalid network name",
		},
		{
			name: "invalid port mapping (illegal char)",
			cli: &CLIOptions{
				Image: ptr("alpine"),
				Ports: []string{"80:80/tcp!"},
			},
			wantErr: "security validation failed for publish[0]: invalid protocol",
		},
		{
			name: "invalid expose port (invalid protocol)",
			cli: &CLIOptions{
				Image:  ptr("alpine"),
				Expose: []string{"80/http"},
			},
			wantErr: "security validation failed for expose[0]: invalid protocol",
		},
		{
			name: "invalid expose port range (non-numeric)",
			cli: &CLIOptions{
				Image:  ptr("alpine"),
				Expose: []string{"80-abc"},
			},
			wantErr: "security validation failed for expose[0]: invalid end port in range",
		},
		{
			name: "invalid group-add with control char",
			cli: &CLIOptions{
				Image:    ptr("alpine"),
				GroupAdd: []string{"sudo\n"},
			},
			wantErr: "security validation failed for group-add[0]: invalid character in path or configuration",
		},
		{
			name: "invalid group-add with invalid identifier",
			cli: &CLIOptions{
				Image:    ptr("alpine"),
				GroupAdd: []string{"sudo;rm"},
			},
			wantErr: "security validation failed for group-add[0]: invalid supplementary group name or GID",
		},
		{
			name: "invalid pull policy",
			cli: &CLIOptions{
				Image: ptr("alpine"),
				Pull:  ptr("invalid-policy"),
			},
			wantErr: "security validation failed for \"pull\": invalid pull policy",
		},
		{
			name: "invalid device permissions with control char",
			cli: &CLIOptions{
				Image:   ptr("alpine"),
				Devices: []string{"/dev/sda:/dev/sda:rw\t"},
			},
			wantErr: "invalid device config: \"/dev/sda:/dev/sda:rw\\t\"",
		},
		{
			name: "invalid device permissions values",
			cli: &CLIOptions{
				Image:   ptr("alpine"),
				Devices: []string{"/dev/sda:/dev/sda:abc"},
			},
			wantErr: "invalid device config: \"/dev/sda:/dev/sda:abc\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ResolveWithFS("tool", tt.cli, nil, nil, fs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
