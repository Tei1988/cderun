package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_DNSOptions_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_DNS_SEARCH": "example.com,mycompany.com",
			"CDERUN_DNS_OPTION": "ndots:5,timeout:2",
		},
	}

	cli := &CLIOptions{
		Image: ptr("alpine"),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com", "mycompany.com"}, res.DNSSearch)
	assert.Equal(t, []string{"ndots:5", "timeout:2"}, res.DNSOptions)
}
