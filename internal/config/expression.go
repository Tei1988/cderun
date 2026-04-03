package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	exprRegex   = regexp.MustCompile(`\{\{([^}]+)\}\}`)
	envKeyRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

const MaxDirectiveFileSize = 1024 * 1024 // 1MB

type fileCacheEntry struct {
	content string
	err     error
}

// ExpressionResolver handles resolution of {{...}} expressions and tilde expansion in config values.
type ExpressionResolver struct {
	fs          FileSystem
	Home        string
	Pwd         string
	HostContext *HostContext
	fileCache   map[string]fileCacheEntry
	loader      *ConfigLoader
	err         error
}

func NewExpressionResolver(hostCtx *HostContext) (*ExpressionResolver, error) {
	return NewExpressionResolverWithFS(hostCtx, RealFileSystem{})
}

func NewExpressionResolverWithFS(hostCtx *HostContext, fs FileSystem) (*ExpressionResolver, error) {
	home, err := fs.UserHomeDir()
	if err != nil {
		home = ""
	}
	pwd, err := fs.Getwd()
	if err != nil {
		pwd = ""
	}
	return &ExpressionResolver{
		fs:          fs,
		Home:        home,
		Pwd:         pwd,
		HostContext: hostCtx,
		fileCache:   make(map[string]fileCacheEntry),
		loader:      NewConfigLoaderWithFS(fs),
	}, nil
}

// Error returns the first error encountered during expression resolution.
func (r *ExpressionResolver) Error() error {
	return r.err
}

// WithoutHostContext returns a shallow copy of the resolver with HostContext set to nil.
// Use this when resolving container-side paths (e.g. mount targets) that should not
// undergo reverse path resolution.
func (r *ExpressionResolver) WithoutHostContext() *ExpressionResolver {
	clone := *r
	clone.HostContext = nil
	return &clone
}

func (r *ExpressionResolver) setError(err error) {
	if r.err == nil {
		r.err = err
	}
}

// Resolve processes a value (string, slice, or map) and resolves any expressions or tilde expansion found.
func (r *ExpressionResolver) Resolve(v any) any {
	if r.err != nil {
		return v
	}
	switch val := v.(type) {
	case string:
		return r.resolveString(val)
	case []any:
		for i, item := range val {
			val[i] = r.Resolve(item)
		}
		return val
	case map[string]any:
		for k, v := range val {
			val[k] = r.Resolve(v)
		}
		return val
	default:
		return v
	}
}

// ResolveString resolves expressions and tilde expansion in a string.
func (r *ExpressionResolver) ResolveString(s string) (string, error) {
	res := r.resolveString(s)
	return res, r.err
}

func (r *ExpressionResolver) resolveString(s string) string {
	if r.err != nil || s == "" {
		return s
	}

	hasExpr := strings.Contains(s, "{{")

	// Fast-path: no expressions and no tilde expansion
	if !hasExpr && !strings.HasPrefix(s, "~") {
		return s
	}

	// 1. Resolve {{...}} expressions
	resolved := s
	if hasExpr {
		resolved = exprRegex.ReplaceAllStringFunc(s, func(match string) string {
			if r.err != nil {
				return match
			}
			content := strings.TrimSpace(match[2 : len(match)-2])

			// Special case: if content is empty (e.g. {{ }}), resolveDirective will be skipped or handle it
			if content == "" {
				return match
			}

			res, err := r.resolveDirective(content)
			if err != nil {
				r.setError(err)
				return match
			}
			return res
		})
	}

	if r.err != nil {
		return resolved
	}

	// 2. Expand ~ if it's at the beginning
	if strings.HasPrefix(resolved, "~") {
		expanded, err := expandHome(resolved, r.fs)
		if err != nil {
			r.setError(err)
			return resolved
		}
		return expanded
	}

	return resolved
}

func (r *ExpressionResolver) resolveDirective(content string) (string, error) {
	// 1. Magic Words
	switch content {
	case "HOME":
		return r.Home, nil
	case "PWD":
		return r.Pwd, nil
	case "BASE_HOME":
		if r.HostContext != nil && r.HostContext.HomeDir != "" {
			return r.HostContext.HomeDir, nil
		}
		return r.Home, nil
	case "BASE_PWD":
		if r.HostContext != nil && r.HostContext.WorkingDir != "" {
			return r.HostContext.WorkingDir, nil
		}
		return r.Pwd, nil
	}

	// 2. Directives
	if after, ok := strings.CutPrefix(content, "file:"); ok {
		return r.resolveFile(strings.TrimSpace(after))
	}

	if after, ok := strings.CutPrefix(content, "find_dir:"); ok {
		return r.resolveFindDir(strings.TrimSpace(after))
	}

	if after, ok := strings.CutPrefix(content, "env:"); ok {
		return r.resolveEnv(strings.TrimSpace(after))
	}

	return "{{" + content + "}}", nil // Keep as is if unknown
}

func (r *ExpressionResolver) resolveFile(filename string) (string, error) {
	if filepath.IsAbs(filename) || !filepath.IsLocal(filename) {
		return "", fmt.Errorf("absolute paths and parent directory references are not allowed in file directive: %s", filename)
	}

	if cached, ok := r.fileCache[filename]; ok {
		return cached.content, cached.err
	}

	paths := r.loader.FindConfigs(filename)
	if len(paths) == 0 {
		err := fmt.Errorf("file not found: %s", filename)
		r.fileCache[filename] = fileCacheEntry{err: err}
		return "", err
	}

	info, err := r.fs.Stat(paths[0])
	if err != nil {
		wrappedErr := fmt.Errorf("failed to stat file %s: %w", paths[0], err)
		r.fileCache[filename] = fileCacheEntry{err: wrappedErr}
		return "", wrappedErr
	}

	if info.Size() > MaxDirectiveFileSize {
		err := fmt.Errorf("file %s is too large (%d bytes, max %d)", paths[0], info.Size(), MaxDirectiveFileSize)
		r.fileCache[filename] = fileCacheEntry{err: err}
		return "", err
	}

	data, err := r.fs.ReadFile(paths[0])
	if err != nil {
		wrappedErr := fmt.Errorf("failed to read file %s: %w", paths[0], err)
		r.fileCache[filename] = fileCacheEntry{err: wrappedErr}
		return "", wrappedErr
	}

	result := strings.TrimSpace(string(data))
	r.fileCache[filename] = fileCacheEntry{content: result}
	return result, nil
}

func (r *ExpressionResolver) resolveFindDir(name string) (string, error) {
	if filepath.IsAbs(name) || !filepath.IsLocal(name) {
		return "", fmt.Errorf("absolute paths and parent directory references are not allowed in find_dir directive: %s", name)
	}

	paths := r.loader.FindConfigs(name)
	if len(paths) == 0 {
		return "", fmt.Errorf("item not found for find_dir: %s", name)
	}

	dir := filepath.Dir(paths[0])

	rel, err := filepath.Rel(r.Pwd, dir)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path for %s: %w", dir, err)
	}

	return rel, nil
}

// resolveEnv returns the value of an environment variable.
// It mirrors os.Getenv behavior and returns an empty string if the key is missing.
// It supports default value syntax: {{env:KEY:-default}}.
func (r *ExpressionResolver) resolveEnv(input string) (string, error) {
	key, defaultValue, hasDefault := strings.Cut(input, ":-")
	if !envKeyRegex.MatchString(key) {
		return "", fmt.Errorf("invalid environment variable key: %q", key)
	}
	val := r.fs.Getenv(key)
	if hasDefault && val == "" {
		return defaultValue, nil
	}
	return val, nil
}
