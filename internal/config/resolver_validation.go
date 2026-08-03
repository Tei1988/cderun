package config

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"cderun/internal/logging"
)

// highlyPrivilegedCaps defines highly privileged Linux capabilities in both standard
// and "CAP_"-prefixed forms. Granting these capabilities poses security risks (e.g., container breakout).
var highlyPrivilegedCaps = map[string]bool{
	"ALL":            true,
	"SYS_ADMIN":      true,
	"NET_ADMIN":      true,
	"SYS_RAWIO":      true,
	"SYS_PTRACE":     true,
	"SYS_MODULE":     true,
	"CAP_ALL":        true,
	"CAP_SYS_ADMIN":  true,
	"CAP_NET_ADMIN":  true,
	"CAP_SYS_RAWIO":  true,
	"CAP_SYS_PTRACE": true,
	"CAP_SYS_MODULE": true,
}

// isHighlyPrivilegedCapability returns true if the given capability name is highly privileged.
// It performs a case-insensitive, trimmed, read-only lookup to prevent mutation of the underlying map.
func isHighlyPrivilegedCapability(capName string) bool {
	upperCap := strings.ToUpper(strings.TrimSpace(capName))
	return highlyPrivilegedCaps[upperCap]
}

func (rv *resolver) validateSecurity() error {
	if err := rv.validateCriticalFields(); err != nil {
		return err
	}
	if err := rv.validateSlices(); err != nil {
		return err
	}
	if err := rv.validateEnvSecurity(); err != nil {
		return err
	}
	if err := rv.validateMountSecurity(); err != nil {
		return err
	}
	if err := rv.validateDeviceSecurity(); err != nil {
		return err
	}
	var found []string
	for _, capName := range rv.res.CapAdd {
		if isHighlyPrivilegedCapability(capName) {
			found = append(found, capName)
		}
	}

	if logging.Enabled(logging.WarnLevel) {
		if rv.res.Privileged {
			logging.Warn("Container is running in privileged mode. This reduces container isolation and may pose security risks.")
			if len(found) > 0 {
				logging.Warn("Highly privileged capability %v detected in CapAdd while running in privileged mode. Please consider minimizing privileges.", found)
			}
		} else if len(found) > 0 {
			logging.Warn("Highly privileged capability %v detected in CapAdd. Please consider minimizing privileges.", found)
		}
		if rv.res.Network == "host" {
			logging.Warn("Container is running with host network mode enabled. This disables network isolation and may expose host network services to the container.")
		}
		for _, m := range rv.res.Mounts {
			if (m.Type == "bind" || m.Type == "") && m.Source != "" {
				cleanSource := path.Clean(m.Source)
				sensitivePaths := []string{"/boot", "/dev", "/etc", "/proc", "/sys"}
				isSensitive := cleanSource == "/"
				for _, p := range sensitivePaths {
					if cleanSource == p || strings.HasPrefix(cleanSource, p+"/") {
						isSensitive = true
						break
					}
				}
				if isSensitive {
					logging.Warn("Mounting highly sensitive host path %q into the container reduces host security isolation. Please ensure this is intended.", m.Source)
				}
			}
		}
	}
	if rv.res.MountSocket {
		if logging.Enabled(logging.WarnLevel) {
			logging.Warn("Container socket mounting is enabled. Granting access to the container runtime socket is highly privileged and allows full control over the container engine.")

			if ContainsNumericGID(rv.res.GroupAdd) {
				logging.Warn("Granting container socket permissions through a numeric VM socket GID allows socket access but is highly privileged. Limit such deployments to trusted environments.")
			}
		}
	}
	return nil
}

