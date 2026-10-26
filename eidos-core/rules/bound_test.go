// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The package of the fixture and the names that it declares. Every spec
// of the package builds on them.
const (
	svcPath     = "svc"
	depPath     = "dep"
	rowName     = "Row"
	baseName    = "Base"
	derivedName = "Derived"
	storeName   = "Store"
	intSpelling = "int"
	strSpelling = "string"
	// nameField is the string field of Row, and countField is its int
	// field.
	nameField  = "name"
	countField = "count"
	// keyName is a defined type that the package of a fixture declares
	// methods for, and keyedName is the struct that embeds it.
	keyName   = "Key"
	keyedName = "Keyed"
)

// promotedMethods is the number of methods that the base of [promoted]
// declares, and mixinName is the second base of [linearised], which
// declares the same methods.
const (
	promotedMethods = 20
	mixinName       = "Mixin"
)

// presentField is the field for which [presenceLang] reports presence.
// dynamicSpelling is the builtin that it folds to the top type.
const (
	presentField    = "present"
	dynamicSpelling = "any"
)

// tagKey is the key of the tag cases, and sentinelPrefix the prefix of
// the error values of [capable].
const (
	tagKey         = "json"
	sentinelPrefix = "Err"
)

// taggedField is a field with a tag, which [capable] reads under every
// key.
var taggedField = &node.Field{Name: nameField, Tag: `json:"name"`}

// The allocations of the methods of a binding over the walk fixture.
// TestBoundAllocs checks them in the ordinary run, and BenchmarkBound in a
// benchmark run. A method without a constant allocates nothing.
const (
	// newBoundAllocs is the memo of folded shapes.
	newBoundAllocs = 1
	// callableAllocs is the projection of a method with one parameter
	// and one return. It allocates the parameter list, the return list,
	// and the list of return roles that the scripted language classifies.
	callableAllocs = 3
	// foldAllocs is the first fold of a builtin reference. It allocates
	// the first group of the memo, and the shape that the memo stores out
	// of line, because a TypeShape is larger than what the map stores in
	// place.
	foldAllocs = 2
	// foldBytesAllocs is the first fold of a list of bytes. It allocates
	// the first group of the memo and the shapes of the element and of the
	// list. Bytes has no list of children.
	foldBytesAllocs = 3
	// membersAllocs is a member walk of a struct that embeds one type, a
	// struct or a defined type. It allocates the members that it returns
	// and the path of the one contributor that it descends into. The
	// working storage of the walk comes from a pool, and the methods of a
	// defined type are the declarations of its package.
	membersAllocs = 2
	// linearisedAllocs is a member walk under ShadowLinearise of a struct
	// that extends two types with the same methods. It allocates the
	// members that it returns and the paths of the two contributors. The
	// bases and the linearizations of the walk come from the pool.
	linearisedAllocs = 3
	// fieldTypeOfAllocs is the list of the one child of the optional that
	// wraps the type of a field with presence.
	fieldTypeOfAllocs = 1
	// witnessesAllocs is the list of the witness of one parameter, and the
	// reference that the scripted language derives.
	witnessesAllocs = 2
	// sentinelNameAllocs is the name that [capable] joins.
	sentinelNameAllocs = 1
)

// native is the scripted rules under the language of the fixture, so a
// contributor in the fixture walks under the scripted policy. A foreign
// language has the absent policy. native keeps the scripted generics
// capability.
type native struct {
	rules.SourceRules
	rules.GenericsRules
}

// Lang returns the language of the fixture.
func (native) Lang() symbol.Lang { return coretest.Lang }

// policy overrides the member policy of the scripted language, so that a
// case exercises one shadowing rule or one contribution list.
type policy struct {
	rules.SourceRules
	members rules.MemberPolicy
}

// Members returns the overriding policy.
func (p policy) Members() rules.MemberPolicy { return p.members }

// fixedSamples overrides the samples of the scripted language with one
// fixed pair, so that a case controls what the language derives.
type fixedSamples struct {
	rules.SourceRules
	sample, alternate rules.Sample
}

// DeriveSamples returns the fixed pair.
func (f fixedSamples) DeriveSamples(*node.TypeRef, string, rules.View) (rules.Sample, rules.Sample) {
	return f.sample, f.alternate
}

// capable is the scripted rules with tags, error values and error types.
// Its tag is the whole text of the tag of a field under every key, the
// name of its error values has the prefix [sentinelPrefix], and Row is its
// one error type.
type capable struct {
	rules.SourceRules
}

// Tag returns the text of the tag of f, and reports false for a field
// without a tag.
func (capable) Tag(f *node.Field, _ string) (string, bool) { return f.Tag, f.Tag != "" }

// SentinelName returns base after the prefix.
func (capable) SentinelName(base string) string { return sentinelPrefix + base }

// SentinelBase returns ident without the prefix, and reports false for an
// identifier without it.
func (capable) SentinelBase(ident string) (string, bool) {
	return strings.CutPrefix(ident, sentinelPrefix)
}

