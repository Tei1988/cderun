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
	upperKey := key
	if !isUpper(key) {
		upperKey = strings.ToUpper(key)
	}
	for _, p := range patterns {
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
	upperKey := key
	if !isUpper(key) {
		upperKey = strings.ToUpper(key)
	}
	for i := range patterns {
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
	var upperPatterns []string
	if len(patterns) > 0 {
		upperPatterns = make([]string, len(patterns))
		for i, p := range patterns {
			if isUpper(p) {
				upperPatterns[i] = p
			} else {
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