func (rv *resolver) validateCriticalFields() error {
	// image
	if err := validateField(rv.res.Image, "image", ValidateImageName); err != nil {
		return err
	}

	// pid
	pidValidator := func(v string) error {
		if v != "" && v != "host" {
			return fmt.Errorf("unsupported pid namespace: %q", v)
		}
		return nil
	}
	if err := validateField(rv.res.Pid, "pid", pidValidator); err != nil {
		return err
	}

	// user
	if err := validateField(rv.res.User, "user", ValidateUserName); err != nil {
		return err
	}

	// network
	if err := validateField(rv.res.Network, "network", ValidateNetworkName); err != nil {
		return err
	}

	// hostname
	if err := validateField(rv.res.Hostname, "hostname", ValidateHostname); err != nil {
		return err
	}

	// workdir
	if err := validateField(rv.res.Workdir, "workdir", ValidateWorkdir); err != nil {
		return err
	}

	// runtime
	runtimeValidator := func(v string) error {
		if v != "docker" && v != "podman" && v != "containerd" {
			return fmt.Errorf("unsupported runtime: %q", v)
		}
		return nil
	}
	if err := validateField(rv.res.Runtime, "runtime", runtimeValidator); err != nil {
		return err
	}

	// pull
	pullValidator := func(v string) error {
		if v != "" && v != "always" && v != "missing" && v != "never" {
			return fmt.Errorf("invalid pull policy %q: allowed values are \"always\", \"missing\", or \"never\"", v)
		}
		return nil
	}
	if err := validateField(rv.res.Pull, "pull", pullValidator); err != nil {
		return err
	}

	// socket-path
	if err := validateField(rv.res.SocketPath, "socket-path", nil); err != nil {
		return err
	}

	// mount-socket-path
	if err := validateField(rv.res.MountSocketPath, "mount-socket-path", nil); err != nil {
		return err
	}

	// mount-cderun-path
	if err := validateField(rv.res.MountCderunPath, "mount-cderun-path", nil); err != nil {
		return err
	}

	// dry-run-format
	dryRunFormatValidator := func(v string) error {
		if v != "" && v != "yaml" && v != "json" && v != "simple" {
			return fmt.Errorf("unsupported dry-run format: %q", v)
		}
		return nil
	}
	if err := validateField(rv.res.DryRunFormat, "dry-run-format", dryRunFormatValidator); err != nil {
		return err
	}

	// diagnosis-format
	diagnosisFormatValidator := func(v string) error {
		if v != "" && v != "yaml" && v != "json" && v != "simple" {
			return fmt.Errorf("unsupported diagnosis format: %q", v)
		}
		return nil
	}
	if err := validateField(rv.res.DiagnosisFormat, "diagnosis-format", diagnosisFormatValidator); err != nil {
		return err
	}

	// log-level
	logLevelValidator := func(v string) error {
		if v != "" {
			l := strings.ToLower(v)
			if l != "error" && l != "warn" && l != "warning" && l != "info" && l != "debug" && l != "trace" {
				return fmt.Errorf("unsupported log level: %q", v)
			}
		}
		return nil
	}
	if err := validateField(rv.res.LogLevel, "log-level", logLevelValidator); err != nil {
		return err
	}

	// log-format
	logFormatValidator := func(v string) error {
		if v != "" && v != "text" && v != "json" {
			return fmt.Errorf("unsupported log format: %q", v)
		}
		return nil
	}
	if err := validateField(rv.res.LogFormat, "log-format", logFormatValidator); err != nil {
		return err
	}

	if rv.res.Memory < 0 {
		return &InvalidConfigError{
			Field: "memory",
			Value: fmt.Sprintf("%d", rv.res.Memory),
			Err:   errors.New("memory limit cannot be negative"),
		}
	}
	if rv.res.CPUs < 0 {
		return &InvalidConfigError{
			Field: "cpus",
			Value: fmt.Sprintf("%g", rv.res.CPUs),
			Err:   errors.New("CPU limit cannot be negative"),
		}
	}

	return nil
}