// ErrorContract returns the contract of Row, an error behind a pointer
// with the field name as its cause, and reports false for every other
// declaration.
func (capable) ErrorContract(decl symbol.Symbol, _ rules.View) (rules.ErrorContract, bool) {
	s, isStruct := decl.(*node.Struct)
	if !isStruct || s.Name != rowName {
		return rules.ErrorContract{}, false
	}
	return rules.ErrorContract{Cause: s.Fields[0], Pointer: true}, true
}

// errClosed is the reason of the import rule of [capable].
var errClosed = errors.New("rules_test: the code refers to the declarations of its own package alone")

// Importable returns errClosed for a reference to a declaration of another
// package than pkg, and nil for every other reference.
func (capable) Importable(pkg symbol.Identity, ref *node.TypeRef) error {
	if ref.Target.Package != pkg.Package {
		return errClosed
	}
	return nil
}

// ImportableValue returns errClosed for a value whose callee is a
// declaration of another package than pkg, and nil for every other value.
func (capable) ImportableValue(pkg symbol.Identity, v emit.Value) error {
	if v.Callee.Package != pkg.Package {
		return errClosed
	}
	return nil
}

// otherLang is the scripted rules under the name of a language in which
// the fixture declares nothing. A case reads the declarations of the
// fixture through the rules of another language.
type otherLang struct {
	rules.SourceRules
}

// Lang returns the other language.
func (otherLang) Lang() symbol.Lang { return "other" }

// nongeneric hides the generics capability of the scripted language. It
// forwards every method of the contract by name, because an embedding of
// the rules would promote the capability too.
type nongeneric struct {
	inner rules.SourceRules
}

// Lang returns the language of the inner rules.
func (n nongeneric) Lang() symbol.Lang { return n.inner.Lang() }

// Members returns the member policy of the inner rules.
func (n nongeneric) Members() rules.MemberPolicy { return n.inner.Members() }

// ParamRole returns the role that the inner rules give a parameter.
func (n nongeneric) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	return n.inner.ParamRole(p, v)
}

// ReturnRoles returns the roles that the inner rules give the returns.
func (n nongeneric) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return n.inner.ReturnRoles(rs, v)
}

// Builtin returns the shape that the inner rules fold for a spelling.
func (n nongeneric) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	return n.inner.Builtin(ref, v)
}

// Resolve returns the declaration to which the inner rules resolve a
// name.
func (n nongeneric) Resolve(
	s rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	return n.inner.Resolve(s, name, kind, v)
}

// DeriveSamples returns the pair that the inner rules derive for a type.
func (n nongeneric) DeriveSamples(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return n.inner.DeriveSamples(ref, hint, v)
}

// ZeroValue returns the zero value that the inner rules derive for a
// type.
func (n nongeneric) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return n.inner.ZeroValue(ref, v)
}

// LiteralFor returns the value that the inner rules derive for a text.
func (n nongeneric) LiteralFor(s rules.Scope, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	return n.inner.LiteralFor(s, ref, text, v)
}

// presenceLang is the scripted rules with presence beyond the type. The
// field named [presentField] has presence, and [dynamicSpelling] folds to
// the top type.
type presenceLang struct {
	rules.SourceRules
}

// Builtin folds dynamicSpelling to the top type, and every other spelling
// as the scripted language folds it.
func (p presenceLang) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	if ref.Spelling == dynamicSpelling {
		return rules.Leaf(symbol.FormDynamic, ref.Spelling)
	}
	return p.SourceRules.Builtin(ref, v)
}

// Presence reports presence for the field named presentField.
func (presenceLang) Presence(f *node.Field, _ rules.View) bool { return f.Name == presentField }

// allocCall is one call of a function or a method. name is the method,
// which is also the name of its benchmark. caseName is the case that the
// call measures where the method has more than one call. allocs is the
// number of allocations of the call. check checks what the last call
// returned, after the count.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
}

