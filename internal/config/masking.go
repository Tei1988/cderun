package config

import (
	"path"
	"strings"
)

// MaskSensitiveEnv redacts sensitive environment variables based on key names and provided patterns.
// If patterns is nil or empty, no environment variables are masked.
// If patterns is non-nil, only keys matching the patterns are masked.
func MaskSensitiveEnv(key, value string, patterns []string) string {
	if value == "" || len(patterns) == 0 {
		return value
	}

	// Case: patterns is non-nil -> Mask only matching keys
	// Matching is case-insensitive for patterns and keys.
	upperKey := strings.ToUpper(key)
	for _, p := range patterns {
		upperPattern := strings.ToUpper(p)
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
	res := make([]string, len(env))
	for i, e := range env {
		if k, v, found := strings.Cut(e, "="); found {
			maskedV := MaskSensitiveEnv(k, v, patterns)
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
