package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const MaxDirectiveFileSize = 1024 * 1024 // 1MB

type fileCacheEntry struct {
	content string
	err     error
}

type statCacheEntry struct {
	info os.FileInfo
	err  error
}

type anchorRange struct {
	start int // inclusive index of the first '{'
	end   int // exclusive index after the last '}'
}

// scanAnchors finds all top-level matched {{...}} expression ranges in a string.
// It handles nested braces and treats unmatched openers as literal text.
// It performs a single-pass scan to ensure O(n) complexity.
func scanAnchors(s string) []anchorRange {
	if !strings.Contains(s, "{{") {
		return nil
	}

	var stackBuf [8]int
	var allPairsBuf [8]anchorRange
	stack := stackBuf[:0]
	allPairs := allPairsBuf[:0]

	for i := 0; i < len(s)-1; i++ {
		if s[i] == '{' && s[i+1] == '{' {
			stack = append(stack, i)
			i++
		} else if s[i] == '}' && s[i+1] == '}' {
			if len(stack) > 0 {
				start := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				allPairs = append(allPairs, anchorRange{start: start, end: i + 2})
			}
			i++
		}
	}

	if len(allPairs) == 0 {
		return nil
	}

	// Since allPairs are collected in order of their closing braces, we can
	// identify top-level (outermost) ranges in a single backward pass.
	var resBuf [8]anchorRange
	res := resBuf[:0]
	lastStart := len(s) + 1
	for i := len(allPairs) - 1; i >= 0; i-- {
		p := allPairs[i]
		if p.end <= lastStart {
			res = append(res, p)
			lastStart = p.start
		}
	}

	finalRes := make([]anchorRange, len(res))
	copy(finalRes, res)

	// Reverse to maintain original order
	for i, j := 0, len(finalRes)-1; i < j; i, j = i+1, j-1 {
		finalRes[i], finalRes[j] = finalRes[j], finalRes[i]
	}

	return finalRes
}

// ExpressionResolver handles resolution of {{...}} expressions and tilde expansion in config values.
type ExpressionResolver struct {
	fs          FileSystem
	Home        string
	Pwd         string
	HostContext *HostContext
	shared      *resolverSharedState
	err         error
}

type resolverSharedState struct {
	mu         sync.RWMutex
	fileCache  map[string]fileCacheEntry
	statCache  map[string]statCacheEntry
	loader     *ConfigLoader
	cacheOnce  sync.Once
	loaderOnce sync.Once
}

func NewExpressionResolver(hostCtx *HostContext) (*ExpressionResolver, error) {
	return NewExpressionResolverWithFS(hostCtx, RealFileSystem{})
}

func NewExpressionResolverWithFS(hostCtx *HostContext, fs FileSystem) (*ExpressionResolver, error) {
	home, err := fs.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	pwd, err := fs.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	return &ExpressionResolver{
		fs:          fs,
		Home:        home,
		Pwd:         pwd,
		HostContext: hostCtx,
		shared:      &resolverSharedState{},
	}, nil
}

// Error returns the first error encountered during expression resolution.
func (r *ExpressionResolver) Error() error {
	return r.err
}

// WithoutHostContext returns a new instance of the resolver with HostContext set to nil.
// Use this when resolving container-side paths (e.g. mount targets) that should not
// undergo reverse path resolution.
func (r *ExpressionResolver) WithoutHostContext() *ExpressionResolver {
	return &ExpressionResolver{
		fs:          r.fs,
		Home:        r.Home,
		Pwd:         r.Pwd,
		HostContext: nil,
		shared:      r.shared,
		err:         r.err,
	}
}

func (r *ExpressionResolver) ensureFileCache() {
	r.shared.cacheOnce.Do(func() {
		if r.shared.fileCache == nil {
			r.shared.fileCache = make(map[string]fileCacheEntry)
		}
		if r.shared.statCache == nil {
			r.shared.statCache = make(map[string]statCacheEntry)
		}
	})
}

func (r *ExpressionResolver) ensureLoader() {
	r.shared.loaderOnce.Do(func() {
		if r.shared.loader == nil {
			r.shared.loader = NewConfigLoaderWithFS(r.fs)
		}
	})
}