// TestBound pins the defaults of a binding and the calls that it passes
// to the language. A handler calls the methods of the binding.
func TestBound(t *testing.T) {
	t.Parallel()

	t.Run("NewBound", func(t *testing.T) {
		t.Parallel()

		t.Run("binds the absent rules for nil rules", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(nil, rules.View{}, nil)
			assert.True(t, rules.IsAbsent(b.Source()), "a nil source binds the absent rules")
			assert.Equal(t, b.Lang(), symbol.Lang(""), "for the zero language")
		})
	})

	t.Run("Source", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the rules of the binding", func(t *testing.T) {
			t.Parallel()

			source := rulestest.Scripted()
			assert.Equal(t, rules.NewBound(source, rules.View{}, nil).Source(), source, "the rules of the binding",
				assert.ByIdentity())
		})
	})

	t.Run("View", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the view of the binding", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.False(t, b.View().IsZero(), "the view of the fixture is not the zero view")
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the language of the rules", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Equal(t, b.Lang(), coretest.Lang, "the language of the fixture")
		})
	})

	t.Run("FieldTypeOf", func(t *testing.T) {
		t.Parallel()

		scalar := rules.Scalar(intSpelling, rules.ScalarInt, 0)
		optional := rules.TypeShape{Form: symbol.FormOptional, Spelling: intSpelling, Elems: []rules.TypeShape{scalar}}
		optionalRef := &node.TypeRef{
			Spelling: "int?", Form: symbol.FormOptional, Elems: []*node.TypeRef{builtin(intSpelling)},
		}
		marked := func(typ *node.TypeRef) *node.Field {
			f := field(svcPath, rowName, countField, typ)
			f.Optional = true
			return f
		}
		tests := []struct {
			name   string
			source rules.SourceRules
			give   *node.Field
			want   rules.TypeShape
		}{
			{
				name:   "returns the shape of the type of a field without presence",
				source: presenceLang{scripted()},
				give:   field(svcPath, rowName, countField, builtin(intSpelling)),
				want:   scalar,
			},
			{
				name:   "wraps the type of a field with the optional mark in an optional",
				source: presenceLang{scripted()},
				give:   marked(builtin(intSpelling)),
				want:   optional,
			},
			{
				name:   "wraps the type of a field with presence under the rule of the language in an optional",
				source: presenceLang{scripted()},
				give:   field(svcPath, rowName, presentField, builtin(intSpelling)),
				want:   optional,
			},
			{
				name:   "returns an optional type unchanged",
				source: presenceLang{scripted()},
				give:   marked(optionalRef),
				want: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: optionalRef.Spelling, Elems: []rules.TypeShape{scalar},
				},
			},
			{
				name:   "returns the top type unchanged",
				source: presenceLang{scripted()},
				give:   marked(builtin(dynamicSpelling)),
				want:   rules.Leaf(symbol.FormDynamic, dynamicSpelling),
			},
			{
				name:   "reads presence from the optional mark alone in a language without the rule",
				source: scripted(),
				give:   field(svcPath, rowName, presentField, builtin(intSpelling)),
				want:   scalar,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := rules.NewBound(tt.source, viewOnly(t), nil).FieldTypeOf(tt.give)
				assert.Equal(t, got, tt.want, "the shape of the type of the field with its presence")
			})
		}

		t.Run("folds a nil field to Opaque", func(t *testing.T) {
			t.Parallel()

			got := rules.NewBound(presenceLang{scripted()}, viewOnly(t), nil).FieldTypeOf(nil)
			assert.Equal(t, got, rules.Opaque(nil), "a nil field has no type to fold")
		})
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the zero value of a type under the language", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			zero, held := b.ZeroValue(builtin(intSpelling))
			assert.True(t, held, "a zero derives")
			assert.Equal(t, zero, emit.Literal(emit.LiteralInt, "0"), "as the language spells it")
		})

		t.Run("reports false for the zero view", func(t *testing.T) {
			t.Parallel()

			_, held := rules.NewBound(rulestest.Scripted(), rules.View{}, nil).ZeroValue(builtin(intSpelling))
			assert.False(t, held, "the zero view derives no zero value")
		})

		t.Run("reports false for a nil reference", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			_, held := b.ZeroValue(nil)
			assert.False(t, held, "a nil reference has no zero value")
		})
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the value of a text under the language", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			lit, held := b.LiteralFor(rules.Scope{}, builtin(strSpelling), "x")
			assert.True(t, held, "a literal derives")
			assert.Equal(t, lit, emit.Literal(emit.LiteralString, "x"), "as text")
		})

		t.Run("passes the scope to the language", func(t *testing.T) {
			t.Parallel()

			v := viewOnly(t)
			scope := v.ScopeOf(rowField(nameField))
			lit, held := rules.NewBound(scoped{scripted()}, v, nil).
				LiteralFor(scope, named(svcPath, rowName, symbol.KindStruct), "x")
			assert.True(t, held, "the language reads the text in the scope")
			assert.Equal(t, lit, emit.Name(scope.Subject), "the language receives the scope of the caller")
		})

		t.Run("reports false for the zero view", func(t *testing.T) {
			t.Parallel()

			_, held := rules.NewBound(rulestest.Scripted(), rules.View{}, nil).
				LiteralFor(rules.Scope{}, builtin(strSpelling), "x")
			assert.False(t, held, "the zero view derives no literal")
		})
	})

	t.Run("Tag", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tag of the language under the key", func(t *testing.T) {
			t.Parallel()

			got, held := rules.NewBound(capable{scripted()}, rules.View{}, nil).Tag(taggedField, tagKey)
			assert.True(t, held, "the field has a tag")
			assert.Equal(t, got, taggedField.Tag, "Tag returns the value that the language reads")
		})

		t.Run("reports false for a nil field", func(t *testing.T) {
			t.Parallel()

			_, held := rules.NewBound(capable{scripted()}, rules.View{}, nil).Tag(nil, tagKey)
			assert.False(t, held, "a nil field has no tag")
		})

		t.Run("reports false for a language without tags", func(t *testing.T) {
			t.Parallel()

			_, held := rules.NewBound(scripted(), rules.View{}, nil).Tag(taggedField, tagKey)
			assert.False(t, held, "the scripted language does not implement TagRules")
		})
	})

	t.Run("SentinelName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name of the error value of the language", func(t *testing.T) {
			t.Parallel()

			got, held := rules.NewBound(capable{scripted()}, rules.View{}, nil).SentinelName(rowName)
			assert.True(t, held, "the language has error values")
			assert.Equal(t, got, sentinelPrefix+rowName, "SentinelName returns the name of the language")
		})

		t.Run("reports false for a language without error values", func(t *testing.T) {
			t.Parallel()

			_, held := rules.NewBound(scripted(), rules.View{}, nil).SentinelName(rowName)
			assert.False(t, held, "the scripted language does not implement ErrorValueRules")
		})
	})

	t.Run("SentinelBase", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the base of a name that follows the convention of the language", func(t *testing.T) {
			t.Parallel()

			able := rules.NewBound(capable{scripted()}, rules.View{}, nil)
			base, sentinel := able.SentinelBase(sentinelPrefix + rowName)
			assert.True(t, sentinel, "the name has the prefix of an error value")
			assert.Equal(t, base, rowName, "SentinelBase returns the name without the prefix")
		})

		t.Run("reports false for a name that does not follow the convention", func(t *testing.T) {
			t.Parallel()

			_, sentinel := rules.NewBound(capable{scripted()}, rules.View{}, nil).SentinelBase(rowName)
			assert.False(t, sentinel, "the name does not have the prefix")
		})

		t.Run("reports false for a language without error values", func(t *testing.T) {
			t.Parallel()

			_, sentinel := rules.NewBound(scripted(), rules.View{}, nil).SentinelBase(sentinelPrefix + rowName)
			assert.False(t, sentinel, "the scripted language does not implement ErrorValueRules")
		})
	})

	t.Run("Importable", func(t *testing.T) {
		t.Parallel()

		row := named(svcPath, rowName, symbol.KindStruct)

		t.Run("returns the reason of the language for a declaration that the code cannot refer to", func(t *testing.T) {
			t.Parallel()

			err := rules.NewBound(capable{scripted()}, rules.View{}, nil).Importable(coretest.PackageID("web"), row)
			assert.ErrorIs(t, err, errClosed, "the language refuses the declaration of another package")
		})

		t.Run("returns nil for a declaration that the code can refer to", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, rules.NewBound(capable{scripted()}, rules.View{}, nil).
				Importable(coretest.PackageID(svcPath), row), "the code refers to a declaration of its own package")
		})

		t.Run("returns nil for a nil reference", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, rules.NewBound(capable{scripted()}, rules.View{}, nil).
				Importable(coretest.PackageID("web"), nil), "a nil reference names no declaration")
		})

		t.Run("returns nil for a language without import rules", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, rules.NewBound(scripted(), rules.View{}, nil).Importable(coretest.PackageID("web"), row),
				"the scripted language does not implement ImportRules")
		})
	})

	t.Run("ImportableValue", func(t *testing.T) {
		t.Parallel()

		call := emit.Call(coretest.ID(svcPath, rowName, symbol.KindFunction))

		t.Run("returns the reason of the language for a value that the code cannot refer to", func(t *testing.T) {
			t.Parallel()

			err := rules.NewBound(capable{scripted()}, rules.View{}, nil).
				ImportableValue(coretest.PackageID("web"), call)
			assert.ErrorIs(t, err, errClosed, "the language refuses the call of a function of another package")
		})

		t.Run("returns nil for a value that the code can refer to", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, rules.NewBound(capable{scripted()}, rules.View{}, nil).
				ImportableValue(coretest.PackageID(svcPath), call), "the code calls a function of its own package")
		})

		t.Run("returns nil for a language without value import rules", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, rules.NewBound(scripted(), rules.View{}, nil).
				ImportableValue(coretest.PackageID("web"), call),
				"the scripted language does not implement ValueImportRules")
		})
	})

	t.Run("ErrorContract", func(t *testing.T) {
		t.Parallel()

		g := coretest.Frozen(t, hierarchy())
		row, _ := g.Lookup(coretest.ID(svcPath, rowName, symbol.KindStruct))

		t.Run("returns the contract that the language of the declaration states", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, g)
			c, isError := rules.NewBound(capable{scripted()}, v, nil).ErrorContract(row)
			assert.True(t, isError, "the language takes Row as an error type")
			assert.True(t, c.Pointer, "a pointer to Row is the error")
			assert.Equal(t, c.Cause.Name, nameField, "the field name is the cause")
		})

		t.Run("reports false for a declaration that is not an error type", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, g)
			store, _ := g.Lookup(coretest.ID(svcPath, storeName, symbol.KindInterface))
			_, isError := rules.NewBound(capable{scripted()}, v, nil).ErrorContract(store)
			assert.False(t, isError, "the language does not take Store as an error type")
		})

		t.Run("reports false for a symbol that is not a declaration", func(t *testing.T) {
			t.Parallel()

			_, isError := rules.NewBound(capable{scripted()}, viewOnly(t), nil).ErrorContract(nil)
			assert.False(t, isError, "a nil symbol is not a declaration")
		})

		t.Run("reports false for a language without error types", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, g)
			_, isError := rules.NewBound(scripted(), v, nil).ErrorContract(row)
			assert.False(t, isError, "the scripted language does not implement ErrorTypeRules")
		})
	})
}

