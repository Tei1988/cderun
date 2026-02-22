package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"cderun/internal/container"

	"gopkg.in/yaml.v3"
)

// ConfigPath is an intermediate representation for paths in configuration.
// It stores the raw value before expression expansion and the base directory for relative path resolution.
type ConfigPath struct {
	Raw     string
	BaseDir string
}

func (cp ConfigPath) DeepCopy() ConfigPath {
	return cp
}

func (cp *ConfigPath) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&cp.Raw)
}

func (cp ConfigPath) MarshalYAML() (any, error) {
	if cp.IsEmpty() {
		return nil, nil
	}
	return cp.Raw, nil
}

func (cp ConfigPath) IsEmpty() bool {
	return cp.Raw == ""
}

// Resolve expands expressions and resolves the path relative to BaseDir.
func (cp ConfigPath) Resolve(r *ExpressionResolver) (string, error) {
	if cp.IsEmpty() {
		return "", nil
	}
	return ResolvePath(cp.Raw, cp.BaseDir, r)
}

// ResolveVolume expands expressions and resolves the volume host path relative to BaseDir.
func (cp ConfigPath) ResolveVolume(r *ExpressionResolver) (string, error) {
	if cp.IsEmpty() {
		return "", nil
	}
	return resolveVolumePath(cp.Raw, cp.BaseDir, r)
}

// ResolveDevice expands expressions and resolves the device host path relative to BaseDir.
func (cp ConfigPath) ResolveDevice(r *ExpressionResolver) (string, error) {
	if cp.IsEmpty() {
		return "", nil
	}
	return resolveDevicePath(cp.Raw, cp.BaseDir, r)
}

// MountConfig is an intermediate representation for mount points in configuration.
type MountConfig struct {
	Type     string
	Source   ConfigPath
	Target   ConfigPath
	ReadOnly bool
}

func (mc MountConfig) DeepCopy() MountConfig {
	return mc
}

func (mc *MountConfig) UnmarshalYAML(node *yaml.Node) error {
	// Support structure format
	var a struct {
		Type     string     `yaml:"type"`
		Source   ConfigPath `yaml:"source"`
		Target   ConfigPath `yaml:"target"`
		ReadOnly bool       `yaml:"read_only"`
	}
	if err := node.Decode(&a); err == nil {
		mc.Type = a.Type
		if mc.Type == "" {
			mc.Type = "bind"
		}
		mc.Source = a.Source
		mc.Target = a.Target
		mc.ReadOnly = a.ReadOnly

		if mc.Target.IsEmpty() {
			return fmt.Errorf("mount target is required at line %d (tag %s)", node.Line, node.Tag)
		}
		return nil
	}

	return fmt.Errorf("invalid mount config at line %d (tag %s): %v", node.Line, node.Tag, node.Value)
}

func (mc MountConfig) IsEmpty() bool {
	return mc.Target.IsEmpty()
}

func (mc MountConfig) MarshalYAML() (any, error) {
	if mc.IsEmpty() {
		return nil, nil
	}
	// We use an anonymous struct to avoid infinite recursion
	var a struct {
		Type     string     `yaml:"type,omitempty"`
		Source   ConfigPath `yaml:"source,omitempty"`
		Target   ConfigPath `yaml:"target"`
		ReadOnly bool       `yaml:"read_only,omitempty"`
	}
	a.Type = mc.Type
	a.Source = mc.Source
	a.Target = mc.Target
	a.ReadOnly = mc.ReadOnly
	return a, nil
}

func (mc *MountConfig) SetBaseDir(baseDir string) {
	if mc.Source.Raw != "" {
		mc.Source.BaseDir = baseDir
	}
	if mc.Target.Raw != "" {
		mc.Target.BaseDir = baseDir
	}
}

func (mc MountConfig) Resolve(r *ExpressionResolver) (container.Mount, error) {
	source := ""
	if mc.Type == "bind" {
		s, err := mc.Source.Resolve(r)
		if err != nil {
			return container.Mount{}, err
		}
		source = s
	} else {
		if !mc.Source.IsEmpty() {
			s, err := mc.Source.Resolve(r)
			if err != nil {
				return container.Mount{}, err
			}
			source = s
		}
	}

	target, err := mc.Target.Resolve(r)
	if err != nil {
		return container.Mount{}, err
	}

	return container.Mount{
		Type:     mc.Type,
		Source:   source,
		Target:   target,
		ReadOnly: mc.ReadOnly,
	}, nil
}

// DeviceConfig is an intermediate representation for device mappings.
type DeviceConfig struct {
	Source      ConfigPath
	Destination ConfigPath
	Permissions string
}

func (dc DeviceConfig) DeepCopy() DeviceConfig {
	return dc
}

func (dc *DeviceConfig) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	parsed, ok := ParseDeviceConfig(s)
	if !ok {
		return fmt.Errorf("invalid device config: %s", s)
	}
	*dc = parsed
	return nil
}

func (dc DeviceConfig) IsEmpty() bool {
	return dc.Source.IsEmpty() && dc.Destination.IsEmpty()
}

func (dc DeviceConfig) MarshalYAML() (any, error) {
	if dc.IsEmpty() {
		return nil, nil
	}
	// Marshal back to string format host:container[:perms]
	res := dc.Source.Raw + ":" + dc.Destination.Raw
	if dc.Permissions != "" && dc.Permissions != "rwm" {
		res += ":" + dc.Permissions
	}
	return res, nil
}

func (dc *DeviceConfig) SetBaseDir(baseDir string) {
	if dc.Source.Raw != "" {
		dc.Source.BaseDir = baseDir
	}
	if dc.Destination.Raw != "" {
		dc.Destination.BaseDir = baseDir
	}
}

