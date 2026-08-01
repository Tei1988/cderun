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
		// Optimization: if pattern does not contain glob characters,
		// we can perform a case-insensitive exact match using strings.EqualFold.
		// This avoids allocating uppercase strings for 'key' or 'p'.
		if !strings.ContainsAny(p, "*?[\\") {
			if strings.EqualFold(key, p) {
				return "[REDACTED]"
			}
		} else {
			// If it's a glob pattern, fall back to path.Match.
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
		// Optimization: if pattern does not contain glob characters,
		// use strings.EqualFold to bypass allocations entirely.
		if !strings.ContainsAny(p, "*?[\\") {
			if strings.EqualFold(key, p) {
				return "[REDACTED]"
			}
		} else {
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