// TestBoundAllocs checks in the ordinary run that each method of the
// binding allocates only what it returns or what the language allocates.
// A first fold takes a binding that the setup builds outside the count,
// because a fold fills the memo that it reads. The test does not run in
// parallel, because the count includes the allocations of every
// goroutine.
func TestBoundAllocs(t *testing.T) {
	checkCalls(t, boundCalls(t))

	folded, ref := foldedBinding(t)
	var shape rules.TypeShape
	assert.MaxAllocs(t, func() { shape = folded.TypeOf(ref) }, 0,
		"TypeOf allocates nothing for a reference that the binding folded")
	assert.Equal(t, shape.Form, symbol.FormScalar, "TypeOf returns the folded scalar")

	v, bytes := viewOnly(t), byteList()
	assert.MaxAllocsWithSetup(t, func() rules.Bound { return rules.NewBound(scripted(), v, nil) },
		func(b rules.Bound) { shape = b.TypeOf(ref) },
		foldAllocs, "TypeOf allocates the group of the memo and the shape for a first fold")
	assert.Equal(t, shape.Form, symbol.FormScalar, "TypeOf folds int to a scalar")

	assert.MaxAllocsWithSetup(t, func() rules.Bound { return rules.NewBound(bytesLang{scripted()}, v, nil) },
		func(b rules.Bound) { shape = b.TypeOf(bytes) },
		foldBytesAllocs, "TypeOf allocates the group of the memo and two shapes for a first fold of a list of bytes")
	assert.Equal(t, shape.Form, symbol.FormBytes, "TypeOf folds a list of bytes to Bytes")
}

