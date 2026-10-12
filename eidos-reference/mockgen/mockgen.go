// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package mockgen

import (
	"embed"
	"slices"

	"go.dokimi.dev/eidos/reference/internal/double"
	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/reference/repogen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "mockgen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that mockgen provides. A plugin that
// extends a mock requires it.
const Capability plugin.Capability = "mock"

// MockDirective is the directive that puts an interface of the source
// under mockgen.
const MockDirective directive.Name = "mock"

// Word is the word of the family of the mocks. TestTag is the tag that
// routes the family into the test files of a target.
const (
	Word    = "mock"
	TestTag = sdk.TagTest
)

// fileTemplate is the template of each file of the family.
const fileTemplate = "mock.tmpl"

// schema is the directive of mockgen.
var schema = directive.Schema{
	Plugin: string(ID),
	Name:   MockDirective,
	Doc:    "generates a mock of the interface whose methods call function fields",
}

// templates contains the templates of mockgen in one directory for each
// target.
//
//go:embed templates
var templates embed.FS

// data is the data that the file template reads beside the record of a
// mock. Interface is the name of the interface that the mock implements.
type data struct {
	Interface string
}

// New returns mockgen. It writes the mock <I>Mock of each interface under
// [MockDirective] and of each interface that the plan emits, such as a
// repository of repogen. A mock has a function field for each method. Each
// method of the mock calls its field. The mock of a generic interface has
// the type parameters of the interface, and implements the interface at
// those parameters.
//
// The family [Word] claims its files through the template mock.tmpl of the
// plan's target. The tree of the target golang writes the fields as func
// types. The tree of the target typescript writes them as optional arrow
// types. The tag [TestTag] routes the family into the test files. The
// kernel directive skip plugin=mockgen on the origin of an emitted
// interface excludes the interface.
//
// Each call returns a new plugin.
func New() plugin.Generator {
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Requires(repogen.Capability).
		Output(plugin.Output{Tag: TestTag, Per: plugin.PerSource, Word: Word, Template: fileTemplate}).
		For(double.GoTarget, sdk.Templates(tree.Of(templates, double.GoTarget))).
		For(double.TypeScriptTarget, sdk.Templates(tree.Of(templates, double.TypeScriptTarget))).
		Handle(
			sdk.Directive(schema, sdk.OnInterface(source)),
			sdk.OnEmit(symbol.KindInterface, emitted),
		).
		Generator()
}

// source writes the mock of an interface of the source under the
// directive. It writes nothing for an interface without a double.
// [double.Of] reports why such an interface has none.
func source(m *sdk.InterfaceMatch, e *sdk.Emitter) error {
	i := m.Interface
	name := e.JoinName(Word, i.Name)
	methods, mocked := double.Of(m, name, i)
	if !mocked {
		return nil
	}
	mock := &emit.Struct{
		Origin:     i.ID,
		Name:       name,
		Visibility: i.Visibility,
		TypeParams: sdk.MirrorTypeParams(i.TypeParams),
	}
	mock.Implements = []*emit.TypeRef{{Spelling: i.Name, Target: i.ID, Args: sdk.TypeArgs(mock.TypeParams)}}
	mock.Methods.Append(methods...)
	e.File(TestTag).Record(data{Interface: i.Name}, mock)
	return nil
}

// emitted writes the mock of an interface that an earlier plugin emitted.
// The mock has the origin of the interface. It shares the type parameters
// of the interface, which the settle respells in place for both.
func emitted(m *sdk.EmitMatch, e *sdk.Emitter) error {
	i, _ := m.Value.(*emit.Interface)
	name := e.JoinName(Word, i.Name)
	mock := &emit.Struct{
		Origin:     i.Origin,
		Name:       name,
		Visibility: i.Visibility,
		TypeParams: slices.Clone(i.TypeParams),
		Implements: []*emit.TypeRef{{Spelling: i.Name, Args: sdk.TypeArgs(i.TypeParams)}},
	}
	mock.Methods.Append(double.Copy(name, i)...)
	e.File(TestTag).Record(data{Interface: i.Name}, mock)
	return nil
}
