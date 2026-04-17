package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type configPathOptions struct {
	p1Set        bool
	p1Val        string
	cliSet       bool
	cliVal       string
	envKey       string
	subcommand   string
	tools        ToolsConfig
	toolGetter   func(ToolConfig) ConfigPath
	global       *CDERunConfig
	globalGetter func(CDERunConfig) ConfigPath
	fallback     string
	pathType     string
}

func resolveConfigPath(opts configPathOptions, r *ExpressionResolver, fs FileSystem) (string, error) {
	var cp ConfigPath
	if opts.p1Set {
		cp = ConfigPath{Raw: opts.p1Val, BaseDir: r.Pwd}
	} else if opts.cliSet {
		cp = ConfigPath{Raw: opts.cliVal, BaseDir: r.Pwd}
	} else if env := fs.Getenv(opts.envKey); env != "" {
		cp = ConfigPath{Raw: env, BaseDir: r.Pwd}
	} else {
		found := false
		if opts.tools != nil {
			if tool, ok := opts.tools[opts.subcommand]; ok {
				if opts.toolGetter != nil {
					if t := opts.toolGetter(tool); !t.IsEmpty() {
						cp = t
						found = true
					}
				}
			}
		}
		if !found && opts.global != nil {
			if opts.globalGetter != nil {
				if g := opts.globalGetter(*opts.global); !g.IsEmpty() {
					cp = g
					found = true
				}
			}
		}
		if !found {
			cp = ConfigPath{Raw: opts.fallback, BaseDir: r.Pwd}
		}
	}

	switch opts.pathType {
	case "volume":
		return cp.ResolveVolume(r)
	case "device":
		return cp.ResolveDevice(r)
	default:
		return cp.Resolve(r)
	}
}

func resolveMultiSource[T any, R any](
	p1 []string,
	p2 []string,
	envKey string,
	subcommand string,
	tools ToolsConfig,
	global *CDERunConfig,
	r *ExpressionResolver,
	fs FileSystem,
	envSep string,
	parser func(string) (T, error),
	setBaseDir func(*T, string),
	toolGetter func(ToolConfig) []T,
	globalGetter func(CDERunConfig) []T,
	resolver func(T) (R, error),
	skipPredicate func(T, R) (bool, error),
) ([]R, error) {
	var configs []T

	if p1 != nil {
		configs = make([]T, 0, len(p1))
		for _, s := range p1 {
			parsed, err := parser(s)
			if err != nil {
				return nil, fmt.Errorf("invalid config (override): %w", err)
			}
			setBaseDir(&parsed, r.Pwd)
			configs = append(configs, parsed)
		}
	} else if p2 != nil {
		configs = make([]T, 0, len(p2))
		for _, s := range p2 {
			parsed, err := parser(s)
			if err != nil {
				return nil, fmt.Errorf("invalid config: %w", err)
			}
			setBaseDir(&parsed, r.Pwd)
			configs = append(configs, parsed)
		}
	} else if env, ok := fs.LookupEnv(envKey); ok {
		configs = []T{}
		for s := range strings.SplitSeq(env, envSep) {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			parsed, err := parser(s)
			if err != nil {
				return nil, fmt.Errorf("invalid config in %s: %w", envKey, err)
			}
			setBaseDir(&parsed, r.Pwd)
			configs = append(configs, parsed)
		}
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			configs = toolGetter(tool)
		}
	}

	if configs == nil && global != nil {
		configs = globalGetter(*global)
	}

	var res []R
	for _, cfg := range configs {
		resolved, err := resolver(cfg)
		if err != nil {
			return nil, err
		}
		if skipPredicate != nil {
			skip, err := skipPredicate(cfg, resolved)
			if err != nil {
				return nil, err
			}
			if skip {
				continue
			}
		}
		res = append(res, resolved)
	}
	return res, nil
}

func resolveDevices(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) ([]container.DeviceMapping, error) {
	return resolveMultiSource(
		p1, p2, "CDERUN_DEVICE", subcommand, tools, global, r, fs, ",",
		func(s string) (DeviceConfig, error) {
			parsed, ok := ParseDeviceConfig(s)
			if !ok {
				return DeviceConfig{}, fmt.Errorf("invalid device config: %q", s)
			}
			return parsed, nil
		},
		func(dc *DeviceConfig, pwd string) { dc.SetBaseDir(pwd) },
		func(t ToolConfig) []DeviceConfig { return t.Devices },
		func(g CDERunConfig) []DeviceConfig { return g.Defaults.Devices },
		func(dc DeviceConfig) (container.DeviceMapping, error) { return dc.Resolve(r) },
		nil,
	)
}

