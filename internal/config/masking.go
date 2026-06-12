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
}

// containsIgnoreCase checks if s contains substr (which must be uppercase ASCII letters) case-insensitively.
// This function is optimized for performance and avoids allocations.
func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	firstUpper := substr[0]
	firstLower := firstUpper + ('a' - 'A')

	for i := 0; i <= len(s)-len(substr); i++ {
		c := s[i]
		if c == firstUpper || c == firstLower {
			match := true
			for j := 1; j < len(substr); j++ {
				curr := s[i+j]
				if curr >= 'a' && curr <= 'z' {
					curr -= 'a' - 'A'
				}
				if curr != substr[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// MaskSensitiveEnv redacts sensitive environment variables based on key names.
func MaskSensitiveEnv(key, value string) string {
	if value == "" {
		return ""
	}

	// Fast path: if the key doesn't contain any potential sensitive keywords, skip complex splitting.
	hasSensitive := false
	for kw := range sensitiveKeywords {
		if containsIgnoreCase(key, kw) {
			hasSensitive = true
			break
		}
	}
	if !hasSensitive {
		return value
	}

	// Avoid ToUpper if already uppercase
	upperKey := key
	alreadyUpper := true
	for i := 0; i < len(key); i++ {
		if key[i] >= 'a' && key[i] <= 'z' {
			alreadyUpper = false
			break
		}
	}
	if !alreadyUpper {
		upperKey = strings.ToUpper(key)
	}

	// Split by non-alphanumeric characters and also split camelCase.
	// This ensures segments like "dbPassword" are correctly identified as ["db", "Password"].
	// We perform a single pass and check segments against sensitiveKeywords without extra allocations.
	useUpperDirectly := len(upperKey) == len(key)
	start := -1
	var lastRune rune

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
					// Look ahead to the next rune without allocation.
					nextIdx := i + utf8.RuneLen(r)
					if nextIdx < len(key) {
						nextRune, _ := utf8.DecodeRuneInString(key[nextIdx:])
						if nextRune != utf8.RuneError && unicode.IsLower(nextRune) {
							isAcronym = true
						}
					}
				}

				if isCamel || isLetterDigit || isAcronym {
					var segment string
					if useUpperDirectly {
						segment = upperKey[start:i]
					} else {
						segment = strings.ToUpper(key[start:i])
					}
					if _, ok := sensitiveKeywords[segment]; ok {
						return "[REDACTED]"
					}
					start = i
				}
			}
		} else {
			if start != -1 {
				var segment string
				if useUpperDirectly {
					segment = upperKey[start:i]
				} else {
					segment = strings.ToUpper(key[start:i])
				}
				if _, ok := sensitiveKeywords[segment]; ok {
					return "[REDACTED]"
				}
				start = -1
			}
		}
		lastRune = r
	}

	if start != -1 {
		var segment string
		if useUpperDirectly {
			segment = upperKey[start:]
		} else {
			segment = strings.ToUpper(key[start:])
		}
		if _, ok := sensitiveKeywords[segment]; ok {
			return "[REDACTED]"
		}
	}

	return value
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
