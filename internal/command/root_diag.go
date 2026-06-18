package command

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"

	"cderun/internal/config"
	"cderun/internal/container"
)

type diagnosticsInfo struct {
	Runtime struct {
		Name   string `json:"name" yaml:"name"`
		Socket string `json:"socket" yaml:"socket"`
		Status string `json:"status" yaml:"status"`
	} `json:"runtime" yaml:"runtime"`
	Configs struct {
		Global []string `json:"global" yaml:"global"`
		Tools  []string `json:"tools" yaml:"tools"`
	} `json:"configs" yaml:"configs"`
	AvailableTools []string `json:"available_tools,omitempty" yaml:"available_tools,omitempty"`
}

func (o *rootOptions) handleDiagnosis(cmd *cobra.Command, resolved *config.ResolvedConfig, toolsCfg config.ToolsConfig, globalPaths, toolsPaths []string) error {
	o.ensureHooks()
	info := diagnosticsInfo{}
	info.Runtime.Name = resolved.Runtime
	info.Runtime.Socket = resolved.SocketPath
	if _, err := o.fs.Stat(resolved.SocketPath); err == nil {
		info.Runtime.Status = "accessible"
	} else {
		info.Runtime.Status = fmt.Sprintf("not found or inaccessible: %v", err)
	}
	info.Configs.Global = globalPaths
	info.Configs.Tools = toolsPaths
	for toolName := range toolsCfg {
		info.AvailableTools = append(info.AvailableTools, toolName)
	}
	sort.Strings(info.AvailableTools)

	return o.writeFormatted(cmd.OutOrStdout(), resolved.DiagnosisFormat, info, func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "Runtime: %s (%s)\n", info.Runtime.Name, info.Runtime.Socket)
		_, _ = fmt.Fprintf(w, "Runtime Status: %s\n", info.Runtime.Status)
		_, _ = fmt.Fprintf(w, "Global Config: %s\n", strings.Join(info.Configs.Global, ", "))
		_, _ = fmt.Fprintf(w, "Tools Config: %s\n", strings.Join(info.Configs.Tools, ", "))
		_, _ = fmt.Fprintf(w, "Available Tools: %s\n", strings.Join(info.AvailableTools, ", "))
	})
}

func (o *rootOptions) handleDryRun(cmd *cobra.Command, containerConfig *container.ContainerConfig, resolved *config.ResolvedConfig) error {
	o.ensureHooks()

	// Mask sensitive environment variables in dry-run output
	maskedContainerConfig := *containerConfig
	maskedContainerConfig.Env = config.MaskSensitiveEnvList(containerConfig.Env)

	return o.writeFormatted(cmd.OutOrStdout(), resolved.DryRunFormat, &maskedContainerConfig, func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "Image: %s\n", maskedContainerConfig.Image)
		var quotedCmd []string
		for _, arg := range maskedContainerConfig.Command {
			quotedCmd = append(quotedCmd, fmt.Sprintf("%q", arg))
		}
		_, _ = fmt.Fprintf(w, "Command: %s\n", strings.Join(quotedCmd, " "))
		_, _ = fmt.Fprintf(w, "TTY: %v\n", maskedContainerConfig.TTY)
		_, _ = fmt.Fprintf(w, "Interactive: %v\n", maskedContainerConfig.Interactive)
		_, _ = fmt.Fprintf(w, "Network: %s\n", maskedContainerConfig.Network)
		_, _ = fmt.Fprintf(w, "Remove: %v\n", maskedContainerConfig.Remove)
		var mounts []string
		for _, m := range maskedContainerConfig.Mounts {
			mounts = append(mounts, fmt.Sprintf("type=%s,source=%q,target=%q,readonly=%v", m.Type, m.Source, m.Target, m.ReadOnly))
		}
		_, _ = fmt.Fprintf(w, "Mounts: %s\n", strings.Join(mounts, ", "))
		var quotedEnvs []string
		for _, e := range maskedContainerConfig.Env {
			if k, v, found := strings.Cut(e, "="); found {
				quotedEnvs = append(quotedEnvs, fmt.Sprintf("%q=%q", k, v))
			} else {
				quotedEnvs = append(quotedEnvs, fmt.Sprintf("%q", e))
			}
		}
		_, _ = fmt.Fprintf(w, "Env: %s\n", strings.Join(quotedEnvs, ", "))
		_, _ = fmt.Fprintf(w, "Workdir: %s\n", maskedContainerConfig.Workdir)
		_, _ = fmt.Fprintf(w, "User: %s\n", maskedContainerConfig.User)
		_, _ = fmt.Fprintf(w, "Ports: %s\n", strings.Join(maskedContainerConfig.Ports, ", "))
		_, _ = fmt.Fprintf(w, "PublishAll: %v\n", maskedContainerConfig.PublishAll)
		_, _ = fmt.Fprintf(w, "Expose: %s\n", strings.Join(maskedContainerConfig.Expose, ", "))
		_, _ = fmt.Fprintf(w, "Hostname: %s\n", maskedContainerConfig.Hostname)
		_, _ = fmt.Fprintf(w, "DNS: %s\n", strings.Join(maskedContainerConfig.DNS, ", "))
		_, _ = fmt.Fprintf(w, "AddHosts: %s\n", strings.Join(maskedContainerConfig.AddHosts, ", "))
		_, _ = fmt.Fprintf(w, "Privileged: %v\n", maskedContainerConfig.Privileged)
		_, _ = fmt.Fprintf(w, "CapAdd: %s\n", strings.Join(maskedContainerConfig.CapAdd, ", "))
		_, _ = fmt.Fprintf(w, "CapDrop: %s\n", strings.Join(maskedContainerConfig.CapDrop, ", "))

		var devices []string
		for _, d := range maskedContainerConfig.Devices {
			if d.PathOnHost == d.PathInContainer && d.CgroupPermissions == "rwm" {
				devices = append(devices, d.PathOnHost)
			} else {
				devices = append(devices, fmt.Sprintf("%s:%s:%s", d.PathOnHost, d.PathInContainer, d.CgroupPermissions))
			}
		}
		_, _ = fmt.Fprintf(w, "Devices: %s\n", strings.Join(devices, ", "))

		if maskedContainerConfig.Memory > 0 {
			_, _ = fmt.Fprintf(w, "Memory: %s\n", units.BytesSize(float64(maskedContainerConfig.Memory)))
		}
		if maskedContainerConfig.CPUs > 0 {
			_, _ = fmt.Fprintf(w, "CPUs: %s\n", strconv.FormatFloat(maskedContainerConfig.CPUs, 'f', -1, 64))
		}
		if len(maskedContainerConfig.Entrypoint) > 0 {
			var quotedEntry []string
			for _, arg := range maskedContainerConfig.Entrypoint {
				quotedEntry = append(quotedEntry, fmt.Sprintf("%q", arg))
			}
			_, _ = fmt.Fprintf(w, "Entrypoint: %s\n", strings.Join(quotedEntry, " "))
		}
	})
}

func (o *rootOptions) writeFormatted(w io.Writer, format string, data any, simpleWriter func(io.Writer)) error {
	switch strings.ToLower(format) {
	case "json":
		marshaled, err := o.jsonMarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		_, err = fmt.Fprintln(w, string(marshaled))
		return err
	case "simple":
		simpleWriter(w)
		return nil
	case "yaml", "": // Default to YAML if format is empty
		marshaled, err := o.yamlMarshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		_, err = fmt.Fprint(w, string(marshaled))
		return err
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}