// checkCalls checks the ceiling of every call under [assert.MaxAllocs],
// and then what the last run of the call returned.
func checkCalls(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, tt := range calls {
		msg := tt.name + " allocates what it returns"
		if tt.caseName != "" {
			msg = tt.name + " for " + tt.caseName + " allocates what it returns"
		}
		assert.MaxAllocs(t, tt.call, tt.allocs, msg)
		tt.check(t)
	}
}

// BenchmarkBound measures each method of a binding over the walk
// fixture, which a handler calls once per match, and the first fold of a
// reference.
func BenchmarkBound(b *testing.B) {
	benchCalls(b, boundCalls(b))

	b.Run("TypeOf", func(b *testing.B) {
		b.Run("a reference the binding folded", func(b *testing.B) {
			bound, ref := foldedBinding(b)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var shape rules.TypeShape
			for c.Loop() {
				shape = bound.TypeOf(ref)
			}
			assert.Equal(b, shape.Form, symbol.FormScalar, "TypeOf returns the folded scalar")
		})

		b.Run("a reference new to the binding", func(b *testing.B) {
			source, v, ref := scripted(), viewOnly(b), builtin(intSpelling)
			c := bench.Start(b).MaxAllocs(foldAllocs)
			defer c.End()
			var (
				bound rules.Bound
				shape rules.TypeShape
			)
			for c.Loop() {
				c.Excluding(func() { bound = rules.NewBound(source, v, nil) })
				shape = bound.TypeOf(ref)
			}
			assert.Equal(b, shape.Form, symbol.FormScalar, "TypeOf folds int to a scalar")
		})

		b.Run("a list of bytes new to the binding", func(b *testing.B) {
			var source rules.SourceRules = bytesLang{scripted()}
			v, ref := viewOnly(b), byteList()
			c := bench.Start(b).MaxAllocs(foldBytesAllocs)
			defer c.End()
			var (
				bound rules.Bound
				shape rules.TypeShape
			)
			for c.Loop() {
				c.Excluding(func() { bound = rules.NewBound(source, v, nil) })
				shape = bound.TypeOf(ref)
			}
			assert.Equal(b, shape.Form, symbol.FormBytes, "TypeOf folds a list of bytes to Bytes")
		})
	})
}

// benchCalls measures every call under the bench contract at its
// ceiling. It runs one sub-benchmark for each method, in the order in
// which the methods first appear, and inside it one for each case of a
// method with cases.
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

// benchCall measures one call under the bench contract at its ceiling,
// and then checks what the last call returned. One warm-up iteration
// runs before the contract counts, so the count leaves out what the first
// call initialises.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}

// foldedBinding returns a binding over the walk fixture and a reference
// to int that the binding has folded, so TypeOf reads the shape of the
// reference from the memo of the binding.
func foldedBinding(tb assert.TB) (rules.Bound, *node.TypeRef) {
	tb.Helper()

	b, _, _ := boundOver(tb, coretest.Frozen(tb, hierarchy()))
	ref := builtin(intSpelling)
	b.TypeOf(ref)
	return b, ref
}

