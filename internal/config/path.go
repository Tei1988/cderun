package config

import (
	"fmt"
	"net"
	"path"
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
	Optional bool
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
		Optional bool       `yaml:"optional"`
	}
	if err := node.Decode(&a); err == nil {
		mc.Type = a.Type
		if mc.Type == "" {
			mc.Type = "bind"
		}
		mc.Source = a.Source
		mc.Target = a.Target
		mc.ReadOnly = a.ReadOnly
		mc.Optional = a.Optional

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
		Optional bool       `yaml:"optional,omitempty"`
	}
	a.Type = mc.Type
	a.Source = mc.Source
	a.Target = mc.Target
	a.ReadOnly = mc.ReadOnly
	a.Optional = mc.Optional
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

	// Resolve the target using WithoutHostContext to avoid reverse resolution on the target itself.
	target, err := mc.Target.Resolve(r.WithoutHostContext())
	if err != nil {
		return container.Mount{}, err
	}
	// Check if it's absolute after resolution (Feedback point 3)
	if target != "" && !filepath.IsAbs(target) {
		return container.Mount{}, fmt.Errorf("mount target must be an absolute path: %q", target)
	}

	return container.Mount{
		Type:     mc.Type,
		Source:   source,
		Target:   target,
		ReadOnly: mc.ReadOnly,
		Optional: mc.Optional,
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
		return fmt.Errorf("invalid device config: %q", s)
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
	containerPath, err := dc.Destination.Resolve(r.WithoutHostContext())
	if err != nil {
		return container.DeviceMapping{}, err
	}
	// Check if it's absolute after resolution (Feedback point 3)
	if containerPath != "" && !filepath.IsAbs(containerPath) {
		return container.DeviceMapping{}, fmt.Errorf("device destination must be an absolute path: %q", containerPath)
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
			if part == "optional" {
				res.Optional = true
				continue
			}
			return MountConfig{}, fmt.Errorf("invalid mount format: %q", s)
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
				return MountConfig{}, fmt.Errorf("invalid readonly value: %q", val)
			}
			res.ReadOnly = b
		case "optional":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return MountConfig{}, fmt.Errorf("invalid optional value: %q", val)
			}
			res.Optional = b
		default:
		}
	}

	if res.Target.IsEmpty() {
		return MountConfig{}, fmt.Errorf("mount target is required: %q", s)
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
	schemeRegex       = regexp.MustCompile(`^[a-z]+://`)
	permsRegex        = regexp.MustCompile(`^[rwm]+$`)
	magicWordPreRegex = regexp.MustCompile(`^(~)([/\\]|$)`)

	hostnameRegex = regexp.MustCompile(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`)
	networkRegex  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	userPartRegex = regexp.MustCompile(`^([a-z_][a-z0-9_-]*[$]?|[0-9]+)$`)
	imageRegex    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-/:@]*$`)
	envKeyRegex   = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	capRegex      = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

// ResolvePath resolves expressions, expands tilde, and handles relative paths.
func ResolvePath(p string, baseDir string, r *ExpressionResolver) (string, error) {
	if p == "" {
		return p, nil
	}

	var fs FileSystem = RealFileSystem{}
	if r != nil && r.fs != nil {
		fs = r.fs
	}

	var res string
	if r != nil {
		resolved, err := r.ResolveString(p)
		if err != nil {
			return "", err
		}
		res = resolved
	} else {
		expanded, err := expandHome(p, fs)
		if err != nil {
			return "", err
		}
		res = expanded
	}

	// If expanded value contains a scheme, return it as is
	if schemeRegex.FindString(res) != "" {
		return res, nil
	}

	// Always join with baseDir for relative inputs (Feedback point 1)
	if !filepath.IsAbs(res) {
		res = filepath.Join(baseDir, res)
	}

	absPath := filepath.Clean(res)

	// Ensure absolute path via fs.Abs ONLY if we are in a nested context (Level > 0)
	if r != nil && r.HostContext != nil && r.HostContext.Level > 0 {
		abs, err := fs.Abs(absPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for %q: %w", absPath, err)
		}
		absPath = abs
	}

	if err := validateAnchorBoundaries(p, absPath, r, fs); err != nil {
		return "", err
	}

	if r != nil {
		resolvedAbs, err := r.applyReverseResolution(absPath)
		if err != nil {
			return "", err
		}
		absPath = resolvedAbs
	}

	return absPath, nil
}

var winDriveRegex = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func resolveVolumePath(v string, baseDir string, r *ExpressionResolver) (string, error) {
	host, remainder, ok := SplitHostRemainder(v)
	if !ok {
		resolved := v
		if r != nil {
			var err error
			resolved, err = r.ResolveString(v)
			if err != nil {
				return "", err
			}
		}
		if isNamedVolume(resolved) {
			return resolved, nil
		}
		return ResolvePath(v, baseDir, r)
	}

	resolvedHost := host
	if r != nil {
		var err error
		resolvedHost, err = r.ResolveString(host)
		if err != nil {
			return "", err
		}
	}

	if isNamedVolume(resolvedHost) {
		return resolvedHost + ":" + remainder, nil
	}

	finalHost, err := ResolvePath(host, baseDir, r)
	if err != nil {
		return "", err
	}
	return finalHost + ":" + remainder, nil
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

// findAnchors finds all top-level {{...}} expressions in a string, respecting nested braces.
func findAnchors(s string) []string {
	ranges := scanAnchors(s)
	if len(ranges) == 0 {
		return nil
	}
	res := make([]string, len(ranges))
	for i, r := range ranges {
		res[i] = s[r.start:r.end]
	}
	return res
}

// validatePathChars ensures the string does not contain ASCII control characters.
func validatePathChars(s string) error {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= 31 || c == 127 {
			return fmt.Errorf("invalid character in path or configuration: %q (position %d)", c, i)
		}
	}
	return nil
}

func validateAnchorBoundaries(original, resolved string, r *ExpressionResolver, fs FileSystem) error {
	hasTilde := strings.HasPrefix(original, "~") && (len(original) == 1 || original[1] == '/' || original[1] == '\\')
	exprAnchors := findAnchors(original)

	if !hasTilde && len(exprAnchors) == 0 {
		return nil
	}

	absResolved := resolved
	if !filepath.IsAbs(absResolved) {
		a, err := fs.Abs(resolved)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for resolved path %q: %w", resolved, err)
		}
		absResolved = a
	}
	absResolved = filepath.Clean(absResolved)

	processBoundary := func(anchorRaw, anchorPath string) error {
		if anchorPath == "" {
			return fmt.Errorf("anchor path is empty for %q", anchorRaw)
		}

		absAnchor, err := fs.Abs(anchorPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for anchor %q: %w", anchorPath, err)
		}
		absAnchor = filepath.Clean(absAnchor)

		rel, err := filepath.Rel(absAnchor, absResolved)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path between %q and %q: %w", absAnchor, absResolved, err)
		}

		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("path traversal detected: %q escapes anchor boundary %q", original, anchorPath)
		}
		return nil
	}

	if hasTilde {
		var anchorPath string
		if r != nil {
			anchorPath = r.Home
		} else {
			home, err := fs.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get anchor home directory: %w", err)
			}
			anchorPath = home
		}
		if err := processBoundary("~", anchorPath); err != nil {
			return err
		}
	}

	for _, anchor := range exprAnchors {
		if r == nil {
			return fmt.Errorf("expression resolver required for anchor validation")
		}
		// Use a fresh resolver instance for each anchor to ensure that a sticky error
		// from one anchor resolution doesn't skip subsequent anchor resolutions.
		// Reusing Home and Pwd from current resolver to avoid redundant syscalls.
		cleanR := &ExpressionResolver{
			fs:          r.fs,
			Home:        r.Home,
			Pwd:         r.Pwd,
			HostContext: r.HostContext,
			shared:      r.shared,
		}
		anchorPath, err := cleanR.ResolveString(anchor)
		if err != nil {
			return fmt.Errorf("failed to resolve anchor %q: %w", anchor, err)
		}
		if unresolved := scanAnchors(anchorPath); len(unresolved) > 0 {
			return fmt.Errorf("unresolved expression in anchor %q: %q", anchor, anchorPath)
		}
		if err := processBoundary(anchor, anchorPath); err != nil {
			return err
		}
	}

	return nil
}

