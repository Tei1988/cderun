package config

import (
	"cderun/internal/container"
	"cderun/internal/logging"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigPath is an intermediate representation for paths in configuration.
// It stores the raw value before expression expansion and the base directory for relative path resolution.
type ConfigPath struct {
	Raw     string
	BaseDir string
}

func (cp *ConfigPath) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&cp.Raw)
}

func (cp ConfigPath) MarshalYAML() (interface{}, error) {
	return cp.Raw, nil
}

func (cp ConfigPath) IsEmpty() bool {
	return cp.Raw == ""
}

// Resolve expands expressions and resolves the path relative to BaseDir.
func (cp ConfigPath) Resolve(r *ExpressionResolver) string {
	if cp.IsEmpty() {
		return ""
	}
	resolved := r.resolveString(cp.Raw)
	return resolvePath(resolved, cp.BaseDir)
}

// ResolveHost expands expressions, resolves the path relative to BaseDir, and translates it back to the Base Host path if needed.
func (cp ConfigPath) ResolveHost(r *ExpressionResolver) string {
	if cp.IsEmpty() {
		return ""
	}
	resolved := r.resolveString(cp.Raw)
	p := resolvePath(resolved, cp.BaseDir)
	if r.HostContext != nil {
		p = ResolveHostPath(p, r.HostContext.Mounts)
	}
	return p
}

// ResolveVolume expands expressions and resolves the volume host path relative to BaseDir.
func (cp ConfigPath) ResolveVolume(r *ExpressionResolver) string {
	if cp.IsEmpty() {
		return ""
	}
	resolved := r.resolveString(cp.Raw)
	p := resolveVolumePath(resolved, cp.BaseDir)
	if r.HostContext != nil {
		host, remainder, ok := SplitHostRemainder(p)
		if ok {
			p = ResolveHostPath(host, r.HostContext.Mounts) + ":" + remainder
		} else {
			p = ResolveHostPath(p, r.HostContext.Mounts)
		}
	}
	return p
}

// ResolveDevice expands expressions and resolves the device host path relative to BaseDir.
func (cp ConfigPath) ResolveDevice(r *ExpressionResolver) string {
	if cp.IsEmpty() {
		return ""
	}
	resolved := r.resolveString(cp.Raw)
	p := resolveDevicePath(resolved, cp.BaseDir)
	if r.HostContext != nil {
		host, remainder, ok := SplitHostRemainder(p)
		if ok {
			p = ResolveHostPath(host, r.HostContext.Mounts) + ":" + remainder
		} else {
			p = ResolveHostPath(p, r.HostContext.Mounts)
		}
	}
	return p
}

