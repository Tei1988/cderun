package controlsocket

import (
	"fmt"
	"path/filepath"
	"strings"

	"cderun/internal/container"
)

// ValidateInheritedCeiling validates a child container configuration against a parent container configuration
// according to the inherited-ceiling security model.
// Returns an error if the child requests privileges, capabilities, devices, or access exceeding the parent.
func ValidateInheritedCeiling(child, parent *container.ContainerConfig) error {
	if child == nil {
		return fmt.Errorf("security violation: child container config is nil")
	}
	if parent == nil {
		return nil
	}

	// 1. Privileged Mode
	if child.Privileged && !parent.Privileged {
		return fmt.Errorf("security violation: child requested privileged mode when parent is non-privileged")
	}

	// If parent is privileged, parent holds all capabilities, device access, and namespace access.
	if parent.Privileged {
		return nil
	}

	// 2. Linux Capabilities (CapAdd)
	if err := validateCapAdd(child.CapAdd, parent.CapAdd); err != nil {
		return err
	}

	// 3. Namespace Isolation (Network, Pid, Ipc)
	if err := validateNamespaces(child, parent); err != nil {
		return err
	}

	// 4. Host Devices
	if err := validateDevices(child.Devices, parent.Devices); err != nil {
		return err
	}

	// 5. Read-Only Mount Boundaries
	if err := validateMounts(child.Mounts, parent); err != nil {
		return err
	}

	// 6. Security Options
	if err := validateSecurityOpt(child.SecurityOpt, parent.SecurityOpt); err != nil {
		return err
	}

	return nil
}

func normalizeCap(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	return strings.TrimPrefix(c, "CAP_")
}

func validateCapAdd(childCaps, parentCaps []string) error {
	if len(childCaps) == 0 {
		return nil
	}
	parentSet := make(map[string]bool, len(parentCaps))
	for _, cap := range parentCaps {
		parentSet[normalizeCap(cap)] = true
	}

	for _, cap := range childCaps {
		norm := normalizeCap(cap)
		if !parentSet[norm] {
			return fmt.Errorf("security violation: child requested capability %q not permitted by parent", cap)
		}
	}
	return nil
}

func validateNamespaces(child, parent *container.ContainerConfig) error {
	// Network
	parentNet := strings.ToLower(strings.TrimSpace(parent.Network))
	childNet := strings.ToLower(strings.TrimSpace(child.Network))

	if parentNet == "none" && childNet != "" && childNet != "none" {
		return fmt.Errorf("security violation: child requested network mode %q when parent network is %q", child.Network, parent.Network)
	}
	if childNet == "host" && parentNet != "host" {
		return fmt.Errorf("security violation: child requested host network mode when parent is not host network")
	}

	// PID
	childPid := strings.ToLower(strings.TrimSpace(child.Pid))
	parentPid := strings.ToLower(strings.TrimSpace(parent.Pid))
	if childPid == "host" && parentPid != "host" {
		return fmt.Errorf("security violation: child requested host PID mode when parent is not host PID")
	}

	// IPC
	childIpc := strings.ToLower(strings.TrimSpace(child.IPC))
	parentIpc := strings.ToLower(strings.TrimSpace(parent.IPC))
	if childIpc == "host" && parentIpc != "host" {
		return fmt.Errorf("security violation: child requested host IPC mode when parent is not host IPC")
	}

	return nil
}

func validateDevices(childDevs, parentDevs []container.DeviceMapping) error {
	if len(childDevs) == 0 {
		return nil
	}

	parentDevMap := make(map[string]container.DeviceMapping, len(parentDevs))
	for _, pDev := range parentDevs {
		cleanHost := filepath.Clean(pDev.PathOnHost)
		parentDevMap[cleanHost] = pDev
	}

	for _, cDev := range childDevs {
		cleanHost := filepath.Clean(cDev.PathOnHost)
		pDev, exists := parentDevMap[cleanHost]
		if !exists {
			return fmt.Errorf("security violation: child requested host device %q not permitted by parent", cDev.PathOnHost)
		}

		// Verify cgroup permissions subset (e.g., "r", "rw", "rwm")
		cPerms := cDev.CgroupPermissions
		if cPerms == "" {
			cPerms = "rwm"
		}
		pPerms := pDev.CgroupPermissions
		if pPerms == "" {
			pPerms = "rwm"
		}

		for _, ch := range cPerms {
			if !strings.ContainsRune(pPerms, ch) {
				return fmt.Errorf("security violation: child requested device %q permissions %q exceeding parent permissions %q", cDev.PathOnHost, cDev.CgroupPermissions, pDev.CgroupPermissions)
			}
		}
	}

	return nil
}

func isSubpathOrEqual(sub, parent string) bool {
	subClean := filepath.Clean(sub)
	parentClean := filepath.Clean(parent)

	if subClean == parentClean {
		return true
	}
	rel, err := filepath.Rel(parentClean, subClean)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

func validateMounts(childMounts []container.Mount, parent *container.ContainerConfig) error {
	if parent.ReadOnly {
		for _, cMount := range childMounts {
			if cMount.Type == "bind" || cMount.Type == "" {
				if !cMount.ReadOnly {
					return fmt.Errorf("security violation: child requested read-write mount %q when parent root is read-only", cMount.Source)
				}
			}
		}
	}

	for _, pMount := range parent.Mounts {
		if !pMount.ReadOnly {
			continue
		}

		for _, cMount := range childMounts {
			if cMount.ReadOnly {
				continue
			}
			if cMount.Type != "bind" && cMount.Type != "" {
				continue
			}

			// Check if child's mount source falls inside parent's read-only mount target or source
			if (pMount.Target != "" && isSubpathOrEqual(cMount.Source, pMount.Target)) ||
				(pMount.Source != "" && isSubpathOrEqual(cMount.Source, pMount.Source)) {
				return fmt.Errorf("security violation: child requested read-write mount for %q within parent read-only path", cMount.Source)
			}
		}
	}

	return nil
}

func isHardeningOpt(opt string) bool {
	optLower := strings.ToLower(strings.TrimSpace(opt))
	return optLower == "no-new-privileges" || strings.HasPrefix(optLower, "no-new-privileges:") || optLower == "no-new-privileges=true"
}

func validateSecurityOpt(childOpts, parentOpts []string) error {
	if len(childOpts) == 0 {
		return nil
	}

	parentSet := make(map[string]bool, len(parentOpts))
	for _, opt := range parentOpts {
		parentSet[strings.ToLower(strings.TrimSpace(opt))] = true
	}

	for _, opt := range childOpts {
		if isHardeningOpt(opt) {
			continue
		}
		normOpt := strings.ToLower(strings.TrimSpace(opt))
		if !parentSet[normOpt] {
			return fmt.Errorf("security violation: child requested security option %q not permitted by parent", opt)
		}
	}

	return nil
}
