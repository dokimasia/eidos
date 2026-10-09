// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapetest

import (
	"slices"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ConsumerID is the name of the generator that [Run.Consume] runs.
const ConsumerID plugin.ID = "consumer"

// firstDirectiveLine is the line of the first directive that
// [Run.Declare] records. The declarations of a fixture are on the lines
// before it, so no directive has the position of a declaration.
const firstDirectiveLine = 1000

// The canonical names of the three directives of the catalog.
var (
	shapeDirective    = directive.Schema{Plugin: string(catalog.ShapeID), Name: catalog.ShapeDirective}.Canonical()
	mixinDirective    = directive.Schema{Plugin: string(catalog.ShapeID), Name: catalog.MixinDirective}.Canonical()
	contractDirective = directive.Schema{Plugin: string(catalog.ShapeID), Name: catalog.ContractDirective}.Canonical()
)

// Instance is one directive instance that a test declares. One of Shape,
// Mixin and Contract is the classification that the instance declares.
type Instance struct {
	Shape    shape.Shape
	Mixin    shape.Mixin
	Contract shape.Contract
	// Role is the role of a contract instance.
	Role shape.Role
	// Params are the validated values of the params of the instance, the
	// id of a contract instance included.
	Params map[directive.ParamKey]directive.Value
}

// Run is a plugintest fixture with the rules of [Lang], the plugins of the
// catalog, and their keys. A Run belongs to one test and is not safe for
// concurrent use.
type Run struct {
	*plugintest.Fixture
	// Annotators are the plugin shape and the plugin shapecheck, whose keys
	// the fixture registers.
	Annotators []plugin.Annotator
	line       int
}

// New returns a run over the packages. It registers the rules of [Lang]
// and the keys of the plugins of the catalog in the fixture, as a
// composition registers them before the first phase.
func New(tb assert.TB, pkgs ...*node.Package) *Run {
	tb.Helper()

	f := plugintest.New(tb).Load(tb, pkgs...)
	f.Rules = rules.NewRegistry()
	assert.NoError(tb, f.Rules.Register(Rules()), "the rules of the test language register")
	r := &Run{Fixture: f, Annotators: catalog.Annotators(), line: firstDirectiveLine}
	for _, a := range r.Annotators {
		keys, provides := a.(plugin.KeyProvider)
		assert.True(tb, provides, "the plugin "+string(a.Name())+" provides its keys")
		assert.NoError(tb, keys.Keys(f.Keys), "the keys of the plugin "+string(a.Name())+" register")
	}
	return r
}

// Declare records the instances on a subject as validation returns them,
// and returns them. Each instance has the canonical name of the directive
// of its form, its classification as the variant, a line of its own in
// [File], and its number among the instances of its directive on the
// subject. An instance of the shape directive overrides the detectors, as
// the schema of the shape directive states. A test declares every
// instance of a subject in one call.
func (r *Run) Declare(tb assert.TB, subject symbol.Identity, instances ...Instance) []directive.Directive {
	tb.Helper()

	out := make([]directive.Directive, 0, len(instances))
	numbers := map[directive.Name]int{}
	for _, in := range instances {
		d := directive.Directive{Role: string(in.Role), Params: in.Params}
		switch {
		case in.Shape != "":
			d.Name, d.Variant, d.Overrides = shapeDirective, string(in.Shape), true
		case in.Mixin != "":
			d.Name, d.Variant = mixinDirective, string(in.Mixin)
		default:
			d.Name, d.Variant = contractDirective, string(in.Contract)
		}
		d.Pos = position.Pos{File: File, Line: r.line}
		r.line++
		d.Instance = numbers[d.Name]
		numbers[d.Name]++
		out = append(out, d)
	}
	r.Validated(tb, subject, out...)
	return out
}

// Classify runs the plugin shape and then the plugin shapecheck over the
// fixture, and returns the findings of both in order.
func (r *Run) Classify(tb assert.TB) []diag.Diag {
	tb.Helper()

	var out []diag.Diag
	for _, a := range r.Annotators {
		res := r.Annotate(tb, a)
		assert.NoError(tb, res.Err, "the plugin "+string(a.Name())+" runs whole")
		out = slices.AppendSeq(out, res.Sink.All())
	}
	return out
}

// Consume runs a generator named [ConsumerID] with the rules over the
// fixture, as a plugin that consumes the catalog runs after it.
func (r *Run) Consume(tb assert.TB, rs ...sdk.Rule) {
	tb.Helper()

	res := r.Generate(tb, sdk.NewPlugin(ConsumerID).Handle(rs...).Build())
	assert.NoError(tb, res.Err, "the consumer runs whole")
}
