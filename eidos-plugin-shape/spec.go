// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape

import (
	"slices"
	"strconv"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/rules"
)

// Form is the form of a classification. The zero Form is no form.
type Form uint8

const (
	// FormShape is the form of a shape. A callable has at most one shape.
	FormShape Form = 1
	// FormMixin is the form of a mixin. A callable has any number of
	// mixins.
	FormMixin Form = 2
	// FormContract is the form of a contract. A callable has at most one
	// role in an instance of each contract.
	FormContract Form = 3
)

// String returns the spelling of the form in a spec, and the decimal
// number of a form that no constant declares. It allocates nothing for a
// declared form.
func (f Form) String() string {
	switch f {
	case FormShape:
		return "shape"
	case FormMixin:
		return "mixin"
	case FormContract:
		return "contract"
	default:
		return strconv.Itoa(int(f))
	}
}

// Arity is the number of callables that one role of one contract instance
// has. The zero Arity is no arity.
type Arity uint8

const (
	// ArityOne requires exactly one callable.
	ArityOne Arity = 1
	// ArityOptional admits one callable or none.
	ArityOptional Arity = 2
	// ArityMany requires one callable or more.
	ArityMany Arity = 3
	// ArityAny admits any number of callables, none included.
	ArityAny Arity = 4
)

// String returns the spelling of the arity in a spec, and the decimal
// number of an arity that no constant declares. It allocates nothing for a
// declared arity.
func (a Arity) String() string {
	switch a {
	case ArityOne:
		return "one"
	case ArityOptional:
		return "optional"
	case ArityMany:
		return "many"
	case ArityAny:
		return "any"
	default:
		return strconv.Itoa(int(a))
	}
}

// Admits reports whether a role with the arity can have n callables in
// one instance. The zero Arity admits no number. Admits allocates
// nothing.
func (a Arity) Admits(n int) bool {
	switch a {
	case ArityOne:
		return n == 1
	case ArityOptional:
		return n <= 1
	case ArityMany:
		return n >= 1
	case ArityAny:
		return true
	default:
		return false
	}
}

// Source is the list of positions of a callable that a binding counts in.
// The zero Source is no source.
type Source uint8

const (
	// SourceInput counts the parameters with the input role, so a context
	// parameter has no position.
	SourceInput Source = 1
	// SourceResult counts the returns with the value or the stream role, so
	// the error and the ok flag have no position.
	SourceResult Source = 2
)

// String returns the spelling of the source in a spec, and the decimal
// number of a source that no constant declares. It allocates nothing for a
// declared source.
func (s Source) String() string {
	switch s {
	case SourceInput:
		return "input"
	case SourceResult:
		return "result"
	default:
		return strconv.Itoa(int(s))
	}
}

// Spec is the generated description of one classification. It contains
// the name, the form and the claim of the classification, the names of its
// group and its keys, its params, bindings and roles, and its precedence.
type Spec struct {
	// Name is the name of the spec, and the value of the constant that the
	// package declares for it.
	Name string
	Form Form
	// Doc is the claim of the spec on one line.
	Doc string
	// Detected is true for a shape that a detector classifies callables
	// as.
	Detected bool
	// Documentary is true for a mixin or a contract that documents its
	// callables and licenses no check.
	Documentary bool
	// Group is the fact group of the classification. A meta drop of the
	// group removes every key of the classification.
	Group meta.GroupName
	// Key is the family key of the classification. A shape and a mixin
	// stamp true under it, and a contract stamps the role of the callable.
	Key meta.KeyName
	// ID is the key of the id of a contract instance, and empty for a shape
	// and a mixin.
	ID meta.KeyName
	// Params are the params of the variant of the spec, in the order of the
	// spec.
	Params []Param
	// Bindings are the bindings of a shape, in the order of the spec.
	Bindings []Binding
	// Roles are the roles of a contract with their arity, in the order of
	// the spec.
	Roles []RoleSpec
	// YieldsTo are the detected shapes that rank before a detected shape.
	YieldsTo []Shape
}

// Variant returns the variant of the spec in the directive of its form.
// The variant has the name of the spec, the claim as its doc, the
// directive spec of each param, and the roles of a contract. An instance
// of a contract writes one of the roles. Variant allocates the lists of
// params and roles.
func (s Spec) Variant() directive.Variant {
	v := directive.Variant{Name: s.Name, Doc: s.Doc, RolesRequired: len(s.Roles) > 0}
	if len(s.Params) > 0 {
		v.Params = make([]directive.ParamSpec, 0, len(s.Params))
		for _, p := range s.Params {
			v.Params = append(v.Params, p.ParamSpec)
		}
	}
	if len(s.Roles) > 0 {
		v.Roles = make([]string, 0, len(s.Roles))
		for _, r := range s.Roles {
			v.Roles = append(v.Roles, string(r.Name))
		}
	}
	return v
}

// Param is one param of a spec. It embeds the directive spec of the param,
// and adds the key of the param's fact and the checks of an instance that
// the validation of the directive does not run.
type Param struct {
	directive.ParamSpec
	// Fact is the key that the plugin shape stamps the value of the param
	// under. The value is a string, an int64 or the identity of the
	// declaration that a reference resolved to, by the type of the param.
	Fact meta.KeyName
	// Minimum is the least value of an int param, where HasMinimum is true.
	Minimum    int64
	HasMinimum bool
	// Excludes are the params that an instance does not write beside this
	// one.
	Excludes []directive.ParamKey
	// AlsoOn are the callable params whose callables also declare the
	// parameter that this host-param reference resolves to.
	AlsoOn []directive.ParamKey
}

// Binding states the position of the parameter or the return that fills
// one part of a shape. The plugin shape stamps the identity of that
// parameter or return under Fact.
type Binding struct {
	// Key is the name of the binding, and the key of its value in
	// [Params].
	Key  directive.ParamKey
	Fact meta.KeyName
	From Source
	// Index is the position in the list of From, counted from 0.
	Index int
	Doc   string
}

// RoleSpec is one role of a contract and its arity.
type RoleSpec struct {
	Name  Role
	Arity Arity
}

// catalog returns the description of every spec. The first call builds
// the descriptions with [Specs], and every later call returns the same
// slice.
var catalog = sync.OnceValue(Specs)

// SpecOf returns the description of the spec of a name, and false for a
// name that no spec has. The name is a constant of a form, such as
// [Writer], or the name of a spec as a string. The description shares its
// lists with every other call, so a caller does not modify them. SpecOf
// searches the specs in name order, and allocates nothing after the first
// read of the catalog in the process.
func SpecOf[N ~string](name N) (Spec, bool) {
	specs := catalog()
	at, found := slices.BinarySearchFunc(specs, string(name), func(s Spec, n string) int {
		return strings.Compare(s.Name, n)
	})
	if !found {
		return Spec{}, false
	}
	return specs[at], true
}

// Detection is one detected shape and its detector.
type Detection struct {
	Shape Shape
	// Detect reports whether a callable has the shape. It reads only the
	// projection of the callable and the bound rules, so one detector
	// serves every language whose rules project callables.
	Detect func(c rules.Callable, b rules.Bound) bool
}
