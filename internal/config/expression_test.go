package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exprMockFS struct {
	MockFileSystem
	homeDirErr  error
	getwdErr    error
	readFileErr error
}

func (m *exprMockFS) UserHomeDir() (string, error) {
	if m.homeDirErr != nil {
		return "", m.homeDirErr
	}
	return m.MockFileSystem.UserHomeDir()
}

func (m *exprMockFS) Getwd() (string, error) {
	if m.getwdErr != nil {
		return "", m.getwdErr
	}
	return m.MockFileSystem.Getwd()
}

func (m *exprMockFS) ReadFile(name string) ([]byte, error) {
	if m.readFileErr != nil {
		return nil, m.readFileErr
	}
	return m.MockFileSystem.ReadFile(name)
}

func TestUnit_Expression_BaseHomeAndBasePwd(t *testing.T) {
	fs := &MockFileSystem{
		WD:      "/container/work",
		HomeDir: "/root",
	}

	t.Run("BASE_HOME and BASE_PWD fall back to HOME/PWD at level 0", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)
		assert.Equal(t, "/root", r.resolveString("{{BASE_HOME}}"))
		assert.Equal(t, "/container/work", r.resolveString("{{BASE_PWD}}"))
	})

	t.Run("BASE_HOME and BASE_PWD return host values in nested execution", func(t *testing.T) {
		hostCtx := &HostContext{
			Level:      1,
			HomeDir:    "/Users/user",
			WorkingDir: "/Users/user/project",
		}
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		assert.Equal(t, "/Users/user", r.resolveString("{{BASE_HOME}}"))
		assert.Equal(t, "/Users/user/project", r.resolveString("{{BASE_PWD}}"))
		// HOME and PWD still return container-local values
		assert.Equal(t, "/root", r.resolveString("{{HOME}}"))
		assert.Equal(t, "/container/work", r.resolveString("{{PWD}}"))
	})

	t.Run("NewExpressionResolverWithFS - UserHomeDir failure", func(t *testing.T) {
		fsErr := &exprMockFS{
			homeDirErr: assert.AnError,
			MockFileSystem: MockFileSystem{
				WD: "/wd",
			},
		}
		r, err := NewExpressionResolverWithFS(nil, fsErr)
		require.NoError(t, err)
		assert.Empty(t, r.Home)
	})

	t.Run("NewExpressionResolverWithFS - Getwd failure", func(t *testing.T) {
		fsErr := &exprMockFS{
			getwdErr: assert.AnError,
			MockFileSystem: MockFileSystem{
				HomeDir: "/home",
			},
		}
		r, err := NewExpressionResolverWithFS(nil, fsErr)
		require.NoError(t, err)
		assert.Empty(t, r.Pwd)
	})
}

