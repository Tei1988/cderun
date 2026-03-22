package command

import (
	"cderun/internal/config"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Snapshot_DiscoverOverlayUpperDir_NoSeparator(t *testing.T) {
	mfs := &config.MockFileSystem{}
	originalReader := defaultMountInfoReader
	defer func() { defaultMountInfoReader = originalReader }()

	mountinfo := "24 25 0:21 / / rw,relatime overlay overlay rw,upperdir=/u\n"
	defaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

	upperdir, err := discoverOverlayUpperDir(mfs)
	require.NoError(t, err)
	assert.Empty(t, upperdir)
}

func TestUnit_Snapshot_DiscoverOverlayUpperDir_NonRootOverlay(t *testing.T) {
	mfs := &config.MockFileSystem{}
	originalReader := defaultMountInfoReader
	defer func() { defaultMountInfoReader = originalReader }()

	mountinfo := "24 25 0:21 / /mnt/foo rw,relatime - overlay overlay rw,upperdir=/u\n"
	defaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

	upperdir, err := discoverOverlayUpperDir(mfs)
	require.NoError(t, err)
	assert.Empty(t, upperdir)
}

func TestUnit_Snapshot_DiscoverOverlayUpperDir_FewFieldsAfterSeparator(t *testing.T) {
	mfs := &config.MockFileSystem{}
	originalReader := defaultMountInfoReader
	defer func() { defaultMountInfoReader = originalReader }()

	mountinfo := "24 25 0:21 / / rw,relatime - overlay\n"
	defaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

	upperdir, err := discoverOverlayUpperDir(mfs)
	require.NoError(t, err)
	assert.Empty(t, upperdir)
}
