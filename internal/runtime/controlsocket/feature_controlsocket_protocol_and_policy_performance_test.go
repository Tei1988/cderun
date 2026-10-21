package controlsocket

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

func TestUnit_ControlSocket_Protocol_ReadWriteFrameInvariants(t *testing.T) {
	t.Parallel()

	t.Run("empty payload frame roundtrip", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := WriteFrame(&buf, nil)
		require.NoError(t, err)

		readPayload, err := ReadFrame(&buf)
		require.NoError(t, err)
		assert.Empty(t, readPayload)
	})

	t.Run("non-empty payload frame roundtrip", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		payload := []byte("hello controlsocket framing")
		err := WriteFrame(&buf, payload)
		require.NoError(t, err)

		readPayload, err := ReadFrame(&buf)
		require.NoError(t, err)
		assert.Equal(t, payload, readPayload)
	})

	t.Run("write frame payload exceeds max size", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		tooLargePayload := make([]byte, MaxFrameSize+1)
		err := WriteFrame(&buf, tooLargePayload)
		require.ErrorContains(t, err, "exceeds maximum allowed size")
	})

	t.Run("read frame length header incomplete", func(t *testing.T) {
		t.Parallel()
		shortBuf := bytes.NewReader([]byte{0x00, 0x00})
		_, err := ReadFrame(shortBuf)
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("read frame payload incomplete", func(t *testing.T) {
		t.Parallel()
		// Header indicates 10 bytes, but stream has only 3 bytes
		truncatedBuf := bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x0A, 0x01, 0x02, 0x03})
		_, err := ReadFrame(truncatedBuf)
		require.ErrorContains(t, err, "failed to read complete frame payload")
	})
}

func TestUnit_ControlSocket_Policy_HasPathTraversalAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("hasPathTraversal edge cases", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			path     string
			expected bool
		}{
			{"", false},
			{".", false},
			{"/usr/local/bin", false},
			{"/tmp/a..b/file", false},
			{"..", true},
			{"../a", true},
			{"a/../b", true},
			{"a/b/..", true},
			{"a/..", true},
			{"/a/b/../c/d", true},
		}

		for _, tc := range cases {
			assert.Equal(t, tc.expected, hasPathTraversal(tc.path), "path: %q", tc.path)
		}
	})

	t.Run("validateMounts short circuits when child mounts are empty", func(t *testing.T) {
		t.Parallel()
		parent := &container.ContainerConfig{ReadOnly: true}
		err := validateMounts(nil, parent)
		require.NoError(t, err)

		err = validateMounts([]container.Mount{}, parent)
		require.NoError(t, err)
	})

	t.Run("validateDevices fails early when parent has no devices", func(t *testing.T) {
		t.Parallel()
		childDevs := []container.DeviceMapping{
			{PathOnHost: "/dev/kvm"},
		}
		err := validateDevices(childDevs, nil)
		require.ErrorContains(t, err, "security violation: child requested host device \"/dev/kvm\" not permitted by parent")
	})

	t.Run("validateCapAdd fails early when parent caps and parent drop caps are empty", func(t *testing.T) {
		t.Parallel()
		childCaps := []string{"SYS_ADMIN"}
		err := validateCapAdd(childCaps, nil, nil)
		require.ErrorContains(t, err, "security violation: child requested capability \"SYS_ADMIN\" not permitted by parent")
	})
}