// boundCalls returns one call of each method of a binding over the walk
// fixture, with what the call allocates. Each check reads what the last
// call returned, so a call that measured a refusal fails. The binding
// has folded the reference that the calls read, and MembersOf walks the
// structs that [promoted], [linearised] and [aliased] return.
// [foldedBinding] measures TypeOf.
func boundCalls(tb assert.TB) []allocCall {
	tb.Helper()

	b, _, _ := boundOver(tb, coretest.Frozen(tb, hierarchy()))
	promoting, embedder := promoted(tb, rules.ShadowPromote)
	overriding, extender := promoted(tb, rules.ShadowOverride)
	linearising, mixer := linearised(tb)
	defining, keyed := aliased(tb)
	source, view := scripted(), b.View()
	intRef, strRef := builtin(intSpelling), builtin(strSpelling)
	b.TypeOf(intRef)
	present := rules.NewBound(presenceLang{source}, view, nil)
	present.TypeOf(intRef)
	plainField := field(svcPath, rowName, countField, intRef)
	withPresence := field(svcPath, rowName, presentField, intRef)
	get := method(svcPath, rowName, "Get")
	count := rowField(countField)
	params := []*node.TypeParam{{ID: coretest.MemberID(svcPath, "Box", "T", symbol.KindTypeParam), Name: "T"}}
	able := rules.NewBound(capable{source}, view, nil)
	sentinel := sentinelPrefix + rowName
	row, _ := view.Lookup(coretest.ID(svcPath, rowName, symbol.KindStruct))
	var (
		bound     rules.Bound
		src       rules.SourceRules
		v         rules.View
		lang      symbol.Lang
		callable  rules.Callable
		held      bool
		promotes  rules.MemberSet
		overrides rules.MemberSet
		linears   rules.MemberSet
		defines   rules.MemberSet
		sample    rules.Sample
		name      string
		witnesses []*node.TypeRef
		shape     rules.TypeShape
		contract  rules.ErrorContract
	)
	return []allocCall{
		{
			name: "NewBound", allocs: newBoundAllocs,
			call: func() { bound = rules.NewBound(source, view, nil) },
			check: func(tb assert.TB) {
				assert.Equal(tb, bound.Lang(), coretest.Lang, "NewBound binds the language of the source")
			},
		},
		{
			name:  "Source",
			call:  func() { src = b.Source() },
			check: func(tb assert.TB) { assert.False(tb, rules.IsAbsent(src), "Source returns the rules of the binding") },
		},
		{
			name: "View", call: func() { v = b.View() },
			check: func(tb assert.TB) { assert.False(tb, v.IsZero(), "View returns the view of the binding") },
		},
		{
			name:  "Lang",
			call:  func() { lang = b.Lang() },
			check: func(tb assert.TB) { assert.Equal(tb, lang, coretest.Lang, "Lang returns the language of the rules") },
		},
		{
			name: "CallableOf", allocs: callableAllocs, call: func() { callable, held = b.CallableOf(get) },
			check: func(tb assert.TB) {
				assert.True(tb, held, "CallableOf projects the method")
				assert.Length(tb, callable.Params, 1, "with its one parameter")
			},
		},
		{
			name: "MembersOf", caseName: "under ShadowPromote", allocs: membersAllocs,
			call: func() { promotes, _ = promoting.MembersOf(embedder) },
			check: func(tb assert.TB) {
				assert.Length(tb, promotes.Members, promotedMethods, "MembersOf promotes every method")
			},
		},
		{
			name: "MembersOf", caseName: "under ShadowOverride", allocs: membersAllocs,
			call: func() { overrides, _ = overriding.MembersOf(extender) },
			check: func(tb assert.TB) {
				assert.Length(tb, overrides.Members, promotedMethods, "MembersOf inherits every method")
			},
		},
		{
			name: "MembersOf", caseName: "under ShadowLinearise", allocs: linearisedAllocs,
			call: func() { linears, _ = linearising.MembersOf(mixer) },
			check: func(tb assert.TB) {
				assert.Length(tb, linears.Members, promotedMethods, "MembersOf takes each method from the first base")
			},
		},
		{
			name: "MembersOf", caseName: "through a defined type", allocs: membersAllocs,
			call: func() { defines, _ = defining.MembersOf(keyed) },
			check: func(tb assert.TB) {
				assert.Length(tb, defines.Members, promotedMethods,
					"MembersOf promotes every method of the defined type")
			},
		},
		{
			name: "SamplesOf", call: func() { sample, _ = b.SamplesOf(count, intRef, countField) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "SamplesOf derives a sample") },
		},
		{
			name: "ZeroValue", call: func() { _, held = b.ZeroValue(intRef) },
			check: func(tb assert.TB) { assert.True(tb, held, "ZeroValue derives a zero") },
		},
		{
			name: "LiteralFor", call: func() { _, held = b.LiteralFor(rules.Scope{}, strRef, "x") },
			check: func(tb assert.TB) { assert.True(tb, held, "LiteralFor derives a literal") },
		},
		{
			name: "Witnesses", allocs: witnessesAllocs, call: func() { witnesses = b.Witnesses(params) },
			check: func(tb assert.TB) { assert.Length(tb, witnesses, 1, "Witnesses derives one reference") },
		},
		{
			name: "FieldTypeOf", caseName: "a field without presence",
			call: func() { shape = present.FieldTypeOf(plainField) },
			check: func(tb assert.TB) {
				assert.Equal(tb, shape.Form, symbol.FormScalar, "FieldTypeOf returns the folded type of the field")
			},
		},
		{
			name: "FieldTypeOf", caseName: "a field with presence", allocs: fieldTypeOfAllocs,
			call: func() { shape = present.FieldTypeOf(withPresence) },
			check: func(tb assert.TB) {
				assert.Equal(tb, shape.Form, symbol.FormOptional,
					"FieldTypeOf wraps the type of the field in an optional")
			},
		},
		{
			name: "Tag", call: func() { name, held = able.Tag(taggedField, tagKey) },
			check: func(tb assert.TB) {
				assert.True(tb, held, "Tag reads the tag")
				assert.Equal(tb, name, taggedField.Tag, "Tag returns the text of the tag")
			},
		},
		{
			name: "SentinelName", allocs: sentinelNameAllocs, call: func() { name, held = able.SentinelName(rowName) },
			check: func(tb assert.TB) { assert.Equal(tb, name, sentinel, "SentinelName joins the prefix") },
		},
		{
			name: "SentinelBase", call: func() { name, held = able.SentinelBase(sentinel) },
			check: func(tb assert.TB) {
				assert.True(tb, held, "SentinelBase recognises the prefix")
				assert.Equal(tb, name, rowName, "SentinelBase returns the base")
			},
		},
		{
			name: "ErrorContract", call: func() { contract, held = able.ErrorContract(row) },
			check: func(tb assert.TB) {
				assert.True(tb, held, "ErrorContract takes Row as an error type")
				assert.True(tb, contract.Pointer, "ErrorContract returns the contract of the language")
			},
		},
	}
}

