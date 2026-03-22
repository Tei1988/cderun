package config

import (
	"strconv"
	"strings"
)

// OptionDef defines a single resolvable option with its full P1-P5 resolution chain.
// T is the value type (string, bool, []string, float64).
// For bool options, ToolGetter and GlobalGetter return *bool to distinguish unset from false.
type OptionDef[T any] struct {
	// P3 environment variable key
	EnvKey string
	// P4: getter from tool-specific config
	ToolGetter func(ToolConfig) T
	// P5: getter from global defaults
	GlobalGetter func(CDERunConfig) T
	// P6: hardcoded fallback
	Fallback T
}

// resolveStringOpt resolves a string option through P1-P6.
func resolveStringOpt(
	def OptionDef[string],
	p1Set bool, p1Val string,
	p2Set bool, p2Val string,
	subcommand string, tools ToolsConfig, global *CDERunConfig,
	r *ExpressionResolver, fs FileSystem,
) string {
	if p1Set {
		return r.resolveString(p1Val)
	}
	if p2Set {
		return r.resolveString(p2Val)
	}
	if env := fs.Getenv(def.EnvKey); env != "" {
		return r.resolveString(env)
	}
	if def.ToolGetter != nil && tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if s := def.ToolGetter(tool); s != "" {
				return r.resolveString(s)
			}
		}
	}
	if def.GlobalGetter != nil && global != nil {
		if s := def.GlobalGetter(*global); s != "" {
			return r.resolveString(s)
		}
	}
	return r.resolveString(def.Fallback)
}

// resolveBoolOpt resolves a bool option through P1-P6.
// ToolGetter and GlobalGetter return *bool to distinguish unset from false.
func resolveBoolOpt(
	def OptionDef[*bool],
	fallback bool,
	p1Set bool, p1Val bool,
	p2Set bool, p2Val bool,
	subcommand string, tools ToolsConfig, global *CDERunConfig,
	fs FileSystem,
) bool {
	val, specified := resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, fs)
	if specified {
		return val
	}
	return fallback
}

// resolveBoolOptInfo resolves a bool option and reports whether a value was found.
func resolveBoolOptInfo(
	def OptionDef[*bool],
	p1Set bool, p1Val bool,
	p2Set bool, p2Val bool,
	subcommand string, tools ToolsConfig, global *CDERunConfig,
	fs FileSystem,
) (bool, bool) {
	if p1Set {
		return p1Val, true
	}
	if p2Set {
		return p2Val, true
	}
	if def.EnvKey != "" {
		if env := fs.Getenv(def.EnvKey); env != "" {
			if b, err := strconv.ParseBool(env); err == nil {
				return b, true
			}
		}
	}
	if def.ToolGetter != nil && tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if b := def.ToolGetter(tool); b != nil {
				return *b, true
			}
		}
	}
	if def.GlobalGetter != nil && global != nil {
		if b := def.GlobalGetter(*global); b != nil {
			return *b, true
		}
	}
	return false, false
}

// resolveStringSliceOpt resolves a []string option through P1-P6.
// P1/P2 are passed as slices directly (already parsed by Cobra).
// P3 env var is split by the given separator.
func resolveStringSliceOpt(
	def OptionDef[[]string],
	envSep string,
	p1 []string, p2 []string,
	subcommand string, tools ToolsConfig, global *CDERunConfig,
	r *ExpressionResolver, fs FileSystem,
) []string {
	var vals []string
	if p1 != nil {
		vals = p1
	} else if p2 != nil {
		vals = p2
	} else if env, ok := fs.LookupEnv(def.EnvKey); ok {
		vals = strings.Split(env, envSep)
	} else if def.ToolGetter != nil && tools != nil {
		if tool, ok := tools[subcommand]; ok {
			vals = def.ToolGetter(tool)
		}
	}
	if vals == nil && def.GlobalGetter != nil && global != nil {
		vals = def.GlobalGetter(*global)
	}
	var res []string
	for _, v := range vals {
		res = append(res, r.resolveString(v))
	}
	return res
}

// resolveStringSliceCommaOpt resolves a comma-string option (e.g. --mount-tools)
// where P1/P2 are single comma-separated strings rather than repeated flags.
func resolveStringSliceCommaOpt(
	def OptionDef[[]string],
	p1Set bool, p1Val string,
	p2Set bool, p2Val string,
	subcommand string, tools ToolsConfig, global *CDERunConfig,
	r *ExpressionResolver, fs FileSystem,
) []string {
	var vals []string
	if p1Set {
		vals = strings.Split(p1Val, ",")
	} else if p2Set {
		vals = strings.Split(p2Val, ",")
	} else if env, ok := fs.LookupEnv(def.EnvKey); ok {
		vals = strings.Split(env, ",")
	} else if def.ToolGetter != nil && tools != nil {
		if tool, ok := tools[subcommand]; ok {
			vals = def.ToolGetter(tool)
		}
	}
	if vals == nil && def.GlobalGetter != nil && global != nil {
		vals = def.GlobalGetter(*global)
	}
	var res []string
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			res = append(res, r.resolveString(v))
		}
	}
	return res
}

// resolveFloat64Opt resolves a float64 option through P1-P6.
func resolveFloat64Opt(
	def OptionDef[*float64],
	p1Set bool, p1Val float64,
	p2Set bool, p2Val float64,
	subcommand string, tools ToolsConfig, global *CDERunConfig,
	fs FileSystem,
) float64 {
	if p1Set {
		return p1Val
	}
	if p2Set {
		return p2Val
	}
	if env := fs.Getenv(def.EnvKey); env != "" {
		if f, err := strconv.ParseFloat(env, 64); err == nil {
			return f
		}
	}
	if def.ToolGetter != nil && tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if f := def.ToolGetter(tool); f != nil {
				return *f
			}
		}
	}
	if def.GlobalGetter != nil && global != nil {
		if f := def.GlobalGetter(*global); f != nil {
			return *f
		}
	}
	if def.Fallback != nil {
		return *def.Fallback
	}
	return 0
}
