package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Expression_ReverseResolution_RootFallbackInvariants(t *testing.T) {
	t.Parallel()

	t.Run("root mount fallback bypass when path is under Base Host root and absent on execution host", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files:   make(map[string][]byte),
			Dirs:    make(map[string]bool),
			HomeDir: "/home/execution-user",
			WD:      "/app/execution",
		}

		hc := &HostContext{
			Level:      1,
			HomeDir:    "/Users/basehost-user",
			WorkingDir: "/Users/basehost-user/workspace",
			Mounts: []MountMapping{
				{
					Source: "/var/lib/docker/overlay2/upper",
					Target: "/",
					Level:  0,
				},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hc, mfs)
		require.NoError(t, err)

		// /Users/basehost-user/workspace/config.yaml is under Base Host WorkingDir,
		// and does not exist in mfs.
		res, err := resolver.applyReverseResolution("/Users/basehost-user/workspace/config.yaml")
		require.NoError(t, err)
		assert.Equal(t, "/Users/basehost-user/workspace/config.yaml", res,
			"should bypass overlayfs root fallback when path is base host root and absent on execution host")
	})

	t.Run("root mount fallback applied when path exists on execution host", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/Users/basehost-user/workspace/config.yaml": []byte("some content"),
			},
			Dirs:    make(map[string]bool),
			HomeDir: "/home/execution-user",
			WD:      "/app/execution",
		}

		hc := &HostContext{
			Level:      1,
			HomeDir:    "/Users/basehost-user",
			WorkingDir: "/Users/basehost-user/workspace",
			Mounts: []MountMapping{
				{
					Source: "/var/lib/docker/overlay2/upper",
					Target: "/",
					Level:  0,
				},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hc, mfs)
		require.NoError(t, err)

		res, err := resolver.applyReverseResolution("/Users/basehost-user/workspace/config.yaml")
		require.NoError(t, err)
		assert.Equal(t, "/var/lib/docker/overlay2/upper/Users/basehost-user/workspace/config.yaml", res,
			"should apply root mount fallback when path exists on execution host")
	})

	t.Run("root mount fallback applied when path is outside Base Host roots", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files:   make(map[string][]byte),
			Dirs:    make(map[string]bool),
			HomeDir: "/home/execution-user",
			WD:      "/app/execution",
		}

		hc := &HostContext{
			Level:      1,
			HomeDir:    "/Users/basehost-user",
			WorkingDir: "/Users/basehost-user/workspace",
			Mounts: []MountMapping{
				{
					Source: "/var/lib/docker/overlay2/upper",
					Target: "/",
					Level:  0,
				},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hc, mfs)
		require.NoError(t, err)

		res, err := resolver.applyReverseResolution("/tmp/scratch/file.txt")
		require.NoError(t, err)
		assert.Equal(t, "/var/lib/docker/overlay2/upper/tmp/scratch/file.txt", res,
			"should apply overlay fallback when path is outside Base Host roots")
	})

	t.Run("ambiguous root handling where Base Host root equals execution host root", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files:   make(map[string][]byte),
			Dirs:    make(map[string]bool),
			HomeDir: "/workspace",
			WD:      "/workspace",
		}

		hc := &HostContext{
			Level:      1,
			HomeDir:    "/workspace", // identical to execution host HomeDir/WD
			WorkingDir: "/workspace",
			Mounts: []MountMapping{
				{
					Source: "/var/lib/docker/overlay2/upper",
					Target: "/",
					Level:  0,
				},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hc, mfs)
		require.NoError(t, err)

		// When base host root == execution host root, isBaseHostPath returns false,
		// allowing normal mapping to proceed without ambiguous bypass.
		res, err := resolver.applyReverseResolution("/workspace/sub/file.txt")
		require.NoError(t, err)
		assert.Equal(t, "/var/lib/docker/overlay2/upper/workspace/sub/file.txt", res)
	})
}

