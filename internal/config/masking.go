package config

import (
	"path"
	"strings"
)

// MaskSensitiveEnv redacts sensitive environment variables based on key names and provided patterns.
// It supports three modes:
// 1. patterns is nil (Unset): ALL environment variables are masked.
// 2. patterns is non-nil but empty: NO environment variables are masked.
// 3. patterns is non-empty: Only keys matching the glob patterns are masked (fail-closed on invalid glob).

func isUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			return false
		}
	}
	return true
}

func equalFoldASCII(s1, s2 string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		c1 := s1[i]
		c2 := s2[i]
		if c1 != c2 {
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				return false
			}
		}
	}
	return true
}

func hasSuffixFoldASCII(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	start := len(s) - len(suffix)
	for i := 0; i < len(suffix); i++ {
		c1 := s[start+i]
		c2 := suffix[i]
		if c1 != c2 {
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				return false
			}
		}
	}
	return true
}

func hasPrefixFoldASCII(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c1 := s[i]
		c2 := prefix[i]
		if c1 != c2 {
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				return false
			}
		}
	}
	return true
}

func containsFoldASCII(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	limit := len(s) - len(substr)
	for i := 0; i <= limit; i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			if c1 != c2 {
				if c1 >= 'A' && c1 <= 'Z' {
					c1 += 'a' - 'A'
				}
				if c2 >= 'A' && c2 <= 'Z' {
					c2 += 'a' - 'A'
				}
				if c1 != c2 {
					match = false
					break
				}
			}
		}
		if match {
			return true
		}
	}
	return false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 128 {
			return false
		}
	}
	return true
}

func fastMatchFold(key, pattern string) (bool, bool) {
	// Returns (matched, ok). If ok is false, caller must fall back to slow path (glob).
	if !isASCII(key) || !isASCII(pattern) {
		return false, false
	}

	// 1. Exact match (no wildcard characters)
	if !strings.ContainsAny(pattern, "*?[\\") {
		return equalFoldASCII(key, pattern), true
	}

	// 2. Suffix match "*suffix" (contains exactly one '*' at index 0)
	if strings.HasPrefix(pattern, "*") && !strings.ContainsAny(pattern[1:], "*?[\\") {
		return hasSuffixFoldASCII(key, pattern[1:]), true
	}

	// 3. Prefix match "prefix*" (contains exactly one '*' at the end)
	if strings.HasSuffix(pattern, "*") && !strings.ContainsAny(pattern[:len(pattern)-1], "*?[\\") {
		return hasPrefixFoldASCII(key, pattern[:len(pattern)-1]), true
	}

	// 4. Substring match "*substr*" (contains '*' at start and end, and no other wildcard characters)
	if len(pattern) >= 2 && strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && !strings.ContainsAny(pattern[1:len(pattern)-1], "*?[\\") {
		return containsFoldASCII(key, pattern[1:len(pattern)-1]), true
	}

	return false, false
}

func MaskSensitiveEnv(key, value string, patterns []string) string {
	if value == "" {
		return ""
	}

	// Mode 1: patterns is nil (Unset) -> Mask everything
	if patterns == nil {
		return "[REDACTED]"
	}

	// Mode 2: patterns is non-nil but empty -> Mask nothing
	if len(patterns) == 0 {
		return value
	}

	// Mode 3: patterns provided -> Mask matching keys
	// Matching is case-insensitive for patterns and keys.
	var upperKey string
	var upperKeyInitialized bool

	for _, p := range patterns {
		// Try allocation-free fast-path first!
		if matched, ok := fastMatchFold(key, p); ok {
			if matched {
				return "[REDACTED]"
			}
			continue
		}

		// Fallback to slow path (Glob/Unicode)
		// Lazily initialize upperKey only when a glob pattern is encountered.
		if !upperKeyInitialized {
			upperKey = key
			if !isUpper(key) {
				upperKey = strings.ToUpper(key)
			}
			upperKeyInitialized = true
		}
		upperPattern := p
		if !isUpper(p) {
			upperPattern = strings.ToUpper(p)
		}
		matched, err := path.Match(upperPattern, upperKey)
		if err != nil {
			// Fail-closed: if the pattern is invalid, we redact to be safe.
			return "[REDACTED]"
		}
		if matched {
			return "[REDACTED]"
		}
	}

	return value
}

func maskSensitiveEnvWithUpperPatterns(key, value string, patterns, upperPatterns []string) string {
	if value == "" {
		return ""
	}

	// Mode 1: patterns is nil (Unset) -> Mask everything
	if patterns == nil {
		return "[REDACTED]"
	}

	// Mode 2: patterns is non-nil but empty -> Mask nothing
	if len(patterns) == 0 {
		return value
	}

	// Mode 3: patterns provided -> Mask matching keys
	// Matching is case-insensitive for patterns and keys.
	var upperKey string
	var upperKeyInitialized bool

	for i := range patterns {
		p := patterns[i]
		// Try allocation-free fast-path first!
		if matched, ok := fastMatchFold(key, p); ok {
			if matched {
				return "[REDACTED]"
			}
			continue
		}

		// Fallback to slow path
		// Lazily initialize upperKey only when a glob pattern is encountered.
		if !upperKeyInitialized {
			upperKey = key
			if !isUpper(key) {
				upperKey = strings.ToUpper(key)
			}
			upperKeyInitialized = true
		}
		upperPattern := upperPatterns[i]
		matched, err := path.Match(upperPattern, upperKey)
		if err != nil {
			// Fail-closed: if the pattern is invalid, we redact to be safe.
			return "[REDACTED]"
		}
		if matched {
			return "[REDACTED]"
		}
	}

	return value
}

// MaskSensitiveEnvList returns a new slice of environment variables with sensitive values masked.
func MaskSensitiveEnvList(env []string, patterns []string) []string {
	if env == nil {
		return nil
	}

	// Mode 2 Fast Path: patterns is non-nil but empty -> Mask nothing.
	// We can safely return the original slice directly, saving allocations and matching logic.
	if patterns != nil && len(patterns) == 0 {
		return env
	}

	upperPatterns := patterns
	if len(patterns) > 0 {
		var copied []string
		for i, p := range patterns {
			// Optimization: only normalize patterns that contain glob syntax.
			// Pure literal patterns are matched case-insensitively via strings.EqualFold,
			// so they do not require eager uppercase normalization/allocations.
			if strings.ContainsAny(p, "*?[\\") && !isUpper(p) {
				if copied == nil {
					copied = make([]string, len(patterns))
					copy(copied, patterns)
					upperPatterns = copied
				}
				upperPatterns[i] = strings.ToUpper(p)
			}
		}
	}
	res := make([]string, len(env))
	for i, e := range env {
		if k, v, found := strings.Cut(e, "="); found {
			maskedV := maskSensitiveEnvWithUpperPatterns(k, v, patterns, upperPatterns)
			if maskedV == v {
				res[i] = e
			} else {
				res[i] = k + "=" + maskedV
			}
		} else {
			res[i] = e
		}
	}
	return res
}
