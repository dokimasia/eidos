// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stubgen

import (
	"embed"
	"slices"
	"strings"
	"text/template"
	"unicode"

	"go.dokimi.dev/eidos/reference/internal/double"
	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
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
// The stub of a generic interface has the type parameters of the
// interface. Its test checks it at the witnesses of the type parameters,
// and a generic stub has no test where a type parameter has no witness.
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
// [double.Of] reports why such an interface has none. It also records
// nothing for an interface with a method that has type parameters, after
// it reports [GenericMethod] at the interface.
//
// The stub has the methods of the double and the type parameters of the
// interface. Its nested types are the call records of the methods, in the
// order of the methods. A call record has a field for each parameter of its
// method, with the name of the parameter. It has the type parameters that
// the types of the parameters mention, so a TypeScript record passes
// noUnusedParameters. The templates write the type of each field from the
// parameter, because the list of a variadic parameter has no neutral
// spelling.
//
// The test has the [double.Signatures] of the methods of the stub and the
// type parameters of the interface. It implements the interface at the
// witnesses of the type parameters. generate does not record a test of a
// generic stub where a type parameter does not have a witness.
func generate(m *sdk.InterfaceMatch, e *sdk.Emitter) error {
	i := m.Interface
	name := e.JoinName(Word, i.Name)
	methods, stubbed := double.Of(m, name, i)
	if !stubbed {
		return nil
	}
	for _, method := range methods {
		if len(method.TypeParams) > 0 {
			m.Errorf(
				GenericMethod,
				"%s has the method %s with type parameters, whose arguments a call record cannot state, so it has no stub",
				i.Name,
				method.Name,
			)
			return nil
		}
	}
	stub := &emit.Struct{
		Origin:     i.ID,
		Name:       name,
		Visibility: i.Visibility,
		TypeParams: sdk.MirrorTypeParams(i.TypeParams),
	}
	stub.Implements = []*emit.TypeRef{{Spelling: i.Name, Target: i.ID, Args: sdk.TypeArgs(stub.TypeParams)}}
	stub.Methods.Append(methods...)
	for _, method := range methods {
		call := &emit.Struct{
			Name:       e.JoinName(CallWord, e.JoinName(method.Name, i.Name)),
			Visibility: i.Visibility,
			TypeParams: mentioned(sdk.MirrorTypeParams(i.TypeParams), method.Params),
		}
		for _, p := range method.Params {
			call.Fields.Append(&emit.Field{Name: p.Name})
		}
		stub.Types.Append(call)
	}
	d := data{Interface: i.Name}
	e.File().Record(d, stub)
	witnesses := m.Rules().Witnesses(i.TypeParams)
	if len(i.TypeParams) > 0 && witnesses == nil {
		return nil
	}
	var args []*emit.TypeRef
	for _, w := range witnesses {
		args = append(args, rules.EmitRef(w))
	}
	test := &emit.Struct{
		Origin:     i.ID,
		Name:       name,
		Visibility: i.Visibility,
		TypeParams: sdk.MirrorTypeParams(i.TypeParams),
		Implements: []*emit.TypeRef{{Spelling: i.Name, Target: i.ID, Args: args}},
	}
	test.Methods.Append(double.Signatures(methods)...)
	e.File(TestTag).Record(d, test)
	return nil
}

// mentioned returns the type parameters of tps that the type of a
// parameter of params mentions, in order, and nil where no type mentions
// one.
func mentioned(tps []*emit.TypeParam, params []*emit.Param) []*emit.TypeParam {
	var out []*emit.TypeParam
	for _, tp := range tps {
		if slices.ContainsFunc(params, func(p *emit.Param) bool { return mentions(p.Type, tp.Name) }) {
			out = append(out, tp)
		}
	}
	return out
}

// mentions reports whether the reference t mentions the type parameter
// named name. For a named reference, the spelling or a type argument must
// match the name. For a structural reference, a word of the written
// spelling must equal the name, because a written spelling contains the
// spelling of each child. A word is a run of letters, digits, underscores
// and dollar signs, so a qualified name, a literal type and a member of an
// inline body with the name of the parameter count as a mention. A nil
// reference, which a parameter without a type has, reports false.
func mentions(t *emit.TypeRef, name string) bool {
	if t == nil {
		return false
	}
	if t.Form != symbol.FormNamed {
		words := strings.FieldsFunc(t.Spelling, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$'
		})
		return slices.Contains(words, name)
	}
	if t.Spelling == name {
		return true
	}
	return slices.ContainsFunc(t.Args, func(a *emit.TypeRef) bool { return mentions(a, name) })
}
