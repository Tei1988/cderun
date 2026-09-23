package config

import (
	"path"
	"strings"
	"sync/atomic"
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

func equalFoldASCIILower(s, cleanLower string) bool {
	if len(s) != len(cleanLower) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != cleanLower[i] {
			return false
		}
	}
	return true
}

func hasSuffixFoldASCIILower(s, suffixLower string) bool {
	if len(s) < len(suffixLower) {
		return false
	}
	start := len(s) - len(suffixLower)
	for i := 0; i < len(suffixLower); i++ {
		c := s[start+i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != suffixLower[i] {
			return false
		}
	}
	return true
}

func hasPrefixFoldASCIILower(s, prefixLower string) bool {
	if len(s) < len(prefixLower) {
		return false
	}
	for i := 0; i < len(prefixLower); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefixLower[i] {
			return false
		}
	}
	return true
}

func containsFoldASCIILower(s, substrLower string) bool {
	if len(substrLower) == 0 {
		return true
	}
	if len(s) < len(substrLower) {
		return false
	}
	limit := len(s) - len(substrLower)
	for i := 0; i <= limit; i++ {
		match := true
		for j := 0; j < len(substrLower); j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != substrLower[j] {
				match = false
				break
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
	raw           string
	upperPattern  string
	isASCII       bool
	isGlob        bool
	isSuffix      bool
	isPrefix      bool
	isSubstr      bool
	cleanPat      string
	cleanPatLower string
}

type analyzedPatternsCacheEntry struct {
	patterns []string
	analyzed []preAnalyzedPattern
}

var (
	patternsCache atomic.Pointer[analyzedPatternsCacheEntry]
)

func getAnalyzedPatterns(patterns []string) []preAnalyzedPattern {
	if len(patterns) == 0 {
		return nil
	}
	cached := patternsCache.Load()
	if cached != nil && len(cached.patterns) == len(patterns) {
		match := true
		for i, p := range patterns {
			if cached.patterns[i] != p {
				match = false
				break
			}
		}
		if match {
			return cached.analyzed
		}
	}

	analyzed := make([]preAnalyzedPattern, len(patterns))
	for i, p := range patterns {
		analyzed[i] = preAnalyzePattern(p)
	}

	patCopy := make([]string, len(patterns))
	copy(patCopy, patterns)

	patternsCache.Store(&analyzedPatternsCacheEntry{
		patterns: patCopy,
		analyzed: analyzed,
	})

	return analyzed
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
		ap.cleanPatLower = strings.ToLower(p)
		return ap
	}

	ap.isGlob = true
	if strings.HasPrefix(p, "*") && !strings.ContainsAny(p[1:], "*?[\\") {
		ap.isSuffix = true
		ap.cleanPat = p[1:]
		ap.cleanPatLower = strings.ToLower(p[1:])
		return ap
	}

	if strings.HasSuffix(p, "*") && !strings.ContainsAny(p[:len(p)-1], "*?[\\") {
		ap.isPrefix = true
		ap.cleanPat = p[:len(p)-1]
		ap.cleanPatLower = strings.ToLower(p[:len(p)-1])
		return ap
	}

	if len(p) >= 2 && strings.HasPrefix(p, "*") && strings.HasSuffix(p, "*") && !strings.ContainsAny(p[1:len(p)-1], "*?[\\") {
		ap.isSubstr = true
		ap.cleanPat = p[1 : len(p)-1]
		ap.cleanPatLower = strings.ToLower(p[1 : len(p)-1])
		return ap
	}

	return ap
}

func matchPreAnalyzed(key string, keyIsASCII bool, ap *preAnalyzedPattern, upperKey *string) bool {
	if keyIsASCII && ap.isASCII {
		if !ap.isGlob {
			if key == ap.cleanPat {
				return true
			}
			return equalFoldASCIILower(key, ap.cleanPatLower)
		}
		if ap.isSuffix {
			if strings.HasSuffix(key, ap.cleanPat) {
				return true
			}
			return hasSuffixFoldASCIILower(key, ap.cleanPatLower)
		}
		if ap.isPrefix {
			if strings.HasPrefix(key, ap.cleanPat) {
				return true
			}
			return hasPrefixFoldASCIILower(key, ap.cleanPatLower)
		}
		if ap.isSubstr {
			if strings.Contains(key, ap.cleanPat) {
				return true
			}
			return containsFoldASCIILower(key, ap.cleanPatLower)
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
	if value == "" || value == "[REDACTED]" {
		return value
	}

	if patterns == nil {
		return "[REDACTED]"
	}

	if len(patterns) == 0 {
		return value
	}

	analyzed := getAnalyzedPatterns(patterns)

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
			if idx := strings.IndexByte(e, '='); idx >= 0 {
				v := e[idx+1:]
				if v != "" && v != "[REDACTED]" {
					if res == nil {
						res = make([]string, len(env))
						copy(res, env[:i])
					}
					res[i] = e[:idx+1] + "[REDACTED]"
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

	analyzed := getAnalyzedPatterns(patterns)

	var res []string
	for i, e := range env {
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			k := e[:idx]
			v := e[idx+1:]
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
				if v == "[REDACTED]" {
					if res != nil {
						res[i] = e
					}
					continue
				}
				if res == nil {
					res = make([]string, len(env))
					copy(res, env[:i])
				}
				res[i] = e[:idx+1] + "[REDACTED]"
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
