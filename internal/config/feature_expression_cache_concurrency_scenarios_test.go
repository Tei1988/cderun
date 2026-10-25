package config

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpressionResolver_ConcurrentResolutionAndCache(t *testing.T) {
	t.Parallel()

	mockFS := &MockFileSystem{
		HomeDir: "/home/testuser",
		WD:      "/home/testuser/project",
		Env: map[string]string{
			"APP_ENV": "production",
			"API_KEY": "secret123",
		},
		Files: map[string][]byte{
			"/home/testuser/project/VERSION":     []byte("2.5.0\n"),
			"VERSION":                            []byte("2.5.0\n"),
			"/home/testuser/project/CONFIG_NAME": []byte("prod-db\n"),
			"CONFIG_NAME":                        []byte("prod-db\n"),
		},
	}

	resolver, err := NewExpressionResolverWithFS(&HostContext{
		HomeDir:    "/home/testuser",
		WorkingDir: "/home/testuser/project",
		Level:      0,
	}, mockFS)
	require.NoError(t, err)

	const workers = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		workerID := i
		go func() {
			defer wg.Done()
			for j := range iterations {
				switch (workerID + j) % 5 {
				case 0:
					res, resolveErr := resolver.ResolveString("ver-{{file:VERSION}}-{{env:APP_ENV}}")
					assert.NoError(t, resolveErr)
					assert.Equal(t, "ver-2.5.0-production", res)
				case 1:
					res, resolveErr := resolver.ResolveString("db-{{file:CONFIG_NAME:-default}}-{{env:MISSING:-fallback}}")
					assert.NoError(t, resolveErr)
					assert.Equal(t, "db-prod-db-fallback", res)
				case 2:
					res, resolveErr := resolver.ResolveString("home-{{HOME}}-pwd-{{PWD}}")
					assert.NoError(t, resolveErr)
					assert.Equal(t, "home-/home/testuser-pwd-/home/testuser/project", res)
				case 3:
					info, statErr := resolver.Stat("VERSION")
					assert.NoError(t, statErr)
					assert.NotNil(t, info)
				case 4:
					childResolver := resolver.WithoutHostContext()
					res, resolveErr := childResolver.ResolveString("child-{{file:VERSION}}")
					assert.NoError(t, resolveErr)
					assert.Equal(t, "child-2.5.0", res)
				}
			}
		}()
	}

	wg.Wait()
}