// ValidateEnvKey ensures the environment variable key follows a safe and standard format.
func ValidateEnvKey(s string) error {
	if s == "" {
		return fmt.Errorf("environment variable key cannot be empty")
	}
	// Manual check: ^[a-zA-Z_][a-zA-Z0-9_]*$
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
				return fmt.Errorf("invalid environment variable key: %q", s)
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				return fmt.Errorf("invalid environment variable key: %q", s)
			}
		}
	}
	return nil
}

// ValidateImageName ensures the image name follows a safe and standard format.
func ValidateImageName(s string) error {
	if s == "" {
		return nil
	}
	// Manual check: ^[a-zA-Z0-9][a-zA-Z0-9._\-/:@]*$
	// Rejects multiple @ symbols
	atCount := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return fmt.Errorf("invalid image name: %q", s)
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
				c == '.' || c == '_' || c == '-' || c == '/' || c == ':' || c == '@') {
				return fmt.Errorf("invalid image name: %q", s)
			}
			if c == '@' {
				atCount++
			}
		}
	}
	if atCount > 1 {
		return fmt.Errorf("invalid image name: %q (multiple @ symbols)", s)
	}
	return nil
}

// ValidateHostname ensures the hostname follows standard DNS label rules.
func ValidateHostname(s string) error {
	if s == "" {
		return nil
	}
	if len(s) > 253 {
		return fmt.Errorf("hostname too long: %d characters (max 253)", len(s))
	}
	if !hostnameRegex.MatchString(s) {
		return fmt.Errorf("invalid hostname: %q", s)
	}
	return nil
}

