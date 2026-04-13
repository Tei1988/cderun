package command

import (
	"fmt"
	"reflect"

	"cderun/internal/config"

	"github.com/spf13/cobra"
)

func registerFlags(cmd *cobra.Command, o *rootOptions) {
	f := cmd.PersistentFlags()

	process := func(name, fieldName, shorthand, usage string, defVal any) {
		if fieldName == "" {
			fieldName = config.PascalCase(name)
		}

		p2, p1 := getFlagPointersAny(o, name, fieldName)

		switch dv := defVal.(type) {
		case bool:
			p2b, _ := p2.(*bool)
			p1b, _ := p1.(*bool)
			if shorthand != "" {
				f.BoolVarP(p2b, name, shorthand, dv, usage)
			} else {
				f.BoolVar(p2b, name, dv, usage)
			}
			f.BoolVar(p1b, "cderun-"+name, dv, "Override "+name+" setting (highest priority, can be used after subcommand)")
		case string:
			p2s, _ := p2.(*string)
			p1s, _ := p1.(*string)
			if shorthand != "" {
				f.StringVarP(p2s, name, shorthand, dv, usage)
			} else {
				f.StringVar(p2s, name, dv, usage)
			}
			f.StringVar(p1s, "cderun-"+name, "", "Override "+name+" setting (highest priority, can be used after subcommand)")
		case int:
			p2i, _ := p2.(*int)
			p1i, _ := p1.(*int)
			if shorthand != "" {
				f.IntVarP(p2i, name, shorthand, dv, usage)
			} else {
				f.IntVar(p2i, name, dv, usage)
			}
			f.IntVar(p1i, "cderun-"+name, 0, "Override "+name+" setting (highest priority, can be used after subcommand)")
		case float64:
			p2f, _ := p2.(*float64)
			p1f, _ := p1.(*float64)
			if shorthand != "" {
				f.Float64VarP(p2f, name, shorthand, dv, usage)
			} else {
				f.Float64Var(p2f, name, dv, usage)
			}
			f.Float64Var(p1f, "cderun-"+name, 0, "Override "+name+" setting (highest priority, can be used after subcommand)")
		case []string:
			p2ss, _ := p2.(*[]string)
			p1ss, _ := p1.(*[]string)
			if shorthand != "" {
				f.StringArrayVarP(p2ss, name, shorthand, nil, usage)
			} else {
				f.StringArrayVar(p2ss, name, nil, usage)
			}
			f.StringArrayVar(p1ss, "cderun-"+name, nil, "Override "+name+" setting (highest priority, can be used after subcommand)")
		}
	}

	for _, opt := range config.BoolOptions {
		process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, opt.Default)
	}
	for _, opt := range config.StringOptions {
		process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, opt.Default)
	}
	for _, opt := range config.IntOptions {
		process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, opt.Default)
	}
	for _, opt := range config.Float64Options {
		process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, opt.Default)
	}
	for _, opt := range config.StringSliceOptions {
		process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, []string(nil))
	}
}

func getFlagPointersAny(o *rootOptions, name, fieldName string) (any, any) {
	v := reflect.ValueOf(o).Elem()
	if fieldName == "" {
		fieldName = config.PascalCase(name)
	}

	p2Field := v.FieldByName(fieldName)
	if !p2Field.IsValid() {
		panic(fmt.Sprintf("could not find field %q for option %q", fieldName, name))
	}
	p2 := p2Field.Addr().Interface()

	p1FieldName := "Cderun" + fieldName
	p1Field := v.FieldByName(p1FieldName)
	if !p1Field.IsValid() {
		panic(fmt.Sprintf("could not find field %q for option %q", p1FieldName, name))
	}
	p1 := p1Field.Addr().Interface()

	return p2, p1
}

func getFlagPointers[T any](o *rootOptions, name, fieldName string) (*T, *T) {
	p2, p1 := getFlagPointersAny(o, name, fieldName)
	p2t, _ := p2.(*T)
	p1t, _ := p1.(*T)
	return p2t, p1t
}
