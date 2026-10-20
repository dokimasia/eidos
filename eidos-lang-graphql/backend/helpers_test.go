// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"github.com/dokimasia/gqlparser/v2/ast"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of the types that two or more specs give their members.
const (
	stringType = graphql.StringScalar
	intType    = graphql.IntScalar
	idType     = graphql.IDScalar
)

// The names of the declarations and the members that two or more specs
// lower or write.
const (
	rowName   = "Row"
	nodeName  = "Node"
	phaseName = "Phase"
	idField   = "id"
	nameField = "name"
)

// The selection set that the specs give an executable definition.
const selection = "{\n  __typename\n}"

// The markers that two or more specs state: an input object, an extension,
// a query, a subscription, an anonymous operation, a fragment on the type
// Row, the root of queries of a schema, a directive definition that applies
// to the definition of a field and to an object type, the directive marker
// without a location of an extension of a directive, and a repeatable
// directive.
var (
	inputMarker    = symbol.Annotation{Name: string(graphql.InputMarker)}
	extendMarker   = symbol.Annotation{Name: string(graphql.ExtendMarker)}
	queryOperation = symbol.Annotation{
		Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationQuery)},
	}
	subscriptionOperation = symbol.Annotation{
		Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationSubscription)},
	}
	anonymous       = symbol.Annotation{Name: string(graphql.AnonymousMarker)}
	rowFragment     = symbol.Annotation{Name: string(graphql.FragmentMarker), Args: []string{rowName}}
	queryRoot       = symbol.Annotation{Name: string(graphql.QueryMarker), Args: []string{"Root"}}
	cachedLocations = symbol.Annotation{
		Name: string(graphql.DirectiveMarker),
		Args: []string{string(ast.LocationFieldDefinition), string(ast.LocationObject)},
	}
	directiveMarker = symbol.Annotation{Name: string(graphql.DirectiveMarker)}
	repeatable      = symbol.Annotation{Name: string(graphql.RepeatableMarker)}
)

// The applied directives that two or more specs state: one without
// arguments, and the deprecation of a member.
var (
	shareable  = symbol.Annotation{Name: "shareable"}
	deprecated = symbol.Annotation{Name: graphql.DeprecatedDirective}
)

// rowOrigin is the origin of the declarations that the specs of the
// lowering lower, which every declaration that the lowering synthesizes
// takes.
var rowOrigin = symbol.Identity{Lang: graphql.Lang, Package: "shop", Name: rowName, Kind: symbol.KindStruct}

// The types that the specs of the writers give their members. A writer
// reads a type and never writes one, so parallel cases share them. The
// lowering rewrites the forms of a type, so a spec that lowers builds its
// own types.
var (
	stringRef = &emit.TypeRef{Spelling: stringType}
	intRef    = &emit.TypeRef{Spelling: intType}
	idRef     = &emit.TypeRef{Spelling: idType}
)

// allocCall is one call that an allocation test and a benchmark share: the
// method that it calls, which is the name of its benchmark, the case that it
// measures where the method has more than one call, its ceiling, the call,
// and the check of the result of the call. A case whose ceiling only a
// benchmark checks sets bench, which measures the case in place of the
// call, and no list that an allocation test reads contains it.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
	bench    func(b *testing.B)
}

// checkAllocs checks the ceiling of every call in the ordinary run, and the
// result of each call.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		if c.caseName != "" {
			msg = c.name + " for " + c.caseName + " allocates within its ceiling"
		}
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its ceiling:
// one sub-benchmark for each method, in the order of the first call of each
// method, and inside it one for each case of a method with cases.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	var methods []string
	byMethod := map[string][]allocCall{}
	for _, c := range calls {
		if _, seen := byMethod[c.name]; !seen {
			methods = append(methods, c.name)
		}
		byMethod[c.name] = append(byMethod[c.name], c)
	}
	for _, name := range methods {
		cases := byMethod[name]
		b.Run(name, func(b *testing.B) {
			if len(cases) == 1 && cases[0].caseName == "" {
				benchCall(b, cases[0])
				return
			}
			for _, tt := range cases {
				b.Run(tt.caseName, func(b *testing.B) { benchCall(b, tt) })
			}
		})
	}
}

// benchCall measures one call under the bench contract at its ceiling, and
// checks the result of the last call. The contract warms up with one call,
// so the count leaves out what the first call initialises. A case that sets
// bench runs it instead.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	if tt.bench != nil {
		tt.bench(b)
		return
	}
	c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}
