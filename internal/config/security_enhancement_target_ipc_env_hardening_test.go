package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_SecurityEnhancement_TargetIpcEnvHardening_MountAndDeviceExpandedParentTraversal(t *testing.T) {
	mfs := &MockFileSystem{
		WD:  "/tmp",
		Env: map[string]string{"TRAVERSAL_PART": ".."},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// 1. MountConfig.Resolve with expanded parent traversal in target
	mc := MountConfig{
		Type:   "bind",
		Source: ConfigPath{Raw: "/tmp/host/src"},
		Target: ConfigPath{Raw: "/container/foo/{{env:TRAVERSAL_PART}}"},
	}

	_, err = mc.Resolve(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")

	// 2. DeviceConfig.Resolve with expanded parent traversal in source and destination
	dcTarget := DeviceConfig{
		Source:      ConfigPath{Raw: "/dev/null"},
		Destination: ConfigPath{Raw: "/dev/foo/{{env:TRAVERSAL_PART}}"},
		Permissions: "rwm",
	}

	_, err = dcTarget.Resolve(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "device destination cannot contain parent directory references")

	dcSource := DeviceConfig{
		Source:      ConfigPath{Raw: "/dev/foo/{{env:TRAVERSAL_PART}}"},
		Destination: ConfigPath{Raw: "/dev/null"},
		Permissions: "rwm",
	}

	_, err = dcSource.Resolve(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "device source cannot contain parent directory references")
}
