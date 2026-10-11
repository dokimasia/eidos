// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package sentinel

import (
	"embed"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "sentinel"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// KeyValue is the key of the fact that sentinel stamps on each variable
// that the rules of its language name as an error value.
const KeyValue meta.KeyName = "sentinel.value"

// Word is the word of the family of the checks, and TestTag the tag that
// routes the family into the test files of a package.
const (
	Word    = "sentinel"
	TestTag = "test"
)

// PrefixOff is the value of the option prefix that withholds the check of
// the prefix of each message.
const PrefixOff = "off"

// The template of each file of the family, the method that exposes the
// cause of an error, the type of a cause, and the words that the name of
// the test function joins onto the name of the package.
const (
	fileTemplate = "sentinel.test.tmpl"
	unwrapMethod = "Unwrap"
	errorType    = "error"
	testWord     = "Test"
	errorsWord   = "Errors"
)

// templates contains the templates of sentinel in one directory for each
// target.
//
//go:embed templates
var templates embed.FS

// Options are the options of sentinel. Prefix replaces the name of the
// package in the prefix that each message starts with, and [PrefixOff]
// withholds the check of the prefix.
type Options struct {
	Prefix string `opt:"prefix" doc:"the prefix of each message in place of the name of the package, or off to withhold the check"`
}

// Subject is the data that the test template reads beside a record. Test is
// the name of the test function of the package, and Prefix is the option
// prefix. Checked reports that the file checks the prefix of each message.
// Type reports that the record is an error type, and a sentinel otherwise.
// Cause is the field of an error type that its method Unwrap returns, and
// empty for an error type without one.
type Subject struct {
	Test    string
	Prefix  string
	Checked bool
	Type    bool
	Cause   string
}

// conclusion is the state of one instance of sentinel: its options, and the
// handle of its key, which the registration sets and the annotator stamps
// through.
//
// # Concurrency
//
// The composition sets the options and registers the keys before the first
// phase call, and the handlers read them afterwards, so the workers of a
// phase call read them concurrently.
type conclusion struct {
	options *Options
	value   meta.Key[bool]
}

// New returns sentinel. Its annotator stamps [KeyValue] on each variable
// whose name the rules.ErrorValueRules of its language accept, such as
// ErrNotFound in Go. Its generator selects the variables with the fact, and
// the structs that the Go satellite stamps as errors, so its handlers do
// not filter their subjects. It records each into the test file of the
// family [Word] of its package. The template sentinel.test.tmpl claims the
// file.
//
// The file checks that each sentinel has a message that starts with the
// prefix of the package, that each message differs from every other and
// does not start with another, and that no sentinel matches another under
// errors.Is. It checks that each error type reports a message for its zero
// value, and that an error type with a cause exposes the cause that it was
// given. The prefix is the name of the package and a colon, or the option
// prefix in place of the name. A generic error type has no checks.
//
// The plugin serves the target golang alone. Each call returns a new
// plugin, because a plugin instance belongs to one workspace.
func New() plugin.Generator {
	c := &conclusion{options: &Options{}}
	g, _ := sdk.NewPlugin(ID).
		Version(Version).
		Options(c.options).
		Keys(c.register).
		Output(plugin.Output{Tag: TestTag, Per: plugin.PerPackage, Word: Word, Template: fileTemplate}).
		For(golang.Target, sdk.Templates(tree.Of(templates, golang.Target))).
		Handle(
			sdk.OnVariable(c.stamp),
			sdk.Where(sdk.HasKey(meta.Named[bool](KeyValue)), sdk.OnVariable(c.sentinel)),
			sdk.Where(sdk.HasKey(meta.Named[bool](golang.SatisfiesErrorKey)), sdk.OnStruct(c.errorType)),
		).
		Build().(plugin.Generator)
	return g
}

// register claims the namespace of sentinel and registers [KeyValue].
//
// Error modes: the error of the claim, such as a namespace that another
// registrant claimed, and the error of the registration.
func (c *conclusion) register(r *meta.Registry) error {
	if err := r.ClaimNamespace(KeyValue.Namespace()); err != nil {
		return err
	}
	var err error
	c.value, err = meta.Register[bool](r, meta.KeySpec{
		Name:  KeyValue,
		Kinds: []symbol.Kind{symbol.KindVariable},
		Doc:   "marks a variable that the rules of its language name as an error value",
	})
	return err
}

// stamp stamps [KeyValue] on a variable whose name the ErrorValueRules of
// its language accept. A language whose rules do not implement
// rules.ErrorValueRules has no sentinels.
func (c *conclusion) stamp(m *sdk.VariableMatch, st *sdk.Stamper) error {
	names, named := m.Rules().Source().(rules.ErrorValueRules)
	if named && names.IsSentinelName(m.Variable.Name) {
		sdk.Stamp(st, c.value, true)
	}
	return nil
}

// sentinel records a variable with the fact into the test file of its
// package.
func (c *conclusion) sentinel(m *sdk.VariableMatch, e *sdk.Emitter) error {
	v := m.Variable
	e.PackageFile(TestTag).
		Record(c.subject(e, v.ID), &emit.Variable{Origin: v.ID, Name: v.Name, Visibility: v.Visibility})
	return nil
}

// errorType records a struct that the Go satellite stamps as an error into
// the test file of its package. The cause of the struct is its first field
// of the type error, where the struct declares the method Unwrap that
// returns one error. A generic struct has no record, because the test cannot
// make a value of it without its type arguments.
func (c *conclusion) errorType(m *sdk.StructMatch, e *sdk.Emitter) error {
	s := m.Struct
	if len(s.TypeParams) > 0 {
		return nil
	}
	data := c.subject(e, s.ID)
	data.Type = true
	if unwraps(s) {
		for _, f := range s.Fields {
			if f.Type.Spelling == errorType {
				data.Cause = f.Name
				break
			}
		}
	}
	e.PackageFile(TestTag).Record(data, &emit.Struct{Origin: s.ID, Name: s.Name, Visibility: s.Visibility})
	return nil
}

// subject returns the data of a record of the declaration id, before the
// handler of its kind completes it. The name of the test function joins the
// name that the import path of the package assumes.
func (c *conclusion) subject(e *sdk.Emitter, id symbol.Identity) Subject {
	return Subject{
		Test:    e.JoinName(errorsWord, e.JoinName(golang.AssumedName(id.Package), testWord)),
		Prefix:  c.options.Prefix,
		Checked: c.options.Prefix != PrefixOff,
	}
}

// unwraps reports whether s declares the method Unwrap that takes nothing
// and returns one error.
func unwraps(s *node.Struct) bool {
	for _, m := range s.Methods {
		if m.Name == unwrapMethod && len(m.Params) == 0 && len(m.Returns) == 1 &&
			m.Returns[0].Type.Spelling == errorType {

			return true
		}
	}
	return false
}
