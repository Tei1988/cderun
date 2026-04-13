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

		v := reflect.ValueOf(o).Elem()
		p2Field := v.FieldByName(fieldName)
		if !p2Field.IsValid() {
			panic(fmt.Sprintf("could not find field %q for option %q", fieldName, name))
		}

		p1FieldName := "Cderun" + fieldName
		p1Field := v.FieldByName(p1FieldName)
		if !p1Field.IsValid() {
			panic(fmt.Sprintf("could not find field %q for option %q", p1FieldName, name))
		}

		switch dv := defVal.(type) {
		case bool:
			p2 := p2Field.Addr().Interface().(*bool)
			p1 := p1Field.Addr().Interface().(*bool)
			if shorthand != "" {
				f.BoolVarP(p2, name, shorthand, dv, usage)
			} else {
				f.BoolVar(p2, name, dv, usage)
			}
			f.BoolVar(p1, "cderun-"+name, dv, "Override "+name+" setting (highest priority, can be used after subcommand)")
		case string:
			p2 := p2Field.Addr().Interface().(*string)
			p1 := p1Field.Addr().Interface().(*string)
			if shorthand != "" {
				f.StringVarP(p2, name, shorthand, dv, usage)
			} else {
				f.StringVar(p2, name, dv, usage)
			}
			f.StringVar(p1, "cderun-"+name, "", "Override "+name+" setting (highest priority, can be used after subcommand)")
		case int:
			p2 := p2Field.Addr().Interface().(*int)
			p1 := p1Field.Addr().Interface().(*int)
			if shorthand != "" {
				f.IntVarP(p2, name, shorthand, dv, usage)
			} else {
				f.IntVar(p2, name, dv, usage)
			}
			f.IntVar(p1, "cderun-"+name, 0, "Override "+name+" setting (highest priority, can be used after subcommand)")
		case float64:
			p2 := p2Field.Addr().Interface().(*float64)
			p1 := p1Field.Addr().Interface().(*float64)
			if shorthand != "" {
				f.Float64VarP(p2, name, shorthand, dv, usage)
			} else {
				f.Float64Var(p2, name, dv, usage)
			}
			f.Float64Var(p1, "cderun-"+name, 0, "Override "+name+" setting (highest priority, can be used after subcommand)")
		case []string:
			p2 := p2Field.Addr().Interface().(*[]string)
			p1 := p1Field.Addr().Interface().(*[]string)
			if shorthand != "" {
				f.StringArrayVarP(p2, name, shorthand, nil, usage)
			} else {
				f.StringArrayVar(p2, name, nil, usage)
			}
			f.StringArrayVar(p1, "cderun-"+name, nil, "Override "+name+" setting (highest priority, can be used after subcommand)")
		}
	}

	for _, opt := range config.BoolOptions { process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, opt.Default) }
	for _, opt := range config.StringOptions { process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, opt.Default) }
	for _, opt := range config.IntOptions { process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, opt.Default) }
	for _, opt := range config.Float64Options { process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, opt.Default) }
	for _, opt := range config.StringSliceOptions { process(opt.Name, opt.FieldName, opt.Shorthand, opt.Usage, []string(nil)) }
}

func getFlagPointers[T any](o *rootOptions, name, fieldName string) (p2, p1 *T) {
	v := reflect.ValueOf(o).Elem()
	if fieldName == "" {
		fieldName = config.PascalCase(name)
	}

	p2Field := v.FieldByName(fieldName)
	if !p2Field.IsValid() {
		panic(fmt.Sprintf("could not find field %q for option %q", fieldName, name))
	}
	p2 = p2Field.Addr().Interface().(*T)

	p1FieldName := "Cderun" + fieldName
	p1Field := v.FieldByName(p1FieldName)
	if !p1Field.IsValid() {
		panic(fmt.Sprintf("could not find field %q for option %q", p1FieldName, name))
	}
	p1 = p1Field.Addr().Interface().(*T)

	return p2, p1
}
