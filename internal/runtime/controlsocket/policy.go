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

	// 2. Linux Capabilities (CapAdd & CapDrop)
	if err := validateCapAdd(child.CapAdd, parent.CapAdd, parent.CapDrop); err != nil {
		return err
	}
	if err := validateCapDrop(child.CapDrop, parent.CapDrop); err != nil {
		return err
	}

	// 3. Namespace Isolation & Runtimes (Network, Pid, Ipc, Cgroupns, OciRuntime)
	if err := validateNamespaces(child, parent); err != nil {
		return err
	}

	// 4. Kernel Parameters (Sysctls)
	if err := validateSysctls(child.Sysctls, parent.Sysctls); err != nil {
		return err
	}

	// 5. Host Devices
	if err := validateDevices(child.Devices, parent.Devices); err != nil {
		return err
	}

	// 6. Read-Only Mount Boundaries & Source Paths
	if err := validateMounts(child.Mounts, parent); err != nil {
		return err
	}

	// 7. Security Options
	if err := validateSecurityOpt(child.SecurityOpt, parent.SecurityOpt); err != nil {
		return err
	}

	return nil
}

func isASCIIWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func trimSpaceASCII(s string) string {
	start := 0
	for start < len(s) && isASCIIWhitespace(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isASCIIWhitespace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func normalizeCap(c string) string {
	trimmed := trimSpaceASCII(c)
	if trimmed == "" {
		return ""
	}

	// Check if already uppercase ASCII and starts with CAP_
	allUpper := true
	for i := 0; i < len(trimmed); i++ {
		b := trimmed[i]
		if b >= 'a' && b <= 'z' {
			allUpper = false
			break
		}
	}

	if allUpper {
		return strings.TrimPrefix(trimmed, "CAP_")
	}

	c = strings.ToUpper(trimmed)
	return strings.TrimPrefix(c, "CAP_")
}

func validateCapAdd(childCaps, parentCaps, parentDropCaps []string) error {
	if len(childCaps) == 0 {
		return nil
	}

	if len(parentCaps) == 0 && len(parentDropCaps) == 0 {
		for _, cap := range childCaps {
			return fmt.Errorf("security violation: child requested capability %q not permitted by parent", cap)
		}
		return nil
	}

	parentSet := make(map[string]bool, len(parentCaps))
	for _, cap := range parentCaps {
		parentSet[normalizeCap(cap)] = true
	}

	parentDropSet := make(map[string]bool, len(parentDropCaps))
	for _, cap := range parentDropCaps {
		parentDropSet[normalizeCap(cap)] = true
	}

	parentHasAll := parentSet["ALL"]

	for _, cap := range childCaps {
		norm := normalizeCap(cap)
		if norm == "ALL" {
			if len(parentDropSet) > 0 {
				return fmt.Errorf("security violation: child requested CapAdd \"ALL\" when parent dropped capabilities %v", parentDropCaps)
			}
			if !parentHasAll {
				return fmt.Errorf("security violation: child requested capability %q not permitted by parent", cap)
			}
			continue
		}

		if parentDropSet["ALL"] || parentDropSet[norm] {
			return fmt.Errorf("security violation: child requested capability %q dropped by parent", cap)
		}
		if !parentHasAll && !parentSet[norm] {
			return fmt.Errorf("security violation: child requested capability %q not permitted by parent", cap)
		}
	}
	return nil
}

func validateCapDrop(childDrop, parentDrop []string) error {
	if len(parentDrop) == 0 {
		return nil
	}

	// Fast path check if parent dropped ALL
	parentHasAll := false
	for _, cap := range parentDrop {
		if normalizeCap(cap) == "ALL" {
			parentHasAll = true
			break
		}
	}

	if len(childDrop) == 0 {
		if parentHasAll {
			return fmt.Errorf("security violation: parent dropped ALL capabilities, child must also drop ALL")
		}
		return fmt.Errorf("security violation: child must drop capability %q dropped by parent", parentDrop[0])
	}

	childDropSet := make(map[string]bool, len(childDrop))
	for _, cap := range childDrop {
		childDropSet[normalizeCap(cap)] = true
	}

	if childDropSet["ALL"] {
		return nil
	}

	if parentHasAll {
		return fmt.Errorf("security violation: parent dropped ALL capabilities, child must also drop ALL")
	}

	for _, cap := range parentDrop {
		norm := normalizeCap(cap)
		if !childDropSet[norm] {
			return fmt.Errorf("security violation: child must drop capability %q dropped by parent", cap)
		}
	}
	return nil
}

func normalizeNamespaceMode(s string) string {
	s = trimSpaceASCII(s)
	if s == "" {
		return ""
	}
	allLower := true
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 'A' && b <= 'Z' {
			allLower = false
			break
		}
	}
	if allLower {
		return s
	}
	return strings.ToLower(s)
}

func validateNamespaces(child, parent *container.ContainerConfig) error {
	// Network
	if child.Network != parent.Network {
		parentNet := normalizeNamespaceMode(parent.Network)
		childNet := normalizeNamespaceMode(child.Network)

		if parentNet == "none" && childNet != "" && childNet != "none" {
			return fmt.Errorf("security violation: child requested network mode %q when parent network is %q", child.Network, parent.Network)
		}
		if childNet == "host" && parentNet != "host" {
			return fmt.Errorf("security violation: child requested host network mode when parent is not host network")
		}
		if strings.HasPrefix(childNet, "container:") && childNet != parentNet {
			return fmt.Errorf("security violation: child requested container network mode %q not permitted by parent %q", child.Network, parent.Network)
		}
	}

	// PID
	if child.Pid != parent.Pid {
		childPid := normalizeNamespaceMode(child.Pid)
		parentPid := normalizeNamespaceMode(parent.Pid)
		if childPid == "host" && parentPid != "host" {
			return fmt.Errorf("security violation: child requested host PID mode when parent is not host PID")
		}
		if strings.HasPrefix(childPid, "container:") && childPid != parentPid {
			return fmt.Errorf("security violation: child requested container PID mode %q not permitted by parent %q", child.Pid, parent.Pid)
		}
	}

	// IPC
	if child.IPC != parent.IPC {
		childIpc := normalizeNamespaceMode(child.IPC)
		parentIpc := normalizeNamespaceMode(parent.IPC)
		if childIpc == "host" && parentIpc != "host" {
			return fmt.Errorf("security violation: child requested host IPC mode when parent is not host IPC")
		}
		if strings.HasPrefix(childIpc, "container:") && childIpc != parentIpc {
			return fmt.Errorf("security violation: child requested container IPC mode %q not permitted by parent %q", child.IPC, parent.IPC)
		}
	}

	// Cgroupns
	if child.Cgroupns != parent.Cgroupns {
		childCgroupns := normalizeNamespaceMode(child.Cgroupns)
		parentCgroupns := normalizeNamespaceMode(parent.Cgroupns)
		if childCgroupns == "host" && parentCgroupns != "host" {
			return fmt.Errorf("security violation: child requested host cgroupns mode when parent is not host cgroupns")
		}
	}

	// OciRuntime
	if child.OciRuntime != parent.OciRuntime {
		childOci := normalizeNamespaceMode(child.OciRuntime)
		parentOci := normalizeNamespaceMode(parent.OciRuntime)
		if childOci != "" && parentOci != "" && childOci != parentOci {
			return fmt.Errorf("security violation: child requested OCI runtime %q when parent specifies %q", child.OciRuntime, parent.OciRuntime)
		}
	}

	return nil
}

func validateSysctls(childSysctls, parentSysctls map[string]string) error {
	if len(childSysctls) == 0 {
		return nil
	}
	for k, childVal := range childSysctls {
		parentVal, exists := parentSysctls[k]
		if !exists || parentVal != childVal {
			return fmt.Errorf("security violation: child requested sysctl %s=%q not permitted by parent", k, childVal)
		}
	}
	return nil
}

func validateDevices(childDevs, parentDevs []container.DeviceMapping) error {
	if len(childDevs) == 0 {
		return nil
	}

	if len(parentDevs) == 0 {
		return fmt.Errorf("security violation: child requested host device %q not permitted by parent", childDevs[0].PathOnHost)
	}

	parentDevMap := make(map[string]container.DeviceMapping, len(parentDevs))
	for _, pDev := range parentDevs {
		cleanHost := canonicalizePath(pDev.PathOnHost)
		parentDevMap[cleanHost] = pDev
	}

	for _, cDev := range childDevs {
		cleanHost := canonicalizePath(cDev.PathOnHost)
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

func canonicalizePath(p string) string {
	clean := filepath.Clean(p)
	eval, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return filepath.Clean(eval)
	}
	return clean
}

func hasPathTraversal(pathStr string) bool {
	s := filepath.ToSlash(pathStr)
	if !strings.Contains(s, "..") {
		return false
	}
	for s != "" {
		var seg string
		seg, s, _ = strings.Cut(s, "/")
		if seg == ".." {
			return true
		}
	}
	return false
}

func isSubpathOrEqual(sub, parent string) bool {
	subClean := canonicalizePath(sub)
	parentClean := canonicalizePath(parent)

	if subClean == parentClean {
		return true
	}
	rel, err := filepath.Rel(parentClean, subClean)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func validateMounts(childMounts []container.Mount, parent *container.ContainerConfig) error {
	if len(childMounts) == 0 {
		return nil
	}

	if parent.ReadOnly {
		for _, cMount := range childMounts {
			if cMount.Type == "bind" || cMount.Type == "" {
				if !cMount.ReadOnly {
					return fmt.Errorf("security violation: child requested read-write mount %q when parent root is read-only", cMount.Source)
				}
			}
		}
	}

	for _, cMount := range childMounts {
		if cMount.Type != "bind" && cMount.Type != "" {
			continue
		}

		if hasPathTraversal(cMount.Source) {
			return fmt.Errorf("security violation: child mount source %q contains path traversal components", cMount.Source)
		}

		cSource := canonicalizePath(cMount.Source)

		for _, pMount := range parent.Mounts {
			if !pMount.ReadOnly {
				continue
			}

			if !cMount.ReadOnly {
				pTarget := canonicalizePath(pMount.Target)
				pSource := canonicalizePath(pMount.Source)
				if (pTarget != "" && isSubpathOrEqual(cSource, pTarget)) ||
					(pSource != "" && isSubpathOrEqual(cSource, pSource)) {
					return fmt.Errorf("security violation: child requested read-write mount for %q within parent read-only path", cMount.Source)
				}
			}
		}

		if !cMount.ReadOnly {
			insideParentMount := false
			for _, pMount := range parent.Mounts {
				pSource := canonicalizePath(pMount.Source)
				if pSource != "" && isSubpathOrEqual(cSource, pSource) {
					insideParentMount = true
					break
				}
			}
			if !insideParentMount {
				return fmt.Errorf("security violation: child requested read-write mount for %q outside parent mount sources", cMount.Source)
			}
		}
	}

	return nil
}

func isHardeningOpt(opt string) bool {
	trimmed := trimSpaceASCII(opt)
	// Fast check for common exact matches without allocation
	if trimmed == "no-new-privileges" || trimmed == "no-new-privileges=true" || trimmed == "no-new-privileges:true" {
		return true
	}
	optLower := strings.ToLower(trimmed)
	return optLower == "no-new-privileges" || strings.HasPrefix(optLower, "no-new-privileges:") || optLower == "no-new-privileges=true"
}

func validateSecurityOpt(childOpts, parentOpts []string) error {
	if len(childOpts) == 0 {
		return nil
	}

	// Check if all child options are hardening options (e.g. no-new-privileges)
	// In this very common case, no parentSet map allocation or parentOpts iteration is needed.
	hasNonHardening := false
	for _, opt := range childOpts {
		if !isHardeningOpt(opt) {
			hasNonHardening = true
			break
		}
	}
	if !hasNonHardening {
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
