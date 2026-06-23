package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"cderun/internal/container"
	"cderun/internal/logging"
)

func pickConfigs[T any](
	p1 []string,
	p2 []string,
	envKey string,
	envSep string,
	subcommand string,
	tools ToolsConfig,
	toolGetter func(ToolConfig) []T,
	global *CDERunConfig,
	globalGetter func(CDERunConfig) []T,
	parser func(string, string) (T, error),
	fs FileSystem,
) ([]T, error) {
	if p1 != nil {
		res := make([]T, 0, len(p1))
		for _, s := range p1 {
			v, err := parser(s, "override")
			if err != nil {
				return nil, err
			}
			res = append(res, v)
		}
		return res, nil
	}
	if p2 != nil {
		res := make([]T, 0, len(p2))
		for _, s := range p2 {
			v, err := parser(s, "")
			if err != nil {
				return nil, err
			}
			res = append(res, v)
		}
		return res, nil
	}
	if envKey != "" {
		if env, ok := fs.LookupEnv(envKey); ok {
			res := []T{}
			for s := range strings.SplitSeq(env, envSep) {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				v, err := parser(s, "env")
				if err != nil {
					return nil, err
				}
				res = append(res, v)
			}
			return res, nil
		}
	}
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if v := toolGetter(tool); v != nil {
				return v, nil
			}
		}
	}
	if global != nil {
		if v := globalGetter(*global); v != nil {
			return v, nil
		}
	}
	return nil, nil
}

func resolveDevices(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) ([]container.DeviceMapping, error) {
	dcs, err := pickConfigs(
		p1, p2, "CDERUN_DEVICE", ",", subcommand, tools,
		func(t ToolConfig) []DeviceConfig { return t.Devices },
		global, func(g CDERunConfig) []DeviceConfig { return g.Defaults.Devices },
		func(s, src string) (DeviceConfig, error) {
			parsed, ok := ParseDeviceConfig(s)
			if !ok {
				switch src {
				case "override":
					return DeviceConfig{}, fmt.Errorf("invalid device config (override): %q", s)
				case "env":
					return DeviceConfig{}, fmt.Errorf("invalid device config in CDERUN_DEVICE: %q", s)
				default:
					return DeviceConfig{}, fmt.Errorf("invalid device config: %q", s)
				}
			}
			parsed.SetBaseDir(r.Pwd)
			return parsed, nil
		},
		fs,
	)
	if err != nil {
		return nil, err
	}

	res := make([]container.DeviceMapping, 0, len(dcs))
	for _, dc := range dcs {
		resolved, err := dc.Resolve(r)
		if err != nil {
			return nil, err
		}
		res = append(res, resolved)
	}
	return res, nil
}

func resolveEnv(p1 []string, p2 []string, envKey string, subcommand string, tools ToolsConfig, global *CDERunConfig, strict bool, r *ExpressionResolver, fs FileSystem) ([]string, error) {
	envs, err := pickConfigs(
		p1, p2, envKey, ";", subcommand, tools,
		func(t ToolConfig) []string { return t.Env },
		global, func(g CDERunConfig) []string { return g.Defaults.Env },
		func(s, src string) (string, error) { return s, nil },
		fs,
	)
	if err != nil {
		return nil, err
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
	if cliImage == "" || configImage == "" || cliImage == configImage {
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
	res := make([]string, 0, len(env))
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

		if resolvedE == e && strings.Contains(e, "=") {
			res = append(res, e)
		} else {
			res = append(res, key+"="+val)
		}
	}
	return res, nil
}

func resolveMounts(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) ([]container.Mount, error) {
	mcs, err := pickConfigs(
		p1, p2, "CDERUN_MOUNT", ";", subcommand, tools,
		func(t ToolConfig) []MountConfig { return t.Mounts },
		global, func(g CDERunConfig) []MountConfig { return g.Defaults.Mounts },
		func(s, src string) (MountConfig, error) {
			parsed, err := ParseMountFlag(s)
			if err != nil {
				switch src {
				case "override":
					return MountConfig{}, fmt.Errorf("invalid mount config (override): %w", err)
				case "env":
					return MountConfig{}, fmt.Errorf("invalid mount config in CDERUN_MOUNT: %w", err)
				default:
					return MountConfig{}, fmt.Errorf("invalid mount config: %w", err)
				}
			}
			parsed.SetBaseDir(r.Pwd)
			return parsed, nil
		},
		fs,
	)
	if err != nil {
		return nil, err
	}

	res := make([]container.Mount, 0, len(mcs))
	for _, mc := range mcs {
		if mc.Optional && (mc.Type == "bind" || mc.Type == "") && !mc.Source.IsEmpty() {
			hostPath, err := mc.Source.Resolve(r)
			if err != nil {
				return nil, err
			}
			if _, err := r.Stat(hostPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					// Skip if source doesn't exist
					continue
				}
				return nil, err
			}
		}

		resolved, err := mc.Resolve(r)
		if err != nil {
			return nil, err
		}
		res = append(res, resolved)
	}
	return res, nil
}
