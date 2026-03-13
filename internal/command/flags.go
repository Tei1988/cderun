package command

import "github.com/spf13/cobra"

type boolFlagDef struct {
	p2Name, p1Name string
	p2Short        string
	defaultVal     bool
	p2Usage        string
	p2Field        *bool
	p1Field        *bool
}

type stringFlagDef struct {
	p2Name, p1Name string
	p2Short        string
	defaultVal     string
	p2Usage        string
	p2Field        *string
	p1Field        *string
}

type stringSliceFlagDef struct {
	p2Name, p1Name string
	p2Short        string
	p2Usage        string
	p2Field        *[]string
	p1Field        *[]string
}

type float64FlagDef struct {
	p2Name, p1Name string
	defaultVal     float64
	p2Usage        string
	p2Field        *float64
	p1Field        *float64
}

func registerFlags(cmd *cobra.Command, o *rootOptions) {
	boolDefs := []boolFlagDef{
		{"tty", "cderun-tty", "t", false, "Allocate a pseudo-TTY", &o.tty, &o.cderunTTY},
		{"interactive", "cderun-interactive", "i", false, "Keep STDIN open even if not attached", &o.interactive, &o.cderunInteractive},
		{"mount-socket", "cderun-mount-socket", "", false, "Mount the container runtime socket into the container", &o.mountSocket, &o.cderunMountSocket},
		{"mount-cderun", "cderun-mount-cderun", "", false, "Mount cderun binary for use inside container", &o.mountCderun, &o.cderunMountCderun},
		{"mount-all-tools", "cderun-mount-all-tools", "", false, "Mount all defined tools into the container", &o.mountAllTools, &o.cderunMountAllTools},
		{"remove", "cderun-remove", "", true, "Automatically remove the container when it exits", &o.remove, &o.cderunRemove},
		{"publish-all", "cderun-publish-all", "P", false, "Publish all exposed ports to random ports", &o.publishAll, &o.cderunPublishAll},
		{"privileged", "cderun-privileged", "", false, "Give extended privileges to this container", &o.privileged, &o.cderunPrivileged},
		{"strict-env", "cderun-strict-env", "", false, "Require all environment variables to be present on the host", &o.strictEnv, &o.cderunStrictEnv},
		{"dry-run", "cderun-dry-run", "", false, "Preview container configuration without execution", &o.dryRun, &o.cderunDryRun},
		{"diagnosis", "cderun-diagnosis", "", false, "Show system diagnostics and available tools", &o.diagnosis, &o.cderunDiagnosis},
		{"log-timestamp", "cderun-log-timestamp", "", true, "Include timestamp in logs", &o.logTimestamp, &o.cderunLogTimestamp},
	}

	stringDefs := []stringFlagDef{
		{"network", "cderun-network", "", "bridge", "Connect a container to a network", &o.network, &o.cderunNetwork},
		{"socket-path", "cderun-socket-path", "", "", "Path to the container runtime socket on the host", &o.socketPath, &o.cderunSocketPath},
		{"mount-socket-path", "cderun-mount-socket-path", "", "", "Path where the socket should be mounted inside the container (defaults to host path)", &o.mountSocketPath, &o.cderunMountSocketPath},
		{"mount-cderun-path", "cderun-mount-cderun-path", "", "", "Host path to cderun binary to mount inside container", &o.mountCderunPath, &o.cderunMountCderunPath},
		{"image", "cderun-image", "", "", "Docker image to use", &o.image, &o.cderunImage},
		{"runtime", "cderun-runtime", "", "docker", "Container runtime to use (docker/podman)", &o.runtimeName, &o.cderunRuntime},
		{"workdir", "cderun-workdir", "w", "", "Working directory inside the container", &o.workdir, &o.cderunWorkdir},
		{"mount-tools", "cderun-mount-tools", "", "", "Mount specified tools into the container", &o.mountTools, &o.cderunMountTools},
		{"config", "cderun-config", "", "", "Path to cderun config file", &o.configPath, &o.cderunConfigPath},
		{"tool-config", "cderun-tool-config", "", "", "Path to tools config file", &o.toolConfigPath, &o.cderunToolConfigPath},
		{"hostname", "cderun-hostname", "", "", "Container host name", &o.hostname, &o.cderunHostname},
		{"user", "cderun-user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])", &o.user, &o.cderunUser},
		{"pull", "cderun-pull", "", "missing", "Pull image before running (always, missing, never)", &o.pull, &o.cderunPull},
		{"memory", "cderun-memory", "m", "", "Memory limit", &o.memory, &o.cderunMemory},
		{"dry-run-format", "cderun-dry-run-format", "f", "yaml", "Output format (yaml, json, simple)", &o.dryRunFormat, &o.cderunDryRunFormat},
		{"diagnosis-format", "cderun-diagnosis-format", "", "yaml", "Diagnosis output format (yaml, json, simple)", &o.diagnosisFormat, &o.cderunDiagnosisFormat},
		{"log-level", "cderun-log-level", "", "", "Set log level (error, warn, info, debug, trace)", &o.logLevel, &o.cderunLogLevel},
		{"log-format", "cderun-log-format", "", "text", "Set log format (text, json)", &o.logFormat, &o.cderunLogFormat},
		{"hang-timeout", "cderun-hang-timeout", "", "", "Grace period after I/O completion before force-terminating the container (e.g. 2s, 500ms)", &o.hangTimeout, &o.cderunHangTimeout},
	}

	stringSliceDefs := []stringSliceFlagDef{
		{"env", "cderun-env", "e", "Set environment variables", &o.env, &o.cderunEnv},
		{"mount", "cderun-mount", "", "Attach a filesystem mount to the container", &o.mounts, &o.cderunMounts},
		{"publish", "cderun-publish", "p", "Publish a container's port(s) to the host", &o.ports, &o.cderunPorts},
		{"expose", "cderun-expose", "", "Expose a port or a range of ports", &o.expose, &o.cderunExpose},
		{"dns", "cderun-dns", "", "Set custom DNS servers", &o.dns, &o.cderunDNS},
		{"add-host", "cderun-add-host", "", "Add a custom host-to-IP mapping (host:ip)", &o.addHosts, &o.cderunAddHosts},
		{"cap-add", "cderun-cap-add", "", "Add Linux capabilities", &o.capAdd, &o.cderunCapAdd},
		{"cap-drop", "cderun-cap-drop", "", "Drop Linux capabilities", &o.capDrop, &o.cderunCapDrop},
		{"entrypoint", "cderun-entrypoint", "", "Overwrite the default ENTRYPOINT of the image", &o.entrypoint, &o.cderunEntrypoint},
		{"device", "cderun-device", "", "Add a host device to the container", &o.devices, &o.cderunDevices},
	}

	float64Defs := []float64FlagDef{
		{"cpus", "cderun-cpus", 0, "Number of CPUs", &o.cpus, &o.cderunCPUs},
	}

	f := cmd.PersistentFlags()
	for _, d := range boolDefs {
		if d.p2Short != "" {
			f.BoolVarP(d.p2Field, d.p2Name, d.p2Short, d.defaultVal, d.p2Usage)
		} else {
			f.BoolVar(d.p2Field, d.p2Name, d.defaultVal, d.p2Usage)
		}
		f.BoolVar(d.p1Field, d.p1Name, d.defaultVal, "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
	for _, d := range stringDefs {
		if d.p2Short != "" {
			f.StringVarP(d.p2Field, d.p2Name, d.p2Short, d.defaultVal, d.p2Usage)
		} else {
			f.StringVar(d.p2Field, d.p2Name, d.defaultVal, d.p2Usage)
		}
		f.StringVar(d.p1Field, d.p1Name, "", "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
	for _, d := range stringSliceDefs {
		if d.p2Short != "" {
			f.StringArrayVarP(d.p2Field, d.p2Name, d.p2Short, nil, d.p2Usage)
		} else {
			f.StringArrayVar(d.p2Field, d.p2Name, nil, d.p2Usage)
		}
		f.StringArrayVar(d.p1Field, d.p1Name, nil, "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
	for _, d := range float64Defs {
		f.Float64Var(d.p2Field, d.p2Name, d.defaultVal, d.p2Usage)
		f.Float64Var(d.p1Field, d.p1Name, 0, "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
}
