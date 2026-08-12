package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_SecurityOpt_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_SECURITY_OPT": "no-new-privileges,seccomp=unconfined",
		},
	}

	cli := &CLIOptions{
		Image: ptr("alpine"),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, []string{"no-new-privileges", "seccomp=unconfined"}, res.SecurityOpt)
}
