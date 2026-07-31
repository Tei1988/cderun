package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_jules_ContainsNumericGID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected bool
	}{
		{"nil slice", nil, false},
		{"empty slice", []string{}, false},
		{"purely non-numeric strings", []string{"sudo", "docker"}, false},
		{"purely numeric GID", []string{"1000"}, true},
		{"zero GID", []string{"0"}, true},
		{"partially numeric strings", []string{"1000a", "sudo"}, false},
		{"mixed slice with numeric GID", []string{"docker", "1001", "sudo"}, true},
		{"mixed slice without numeric GID", []string{"docker", "sudo123"}, false},
		{"empty string inside slice", []string{"", "docker"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ContainsNumericGID(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestUnit_Config_jules_ValidateExposePort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"standard single port", "80", false},
		{"port with protocol", "80/tcp", false},
		{"port with udp protocol", "53/udp", false},
		{"port range", "80-90", false},
		{"port range with protocol", "80-90/tcp", false},
		{"maximum single port", "65535", false},
		{"maximum port range with protocol", "1-65535/udp", false},
		{"empty exposed port", "", false},

		{"port zero", "0", true},
		{"out of bounds port", "70000", true},
		{"negative port", "-1", true},
		{"invalid protocol", "80/http", true},
		{"port range reversed start > end", "90-80", true},
		{"port range invalid start", "abc-80", true},
		{"port range invalid end", "80-abc", true},
		{"control character injection", "80\n", true},
		{"shell command injection", "80;rm", true},
		{"multiple protocols", "80/tcp/udp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExposePort(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "input: %q", tt.input)
			} else {
				assert.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_jules_ValidateNetworkName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"standard bridge network", "bridge", false},
		{"network with hyphen", "my-net", false},
		{"network with dot", "my.net", false},
		{"network with underscore", "my_net", false},
		{"network with numbers", "net123", false},
		{"empty network name", "", false},

		{"starts with hyphen", "-net", true},
		{"starts with dot", ".net", true},
		{"starts with underscore", "_net", true},
		{"contains semicolon", "my_net;", true},
		{"contains control char newline", "net\n", true},
		{"contains control char tab", "net\t", true},
		{"contains space", "my net", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetworkName(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "input: %q", tt.input)
			} else {
				assert.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_jules_ValidateUserName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple user", "root", false},
		{"numeric UID", "1000", false},
		{"user and group", "root:root", false},
		{"UID and GID", "1000:1000", false},
		{"user and GID", "root:1000", false},
		{"NIS Samba compatible user", "domain-users$", false},
		{"NIS Samba user and group", "domain-users$:domain-users$", false},
		{"empty user name", "", false},

		{"too many colons", "root:root:root", true},
		{"invalid user starting character", "-root", true},
		{"contains spaces", "root name", true},
		{"contains semicolon shell char", "root;rm", true},
		{"contains newline", "root\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserName(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "input: %q", tt.input)
			} else {
				assert.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_jules_TildeExpansion_Error(t *testing.T) {
	t.Parallel()

	t.Run("home dir error triggers path resolution failure", func(t *testing.T) {
		mfs := &exprMockFS{
			homeDirErr: errors.New("cannot fetch home directory"),
			MockFileSystem: MockFileSystem{
				WD: "/work",
			},
		}

		_, err := NewExpressionResolverWithFS(nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get user home directory")
		assert.Contains(t, err.Error(), "cannot fetch home directory")
	})
}

func TestUnit_Config_jules_RecursiveAndNestedExpressions(t *testing.T) {
	t.Parallel()

	t.Run("deep triple nested dynamic evaluation fallback sequence", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Env: map[string]string{
				"VAR_A": "found_a",
				"VAR_B": "",
			},
		}

		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Triple nesting default values:
		// {{env:VAR_C:-{{env:VAR_B:-{{env:VAR_A:-default}}}}}}
		// VAR_C is unset -> goes to VAR_B
		// VAR_B is empty -> goes to VAR_A
		// VAR_A is "found_a" -> resolves to "found_a"
		val := r.resolveString("{{env:VAR_C:-{{env:VAR_B:-{{env:VAR_A:-default}}}}}}")
		require.NoError(t, r.Error())
		assert.Equal(t, "found_a", val)
	})

	t.Run("deep triple nested fallback to final literal default", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Env: map[string]string{
				"VAR_A": "",
				"VAR_B": "",
			},
		}

		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Triple nesting default values ending in final default:
		// {{env:VAR_C:-{{env:VAR_B:-{{env:VAR_A:-final_fallback}}}}}}
		val := r.resolveString("{{env:VAR_C:-{{env:VAR_B:-{{env:VAR_A:-final_fallback}}}}}}")
		require.NoError(t, r.Error())
		assert.Equal(t, "final_fallback", val)
	})
}