func (r *ExpressionResolver) setError(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *ExpressionResolver) Stat(name string) (os.FileInfo, error) {
	if r.shared == nil {
		return r.fs.Stat(name)
	}
	r.ensureFileCache()

	r.shared.mu.RLock()
	cached, ok := r.shared.statCache[name]
	r.shared.mu.RUnlock()
	if ok {
		return cached.info, cached.err
	}

	info, err := r.fs.Stat(name)

	r.shared.mu.Lock()
	r.shared.statCache[name] = statCacheEntry{info: info, err: err}
	r.shared.mu.Unlock()

	return info, err
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
	hasTilde := strings.HasPrefix(s, "~")

	// Fast-path: no expressions and no tilde expansion
	if !hasExpr && !hasTilde {
		return s
	}

	// 1. Resolve {{...}} expressions
	resolved := s
	if hasExpr {
		// Optimization: handle exact matches of magic words or simple directives (e.g. "{{HOME}}", "{{env:KEY}}")
		if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
			content := strings.TrimSpace(s[2 : len(s)-2])
			if !strings.Contains(content, "{{") && !strings.Contains(content, "}}") { // No nested expressions
				res, err := r.resolveDirective(content)
				if err == nil && !strings.HasPrefix(res, "{{") {
					resolved = res
					hasExpr = false // Resolved
				}
			}
		}

		if hasExpr {
			ranges := scanAnchors(s)
			if len(ranges) > 0 {
				var sb strings.Builder
				last := 0
				sbInitialized := false
				for _, rng := range ranges {
					if r.err != nil {
						if sbInitialized {
							sb.WriteString(s[last:rng.start])
							sb.WriteString(s[rng.start:rng.end])
						}
					} else {
						if !sbInitialized {
							sb.Grow(len(s))
							sb.WriteString(s[:rng.start])
							sbInitialized = true
						} else {
							sb.WriteString(s[last:rng.start])
						}
						content := strings.TrimSpace(s[rng.start+2 : rng.end-2])

						var res string
						var err error

						// Escape mechanism: {{{{...}}}} -> {{...}}
						if strings.HasPrefix(content, "{{") && strings.HasSuffix(content, "}}") {
							res = content
						} else {
							// Resolve content first to support nested expressions like {{env:{{VAR}}}} or {{env:DIR:-{{HOME}}}}
							resolvedContent := content
							if strings.Contains(content, "{{") {
								resolvedContent = r.resolveString(content)
							}
							res, err = r.resolveDirective(resolvedContent)
						}

						if err != nil {
							r.setError(err)
							sb.WriteString(s[rng.start:rng.end])
						} else {
							sb.WriteString(res)
						}
					}
					last = rng.end
				}
				if sbInitialized {
					sb.WriteString(s[last:])
					resolved = sb.String()
				}
			}
		}
	}

	if r.err != nil {
		return resolved
	}

	// 2. Expand ~ if it's at the beginning
	if strings.HasPrefix(resolved, "~") {
		if resolved == "~" {
			return r.Home
		}
		if strings.HasPrefix(resolved, "~/") {
			return filepath.Join(r.Home, resolved[2:])
		}
		// Fallback for cases like ~user (though not explicitly supported as primary feature)
		return expandHome(resolved, r.Home)
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
		return r.resolveFile(after)
	}

	if after, ok := strings.CutPrefix(content, "find_dir:"); ok {
		return r.resolveFindDir(after)
	}

	if after, ok := strings.CutPrefix(content, "env:"); ok {
		return r.resolveEnv(after)
	}

	// Strict resolution (T11): error on unknown directives or magic words.
	// Heuristic: ALL_UPPER (magic word style) or contains ":" (directive style).
	isMagicWordCandidate := len(content) > 0
	for i := 0; i < len(content); i++ {
		c := content[i]
		if (c < 'A' || c > 'Z') && c != '_' && (c < '0' || c > '9') {
			isMagicWordCandidate = false
			break
		}
	}
	if isMagicWordCandidate || strings.Contains(content, ":") {
		return "", fmt.Errorf("unknown directive or magic word: %q", content)
	}

	return "{{" + content + "}}", nil // Keep as is if it doesn't look like a directive
}

