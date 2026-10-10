// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"strconv"
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

// The fixture's package and the names it declares, which every spec of
// the package builds on.
const (
	svcPath     = "svc"
	depPath     = "dep"
	rowName     = "Row"
	baseName    = "Base"
	derivedName = "Derived"
	storeName   = "Store"
	intSpelling = "int"
	strSpelling = "string"
	// nameField and countField are Row's two fields: a string and an
	// int.
	nameField  = "name"
	countField = "count"
)

// promotedMethods is how many methods the base of [promoted] declares.
const promotedMethods = 20

// presentField is the field that [presenceLang] reports presence for, and
// dynamicSpelling the builtin it folds to the top type.
const (
	presentField    = "present"
	dynamicSpelling = "any"
)

// The allocations of the methods of a binding over the walk fixture,
// which TestBoundAllocs checks in the ordinary run and BenchmarkBound in
// a benchmark run. A method without a constant allocates nothing.
const (
	// newBoundAllocs is the memo of folded shapes.
	newBoundAllocs = 1
	// callableAllocs is the projection of a method with one parameter
	// and one return: the parameter list, the return list, and the list
	// of return roles the scripted language classifies.
	callableAllocs = 3
	// foldAllocs is the first fold of a builtin reference: the memo's
	// first group, and the shape the memo stores out of line, because a
	// TypeShape is larger than the map stores in place.
	foldAllocs = 2
	// foldBytesAllocs is the first fold of a list of bytes: the memo's
	// first group, and the shapes of the element and of the list. Bytes
	// has no list of children.
	foldBytesAllocs = 3
	// membersAllocs is a member walk of a struct that embeds one type:
	// the members it returns, and the path of the one contributor it
	// descends into. The walk's working storage is a pooled walk's.
	membersAllocs = 2
	// typeNameAllocs is the joined name.
	typeNameAllocs = 1
	// fieldTypeOfAllocs is the list of the one child of the optional that
	// wraps the type of a field with presence.
	fieldTypeOfAllocs = 1
	// witnessesAllocs is the list of one parameter's witness, and the
	// reference the scripted language derives.
	witnessesAllocs = 2
)

// native is the scripted rules under the fixture's language, so a
// contributor in the fixture walks under the scripted policy. A
// foreign language gets the absent policy. It keeps the scripted
// generics capability.
type native struct {
	rules.SourceRules
	rules.GenericsRules
}

// Lang returns the fixture's language.
func (native) Lang() symbol.Lang { return coretest.Lang }

// policy overrides the scripted language's member policy, so a
// case exercises one shadowing rule or contribution list.
type policy struct {
	rules.SourceRules
	members rules.MemberPolicy
}

// Members returns the overriding policy.
func (p policy) Members() rules.MemberPolicy { return p.members }

// fixedSamples overrides the scripted language's samples with one
// fixed pair, so a case controls what the language derives.
type fixedSamples struct {
	rules.SourceRules
	sample, alternate rules.Sample
}

// SamplesOf returns the fixed pair.
func (f fixedSamples) SamplesOf(*node.TypeRef, string, rules.View) (rules.Sample, rules.Sample) {
	return f.sample, f.alternate
}

// otherLang is the scripted rules under the name of a language the
// fixture declares nothing in, so a case reads the fixture's
// declarations through rules that are not their own.
type otherLang struct {
	rules.SourceRules
}

// Lang returns the other language.
func (otherLang) Lang() symbol.Lang { return "other" }

// nongeneric hides the scripted language's generics capability. It
// forwards every method of the contract by name, because embedding
// the rules would promote the capability too.
type nongeneric struct {
	inner rules.SourceRules
}

// Lang returns the inner rules' language.
func (n nongeneric) Lang() symbol.Lang { return n.inner.Lang() }

// Members returns the inner rules' member policy.
func (n nongeneric) Members() rules.MemberPolicy { return n.inner.Members() }

