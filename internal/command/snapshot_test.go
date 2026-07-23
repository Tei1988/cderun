package command

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
)

func TestUnit_Snapshot_PathResolutionInNestedExecution(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}

	// Simulate OverlayFS: container / maps to host /var/lib/docker/overlay2/abc/diff
	mountinfo := "24 25 0:21 / / rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/var/lib/docker/overlay2/abc/diff,workdir=/w\n"
	reader := &mockMountInfoReader{Content: []byte(mountinfo)}

	globalCfg := &config.CDERunConfig{
		HostContext: &config.HostContext{
			Level: 1,
		},
	}

	containerDir, hostDir, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, config.ToolsConfig{}, nil, reader)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	// hostDir should be resolved via OverlayFS upperdir, not container-local /tmp
	assert.True(t, strings.HasPrefix(hostDir, "/var/lib/docker/overlay2/abc/diff/"), "expected host path via OverlayFS, got: %s", hostDir)
}

func TestUnit_Snapshot_ConfigurationImmutability(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{
		Runtime: "docker",
		HostContext: &config.HostContext{
			Level: 1,
			Mounts: []config.MountMapping{
				{Source: "/h1", Target: "/c1", Level: 1},
			},
		},
	}
	// Save initial state for comparison
	initialLevel := globalCfg.HostContext.Level
	initialMountsCount := len(globalCfg.HostContext.Mounts)
	initialMountSource := globalCfg.HostContext.Mounts[0].Source

	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{
		{Type: "bind", Source: "/h2", Target: "/c2"},
	}

	containerDir, _, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts, nil)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	// Verify that globalCfg was NOT mutated
	assert.Equal(t, initialLevel, globalCfg.HostContext.Level)
	assert.Len(t, globalCfg.HostContext.Mounts, initialMountsCount)
	assert.Equal(t, initialMountSource, globalCfg.HostContext.Mounts[0].Source)
}

func TestUnit_Snapshot_InitializationWithNilHostContext(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{
		Runtime: "docker",
	}
	assert.Nil(t, globalCfg.HostContext)

	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	containerDir, _, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts, nil)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	// Verify that globalCfg.HostContext is still nil
	assert.Nil(t, globalCfg.HostContext)
}

func TestUnit_Snapshot_DirectoryAndFilePermissions(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{}
	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	containerDir, _, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts, nil)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	// Verify snapshot directory permissions (0700)
	assert.Equal(t, os.FileMode(0o700), mfs.Perms[containerDir])

	// Verify configuration file permissions (0600)
	assert.Equal(t, os.FileMode(0o600), mfs.Perms[filepath.Join(containerDir, ".cderun.yaml")])
	assert.Equal(t, os.FileMode(0o600), mfs.Perms[filepath.Join(containerDir, ".tools.yaml")])
}

type mockMountInfoReader struct {
	Content []byte
	Err     error
}

func (m *mockMountInfoReader) ReadMountInfo(fs config.FileSystem) ([]byte, error) {
	return m.Content, m.Err
}

func TestUnit_Snapshot_OverlayFSDiscovery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		mountinfo string
		expected  string
	}{
		{
			name:      "successfully discover upperdir",
			mountinfo: "24 25 0:21 / / rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/u,workdir=/w\n",
			expected:  "/u",
		},
		{
			name:      "no overlay found",
			mountinfo: "24 25 0:21 / / rw,relatime - ext4 /dev/sda1 rw\n",
			expected:  "",
		},
		{
			name:      "malformed mountinfo",
			mountinfo: "too few fields\n",
			expected:  "",
		},
		{
			name:      "no separator",
			mountinfo: "24 25 0:21 / / rw,relatime overlay overlay rw,upperdir=/u\n",
			expected:  "",
		},
		{
			name:      "non-root overlay",
			mountinfo: "24 25 0:21 / /mnt/foo rw,relatime - overlay overlay rw,upperdir=/u\n",
			expected:  "",
		},
		{
			name:      "few fields after separator",
			mountinfo: "24 25 0:21 / / rw,relatime - overlay\n",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := &config.MockFileSystem{}
			reader := &mockMountInfoReader{Content: []byte(tt.mountinfo)}
			upperdir, err := discoverOverlayUpperDir(mfs, reader)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, upperdir)
		})
	}
}

type errorFS struct {
	wfFunc func(path string, data []byte, perm os.FileMode) error
	*config.MockFileSystem
	mkdirErr error
	writeErr error
}

func (f *errorFS) MkdirAll(path string, perm os.FileMode) error {
	if f.mkdirErr != nil {
		return f.mkdirErr
	}
	return f.MockFileSystem.MkdirAll(path, perm)
}

func (f *errorFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	if f.wfFunc != nil {
		return f.wfFunc(path, data, perm)
	}
	if f.writeErr != nil {
		return f.writeErr
	}
	return f.MockFileSystem.WriteFile(path, data, perm)

}

