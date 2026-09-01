package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/docker/go-units"
)

// parseSlice parses a slice of string configs into type T using the provided parser.
func parseSlice[T any](slice []string, sourceLabel string, parser func(string, string) (T, error)) ([]T, error) {
	if parser == nil {
		if res, ok := any(slice).([]T); ok {
			return res, nil
		}
		return nil, errors.New("parser required for non-string types")
	}
	res := make([]T, len(slice))
	for i, s := range slice {
		v, err := parser(s, sourceLabel)
		if err != nil {
			return nil, err
		}
		res[i] = v
	}
	return res, nil
}

func resolveUlimitsFromRaws(raws []string, r *ExpressionResolver) ([]container.Ulimit, error) {
	if len(raws) == 0 {
		return []container.Ulimit{}, nil
	}

	res := make([]container.Ulimit, 0, len(raws))
	for _, raw := range raws {
		resolvedRaw := raw
		if r != nil {
			resolvedRaw = r.resolveString(raw)
			if err := r.Error(); err != nil {
				return nil, err
			}
		}

		if err := validatePathChars(resolvedRaw); err != nil {
			return nil, &InvalidConfigError{Field: "ulimit", Value: raw, Err: err}
		}
		parsed, err := units.ParseUlimit(resolvedRaw)
		if err != nil {
			return nil, &InvalidConfigError{Field: "ulimit", Value: raw, Err: err}
		}
		if parsed.Hard < -1 || parsed.Soft < -1 {
			return nil, &InvalidConfigError{Field: "ulimit", Value: raw, Err: fmt.Errorf("limit values must be at least -1")}
		}
		res = append(res, container.Ulimit{
			Name: parsed.Name,
			Hard: parsed.Hard,
			Soft: parsed.Soft,
		})
	}
	return res, nil
}

func resolveUlimits(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) ([]container.Ulimit, error) {
	raws, err := pickConfigs(
		p1, p2, "CDERUN_ULIMIT", ",", subcommand, tools,
		func(t ToolConfig) []string { return t.Ulimits },
		global, func(g CDERunConfig) []string { return g.Defaults.Ulimits },
		nil,
		fs,
	)
	if err != nil {
		return nil, err
	}
	return resolveUlimitsFromRaws(raws, r)
}

func resolveSysctlsFromRaws(raws []string, r *ExpressionResolver) (map[string]string, error) {
	if len(raws) == 0 {
		return nil, nil
	}

	res := make(map[string]string, len(raws))
	for _, raw := range raws {
		resolvedRaw := raw
		if r != nil {
			resolvedRaw = r.resolveString(raw)
			if err := r.Error(); err != nil {
				return nil, err
			}
		}

		key, val, found := strings.Cut(resolvedRaw, "=")
		if !found || strings.TrimSpace(key) == "" {
			return nil, &InvalidConfigError{
				Field: "sysctl",
				Value: raw,
				Err:   fmt.Errorf("sysctl parameter must be in key=value format"),
			}
		}

		if key != strings.TrimSpace(key) {
			return nil, &InvalidConfigError{
				Field: "sysctl",
				Value: raw,
				Err:   fmt.Errorf("security validation failed for sysctl key: leading or trailing whitespace detected"),
			}
		}

		if err := validatePathChars(key); err != nil {
			return nil, &InvalidConfigError{
				Field: "sysctl",
				Value: raw,
				Err:   fmt.Errorf("security validation failed for sysctl key (null byte injection or invalid control characters): %w", err),
			}
		}
		if err := validatePathChars(val); err != nil {
			return nil, &InvalidConfigError{
				Field: "sysctl",
				Value: raw,
				Err:   fmt.Errorf("security validation failed for sysctl value (null byte injection or invalid control characters): %w", err),
			}
		}

		k := strings.TrimSpace(key)
		if err := ValidateSysctlKey(k); err != nil {
			return nil, &InvalidConfigError{
				Field: "sysctl",
				Value: raw,
				Err:   fmt.Errorf("security validation failed for sysctl key (null byte injection or invalid control characters): %w", err),
			}
		}
		if err := ValidateSysctlValue(val); err != nil {
			return nil, &InvalidConfigError{
				Field: "sysctl",
				Value: raw,
				Err:   fmt.Errorf("security validation failed for sysctl value (null byte injection or invalid control characters): %w", err),
			}
		}

		res[k] = val
	}
	return res, nil
}

func resolveSysctls(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) (map[string]string, error) {
	raws, err := pickConfigs(
		p1, p2, "CDERUN_SYSCTL", ",", subcommand, tools,
		func(t ToolConfig) []string { return t.Sysctls },
		global, func(g CDERunConfig) []string { return g.Defaults.Sysctls },
		nil,
		fs,
	)
	if err != nil {
		return nil, err
	}
	return resolveSysctlsFromRaws(raws, r)
}

