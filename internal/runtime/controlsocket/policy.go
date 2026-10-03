package controlsocket

import (
	"fmt"
	"path/filepath"
	"strings"

	"cderun/internal/container"
)

// ValidateInheritedCeiling validates a child ContainerConfig against a parent ContainerConfig
// according to the inherited-ceiling security model. A nested invocation cannot request
// broader privileges than the parent container was granted.
func ValidateInheritedCeiling(parent, child *container.ContainerConfig) error {
	if parent == nil || child == nil {
		return nil
	}

	// 1. Privileged mode escalation
	if !parent.Privileged && child.Privileged {
		return fmt.Errorf("inherited ceiling validation failed: privileged mode requested by nested container but not granted to parent")
	}

	// 2. Capabilities escalation
	if !parent.Privileged && !hasCapAll(parent.CapAdd) && len(child.CapAdd) > 0 {
		for _, cap := range child.CapAdd {
			if !hasCapability(parent.CapAdd, cap) {
				return fmt.Errorf("inherited ceiling validation failed: capability %q requested by nested container but not granted to parent", cap)
			}
		}
	}

	// 3. Namespace escalation: Network
	if parent.Network != "host" && child.Network == "host" {
		return fmt.Errorf("inherited ceiling validation failed: host network requested by nested container but not granted to parent")
	}

	// 4. Namespace escalation: Pid
	if parent.Pid != "host" && child.Pid == "host" {
		return fmt.Errorf("inherited ceiling validation failed: host PID namespace requested by nested container but not granted to parent")
	}

	// 5. Namespace escalation: IPC
	if parent.IPC != "host" && child.IPC == "host" {
		return fmt.Errorf("inherited ceiling validation failed: host IPC namespace requested by nested container but not granted to parent")
	}

	// 6. Devices escalation
	if !parent.Privileged && len(parent.Devices) == 0 && len(child.Devices) > 0 {
		return fmt.Errorf("inherited ceiling validation failed: devices requested by nested container but not granted to parent")
	}
	if !parent.Privileged && len(parent.Devices) > 0 && len(child.Devices) > 0 {
		for _, cd := range child.Devices {
			if !hasDevice(parent.Devices, cd) {
				return fmt.Errorf("inherited ceiling validation failed: device %q requested by nested container but not granted to parent", cd.PathOnHost)
			}
		}
	}

	// 7. Mount ReadOnly escalation
	for _, cm := range child.Mounts {
		if !cm.ReadOnly && isParentMountReadOnly(parent.Mounts, cm.Source, cm.Target) {
			return fmt.Errorf("inherited ceiling validation failed: writable mount requested by nested container for read-only parent source/target path %q", cm.Source)
		}
	}

	// 8. SecurityOpt escalation
	if !parent.Privileged && len(child.SecurityOpt) > 0 {
		for _, opt := range child.SecurityOpt {
			if !isSecurityOptAllowed(parent.SecurityOpt, opt) {
				return fmt.Errorf("inherited ceiling validation failed: security option %q requested by nested container but not granted to parent", opt)
			}
		}
	}

	return nil
}

func hasCapAll(caps []string) bool {
	for _, c := range caps {
		if strings.EqualFold(c, "ALL") {
			return true
		}
	}
	return false
}

func hasCapability(caps []string, cap string) bool {
	normalizedCap := normalizeCap(cap)
	for _, c := range caps {
		if normalizeCap(c) == normalizedCap {
			return true
		}
	}
	return false
}

func normalizeCap(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	if strings.HasPrefix(c, "CAP_") {
		return c
	}
	return "CAP_" + c
}

func hasDevice(parentDevices []container.DeviceMapping, childDev container.DeviceMapping) bool {
	childHostPath := filepath.Clean(childDev.PathOnHost)
	for _, pd := range parentDevices {
		parentHostPath := filepath.Clean(pd.PathOnHost)
		if parentHostPath == childHostPath {
			if isCgroupPermSubset(childDev.CgroupPermissions, pd.CgroupPermissions) {
				return true
			}
		}
	}
	return false
}

func isCgroupPermSubset(childPerms, parentPerms string) bool {
	if parentPerms == "" {
		parentPerms = "rwm"
	}
	if childPerms == "" {
		childPerms = "rwm"
	}
	for _, char := range childPerms {
		if !strings.ContainsRune(parentPerms, char) {
			return false
		}
	}
	return true
}

func isParentMountReadOnly(parentMounts []container.Mount, childSource, childTarget string) bool {
	childSourceClean := filepath.Clean(childSource)
	childTargetClean := filepath.Clean(childTarget)

	for _, pm := range parentMounts {
		if !pm.ReadOnly {
			continue
		}
		pSourceClean := filepath.Clean(pm.Source)
		pTargetClean := filepath.Clean(pm.Target)

		if pSourceClean != "." && (isSubPathOrEqual(childSourceClean, pSourceClean) || isSubPathOrEqual(pSourceClean, childSourceClean)) {
			return true
		}
		if pTargetClean != "." && (isSubPathOrEqual(childTargetClean, pTargetClean) || isSubPathOrEqual(pTargetClean, childTargetClean)) {
			return true
		}
	}
	return false
}

func isSubPathOrEqual(childPath, parentPath string) bool {
	if childPath == parentPath {
		return true
	}
	if parentPath == string(filepath.Separator) {
		return true
	}
	parentDir := parentPath
	if !strings.HasSuffix(parentDir, string(filepath.Separator)) {
		parentDir += string(filepath.Separator)
	}
	return strings.HasPrefix(childPath, parentDir)
}

func isSecurityOptAllowed(parentOpts []string, childOpt string) bool {
	optLower := strings.ToLower(strings.TrimSpace(childOpt))
	if optLower == "no-new-privileges" || optLower == "no-new-privileges:true" || optLower == "no-new-privileges=true" {
		return true
	}
	return hasSecurityOpt(parentOpts, childOpt)
}

func hasSecurityOpt(parentOpts []string, childOpt string) bool {
	childOptLower := strings.ToLower(strings.TrimSpace(childOpt))
	for _, po := range parentOpts {
		if strings.ToLower(strings.TrimSpace(po)) == childOptLower {
			return true
		}
	}
	return false
}
