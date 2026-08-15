package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_SecurityHardening_NetworkName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid standard bridge", "bridge", false},
		{"valid custom network", "my-custom_net.1", false},
		{"valid container reference", "container:my_app_container", false},
		{"invalid empty container reference", "container:", true},
		{"invalid character in container reference", "container:my_app;bad", true},
		{"valid ns path", "ns:/proc/1/ns/net", false},
		{"invalid empty ns path", "ns:", true},
		{"invalid relative ns path", "ns:proc/1/ns/net", true},
		{"invalid ns path traversal", "ns:/proc/../etc/passwd", true},
		{"invalid parent traversal in network", "../etc/passwd", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateNetworkName(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_SecurityHardening_PIDNamespace(t *testing.T) {
	t.Parallel()

	r := &resolver{
		fs:  RealFileSystem{},
		cli: &CLIOptions{},
		res: &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
		},
	}

	// Empty and host are valid
	r.res.Pid = ""
	assert.NoError(t, r.validateCriticalFields())

	r.res.Pid = "host"
	assert.NoError(t, r.validateCriticalFields())

	// Valid container PID namespace
	r.res.Pid = "container:target_container_123"
	assert.NoError(t, r.validateCriticalFields())

	// Invalid empty container reference
	r.res.Pid = "container:"
	err := r.validateCriticalFields()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty container reference")

	// Invalid character in container PID reference
	r.res.Pid = "container:target;injection"
	err = r.validateCriticalFields()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character in container pid reference")

	// Unsupported PID namespace string
	r.res.Pid = "invalid_mode"
	err = r.validateCriticalFields()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported pid namespace")
}

func TestUnit_Config_SecurityHardening_ParentTraversal(t *testing.T) {
	t.Parallel()

	t.Run("ValidateImageName with traversal", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateImageName("alpine:latest"))
		assert.NoError(t, ValidateImageName("myregistry.com/org/repo:1.0"))
		err := ValidateImageName("myregistry.com/../malicious/repo:1.0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contains parent directory references")
	})

	t.Run("ValidateSecurityOpt with traversal", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("apparmor=/etc/apparmor.d/profile"))
		err := ValidateSecurityOpt("apparmor=/etc/../passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contains parent directory references")
	})

	t.Run("ValidateDNSOption with traversal", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, ValidateDNSOption("ndots:2"))
		err := ValidateDNSOption("ndots:2/..")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contains parent directory references")
	})
}