// promoted returns a binding under a shadowing rule over a struct whose
// one contributor declares [promotedMethods] methods, and the struct.
// Under ShadowOverride the struct extends the contributor, and under
// every other rule it embeds it.
func promoted(tb assert.TB, rule rules.Shadowing) (rules.Bound, *node.Struct) {
	tb.Helper()

	base := coretest.Struct(svcPath, baseName)
	for i := range promotedMethods {
		base.Methods = append(base.Methods, method(svcPath, baseName, "M"+strconv.Itoa(i)))
	}
	derived := coretest.Struct(svcPath, derivedName)
	contribution := rules.ContributesEmbeds
	if rule == rules.ShadowOverride {
		contribution = rules.ContributesExtends
		derived.Extends = []*node.TypeRef{named(svcPath, baseName, symbol.KindStruct)}
	} else {
		derived.Embeds = []*node.Embed{embedding(svcPath, baseName)}
	}
	v, _, _ := viewOver(tb, coretest.Frozen(tb, coretest.Package(svcPath, base, derived)))
	source := policy{scripted(), rules.MemberPolicy{Contributes: []rules.Contribution{contribution}, Shadowing: rule}}
	return rules.NewBound(source, v, nil), derived
}

// linearised returns a binding under ShadowLinearise over a struct that
// extends two contributors, and the struct. Each contributor declares the
// [promotedMethods] methods of [promoted], so every name arrives twice
// and the settle computes the C3 linearization of the struct.
func linearised(tb assert.TB) (rules.Bound, *node.Struct) {
	tb.Helper()

	derived := coretest.Struct(svcPath, derivedName)
	hosts := []string{baseName, mixinName}
	decls := make([]symbol.Symbol, 0, len(hosts)+1)
	decls = append(decls, derived)
	for _, host := range hosts {
		s := coretest.Struct(svcPath, host)
		for i := range promotedMethods {
			s.Methods = append(s.Methods, method(svcPath, host, "M"+strconv.Itoa(i)))
		}
		derived.Extends = append(derived.Extends, named(svcPath, host, symbol.KindStruct))
		decls = append(decls, s)
	}
	v, _, _ := viewOver(tb, coretest.Frozen(tb, coretest.Package(svcPath, decls...)))
	source := policy{scripted(), rules.MemberPolicy{
		Contributes: []rules.Contribution{rules.ContributesExtends}, Shadowing: rules.ShadowLinearise,
	}}
	return rules.NewBound(source, v, nil), derived
}

// aliased returns a binding over a struct that embeds a defined type over
// int, and the struct. The package of the defined type declares the
// [promotedMethods] methods of [promoted] for it, outside its declaration,
// as a Go package declares the methods of a defined type.
func aliased(tb assert.TB) (rules.Bound, *node.Struct) {
	tb.Helper()

	key := coretest.Alias(svcPath, keyName)
	key.Defined, key.Target = true, builtin(intSpelling)
	keyed := coretest.Struct(svcPath, keyedName)
	keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
	decls := make([]symbol.Symbol, 0, promotedMethods+2)
	decls = append(decls, key, keyed)
	for i := range promotedMethods {
		decls = append(decls, method(svcPath, keyName, "M"+strconv.Itoa(i)))
	}
	b, _, _ := boundOver(tb, coretest.Frozen(tb, coretest.Package(svcPath, decls...)))
	return b, keyed
}

// byteList returns a list of the eight-bit unsigned scalar that
// [bytesLang] classifies.
func byteList() *node.TypeRef {
	return &node.TypeRef{Spelling: "[]byte", Form: symbol.FormList, Elems: []*node.TypeRef{builtin("byte")}}
}

