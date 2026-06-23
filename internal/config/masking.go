package config

import (
	"path"
	"strings"
)

// MaskSensitiveEnv redacts sensitive environment variables based on key names and provided patterns.
// If patterns is nil, ALL environment variables are masked.
// If patterns is non-nil (including empty slice), only keys matching the patterns are masked.
func MaskSensitiveEnv(key, value string, patterns []string) string {
	if value == "" {
		return ""
	}

	// Case 1: patterns is nil (Unset) -> Mask everything
	if patterns == nil {
		return "[REDACTED]"
	}

	// Case 2: patterns is non-nil -> Mask only matching keys
	// Matching is case-insensitive for patterns and keys.
	upperKey := strings.ToUpper(key)
	for _, p := range patterns {
		upperPattern := strings.ToUpper(p)
		if matched, _ := path.Match(upperPattern, upperKey); matched {
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
