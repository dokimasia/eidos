// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package detectors_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape/detectors"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the types of the fixture package and of the callables of
// the cases.
const (
	valueName  = "Value"
	sourceName = "Source"
	ghostName  = "Ghost"
	getName    = "Get"
	saveName   = "Save"
)

// The identities of the struct Value and the interface Source, which the
// fixture package declares, and of the struct Ghost, which it does not.
var (
	valueID  = symbol.Identity{Lang: shapetest.Lang, Package: shapetest.Path, Name: valueName, Kind: symbol.KindStruct}
	sourceID = symbol.Identity{
		Lang:    shapetest.Lang,
		Package: shapetest.Path,
		Name:    sourceName,
		Kind:    symbol.KindInterface,
	}
	ghostID = symbol.Identity{
		Lang:    shapetest.Lang,
		Package: shapetest.Path,
		Name:    ghostName,
		Kind:    symbol.KindInterface,
	}
)

// The references of the cases. A case reads them and never changes them.
var (
	stringRef   = &node.TypeRef{Spelling: shapetest.String}
	intRef      = &node.TypeRef{Spelling: shapetest.Int}
	boolRef     = &node.TypeRef{Spelling: shapetest.Bool}
	errorRef    = &node.TypeRef{Spelling: shapetest.Error}
	contextRef  = &node.TypeRef{Spelling: shapetest.Context}
	valueRef    = &node.TypeRef{Spelling: valueName, Target: valueID}
	sourceRef   = &node.TypeRef{Spelling: sourceName, Target: sourceID}
	ghostRef    = &node.TypeRef{Spelling: ghostName, Target: ghostID}
	optionalRef = &node.TypeRef{Form: symbol.FormOptional, Elems: []*node.TypeRef{valueRef}}
	listRef     = &node.TypeRef{Form: symbol.FormList, Elems: []*node.TypeRef{valueRef}}
	bytesRef    = &node.TypeRef{Form: symbol.FormList, Elems: []*node.TypeRef{{Spelling: shapetest.Byte}}}
	mapRef      = &node.TypeRef{Form: symbol.FormMap, Elems: []*node.TypeRef{stringRef, valueRef}}
	funcRef     = &node.TypeRef{Form: symbol.FormFunc}
	streamRef   = &node.TypeRef{Form: symbol.FormStream, Elems: []*node.TypeRef{valueRef}}
)

// The inline bodies of the cases: one with a method, one with a field, and
// one without a member.
var (
	inlineInterfaceRef = &node.TypeRef{Form: symbol.FormInline, Methods: []*node.Method{{Name: getName}}}
	inlineRecordRef    = &node.TypeRef{Form: symbol.FormInline, Fields: []*node.Field{{Name: valueName, Type: intRef}}}
	emptyInlineRef     = &node.TypeRef{Form: symbol.FormInline}
)

// The parameters and the returns of the cases.
var (
	key      = rules.ParamView{Ref: stringRef}
	keys     = rules.ParamView{Ref: stringRef, Variadic: true}
	ctx      = rules.ParamView{Ref: contextRef, Role: rules.ParamContext}
	valueIn  = rules.ParamView{Ref: valueRef}
	listIn   = rules.ParamView{Ref: listRef}
	value    = rules.ReturnView{Ref: valueRef}
	number   = rules.ReturnView{Ref: intRef}
	truth    = rules.ReturnView{Ref: boolRef}
	okFlag   = rules.ReturnView{Ref: boolRef, Role: rules.ReturnOkBool}
	failure  = rules.ReturnView{Ref: errorRef, Role: rules.ReturnError}
	stream   = rules.ReturnView{Ref: streamRef, Role: rules.ReturnStream}
	list     = rules.ReturnView{Ref: listRef}
	optional = rules.ReturnView{Ref: optionalRef}
)

// detection is one case of a detector. It has a callable, and whether the
// detector reports it.
type detection struct {
	name string
	give rules.Callable
	want bool
}

// A detector reads the inputs, the values, the context and the error
// model of a callable. These cases state each rule through a detector
// that depends on it.
func TestSignature(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Writer, []detection{
			{
				name: "counts a context parameter as no input",
				give: callable(saveName, []rules.ParamView{ctx, valueIn}, failure),
				want: true,
			},
			{
				name: "counts a thrown error model as a failure",
				give: rules.Callable{Name: saveName, Params: []rules.ParamView{valueIn}, Errors: rules.ErrorsThrown},
				want: true,
			},
		})
	})

	t.Run("Aggregator", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Aggregator, []detection{
			{name: "counts the error return as no value", give: callable(getName, nil, number, failure), want: true},
			{name: "counts the ok flag as no value", give: callable(getName, nil, number, okFlag), want: true},
			{name: "counts a stream return as a value", give: callable(getName, nil, stream), want: true},
		})
	})

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.Reader, []detection{{
			name: "reads the first input after a context parameter as the key",
			give: callable(getName, []rules.ParamView{ctx, listIn}, value, failure),
			want: false,
		}})
	})

	t.Run("BatchReader", func(t *testing.T) {
		t.Parallel()

		detect(t, detectors.BatchReader, []detection{{
			name: "reads the first value after a return that is no value",
			give: rules.Callable{
				Name: getName, Params: []rules.ParamView{keys}, Returns: []rules.ReturnView{okFlag, list, failure},
				Errors: rules.ErrorsLastReturn,
			},
			want: true,
		}})
	})
}

// detect runs each case of a detector as a parallel subtest, over a
// binding of its own.
func detect(t *testing.T, detector func(rules.Callable, rules.Bound) bool, tests []detection) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, detector(tt.give, bound(t)), tt.want,
				"the detector reports the callable exactly where its signature has the shape")
		})
	}
}

// bound returns the rules of the test language, bound to a view of the
// fixture package. The package declares the struct Value and the
// interface Source.
func bound(tb testing.TB) rules.Bound {
	tb.Helper()

	g := store.New()
	pkg := shapetest.Package(
		&node.Struct{ID: valueID, Name: valueName},
		&node.Interface{ID: sourceID, Name: sourceName},
	)
	assert.NoError(tb, g.AddPackage(pkg), "the fixture package loads")
	g.Freeze()
	r, err := g.Reader(store.NewReadSet(), nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	return rules.NewBound(shapetest.Rules(), rules.View{Decls: r}, nil)
}

// callable returns the projection of a callable with the parameters and
// the returns. It has the last-return model where a return has the error
// role, and no error model otherwise.
func callable(name string, params []rules.ParamView, returns ...rules.ReturnView) rules.Callable {
	c := rules.Callable{Name: name, Params: params, Returns: returns}
	if slices.ContainsFunc(returns, func(r rules.ReturnView) bool { return r.Role == rules.ReturnError }) {
		c.Errors = rules.ErrorsLastReturn
	}
	return c
}
