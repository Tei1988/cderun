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
				Image: Ptr("alpine"),
				SocketPath: Ptr("/var/run/docker.sock\x01")},
			wantErr: "security validation failed for \"socket-path\""},
		{
			name: "mount-socket-path with control char",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				MountSocketPath: Ptr("/var/run/docker.sock\x02")},
			wantErr: "security validation failed for \"mount-socket-path\""},
		{
			name: "mount-cderun-path with control char",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				MountCderunPath: Ptr("/usr/local/bin/cderun\x03")},
			wantErr: "security validation failed for \"mount-cderun-path\""},
		{
			name: "invalid user format (too many colons)",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				User: Ptr("user:group:extra")},
			wantErr: "security validation failed for \"user\": invalid user format"},
		{
			name: "invalid user identifier (illegal char)",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				User: Ptr("user!")},
			wantErr: "security validation failed for \"user\": invalid user or group identifier"},
		{
			name: "unsupported runtime",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				Runtime: Ptr("rkt")},
			wantErr: "security validation failed for \"runtime\": unsupported runtime: \"rkt\""},
		{
			name: "unsupported log level",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				LogLevel: Ptr("verbose")},
			wantErr: "security validation failed for \"log-level\": unsupported log level: \"verbose\""},
		{
			name: "unsupported log format",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				LogFormat: Ptr("xml")},
			wantErr: "security validation failed for \"log-format\": unsupported log format: \"xml\""},
		{
			name: "unsupported dry-run format",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				DryRunFormat: Ptr("csv")},
			wantErr: "security validation failed for \"dry-run-format\": unsupported dry-run format: \"csv\""},
		{
			name: "unsupported diagnosis format",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				Diagnosis: Ptr(true),
				DiagnosisFormat: Ptr("pdf")},
			wantErr: "security validation failed for \"diagnosis-format\": unsupported diagnosis format: \"pdf\""},
		{
			name: "hostname too long",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				Hostname:    Ptr(strings.Repeat("a", 254))},
			wantErr: "security validation failed for \"hostname\": hostname too long"},
		{
			name: "invalid hostname (starts with hyphen)",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				Hostname: Ptr("-host")},
			wantErr: "security validation failed for \"hostname\": invalid hostname"},
		{
			name: "invalid network name (illegal char)",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				Network: Ptr("net!")},
			wantErr: "security validation failed for \"network\": invalid network name"},
		{
			name: "invalid port mapping (illegal char)",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				Ports:    []string{"80:80/tcp!"}},
			wantErr: "security validation failed for ports[0]: invalid protocol"},
		{
			name: "invalid expose port (invalid protocol)",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				Expose:   []string{"80/http"}},
			wantErr: "security validation failed for expose[0]: invalid protocol"},
		{
			name: "invalid expose port range (non-numeric)",
			cli: &CLIOptions{
				Image: Ptr("alpine"),
				Expose:   []string{"80-abc"}},
			wantErr: "security validation failed for expose[0]: invalid port range"}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ResolveWithFS("tool", tt.cli, nil, nil, fs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
