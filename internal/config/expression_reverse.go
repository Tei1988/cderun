package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (r *ExpressionResolver) applyReverseResolution(absPath string) (string, error) {
	if r.HostContext == nil || r.HostContext.Level == 0 {
		return absPath, nil
	}

	abs := absPath
	if !filepath.IsAbs(abs) {
		a, err := r.fs.Abs(abs)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for %q: %w", abs, err)
		}
		abs = a
	}

	found := false
	bestRel := ""
	bestSource := ""
	bestTarget := ""
	maxLevel := -1

	for _, m := range r.HostContext.Mounts {
		rel, err := filepath.Rel(m.Target, abs)
		if err == nil && !strings.HasPrefix(rel, "..") {
			if len(m.Target) > len(bestTarget) || (len(m.Target) == len(bestTarget) && m.Level > maxLevel) {
				maxLevel = m.Level
				bestTarget = m.Target
				bestSource = m.Source
				bestRel = rel
				found = true
			}
		}
	}

	if found {
		return filepath.Join(bestSource, bestRel), nil
	}
	return absPath, nil
}