// viewOver returns a view over a frozen graph with a fresh read set and
// the registered keys of the kernel. It also returns the set and the
// facts, so that a case can stamp and read.
func viewOver(tb assert.TB, g *store.Graph) (rules.View, *store.ReadSet, *meta.Facts) {
	tb.Helper()

	reads := store.NewReadSet()
	reader, err := g.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph returns a reader")
	registry := meta.NewRegistry()
	keys, err := meta.Kernel(registry)
	assert.NoError(tb, err, "the kernel keys register")
	facts := meta.NewFacts(registry)
	return rules.NewView(reader, facts, reads, keys), reads, facts
}

// viewOnly returns a view over the walk fixture.
func viewOnly(tb assert.TB) rules.View {
	tb.Helper()

	v, _, _ := viewOver(tb, coretest.Frozen(tb, hierarchy()))
	return v
}

// boundOver binds the rules of the fixture to a view over the graph.
func boundOver(tb assert.TB, g *store.Graph) (rules.Bound, *store.ReadSet, *meta.Facts) {
	tb.Helper()

	v, reads, facts := viewOver(tb, g)
	return rules.NewBound(scripted(), v, nil), reads, facts
}

// scripted returns the scripted rules bound to the language of the
// fixture.
func scripted() rules.SourceRules {
	s := rulestest.Scripted()
	return native{SourceRules: s, GenericsRules: s.(rules.GenericsRules)}
}

// named returns a resolved reference to a declaration in the language of
// the fixture.
func named(path, name string, kind symbol.Kind) *node.TypeRef {
	return &node.TypeRef{Spelling: name, Target: coretest.ID(path, name, kind)}
}

// builtin returns a reference that the resolution step left without a
// target.
func builtin(spelling string) *node.TypeRef { return &node.TypeRef{Spelling: spelling} }

// field returns a typed field on a host.
func field(path, host, name string, typ *node.TypeRef) *node.Field {
	f := coretest.Field(path, host, name)
	f.Type = typ
	return f
}

// method returns a method on a host with one int parameter and one
// string result.
func method(path, host, name string) *node.Method {
	m := coretest.Method(path, host, name)
	m.Params = []*node.Param{{Name: "n", Type: builtin(intSpelling)}}
	m.Returns = []*node.Return{{Type: builtin(strSpelling)}}
	return m
}

// embedding returns an embed of a resolved struct.
func embedding(path, name string) *node.Embed {
	return &node.Embed{Ref: named(path, name, symbol.KindStruct)}
}

// embedOf returns an embed of a resolved declaration of any kind.
func embedOf(path, name string, kind symbol.Kind) *node.Embed {
	return &node.Embed{Ref: named(path, name, kind)}
}

// namedEmbed returns an embed of a resolved declaration with the identity
// that the load assigns to it. The identity is the bare name of the
// embedded type under the host.
func namedEmbed(path, host, name string, kind symbol.Kind) *node.Embed {
	e := embedOf(path, name, kind)
	e.ID = symbol.Identity{Lang: coretest.Lang, Package: path, Owner: host, Name: name, Kind: symbol.KindEmbed}
	return e
}

// iface returns an interface without members.
func iface(path, name string) *node.Interface {
	return &node.Interface{ID: coretest.ID(path, name, symbol.KindInterface), Name: name}
}

// hierarchy builds the walk fixture. It declares these types:
//   - Base, with a field and a method
//   - Derived, which embeds Base and declares a field of its own
//   - Row, with two builtin fields
//   - Store, an interface with one method
func hierarchy() *node.Package {
	base := coretest.Struct(svcPath, baseName)
	base.Fields = []*node.Field{field(svcPath, baseName, "id", builtin(intSpelling))}
	base.Methods = []*node.Method{method(svcPath, baseName, "ID")}
	derived := coretest.Struct(svcPath, derivedName)
	derived.Embeds = []*node.Embed{embedding(svcPath, baseName)}
	derived.Fields = []*node.Field{field(svcPath, derivedName, "name", builtin(strSpelling))}
	row := coretest.Struct(svcPath, rowName)
	row.Fields = []*node.Field{
		field(svcPath, rowName, nameField, builtin(strSpelling)),
		field(svcPath, rowName, countField, builtin(intSpelling)),
	}
	iface := coretest.Interface(svcPath, storeName)
	iface.Methods = []*node.Method{method(svcPath, storeName, "Get")}
	return coretest.Package(svcPath, base, derived, row, iface)
}

// rowField returns the identity of a field of Row.
func rowField(name string) symbol.Identity {
	return coretest.MemberID(svcPath, rowName, name, symbol.KindField)
}

// stamp stamps one authored text on a subject under a kernel key, as the
// sample annotator stamps it.
func stamp(tb assert.TB, facts *meta.Facts, k meta.Key[string], subject symbol.Identity, text string) {
	tb.Helper()

	assert.NoError(tb, meta.Stamp(facts, k, text, meta.Claim{Subject: subject}), "the author states a value")
}
