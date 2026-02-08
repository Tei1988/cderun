package config

import (
	"os"
	"regexp"
	"strings"
)

var exprRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// ExpressionResolver handles resolution of {{...}} expressions in config values.
type ExpressionResolver struct {
	Home        string
	Pwd         string
	HostContext *HostContext
}

// NewExpressionResolver creates an ExpressionResolver initialized with the current user's home directory, the current working directory, and the supplied HostContext.
// It attempts to obtain the home directory and working directory; if either lookup fails the corresponding field is set to the empty string. The returned error is always nil.
func NewExpressionResolver(hostCtx *HostContext) (*ExpressionResolver, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	pwd, err := os.Getwd()
	if err != nil {
		pwd = ""
	}
	return &ExpressionResolver{
		Home:        home,
		Pwd:         pwd,
		HostContext: hostCtx,
	}, nil
}

// Resolve processes a value (string, slice, or map) and resolves any expressions found.
func (r *ExpressionResolver) Resolve(v any) any {
	switch val := v.(type) {
	case string:
		return r.resolveString(val)
	case []any:
		for i, item := range val {
			val[i] = r.Resolve(item)
		}
		return val
	case map[string]any:
		for k, v := range val {
			val[k] = r.Resolve(v)
		}
		return val
	default:
		return v
	}
}

func (r *ExpressionResolver) resolveString(s string) string {
	return exprRegex.ReplaceAllStringFunc(s, func(match string) string {
		content := strings.TrimSpace(match[2 : len(match)-2])

		// 1. Magic Words
		switch content {
		case "HOME":
			return r.Home
		case "PWD":
			return r.Pwd
		}

		// 2. Directives
		if strings.HasPrefix(content, "file:") {
			filename := strings.TrimPrefix(content, "file:")
			return r.resolveFile(filename)
		}

		return match // Keep as is if unknown
	})
}

func (r *ExpressionResolver) resolveFile(filename string) string {
	paths := FindConfigs(filename)
	if len(paths) == 0 {
		return ""
	}

	// Use the highest priority file
	data, err := os.ReadFile(paths[0])
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}