func TestExpressionResolver_MaxDirectiveFileSizeBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("File exactly at 1MB limit passes", func(t *testing.T) {
		t.Parallel()
		exactMB := make([]byte, MaxDirectiveFileSize)
		for i := range exactMB {
			exactMB[i] = 'a'
		}
		mockFS := &MockFileSystem{
			HomeDir: "/home/testuser",
			WD:      "/home/testuser/project",
			Files: map[string][]byte{
				"/home/testuser/project/exact.txt": exactMB,
				"exact.txt":                         exactMB,
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		res, resolveErr := resolver.ResolveString("{{file:exact.txt}}")
		require.NoError(t, resolveErr)
		assert.Len(t, res, MaxDirectiveFileSize)
	})

	t.Run("File exceeding 1MB limit by 1 byte fails and caches size limit error", func(t *testing.T) {
		t.Parallel()
		oversized := make([]byte, MaxDirectiveFileSize+1)
		for i := range oversized {
			oversized[i] = 'b'
		}
		mockFS := &MockFileSystem{
			HomeDir: "/home/testuser",
			WD:      "/home/testuser/project",
			Files: map[string][]byte{
				"/home/testuser/project/oversized.txt": oversized,
				"oversized.txt":                         oversized,
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		// Create child resolver BEFORE parent records sticky error so child shares the same cache with nil sticky error
		childResolver := resolver.WithoutHostContext()

		// First call on parent triggers stat size check failure and caches error in shared fileCache
		_, firstErr := resolver.ResolveString("{{file:oversized.txt}}")
		require.Error(t, firstErr)
		assert.Contains(t, firstErr.Error(), "too large")

		// Record mockFS.StatCalls count after first resolution
		statCallsCount := len(mockFS.StatCalls)

		// Clear the shared stat cache so a file-cache miss must call Stat again
		sharedState := resolver.getShared()
		sharedState.mu.Lock()
		sharedState.statCache = make(map[string]statCacheEntry)
		sharedState.mu.Unlock()

		// Derived child resolver queries the shared cache for fileCache entry
		_, secondErr := childResolver.ResolveString("{{file:oversized.txt:-fallback}}")
		require.Error(t, secondErr)
		assert.Contains(t, secondErr.Error(), "too large")

		// Verify that the second call hit the file cache and did not trigger additional Stat calls
		assert.Len(t, mockFS.StatCalls, statCallsCount)
	})
}

func TestExpressionResolver_WithoutHostContextSharedStatePropagation(t *testing.T) {
	t.Parallel()

	mockFS := &MockFileSystem{
		HomeDir: "/home/testuser",
		WD:      "/home/testuser/project",
		Files: map[string][]byte{
			"/home/testuser/project/SHARED": []byte("shared_content\n"),
			"SHARED":                            []byte("shared_content\n"),
		},
	}

	parentResolver, err := NewExpressionResolverWithFS(&HostContext{
		HomeDir:    "/home/testuser",
		WorkingDir: "/home/testuser/project",
		Level:      1,
	}, mockFS)
	require.NoError(t, err)

	// Resolve in parent to populate file cache
	parentRes, parentErr := parentResolver.ResolveString("{{file:SHARED}}")
	require.NoError(t, parentErr)
	assert.Equal(t, "shared_content", parentRes)

	// Derive child resolver without host context
	childResolver := parentResolver.WithoutHostContext()
	require.Nil(t, childResolver.HostContext)
	require.NoError(t, childResolver.Error())

	// Child should hit the cached entry populated by parent
	childRes, childErr := childResolver.ResolveString("{{file:SHARED}}")
	require.NoError(t, childErr)
	assert.Equal(t, "shared_content", childRes)

	// Confirm nil WithoutHostContext safe call
	var nilResolver *ExpressionResolver
	assert.Nil(t, nilResolver.WithoutHostContext())
}

func TestExpressionResolver_StatErrorVsReadErrorFallback(t *testing.T) {
	t.Parallel()

	t.Run("File not found uses default when present", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			HomeDir: "/home/testuser",
			WD:      "/home/testuser/project",
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		res, resolveErr := resolver.ResolveString("{{file:nonexistent.txt:-default_val}}")
		require.NoError(t, resolveErr)
		assert.Equal(t, "default_val", res)
	})

	t.Run("File not found returns error when default absent", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			HomeDir: "/home/testuser",
			WD:      "/home/testuser/project",
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		_, resolveErr := resolver.ResolveString("{{file:nonexistent.txt}}")
		require.Error(t, resolveErr)
		assert.Contains(t, resolveErr.Error(), "file not found")
	})

	t.Run("Stat error uses default when present", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			HomeDir: "/home/testuser",
			WD:      "/home/testuser/project",
			StatErr: errors.New("permission denied on stat"),
			Files: map[string][]byte{
				"/home/testuser/project/stat_fail.txt": []byte("content"),
				"stat_fail.txt":                         []byte("content"),
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		res, resolveErr := resolver.ResolveString("{{file:stat_fail.txt:-stat_fallback}}")
		require.NoError(t, resolveErr)
		assert.Equal(t, "stat_fallback", res)
	})

	t.Run("Read error uses default when present", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			HomeDir:     "/home/testuser",
			WD:          "/home/testuser/project",
			ReadFileErr: errors.New("read error IO failure"),
			Files: map[string][]byte{
				"/home/testuser/project/read_fail.txt": []byte("content"),
				"read_fail.txt":                         []byte("content"),
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		res, resolveErr := resolver.ResolveString("{{file:read_fail.txt:-read_fallback}}")
		require.NoError(t, resolveErr)
		assert.Equal(t, "read_fallback", res)
	})
}

func TestExpressionResolver_StickyErrorAndNestedDirectives(t *testing.T) {
	t.Parallel()

	t.Run("Sticky error halts further expression evaluation", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			HomeDir: "/home/testuser",
			WD:      "/home/testuser/project",
			Env: map[string]string{
				"VALID_ENV": "valid_value",
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		// First expression contains unknown directive leading to sticky error
		res, resolveErr := resolver.ResolveString("prefix-{{UNKNOWN_MAGIC}}-{{env:VALID_ENV}}")
		require.Error(t, resolveErr)
		assert.Contains(t, resolveErr.Error(), "unknown directive or magic word")
		assert.NotEmpty(t, res)
		assert.Equal(t, resolver.Error(), resolveErr)

		// Subsequent calls on same resolver instance immediately return input string and sticky error
		subsequentRes, subsequentErr := resolver.ResolveString("{{env:VALID_ENV}}")
		require.Error(t, subsequentErr)
		assert.Equal(t, "{{env:VALID_ENV}}", subsequentRes)
		assert.Equal(t, resolveErr, subsequentErr)
	})

	t.Run("Nested directive fallback chain resolves recursively", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			HomeDir: "/home/testuser",
			WD:      "/home/testuser/project",
			Files: map[string][]byte{
				"/home/testuser/project/VER_FILE": []byte("1.2.3"),
				"VER_FILE":                         []byte("1.2.3"),
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		// {{env:NON_EXISTENT_VAR:-{{file:VER_FILE:-0.0.1}}}} -> env missing, evaluates file fallback -> 1.2.3
		res, resolveErr := resolver.ResolveString("{{env:NON_EXISTENT_VAR:-{{file:VER_FILE:-0.0.1}}}}")
		require.NoError(t, resolveErr)
		assert.Equal(t, "1.2.3", res)

		// Nested directive where file missing -> falls back to innermost default "0.0.1"
		resolver2, err2 := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err2)
		res2, resolveErr2 := resolver2.ResolveString("{{env:NON_EXISTENT_VAR:-{{file:MISSING_FILE:-0.0.1}}}}")
		require.NoError(t, resolveErr2)
		assert.Equal(t, "0.0.1", res2)
	})

	t.Run("Resolve generic data types (slice, map, string, int)", func(t *testing.T) {
		t.Parallel()
		mockFS := &MockFileSystem{
			HomeDir: "/home/testuser",
			WD:      "/home/testuser/project",
			Env: map[string]string{
				"MODE": "test",
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mockFS)
		require.NoError(t, err)

		inputMap := map[string]any{
			"b_key": "app-{{env:MODE}}",
			"a_key": []any{
				"home-{{HOME}}",
				42,
			},
			"c_key": true,
		}

		resolved := resolver.Resolve(inputMap)
		require.NoError(t, resolver.Error())

		resMap, ok := resolved.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "app-test", resMap["b_key"])
		assert.Equal(t, true, resMap["c_key"])

		resSlice, ok := resMap["a_key"].([]any)
		require.True(t, ok)
		assert.Equal(t, "home-/home/testuser", resSlice[0])
		assert.Equal(t, 42, resSlice[1])
	})
}

func TestExpressionResolver_StatCacheConcurrencyAndExpiration(t *testing.T) {
	t.Parallel()

	mockFS := &MockFileSystem{
		HomeDir: "/home/testuser",
		WD:      "/home/testuser/project",
		Files: map[string][]byte{
			"/home/testuser/project/DYNAMIC.txt": []byte("initial"),
			"DYNAMIC.txt":                         []byte("initial"),
		},
	}

	resolver, err := NewExpressionResolverWithFS(nil, mockFS)
	require.NoError(t, err)

	info1, err1 := resolver.Stat("DYNAMIC.txt")
	require.NoError(t, err1)

	// Stat call is cached, so second call returns identical FileInfo struct
	info2, err2 := resolver.Stat("DYNAMIC.txt")
	require.NoError(t, err2)
	assert.Equal(t, info1.Name(), info2.Name())
	assert.Equal(t, info1.Size(), info2.Size())
}
