package command

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_ResourceLimits_DryRunSimple(t *testing.T) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{
		"--dry-run",
		"--dry-run-format", "simple",
		"--image", "alpine",
		"--cgroupns", "host",
		"--pids-limit", "100",
		"--cpu-shares", "512",
		"--cpuset-cpus", "0,1",
		"--cpuset-mems", "0",
		"--gpus", "all",
		"sh",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Cgroupns: host")
	assert.Contains(t, output, "PidsLimit: 100")
	assert.Contains(t, output, "CPUShares: 512")
	assert.Contains(t, output, "CpusetCpus: 0,1")
	assert.Contains(t, output, "CpusetMems: 0")
	assert.Contains(t, output, "GPUs: all")
}
