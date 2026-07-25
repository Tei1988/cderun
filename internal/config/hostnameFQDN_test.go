package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUnit_Config_FQDN_HostnameValidation verifies FQDN validation scenarios in ValidateHostname.
func TestUnit_Config_FQDN_HostnameValidation(t *testing.T) {
	t.Parallel()

	validCases := []string{
		"dev.local.host",
		"sub-domain.example.com",
		"a.b.c.d.e.f",
		"hostname-with-63-characters-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}

	for _, tc := range validCases {
		t.Run("valid FQDN: "+tc, func(t *testing.T) {
			err := ValidateHostname(tc)
			assert.NoError(t, err)
		})
	}

	invalidCases := []struct {
		name string
		host string
	}{
		{"double dots", "invalid..domain.com"},
		{"trailing hyphen in segment", "invalid.domain.com-"},
		{"leading hyphen in segment", "-invalid.domain.com"},
		{"label segment too long", "label-too-long-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
		{"invalid character underscore", "host_name.com"},
	}

	for _, tc := range invalidCases {
		t.Run("invalid FQDN: "+tc.name, func(t *testing.T) {
			err := ValidateHostname(tc.host)
			assert.Error(t, err)
		})
	}
}
