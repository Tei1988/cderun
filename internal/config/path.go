package config

import (
	"cderun/internal/container"
	"os"
	"path/filepath"
	"regexp"
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

// ResolveVolume expands expressions and resolves the volume host path relative to BaseDir.
func (cp ConfigPath) ResolveVolume(r *ExpressionResolver) string {
	if cp.IsEmpty() {
		return ""
	}
	resolved := r.resolveString(cp.Raw)
	return resolveVolumePath(resolved, cp.BaseDir)
}

// ResolveDevice expands expressions and resolves the device host path relative to BaseDir.
func (cp ConfigPath) ResolveDevice(r *ExpressionResolver) string {
	if cp.IsEmpty() {
		return ""
	}
	resolved := r.resolveString(cp.Raw)
	return resolveDevicePath(resolved, cp.BaseDir)
}

// VolumeConfig is an intermediate representation for volume mappings.
type VolumeConfig struct {
	Source      ConfigPath
	Destination ConfigPath
	ReadOnly    bool
}

func (vc *VolumeConfig) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	if parsed, ok := ParseVolumeConfig(s); ok {
		*vc = parsed
	}
	return nil
}

func (vc VolumeConfig) IsEmpty() bool {
	return vc.Source.IsEmpty() && vc.Destination.IsEmpty()
}

func (vc *VolumeConfig) SetBaseDir(baseDir string) {
	if vc.Source.Raw != "" {
		vc.Source.BaseDir = baseDir
	}
	if vc.Destination.Raw != "" {
		vc.Destination.BaseDir = baseDir
	}
}

func (vc VolumeConfig) Resolve(r *ExpressionResolver) container.VolumeMount {
	return container.VolumeMount{
		HostPath:      vc.Source.Resolve(r),
		ContainerPath: vc.Destination.Resolve(r),
		ReadOnly:      vc.ReadOnly,
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
	if parsed, ok := ParseDeviceConfig(s); ok {
		*dc = parsed
	}
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

func ParseVolumeConfig(v string) (VolumeConfig, bool) {
	if v == "" {
		return VolumeConfig{}, false
	}

	host, remainder, ok := SplitHostRemainder(v)
	if !ok {
		return VolumeConfig{}, false
	}

	var containerPath string
	var readOnly bool

	lastColon := strings.LastIndex(remainder, ":")
	if lastColon != -1 {
		mode := remainder[lastColon+1:]
		if mode == "ro" || mode == "rw" {
			readOnly = (mode == "ro")
			containerPath = remainder[:lastColon]
		} else {
			containerPath = remainder
		}
	} else {
		containerPath = remainder
	}

	if host == "" || containerPath == "" {
		return VolumeConfig{}, false
	}

	return VolumeConfig{
		Source:      ConfigPath{Raw: host},
		Destination: ConfigPath{Raw: containerPath},
		ReadOnly:    readOnly,
	}, true
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
