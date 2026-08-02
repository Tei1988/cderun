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

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 128 {
			return false
		}
	}
	return true
}

func containsFold(s, substr string) bool {
	if !isASCII(s) || !isASCII(substr) {
		return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
	}
	if len(substr) > len(s) {
		return false
	}
	if len(substr) == 0 {
		return true
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

func fastMatchFold(pattern, key string) (bool, bool) {
	if strings.ContainsAny(pattern, "?[\\]") {
		return false, false
	}

	count := strings.Count(pattern, "*")
	if count == 0 {
		return strings.EqualFold(key, pattern), true
	}

	if pattern == "*" {
		return true, true
	}

	if count == 1 {
		if pattern[0] == '*' {
			suffix := pattern[1:]
			if len(key) >= len(suffix) {
				return strings.EqualFold(key[len(key)-len(suffix):], suffix), true
			}
			return false, true
		}
		if pattern[len(pattern)-1] == '*' {
			prefix := pattern[:len(pattern)-1]
			if len(key) >= len(prefix) {
				return strings.EqualFold(key[:len(prefix)], prefix), true
			}
			return false, true
		}
	}

	if count == 2 && pattern[0] == '*' && pattern[len(pattern)-1] == '*' {
		substr := pattern[1 : len(pattern)-1]
		if substr == "" {
			return true, true
		}
		if len(key) >= len(substr) {
			return containsFold(key, substr), true
		}
		return false, true
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
		if matched, ok := fastMatchFold(p, key); ok {
			if matched {
				return "[REDACTED]"
			}
			continue
		}

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

	for i, p := range patterns {
		if matched, ok := fastMatchFold(p, key); ok {
			if matched {
				return "[REDACTED]"
			}
			continue
		}

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

	// Mode 2: patterns is non-nil but empty -> Mask nothing
	if patterns != nil && len(patterns) == 0 {
		return env
	}

	upperPatterns := patterns
	if len(patterns) > 0 {
		var copied []string
		for i, p := range patterns {
			if !isUpper(p) {
				if copied == nil {
					copied = make([]string, len(patterns))
					copy(copied, patterns)
					upperPatterns = copied
				}
				upperPatterns[i] = strings.ToUpper(p)
			}
		}
	}

	var res []string // Lazily initialized only when a value is actually masked

	for i, e := range env {
		if k, v, found := strings.Cut(e, "="); found {
			maskedV := maskSensitiveEnvWithUpperPatterns(k, v, patterns, upperPatterns)
			if maskedV == v {
				if res != nil {
					res[i] = e
				}
			} else {
				if res == nil {
					res = make([]string, len(env))
					copy(res, env[:i])
				}
				res[i] = k + "=" + maskedV
			}
		} else {
			if res != nil {
				res[i] = e
			}
		}
	}

	if res == nil {
		return env
	}
	return res
}
