package config

func init() {
	StringSliceOptions = append(StringSliceOptions, StringSliceOption{
		Name:      "sysctl",
		FieldName: "Sysctls",
		EnvKey:    "CDERUN_SYSCTL",
		Usage:     "Set kernel parameters (sysctl) (e.g. net.ipv4.ip_forward=1)",
		ToolGetter: func(t ToolConfig) []string {
			return t.Sysctls
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.Sysctls
		},
		SkipResolution: true, // resolved in resolveComplexOptions
	})
}
