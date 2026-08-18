package config

import (
	"fmt"
	"net"
	"path"
	"path/filepath"
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
}
func ValidateMountType(t string) error {
	if t == "" || t == "bind" || t == "volume" || t == "tmpfs" {
		return nil
	}
	return fmt.Errorf("unsupported mount type: %q (allowed: bind, volume, tmpfs)", t)
}

func (mc MountConfig) Resolve(r *ExpressionResolver) (container.Mount, error) {
	if err := ValidateMountType(mc.Type); err != nil {
		return container.Mount{}, err
	}

	mountType := mc.Type
	if mountType == "" {
		mountType = "bind"
	}

	// Prevent parent directory references in mount target raw inputs to avoid obfuscation/traversal
	if HasParentTraversal(mc.Target.Raw) {
		return container.Mount{}, fmt.Errorf("mount target cannot contain parent directory references: %q", mc.Target.Raw)
	}

	source := ""
	if mountType == "bind" {
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
		Type:     mountType,
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
	if node.Kind == yaml.ScalarNode {
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		parsed, ok := ParseDeviceConfig(s)
		if !ok {
			return fmt.Errorf("invalid device config: %q at line %d", s, node.Line)
		}

		*dc = parsed
		return nil
	}

	if node.Kind == yaml.MappingNode {
		var a struct {
			Source      ConfigPath `yaml:"source"`
			Destination ConfigPath `yaml:"destination"`
			Permissions string     `yaml:"permissions"`
		}
		if err := node.Decode(&a); err != nil {
			return err
		}
		if a.Source.IsEmpty() {
			return fmt.Errorf("device source is required at line %d", node.Line)
		}
		if a.Destination.IsEmpty() {
			return fmt.Errorf("device destination is required at line %d", node.Line)
		}
		if a.Permissions != "" && !isValidPerms(a.Permissions) {
			return fmt.Errorf("invalid device permissions at line %d: %q", node.Line, a.Permissions)
		}

		dc.Source = a.Source
		dc.Destination = a.Destination
		dc.Permissions = a.Permissions
		if dc.Permissions == "" {
			dc.Permissions = "rwm"
		}
		return nil
	}

	return fmt.Errorf("invalid device config at line %d (tag %s): %v", node.Line, node.Tag, node.Value)
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
}
func (dc DeviceConfig) Resolve(r *ExpressionResolver) (container.DeviceMapping, error) {
	// Prevent parent directory references in device destination raw inputs to avoid obfuscation/traversal
	if HasParentTraversal(dc.Destination.Raw) {
		return container.DeviceMapping{}, fmt.Errorf("device destination cannot contain parent directory references: %q", dc.Destination.Raw)
	}
	// Prevent parent directory references in device source raw inputs to avoid obfuscation/traversal
	if HasParentTraversal(dc.Source.Raw) {
		return container.DeviceMapping{}, fmt.Errorf("device source cannot contain parent directory references: %q", dc.Source.Raw)
	}

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
			return MountConfig{}, fmt.Errorf("unknown mount option: %q (supported: type, source, src, target, dst, destination, readonly, optional)", key)
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
		if isValidPerms(perms) {
			permissions = perms
			containerPath = remainder[:lastColon]
			// If there's another colon, it's malformed (we only support host:container:perms).
			if strings.Contains(containerPath, ":") {
				return DeviceConfig{}, false
			}
		} else {
			// If a second colon exists, the remainder must be a valid permissions suffix.
			return DeviceConfig{}, false
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

func hasScheme(s string) bool {
	idx := strings.Index(s, "://")
	if idx <= 0 {
		return false
	}
	for i := range idx {
		c := s[i]
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

func isValidPerms(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != 'r' && c != 'w' && c != 'm' {
			return false
		}
	}
	return true
}

func isWinDrive(s string) bool {
	if len(s) < 3 {
		return false
	}
	c := s[0]
	if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) && s[1] == ':' && (s[2] == '/' || s[2] == '\\') {
		return true
	}
	return false
}

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
		if strings.HasPrefix(p, "~") {
			home, err := fs.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home directory: %w", err)
			}
			res = expandHome(p, home)
		} else {
			res = p
		}
	}

	// If expanded value contains a scheme, return it as is
	if hasScheme(res) {
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

	if isWinDrive(s) {
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
	var buf [8]anchorRange
	ranges := scanAnchors(s, buf[:0])
	if len(ranges) == 0 {
		return nil
	}
	res := make([]string, len(ranges))
	for i, r := range ranges {
		res[i] = s[r.start:r.end]
	}
	return res
}

// ContainsNumericGID returns true if any element in the given slice is a completely numeric string.
func ContainsNumericGID(groups []string) bool {
	for _, gid := range groups {
		isNum := len(gid) > 0
		for i := 0; i < len(gid); i++ {
			if gid[i] < '0' || gid[i] > '9' {
				isNum = false
				break
			}
		}
		if isNum {
			return true
		}
	}
	return false
}

// HasParentTraversal checks if a path contains parent directory traversal ("..") segments.
func HasParentTraversal(s string) bool {
	if !strings.Contains(s, "..") {
		return false
	}
	idx := 0
	for {
		i := strings.Index(s[idx:], "..")
		if i == -1 {
			return false
		}
		pos := idx + i
		startOk := pos == 0 || s[pos-1] == '/' || s[pos-1] == '\\'
		endOk := pos+2 == len(s) || s[pos+2] == '/' || s[pos+2] == '\\'
		if startOk && endOk {
			return true
		}
		idx = pos + 1
	}
}

// validatePathChars ensures the string does not contain ASCII control characters.
func validatePathChars(s string) error {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= 31 || c == 127 {
			return fmt.Errorf("invalid character in path or configuration: %q (position %d)", rune(c), i)
		}
	}
	return nil
}

// ValidateCpuset restricts cpuset configurations to standard digits, commas, and hyphens.
func ValidateCpuset(s string) error {
	if s == "" {
		return nil
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || c == ',' || c == '-') {
			return fmt.Errorf("invalid characters in cpuset: %q", s)
		}
	}
	return nil
}