// ValidateNetworkName ensures the network name follows Docker-compatible rules.
func ValidateNetworkName(s string) error {
	if s == "" {
		return nil
	}
	// Manual check: ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return fmt.Errorf("invalid network name: %q", s)
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
				c == '_' || c == '.' || c == '-') {
				return fmt.Errorf("invalid network name: %q", s)
			}
		}
	}
	return nil
}

// ValidateUserName ensures the user name (or user:group) is valid.
func ValidateUserName(s string) error {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ":")
	if len(parts) > 2 {
		return fmt.Errorf("invalid user format: %q", s)
	}
	for _, p := range parts {
		if !userPartRegex.MatchString(p) {
			return fmt.Errorf("invalid user or group identifier: %q", p)
		}
	}
	return nil
}

// ValidatePort ensures the port mapping is valid.
// Supports formats: [ip:][hostPort:]containerPort[/protocol]
func ValidatePort(s string) error {
	if s == "" {
		return nil
	}

	proto := ""
	remainder := s
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		remainder = parts[0]
		proto = parts[1]
		if proto != "tcp" && proto != "udp" {
			return fmt.Errorf("invalid protocol: %q", proto)
		}
	}

	parts := strings.Split(remainder, ":")
	switch len(parts) {
	case 1:
		// containerPort
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return fmt.Errorf("invalid container port: %q", parts[0])
		}
	case 2:
		// hostPort:containerPort OR ip:containerPort
		if _, err := strconv.Atoi(parts[1]); err != nil {
			return fmt.Errorf("invalid container port: %q", parts[1])
		}
		// If parts[0] is not a number, it must be an IP
		if _, err := strconv.Atoi(parts[0]); err != nil {
			if net.ParseIP(parts[0]) == nil {
				return fmt.Errorf("invalid host port or IP: %q", parts[0])
			}
		}
	case 3:
		// ip:hostPort:containerPort
		if _, err := strconv.Atoi(parts[2]); err != nil {
			return fmt.Errorf("invalid container port: %q", parts[2])
		}
		if parts[1] != "" {
			if _, err := strconv.Atoi(parts[1]); err != nil {
				return fmt.Errorf("invalid host port: %q", parts[1])
			}
		}
		if net.ParseIP(parts[0]) == nil {
			return fmt.Errorf("invalid IP: %q", parts[0])
		}
	default:
		return fmt.Errorf("invalid port format: %q", s)
	}

	return nil
}

