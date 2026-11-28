package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docs/features/value-resolution.md: Directive Fallback Syntax & Rules
func TestUnit_Expression_LazyFallback(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		input string
		want  string
		err   string
	}{
		{name: "env skips fallback", input: "{{env:SET:-{{UNKNOWN}}}}", want: "present"},
		{name: "file skips fallback", input: "{{file:version:-{{UNKNOWN}}}}", want: "1.2.3"},
		{name: "find_dir skips fallback", input: "{{find_dir:version:-{{UNKNOWN}}}}", want: "/work"},
		{name: "selected fallback error", input: "{{env:UNSET:-{{UNKNOWN}}}}", err: "unknown directive or magic word"},
		{name: "invalid primary skips fallback", input: "{{find_dir:../invalid:-{{UNKNOWN}}}}", err: "only a single file or directory name"},
		{name: "nested primary with fallback", input: "{{env:{{env:UNSET:-SET}}:-{{UNKNOWN}}}}", want: "present"},
		{name: "escaped default", input: "{{env:UNSET:-{{{{UNKNOWN}}}}}}", want: "{{UNKNOWN}}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := &MockFileSystem{
				WD:    "/work",
				Env:   map[string]string{"SET": "present"},
				Files: map[string][]byte{"/work/version": []byte("1.2.3")},
			}
			r, err := NewExpressionResolverWithFS(nil, fs)
			require.NoError(t, err)
			got, err := r.ResolveString(tt.input)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// docs/features/value-resolution.md: Nested Expressions
func TestUnit_Expression_MountSourceFallbackChain(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		wd     string
		dirs   map[string]bool
		files  map[string][]byte
		base   string
		nested string
	}{
		{
			name: "worktree",
			wd:   "/work/a/feat/sub",
			dirs: map[string]bool{"/work/a/main": true, "/work/a/main/.git": true},
			files: map[string][]byte{
				"/work/a/feat/.git": []byte("gitdir: ../main/.git/worktrees/feat"),
			},
			base:   "/work/a",
			nested: "/host/work/a",
		},
		{
			name:   "standalone repository",
			wd:     "/work/b/sub",
			dirs:   map[string]bool{"/work/b/.git": true},
			base:   "/work/b",
			nested: "/host/work/b",
		},
		{
			name:   "no repository",
			wd:     "/work/c/sub",
			base:   "/work/c/sub",
			nested: "/host/base-pwd",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, level := range []struct {
				name string
				host *HostContext
				want string
			}{
				{name: "base host", want: tt.base},
				{
					name: "nested",
					host: &HostContext{
						Level:      1,
						WorkingDir: "/host/base-pwd",
						Mounts:     []MountMapping{{Source: "/host/work", Target: "/work"}},
					},
					want: tt.nested,
				},
			} {
				t.Run(level.name, func(t *testing.T) {
					fs := &MockFileSystem{WD: tt.wd, Dirs: tt.dirs, Files: tt.files}
					r, err := NewExpressionResolverWithFS(level.host, fs)
					require.NoError(t, err)
					mount := MountConfig{
						Source: ConfigPath{Raw: "{{find_dir:main:-{{find_dir:.git:-{{BASE_PWD}}}}}}"},
						Target: ConfigPath{Raw: "/workspace"},
					}
					resolved, err := mount.Resolve(r)
					require.NoError(t, err)
					assert.Equal(t, level.want, resolved.Source)
					if tt.name == "worktree" {
						assert.NotContains(t, fs.StatCalls, "/work/a/feat/.git")
					}
				})
			}
		})
	}
}