// ParamRole returns the inner rules' role for a parameter.
func (n nongeneric) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	return n.inner.ParamRole(p, v)
}

// ReturnRoles returns the inner rules' roles for the returns.
func (n nongeneric) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return n.inner.ReturnRoles(rs, v)
}

// Builtin returns the inner rules' shape for a spelling.
func (n nongeneric) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	return n.inner.Builtin(ref, v)
}

// Resolve returns the inner rules' resolution of a spelling.
func (n nongeneric) Resolve(
	s rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	return n.inner.Resolve(s, name, kind, v)
}

// SamplesOf returns the inner rules' pair for a type.
func (n nongeneric) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return n.inner.SamplesOf(ref, hint, v)
}

// ZeroValue returns the inner rules' zero for a type.
func (n nongeneric) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return n.inner.ZeroValue(ref, v)
}

// LiteralFor returns the inner rules' value for a text.
func (n nongeneric) LiteralFor(f *node.File, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	return n.inner.LiteralFor(f, ref, text, v)
}

// TypeName returns the inner rules' join of a word onto a base.
func (n nongeneric) TypeName(word, base string) string { return n.inner.TypeName(word, base) }

// presenceLang is the scripted rules with presence beyond the type: the
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

// allocCall is one call of a function or a method: the method, which
// names its benchmark, the case it measures where the method has more
// than one call, the allocations the call makes, and the check of what
// the last call returned, which runs after the count.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
}

// A handler calls the binding's methods, so the binding's defaults and
// the calls it passes to the language are pinned here.
func TestBound(t *testing.T) {
	t.Parallel()

	t.Run("NewBound", func(t *testing.T) {
		t.Parallel()

		t.Run("binds the absent rules for nil rules", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(nil, rules.View{}, nil)
			assert.True(t, rules.IsAbsent(b.Source()), "no source is the absent one")
			assert.Equal(t, b.Lang(), symbol.Lang(""), "for the zero language")
		})
	})

	t.Run("Source", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the rules the binding was made with", func(t *testing.T) {
			t.Parallel()

			source := rulestest.Scripted()
			assert.Equal(t, rules.NewBound(source, rules.View{}, nil).Source(), source, "the binding's own rules",
				assert.ByIdentity())
		})
	})

	t.Run("View", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the view the binding reads through", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.False(t, b.View().IsZero(), "the view the fixture minted is no zero view")
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the rules' language", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Equal(t, b.Lang(), coretest.Lang, "the fixture's language")
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
				name:   "wraps the type of a field that the language's rule gives presence in an optional",
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
				assert.Equal(t, got, tt.want, "the shape of the field's type with the field's presence")
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

		t.Run("returns the language's zero of a type", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			zero, held := b.ZeroValue(builtin(intSpelling))
			assert.True(t, held, "a zero derives")
			assert.Equal(t, zero, emit.Literal(emit.LiteralInt, "0"), "as the language spells it")
		})

		t.Run("reports false for the zero view", func(t *testing.T) {
			t.Parallel()

			_, held := rules.NewBound(rulestest.Scripted(), rules.View{}, nil).ZeroValue(builtin(intSpelling))
			assert.False(t, held, "no view, no zero")
		})

		t.Run("reports false for a nil reference", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			_, held := b.ZeroValue(nil)
			assert.False(t, held, "no reference, no zero")
		})
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the language's value of a text", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			lit, held := b.LiteralFor(nil, builtin(strSpelling), "x")
			assert.True(t, held, "a literal derives")
			assert.Equal(t, lit, emit.Literal(emit.LiteralString, "x"), "as text")
		})

		t.Run("reports false for the zero view", func(t *testing.T) {
			t.Parallel()

			_, held := rules.NewBound(rulestest.Scripted(), rules.View{}, nil).
				LiteralFor(nil, builtin(strSpelling), "x")
			assert.False(t, held, "no view, no literal")
		})
	})

	t.Run("TypeName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the language's join of a word onto a base", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Equal(t, b.TypeName("Builder", "Row"), "RowBuilder", "the join is the language's")
		})
	})
}