func parseEnvItem[T any](s string, parser func(string, string) (T, error)) (T, error) {
	if parser == nil {
		if sv, ok := any(s).(T); ok {
			return sv, nil
		}
		var zero T
		return zero, errors.New("parser required for non-string types")
	}
	return parser(s, "env")
}

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
		return parseSlice(p1, "override", parser)
	}
	if p2 != nil {
		return parseSlice(p2, "", parser)
	}
	if envKey != "" {
		if env, ok := fs.LookupEnv(envKey); ok {
			var res []T
			if envSep == "" || !strings.Contains(env, envSep) {
				s := strings.TrimSpace(env)
				if s != "" {
					v, err := parseEnvItem(s, parser)
					if err != nil {
						return nil, err
					}
					res = []T{v}
				} else {
					res = []T{}
				}
			} else {
				for s := range strings.SplitSeq(env, envSep) {
					s = strings.TrimSpace(s)
					if s == "" {
						continue
					}
					v, err := parseEnvItem(s, parser)
					if err != nil {
						return nil, err
					}
					if res == nil {
						res = make([]T, 0, 4)
					}
					res = append(res, v)
				}
				if res == nil {
					res = []T{}
				}
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

func resolveDevicesFromConfigs(dcs []DeviceConfig, r *ExpressionResolver) ([]container.DeviceMapping, error) {
	if len(dcs) == 0 {
		return nil, nil
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
			if r != nil {
				parsed.SetBaseDir(r.Pwd)
			}
			return parsed, nil
		},
		fs,
	)
	if err != nil {
		return nil, err
	}
	return resolveDevicesFromConfigs(dcs, r)
}

func resolveEnv(p1 []string, p2 []string, envKey string, subcommand string, tools ToolsConfig, global *CDERunConfig, sensitivePatterns []string, strict bool, r *ExpressionResolver, fs FileSystem) ([]string, error) {
	envs, err := pickConfigs(
		p1, p2, envKey, ";", subcommand, tools,
		func(t ToolConfig) []string { return t.Env },
		global, func(g CDERunConfig) []string { return g.Defaults.Env },
		nil,
		fs,
	)
	if err != nil {
		return nil, err
	}

	// Deduplicate within the winning source (last-one-wins for the same key)
	var merged []string
	if len(envs) > 0 {
		merged = deduplicateEnv(envs)
	}

	return resolveEnvValues(merged, sensitivePatterns, strict, r, fs)
}

func addEnv(m map[string]string, keys *[]string, env []string) {
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if _, ok := m[key]; !ok {
			*keys = append(*keys, key)
		}
		m[key] = e
	}
}

func deduplicateEnv(env []string) []string {
	if len(env) <= 1 {
		return env
	}
	if len(env) <= 8 {
		var keys [8]string
		var vals [8]string
		size := 0
		hasDuplicates := false

		for _, e := range env {
			key, _, _ := strings.Cut(e, "=")
			foundIdx := -1
			for j := 0; j < size; j++ {
				if keys[j] == key {
					foundIdx = j
					break
				}
			}
			if foundIdx >= 0 && foundIdx < 8 {
				//nolint:gosec // false positive G602: bounds checked above
				vals[foundIdx] = e
				hasDuplicates = true
			} else {
				keys[size] = key
				vals[size] = e
				size++
			}
		}

		if !hasDuplicates {
			return env
		}

		res := make([]string, size)
		copy(res, vals[:size])
		return res
	}

	m := make(map[string]string, len(env))
	keys := make([]string, 0, len(env))
	addEnv(m, &keys, env)

	if len(keys) == len(env) {
		return env
	}

	res := make([]string, 0, len(keys))
	for _, k := range keys {
		res = append(res, m[k])
	}
	return res
}

func addEnvSmall(keys *[8]string, vals *[8]string, size *int, env []string) {
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		foundIdx := -1
		for j := 0; j < *size; j++ {
			if keys[j] == key {
				foundIdx = j
				break
			}
		}
		if foundIdx >= 0 && foundIdx < 8 {
			//nolint:gosec // false positive G602: bounds checked above
			vals[foundIdx] = e
		} else if *size < 8 {
			keys[*size] = key
			vals[*size] = e
			(*size)++
		}
	}
}