func TestUnit_Expression_FindDir(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/modules/foo":                      []byte("bar"),
			"/project/services/production/.cderun.yaml": []byte("runtime: docker"),
		},
		Dirs: map[string]bool{
			"/project":                     true,
			"/project/modules":             true,
			"/project/services":            true,
			"/project/services/production": true,
		},
		WD: "/project/services/production",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("find_dir existing directory", func(t *testing.T) {
		val := r.resolveString("{{ find_dir:modules }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "../..", val)
	})

	t.Run("find_dir existing file", func(t *testing.T) {
		val := r.resolveString("{{ find_dir:modules/foo }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "../../modules", val)
	})

	t.Run("find_dir not found", func(t *testing.T) {
		r2, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		r2.resolveString("{{ find_dir:nonexistent }}")
		require.Error(t, r2.Error())
		assert.Contains(t, r2.Error().Error(), "item not found for find_dir: nonexistent")
	})

	t.Run("filepath.Rel failure in resolveFindDir", func(t *testing.T) {
		// On Unix, filepath.Rel("rel", "/abs") fails.
		fs := &MockFileSystem{
			Files: map[string][]byte{"/abs/foo": []byte("bar")},
			Dirs:  map[string]bool{"/abs": true},
			WD:    "/abs",
		}
		r2, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		// Force find_dir to use a search result that will fail Rel against r2.Pwd
		// resolveFindDir calls FindConfigs, which returns absolute paths from MockFileSystem.Abs
		// Then it calls filepath.Rel(r.Pwd, dir)
		r2.Pwd = "relative" // rel
		// FindConfigs will find /abs/foo
		r2.resolveString("{{ find_dir:foo }}")
		require.Error(t, r2.Error())
		assert.Contains(t, r2.Error().Error(), "failed to calculate relative path")
	})
}

func TestUnit_Expression_FileError(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/.go-version": []byte("1.21\n"),
		},
		Dirs: map[string]bool{
			"/project": true,
		},
		WD: "/project",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("file found", func(t *testing.T) {
		val := r.resolveString("{{ file:.go-version }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "1.21", val)
	})

	t.Run("file not found", func(t *testing.T) {
		r2, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		r2.resolveString("{{ file:missing }}")
		require.Error(t, r2.Error())
		assert.Contains(t, r2.Error().Error(), "file not found: missing")
	})

	t.Run("ReadFile failure in resolveFile", func(t *testing.T) {
		fsErr := &exprMockFS{
			MockFileSystem: MockFileSystem{
				Files: map[string][]byte{"/project/.go-version": []byte("1.21")},
				Dirs:  map[string]bool{"/project": true},
				WD:    "/project",
			},
			readFileErr: assert.AnError,
		}
		r2, err := NewExpressionResolverWithFS(hostCtx, fsErr)
		require.NoError(t, err)
		r2.resolveString("{{ file:.go-version }}")
		require.Error(t, r2.Error())
		assert.Contains(t, r2.Error().Error(), "failed to read file")
	})
}

func TestUnit_Expression_FileEmpty(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/empty.txt":  []byte("   "),
			"/project/normal.txt": []byte("content"),
		},
		Dirs: map[string]bool{
			"/project": true,
		},
		WD: "/project",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("empty file resolves to empty string", func(t *testing.T) {
		val := r.resolveString("{{ file:empty.txt }}")
		require.NoError(t, r.Error())
		assert.Empty(t, val)
	})

	t.Run("normal file still works", func(t *testing.T) {
		val := r.resolveString("{{ file:normal.txt }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "content", val)
	})

	t.Run("cached empty file still works", func(t *testing.T) {
		// Second call should hit cache
		val := r.resolveString("{{ file:empty.txt }}")
		require.NoError(t, r.Error())
		assert.Empty(t, val)
	})
}

func TestUnit_Expression_SecurityAndEdgeCases(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/inner.txt": []byte("outer.txt"),
			"/project/outer.txt": []byte("content"),
		},
		Dirs: map[string]bool{
			"/project": true,
		},
		WD: "/project",
	}

	hostCtx := &HostContext{}

	t.Run("nested expressions (partial match due to non-recursive regex)", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{ file:{{ file:inner.txt }} }}")
		// Matches "{{ file:{{ file:inner.txt }}" and fails to find such file
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "file not found")
		assert.Equal(t, "{{ file:{{ file:inner.txt }} }}", val)
	})

	t.Run("multiple expressions", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{ PWD }}/{{ file:inner.txt }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "/project/outer.txt", val)
	})

	t.Run("path traversal attempt in file", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		r.resolveString("{{ file:../../etc/passwd }}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "parent directory references are not allowed")
	})

	t.Run("absolute path attempt in file", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		r.resolveString("{{ file:/etc/passwd }}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "absolute paths")
		assert.Contains(t, r.Error().Error(), "are not allowed")
	})

	t.Run("invalid directive", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{ unknown:value }}")
		require.NoError(t, r.Error())
		// Unknown directive is returned as is (but trimmed)
		assert.Equal(t, "{{unknown:value}}", val)
	})

	t.Run("unclosed expression", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{ PWD")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{ PWD", val)
	})

	t.Run("empty expression", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{}}")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{}}", val)
	})

	t.Run("expression with whitespace only", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{   }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{}}", val)
	})

	t.Run("complex mixed resolution", func(t *testing.T) {
		fsWithHome := *fs
		fsWithHome.HomeDir = "/home/user"
		r, err := NewExpressionResolverWithFS(hostCtx, &fsWithHome)
		require.NoError(t, err)
		val := r.resolveString("~/.config/{{file:inner.txt}}/settings.json")
		require.NoError(t, r.Error())
		assert.Equal(t, "/home/user/.config/outer.txt/settings.json", val)
	})

	t.Run("sticky error state", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		// First call triggers error
		r.resolveString("{{ file:nonexistent }}")
		require.Error(t, r.Error())

		// Subsequent call should stay in error state
		val := r.resolveString("{{ PWD }}")
		assert.Equal(t, "{{ PWD }}", val) // Unchanged because of sticky error

		// Resolve should also be affected
		res := r.Resolve("{{ PWD }}")
		assert.Equal(t, "{{ PWD }}", res)
	})
}

func TestUnit_Expression_Resolve_Complex(t *testing.T) {
	fs := &MockFileSystem{
		WD: "/work",
	}
	r, err := NewExpressionResolverWithFS(nil, fs)
	require.NoError(t, err)

	t.Run("Resolve []any", func(t *testing.T) {
		input := []any{"{{PWD}}", 123, []any{"{{PWD}}"}}
		expected := []any{"/work", 123, []any{"/work"}}
		actual := r.Resolve(input)
		assert.Equal(t, expected, actual)
	})

	t.Run("Resolve map[string]any", func(t *testing.T) {
		input := map[string]any{
			"a": "{{PWD}}",
			"b": 456,
			"c": map[string]any{"d": "{{PWD}}"},
		}
		expected := map[string]any{
			"a": "/work",
			"b": 456,
			"c": map[string]any{"d": "/work"},
		}
		actual := r.Resolve(input)
		assert.Equal(t, expected, actual)
	})

	t.Run("Resolve other types", func(t *testing.T) {
		assert.Equal(t, 123, r.Resolve(123))
		assert.Equal(t, true, r.Resolve(true))
	})
}

func TestUnit_Expression_Security_Advanced(t *testing.T) {
	fs := &MockFileSystem{WD: "/work"}

	t.Run("resolveFindDir absolute path", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)
		r.resolveString("{{ find_dir:/etc }}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "absolute paths")
	})

	t.Run("resolveFindDir parent directory", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)
		r.resolveString("{{ find_dir:../secret }}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "parent directory references")
	})
}

func TestUnit_Expression_EnvWithDefault(t *testing.T) {
	fs := &MockFileSystem{
		Env: map[string]string{
			"VAR_SET":   "value",
			"VAR_EMPTY": "",
		},
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "env set",
			input:    "{{ env:VAR_SET }}",
			expected: "value",
		},
		{
			name:     "env unset",
			input:    "{{ env:VAR_UNSET }}",
			expected: "",
		},
		{
			name:     "env set with default",
			input:    "{{ env:VAR_SET:-default }}",
			expected: "value",
		},
		{
			name:     "env empty with default",
			input:    "{{ env:VAR_EMPTY:-default }}",
			expected: "default",
		},
		{
			name:     "env unset with default",
			input:    "{{ env:VAR_UNSET:-default }}",
			expected: "default",
		},
		{
			name:     "env with complex default",
			input:    "{{ env:VAR_UNSET:-default-value }}",
			expected: "default-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := r.resolveString(tt.input)
			require.NoError(t, r.Error())
			assert.Equal(t, tt.expected, val)
		})
	}
}