// Each method of the binding allocates what it returns or what the
// language allocates, in the ordinary run, which runs no benchmark. A
// first fold takes a binding built outside the count, because a fold
// fills the memo it reads. The check runs alone, because the count
// includes every goroutine's allocations.
func TestBoundAllocs(t *testing.T) {
	checkCalls(t, boundCalls(t))

	folded, ref := foldedBinding(t)
	var shape rules.TypeShape
	assert.MaxAllocs(t, func() { shape = folded.TypeOf(ref) }, 0,
		"TypeOf allocates nothing for a reference the binding folded")
	assert.Equal(t, shape.Form, symbol.FormScalar, "TypeOf returns the folded scalar")

	v, bytes := viewOnly(t), byteList()
	assert.MaxAllocsWithSetup(t, func() rules.Bound { return rules.NewBound(scripted(), v, nil) },
		func(b rules.Bound) { shape = b.TypeOf(ref) },
		foldAllocs, "TypeOf allocates the memo's group and the shape for a first fold")
	assert.Equal(t, shape.Form, symbol.FormScalar, "TypeOf folds int to a scalar")

	assert.MaxAllocsWithSetup(t, func() rules.Bound { return rules.NewBound(bytesLang{scripted()}, v, nil) },
		func(b rules.Bound) { shape = b.TypeOf(bytes) },
		foldBytesAllocs, "TypeOf allocates the memo's group and two shapes for a first fold of a list of bytes")
	assert.Equal(t, shape.Form, symbol.FormBytes, "TypeOf folds a list of bytes to Bytes")
}

