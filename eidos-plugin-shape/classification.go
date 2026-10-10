// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape

import (
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The handles of the summary keys that [Of] reads.
var (
	shapeSummary  = meta.Named[string](KeyShape)
	mixedSummary  = meta.Named[bool](KeyMixed)
	memberSummary = meta.Named[bool](KeyMember)
)

// Classification is every classification of one callable.
type Classification struct {
	// Shape is the shape of the callable, and empty where it has none.
	Shape Shape
	// Mixins are the mixins of the callable, in name order.
	Mixins []Mixin
	// Contracts are the roles of the callable in contract instances, in the
	// order of the names of the contracts.
	Contracts []Membership
	// Params are the values of the params and the bindings of each
	// classification that has a value, keyed by the name of the spec.
	Params map[string]Params
}

// Membership is the role of a callable in one instance of a contract.
// The contract, the package of the callable and the id identify the
// instance, and [InstanceOf] returns its members.
type Membership struct {
	Contract Contract
	Role     Role
	// ID is empty where the package has one instance of the contract.
	ID string
}

// Params are the values of the params and the bindings of one
// classification, keyed by the key of the param or the name of the
// binding. The value of a param is a string, an int64 or a
// [symbol.Identity], by the type of the param. The value of a binding is
// the identity of its parameter or return.
type Params map[directive.ParamKey]any

// Of returns every classification of the subject of m. It reads the
// summary keys of the subject first. It then reads the family key and the
// values of the shape whose name [KeyShape] contains, of every mixin where
// [KeyMixed] is true, and of every contract where [KeyMember] is true. A
// meta drop of a classification removes its family key, so Of leaves the
// classification out. Of records each read in the invocation of m, and
// returns the zero Classification for a subject without a classification.
//
// # Allocation contract
//
// Of allocates the classification that it returns. The lists of mixins and
// of memberships allocate as they grow. The map of params, and the map of
// each classification with a value, cost two allocations each, and each
// value of a reference or a binding costs one, because the map boxes it. A
// detected writer with one binding costs five allocations. A read
// allocates nothing once the read set of the invocation has grown.
//
// Of reads the summary keys, and the family key of each spec of a form
// that the summary keys mark, so a callable with a mixin costs one read
// for each mixin spec, and a callable with a contract role one read for
// each contract spec.
func Of(m sdk.Matcher) Classification {
	var c Classification
	specs := catalog()
	name, shaped := sdk.Fact(m, shapeSummary)
	_, mixed := sdk.Fact(m, mixedSummary)
	_, member := sdk.Fact(m, memberSummary)
	for i := range specs {
		s := &specs[i]
		switch {
		case s.Form == FormShape && shaped && s.Name == name:
			if _, held := sdk.Fact(m, meta.Named[bool](s.Key)); held {
				c.Shape = Shape(s.Name)
				c.params(m, s)
			}
		case s.Form == FormMixin && mixed:
			if _, held := sdk.Fact(m, meta.Named[bool](s.Key)); held {
				c.Mixins = append(c.Mixins, Mixin(s.Name))
				c.params(m, s)
			}
		case s.Form == FormContract && member:
			if role, held := sdk.Fact(m, meta.Named[string](s.Key)); held {
				id, _ := sdk.Fact(m, meta.Named[string](s.ID))
				c.Contracts = append(c.Contracts, Membership{Contract: Contract(s.Name), Role: Role(role), ID: id})
				c.params(m, s)
			}
		}
	}
	return c
}

// params reads the values of the params and the bindings of a
// classification on the subject of m into c.Params. A classification
// without a value has no entry in the map.
func (c *Classification) params(m sdk.Matcher, s *Spec) {
	var values Params
	set := func(key directive.ParamKey, v any) {
		if values == nil {
			values = Params{}
		}
		values[key] = v
	}
	for _, b := range s.Bindings {
		if v, held := sdk.Fact(m, meta.Named[symbol.Identity](b.Fact)); held {
			set(b.Key, v)
		}
	}
	// Each param reads under the Go type of its fact, and only a value
	// that the subject has is boxed into the map.
	for _, p := range s.Params {
		switch p.Type {
		case directive.TypeInt:
			if v, held := sdk.Fact(m, meta.Named[int64](p.Fact)); held {
				set(p.Key, v)
			}
		case directive.TypeReference:
			if v, held := sdk.Fact(m, meta.Named[symbol.Identity](p.Fact)); held {
				set(p.Key, v)
			}
		default:
			if v, held := sdk.Fact(m, meta.Named[string](p.Fact)); held {
				set(p.Key, v)
			}
		}
	}
	if values == nil {
		return
	}
	if c.Params == nil {
		c.Params = map[string]Params{}
	}
	c.Params[s.Name] = values
}
