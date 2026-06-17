package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Resolver_ResolveTransitiveOptions_Extra(t *testing.T) {
	t.Run("mount-socket false explicitly suppresses transitive from mount-cderun", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountCderun:    true,
			MountCderunSet: true,
			MountSocket:    false,
			MountSocketSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.True(t, res.MountCderun)
		assert.False(t, res.MountSocket)
	})

	t.Run("mount-cderun false explicitly suppresses transitive from mount-tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountTools:     "git",
			MountToolsSet:  true,
			MountCderun:    false,
			MountCderunSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.False(t, res.MountCderun)
		// socket is also false because it's transitive from cderun
		assert.False(t, res.MountSocket)
	})

	t.Run("mount-cderun path resolution error", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountCderunPath: "{{file:missing}}",
			MountCderunPathSet: true,
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{WD: "/app"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("mount-socket path resolution error", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountSocketPath: "{{file:missing}}",
			MountSocketPathSet: true,
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{WD: "/app"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("invalid tool name in mount-tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountTools:     "../bad",
			MountToolsSet:  true,
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool name in mount-tools")
	})
}

func TestUnit_Resolver_ApplyMemoryOption_Extra(t *testing.T) {
	t.Run("resolution error for memory", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "{{file:missing}}",
			MemorySet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})
}

func TestUnit_ResolverHelpers_Coverage(t *testing.T) {
	t.Run("resolveEnvValues errors", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
		}
		r, _ := NewExpressionResolverWithFS(nil, mfs)

		// Test resolveStringOpt error via mock FS
		mfs.ReadFileErr = assert.AnError
		_, err := resolveEnvValues([]string{"KEY={{file:err}}"}, false, r, mfs)
		assert.Error(t, err)
	})

	t.Run("resolveMounts errors", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
		}
		r, _ := NewExpressionResolverWithFS(nil, mfs)

		// Test mc.Source.Resolve error via pickConfigs
		mfs.ReadFileErr = assert.AnError
		_, err := resolveMounts([]string{"type=bind,source={{file:err}},target=/t"}, nil, "", nil, nil, r, mfs)
		assert.Error(t, err)

		// Test r.Stat(hostPath) error that is not os.ErrNotExist
		mfs.ReadFileErr = nil
		mfs.StatErr = assert.AnError
		_, err = resolveMounts([]string{"type=bind,source=/host,target=/t,optional"}, nil, "", nil, nil, r, mfs)
		assert.Error(t, err)
	})
}
