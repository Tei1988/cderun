package config

import (
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

// Resolve processes a value (string, slice, or map) and resolves any expressions found.
func (r *ExpressionResolver) Resolve(v any) any {
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
		content := strings.TrimSpace(match[2 : len(match)-2])

		// 1. Magic Words
		switch content {
		case "HOME":
			return r.Home
		case "PWD":
			return r.Pwd
		}

		// 2. Directives
		if after, ok := strings.CutPrefix(content, "file:"); ok {
			filename := after
			return r.resolveFile(filename)
		}

		return match // Keep as is if unknown
	})
}

func (r *ExpressionResolver) resolveFile(filename string) string {
	// Security: Prevent path traversal by disallowing absolute paths or parent directory references.
	// Since FindConfigs searches parent directories automatically, ".." is not needed for legitimate use.
	if filepath.IsAbs(filename) || strings.Contains(filename, "..") {
		return ""
	}

	if cached, ok := r.fileCache[filename]; ok {
		return cached
	}

	paths := r.loader.FindConfigs(filename)
	if len(paths) == 0 {
		r.fileCache[filename] = ""
		return ""
	}

	// Use the highest priority file
	data, err := r.fs.ReadFile(paths[0])
	if err != nil {
		r.fileCache[filename] = ""
		return ""
	}

	result := strings.TrimSpace(string(data))
	r.fileCache[filename] = result
	return result
}