func mergeEnv(base, p2, p1 []string) []string {
	total := len(base) + len(p2) + len(p1)
	if total == 0 {
		return nil
	}
	// Optimization: if only one source is non-empty, use deduplicateEnv
	if len(base) > 0 && len(p2) == 0 && len(p1) == 0 {
		return deduplicateEnv(base)
	}
	if len(base) == 0 && len(p2) > 0 && len(p1) == 0 {
		return deduplicateEnv(p2)
	}
	if len(base) == 0 && len(p2) == 0 && len(p1) > 0 {
		return deduplicateEnv(p1)
	}

	if total <= 8 {
		var keys [8]string
		var vals [8]string
		size := 0

		addEnvSmall(&keys, &vals, &size, base)
		addEnvSmall(&keys, &vals, &size, p2)
		addEnvSmall(&keys, &vals, &size, p1)

		res := make([]string, size)
		copy(res, vals[:size])
		return res
	}

	m := make(map[string]string, total)
	keys := make([]string, 0, total)

	addEnv(m, &keys, base)
	addEnv(m, &keys, p2)
	addEnv(m, &keys, p1)

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

func resolveEnvValues(env []string, sensitivePatterns []string, strict bool, r *ExpressionResolver, fs FileSystem) ([]string, error) {
	if len(env) == 0 {
		return []string{}, nil
	}
	var res []string
	for i, e := range env {
		resolvedE := e
		if r != nil {
			resolvedE = r.resolveString(e)
			if err := r.Error(); err != nil {
				return nil, err
			}
		}

		key, val, hasValue := strings.Cut(resolvedE, "=")
		if err := ValidateEnvKey(key); err != nil {
			return nil, fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
		if !hasValue {
			v, found := fs.LookupEnv(key)
			if !found && strict {
				return nil, fmt.Errorf("required environment variable not found: %q", key)
			}
			val = v
		}

		if strings.ContainsRune(val, 0) {
			return nil, fmt.Errorf("security validation failed for env[%d] (value): null byte injection detected", i)
		}
		hasControlOrNonASCII := false
		for j := 0; j < len(val); j++ {
			b := val[j]
			if (b < 0x20 && b != '\n' && b != '\r' && b != '\t') || b == 0x7f || b >= 0x80 {
				hasControlOrNonASCII = true
				break
			}
		}
		if hasControlOrNonASCII {
			for pos, r := range val {
				if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
					return nil, fmt.Errorf("security validation failed for env[%d] (value): invalid control character %q at position %d", i, r, pos)
				}
			}
		}

		// Optimization: if key and val are unchanged, reuse the original string if it's already in key=val format.
		final := ""
		if len(e) == len(key)+1+len(val) && strings.HasPrefix(e, key) && e[len(key)] == '=' && e[len(key)+1:] == val {
			final = e
		} else {
			final = key + "=" + val
		}

		// Apply masking for debug logs and quoting for safety
		if logging.DebugEnabled() {
			logging.Debug("Resolved Env: %q=%q", key, MaskSensitiveEnv(key, val, sensitivePatterns))
		}

		if res == nil && final != e {
			// Allocation needed, copy previous unchanged entries
			res = make([]string, 0, len(env))
			res = append(res, env[:i]...)
		}
		if res != nil {
			res = append(res, final)
		}
	}
	if res == nil {
		return env, nil
	}
	return res, nil
}

func resolveMountsFromConfigs(mcs []MountConfig, r *ExpressionResolver, fs FileSystem) ([]container.Mount, error) {
	if len(mcs) == 0 {
		return []container.Mount{}, nil
	}

	res := make([]container.Mount, 0, len(mcs))
	for _, mc := range mcs {
		if mc.Optional && (mc.Type == "bind" || mc.Type == "") && !mc.Source.IsEmpty() {
			hostPath, err := mc.Source.Resolve(r)
			if err != nil {
				return nil, err
			}
			var statErr error
			if r != nil {
				_, statErr = r.Stat(hostPath)
			} else if fs != nil {
				_, statErr = fs.Stat(hostPath)
			} else {
				_, statErr = os.Stat(hostPath)
			}
			if statErr != nil {
				if errors.Is(statErr, os.ErrNotExist) {
					// Skip if source doesn't exist
					continue
				}
				return nil, statErr
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
			if r != nil {
				parsed.SetBaseDir(r.Pwd)
			} else if fs != nil {
				if wd, err := fs.Getwd(); err == nil {
					parsed.SetBaseDir(wd)
				}
			}
			return parsed, nil
		},
		fs,
	)
	if err != nil {
		return nil, err
	}
	return resolveMountsFromConfigs(mcs, r, fs)
}

func needsResolverForMounts(mcs []MountConfig, global *CDERunConfig) bool {
	if len(mcs) == 0 {
		return false
	}
	if global != nil && global.HostContext != nil && global.HostContext.Level > 0 {
		return true
	}
	for _, mc := range mcs {
		if strings.Contains(mc.Source.Raw, "{{") || strings.HasPrefix(mc.Source.Raw, "~") ||
			strings.Contains(mc.Target.Raw, "{{") || strings.HasPrefix(mc.Target.Raw, "~") ||
			mc.Optional {
			return true
		}
	}
	return false
}

func needsResolverForDevices(dcs []DeviceConfig, global *CDERunConfig) bool {
	if len(dcs) == 0 {
		return false
	}
	if global != nil && global.HostContext != nil && global.HostContext.Level > 0 {
		return true
	}
	for _, dc := range dcs {
		if strings.Contains(dc.Source.Raw, "{{") || strings.HasPrefix(dc.Source.Raw, "~") ||
			strings.Contains(dc.Destination.Raw, "{{") || strings.HasPrefix(dc.Destination.Raw, "~") {
			return true
		}
	}
	return false
}
