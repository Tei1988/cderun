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
				Image:         "alpine",
				ImageSet:      true,
				SocketPath:    "/var/run/docker.sock\x01",
				SocketPathSet: true,
			},
			wantErr: "security validation failed for \"socket-path\"",
		},
		{
			name: "mount-socket-path with control char",
			cli: &CLIOptions{
				Image:              "alpine",
				ImageSet:           true,
				MountSocketPath:    "/var/run/docker.sock\x02",
				MountSocketPathSet: true,
			},
			wantErr: "security validation failed for \"mount-socket-path\"",
		},
		{
			name: "mount-cderun-path with control char",
			cli: &CLIOptions{
				Image:              "alpine",
				ImageSet:           true,
				MountCderunPath:    "/usr/local/bin/cderun\x03",
				MountCderunPathSet: true,
			},
			wantErr: "security validation failed for \"mount-cderun-path\"",
		},
		{
			name: "invalid user format (too many colons)",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				User:     "user:group:extra",
				UserSet:  true,
			},
			wantErr: "security validation failed for \"user\": invalid user format",
		},
		{
			name: "invalid user identifier (illegal char)",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				User:     "user!",
				UserSet:  true,
			},
			wantErr: "security validation failed for \"user\": invalid user or group identifier",
		},
		{
			name: "unsupported runtime",
			cli: &CLIOptions{
				Image:      "alpine",
				ImageSet:   true,
				Runtime:    "rkt",
				RuntimeSet: true,
			},
			wantErr: "security validation failed for \"runtime\": unsupported runtime: \"rkt\"",
		},
		{
			name: "unsupported log level",
			cli: &CLIOptions{
				Image:       "alpine",
				ImageSet:    true,
				LogLevel:    "verbose",
				LogLevelSet: true,
			},
			wantErr: "security validation failed for \"log-level\": unsupported log level: \"verbose\"",
		},
		{
			name: "unsupported log format",
			cli: &CLIOptions{
				Image:        "alpine",
				ImageSet:     true,
				LogFormat:    "xml",
				LogFormatSet: true,
			},
			wantErr: "security validation failed for \"log-format\": unsupported log format: \"xml\"",
		},
		{
			name: "unsupported dry-run format",
			cli: &CLIOptions{
				Image:           "alpine",
				ImageSet:        true,
				DryRunFormat:    "csv",
				DryRunFormatSet: true,
			},
			wantErr: "security validation failed for \"dry-run-format\": unsupported dry-run format: \"csv\"",
		},
		{
			name: "unsupported diagnosis format",
			cli: &CLIOptions{
				Image:              "alpine",
				ImageSet:           true,
				Diagnosis:          true,
				DiagnosisSet:       true,
				DiagnosisFormat:    "pdf",
				DiagnosisFormatSet: true,
			},
			wantErr: "security validation failed for \"diagnosis-format\": unsupported diagnosis format: \"pdf\"",
		},
		{
			name: "hostname too long",
			cli: &CLIOptions{
				Image:       "alpine",
				ImageSet:    true,
				Hostname:    strings.Repeat("a", 254),
				HostnameSet: true,
			},
			wantErr: "security validation failed for \"hostname\": hostname too long",
		},
		{
			name: "invalid hostname (starts with hyphen)",
			cli: &CLIOptions{
				Image:       "alpine",
				ImageSet:    true,
				Hostname:    "-host",
				HostnameSet: true,
			},
			wantErr: "security validation failed for \"hostname\": invalid hostname",
		},
		{
			name: "invalid network name (illegal char)",
			cli: &CLIOptions{
				Image:      "alpine",
				ImageSet:   true,
				Network:    "net!",
				NetworkSet: true,
			},
			wantErr: "security validation failed for \"network\": invalid network name",
		},
		{
			name: "invalid port mapping (illegal char)",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				Ports:    []string{"80:80/tcp!"},
			},
			wantErr: "security validation failed for ports[0]: invalid protocol",
		},
		{
			name: "invalid expose port (invalid protocol)",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				Expose:   []string{"80/http"},
			},
			wantErr: "security validation failed for expose[0]: invalid protocol",
		},
		{
			name: "invalid expose port range (non-numeric)",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				Expose:   []string{"80-abc"},
			},
			wantErr: "security validation failed for expose[0]: invalid end port in range",
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
