package config

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var sensitiveKeywords = map[string]struct{}{
	"PASSWORD":    {},
	"SECRET":      {},
	"TOKEN":       {},
	"KEY":         {},
	"AUTH":        {},
	"SIG":         {},
	"CERT":        {},
	"PEM":         {},
	"PRIVATE":     {},
	"CREDENTIALS": {},
	"PASSPHRASE":  {},
	"APIKEY":      {},
	"SESSION":     {},
	"ACCESS":      {},
	"JWT":         {},
	"SALT":        {},
	"SIGNATURE":   {},
	"BEARER":      {},
	"OTP":         {},
	"SENSITIVE":   {},
}

const maxKeywordLen = 16

// MaskSensitiveEnv redacts sensitive environment variables based on key names.
func MaskSensitiveEnv(key, value string) string {
	if value == "" {
		return ""
	}

	start := -1
	var lastRune rune
	var buf [maxKeywordLen]byte

	for i, r := range key {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (r > 127 && (unicode.IsLetter(r) || unicode.IsDigit(r)))

		if isAlphaNum {
			if start == -1 {
				start = i
			} else {
				// Boundary split logic (camelCase, letter/digit transition, acronyms)
				isCamel := unicode.IsLower(lastRune) && unicode.IsUpper(r)
				isLetterDigit := (unicode.IsLetter(lastRune) && unicode.IsDigit(r)) || (unicode.IsDigit(lastRune) && unicode.IsLetter(r))
				isAcronym := false
				if unicode.IsUpper(lastRune) && unicode.IsUpper(r) {
					// Check for acronym boundary (e.g. APIKey -> API, Key)
					nextIdx := i + utf8.RuneLen(r)
					if nextIdx < len(key) {
						nextRune, _ := utf8.DecodeRuneInString(key[nextIdx:])
						if nextRune != utf8.RuneError && unicode.IsLower(nextRune) {
							isAcronym = true
						}
					}
				}

				if isCamel || isLetterDigit || isAcronym {
					if checkSegment(key[start:i], &buf) {
						return "[REDACTED]"
					}
					start = i
				}
			}
		} else {
			if start != -1 {
				if checkSegment(key[start:i], &buf) {
					return "[REDACTED]"
				}
				start = -1
			}
		}
		lastRune = r
	}

	if start != -1 {
		if checkSegment(key[start:], &buf) {
			return "[REDACTED]"
		}
	}

	return value
}

func checkSegment(s string, buf *[maxKeywordLen]byte) bool {
	if len(s) == 0 || len(s) > maxKeywordLen {
		return false
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		buf[i] = c
	}

	if _, ok := sensitiveKeywords[string(buf[:len(s)])]; ok {
		return true
	}
	return false
}

// MaskSensitiveEnvList returns a new slice of environment variables with sensitive values masked.
func MaskSensitiveEnvList(env []string) []string {
	if env == nil {
		return nil
	}
	res := make([]string, len(env))
	for i, e := range env {
		if k, v, found := strings.Cut(e, "="); found {
			maskedV := MaskSensitiveEnv(k, v)
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