func resolveEnv(p1 []string, p2 []string, envKey string, subcommand string, tools ToolsConfig, global *CDERunConfig, strict bool, r *ExpressionResolver, fs FileSystem) ([]string, error) {
	var envs []string

	if p1 != nil {
		envs = p1
	} else if p2 != nil {
		envs = p2
	} else if env, ok := fs.LookupEnv(envKey); ok {
		envs = []string{}
		for e := range strings.SplitSeq(env, ";") {
			e = strings.TrimSpace(e)
			if e == "" {
				continue
			}
			envs = append(envs, e)
		}
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok && tool.Env != nil {
			envs = tool.Env
		}
	}

	if envs == nil && global != nil {
		envs = global.Defaults.Env
	}

	// Deduplicate within the winning source (last-one-wins for the same key)
	// We use mergeEnv with nil/nil for other sources to leverage its deduplication logic.
	merged := mergeEnv(nil, nil, envs)

	return resolveEnvValues(merged, strict, r, fs)
}

func mergeEnv(base, p2, p1 []string) []string {
	total := len(base) + len(p2) + len(p1)
	if total == 0 {
		return nil
	}
	m := make(map[string]string, total)
	keys := make([]string, 0, total)

	add := func(env []string) {
		for _, e := range env {
			key, _, _ := strings.Cut(e, "=")
			if _, ok := m[key]; !ok {
				keys = append(keys, key)
			}
			m[key] = e
		}
	}

	add(base)
	add(p2)
	add(p1)

	res := make([]string, 0, len(keys))
	for _, k := range keys {
		res = append(res, m[k])
	}
	return res
}

func validateImageRegistryMatch(cliImage, configImage string) error {
	if cliImage == "" || configImage == "" {
		return nil
	}

	normalize := func(img string) (string, string) {
		parts := strings.Split(img, "/")
		var host, repo string
		if len(parts) == 1 {
			host = "docker.io"
			repo = "library/" + parts[0]
		} else if len(parts) == 2 {
			if strings.ContainsAny(parts[0], ".:") || parts[0] == "localhost" {
				host = parts[0]
				repo = parts[1]
			} else {
				host = "docker.io"
				repo = parts[0] + "/" + parts[1]
			}
		} else {
			host = parts[0]
			repo = strings.Join(parts[1:], "/")
		}

		if host == "docker.io" && !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}

		// Strip tags or digests from repo
		if idx := strings.IndexAny(repo, ":@"); idx != -1 {
			repo = repo[:idx]
		}
		return host, repo
	}

	cliHost, cliRepo := normalize(cliImage)
	cfgHost, cfgRepo := normalize(configImage)

	if cliHost != cfgHost || cliRepo != cfgRepo {
		return &RegistryMismatchError{
			ExpectedRegistry: fmt.Sprintf("%s/%s", cfgHost, cfgRepo),
			ActualRegistry:   fmt.Sprintf("%s/%s", cliHost, cliRepo),
		}
	}

	return nil
}

func resolveEnvValues(env []string, strict bool, r *ExpressionResolver, fs FileSystem) ([]string, error) {
	var res []string
	for _, e := range env {
		resolvedE := r.resolveString(e)
		if err := r.Error(); err != nil {
			return nil, err
		}

		var key, val string
		if k, v, found := strings.Cut(resolvedE, "="); found {
			key = k
			val = v
			if err := ValidateEnvKey(key); err != nil {
				return nil, err
			}
		} else {
			v, found := fs.LookupEnv(resolvedE)
			if !found && strict {
				return nil, fmt.Errorf("required environment variable not found: %q", resolvedE)
			}
			key = resolvedE
			val = v
		}

		// Apply masking for debug logs and quoting for safety
		if logging.DebugEnabled() {
			logging.Debug("Resolved Env: %q=%q", key, MaskSensitiveEnv(key, val))
		}

		res = append(res, key+"="+val)
	}
	return res, nil
}

func resolveMounts(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) ([]container.Mount, error) {
	return resolveMultiSource(
		p1, p2, "CDERUN_MOUNT", subcommand, tools, global, r, fs, ";",
		ParseMountFlag,
		func(mc *MountConfig, pwd string) { mc.SetBaseDir(pwd) },
		func(t ToolConfig) []MountConfig { return t.Mounts },
		func(g CDERunConfig) []MountConfig { return g.Defaults.Mounts },
		func(mc MountConfig) (container.Mount, error) { return mc.Resolve(r) },
		func(mc MountConfig, m container.Mount) (bool, error) {
			if mc.Optional && (mc.Type == "bind" || mc.Type == "") && m.Source != "" {
				if _, err := fs.Stat(m.Source); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return true, nil // skip
					}
					return false, err
				}
			}
			return false, nil
		},
	)
}
