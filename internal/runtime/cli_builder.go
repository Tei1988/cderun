package runtime

import (
	"fmt"
	"strings"
	"unicode"
)

// CLIArgBuilder constructs CLI arguments with argument injection defenses (CWE-88).
// It formats flags in joined `--flag=value` form, separates positional arguments
// with `--`, and performs structural self-checks on the resulting argv.
type CLIArgBuilder struct {
	flags       []string
	positionals []string
}

// NewCLIArgBuilder creates a new CLIArgBuilder instance.
func NewCLIArgBuilder() *CLIArgBuilder {
	return &CLIArgBuilder{
		flags:       make([]string, 0),
		positionals: make([]string, 0),
	}
}

// AddJoinedFlag adds a flag using the joined `--flag=value` syntax.
// If value is empty and omitEmpty is true, the flag is omitted.
func (b *CLIArgBuilder) AddJoinedFlag(flag, value string, omitEmpty bool) *CLIArgBuilder {
	if value == "" && omitEmpty {
		return b
	}
	b.flags = append(b.flags, flag+"="+value)
	return b
}

// AddBoolFlag adds a boolean flag `--flag` if condition is true.
func (b *CLIArgBuilder) AddBoolFlag(flag string, condition bool) *CLIArgBuilder {
	if condition {
		b.flags = append(b.flags, flag)
	}
	return b
}

// AddRepeatJoinedFlag adds multiple joined `--flag=value` entries for a list of values.
func (b *CLIArgBuilder) AddRepeatJoinedFlag(flag string, values []string) *CLIArgBuilder {
	for _, v := range values {
		b.flags = append(b.flags, flag+"="+v)
	}
	return b
}

// AddPositionals adds positional arguments (e.g. image, command, args) that will be placed after `--`.
func (b *CLIArgBuilder) AddPositionals(args ...string) *CLIArgBuilder {
	b.positionals = append(b.positionals, args...)
	return b
}

// Build constructs the full argv slice: [flags..., "--", positionals...].
// If positionals are present, "--" is inserted before them.
func (b *CLIArgBuilder) Build() []string {
	res := make([]string, 0, len(b.flags)+1+len(b.positionals))
	res = append(res, b.flags...)
	if len(b.positionals) > 0 {
		res = append(res, "--")
		res = append(res, b.positionals...)
	}
	return res
}

// VerifyStructure performs a structural self-check on the built argv.
// It verifies that:
// 1. The `--` separator exists if positionals are present.
// 2. All arguments prior to `--` begin with `-`.
// 3. The number of flags prior to `--` matches expectedFlagCount (if expectedFlagCount >= 0).
func (b *CLIArgBuilder) VerifyStructure(argv []string, expectedFlagCount int) error {
	doubleDashIdx := -1
	for i, arg := range argv {
		if arg == "--" {
			doubleDashIdx = i
			break
		}
	}

	flagSlice := argv
	if doubleDashIdx != -1 {
		flagSlice = argv[:doubleDashIdx]
	} else if len(b.positionals) > 0 {
		return fmt.Errorf("structural self-check failed: positionals present but '--' separator missing")
	}

	if expectedFlagCount >= 0 && len(flagSlice) != expectedFlagCount {
		return fmt.Errorf("structural self-check failed: expected %d flags before '--', got %d", expectedFlagCount, len(flagSlice))
	}

	for i, flagArg := range flagSlice {
		if !strings.HasPrefix(flagArg, "-") {
			return fmt.Errorf("structural self-check failed: flag argument at index %d (%q) does not start with '-'", i, flagArg)
		}
	}

	for i, arg := range argv {
		if strings.IndexByte(arg, 0) != -1 {
			return fmt.Errorf("structural self-check failed: argument at index %d contains null byte", i)
		}
		hasControl := false
		for j := 0; j < len(arg); j++ {
			c := arg[j]
			if c < 32 || c == 127 {
				if c != '\n' && c != '\r' && c != '\t' {
					hasControl = true
					break
				}
			} else if c >= 128 {
				hasControl = true
				break
			}
		}
		if hasControl {
			for pos, r := range arg {
				if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
					return fmt.Errorf("structural self-check failed: argument at index %d contains control character %q at position %d", i, r, pos)
				}
			}
		}
	}

	return nil
}
