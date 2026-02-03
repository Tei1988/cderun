package config

import (
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
	host, remainder, ok := splitHostRemainder(v)
	if !ok {
		return v
	}
	return resolvePath(host, baseDir) + ":" + remainder
}

func resolveDevicePath(d string, baseDir string) string {
	// host-path:container-path[:permissions]
	host, remainder, ok := splitHostRemainder(d)
	if !ok {
		return d
	}
	return resolvePath(host, baseDir) + ":" + remainder
}

func splitHostRemainder(s string) (string, string, bool) {
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
