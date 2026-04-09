package config

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ValidatePortBinding_Internal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Single port", "80", false},
		{"Host:Container port", "8080:80", false},
		{"IP:Host:Container port", "127.0.0.1:8080:80", false},
		{"Port/Proto", "80/tcp", false},
		{"IP:Host:Container port/Proto", "127.0.0.1:8080:80/udp", false},
		{"IPv6", "[::1]:8080:80", false},
		{"IPv6 with proto", "[::1]:8080:80/tcp", false},
		{"No host port (IP::Container)", "127.0.0.1::80", false},
		{"Implicit host port (:80)", ":80", false},
		{"Invalid proto", "80/http", true},
		{"Too many segments", "1:2:3:4", true},
		{"Non-numeric port", "abc", true},
		{"Port out of range", "70000", true},
		{"Port zero", "0", true},
		{"Invalid IP", "999.999.999.999:80", true},
		{"Space", "8080 :80", true},
		{"Empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePortBinding(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateExpose_Internal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Single port", "80", false},
		{"Port range", "80-81", false},
		{"Port/Proto", "80/tcp", false},
		{"Range/Proto", "80-81/udp", false},
		{"Invalid range", "80-70", true},
		{"Port out of range", "70000", true},
		{"Invalid proto", "80/sctp", false}, // sctp is allowed
		{"Invalid proto 2", "80/xyz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExpose(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}
