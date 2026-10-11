// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapewriter

import (
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "shapewriter"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// WriterDirective is the directive that marks a struct as a writer. Its
// negated form excludes a struct from the detection.
const WriterDirective directive.Name = "writer"

// KeyWriter is the key of the fact that shapewriter stamps on each writer.
// A plugin that reads the writers selects its subjects through the fact,
// with a named handle.
const KeyWriter meta.KeyName = "shapewriter.writer"

// writeMethod is the name of the method of a writer, and writeResults the
// number of its results.
const (
	writeMethod  = "Write"
	writeResults = 2
)

// schema is the directive of shapewriter.
var schema = directive.Schema{
	Plugin:    string(ID),
	Name:      WriterDirective,
	Negatable: true,
	Doc:       "marks the struct as a writer, and the negated form excludes the struct from the detection",
}

// conclusion is the state of one instance of shapewriter: the handle of its
// key, which the registration sets and the rules stamp through.
//
// # Concurrency
//
// The composition registers the keys before the first phase call, and the
// handlers read the handle afterwards, so the workers of a phase call read
// it concurrently.
type conclusion struct {
	writer meta.Key[bool]
}

// New returns shapewriter, an annotator. It stamps [KeyWriter] on each
// struct under [WriterDirective], and on each struct that declares the
// method Write of a writer. That method takes one parameter of bytes that
// is not variadic, and returns a count of the platform's width and an
// error, as io.Writer declares it. A method that an embedded field promotes does not count. The
// negated directive excludes a struct from the detection. The fact store
// reads the absence of a fact as false, so shapewriter stamps true alone.
// Each call returns a new plugin, because a plugin instance belongs to one
// workspace.
func New() plugin.Annotator {
	c := &conclusion{}
	a, _ := sdk.NewPlugin(ID).
		Version(Version).
		Keys(c.register).
		Handle(
			sdk.Directive(schema, sdk.OnStruct(c.declared)),
			sdk.OnStruct(c.detect),
		).
		Build().(plugin.Annotator)
	return a
}

// register claims the namespace of shapewriter and registers [KeyWriter].
//
// Error modes: the error of the claim, such as a namespace that another
// registrant claimed, and the error of the registration.
func (c *conclusion) register(r *meta.Registry) error {
	if err := r.ClaimNamespace(KeyWriter.Namespace()); err != nil {
		return err
	}
	var err error
	c.writer, err = meta.Register[bool](r, meta.KeySpec{
		Name:  KeyWriter,
		Kinds: []symbol.Kind{symbol.KindStruct},
		Doc:   "marks a struct that writes bytes, as an io.Writer does",
	})
	return err
}

// declared stamps [KeyWriter] on a struct under the directive.
func (c *conclusion) declared(_ *sdk.StructMatch, st *sdk.Stamper) error {
	sdk.Stamp(st, c.writer, true)
	return nil
}

// detect stamps [KeyWriter] on a struct whose declared method Write is the
// method of a writer. The rules of the language of the struct project the
// signature. The one parameter is not variadic and has the form of bytes,
// the first result is a signed integer of the platform's width, and the
// second result has the role of the error.
func (c *conclusion) detect(m *sdk.StructMatch, st *sdk.Stamper) error {
	b := m.Rules()
	for _, method := range m.Struct.Methods {
		if method.Name != writeMethod {
			continue
		}
		call, _ := b.CallableOf(method)
		if len(call.Params) != 1 || call.Params[0].Variadic || len(call.Returns) != writeResults {
			return nil
		}
		bytes, count := b.TypeOf(call.Params[0].Ref), b.TypeOf(call.Returns[0].Ref)
		if bytes.Form == symbol.FormBytes && count.Class == rules.ScalarInt && count.Bits == 0 &&
			call.Returns[1].Role == rules.ReturnError {

			sdk.Stamp(st, c.writer, true)
		}
		return nil
	}
	return nil
}
