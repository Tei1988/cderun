package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMountInfoReader struct {
	Content []byte
	Err     error
}

func (m *mockMountInfoReader) ReadMountInfo(fs FileSystem) ([]byte, error) {
	return m.Content, m.Err
}

func TestUnit_Config_DiscoverOverlay(t *testing.T) {
	mfs := &MockFileSystem{}
	originalReader := DefaultMountInfoReader
	defer func() { DefaultMountInfoReader = originalReader }()

	t.Run("successfully discover upperdir", func(t *testing.T) {
		mountinfo := "24 25 0:21 / / rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/u,workdir=/w\n"
		DefaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := DiscoverOverlayUpperDir(mfs)
		require.NoError(t, err)
		assert.Equal(t, "/u", upperdir)
	})

	t.Run("no overlay found", func(t *testing.T) {
		mountinfo := "24 25 0:21 / / rw,relatime - ext4 /dev/sda1 rw\n"
		DefaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := DiscoverOverlayUpperDir(mfs)
		require.NoError(t, err)
		assert.Empty(t, upperdir)
	})

	t.Run("malformed mountinfo", func(t *testing.T) {
		mountinfo := "too few fields\n"
		DefaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := DiscoverOverlayUpperDir(mfs)
		require.NoError(t, err)
		assert.Empty(t, upperdir)
	})
}
