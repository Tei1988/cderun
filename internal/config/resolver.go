package config

import (
	"cderun/internal/container"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ResolvedConfig contains the final values after resolution.
type ResolvedConfig struct {
	Image       string
	TTY         bool
	Interactive bool
	Network     string
	Remove      bool
	Volumes     []container.VolumeMount
	Env         []string
	Workdir     string
	User        string
	Runtime     string
	Socket      string
}

// CLIOptions represents values from CLI flags.
type CLIOptions struct {
	Image                string
	ImageSet             bool
	TTY                  bool
	TTYSet               bool
	Interactive          bool
	InteractiveSet       bool
	Network              string
	NetworkSet           bool
	Remove               bool
	RemoveSet            bool
	CderunTTY            bool
	CderunTTYSet         bool
	CderunInteractive    bool
	CderunInteractiveSet bool
	Runtime              string
	RuntimeSet           bool
	MountSocket          string
	MountSocketSet       bool
}

// Resolve combines CLI flags, environment variables, tool-specific config, and global defaults.
func Resolve(subcommand string, cli CLIOptions, tools ToolsConfig, global *CDERunConfig) (*ResolvedConfig, error) {
	res := &ResolvedConfig{}

	// 1. Resolve Image
	if cli.ImageSet {
		res.Image = cli.Image
	} else if env := os.Getenv("CDERUN_IMAGE"); env != "" {
		res.Image = env
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok && tool.Image != "" {
			res.Image = tool.Image
		}
	}

	if res.Image == "" {
		return nil, fmt.Errorf("no image mapping found for tool: %s", subcommand)
	}

	// 2. Resolve TTY
	res.TTY = resolveBool(
		cli.CderunTTYSet, cli.CderunTTY,
		cli.TTYSet, cli.TTY,
		"CDERUN_TTY",
		subcommand, tools, func(t ToolConfig) *bool { return t.TTY },
		global, func(g CDERunConfig) *bool { return g.Defaults.TTY },
		false,
	)

	// 3. Resolve Interactive
	res.Interactive = resolveBool(
		cli.CderunInteractiveSet, cli.CderunInteractive,
		cli.InteractiveSet, cli.Interactive,
		"CDERUN_INTERACTIVE",
		subcommand, tools, func(t ToolConfig) *bool { return t.Interactive },
		global, func(g CDERunConfig) *bool { return g.Defaults.Interactive },
		false,
	)

	// 4. Resolve Network
	res.Network = resolveString(
		cli.NetworkSet, cli.Network,
		"CDERUN_NETWORK",
		subcommand, tools, func(t ToolConfig) string { return t.Network },
		global, func(g CDERunConfig) string { return g.Defaults.Network },
		"bridge",
	)

	// 5. Resolve Remove
	res.Remove = resolveBool(
		false, false, // No P1 for Remove
		cli.RemoveSet, cli.Remove,
		"CDERUN_REMOVE",
		subcommand, tools, func(t ToolConfig) *bool { return t.Remove },
		global, func(g CDERunConfig) *bool { return g.Defaults.Remove },
		true, // Default to true as per docs
	)

	// 6. Tool-specific settings (Volumes, Env, Workdir)
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			res.Volumes = parseVolumes(tool.Volumes)
			res.Env = tool.Env
			res.Workdir = tool.Workdir
		}
	}

	// 7. Resolve Runtime
	res.Runtime = resolveString(
		cli.RuntimeSet, cli.Runtime,
		"CDERUN_RUNTIME",
		"", nil, nil, // No tool-specific runtime
		global, func(g CDERunConfig) string { return g.Runtime },
		"docker",
	)

	// 8. Resolve Socket
	res.Socket = resolveString(
		cli.MountSocketSet, cli.MountSocket,
		"DOCKER_HOST", // Or CDERUN_SOCKET? DOCKER_HOST is common
		"", nil, nil,
		nil, nil, // Global doesn't have socket path yet in schema but could
		"/var/run/docker.sock",
	)
	// Special handling for DOCKER_HOST unix:// prefix
	res.Socket = strings.TrimPrefix(res.Socket, "unix://")

	return res, nil
}

func resolveBool(p1Set bool, p1Val bool, p2Set bool, p2Val bool, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) *bool, global *CDERunConfig, globalGetter func(CDERunConfig) *bool, fallback bool) bool {
	if p1Set {
		return p1Val
	}
	if p2Set {
		return p2Val
	}
	if env := os.Getenv(envKey); env != "" {
		if b, err := strconv.ParseBool(env); err == nil {
			return b
		}
	}
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if b := toolGetter(tool); b != nil {
				return *b
			}
		}
	}
	if global != nil {
		if b := globalGetter(*global); b != nil {
			return *b
		}
	}
	return fallback
}

func resolveString(cliSet bool, cliVal string, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) string, global *CDERunConfig, globalGetter func(CDERunConfig) string, fallback string) string {
	if cliSet {
		return cliVal
	}
	if env := os.Getenv(envKey); env != "" {
		return env
	}
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if s := toolGetter(tool); s != "" {
				return s
			}
		}
	}
	if global != nil {
		if s := globalGetter(*global); s != "" {
			return s
		}
	}
	return fallback
}

func parseVolumes(vols []string) []container.VolumeMount {
	var mounts []container.VolumeMount
	for _, v := range vols {
		parts := strings.Split(v, ":")
		if len(parts) >= 2 {
			m := container.VolumeMount{
				HostPath:      parts[0],
				ContainerPath: parts[1],
			}
			if len(parts) >= 3 && parts[2] == "ro" {
				m.ReadOnly = true
			}
			mounts = append(mounts, m)
		}
	}
	return mounts
}