// ValidateGPUs restricts `--gpus` configuration to alphanumerics, commas, equals, and hyphens.
func ValidateGPUs(s string) error {
	if s == "" {
		return nil
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ',' || c == '=' || c == '-') {
			return fmt.Errorf("invalid characters in gpus option: %q", s)
		}
	}
	return nil
}

// ValidateGroupAdd ensures the supplementary group name or GID is safe.
func ValidateGroupAdd(s string) error {
	if s == "" {
		return nil
	}
	if !isValidGroupPart(s) {
		return fmt.Errorf("invalid supplementary group name or GID: %q", s)
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
		if err := validatePathChars(anchorPath); err != nil {
			return fmt.Errorf("invalid character in resolved anchor path for %q: %w", anchorRaw, err)
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

	if len(exprAnchors) > 0 {
		if r == nil {
			return fmt.Errorf("expression resolver required for anchor validation")
		}
		r.ensureShared()
	}

	for _, anchor := range exprAnchors {
		// Use a fresh resolver instance for each anchor to ensure that a sticky error
		// from one anchor resolution doesn't skip subsequent anchor resolutions.
		// Reusing Home and Pwd from current resolver to avoid redundant syscalls.
		cleanR := &ExpressionResolver{
			fs:          r.fs,
			Home:        r.Home,
			Pwd:         r.Pwd,
			HostContext: r.HostContext,
		}
		cleanR.shared.Store(r.shared.Load())
		anchorPath, err := cleanR.ResolveString(anchor)
		if err != nil {
			return fmt.Errorf("failed to resolve anchor %q: %w", anchor, err)
		}
		var checkBuf [4]anchorRange
		if unresolved := scanAnchors(anchorPath, checkBuf[:0]); len(unresolved) > 0 {
			return fmt.Errorf("unresolved expression in anchor %q: %q", anchor, anchorPath)
		}
		if err := processBoundary(anchor, anchorPath); err != nil {
			return err
		}
	}

	return nil
}

func isEnvKeyStartChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isEnvKeyChar(c byte) bool {
	return isEnvKeyStartChar(c) || (c >= '0' && c <= '9')
}

// ValidateEnvKey ensures the environment variable key follows a safe and standard format.
func ValidateEnvKey(s string) error {
	if s == "" {
		return fmt.Errorf("environment variable key cannot be empty")
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if !isEnvKeyStartChar(c) {
				return fmt.Errorf("invalid environment variable key: %q", s)
			}
		} else {
			if !isEnvKeyChar(c) {
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
	if HasParentTraversal(s) {
		return fmt.Errorf("invalid image name: %q (contains parent directory references)", s)
	}
	// Manual check: ^[a-zA-Z0-9][a-zA-Z0-9._\-/:@]*$
	// Rejects multiple @ symbols
	atCount := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
				return fmt.Errorf("invalid image name: %q", s)
			}
		} else {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') &&
				c != '.' && c != '_' && c != '-' && c != '/' && c != ':' && c != '@' {
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

func isValidHostname(s string) bool {
	if s == "" {
		return false
	}
	if len(s) > 253 {
		return false
	}

	start := 0
	for {
		end := strings.IndexByte(s[start:], '.')
		var label string
		if end == -1 {
			label = s[start:]
		} else {
			label = s[start : start+end]
		}

		if len(label) == 0 || len(label) > 63 {
			return false
		}

		// First and last char of label cannot be '-'
		first := label[0]
		if (first < 'a' || first > 'z') && (first < 'A' || first > 'Z') && (first < '0' || first > '9') {
			return false
		}
		last := label[len(label)-1]
		if (last < 'a' || last > 'z') && (last < 'A' || last > 'Z') && (last < '0' || last > '9') {
			return false
		}

		// Other characters must be alphanumerics or '-'
		for i := 1; i < len(label)-1; i++ {
			c := label[i]
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}

		if end == -1 {
			break
		}
		start += end + 1
	}
	return true
}

// ValidateHostname ensures the hostname follows standard DNS label rules.
func ValidateHostname(s string) error {
	if s == "" {
		return nil
	}
	if len(s) > 253 {
		return fmt.Errorf("hostname too long: %d characters (max 253)", len(s))
	}
	if !isValidHostname(s) {
		return fmt.Errorf("invalid hostname: %q", s)
	}
	return nil
}

// ValidateNetworkName ensures the network name follows Docker-compatible rules.
func ValidateNetworkName(s string) error {
	if s == "" {
		return nil
	}
	if HasParentTraversal(s) {
		return fmt.Errorf("invalid network name: %q (contains parent directory references)", s)
	}
	if target, ok := strings.CutPrefix(s, "container:"); ok {
		if target == "" || target == ".." || target == "." {
			return fmt.Errorf("invalid network name: empty or invalid container reference in %q", s)
		}
		for i := 0; i < len(target); i++ {
			c := target[i]
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '.' && c != '-' {
				return fmt.Errorf("invalid character in container network reference: %q", target)
			}
		}
		return nil
	}
	if target, ok := strings.CutPrefix(s, "ns:"); ok {
		if target == "" {
			return fmt.Errorf("invalid network name: empty namespace path in %q", s)
		}
		if !path.IsAbs(target) {
			return fmt.Errorf("network namespace path must be an absolute path: %q", target)
		}
		if err := validatePathChars(target); err != nil {
			return err
		}
		return nil
	}
	// Manual check: ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
				return fmt.Errorf("invalid network name: %q", s)
			}
		} else {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') &&
				c != '_' && c != '.' && c != '-' {
				return fmt.Errorf("invalid network name: %q", s)
			}
		}
	}
	return nil
}

func isValidUserPart(s string) bool {
	if s == "" {
		return false
	}
	// Check if completely numeric
	isNumeric := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric {
		return true
	}

	// Check if it matches: ^[a-z_][a-z0-9_-]*[$]?$
	first := s[0]
	if (first < 'a' || first > 'z') && first != '_' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if i == len(s)-1 && c == '$' {
			break
		}
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func isValidGroupPart(s string) bool {
	if s == "" {
		return false
	}
	// Check if completely numeric
	isNumeric := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric {
		return true
	}

	// Check if it matches: ^[a-zA-Z_][a-zA-Z0-9_-]*[$]?$
	first := s[0]
	if (first < 'a' || first > 'z') && (first < 'A' || first > 'Z') && first != '_' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if i == len(s)-1 && c == '$' {
			break
		}
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// ValidateUserName ensures the user name (or user:group) is valid.
func ValidateUserName(s string) error {
	if s == "" {
		return nil
	}
	user, group, hasGroup := strings.Cut(s, ":")
	if hasGroup && strings.Contains(group, ":") {
		return fmt.Errorf("invalid user format: %q", s)
	}
	if !isValidUserPart(user) {
		return fmt.Errorf("invalid user or group identifier: %q", user)
	}
	if hasGroup && !isValidGroupPart(group) {
		return fmt.Errorf("invalid user or group identifier: %q", group)
	}
	return nil
}

func validatePortNumber(s string, allowZero bool) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid port: %q", s)
	}
	if p < 0 || p > 65535 {
		return 0, fmt.Errorf("port out of range: %d", p)
	}
	if p == 0 && !allowZero {
		return 0, fmt.Errorf("port cannot be zero: %d", p)
	}
	return p, nil
}

// ValidatePort ensures the port mapping is valid.
// Supports formats: [ip:][hostPort:]containerPort[/protocol]
func ValidatePort(s string) error {
	if s == "" {
		return nil
	}
	if err := validatePathChars(s); err != nil {
		return err
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
		if _, err := validatePortNumber(parts[0], false); err != nil {
			return fmt.Errorf("invalid container port: %w", err)
		}
	case 2:
		// hostPort:containerPort OR ip:containerPort
		if _, err := validatePortNumber(parts[1], false); err != nil {
			return fmt.Errorf("invalid container port: %w", err)
		}
		// Try parsing as port first
		if _, err := validatePortNumber(parts[0], true); err != nil {
			// If not a valid port, must be an IP
			if net.ParseIP(parts[0]) == nil {
				return fmt.Errorf("invalid host port or IP: %q", parts[0])
			}
		}
	case 3:
		// ip:hostPort:containerPort
		if _, err := validatePortNumber(parts[2], false); err != nil {
			return fmt.Errorf("invalid container port: %w", err)
		}
		if parts[1] != "" {
			if _, err := validatePortNumber(parts[1], true); err != nil {
				return fmt.Errorf("invalid host port: %w", err)
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
	if err := validatePathChars(s); err != nil {
		return err
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
	if err := validatePathChars(s); err != nil {
		return err
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

func isValidCapability(s string) bool {
	if s == "" {
		return false
	}
	first := s[0]
	if first < 'A' || first > 'Z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// ValidateCapability ensures the Linux capability follows a safe format.
func ValidateCapability(s string) error {
	if s == "" {
		return nil
	}
	if err := validatePathChars(s); err != nil {
		return err
	}
	if !isValidCapability(s) {
		return fmt.Errorf("invalid Linux capability: %q", s)
	}
	return nil
}

func isValidWorkdirChars(s string) bool {
	if len(s) == 0 || s[0] != '/' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '.' && c != '_' && c != '-' && c != '/' {
			return false
		}
	}
	return true
}

// ValidateWorkdir ensures the working directory is a valid absolute path.
func ValidateWorkdir(s string) error {
	if s == "" {
		return nil
	}
	if !path.IsAbs(s) {
		return fmt.Errorf("working directory must be an absolute path: %q", s)
	}
	if !isValidWorkdirChars(s) {
		return fmt.Errorf("invalid characters in working directory: %q", s)
	}
	if HasParentTraversal(s) {
		return fmt.Errorf("working directory cannot contain parent directory references: %q", s)
	}
	return nil
}

// ValidateExposePort ensures the exposed port (port[-port]/proto) is valid.
func ValidateExposePort(s string) error {
	if s == "" {
		return nil
	}
	if err := validatePathChars(s); err != nil {
		return err
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
		start, err := validatePortNumber(parts[0], false)
		if err != nil {
			return fmt.Errorf("invalid start port in range: %w", err)
		}
		end, err := validatePortNumber(parts[1], false)
		if err != nil {
			return fmt.Errorf("invalid end port in range: %w", err)
		}
		if start > end {
			return fmt.Errorf("invalid port range: %d > %d", start, end)
		}
	} else if _, err := validatePortNumber(remainder, false); err != nil {
		return fmt.Errorf("invalid port: %w", err)
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

// ValidateDNSOption ensures the DNS option format is safe and valid.
// Allowed characters are restricted to [a-zA-Z0-9.:_-] to prevent control characters or injection.
func ValidateDNSOption(s string) error {
	if s == "" {
		return nil
	}
	if HasParentTraversal(s) {
		return fmt.Errorf("invalid DNS option: %q (contains parent directory references)", s)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == ':' || c == '_' || c == '-') {
			return fmt.Errorf("invalid characters in DNS option: %q", s)
		}
	}
	return nil
}

// ValidateSecurityOpt restricts characters in security options to a safe alphanumeric and separator set.
func ValidateSecurityOpt(s string) error {
	if s == "" {
		return nil
	}
	if HasParentTraversal(s) {
		return fmt.Errorf("invalid security option: %q (contains parent directory references)", s)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '=' || c == ':' || c == '/' || c == '-' || c == '_' || c == '.' || c == '\\' || c == ',') {
			return fmt.Errorf("invalid characters in security option: %q", s)
		}
	}
	return nil
}

// ValidateSysctlKey ensures the sysctl key contains only safe, standard characters.
func ValidateSysctlKey(s string) error {
	if s == "" {
		return fmt.Errorf("sysctl key cannot be empty")
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			return fmt.Errorf("invalid characters in sysctl key: %q", s)
		}
	}
	return nil
}

// ValidateSysctlValue restricts the sysctl value to standard safe characters, spaces, and separators.
func ValidateSysctlValue(s string) error {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == ' ' || c == '.' || c == '_' || c == '-' || c == ',') {
			return fmt.Errorf("invalid characters in sysctl value: %q", s)
		}
	}
	return nil
}
