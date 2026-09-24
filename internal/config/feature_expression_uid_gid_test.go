package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Expression_UID_GID_MagicWords(t *testing.T) {
	t.Run("Level 0 host context - UID, GID, BASE_UID, BASE_GID", func(t *testing.T) {
		uidVal := 1001
		gidVal := 1002
		mfs := &MockFileSystem{
			WD:       "/workspace",
			HomeDir:  "/home/user",
			UIDValue: &uidVal,
			GIDValue: &gidVal,
		}

		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		uid, err := r.ResolveString("{{UID}}")
		require.NoError(t, err)
		assert.Equal(t, "1001", uid)

		gid, err := r.ResolveString("{{GID}}")
		require.NoError(t, err)
		assert.Equal(t, "1002", gid)

		baseUID, err := r.ResolveString("{{BASE_UID}}")
		require.NoError(t, err)
		assert.Equal(t, "1001", baseUID)

		baseGID, err := r.ResolveString("{{BASE_GID}}")
		require.NoError(t, err)
		assert.Equal(t, "1002", baseGID)
	})

	t.Run("Level 1 nested context - BASE_UID and BASE_GID from HostContext", func(t *testing.T) {
		currentUID := 2001
		currentGID := 2002
		mfs := &MockFileSystem{
			WD:       "/workspace",
			HomeDir:  "/home/user",
			UIDValue: &currentUID,
			GIDValue: &currentGID,
		}

		hostCtx := &HostContext{
			Level:      1,
			WorkingDir: "/host/workspace",
			HomeDir:    "/host/home/user",
			UID:        "1000",
			GID:        "1000",
		}

		r, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		uid, err := r.ResolveString("{{UID}}")
		require.NoError(t, err)
		assert.Equal(t, "2001", uid)

		gid, err := r.ResolveString("{{GID}}")
		require.NoError(t, err)
		assert.Equal(t, "2002", gid)

		baseUID, err := r.ResolveString("{{BASE_UID}}")
		require.NoError(t, err)
		assert.Equal(t, "1000", baseUID)

		baseGID, err := r.ResolveString("{{BASE_GID}}")
		require.NoError(t, err)
		assert.Equal(t, "1000", baseGID)
	})

	t.Run("Fallback to environment variable or 0 when Getuid/Getgid is 0 and non-positive", func(t *testing.T) {
		zeroVal := 0
		mfs := &MockFileSystem{
			WD:       "/workspace",
			HomeDir:  "/home/user",
			UIDValue: &zeroVal,
			GIDValue: &zeroVal,
			Env: map[string]string{
				"UID": "500",
				"GID": "501",
			},
		}

		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		uid, err := r.ResolveString("{{UID}}")
		require.NoError(t, err)
		assert.Equal(t, "0", uid)

		gid, err := r.ResolveString("{{GID}}")
		require.NoError(t, err)
		assert.Equal(t, "0", gid)
	})

	t.Run("Fallback to env var when Getuid returns negative", func(t *testing.T) {
		negVal := -1
		mfs := &MockFileSystem{
			WD:       "/workspace",
			HomeDir:  "/home/user",
			UIDValue: &negVal,
			GIDValue: &negVal,
			Env: map[string]string{
				"UID": "500",
				"GID": "501",
			},
		}

		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		uid, err := r.ResolveString("{{UID}}")
		require.NoError(t, err)
		assert.Equal(t, "500", uid)

		gid, err := r.ResolveString("{{GID}}")
		require.NoError(t, err)
		assert.Equal(t, "501", gid)
	})
}
