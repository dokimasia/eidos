// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registrygen

import (
	"embed"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// ID is the name of the plugin.
const ID plugin.ID = "registrygen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// RegisterDirective is the directive that puts a struct under registrygen.
const RegisterDirective directive.Name = "register"

// Word is the word of the family of the registries.
const Word = "registry"

// DefaultPackage and DefaultFunc are the defaults of the options. They set
// the function Register of the package encoding/gob.
const (
	DefaultPackage = "encoding/gob"
	DefaultFunc    = "Register"
)

// fileTemplate is the template of each file of the family.
const fileTemplate = "registry.tmpl"

// schema is the directive of registrygen.
var schema = directive.Schema{
	Name: RegisterDirective,
	Doc:  "registers a value of the struct in the function init of its package",
}

// templates contains the templates of registrygen in one directory for
// each target.
//
//go:embed templates
var templates embed.FS

// Callee is the data that the file template reads beside the record of a
// struct. It is the function that registers the struct. Package is the
// import path of its package, and Name is the name by which the file refers
// to the package, which golang.Qualifier returns for the path. Func is the
// name of the function.
type Callee struct {
	Package string
	Name    string
	Func    string
}

// New returns registrygen. For each struct under [RegisterDirective], it
// records the struct into the file of the family [Word] of its package.
// The family claims its files through the template registry.tmpl. The
// template writes one function init for each file, with one call of the
// function of the options for each struct. Each call passes the zero value
// of its struct. The template records the import of the package through
// the builtin use.
//
// The options package and func set the function, and default to
// gob.Register. The plugin serves the target golang alone. Each call
// returns a new plugin, because a plugin instance belongs to one workspace.
func New() plugin.Generator {
	o := &callee.Options{Package: DefaultPackage, Func: DefaultFunc}
	return sdk.NewPlugin(ID).
		Version(Version).
		Options(o).
		Output(plugin.Output{Per: plugin.PerPackage, Word: Word, Template: fileTemplate}).
		For(golang.Target, sdk.Templates(tree.Of(templates, golang.Target))).
		Handle(sdk.Directive(schema, sdk.OnStruct(record(o)))).
		Generator()
}

// record returns the rule of registrygen under the options o. The rule
// records a struct under the directive into the registry of its package,
// with the [Callee] of the options. It records nothing where [callee.Check]
// returns false. It reports [Generic] at a struct with type
// parameters and records nothing, because the registry cannot make a value
// of the struct without its type arguments.
func record(o *callee.Options) func(*sdk.StructMatch, *sdk.Emitter) error {
	return func(m *sdk.StructMatch, e *sdk.Emitter) error {
		s := m.Struct
		if !callee.Check(m, s.ID, o.Package, o.Func) {
			return nil
		}
		if len(s.TypeParams) > 0 {
			m.Errorf(Generic, "%s has type parameters, so the registry cannot make a value of it", s.Name)
			return nil
		}
		e.PackageFile().Record(
			Callee{Package: o.Package, Name: golang.Qualifier(o.Package), Func: o.Func},
			&emit.Struct{Origin: s.ID, Name: s.Name, Visibility: s.Visibility},
		)
		return nil
	}
}
