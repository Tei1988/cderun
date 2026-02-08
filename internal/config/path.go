package config

import (
	"cderun/internal/container"
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
	return ResolvePath(resolved, cp.BaseDir, r)
}

// ResolveVolume expands expressions and resolves the volume host path relative to BaseDir.
func (cp ConfigPath) ResolveVolume(r *ExpressionResolver) string {
	if cp.IsEmpty() {
		return ""
	}
	resolved := r.resolveString(cp.Raw)
	return resolveVolumePath(resolved, cp.BaseDir, r)
}

// ResolveDevice expands expressions and resolves the device host path relative to BaseDir.
func (cp ConfigPath) ResolveDevice(r *ExpressionResolver) string {
	if cp.IsEmpty() {
		return ""
	}
	resolved := r.resolveString(cp.Raw)
	return resolveDevicePath(resolved, cp.BaseDir, r)
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
		source = mc.Source.Resolve(r)
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
		PathOnHost:        dc.Source.Resolve(r),
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

// ResolvePath resolves the path p against baseDir, expanding a leading tilde, preserving a scheme prefix (e.g., "http://"), and remapping paths through host mounts when a nested host context is present in r.
// 
// If p is a relative path beginning with "./", "../", "." or "..", it is joined with baseDir. A leading "~" is expanded to the current user's home directory when available. If r and r.HostContext are provided and HostContext.Level > 0, the function attempts a reverse resolution: it finds the best matching host mount whose Target contains the absolute path and rewrites the path to use that mount's Source.
// 
// The returned string is the resolved path with the original scheme prefix (if any) reattached.
func ResolvePath(p string, baseDir string, r *ExpressionResolver) string {
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

	absPath := filepath.Clean(p)

	// Reverse Path Resolution for Nested Execution
	if r != nil && r.HostContext != nil && r.HostContext.Level > 0 {
		abs, err := filepath.Abs(absPath)
		if err == nil {
			bestMatch := -1
			bestSource := ""
			bestTarget := ""
			maxLevel := -1

			for _, m := range r.HostContext.Mounts {
				rel, err := filepath.Rel(m.Target, abs)
				if err == nil && !strings.HasPrefix(rel, "..") {
					// It's a match. Ensure it's not a partial segment match.
					// filepath.Rel already handles this correctly (if it doesn't start with .. and it's a match).
					// But we should double check if the match is exact or follows a separator.

					// Longest match and highest level priority
					if m.Level > maxLevel || (m.Level == maxLevel && len(m.Target) > len(bestTarget)) {
						maxLevel = m.Level
						bestTarget = m.Target
						bestSource = m.Source
						bestMatch = 1
					}
				}
			}

			if bestMatch != -1 {
				rel, err := filepath.Rel(bestTarget, abs)
				if err == nil {
					absPath = filepath.Join(bestSource, rel)
				}
			}
		}
	}

	return prefix + absPath
}

var winDriveRegex = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// resolveVolumePath resolves the host portion of a volume specification and returns the reconstructed volume string.
// If the input contains a host and remainder separated by the recognized colon syntax, the host is resolved via ResolvePath
// and concatenated with ":" and the remainder; otherwise the input is returned unchanged.
func resolveVolumePath(v string, baseDir string, r *ExpressionResolver) string {
	host, remainder, ok := SplitHostRemainder(v)
	if !ok {
		return v
	}
	return ResolvePath(host, baseDir, r) + ":" + remainder
}

// via ResolvePath and the function returns "resolved-host:remainder".
func resolveDevicePath(d string, baseDir string, r *ExpressionResolver) string {
	// host-path:container-path[:permissions]
	host, remainder, ok := SplitHostRemainder(d)
	if !ok {
		return d
	}
	return ResolvePath(host, baseDir, r) + ":" + remainder
}

// SplitHostRemainder splits s into a host (text before the separator) and a remainder (text after the separator)
// using the first ':' as the separator and reports whether a valid separator was found.
// If s contains a leading Windows drive letter (e.g. "C:\" or "C:/"), that first ':' is considered part of the path
// and the function uses the next ':' as the separator. It returns the host, the remainder, and true on success;
// otherwise it returns empty strings and false.
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