// MountConfig is an intermediate representation for mount points in configuration.
type MountConfig struct {
	Type     string
	Source   ConfigPath
	Target   ConfigPath
	ReadOnly bool
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

func (mc *MountConfig) SetBaseDir(baseDir string) {
	if mc.Source.Raw != "" {
		mc.Source.BaseDir = baseDir
	}
	if mc.Target.Raw != "" {
		mc.Target.BaseDir = baseDir
	}
}

func (mc MountConfig) Resolve(r *ExpressionResolver) container.Mount {
	source := ""
	if mc.Type == "bind" {
		source = mc.Source.ResolveHost(r)
	} else {
		source = r.resolveString(mc.Source.Raw)
	}

	return container.Mount{
		Type:     mc.Type,
		Source:   source,
		Target:   mc.Target.Resolve(r),
		ReadOnly: mc.ReadOnly,
	}
}

// DeviceConfig is an intermediate representation for device mappings.
type DeviceConfig struct {
	Source      ConfigPath
	Destination ConfigPath
	Permissions string
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

func (dc *DeviceConfig) SetBaseDir(baseDir string) {
	if dc.Source.Raw != "" {
		dc.Source.BaseDir = baseDir
	}
	if dc.Destination.Raw != "" {
		dc.Destination.BaseDir = baseDir
	}
}

func (dc DeviceConfig) Resolve(r *ExpressionResolver) container.DeviceMapping {
	return container.DeviceMapping{
		PathOnHost:        dc.Source.ResolveHost(r),
		PathInContainer:   dc.Destination.Resolve(r),
		CgroupPermissions: dc.Permissions,
	}
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
			// Ignore unknown options for now (like tmpfs-size)
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
		// Support single path like /dev/fuse
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
		// Basic check for permissions format (usually combinations of r, w, m)
		if regexp.MustCompile(`^[rwm]+$`).MatchString(perms) {
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

var schemeRegex = regexp.MustCompile(`^[a-z]+://`)

func resolvePath(p string, baseDir string) string {
	if p == "" {
		return p
	}

	prefix := schemeRegex.FindString(p)
	p = strings.TrimPrefix(p, prefix)

	// Tilde expansion
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				p = home
			} else {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	// Relative path resolution
	if !filepath.IsAbs(p) && (strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || p == "." || p == "..") {
		p = filepath.Join(baseDir, p)
	}
	return prefix + filepath.Clean(p)
}

var winDriveRegex = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func resolveVolumePath(v string, baseDir string) string {
	host, remainder, ok := SplitHostRemainder(v)
	if !ok {
		return v
	}
	return resolvePath(host, baseDir) + ":" + remainder
}

func resolveDevicePath(d string, baseDir string) string {
	// host-path:container-path[:permissions]
	host, remainder, ok := SplitHostRemainder(d)
	if !ok {
		return d
	}
	return resolvePath(host, baseDir) + ":" + remainder
}

func ResolveHostPath(p string, mounts []HostMount) string {
	if len(mounts) == 0 {
		return p
	}

	logging.Trace("Resolving host path for: %s", p)

	// Ensure p is absolute for prefix matching
	absPath, err := filepath.Abs(p)
	if err != nil {
		absPath = p
	}

	var bestMatch *HostMount
	for i := range mounts {
		m := &mounts[i]
		// Target should be absolute in the HostContext
		if strings.HasPrefix(absPath, m.Target) {
			// Check if it's a full component match
			if len(absPath) == len(m.Target) || absPath[len(m.Target)] == filepath.Separator || m.Target == "/" {
				if bestMatch == nil || m.Level > bestMatch.Level || (m.Level == bestMatch.Level && len(m.Target) > len(bestMatch.Target)) {
					bestMatch = m
				}
			}
		}
	}

	if bestMatch != nil {
		rel, err := filepath.Rel(bestMatch.Target, absPath)
		if err == nil {
			resolved := filepath.Join(bestMatch.Source, rel)
			logging.Trace("Resolved %s to host path %s via target %s", p, resolved, bestMatch.Target)
			return resolved
		}
	}

	return p
}

func DiscoverHostRoot() string {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// Look for root mount
		if !strings.Contains(line, " / / ") {
			continue
		}

		// mountinfo format:
		// 36 35 98:0 /mnt1 /mnt2 rw,relatime master:1 - ext3 /dev/root rw,errors=continue
		// separator is " - "
		parts := strings.Split(line, " - ")
		if len(parts) < 2 {
			continue
		}

		// After " - ", fields are: type, source, data
		tail := strings.SplitN(parts[1], " ", 3)
		if len(tail) < 3 {
			continue
		}

		fstype := tail[0]
		if fstype != "overlay" {
			continue
		}

		mountData := tail[2]
		opts := strings.Split(mountData, ",")
		for _, opt := range opts {
			if strings.HasPrefix(opt, "upperdir=") {
				return strings.TrimPrefix(opt, "upperdir=")
			}
		}
	}

	return ""
}

func SplitHostRemainder(s string) (string, string, bool) {
	sepIdx := strings.Index(s, ":")
	if sepIdx == -1 {
		return "", "", false
	}

	// If it's a Windows drive letter (e.g. C:\ or C:/), the first colon is part of the path.
	// We need to look for the separator colon after the drive letter (index > 1).
	if winDriveRegex.MatchString(s) {
		nextSep := strings.Index(s[sepIdx+1:], ":")
		if nextSep == -1 {
			return "", "", false
		}
		sepIdx = sepIdx + 1 + nextSep
	}

	return s[:sepIdx], s[sepIdx+1:], true
}