func (rv *resolver) validateSlices() error {
	if err := validateSliceElements(rv.res.Entrypoint, "entrypoint", nil); err != nil {
		return err
	}
	if err := validateSliceElements(rv.res.Ports, "ports", ValidatePort); err != nil {
		return err
	}
	if err := validateSliceElements(rv.res.Expose, "expose", ValidateExposePort); err != nil {
		return err
	}
	if err := validateSliceElements(rv.res.DNS, "dns", ValidateDNS); err != nil {
		return err
	}
	if err := validateSliceElements(rv.res.AddHosts, "add-hosts", ValidateAddHost); err != nil {
		return err
	}
	if err := validateSliceElements(rv.res.CapAdd, "cap-add", ValidateCapability); err != nil {
		return err
	}
	if err := validateSliceElements(rv.res.CapDrop, "cap-drop", ValidateCapability); err != nil {
		return err
	}
	if err := validateSliceElements(rv.res.GroupAdd, "group-add", ValidateGroupAdd); err != nil {
		return err
	}

	sensitiveEnvValidator := func(e string) error {
		_, err := path.Match(e, "TEST")
		if err != nil {
			return fmt.Errorf("invalid glob pattern: %w", err)
		}
		return nil
	}
	if err := validateSliceElements(rv.res.SensitiveEnv, "sensitive-env", sensitiveEnvValidator); err != nil {
		return err
	}

	return nil
}

func (rv *resolver) validateEnvSecurity() error {
	for i, e := range rv.res.Env {
		key, val, _ := strings.Cut(e, "=")
		if err := validatePathChars(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
		if err := ValidateEnvKey(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
		if strings.ContainsRune(val, 0) {
			return fmt.Errorf("security validation failed for env[%d] (value): null byte injection detected", i)
		}
	}
	return nil
}

func (rv *resolver) validateMountSecurity() error {
	for i, m := range rv.res.Mounts {
		if err := validatePathChars(m.Source); err != nil {
			return fmt.Errorf("security validation failed for mounts[%d] (source): %w", i, err)
		}
		if err := validatePathChars(m.Target); err != nil {
			return fmt.Errorf("security validation failed for mounts[%d] (target): %w", i, err)
		}
		if err := validateContainerPath(m.Target, "mounts", i, "target", "target"); err != nil {
			return err
		}
	}
	return nil
}

func (rv *resolver) validateDeviceSecurity() error {
	for i, d := range rv.res.Devices {
		if err := validatePathChars(d.PathOnHost); err != nil {
			return fmt.Errorf("security validation failed for devices[%d] (path-on-host): %w", i, err)
		}
		if err := validatePathChars(d.PathInContainer); err != nil {
			return fmt.Errorf("security validation failed for devices[%d] (path-in-container): %w", i, err)
		}
		if err := validateContainerPath(d.PathInContainer, "devices", i, "path-in-container", "destination"); err != nil {
			return err
		}
		if d.CgroupPermissions != "" {
			if err := validatePathChars(d.CgroupPermissions); err != nil {
				return fmt.Errorf("security validation failed for devices[%d] (permissions): %w", i, err)
			}
			if !permsRegex.MatchString(d.CgroupPermissions) {
				return fmt.Errorf("security validation failed for devices[%d] (permissions): invalid cgroup permissions %q", i, d.CgroupPermissions)
			}
		}
	}
	return nil
}

func validateField(value string, name string, validator func(string) error) error {
	if err := validatePathChars(value); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", name, err)
	}
	if validator != nil {
		if err := validator(value); err != nil {
			return fmt.Errorf("security validation failed for %q: %w", name, err)
		}
	}
	return nil
}

func validateSliceElements(slice []string, name string, validator func(string) error) error {
	for i, e := range slice {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", name, i, err)
		}
		if validator != nil {
			if err := validator(e); err != nil {
				return fmt.Errorf("security validation failed for %s[%d]: %w", name, i, err)
			}
		}
	}
	return nil
}

func validateContainerPath(val string, listName string, index int, fieldName string, pathName string) error {
	if val == "" {
		return fmt.Errorf("security validation failed for %s[%d] (%s): %s path cannot be empty", listName, index, fieldName, pathName)
	}
	if !path.IsAbs(val) {
		return fmt.Errorf("security validation failed for %s[%d] (%s): %s path must be an absolute path: %q", listName, index, fieldName, pathName, val)
	}
	if HasParentTraversal(val) {
		return fmt.Errorf("security validation failed for %s[%d] (%s): %s path cannot contain parent directory references: %q", listName, index, fieldName, pathName, val)
	}
	return nil
}
