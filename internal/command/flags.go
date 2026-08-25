package command

import (
	"fmt"

	"cderun/internal/config"

	"github.com/spf13/cobra"
)

func registerFlags(cmd *cobra.Command, o *rootOptions) {
	f := cmd.PersistentFlags()
	const overrideHelpSuffix = " setting (highest priority, can be used after subcommand)"

	for _, opt := range config.BoolOptions {
		p2Field, p1Field := getBoolPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for bool option %q", opt.Name))
		}
		p1Name := "cderun-" + opt.Name
		if opt.Shorthand != "" {
			f.BoolVarP(p2Field, opt.Name, opt.Shorthand, opt.Default, opt.Usage)
		} else {
			f.BoolVar(p2Field, opt.Name, opt.Default, opt.Usage)
		}
		f.BoolVar(p1Field, p1Name, opt.Default, "Override "+opt.Name+overrideHelpSuffix)
	}

	for _, opt := range config.StringOptions {
		p2Field, p1Field := getStringPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for string option %q", opt.Name))
		}
		p1Name := "cderun-" + opt.Name
		if opt.Shorthand != "" {
			f.StringVarP(p2Field, opt.Name, opt.Shorthand, opt.Default, opt.Usage)
		} else {
			f.StringVar(p2Field, opt.Name, opt.Default, opt.Usage)
		}
		f.StringVar(p1Field, p1Name, "", "Override "+opt.Name+overrideHelpSuffix)
	}

	for _, opt := range config.IntOptions {
		p2Field, p1Field := getIntPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for int option %q", opt.Name))
		}
		p1Name := "cderun-" + opt.Name
		if opt.Shorthand != "" {
			f.IntVarP(p2Field, opt.Name, opt.Shorthand, opt.Default, opt.Usage)
		} else {
			f.IntVar(p2Field, opt.Name, opt.Default, opt.Usage)
		}
		f.IntVar(p1Field, p1Name, 0, "Override "+opt.Name+overrideHelpSuffix)
	}

	for _, opt := range config.Float64Options {
		p2Field, p1Field := getFloat64Pointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for float64 option %q", opt.Name))
		}
		p1Name := "cderun-" + opt.Name
		if opt.Shorthand != "" {
			f.Float64VarP(p2Field, opt.Name, opt.Shorthand, opt.Default, opt.Usage)
		} else {
			f.Float64Var(p2Field, opt.Name, opt.Default, opt.Usage)
		}
		f.Float64Var(p1Field, p1Name, 0, "Override "+opt.Name+overrideHelpSuffix)
	}

	for _, opt := range config.StringSliceOptions {
		p2Field, p1Field := getStringSlicePointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for string slice option %q", opt.Name))
		}
		p1Name := "cderun-" + opt.Name
		if opt.Shorthand != "" {
			f.StringArrayVarP(p2Field, opt.Name, opt.Shorthand, nil, opt.Usage)
		} else {
			f.StringArrayVar(p2Field, opt.Name, nil, opt.Usage)
		}
		f.StringArrayVar(p1Field, p1Name, nil, "Override "+opt.Name+overrideHelpSuffix)
	}
}
