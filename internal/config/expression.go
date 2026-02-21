package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var exprRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// ExpressionResolver handles resolution of {{...}} expressions in config values.
type ExpressionResolver struct {
	fs          FileSystem
	Home        string
	Pwd         string
	HostContext *HostContext
	fileCache   map[string]string
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
		fileCache:   make(map[string]string),
		loader: &ConfigLoader{
			fs:              fs,
			systemConfigDir: defaultLoader.systemConfigDir,
			runConfigDir:    defaultLoader.runConfigDir,
		},
	}, nil
}

// Error returns the first error encountered during expression resolution.
func (r *ExpressionResolver) Error() error {
	return r.err
}

func (r *ExpressionResolver) setError(err error) {
	if r.err == nil {
		r.err = err
	}
}

// Resolve processes a value (string, slice, or map) and resolves any expressions found.
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

func (r *ExpressionResolver) resolveString(s string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	return exprRegex.ReplaceAllStringFunc(s, func(match string) string {
		if r.err != nil {
			return match
		}
		content := strings.TrimSpace(match[2 : len(match)-2])

		resolved, err := r.resolveDirective(content)
		if err != nil {
			r.setError(err)
			return match
		}
		return resolved
	})
}

func (r *ExpressionResolver) resolveDirective(content string) (string, error) {
	// 1. Magic Words
	switch content {
	case "HOME":
		return r.Home, nil
	case "PWD":
		return r.Pwd, nil
	}

	// 2. Directives
	if after, ok := strings.CutPrefix(content, "file:"); ok {
		return r.resolveFile(after)
	}

	if after, ok := strings.CutPrefix(content, "find_dir:"); ok {
		return r.resolveFindDir(after)
	}

	return "{{" + content + "}}", nil // Keep as is if unknown
}

func (r *ExpressionResolver) resolveFile(filename string) (string, error) {
	// Security: Prevent path traversal by disallowing absolute paths or parent directory references.
	// Since FindConfigs searches parent directories automatically, ".." is not needed for legitimate use.
	if filepath.IsAbs(filename) || strings.Contains(filename, "..") {
		return "", fmt.Errorf("absolute paths and parent directory references are not allowed in file directive: %s", filename)
	}

	if cached, ok := r.fileCache[filename]; ok {
		if cached == "" && r.err == nil {
			// This was a failed attempt previously cached as empty string
			// Re-finding to get a proper error message if needed, or just error
			return "", fmt.Errorf("file not found: %s", filename)
		}
		return cached, nil
	}

	paths := r.loader.FindConfigs(filename)
	if len(paths) == 0 {
		r.fileCache[filename] = ""
		return "", fmt.Errorf("file not found: %s", filename)
	}

	// Use the highest priority file
	data, err := r.fs.ReadFile(paths[0])
	if err != nil {
		r.fileCache[filename] = ""
		return "", fmt.Errorf("failed to read file %s: %w", paths[0], err)
	}

	result := strings.TrimSpace(string(data))
	r.fileCache[filename] = result
	return result, nil
}

func (r *ExpressionResolver) resolveFindDir(name string) (string, error) {
	if filepath.IsAbs(name) || strings.Contains(name, "..") {
		return "", fmt.Errorf("absolute paths and parent directory references are not allowed in find_dir directive: %s", name)
	}

	paths := r.loader.FindConfigs(name)
	if len(paths) == 0 {
		return "", fmt.Errorf("item not found for find_dir: %s", name)
	}

	// paths[0] is the full path to the found item
	dir := filepath.Dir(paths[0])

	rel, err := filepath.Rel(r.Pwd, dir)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path for %s: %w", dir, err)
	}

	return rel, nil
}
