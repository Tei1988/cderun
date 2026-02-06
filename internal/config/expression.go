package config

import (
	"os"
	"regexp"
	"strings"
)

var exprRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

var DisableDiscovery = false

// ExpressionResolver handles resolution of {{...}} expressions in config values.
type ExpressionResolver struct {
	Home        string
	Pwd         string
	HostContext *HostContext
}

func NewExpressionResolver(global *CDERunConfig) (*ExpressionResolver, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	pwd, err := os.Getwd()
	if err != nil {
		pwd = ""
	}
	res := &ExpressionResolver{
		Home: home,
		Pwd:  pwd,
	}

	if global != nil {
		res.HostContext = global.HostContext
	}

	if DisableDiscovery {
		return res, nil
	}

	// Supplement HostContext with discovered root if applicable
	if res.HostContext == nil {
		rootPath := DiscoverHostRoot()
		if rootPath != "" {
			res.HostContext = &HostContext{
				Level: 0,
				Mounts: []HostMount{
					{Source: rootPath, Target: "/", Level: 0},
				},
			}
		}
	} else {
		// Even if we have a context, ensure we have a base root if none exists
		hasRoot := false
		for _, m := range res.HostContext.Mounts {
			if m.Target == "/" {
				hasRoot = true
				break
			}
		}
		if !hasRoot {
			rootPath := DiscoverHostRoot()
			if rootPath != "" {
				res.HostContext.Mounts = append(res.HostContext.Mounts, HostMount{
					Source: rootPath,
					Target: "/",
					Level:  0,
				})
			}
		}
	}

	return res, nil
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
