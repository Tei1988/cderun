package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Resolver_ResolveTransitiveOptions_Extra(t *testing.T) {
	t.Run("mount-socket false explicitly suppresses transitive from mount-cderun", func(t *testing.T) {
		cli := &CLIOptions{
			Image: Ptr("alpine"),
			MountCderun: Ptr(true),
			MountSocket: Ptr(false)}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.True(t, res.MountCderun)
		assert.False(t, res.MountSocket)
	})

	t.Run("mount-cderun false explicitly suppresses transitive from mount-tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image: Ptr("alpine"),
			MountTools: Ptr("git"),
			MountCderun: Ptr(false)}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.False(t, res.MountCderun)
		// socket is also false because it's transitive from cderun
		assert.False(t, res.MountSocket)
	})

	t.Run("mount-cderun path resolution error", func(t *testing.T) {
		cli := &CLIOptions{
			Image: Ptr("alpine"),
			MountCderunPath: Ptr("{{file:missing}}")}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{WD: "/app"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("mount-socket path resolution error", func(t *testing.T) {
		cli := &CLIOptions{
			Image: Ptr("alpine"),
			MountSocketPath: Ptr("{{file:missing}}")}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{WD: "/app"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("invalid tool name in mount-tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image: Ptr("alpine"),
			MountTools: Ptr("../bad")}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool name in mount-tools")
	})
}

func TestUnit_Resolver_ApplyMemoryOption_Extra(t *testing.T) {
	t.Run("resolution error for memory", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		cli := &CLIOptions{
			Image: Ptr("alpine"),
			Memory: Ptr("{{file:missing}}")}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})
}
