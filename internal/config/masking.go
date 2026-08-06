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

type preAnalyzedPattern struct {
	raw          string
	upperPattern string
	isASCII      bool
	isGlob       bool
	isSuffix     bool
	isPrefix     bool
	isSubstr     bool
	cleanPat     string
}

func preAnalyzePattern(p string) preAnalyzedPattern {
	ap := preAnalyzedPattern{
		raw:          p,
		upperPattern: strings.ToUpper(p),
		isASCII:      isASCII(p),
	}

	if !ap.isASCII {
		ap.isGlob = true
		return ap
	}

	hasWildcard := strings.ContainsAny(p, "*?[\\")
	if !hasWildcard {
		ap.cleanPat = p
		return ap
	}

	ap.isGlob = true
	if strings.HasPrefix(p, "*") && !strings.ContainsAny(p[1:], "*?[\\") {
		ap.isSuffix = true
		ap.cleanPat = p[1:]
		return ap
	}

	if strings.HasSuffix(p, "*") && !strings.ContainsAny(p[:len(p)-1], "*?[\\") {
		ap.isPrefix = true
		ap.cleanPat = p[:len(p)-1]
		return ap
	}

	if len(p) >= 2 && strings.HasPrefix(p, "*") && strings.HasSuffix(p, "*") && !strings.ContainsAny(p[1:len(p)-1], "*?[\\") {
		ap.isSubstr = true
		ap.cleanPat = p[1 : len(p)-1]
		return ap
	}

	return ap
}

func matchPreAnalyzed(key string, keyIsASCII bool, ap *preAnalyzedPattern, upperKey *string) bool {
	if keyIsASCII && ap.isASCII {
		if !ap.isGlob {
			return equalFoldASCII(key, ap.cleanPat)
		}
		if ap.isSuffix {
			return hasSuffixFoldASCII(key, ap.cleanPat)
		}
		if ap.isPrefix {
			return hasPrefixFoldASCII(key, ap.cleanPat)
		}
		if ap.isSubstr {
			return containsFoldASCII(key, ap.cleanPat)
		}
	}

	if *upperKey == "" {
		if isUpper(key) {
			*upperKey = key
		} else {
			*upperKey = strings.ToUpper(key)
		}
	}
	matched, err := path.Match(ap.upperPattern, *upperKey)
	if err != nil {
		return true
	}
	return matched
}

func MaskSensitiveEnv(key, value string, patterns []string) string {
	if value == "" {
		return ""
	}

	if patterns == nil {
		return "[REDACTED]"
	}

	if len(patterns) == 0 {
		return value
	}

	var localBuf [16]preAnalyzedPattern
	var analyzed []preAnalyzedPattern
	if len(patterns) <= 16 {
		analyzed = localBuf[:len(patterns)]
	} else {
		analyzed = make([]preAnalyzedPattern, len(patterns))
	}

	for i, p := range patterns {
		analyzed[i] = preAnalyzePattern(p)
	}

	keyIsASCII := isASCII(key)
	var upperKey string

	for i := range analyzed {
		if matchPreAnalyzed(key, keyIsASCII, &analyzed[i], &upperKey) {
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

	if patterns != nil && len(patterns) == 0 {
		return env
	}

	if patterns == nil {
		var res []string
		for i, e := range env {
			if k, v, found := strings.Cut(e, "="); found {
				if v != "" {
					if res == nil {
						res = make([]string, len(env))
						copy(res, env[:i])
					}
					res[i] = k + "=[REDACTED]"
				} else if res != nil {
					res[i] = e
				}
			} else if res != nil {
				res[i] = e
			}
		}
		if res == nil {
			return env
		}
		return res
	}

	var localBuf [16]preAnalyzedPattern
	var analyzed []preAnalyzedPattern
	if len(patterns) <= 16 {
		analyzed = localBuf[:len(patterns)]
	} else {
		analyzed = make([]preAnalyzedPattern, len(patterns))
	}

	for i, p := range patterns {
		analyzed[i] = preAnalyzePattern(p)
	}

	var res []string
	for i, e := range env {
		if k, v, found := strings.Cut(e, "="); found {
			if v == "" {
				if res != nil {
					res[i] = e
				}
				continue
			}

			keyIsASCII := isASCII(k)
			var upperKey string
			matched := false

			for j := range analyzed {
				if matchPreAnalyzed(k, keyIsASCII, &analyzed[j], &upperKey) {
					matched = true
					break
				}
			}

			if matched {
				if res == nil {
					res = make([]string, len(env))
					copy(res, env[:i])
				}
				res[i] = k + "=[REDACTED]"
			} else if res != nil {
				res[i] = e
			}
		} else if res != nil {
			res[i] = e
		}
	}

	if res == nil {
		return env
	}
	return res
}