func TestUnit_Expression_ReverseResolution_MountPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("longest target match precedence", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files:   make(map[string][]byte),
			Dirs:    make(map[string]bool),
			HomeDir: "/home/user",
			WD:      "/workspace",
		}

		hc := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{
					Source: "/host/base",
					Target: "/workspace",
					Level:  0,
				},
				{
					Source: "/host/specific",
					Target: "/workspace/nested",
					Level:  0,
				},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hc, mfs)
		require.NoError(t, err)

		res, err := resolver.applyReverseResolution("/workspace/nested/file.go")
		require.NoError(t, err)
		assert.Equal(t, "/host/specific/file.go", res)
	})

	t.Run("higher level mount precedence for equal target lengths", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files:   make(map[string][]byte),
			Dirs:    make(map[string]bool),
			HomeDir: "/home/user",
			WD:      "/workspace",
		}

		hc := &HostContext{
			Level: 2,
			Mounts: []MountMapping{
				{
					Source: "/host/level0",
					Target: "/workspace",
					Level:  0,
				},
				{
					Source: "/host/level1",
					Target: "/workspace",
					Level:  1,
				},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hc, mfs)
		require.NoError(t, err)

		res, err := resolver.applyReverseResolution("/workspace/main.go")
		require.NoError(t, err)
		assert.Equal(t, "/host/level1/main.go", res)
	})

	t.Run("level 0 host context does not rewrite paths", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files:   make(map[string][]byte),
			Dirs:    make(map[string]bool),
			HomeDir: "/home/user",
			WD:      "/workspace",
		}

		hc := &HostContext{
			Level: 0,
			Mounts: []MountMapping{
				{
					Source: "/host/base",
					Target: "/workspace",
					Level:  0,
				},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hc, mfs)
		require.NoError(t, err)

		res, err := resolver.applyReverseResolution("/workspace/main.go")
		require.NoError(t, err)
		assert.Equal(t, "/workspace/main.go", res)
	})

	t.Run("no matching mount target leaves path unchanged", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files:   make(map[string][]byte),
			Dirs:    make(map[string]bool),
			HomeDir: "/home/user",
			WD:      "/workspace",
		}

		hc := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{
					Source: "/host/base",
					Target: "/different/path",
					Level:  0,
				},
			},
		}

		resolver, err := NewExpressionResolverWithFS(hc, mfs)
		require.NoError(t, err)

		res, err := resolver.applyReverseResolution("/workspace/main.go")
		require.NoError(t, err)
		assert.Equal(t, "/workspace/main.go", res)
	})
}

