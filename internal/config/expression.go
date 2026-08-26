package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

const MaxDirectiveFileSize = 1024 * 1024 // 1MB

type fileCacheEntry struct {
	content     string
	err         error
	isSizeLimit bool
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
func scanAnchors(s string, buf []anchorRange) []anchorRange {
	if !strings.Contains(s, "{{") {
		return nil
	}

	var stackBuf [16]int
	var allPairsBuf [16]anchorRange
	stack := stackBuf[:0]
	allPairs := allPairsBuf[:0]

	for i := 0; i < len(s)-1; {
		if s[i] == '{' && s[i+1] == '{' {
			stack = append(stack, i)
			i += 2
		} else if s[i] == '}' && s[i+1] == '}' {
			if len(stack) > 0 {
				start := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				allPairs = append(allPairs, anchorRange{start: start, end: i + 2})
			}
			i += 2
		} else {
			i++
		}
	}

	if len(allPairs) == 0 {
		return nil
	}

	res := buf[:0]
	lastStart := len(s) + 1
	for _, p := range slices.Backward(allPairs) {
		if p.end <= lastStart {
			res = append(res, p)
			lastStart = p.start
		}
	}

	// Reverse to maintain original order
	slices.Reverse(res)

	return res
}

// ExpressionResolver handles resolution of {{...}} expressions and tilde expansion in config values.
type ExpressionResolver struct {
	fs          FileSystem
	Home        string
	Pwd         string
	HostContext *HostContext
	shared      atomic.Pointer[resolverSharedState]
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
	r.ensureShared()
	res := &ExpressionResolver{
		fs:          r.fs,
		Home:        r.Home,
		Pwd:         r.Pwd,
		HostContext: nil,
		err:         r.err,
	}
	res.shared.Store(r.shared.Load())
	return res
}

func (r *ExpressionResolver) ensureShared() {
	if r.shared.Load() == nil {
		ns := &resolverSharedState{}
		r.shared.CompareAndSwap(nil, ns)
	}
}

func (r *ExpressionResolver) getShared() *resolverSharedState {
	if r == nil {
		return nil
	}
	r.ensureShared()
	return r.shared.Load()
}

func (r *ExpressionResolver) ensureFileCache() {
	shared := r.getShared()
	shared.cacheOnce.Do(func() {
		shared.fileCache = make(map[string]fileCacheEntry)
		shared.statCache = make(map[string]statCacheEntry)
	})
}

func (r *ExpressionResolver) ensureLoader() {
	shared := r.getShared()
	shared.loaderOnce.Do(func() {
		if shared.loader == nil {
			shared.loader = NewConfigLoaderWithFS(r.fs)
		}
	})
}

func (r *ExpressionResolver) setError(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *ExpressionResolver) Stat(name string) (os.FileInfo, error) {
	r.ensureFileCache()
	shared := r.getShared()

	shared.mu.RLock()
	cached, ok := shared.statCache[name]
	shared.mu.RUnlock()
	if ok {
		return cached.info, cached.err
	}

	info, err := r.fs.Stat(name)

	shared.mu.Lock()
	shared.statCache[name] = statCacheEntry{info: info, err: err}
	shared.mu.Unlock()

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
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			val[k] = r.Resolve(val[k])
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
		if len(s) >= 4 && s[0] == '{' && s[1] == '{' && s[len(s)-2] == '}' && s[len(s)-1] == '}' {
			content := strings.TrimSpace(s[2 : len(s)-2])
			if !strings.Contains(content, "{{") && !strings.Contains(content, "}}") { // No nested expressions
				res, err := r.resolveDirective(content)
				if err == nil && (len(res) < 2 || res[0] != '{' || res[1] != '{') {
					resolved = res
					hasExpr = false // Resolved
				}
			}
		}

		if hasExpr {
			var anchorBuf [16]anchorRange
			ranges := scanAnchors(s, anchorBuf[:0])
			if len(ranges) > 0 {
				var arr [2048]byte
				buf := arr[:0]
				last := 0
				sbInitialized := false
				for _, rng := range ranges {
					if r.err != nil {
						if sbInitialized {
							buf = append(buf, s[last:rng.start]...)
							buf = append(buf, s[rng.start:rng.end]...)
						}
					} else {
						content := strings.TrimSpace(s[rng.start+2 : rng.end-2])

						var res string
						var err error

						// Escape mechanism: {{{{...}}}} -> {{...}}
						if len(content) >= 4 && content[0] == '{' && content[1] == '{' && content[len(content)-2] == '}' && content[len(content)-1] == '}' {
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
							if sbInitialized {
								buf = append(buf, s[last:rng.start]...)
								buf = append(buf, s[rng.start:rng.end]...)
							}
						} else {
							if !sbInitialized {
								// Bounded allowance (16 bytes per range or expansion delta) so short ranges fit within 512B stack buffer
								expansion := len(res) - (rng.end - rng.start)
								if expansion < 16 {
									expansion = 16
								}
								needed := len(s) + len(ranges)*expansion
								if needed > len(arr) {
									buf = make([]byte, 0, needed)
								}
								buf = append(buf, s[:rng.start]...)
								sbInitialized = true
							} else {
								buf = append(buf, s[last:rng.start]...)
							}
							buf = append(buf, res...)
						}
					}
					last = rng.end
				}
				if sbInitialized {
					buf = append(buf, s[last:]...)
					resolved = string(buf)
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
	if isMagicWordCandidate || strings.IndexByte(content, ':') >= 0 {
		return "", fmt.Errorf("unknown directive or magic word: %q", content)
	}

	return "{{" + content + "}}", nil // Keep as is if it doesn't look like a directive
}

func (r *ExpressionResolver) resolveFile(input string) (string, error) {
	filename, defaultValue, hasDefault := strings.Cut(input, ":-")

	if err := validatePathChars(filename); err != nil {
		return "", fmt.Errorf("invalid character in file directive parameter: %w", err)
	}
	if filename == "." || filename == ".." || HasParentTraversal(filename) || filepath.IsAbs(filename) || !filepath.IsLocal(filename) || strings.ContainsAny(filename, "/\\") {
		return "", fmt.Errorf("only a single file name is allowed in file directive: %q", filename)
	}

	r.ensureFileCache()
	shared := r.getShared()
	shared.mu.RLock()
	cached, ok := shared.fileCache[filename]
	shared.mu.RUnlock()

	if ok {
		if cached.err != nil {
			if cached.isSizeLimit || !hasDefault {
				return "", cached.err
			}
			if err := validatePathChars(defaultValue); err != nil {
				return "", fmt.Errorf("security validation failed for file directive default value: %w", err)
			}
			return defaultValue, nil
		}
		if cached.content == "" && hasDefault {
			if err := validatePathChars(defaultValue); err != nil {
				return "", fmt.Errorf("security validation failed for file directive default value: %w", err)
			}
			return defaultValue, nil
		}
		return cached.content, nil
	}

	r.ensureLoader()
	shared = r.getShared()
	paths := shared.loader.FindConfigs(filename)
	if len(paths) == 0 {
		err := fmt.Errorf("file not found: %q", filename)
		shared.mu.Lock()
		shared.fileCache[filename] = fileCacheEntry{err: err}
		shared.mu.Unlock()
		if hasDefault {
			if err := validatePathChars(defaultValue); err != nil {
				return "", fmt.Errorf("security validation failed for file directive default value: %w", err)
			}
			return defaultValue, nil
		}
		return "", err
	}

	info, err := r.Stat(paths[0])
	if err != nil {
		wrappedErr := fmt.Errorf("failed to stat file %q: %w", paths[0], err)
		shared.mu.Lock()
		shared.fileCache[filename] = fileCacheEntry{err: wrappedErr}
		shared.mu.Unlock()
		return "", wrappedErr
	}

	if info.Size() > MaxDirectiveFileSize {
		err := fmt.Errorf("file %q is too large (%d bytes, max %d)", paths[0], info.Size(), MaxDirectiveFileSize)
		shared.mu.Lock()
		shared.fileCache[filename] = fileCacheEntry{err: err, isSizeLimit: true}
		shared.mu.Unlock()
		return "", err
	}

	data, err := r.fs.ReadFile(paths[0])
	if err != nil {
		wrappedErr := fmt.Errorf("failed to read file %q: %w", paths[0], err)
		shared.mu.Lock()
		shared.fileCache[filename] = fileCacheEntry{err: wrappedErr}
		shared.mu.Unlock()
		if hasDefault {
			if err := validatePathChars(defaultValue); err != nil {
				return "", fmt.Errorf("security validation failed for file directive default value: %w", err)
			}
			return defaultValue, nil
		}
		return "", wrappedErr
	}

	if int64(len(data)) > MaxDirectiveFileSize {
		err := fmt.Errorf("file %q is too large (%d bytes, max %d)", paths[0], len(data), MaxDirectiveFileSize)
		shared.mu.Lock()
		shared.fileCache[filename] = fileCacheEntry{err: err, isSizeLimit: true}
		shared.mu.Unlock()
		return "", err
	}

	result := strings.TrimSpace(string(data))
	shared.mu.Lock()
	shared.fileCache[filename] = fileCacheEntry{content: result}
	shared.mu.Unlock()

	if result == "" && hasDefault {
		if err := validatePathChars(defaultValue); err != nil {
			return "", fmt.Errorf("security validation failed for file directive default value: %w", err)
		}
		return defaultValue, nil
	}

	return result, nil
}

func (r *ExpressionResolver) resolveFindDir(input string) (string, error) {
	name, defaultValue, hasDefault := strings.Cut(input, ":-")
	if err := validatePathChars(name); err != nil {
		return "", fmt.Errorf("invalid character in find_dir directive parameter: %w", err)
	}
	if name == "." || name == ".." || HasParentTraversal(name) || filepath.IsAbs(name) || !filepath.IsLocal(name) || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("only a single file or directory name is allowed in find_dir directive: %q", name)
	}

	r.ensureLoader()
	shared := r.getShared()
	paths := shared.loader.FindConfigs(name)
	if len(paths) == 0 {
		if hasDefault {
			if err := validatePathChars(defaultValue); err != nil {
				return "", fmt.Errorf("security validation failed for find_dir directive default value: %w", err)
			}
			return defaultValue, nil
		}
		return "", fmt.Errorf("item not found for find_dir: %q", name)
	}

	dir := filepath.Dir(paths[0])
	absDir, err := r.fs.Abs(dir)
	if err != nil {
		if hasDefault {
			if err := validatePathChars(defaultValue); err != nil {
				return "", fmt.Errorf("security validation failed for find_dir directive default value: %w", err)
			}
			return defaultValue, nil
		}
		return "", fmt.Errorf("failed to get absolute path for %q: %w", dir, err)
	}

	return r.applyReverseResolution(absDir)
}

// resolveEnv returns the value of an environment variable.
// It mirrors os.Getenv behavior and returns an empty string if the key is missing.
// It supports default value syntax: {{env:KEY:-default}}.
func (r *ExpressionResolver) resolveEnv(input string) (string, error) {
	key, defaultValue, hasDefault := strings.Cut(input, ":-")
	if err := ValidateEnvKey(key); err != nil {
		return "", fmt.Errorf("security validation failed for env directive key: %w", err)
	}
	val := r.fs.Getenv(key)
	if hasDefault && val == "" {
		if err := validatePathChars(defaultValue); err != nil {
			return "", fmt.Errorf("security validation failed for env directive default value: %w", err)
		}
		return defaultValue, nil
	}
	if err := validatePathChars(val); err != nil {
		return "", fmt.Errorf("security validation failed for env directive value: %w", err)
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