func (dc DeviceConfig) Resolve(r *ExpressionResolver) (container.DeviceMapping, error) {
	host, err := dc.Source.Resolve(r)
	if err != nil {
		return container.DeviceMapping{}, err
	}
	containerPath, err := dc.Destination.Resolve(r)
	if err != nil {
		return container.DeviceMapping{}, err
	}
	return container.DeviceMapping{
		PathOnHost:        host,
		PathInContainer:   containerPath,
		CgroupPermissions: dc.Permissions,
	}, nil
}

func ParseMountFlag(s string) (MountConfig, error) {
	parts := strings.Split(s, ",")
	res := MountConfig{
		Type: "bind", // Default type
	}

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			if part == "readonly" {
				res.ReadOnly = true
				continue
			}
			return MountConfig{}, fmt.Errorf("invalid mount format: %s", s)
		}

		key := kv[0]
		val := kv[1]

		switch key {
		case "type":
			res.Type = val
		case "source", "src":
			res.Source = ConfigPath{Raw: val}
		case "target", "dst", "destination":
			res.Target = ConfigPath{Raw: val}
		case "readonly":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return MountConfig{}, fmt.Errorf("invalid readonly value: %s", val)
			}
			res.ReadOnly = b
		default:
		}
	}

	if res.Target.IsEmpty() {
		return MountConfig{}, fmt.Errorf("mount target is required: %s", s)
	}

	return res, nil
}

func ParseDeviceConfig(d string) (DeviceConfig, bool) {
	if d == "" {
		return DeviceConfig{}, false
	}

	host, remainder, ok := SplitHostRemainder(d)
	if !ok {
		return DeviceConfig{
			Source:      ConfigPath{Raw: d},
			Destination: ConfigPath{Raw: d},
			Permissions: "rwm",
		}, true
	}

	var containerPath string
	permissions := "rwm"

	lastColon := strings.LastIndex(remainder, ":")
	if lastColon != -1 {
		perms := remainder[lastColon+1:]
		if permsRegex.MatchString(perms) {
			permissions = perms
			containerPath = remainder[:lastColon]
		} else {
			containerPath = remainder
		}
	} else {
		containerPath = remainder
	}

	if host == "" || containerPath == "" {
		return DeviceConfig{}, false
	}

	return DeviceConfig{
		Source:      ConfigPath{Raw: host},
		Destination: ConfigPath{Raw: containerPath},
		Permissions: permissions,
	}, true
}

var (
	schemeRegex = regexp.MustCompile(`^[a-z]+://`)
	permsRegex  = regexp.MustCompile(`^[rwm]+$`)
)

// ResolvePath resolves expressions, expands tilde, and handles relative paths.
func ResolvePath(p string, baseDir string, r *ExpressionResolver) (string, error) {
	if p == "" {
		return p, nil
	}

	prefix := schemeRegex.FindString(p)
	p = strings.TrimPrefix(p, prefix)

	var fs FileSystem = RealFileSystem{}
	if r != nil && r.fs != nil {
		fs = r.fs
	}

	if r != nil {
		resolved, err := r.ResolveString(p)
		if err != nil {
			return "", err
		}
		p = resolved
	} else {
		expanded, err := expandHome(p, fs)
		if err != nil {
			return "", err
		}
		p = expanded
	}

	if !filepath.IsAbs(p) && (strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || p == "." || p == "..") {
		p = filepath.Join(baseDir, p)
	}

	absPath := filepath.Clean(p)

	if r != nil && r.HostContext != nil && r.HostContext.Level > 0 {
		abs := absPath
		if !filepath.IsAbs(abs) {
			wd, err := fs.Getwd()
			if err == nil {
				abs = filepath.Join(wd, abs)
			}
		}

		found := false
		bestRel := ""
		bestSource := ""
		bestTarget := ""
		maxLevel := -1

		for _, m := range r.HostContext.Mounts {
			rel, err := filepath.Rel(m.Target, abs)
			if err == nil && !strings.HasPrefix(rel, "..") {
				if m.Level > maxLevel || (m.Level == maxLevel && len(m.Target) > len(bestTarget)) {
					maxLevel = m.Level
					bestTarget = m.Target
					bestSource = m.Source
					bestRel = rel
					found = true
				}
			}
		}

		if found {
			absPath = filepath.Join(bestSource, bestRel)
		}
	}

	return prefix + absPath, nil
}

var winDriveRegex = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func resolveVolumePath(v string, baseDir string, r *ExpressionResolver) (string, error) {
	host, remainder, ok := SplitHostRemainder(v)
	if !ok {
		return ResolvePath(v, baseDir, r)
	}
	resolvedHost, err := ResolvePath(host, baseDir, r)
	if err != nil {
		return "", err
	}
	return resolvedHost + ":" + remainder, nil
}

func resolveDevicePath(d string, baseDir string, r *ExpressionResolver) (string, error) {
	host, remainder, ok := SplitHostRemainder(d)
	if !ok {
		return ResolvePath(d, baseDir, r)
	}
	resolvedHost, err := ResolvePath(host, baseDir, r)
	if err != nil {
		return "", err
	}
	return resolvedHost + ":" + remainder, nil
}

func SplitHostRemainder(s string) (string, string, bool) {
	sepIdx := strings.Index(s, ":")
	if sepIdx == -1 {
		return "", "", false
	}

	if winDriveRegex.MatchString(s) {
		nextSep := strings.Index(s[sepIdx+1:], ":")
		if nextSep == -1 {
			return "", "", false
		}
		sepIdx = sepIdx + 1 + nextSep
	}

	return s[:sepIdx], s[sepIdx+1:], true
}