func TestUnit_Expression_UID_GID_FallbackInvariants(t *testing.T) {
	t.Parallel()

	t.Run("negative UID falls back to env UID if present", func(t *testing.T) {
		neg := -1
		mfs := &MockFileSystem{
			UIDValue: &neg,
			Env:      map[string]string{"UID": "1002"},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := resolver.ResolveString("{{UID}}")
		require.NoError(t, err)
		assert.Equal(t, "1002", val)
	})

	t.Run("negative UID falls back to 0 when env UID is unset", func(t *testing.T) {
		neg := -1
		mfs := &MockFileSystem{
			UIDValue: &neg,
			Env:      make(map[string]string),
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := resolver.ResolveString("{{UID}}")
		require.NoError(t, err)
		assert.Equal(t, "0", val)
	})

	t.Run("negative GID falls back to env GID if present", func(t *testing.T) {
		neg := -1
		mfs := &MockFileSystem{
			GIDValue: &neg,
			Env:      map[string]string{"GID": "2002"},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := resolver.ResolveString("{{GID}}")
		require.NoError(t, err)
		assert.Equal(t, "2002", val)
	})

	t.Run("negative GID falls back to 0 when env GID is unset", func(t *testing.T) {
		neg := -1
		mfs := &MockFileSystem{
			GIDValue: &neg,
			Env:      make(map[string]string),
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := resolver.ResolveString("{{GID}}")
		require.NoError(t, err)
		assert.Equal(t, "0", val)
	})

	t.Run("nil fs fallback returns 0", func(t *testing.T) {
		resolver := &ExpressionResolver{}
		assert.Equal(t, "0", resolver.getUID())
		assert.Equal(t, "0", resolver.getGID())
	})
}

func TestUnit_Resolver_EngineAndSocketResolution(t *testing.T) {
	t.Parallel()

	t.Run("deduceEngineFromSocketPath returns expected engine names", func(t *testing.T) {
		testCases := []struct {
			socketPath     string
			expectedEngine string
		}{
			{"/var/run/docker.sock", "docker"},
			{"/run/user/1000/podman/podman.sock", "podman"},
			{"/run/containerd/containerd.sock", "containerd"},
			{"/custom/path/to/podman.sock/", "podman"},
			{"/custom/path/containerd.sock", "containerd"},
			{"/unknown/engine.sock", "docker"},
		}

		for _, tc := range testCases {
			actual := deduceEngineFromSocketPath(tc.socketPath)
			assert.Equal(t, tc.expectedEngine, actual, "failed for socket: %s", tc.socketPath)
		}
	})

	t.Run("determineEngineFromWinner evaluates expressions and tilde", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/workspace",
			Env:     map[string]string{"DESIRED_ENGINE": "containerd"},
		}

		cliOpts := &CLIOptions{}
		cliEngine := "{{env:DESIRED_ENGINE}}"
		cliOpts.Engine = &cliEngine

		rv := &resolver{
			fs:     mfs,
			cli:    cliOpts,
			global: &CDERunConfig{},
			res:    &ResolvedConfig{},
		}

		err := rv.determineEngineFromWinner()
		require.NoError(t, err)
		assert.Equal(t, "containerd", rv.res.Engine)
	})

	t.Run("determineEngineFromWinner with tilde expansion", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/workspace",
			Env:     make(map[string]string),
		}

		cliOpts := &CLIOptions{}
		cliEngine := "~/bin/custom-engine"
		cliOpts.Engine = &cliEngine

		rv := &resolver{
			fs:     mfs,
			cli:    cliOpts,
			global: &CDERunConfig{},
			res:    &ResolvedConfig{},
		}

		err := rv.determineEngineFromWinner()
		require.NoError(t, err)
		assert.Equal(t, "/home/user/bin/custom-engine", rv.res.Engine)
	})

	t.Run("determineEngineFromWinner invalid expression returns error", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/workspace",
			Env:     make(map[string]string),
		}

		cliOpts := &CLIOptions{}
		cliEngine := "{{invalid_directive:foo}}"
		cliOpts.Engine = &cliEngine

		rv := &resolver{
			fs:     mfs,
			cli:    cliOpts,
			global: &CDERunConfig{},
			res:    &ResolvedConfig{},
		}

		err := rv.determineEngineFromWinner()
		require.Error(t, err)
	})
}

func TestUnit_Expression_FindDir_FallbackEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("find_dir fallback with nested expression succeeds", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/workspace/myproject",
			Env:     map[string]string{"FALLBACK_DIR": "/tmp/fallback"},
			Dirs: map[string]bool{
				"/workspace/myproject": true,
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := resolver.ResolveString("{{find_dir:nonexistent_marker:-{{env:FALLBACK_DIR}}}}")
		require.NoError(t, err)
		assert.Equal(t, "/tmp/fallback", val)
	})

	t.Run("find_dir parameter validation errors do not fall back", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/workspace",
			Env:     make(map[string]string),
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Directory traversal in parameter must error directly, not use fallback
		_, err = resolver.ResolveString("{{find_dir:../secret:-/default}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file or directory name is allowed")
	})

	t.Run("file parameter validation errors do not fall back", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/workspace",
			Env:     make(map[string]string),
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Path separators in file parameter must error directly
		_, err = resolver.ResolveString("{{file:sub/path.txt:-/default}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file name is allowed")
	})
}