// ValidateDNS ensures the DNS setting is a valid IP address.
func ValidateDNS(s string) error {
	if s == "" {
		return nil
	}
	if net.ParseIP(s) == nil {
		return fmt.Errorf("invalid DNS IP: %q", s)
	}
	return nil
}

// ValidateAddHost ensures the custom host-to-IP mapping (host:ip) is valid.
func ValidateAddHost(s string) error {
	if s == "" {
		return nil
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid add-host format: %q (expected host:ip)", s)
	}
	if err := ValidateHostname(parts[0]); err != nil {
		return fmt.Errorf("invalid host in add-host: %w", err)
	}
	if parts[1] != "host-gateway" && net.ParseIP(parts[1]) == nil {
		return fmt.Errorf("invalid IP in add-host: %q", parts[1])
	}
	return nil
}

// ValidateCapability ensures the Linux capability follows a safe format.
func ValidateCapability(s string) error {
	if s == "" {
		return nil
	}
	if !capRegex.MatchString(s) {
		return fmt.Errorf("invalid Linux capability: %q", s)
	}
	return nil
}

var workdirRegex = regexp.MustCompile(`^/[a-zA-Z0-9._\-/]*$`)

// ValidateWorkdir ensures the working directory is a valid absolute path.
func ValidateWorkdir(s string) error {
	if s == "" {
		return nil
	}
	if !path.IsAbs(s) {
		return fmt.Errorf("working directory must be an absolute path: %q", s)
	}
	if !workdirRegex.MatchString(s) {
		return fmt.Errorf("invalid characters in working directory: %q", s)
	}
	return nil
}

// ValidateExposePort ensures the exposed port (port[-port]/proto) is valid.
func ValidateExposePort(s string) error {
	if s == "" {
		return nil
	}
	// Format: port[-port][/protocol]
	proto := ""
	remainder := s
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		remainder = parts[0]
		proto = parts[1]
		if proto != "tcp" && proto != "udp" {
			return fmt.Errorf("invalid protocol: %q", proto)
		}
	}

	if parts := strings.SplitN(remainder, "-", 2); len(parts) == 2 {
		for _, p := range parts {
			if _, err := strconv.Atoi(p); err != nil {
				return fmt.Errorf("invalid port range: %q", remainder)
			}
		}
	} else if _, err := strconv.Atoi(remainder); err != nil {
		return fmt.Errorf("invalid port: %q", remainder)
	}

	return nil
}

// ValidateToolName ensures the tool name is a safe identifier.
// It rejects empty strings, absolute paths, parent directory references, directory separators,
// control characters, whitespace, and colons.
func ValidateToolName(name string) error {
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("absolute path not allowed for tool name: %q", name)
	}
	// Use a strict whitelist for tool names: alphanumerics, dots, underscores, and hyphens.
	for i, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '_' && r != '-' {
			return fmt.Errorf("invalid character in tool name %q: %q (position %d)", name, r, i)
		}
	}
	// Additional check for parent directory reference.
	// Since '/' and '\' are already rejected by the whitelist above,
	// we only need to check if the entire name is "..".
	if name == ".." || name == "." {
		return fmt.Errorf("current or parent directory reference not allowed for tool name: %q", name)
	}

	return nil
}

func isNamedVolume(s string) bool {
	if s == "" {
		return false
	}
	return !strings.ContainsAny(s, "/\\") && !strings.HasPrefix(s, ".") && !strings.HasPrefix(s, "~")
}