func (r *ExpressionResolver) resolveFile(filename string) (string, error) {
	if filepath.IsAbs(filename) || !filepath.IsLocal(filename) {
		return "", fmt.Errorf("absolute paths and parent directory references are not allowed in file directive: %q", filename)
	}

	if r.shared == nil {
		// Fallback for file directive requires shared loader state
		return "", fmt.Errorf("file directive resolution requires a resolver with shared state")
	}

	r.ensureFileCache()
	r.shared.mu.RLock()
	cached, ok := r.shared.fileCache[filename]
	r.shared.mu.RUnlock()
	if ok {
		return cached.content, cached.err
	}

	r.ensureLoader()
	paths := r.shared.loader.FindConfigs(filename)
	if len(paths) == 0 {
		err := fmt.Errorf("file not found: %q", filename)
		r.shared.mu.Lock()
		r.shared.fileCache[filename] = fileCacheEntry{err: err}
		r.shared.mu.Unlock()
		return "", err
	}

	info, err := r.Stat(paths[0])
	if err != nil {
		wrappedErr := fmt.Errorf("failed to stat file %q: %w", paths[0], err)
		r.shared.mu.Lock()
		r.shared.fileCache[filename] = fileCacheEntry{err: wrappedErr}
		r.shared.mu.Unlock()
		return "", wrappedErr
	}

	if info.Size() > MaxDirectiveFileSize {
		err := fmt.Errorf("file %q is too large (%d bytes, max %d)", paths[0], info.Size(), MaxDirectiveFileSize)
		r.shared.mu.Lock()
		r.shared.fileCache[filename] = fileCacheEntry{err: err}
		r.shared.mu.Unlock()
		return "", err
	}

	data, err := r.fs.ReadFile(paths[0])
	if err != nil {
		wrappedErr := fmt.Errorf("failed to read file %q: %w", paths[0], err)
		r.shared.mu.Lock()
		r.shared.fileCache[filename] = fileCacheEntry{err: wrappedErr}
		r.shared.mu.Unlock()
		return "", wrappedErr
	}

	if int64(len(data)) > MaxDirectiveFileSize {
		err := fmt.Errorf("file %q is too large (%d bytes, max %d)", paths[0], len(data), MaxDirectiveFileSize)
		r.shared.mu.Lock()
		r.shared.fileCache[filename] = fileCacheEntry{err: err}
		r.shared.mu.Unlock()
		return "", err
	}

	result := strings.TrimSpace(string(data))
	r.shared.mu.Lock()
	r.shared.fileCache[filename] = fileCacheEntry{content: result}
	r.shared.mu.Unlock()
	return result, nil
}

func (r *ExpressionResolver) resolveFindDir(name string) (string, error) {
	if filepath.IsAbs(name) || !filepath.IsLocal(name) || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("only a single file or directory name is allowed in find_dir directive: %q", name)
	}

	r.ensureLoader()
	paths := r.shared.loader.FindConfigs(name)
	if len(paths) == 0 {
		return "", fmt.Errorf("item not found for find_dir: %q", name)
	}

	dir := filepath.Dir(paths[0])
	absDir, err := r.fs.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %q: %w", dir, err)
	}

	return r.applyReverseResolution(absDir)
}

// resolveEnv returns the value of an environment variable.
// It mirrors os.Getenv behavior and returns an empty string if the key is missing.
// It supports default value syntax: {{env:KEY:-default}}.
func (r *ExpressionResolver) resolveEnv(input string) (string, error) {
	key, defaultValue, hasDefault := strings.Cut(input, ":-")
	val := r.fs.Getenv(key)
	if hasDefault && val == "" {
		return defaultValue, nil
	}
	return val, nil
}

func (r *ExpressionResolver) applyReverseResolution(absPath string) (string, error) {
	if r.HostContext == nil || r.HostContext.Level == 0 {
		return absPath, nil
	}

	abs := absPath
	if !filepath.IsAbs(abs) {
		a, err := r.fs.Abs(abs)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for %q: %w", abs, err)
		}
		abs = a
	}

	found := false
	bestRel := ""
	bestSource := ""
	bestTarget := ""
	maxLevel := -1

	for _, m := range r.HostContext.Mounts {
		rel, err := filepath.Rel(m.Target, abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if len(m.Target) > len(bestTarget) || (len(m.Target) == len(bestTarget) && m.Level > maxLevel) {
				maxLevel = m.Level
				bestTarget = m.Target
				bestSource = m.Source
				bestRel = rel
				found = true
			}
		}
	}

	if found {
		return filepath.Join(bestSource, bestRel), nil
	}
	return abs, nil
}