func TestUnit_Snapshot_Errors(t *testing.T) {
	t.Parallel()
	t.Run("MkdirAll fails", func(t *testing.T) {
		mfs := &errorFS{
			MockFileSystem: &config.MockFileSystem{},
			mkdirErr:       os.ErrPermission,
		}
		_, _, err := createSnapshot(logging.NewLogger(), mfs, &config.CDERunConfig{}, nil, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create snapshot directory")
	})

	t.Run("WriteFile fails for global config", func(t *testing.T) {
		mfs := &errorFS{
			MockFileSystem: &config.MockFileSystem{},
			writeErr:       os.ErrPermission,
		}
		_, _, err := createSnapshot(logging.NewLogger(), mfs, &config.CDERunConfig{}, nil, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write .cderun.yaml to snapshot")
	})
}

func TestUnit_Snapshot_Cleanup_Errors(t *testing.T) {
	sentinel := errors.New("cleanup failed")
	mfs := &config.MockFileSystem{RemoveAllErr: sentinel}
	err := cleanupSnapshot(mfs, "/tmp/snapshot")
	require.ErrorIs(t, err, sentinel)
}

func TestUnit_Snapshot_WriteFile_ToolsConfig_Failure(t *testing.T) {
	t.Parallel()
	mfs := &errorFS{
		MockFileSystem: &config.MockFileSystem{},
	}
	mfs.wfFunc = func(path string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(path, ".tools.yaml") {
			return os.ErrPermission
		}
		return mfs.MockFileSystem.WriteFile(path, data, perm)
	}

	_, _, err := createSnapshot(logging.NewLogger(), mfs, &config.CDERunConfig{}, config.ToolsConfig{"node": {}}, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write .tools.yaml to snapshot")
}

func TestUnit_Snapshot_Nested_ResolutionFailures(t *testing.T) {
	t.Parallel()
	t.Run("ResolvePath failure when Level > 1", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			// ResolvePath calls r.ResolveString which uses mfs.Getenv for {{env:...}}
			// If we use {{file:nonexistent}}, it should fail.
			TempDirValue: "/tmp/{{file:nonexistent}}",
		}
		globalCfg := &config.CDERunConfig{
			HostContext: &config.HostContext{
				Level: 1, // Will be incremented to 2
			},
		}
		_, _, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, nil, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve snapshot directory")
	})

	// TODO: clarify why ResolvePath returns empty string and whether this subtest should be skipped or re-enabled.
	// This case is difficult to trigger with the current filepath.Join logic in createSnapshot.
}

func TestUnit_Snapshot_ReadMountInfo_Error(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}
	sentinel := errors.New("read mountinfo failed")
	reader := &mockMountInfoReader{Err: sentinel}

	upperdir, err := discoverOverlayUpperDir(mfs, reader)
	require.ErrorIs(t, err, sentinel)
	assert.Empty(t, upperdir)
}

func TestUnit_Snapshot_Log_Failures(t *testing.T) {
	t.Parallel()
	mfs := &snapshotMockFS{
		MockFileSystem: &config.MockFileSystem{
			ExecErr: errors.New("exec error"),
		},
		wdErr:   errors.New("wd error"),
		homeErr: errors.New("home error"),
	}
	var logBuf bytes.Buffer
	logger := logging.NewLogger()
	logger.Init("debug", "text", false)
	logger.SetOutput(&logBuf)

	containerDir, _, err := createSnapshot(logger, mfs, &config.CDERunConfig{}, nil, nil, nil)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	assert.Contains(t, logBuf.String(), "failed to get executable path for snapshot: exec error")
	assert.Contains(t, logBuf.String(), "failed to get working directory for snapshot: wd error")
	assert.Contains(t, logBuf.String(), "failed to get home directory for snapshot: home error")
}

type snapshotMockFS struct {
	*config.MockFileSystem
	wdErr   error
	homeErr error
	wfFunc  func(path string, data []byte, perm os.FileMode) error
}

func (f *snapshotMockFS) Getwd() (string, error) {
	if f.wdErr != nil {
		return "", f.wdErr
	}
	return f.MockFileSystem.Getwd()
}

func (f *snapshotMockFS) UserHomeDir() (string, error) {
	if f.homeErr != nil {
		return "", f.homeErr
	}
	return f.MockFileSystem.UserHomeDir()
}

func (f *snapshotMockFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	if f.wfFunc != nil {
		return f.wfFunc(path, data, perm)
	}
	return f.MockFileSystem.WriteFile(path, data, perm)
}

func TestUnit_Snapshot_Cleanup_Empty(t *testing.T) {
	mfs := &config.MockFileSystem{}
	err := cleanupSnapshot(mfs, "")
	require.NoError(t, err)
}

func TestUnit_Snapshot_Cleanup_Success(t *testing.T) {
	mfs := &config.MockFileSystem{}
	err := cleanupSnapshot(mfs, "/tmp/snapshot")
	require.NoError(t, err)
}
