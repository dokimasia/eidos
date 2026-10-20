// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The rpc of the cases: the language and the package of its schema, the
// name of its service, its name, its documentation, the names of its
// parameter and its return, and the spelling of the failure that the spoke
// of the cases writes.
const (
	rpcLang         = "protobuf"
	rpcPackage      = "svc"
	serviceName     = "Store"
	rpcName         = "Get"
	rpcDoc          = "Get returns the session of a key."
	rpcParamName    = "key"
	rpcReturnName   = "session"
	failureSpelling = "Error"
)

// The references of the rpc of the cases, and the reason of a refusal of the
// spoke or of the translation of the cases.
var (
	rpcKey     = &node.TypeRef{Spelling: "string"}
	rpcSession = &node.TypeRef{Spelling: "Session"}
	errRefused = errors.New("plugin_test: the spoke has no spelling")
)

// rpcCase is the rpc of the cases: an asynchronous rpc of a service with
// one parameter and one return, under the error model that raises a
// failure.
var rpcCase = plugin.RPC{
	ID: symbol.Identity{
		Lang: rpcLang, Package: rpcPackage, Owner: serviceName, Name: rpcName, Kind: symbol.KindMethod,
	},
	Service:    symbol.Identity{Lang: rpcLang, Package: rpcPackage, Name: serviceName, Kind: symbol.KindInterface},
	Doc:        []string{rpcDoc},
	Visibility: symbol.VisibilityPublic,
	Callable: rules.Callable{
		Name: rpcName, Async: true, Errors: rules.ErrorsRaised,
		Params:  []rules.ParamView{{Name: rpcParamName, Ref: rpcKey}},
		Returns: []rules.ReturnView{{Name: rpcReturnName, Ref: rpcSession}},
	},
}

// spelledAs translates each reference as a reference with its spelling.
var spelledAs plugin.Translate = func(ref *node.TypeRef) (*emit.TypeRef, error) {
	return &emit.TypeRef{Spelling: ref.Spelling}, nil
}

// failureSpoke spells every shape as failureSpelling, and returns err where
// err is set.
type failureSpoke struct{ err error }

var _ plugin.TypeSpeller = failureSpoke{}

// SpellType returns the reference of failureSpelling, or err.
func (s failureSpoke) SpellType(rules.TypeShape, plugin.Policy) (*emit.TypeRef, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &emit.TypeRef{Spelling: failureSpelling}, nil
}

// The canonical handler of an rpc is the projection of the rpc in the
// spelling of the plan's target, with the failure that the target spells.
func TestBinder(t *testing.T) {
	t.Parallel()

	t.Run("CanonicalHandler", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a method with the origin, the documentation and the name of the rpc", func(t *testing.T) {
			t.Parallel()

			got, err := plugin.CanonicalHandler(rpcCase, plugin.Policy{}, failureSpoke{}, spelledAs)
			assert.NoError(t, err, "the rpc has a canonical handler")
			expect.Equal(t, got.Origin, rpcCase.ID, "the method derives from the rpc")
			expect.Equal(t, got.Doc, []string{rpcDoc}, "the method has the documentation of the rpc")
			expect.Equal(t, got.Name, rpcName, "the method has the name of the rpc")
			expect.Equal(t, got.Visibility, symbol.VisibilityPublic, "the method has the visibility of the rpc")
		})

		t.Run("returns a parameter for each parameter of the projection", func(t *testing.T) {
			t.Parallel()

			got, err := plugin.CanonicalHandler(rpcCase, plugin.Policy{}, failureSpoke{}, spelledAs)
			assert.NoError(t, err, "the rpc has a canonical handler")
			want := []*emit.Param{{Name: rpcParamName, Type: &emit.TypeRef{Spelling: rpcKey.Spelling}}}
			assert.Equal(t, got.Params.Items(), want,
				"the method takes the parameter of the projection in the spelling of the translation")
		})

		t.Run("returns a return for each return of the projection", func(t *testing.T) {
			t.Parallel()

			got, err := plugin.CanonicalHandler(rpcCase, plugin.Policy{}, failureSpoke{}, spelledAs)
			assert.NoError(t, err, "the rpc has a canonical handler")
			want := []*emit.Return{{Name: rpcReturnName, Type: &emit.TypeRef{Spelling: rpcSession.Spelling}}}
			assert.Equal(t, got.Returns.Items(), want,
				"the method returns the return of the projection in the spelling of the translation")
		})

		t.Run("returns the asynchrony of the projection", func(t *testing.T) {
			t.Parallel()

			got, err := plugin.CanonicalHandler(rpcCase, plugin.Policy{}, failureSpoke{}, spelledAs)
			assert.NoError(t, err, "the rpc has a canonical handler")
			assert.True(t, got.Async, "the method of an asynchronous rpc is asynchronous")
		})

		t.Run("states the failure that the spoke spells under ErrorsRaised", func(t *testing.T) {
			t.Parallel()

			got, err := plugin.CanonicalHandler(rpcCase, plugin.Policy{}, failureSpoke{}, spelledAs)
			assert.NoError(t, err, "the rpc has a canonical handler")
			assert.Equal(t, got.Throws, []*emit.TypeRef{{Spelling: failureSpelling}},
				"the method states the well-known error in the spelling of the spoke")
		})

		t.Run("states no failure under another error model", func(t *testing.T) {
			t.Parallel()

			last := rpcCase
			last.Callable.Errors = rules.ErrorsLastReturn
			got, err := plugin.CanonicalHandler(last, plugin.Policy{}, failureSpoke{err: errRefused}, spelledAs)
			assert.NoError(t, err, "the method does not spell a failure")
			assert.Nil(t, got.Throws, "the returns of the projection state the failure")
		})

		t.Run("returns the error of the spoke for the failure", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.CanonicalHandler(rpcCase, plugin.Policy{}, failureSpoke{err: errRefused}, spelledAs)
			assert.ErrorIs(t, err, errRefused, "the error is the refusal of the spoke")
		})

		tests := []struct {
			name string
			give *node.TypeRef
		}{
			{name: "returns the error of translate for a parameter", give: rpcKey},
			{name: "returns the error of translate for a return", give: rpcSession},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				refusing := plugin.Translate(func(ref *node.TypeRef) (*emit.TypeRef, error) {
					if ref == tt.give {
						return nil, errRefused
					}
					return spelledAs(ref)
				})
				_, err := plugin.CanonicalHandler(rpcCase, plugin.Policy{}, failureSpoke{}, refusing)
				assert.ErrorIs(t, err, errRefused, "the error is the refusal of the translation")
			})
		}
	})
}
