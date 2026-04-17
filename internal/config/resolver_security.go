package config

import (
	"fmt"
	"strings"

	"cderun/internal/logging"
)

func (rv *resolver) validateSecurity() error {
	// Security: validate resolved configuration for injection characters and identifier formats.
	criticalFields := []struct {
		name      string
		value     string
		validator func(string) error
	}{
		{"image", rv.res.Image, ValidateImageName},
		{"user", rv.res.User, ValidateUserName},
		{"network", rv.res.Network, ValidateNetworkName},
		{"hostname", rv.res.Hostname, ValidateHostname},
		{"workdir", rv.res.Workdir, ValidateWorkdir},
		{"runtime", rv.res.Runtime, func(s string) error {
			if s != "docker" && s != "podman" && s != "containerd" {
				return fmt.Errorf("unsupported runtime: %q", s)
			}
			return nil
		}},
		{"socket-path", rv.res.SocketPath, nil},
		{"mount-socket-path", rv.res.MountSocketPath, nil},
		{"mount-cderun-path", rv.res.MountCderunPath, nil},
		{"dry-run-format", rv.res.DryRunFormat, func(s string) error {
			if s != "" && s != "yaml" && s != "json" && s != "simple" {
				return fmt.Errorf("unsupported dry-run format: %q", s)
			}
			return nil
		}},
		{"diagnosis-format", rv.res.DiagnosisFormat, func(s string) error {
			if s != "" && s != "yaml" && s != "json" && s != "simple" {
				return fmt.Errorf("unsupported diagnosis format: %q", s)
			}
			return nil
		}},
		{"log-level", rv.res.LogLevel, func(s string) error {
			if s != "" {
				l := strings.ToLower(s)
				if l != "error" && l != "warn" && l != "warning" && l != "info" && l != "debug" && l != "trace" {
					return fmt.Errorf("unsupported log level: %q", s)
				}
			}
			return nil
		}},
		{"log-format", rv.res.LogFormat, func(s string) error {
			if s != "" && s != "text" && s != "json" {
				return fmt.Errorf("unsupported log format: %q", s)
			}
			return nil
		}},
	}
	for _, f := range criticalFields {
		if err := validatePathChars(f.value); err != nil {
			return fmt.Errorf("security validation failed for %q: %w", f.name, err)
		}
		if f.validator != nil {
			if err := f.validator(f.value); err != nil {
				return fmt.Errorf("security validation failed for %q: %w", f.name, err)
			}
		}
	}

	criticalSlices := []struct {
		name      string
		slice     []string
		validator func(string) error
	}{
		{"entrypoint", rv.res.Entrypoint, nil},
		{"ports", rv.res.Ports, ValidatePort},
		{"expose", rv.res.Expose, ValidateExposePort},
		{"dns", rv.res.DNS, ValidateDNS},
		{"add-hosts", rv.res.AddHosts, ValidateAddHost},
		{"cap-add", rv.res.CapAdd, ValidateCapability},
		{"cap-drop", rv.res.CapDrop, ValidateCapability},
	}
	for _, s := range criticalSlices {
		for i, e := range s.slice {
			if err := validatePathChars(e); err != nil {
				return fmt.Errorf("security validation failed for %s[%d]: %w", s.name, i, err)
			}
			if s.validator != nil {
				if err := s.validator(e); err != nil {
					return fmt.Errorf("security validation failed for %s[%d]: %w", s.name, i, err)
				}
			}
		}
	}

	for i, e := range rv.res.Env {
		key, _, _ := strings.Cut(e, "=")
		if err := validatePathChars(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
		if err := ValidateEnvKey(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
	}

	for i, m := range rv.res.Mounts {
		if err := validatePathChars(m.Source); err != nil {
			return fmt.Errorf("security validation failed for mounts[%d] (source): %w", i, err)
		}
		if err := validatePathChars(m.Target); err != nil {
			return fmt.Errorf("security validation failed for mounts[%d] (target): %w", i, err)
		}
	}

	for i, d := range rv.res.Devices {
		if err := validatePathChars(d.PathOnHost); err != nil {
			return fmt.Errorf("security validation failed for devices[%d] (path-on-host): %w", i, err)
		}
		if err := validatePathChars(d.PathInContainer); err != nil {
			return fmt.Errorf("security validation failed for devices[%d] (path-in-container): %w", i, err)
		}
	}

	if rv.res.Privileged {
		if logging.Enabled(logging.WarnLevel) {
			logging.Warn("Container is running in privileged mode. This reduces container isolation and may pose security risks.")
		}
	}
	return nil
}
