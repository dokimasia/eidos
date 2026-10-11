// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stubgen

import (
	"embed"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/reference/internal/double"
	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// ID is the name of the plugin.
const ID plugin.ID = "stubgen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that stubgen provides. A plugin that
// extends a stub requires it.
const Capability plugin.Capability = "stub"

// StubDirective is the directive that puts an interface of the source
// under stubgen.
const StubDirective directive.Name = "stub"

// Word is the word of the family of the stubs and of the family of their
// tests. TestTag is the tag of the family of the tests. CallWord is the
// word that the name of a call record joins onto the name of the interface
// and the name of the method.
const (
	Word     = "stub"
	TestTag  = "test"
	CallWord = "call"
)

// stubTemplate is the template of each file of the stubs, and testTemplate
// the template of each file of their tests.
const (
	stubTemplate = "stub.tmpl"
	testTemplate = "stub.test.tmpl"
)

// trimSuffix is the helper that the TypeScript template of the tests calls.
// The template derives the module of a stub from the name of its own file
// through it.
const trimSuffix = "trimSuffix"

// schema is the directive of stubgen.
var schema = directive.Schema{
	Plugin: string(ID),
	Name:   StubDirective,
	Doc:    "generates a stub of the interface that records the arguments of each call and calls a function field",
}

// templates contains the templates of stubgen in one directory for each
// target.
//
//go:embed templates
var templates embed.FS

// data is the data that the file templates read beside the record of a
// stub and the record of its test. Interface is the name of the interface
// that the stub implements.
type data struct {
	Interface string
}

// New returns stubgen. It writes the stub <I>Stub of each interface under
// [StubDirective]. A stub has a function field and a list of call records
// for each method. Each method of the stub appends a record of its
// arguments to its list, and then calls its field.
//
// The family [Word] claims the files of the stubs through the template
// stub.tmpl of the plan's target. The family [Word] under the tag [TestTag]
// claims the files of their tests through the template stub.test.tmpl. The
// test of a Go stub checks that the stub implements its interface. It
// checks for each method that a call calls the field once and adds one
// record. The test of a TypeScript stub assigns a new stub to a constant
// of the interface's type. tsc checks the assignment.
//
// Each call returns a new plugin, because a plugin instance belongs to one
// workspace.
func New() plugin.Generator {
	g, _ := sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Output(plugin.Output{Per: plugin.PerSource, Word: Word, Template: stubTemplate}).
		Output(plugin.Output{Tag: TestTag, Per: plugin.PerSource, Word: Word, Template: testTemplate}).
		For(double.GoTarget, sdk.Templates(tree.Of(templates, double.GoTarget))).
		For(double.TypeScriptTarget,
			sdk.Templates(tree.Of(templates, double.TypeScriptTarget)),
			sdk.Funcs(template.FuncMap{trimSuffix: strings.TrimSuffix})).
		Handle(sdk.Directive(schema, sdk.OnInterface(generate))).
		Build().(plugin.Generator)
	return g
}

// generate records the stub of an interface under the directive, and the
// test of the stub. It records nothing for an interface without a double.
// [double.Of] reports why such an interface has none.
//
// The stub has the methods of the double. Its nested types are the call
// records of the methods, in the order of the methods. A call record has a
// field for each parameter of its method, with the name of the parameter.
// The templates write the type of each field from the parameter, because
// the list of a variadic parameter has no neutral spelling. The test has
// the [double.Signatures] of the methods of the stub.
func generate(m *sdk.InterfaceMatch, e *sdk.Emitter) error {
	i := m.Interface
	name := e.JoinName(Word, i.Name)
	methods, stubbed := double.Of(m, name, i)
	if !stubbed {
		return nil
	}
	stub := &emit.Struct{
		Origin:     i.ID,
		Name:       name,
		Visibility: i.Visibility,
		Implements: []*emit.TypeRef{{Spelling: i.Name, Target: i.ID}},
	}
	stub.Methods.Append(methods...)
	for _, method := range methods {
		call := &emit.Struct{Name: e.JoinName(CallWord, e.JoinName(method.Name, i.Name)), Visibility: i.Visibility}
		for _, p := range method.Params {
			call.Fields.Append(&emit.Field{Name: p.Name})
		}
		stub.Types.Append(call)
	}
	test := &emit.Struct{
		Origin:     i.ID,
		Name:       name,
		Visibility: i.Visibility,
		Implements: []*emit.TypeRef{{Spelling: i.Name, Target: i.ID}},
	}
	test.Methods.Append(double.Signatures(methods)...)
	d := data{Interface: i.Name}
	e.File().Record(d, stub)
	e.File(TestTag).Record(d, test)
	return nil
}