// checkCalls checks every call's ceiling under [assert.MaxAllocs], and
// then what the call's last run returned.
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
// ceiling: one sub-benchmark for each method, in the order the methods
// first appear, and inside it one for each case of a method with cases.
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
// runs before the contract counts, so what the first call initialises
// stays out of the count.
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
// to int the binding has folded, so TypeOf reads the reference's shape
// from the binding's memo.
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
// has folded the reference the calls read, and MembersOf walks the
// struct [promoted] returns. [foldedBinding] measures TypeOf.
func boundCalls(tb assert.TB) []allocCall {
	tb.Helper()

	b, _, _ := boundOver(tb, coretest.Frozen(tb, hierarchy()))
	promoting, embedder := promoted(tb, rules.ShadowPromote)
	overriding, extender := promoted(tb, rules.ShadowOverride)
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
	var (
		bound     rules.Bound
		src       rules.SourceRules
		v         rules.View
		lang      symbol.Lang
		callable  rules.Callable
		held      bool
		promotes  rules.MemberSet
		overrides rules.MemberSet
		sample    rules.Sample
		name      string
		witnesses []*node.TypeRef
		shape     rules.TypeShape
	)
	return []allocCall{
		{
			name: "NewBound", allocs: newBoundAllocs,
			call: func() { bound = rules.NewBound(source, view, nil) },
			check: func(tb assert.TB) {
				assert.Equal(tb, bound.Lang(), coretest.Lang, "NewBound binds the source's language")
			},
		},
		{
			name: "Source", call: func() { src = b.Source() },
			check: func(tb assert.TB) { assert.False(tb, rules.IsAbsent(src), "Source returns the binding's rules") },
		},
		{
			name: "View", call: func() { v = b.View() },
			check: func(tb assert.TB) { assert.False(tb, v.IsZero(), "View returns the binding's view") },
		},
		{
			name: "Lang", call: func() { lang = b.Lang() },
			check: func(tb assert.TB) { assert.Equal(tb, lang, coretest.Lang, "Lang returns the rules' language") },
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
			name: "SamplesOf", call: func() { sample, _ = b.SamplesOf(count, intRef, countField) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "SamplesOf derives a sample") },
		},
		{
			name: "ZeroValue", call: func() { _, held = b.ZeroValue(intRef) },
			check: func(tb assert.TB) { assert.True(tb, held, "ZeroValue derives a zero") },
		},
		{
			name: "LiteralFor", call: func() { _, held = b.LiteralFor(nil, strRef, "x") },
			check: func(tb assert.TB) { assert.True(tb, held, "LiteralFor derives a literal") },
		},
		{
			name: "TypeName", allocs: typeNameAllocs, call: func() { name = b.TypeName("Builder", rowName) },
			check: func(tb assert.TB) { assert.Equal(tb, name, "RowBuilder", "TypeName joins the word onto the base") },
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
				assert.Equal(tb, shape.Form, symbol.FormOptional, "FieldTypeOf wraps the field's type in an optional")
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

// byteList returns a list of the eight-bit unsigned scalar [bytesLang]
// classifies.
func byteList() *node.TypeRef {
	return &node.TypeRef{Spelling: "[]byte", Form: symbol.FormList, Elems: []*node.TypeRef{builtin("byte")}}
}

// viewOver returns a view over a frozen graph with a fresh read set and
// the kernel's keys registered, and the set and the facts, so a case can
// stamp and read.
func viewOver(tb assert.TB, g *store.Graph) (rules.View, *store.ReadSet, *meta.Facts) {
	tb.Helper()

	reads := store.NewReadSet()
	reader, err := g.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	registry := meta.NewRegistry()
	keys, err := meta.Kernel(registry)
	assert.NoError(tb, err, "the kernel keys register")
	facts := meta.NewFacts(registry)
	return rules.View{Decls: reader, Facts: facts, Reads: reads, Kernel: keys}, reads, facts
}

// viewOnly returns a view over the walk fixture.
func viewOnly(tb assert.TB) rules.View {
	tb.Helper()

	v, _, _ := viewOver(tb, coretest.Frozen(tb, hierarchy()))
	return v
}

// boundOver binds the fixture's rules to a view over the graph.
func boundOver(tb assert.TB, g *store.Graph) (rules.Bound, *store.ReadSet, *meta.Facts) {
	tb.Helper()

	v, reads, facts := viewOver(tb, g)
	return rules.NewBound(scripted(), v, nil), reads, facts
}

// scripted returns the scripted rules bound to the fixture's
// language.
func scripted() rules.SourceRules {
	s := rulestest.Scripted()
	return native{SourceRules: s, GenericsRules: s.(rules.GenericsRules)}
}

// named returns a resolved reference to a declaration in the
// fixture's language.
func named(path, name string, kind symbol.Kind) *node.TypeRef {
	return &node.TypeRef{Spelling: name, Target: coretest.ID(path, name, kind)}
}

// builtin returns a reference the resolution step left without a
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

// namedEmbed returns an embed of a resolved declaration with the
// identity the load assigns it: the embedded type's bare name under
// the host.
func namedEmbed(path, host, name string, kind symbol.Kind) *node.Embed {
	e := embedOf(path, name, kind)
	e.ID = symbol.Identity{Lang: coretest.Lang, Package: path, Owner: host, Name: name, Kind: symbol.KindEmbed}
	return e
}

// iface returns an interface without members.
func iface(path, name string) *node.Interface {
	return &node.Interface{ID: coretest.ID(path, name, symbol.KindInterface), Name: name}
}

// hierarchy builds the walk fixture: Base with a field and a method,
// Derived embedding Base and declaring a field of its own, Row with
// two builtin fields, and Store, an interface with one method.
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

// rowField returns the identity of one of Row's fields.
func rowField(name string) symbol.Identity {
	return coretest.MemberID(svcPath, rowName, name, symbol.KindField)
}

// stamp states one authored text on a subject under a kernel key,
// the way the sample annotator stamps it.
func stamp(tb assert.TB, facts *meta.Facts, k meta.Key[string], subject symbol.Identity, text string) {
	tb.Helper()

	assert.NoError(tb, meta.Stamp(facts, k, text, meta.Claim{Subject: subject}), "the author states a value")
}
