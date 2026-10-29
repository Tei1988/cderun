package config

import (
	"reflect"
	"testing"
)

func TestUnit_ResolveStringSliceCommaOpt_Scenarios(t *testing.T) {
	def := OptionDef[[]string]{
		EnvKey: "TEST_COMMA_OPT",
		ToolGetter: func(t ToolConfig) []string {
			return t.MountTools
		},
		GlobalGetter: func(c *CDERunConfig) []string {
			return c.Defaults.MountTools
		},
	}

	tests := []struct {
		name       string
		p1Set      bool
		p1Val      string
		p2Set      bool
		p2Val      string
		envVal     string
		subcommand string
		tools      ToolsConfig
		global     *CDERunConfig
		resolver   *ExpressionResolver
		expected   []string
	}{
		{
			name:     "P1 empty string set returns empty non-nil slice",
			p1Set:    true,
			p1Val:    "",
			expected: []string{},
		},
		{
			name:     "P1 whitespace string returns empty non-nil slice",
			p1Set:    true,
			p1Val:    "   , , ",
			expected: []string{},
		},
		{
			name:     "P1 single value without commas",
			p1Set:    true,
			p1Val:    " kubectl ",
			expected: []string{"kubectl"},
		},
		{
			name:     "P1 multiple comma-separated values with whitespace",
			p1Set:    true,
			p1Val:    " kubectl , helm , kustomize ",
			expected: []string{"kubectl", "helm", "kustomize"},
		},
		{
			name:     "P2 precedence over env and tool config",
			p1Set:    false,
			p2Set:    true,
			p2Val:    "docker,podman",
			envVal:   "ignored",
			expected: []string{"docker", "podman"},
		},
		{
			name:     "Env fallback when P1 and P2 unset",
			p1Set:    false,
			p2Set:    false,
			envVal:   " git , gh ",
			expected: []string{"git", "gh"},
		},
		{
			name:       "Tool config fallback when P1 P2 and Env unset",
			p1Set:      false,
			p2Set:      false,
			subcommand: "kubectl",
			tools: ToolsConfig{
				"kubectl": ToolConfig{
					MountTools: []string{" k9s ", "stern"},
				},
			},
			expected: []string{"k9s", "stern"},
		},
		{
			name:  "Global config fallback when all higher layers unset",
			p1Set: false,
			p2Set: false,
			global: &CDERunConfig{
				Defaults: ConfigDefaults{
					MountTools: []string{" default-tool "},
				},
			},
			expected: []string{"default-tool"},
		},
		{
			name:     "Expression resolution in comma items",
			p1Set:    true,
			p1Val:    "{{HOME}}, ~/bin",
			resolver: &ExpressionResolver{Home: "/home/user"},
			expected: []string{"/home/user", "/home/user/bin"},
		},
		{
			name:     "Unset in all layers returns nil",
			p1Set:    false,
			p2Set:    false,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &MockFileSystem{}
			if tt.envVal != "" {
				fs.Env = map[string]string{"TEST_COMMA_OPT": tt.envVal}
			}

			got := resolveStringSliceCommaOpt(
				def,
				tt.p1Set, tt.p1Val,
				tt.p2Set, tt.p2Val,
				tt.subcommand, tt.tools, tt.global,
				tt.resolver, fs,
			)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("resolveStringSliceCommaOpt() = %v (len %d), want %v (len %d)", got, len(got), tt.expected, len(tt.expected))
			}
		})
	}
}

func BenchmarkResolveStringSliceCommaOpt(b *testing.B) {
	def := OptionDef[[]string]{
		EnvKey: "TEST_BENCH_COMMA",
	}
	fs := &MockFileSystem{
		Env: map[string]string{"TEST_BENCH_COMMA": "kubectl, helm, kustomize, terraform, ansible"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res := resolveStringSliceCommaOpt(def, false, "", false, "", "", nil, nil, nil, fs)
		if len(res) != 5 {
			b.Fatalf("expected 5 items, got %d", len(res))
		}
	}
}
