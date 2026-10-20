// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The ceilings of values.go: the pair of an integer and the zero and the
// literal of a builtin allocate nothing, and the pair of a string allocates
// its two hinted texts. The pair of the dataclass Item allocates its
// constructor with its parameters, the bound and the walk of its members, its
// reference, the arguments of the two calls, and the two hinted texts of its
// field sku.
const (
	valuesAllocs    = 0
	textPairAllocs  = 2
	classPairAllocs = 8
)

// The declarations of the fixture that only the specs of values.go read.
var (
	tagsID = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Tags", Kind: symbol.KindAlias}
	pingID = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Ping", Kind: symbol.KindAlias}
	userID = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "UserId", Kind: symbol.KindAlias}
	modeID = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Mode", Kind: symbol.KindAlias}
	bareID = symbol.Identity{Lang: python.Lang, Package: brokenPkg, Name: "Bare", Kind: symbol.KindAlias}
	skuID  = symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Owner: "Item", Name: "sku", Kind: symbol.KindField,
	}
	newID = symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Owner: "Status", Name: "NEW", Kind: symbol.KindEnumVariant,
	}
	paidID = symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Owner: "Status", Name: "PAID", Kind: symbol.KindEnumVariant,
	}
	boxParam = symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Owner: "Box", Name: "T", Kind: symbol.KindTypeParam,
	}
	timestampClass = symbol.Identity{
		Lang: python.Lang, Package: datetimeModule, Name: "datetime", Kind: symbol.KindStruct,
	}
)

// TestValues pins the samples, the zeros and the literals of the types of
// Python.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("DeriveSamples", func(t *testing.T) {
		t.Parallel()

		catalog := []struct {
			name          string
			give          string
			wantSample    func(ref *node.TypeRef) rules.Sample
			wantAlternate func(ref *node.TypeRef) rules.Sample
		}{
			{
				name: "derives a tuple of a sample of each member",
				give: "sizes",
				wantSample: func(ref *node.TypeRef) rules.Sample {
					return rules.Of(emit.Composite(rules.EmitRef(ref),
						emit.Element(emit.Number(emit.LiteralInt, "42", 0)),
						emit.Element(emit.Literal(emit.LiteralString, "sizes-a"))))
				},
				wantAlternate: func(ref *node.TypeRef) rules.Sample {
					return rules.Of(emit.Composite(rules.EmitRef(ref),
						emit.Element(emit.Number(emit.LiteralInt, "7", 0)),
						emit.Element(emit.Literal(emit.LiteralString, "sizes-b"))))
				},
			},
			{
				name: "derives a list of one element for a Sequence",
				give: "weights",
				wantSample: func(ref *node.TypeRef) rules.Sample {
					element := emit.Element(emit.Number(emit.LiteralFloat, "1.5", 64))
					return rules.Of(emit.Composite(rules.EmitRef(ref), element))
				},
				wantAlternate: func(ref *node.TypeRef) rules.Sample {
					element := emit.Element(emit.Number(emit.LiteralFloat, "2.5", 64))
					return rules.Of(emit.Composite(rules.EmitRef(ref), element))
				},
			},
			{
				name: "derives a dict of one entry whose value is a call of its class",
				give: "prices",
				wantSample: func(ref *node.TypeRef) rules.Sample {
					money := emit.Composite(rules.EmitRef(ref.Args[1]),
						emit.NamedField("amount", emit.Number(emit.LiteralInt, "42", 0)))
					return rules.Of(emit.Composite(rules.EmitRef(ref),
						emit.KeyedEntry(emit.Literal(emit.LiteralString, "prices-a"), money)))
				},
				wantAlternate: func(ref *node.TypeRef) rules.Sample {
					money := emit.Composite(rules.EmitRef(ref.Args[1]),
						emit.NamedField("amount", emit.Number(emit.LiteralInt, "42", 0)))
					return rules.Of(emit.Composite(rules.EmitRef(ref),
						emit.KeyedEntry(emit.Literal(emit.LiteralString, "prices-b"), money)))
				},
			},
			{
				name: "derives the bytes as Python text",
				give: "flags",
				wantSample: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Raw(python.Lang, `b"a"`))
				},
				wantAlternate: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Raw(python.Lang, `b"b"`))
				},
			},
			{
				name: "derives two constructions of a datetime",
				give: "when",
				wantSample: func(ref *node.TypeRef) rules.Sample {
					return rules.Of(emit.Construction(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "2023", 0),
						emit.Number(emit.LiteralInt, "11", 0), emit.Number(emit.LiteralInt, "14", 0)))
				},
				wantAlternate: func(ref *node.TypeRef) rules.Sample {
					return rules.Of(emit.Construction(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "2023", 0),
						emit.Number(emit.LiteralInt, "11", 0), emit.Number(emit.LiteralInt, "15", 0)))
				},
			},
			{
				name: "derives two constructions of a timedelta",
				give: "wait",
				wantSample: func(ref *node.TypeRef) rules.Sample {
					return rules.Of(emit.Construction(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "1", 0)))
				},
				wantAlternate: func(ref *node.TypeRef) rules.Sample {
					return rules.Of(emit.Construction(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "2", 0)))
				},
			},
			{
				name: "derives the values of the Literal that an implicit alias names",
				give: "mode",
				wantSample: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Literal(emit.LiteralString, "fast"))
				},
				wantAlternate: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Literal(emit.LiteralString, "slow"))
				},
			},
			{
				name: "derives the one value of a Literal of one value and refuses the second",
				give: "single",
				wantSample: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Literal(emit.LiteralString, "x"))
				},
				wantAlternate: func(*node.TypeRef) rules.Sample { return rules.Refused(rules.RefusedNoLiteral) },
			},
			{
				name:          "derives the names of the members of a Literal of an enum",
				give:          "favourite",
				wantSample:    func(*node.TypeRef) rules.Sample { return rules.Of(emit.Name(newID)) },
				wantAlternate: func(*node.TypeRef) rules.Sample { return rules.Of(emit.Name(paidID)) },
			},
			{
				name: "derives the conversions of the pair of the base of a NewType",
				give: "owner",
				wantSample: func(ref *node.TypeRef) rules.Sample {
					return rules.Of(emit.Conversion(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "42", 0)))
				},
				wantAlternate: func(ref *node.TypeRef) rules.Sample {
					return rules.Of(emit.Conversion(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "7", 0)))
				},
			},
			{
				name: "derives the first two distinct values of the members of a union",
				give: "either",
				wantSample: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Number(emit.LiteralInt, "42", 0))
				},
				wantAlternate: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Number(emit.LiteralInt, "7", 0))
				},
			},
			{
				name: "derives the pair of the type of an optional",
				give: "maybe",
				wantSample: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Literal(emit.LiteralBool, "true"))
				},
				wantAlternate: func(*node.TypeRef) rules.Sample {
					return rules.Of(emit.Literal(emit.LiteralBool, "false"))
				},
			},
			{
				name: "derives the two values of the first member of a union of lists",
				give: "lists",
				wantSample: func(ref *node.TypeRef) rules.Sample {
					element := emit.Element(emit.Number(emit.LiteralInt, "42", 0))
					return rules.Of(emit.Composite(rules.EmitRef(ref.Elems[0]), element))
				},
				wantAlternate: func(ref *node.TypeRef) rules.Sample {
					element := emit.Element(emit.Number(emit.LiteralInt, "7", 0))
					return rules.Of(emit.Composite(rules.EmitRef(ref.Elems[0]), element))
				},
			},
			{
				name:          "refuses a set, which has no form of the model",
				give:          "seen",
				wantSample:    func(*node.TypeRef) rules.Sample { return rules.Refused(rules.RefusedNoLiteral) },
				wantAlternate: func(*node.TypeRef) rules.Sample { return rules.Refused(rules.RefusedNoLiteral) },
			},
			{
				name:          "refuses the top type",
				give:          "anything",
				wantSample:    func(*node.TypeRef) rules.Sample { return rules.Refused(rules.RefusedNoLiteral) },
				wantAlternate: func(*node.TypeRef) rules.Sample { return rules.Refused(rules.RefusedNoLiteral) },
			},
		}
		for _, tt := range catalog {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				ref := f.member(t, catalogID, tt.give).Type
				sample, alternate := pythonrules.Rules{}.DeriveSamples(ref, tt.give, f.view)
				expect.Equal(t, sample, tt.wantSample(ref), "the sample of the field "+tt.give)
				expect.Equal(t, alternate, tt.wantAlternate(ref), "the alternate of the field "+tt.give)
			})
		}

		t.Run("derives a list of the pair of the element of the target of an alias", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			alias, is := f.decl(t, tagsID).(*node.Alias)
			assert.True(t, is, "Tags is an alias")
			sample, alternate := pythonrules.Rules{}.DeriveSamples(f.member(t, catalogID, "tags").Type, "tags", f.view)
			list := rules.EmitRef(alias.Target)
			expect.Equal(
				t,
				sample,
				rules.Of(emit.Composite(list, emit.Element(emit.Literal(emit.LiteralString, "tags-a")))),
				"the sample is a list of one string",
			)
			expect.Equal(t, alternate,
				rules.Of(emit.Composite(list, emit.Element(emit.Literal(emit.LiteralString, "tags-b")))),
				"the alternate is a list of the other string")
		})

		t.Run("derives the pair of the target of a generic alias with its arguments", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			sample, _ := pythonrules.Rules{}.DeriveSamples(f.member(t, catalogID, "pair").Type, "pair", f.view)
			assert.True(t, sample.OK(), "the pair derives")
			expect.Equal(t, sample.Value.Fields, []emit.ValueField{
				emit.Element(emit.Number(emit.LiteralInt, "42", 0)),
				emit.Element(emit.Literal(emit.LiteralString, "pair-a")),
			}, "the tuple of the arguments")
		})

		classes := []struct {
			name          string
			give          string
			pkg           string
			kind          symbol.Kind
			wantSample    []emit.ValueField
			wantAlternate []emit.ValueField
		}{
			{
				name:          "derives two calls of a dataclass with its required field",
				give:          "Item",
				wantSample:    []emit.ValueField{emit.NamedField("sku", emit.Literal(emit.LiteralString, "sku-a"))},
				wantAlternate: []emit.ValueField{emit.NamedField("sku", emit.Literal(emit.LiteralString, "sku-b"))},
			},
			{
				name: "derives two calls of a dataclass that differ in its first field",
				give: "Address",
				wantSample: []emit.ValueField{
					emit.NamedField("street", emit.Literal(emit.LiteralString, "street-a")),
					emit.NamedField("city", emit.Literal(emit.LiteralString, "city-a")),
				},
				wantAlternate: []emit.ValueField{
					emit.NamedField("street", emit.Literal(emit.LiteralString, "street-b")),
					emit.NamedField("city", emit.Literal(emit.LiteralString, "city-a")),
				},
			},
			{
				name:          "derives two calls of a named tuple",
				give:          "Point",
				wantSample:    []emit.ValueField{emit.NamedField("x", emit.Number(emit.LiteralInt, "42", 0))},
				wantAlternate: []emit.ValueField{emit.NamedField("x", emit.Number(emit.LiteralInt, "7", 0))},
			},
			{
				name:          "derives two calls of the __init__ of a class",
				give:          "Account",
				wantSample:    []emit.ValueField{emit.NamedField("owner", emit.Literal(emit.LiteralString, "owner-a"))},
				wantAlternate: []emit.ValueField{emit.NamedField("owner", emit.Literal(emit.LiteralString, "owner-b"))},
			},
			{
				name:          "derives two calls of the __new__ of a class",
				give:          "Token",
				wantSample:    []emit.ValueField{emit.NamedField("text", emit.Literal(emit.LiteralString, "text-a"))},
				wantAlternate: []emit.ValueField{emit.NamedField("text", emit.Literal(emit.LiteralString, "text-b"))},
			},
			{
				name:          "derives two calls of the first overload of a constructor",
				give:          "Parsed",
				wantSample:    []emit.ValueField{emit.NamedField("value", emit.Number(emit.LiteralInt, "42", 0))},
				wantAlternate: []emit.ValueField{emit.NamedField("value", emit.Number(emit.LiteralInt, "7", 0))},
			},
			{
				name:          "derives an argument by position for a parameter before the slash",
				give:          "Pinned",
				wantSample:    []emit.ValueField{emit.Element(emit.Number(emit.LiteralInt, "42", 0))},
				wantAlternate: []emit.ValueField{emit.Element(emit.Number(emit.LiteralInt, "7", 0))},
			},
			{
				name:          "derives the first optional field whose pair derives",
				give:          "Defaults",
				wantSample:    []emit.ValueField{emit.NamedField("label", emit.Literal(emit.LiteralString, "label-a"))},
				wantAlternate: []emit.ValueField{emit.NamedField("label", emit.Literal(emit.LiteralString, "label-b"))},
			},
			{
				name:          "derives two dicts of the first key of a TypedDict without required keys",
				give:          "Payload",
				kind:          symbol.KindInterface,
				wantSample:    []emit.ValueField{emit.NamedField("name", emit.Literal(emit.LiteralString, "name-a"))},
				wantAlternate: []emit.ValueField{emit.NamedField("name", emit.Literal(emit.LiteralString, "name-b"))},
			},
			{
				name:          "derives two dicts of the required key of a TypedDict",
				give:          "Required",
				kind:          symbol.KindInterface,
				wantSample:    []emit.ValueField{emit.NamedField("key", emit.Literal(emit.LiteralString, "key-a"))},
				wantAlternate: []emit.ValueField{emit.NamedField("key", emit.Literal(emit.LiteralString, "key-b"))},
			},
			{
				name: "derives two dicts of the required keys of a TypedDict that differ in the first key",
				give: "Credentials",
				kind: symbol.KindInterface,
				wantSample: []emit.ValueField{
					emit.NamedField("user", emit.Literal(emit.LiteralString, "user-a")),
					emit.NamedField("token", emit.Literal(emit.LiteralString, "token-a")),
				},
				wantAlternate: []emit.ValueField{
					emit.NamedField("user", emit.Literal(emit.LiteralString, "user-b")),
					emit.NamedField("token", emit.Literal(emit.LiteralString, "token-a")),
				},
			},
			{
				name:          "skips a key of a TypedDict at type level and a key whose pair refuses",
				give:          "Loose",
				pkg:           brokenPkg,
				kind:          symbol.KindInterface,
				wantSample:    []emit.ValueField{emit.NamedField("name", emit.Literal(emit.LiteralString, "name-a"))},
				wantAlternate: []emit.ValueField{emit.NamedField("name", emit.Literal(emit.LiteralString, "name-b"))},
			},
		}
		for _, tt := range classes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{
					Lang: python.Lang, Package: cmp.Or(tt.pkg, modelsPkg), Name: tt.give,
					Kind: cmp.Or(tt.kind, symbol.KindStruct),
				}
				ref := &node.TypeRef{Spelling: tt.give, Target: id}
				sample, alternate := pythonrules.Rules{}.DeriveSamples(ref, "", f.view)
				expect.Equal(t, sample, rules.Of(emit.Composite(rules.EmitRef(ref), tt.wantSample...)), "the sample")
				expect.Equal(t, alternate, rules.Of(emit.Composite(rules.EmitRef(ref), tt.wantAlternate...)),
					"the alternate")
			})
		}

		refusals := []struct {
			name  string
			give  string
			pkg   string
			owner string
			kind  symbol.Kind
			want  rules.Refusal
		}{
			{name: "refuses a dataclass whose required field refuses", give: "Bag", want: rules.RefusedNoLiteral},
			{name: "refuses a class whose constructor takes nothing", give: "Plain", want: rules.RefusedNoLiteral},
			{
				name: "refuses a class whose bases form a cycle",
				give: "Up",
				pkg:  brokenPkg,
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a class that refers to itself past the depth of the walk",
				give: "Linked",
				want: rules.RefusedDepth,
			},
			{
				name: "refuses a TypedDict whose required key refuses",
				give: "BadDict", kind: symbol.KindInterface, want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a TypedDict whose one optional key refuses",
				give: "Nothing", kind: symbol.KindInterface, want: rules.RefusedNoLiteral,
			},
			{name: "refuses a Protocol", give: "Named", kind: symbol.KindInterface, want: rules.RefusedNoLiteral},
			{
				name: "refuses an enum without two members of known values",
				give: "Ordinal", kind: symbol.KindEnum, want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a type parameter",
				give: "T", owner: "Box", kind: symbol.KindTypeParam, want: rules.RefusedNoLiteral,
			},
			{name: "refuses a type outside the view", give: "Gone", pkg: "nowhere", want: rules.RefusedUnresolved},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{
					Lang: python.Lang, Package: cmp.Or(tt.pkg, modelsPkg), Owner: tt.owner, Name: tt.give,
					Kind: cmp.Or(tt.kind, symbol.KindStruct),
				}
				sample, alternate := pythonrules.Rules{}.DeriveSamples(&node.TypeRef{Spelling: tt.give, Target: id}, "",
					f.view)
				expect.Equal(t, sample, rules.Refused(tt.want), "the sample refuses")
				expect.Equal(t, alternate, rules.Refused(tt.want), "the alternate refuses")
			})
		}

		t.Run("derives the names of the first two members of distinct values of an enum", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			ref := &node.TypeRef{Spelling: "Status", Target: statusID}
			sample, alternate := pythonrules.Rules{}.DeriveSamples(ref, "", f.view)
			expect.Equal(t, sample, rules.Of(emit.Name(newID)), "the sample is NEW")
			expect.Equal(t, alternate, rules.Of(emit.Name(paidID)), "the alternate is PAID")
		})

		t.Run("refuses an enum whose known members repeat one value", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			same := symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Same", Kind: symbol.KindEnum}
			sample, _ := pythonrules.Rules{}.DeriveSamples(&node.TypeRef{Spelling: same.Name, Target: same}, "", f.view)
			expect.Equal(t, sample, rules.Refused(rules.RefusedNoLiteral), "Same has one value")
		})

		t.Run("takes the values that an author stated for a field", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			claim := meta.Claim{Subject: skuID, Authority: meta.AuthorityPlugin, Plugin: "fixture"}
			for key, value := range map[meta.Key[string]]string{f.Keys.Sample: "stated", f.Keys.Alternate: "other"} {
				assert.NoError(t, meta.Stamp(f.Facts, key, value, claim), "the fixture stamp applies")
			}
			ref := &node.TypeRef{Spelling: "Item", Target: itemID}
			sample, alternate := pythonrules.Rules{}.DeriveSamples(ref, "", f.view)
			assert.True(t, sample.OK(), "the sample derives")
			assert.True(t, alternate.OK(), "the alternate derives")
			expect.Equal(t, sample.Value.Fields, []emit.ValueField{
				emit.NamedField("sku", emit.Literal(emit.LiteralString, "stated")),
			}, "the sample takes the stated value")
			expect.Equal(t, alternate.Value.Fields, []emit.ValueField{
				emit.NamedField("sku", emit.Literal(emit.LiteralString, "other")),
			}, "the alternate takes the stated value")
		})

		t.Run("derives the half that an author did not state", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			claim := meta.Claim{Subject: skuID, Authority: meta.AuthorityPlugin, Plugin: "fixture"}
			assert.NoError(t, meta.Stamp(f.Facts, f.Keys.Sample, "stated", claim), "the fixture stamp applies")
			ref := &node.TypeRef{Spelling: "Item", Target: itemID}
			_, alternate := pythonrules.Rules{}.DeriveSamples(ref, "", f.view)
			assert.True(t, alternate.OK(), "the alternate derives")
			expect.Equal(t, alternate.Value.Fields, []emit.ValueField{
				emit.NamedField("sku", emit.Literal(emit.LiteralString, "sku-b")),
			}, "the alternate is the derived value")
		})

		strRef := &node.TypeRef{Spelling: "str"}
		intRef := &node.TypeRef{Spelling: "int"}
		thing := &node.TypeRef{Spelling: "Thing", Package: modelsPkg}
		none := &node.TypeRef{Spelling: "None"}
		exception := &node.TypeRef{Spelling: "Exception"}
		ints := &node.TypeRef{Spelling: "int[]", Form: symbol.FormList, Elems: []*node.TypeRef{intRef}}
		intList := &node.TypeRef{Spelling: "list", Args: []*node.TypeRef{intRef}}
		set := &node.TypeRef{Spelling: "set"}
		bytearray := &node.TypeRef{Spelling: "bytearray"}
		ones := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: []*node.TypeRef{
			{Spelling: "1"}, {Spelling: "2"},
		}}
		extended := &node.TypeRef{Spelling: literalSpelling, Package: typingExtensions, Args: []*node.TypeRef{
			{Spelling: "1"}, {Spelling: "2"},
		}}
		repeated := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: []*node.TypeRef{
			{Spelling: `"a"`}, {Spelling: `"a"`},
		}}
		built := []struct {
			name          string
			give          *node.TypeRef
			hint          string
			wantSample    rules.Sample
			wantAlternate rules.Sample
		}{
			{
				name:          "derives two integers of a builtin without a target",
				give:          intRef,
				wantSample:    rules.Of(emit.Number(emit.LiteralInt, "42", 0)),
				wantAlternate: rules.Of(emit.Number(emit.LiteralInt, "7", 0)),
			},
			{
				name:          "derives two strings of the default hint",
				give:          strRef,
				wantSample:    rules.Of(emit.Literal(emit.LiteralString, "sample-a")),
				wantAlternate: rules.Of(emit.Literal(emit.LiteralString, "sample-b")),
			},
			{
				name:          "derives None and refuses a second value",
				give:          none,
				wantSample:    rules.Of(emit.Literal(emit.LiteralNil, "")),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name: "derives two exceptions of the hinted strings",
				give: exception,
				hint: "boom",
				wantSample: rules.Of(
					emit.Construction(rules.EmitRef(exception), emit.Literal(emit.LiteralString, "boom-a")),
				),
				wantAlternate: rules.Of(
					emit.Construction(rules.EmitRef(exception), emit.Literal(emit.LiteralString, "boom-b")),
				),
			},
			{
				name:          "derives the values of a Literal of integers",
				give:          ones,
				wantSample:    rules.Of(emit.Number(emit.LiteralInt, "1", 0)),
				wantAlternate: rules.Of(emit.Number(emit.LiteralInt, "2", 0)),
			},
			{
				name:          "derives the values of a Literal of typing_extensions",
				give:          extended,
				wantSample:    rules.Of(emit.Number(emit.LiteralInt, "1", 0)),
				wantAlternate: rules.Of(emit.Number(emit.LiteralInt, "2", 0)),
			},
			{
				name: "derives two constructions of a bytearray",
				give: bytearray,
				wantSample: rules.Of(
					emit.Construction(rules.EmitRef(bytearray), emit.Raw(python.Lang, `b"a"`)),
				),
				wantAlternate: rules.Of(
					emit.Construction(rules.EmitRef(bytearray), emit.Raw(python.Lang, `b"b"`)),
				),
			},
			{
				name:          "derives one value of a Literal that repeats it",
				give:          repeated,
				wantSample:    rules.Of(emit.Literal(emit.LiteralString, "a")),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name: "derives a list of an element of a list form",
				give: ints,
				wantSample: rules.Of(
					emit.Composite(rules.EmitRef(ints), emit.Element(emit.Number(emit.LiteralInt, "42", 0))),
				),
				wantAlternate: rules.Of(
					emit.Composite(rules.EmitRef(ints), emit.Element(emit.Number(emit.LiteralInt, "7", 0))),
				),
			},
			{
				name:          "derives a union from the member after a member that refuses",
				give:          &node.TypeRef{Form: symbol.FormUnion, Elems: []*node.TypeRef{thing, intRef}},
				wantSample:    rules.Of(emit.Number(emit.LiteralInt, "42", 0)),
				wantAlternate: rules.Of(emit.Number(emit.LiteralInt, "7", 0)),
			},
			{
				name:       "derives the values of two kinds of the members of a union",
				give:       &node.TypeRef{Form: symbol.FormUnion, Elems: []*node.TypeRef{none, intList}},
				wantSample: rules.Of(emit.Literal(emit.LiteralNil, "")),
				wantAlternate: rules.Of(
					emit.Composite(rules.EmitRef(intList), emit.Element(emit.Number(emit.LiteralInt, "42", 0))),
				),
			},
			{
				name:          "refuses a union whose members derive one value with the refusal of the first",
				give:          &node.TypeRef{Form: symbol.FormUnion, Elems: []*node.TypeRef{none, none}},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a name outside the table of the builtins",
				give:          thing,
				wantSample:    rules.Refused(rules.RefusedUnresolved),
				wantAlternate: rules.Refused(rules.RefusedUnresolved),
			},
			{
				name:          "refuses a generic class without arguments",
				give:          &node.TypeRef{Spelling: "list"},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a dict whose key refuses",
				give:          &node.TypeRef{Spelling: "dict", Args: []*node.TypeRef{set, intRef}},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a tuple whose member refuses",
				give:          &node.TypeRef{Form: symbol.FormTuple, Elems: []*node.TypeRef{intRef, thing}},
				wantSample:    rules.Refused(rules.RefusedUnresolved),
				wantAlternate: rules.Refused(rules.RefusedUnresolved),
			},
			{
				name:          "refuses a set without a target, which has no form of the model",
				give:          set,
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a tuple without members",
				give:          &node.TypeRef{Spelling: "tuple[()]", Form: symbol.FormTuple},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses an optional without its type",
				give:          &node.TypeRef{Spelling: "None", Form: symbol.FormOptional},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a function type",
				give:          &node.TypeRef{Spelling: "Callable[[],None]", Form: symbol.FormFunc},
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:          "refuses a nil reference",
				wantSample:    rules.Refused(rules.RefusedNoLiteral),
				wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
		}
		for _, tt := range built {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				sample, alternate := pythonrules.Rules{}.DeriveSamples(tt.give, tt.hint, rules.View{})
				expect.Equal(t, sample, tt.wantSample, "the sample")
				expect.Equal(t, alternate, tt.wantAlternate, "the alternate")
			})
		}

		t.Run("derives two dicts of one entry of a dict form", func(t *testing.T) {
			t.Parallel()

			ref := &node.TypeRef{Spelling: "dict", Args: []*node.TypeRef{strRef, intRef}}
			sample, alternate := pythonrules.Rules{}.DeriveSamples(ref, "k", rules.View{})
			value := emit.Number(emit.LiteralInt, "42", 0)
			expect.Equal(
				t,
				sample,
				rules.Of(
					emit.Composite(rules.EmitRef(ref), emit.KeyedEntry(emit.Literal(emit.LiteralString, "k-a"), value)),
				),
				"the sample has the first key",
			)
			expect.Equal(
				t,
				alternate,
				rules.Of(
					emit.Composite(rules.EmitRef(ref), emit.KeyedEntry(emit.Literal(emit.LiteralString, "k-b"), value)),
				),
				"the alternate has the second key",
			)
		})

		t.Run("derives two constructions of a datetime without a target", func(t *testing.T) {
			t.Parallel()

			ref := &node.TypeRef{Spelling: "datetime", Package: datetimeModule}
			sample, _ := pythonrules.Rules{}.DeriveSamples(ref, "", rules.View{})
			want := emit.Construction(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "2023", 0),
				emit.Number(emit.LiteralInt, "11", 0), emit.Number(emit.LiteralInt, "14", 0))
			expect.Equal(t, sample, rules.Of(want), "the sample is a date")
		})
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		list := &node.TypeRef{Spelling: "int[]", Form: symbol.FormList, Elems: []*node.TypeRef{{Spelling: "int"}}}
		empty := &node.TypeRef{Spelling: "tuple[()]", Form: symbol.FormTuple}
		null := emit.Literal(emit.LiteralNil, "")
		maybeA := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: []*node.TypeRef{
			{Spelling: `"a"`}, {Spelling: "None"},
		}}
		onlyA := &node.TypeRef{
			Spelling: literalSpelling,
			Package:  typingModule,
			Args:     []*node.TypeRef{{Spelling: `"a"`}},
		}
		extendedA := &node.TypeRef{
			Spelling: literalSpelling,
			Package:  typingExtensions,
			Args:     []*node.TypeRef{{Spelling: `"a"`}},
		}
		bytearray := &node.TypeRef{Spelling: "bytearray"}
		built := []struct {
			name   string
			give   *node.TypeRef
			want   emit.Value
			wantOK bool
		}{
			{
				name:   "returns 0 for int",
				give:   &node.TypeRef{Spelling: "int"},
				want:   emit.Number(emit.LiteralInt, "0", 0),
				wantOK: true,
			},
			{
				name:   "returns 0 of 64 bits for float",
				give:   &node.TypeRef{Spelling: "float"},
				want:   emit.Number(emit.LiteralFloat, "0", 64),
				wantOK: true,
			},
			{
				name:   "returns False for bool",
				give:   &node.TypeRef{Spelling: "bool"},
				want:   emit.Literal(emit.LiteralBool, "false"),
				wantOK: true,
			},
			{
				name:   "returns the empty string for str",
				give:   &node.TypeRef{Spelling: "str"},
				want:   emit.Literal(emit.LiteralString, ""),
				wantOK: true,
			},
			{
				name:   "returns the empty bytes for bytes",
				give:   &node.TypeRef{Spelling: "bytes"},
				want:   emit.Raw(python.Lang, `b""`),
				wantOK: true,
			},
			{
				name:   "returns the construction without arguments for bytearray",
				give:   bytearray,
				want:   emit.Construction(rules.EmitRef(bytearray)),
				wantOK: true,
			},
			{name: "returns None for the top type", give: &node.TypeRef{Spelling: "object"}, want: null, wantOK: true},
			{name: "returns None for None", give: &node.TypeRef{Spelling: "None"}, want: null, wantOK: true},
			{name: "returns None for an optional Literal", give: maybeA, want: null, wantOK: true},
			{
				name:   "returns None for an optional form",
				give:   &node.TypeRef{Form: symbol.FormOptional, Elems: []*node.TypeRef{{Spelling: "int"}}},
				want:   null,
				wantOK: true,
			},
			{
				name:   "returns the empty list for a list form",
				give:   list,
				want:   emit.Composite(rules.EmitRef(list)),
				wantOK: true,
			},
			{
				name:   "returns the empty tuple for a tuple without members",
				give:   empty,
				want:   emit.Composite(rules.EmitRef(empty)),
				wantOK: true,
			},
			{name: "reports false for a Literal", give: onlyA},
			{name: "reports false for a Literal of typing_extensions", give: extendedA},
			{name: "reports false for a set", give: &node.TypeRef{Spelling: "set"}},
			{name: "reports false for a union", give: &node.TypeRef{Spelling: "int|str", Form: symbol.FormUnion}},
			{name: "reports false for a nil reference"},
		}
		for _, tt := range built {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := pythonrules.Rules{}.ZeroValue(tt.give, rules.View{})
				expect.Equal(t, ok, tt.wantOK, "whether the type has a zero")
				expect.Equal(t, got, tt.want, "the zero of the type")
			})
		}

		fields := []struct {
			name string
			give string
			want func(ref *node.TypeRef) emit.Value
		}{
			{
				name: "returns the empty dict for a dict of typeshed",
				give: "counts",
				want: func(ref *node.TypeRef) emit.Value { return emit.Composite(rules.EmitRef(ref)) },
			},
			{
				name: "returns the conversion of the zero of the base of a NewType",
				give: "owner",
				want: func(ref *node.TypeRef) emit.Value {
					return emit.Conversion(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "0", 0))
				},
			},
		}
		for _, tt := range fields {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				ref := f.member(t, catalogID, tt.give).Type
				got, ok := pythonrules.Rules{}.ZeroValue(ref, f.view)
				assert.True(t, ok, "the type has a zero")
				assert.Equal(t, got, tt.want(ref), "the zero of the type")
			})
		}

		t.Run("returns the empty list of the target of an alias", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			alias, is := f.decl(t, tagsID).(*node.Alias)
			assert.True(t, is, "Tags is an alias")
			got, ok := pythonrules.Rules{}.ZeroValue(f.member(t, catalogID, "tags").Type, f.view)
			assert.True(t, ok, "a list has a zero")
			assert.Equal(t, got, emit.Composite(rules.EmitRef(alias.Target)), "the empty list of the target")
		})

		refusals := []struct {
			name string
			give *node.TypeRef
		}{
			{
				name: "reports false for a well-known type",
				give: &node.TypeRef{Spelling: "datetime", Target: timestampClass},
			},
			{name: "reports false for a class", give: &node.TypeRef{Spelling: "Order", Target: orderID}},
			{name: "reports false for a cycle of aliases", give: &node.TypeRef{Spelling: "Ping", Target: pingID}},
			{name: "reports false for a NewType without a base", give: &node.TypeRef{Spelling: "Bare", Target: bareID}},
			{name: "reports false for a type outside the view", give: &node.TypeRef{Spelling: "Gone", Target: goneID}},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, ok := pythonrules.Rules{}.ZeroValue(tt.give, f.view)
				assert.False(t, ok, "the type has no zero")
			})
		}
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		intRef := &node.TypeRef{Spelling: "int"}
		optional := &node.TypeRef{Form: symbol.FormOptional, Elems: []*node.TypeRef{intRef}}
		union := &node.TypeRef{Form: symbol.FormUnion, Elems: []*node.TypeRef{intRef, {Spelling: "str"}}}
		withNil := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: []*node.TypeRef{
			nil, {Spelling: `"a"`},
		}}
		onlyA := &node.TypeRef{
			Spelling: literalSpelling,
			Package:  typingModule,
			Args:     []*node.TypeRef{{Spelling: `"a"`}},
		}
		ofStatus := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: []*node.TypeRef{statusRef}}
		ofExpression := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: []*node.TypeRef{
			{Spelling: "x + 1"},
		}}
		ofBytes := &node.TypeRef{
			Spelling: literalSpelling,
			Package:  typingModule,
			Args:     []*node.TypeRef{{Spelling: `b"x"`}},
		}
		extendedA := &node.TypeRef{
			Spelling: literalSpelling,
			Package:  typingExtensions,
			Args:     []*node.TypeRef{{Spelling: `"a"`}},
		}
		bytearray := &node.TypeRef{Spelling: "bytearray"}
		built := []struct {
			name   string
			give   *node.TypeRef
			text   string
			want   emit.Value
			wantOK bool
		}{
			{
				name:   "reads an integer as an int",
				give:   intRef,
				text:   "42",
				want:   emit.Number(emit.LiteralInt, "42", 0),
				wantOK: true,
			},
			{
				name:   "reads an integer as a float",
				give:   &node.TypeRef{Spelling: "float"},
				text:   "42",
				want:   emit.Number(emit.LiteralFloat, "42", 64),
				wantOK: true,
			},
			{
				name:   "reads a float as a float",
				give:   &node.TypeRef{Spelling: "float"},
				text:   "1.5",
				want:   emit.Number(emit.LiteralFloat, "1.5", 64),
				wantOK: true,
			},
			{
				name:   "reads True as a bool",
				give:   &node.TypeRef{Spelling: "bool"},
				text:   "True",
				want:   emit.Literal(emit.LiteralBool, "true"),
				wantOK: true,
			},
			{
				name:   "reads a string as a str",
				give:   &node.TypeRef{Spelling: "str"},
				text:   `"x"`,
				want:   emit.Literal(emit.LiteralString, "x"),
				wantOK: true,
			},
			{
				name:   "reads bytes as bytes",
				give:   &node.TypeRef{Spelling: "bytes"},
				text:   `b"x"`,
				want:   emit.Raw(python.Lang, `b"x"`),
				wantOK: true,
			},
			{
				name:   "reads bytes as the conversion of a bytearray",
				give:   bytearray,
				text:   `b"x"`,
				want:   emit.Conversion(rules.EmitRef(bytearray), emit.Raw(python.Lang, `b"x"`)),
				wantOK: true,
			},
			{
				name:   "reads a value of a Literal of typing_extensions",
				give:   extendedA,
				text:   `"a"`,
				want:   emit.Literal(emit.LiteralString, "a"),
				wantOK: true,
			},
			{
				name:   "reads None as None",
				give:   &node.TypeRef{Spelling: "None"},
				text:   "None",
				want:   emit.Literal(emit.LiteralNil, ""),
				wantOK: true,
			},
			{
				name:   "reads an integer as the top type",
				give:   &node.TypeRef{Spelling: "object"},
				text:   "42",
				want:   emit.Number(emit.LiteralInt, "42", 0),
				wantOK: true,
			},
			{
				name:   "reads None as an optional",
				give:   optional,
				text:   "None",
				want:   emit.Literal(emit.LiteralNil, ""),
				wantOK: true,
			},
			{
				name:   "reads a value of the type of an optional",
				give:   optional,
				text:   "1",
				want:   emit.Number(emit.LiteralInt, "1", 0),
				wantOK: true,
			},
			{
				name:   "reads a value of the second member of a union",
				give:   union,
				text:   `"x"`,
				want:   emit.Literal(emit.LiteralString, "x"),
				wantOK: true,
			},
			{
				name:   "reads a value of a Literal",
				give:   withNil,
				text:   `"a"`,
				want:   emit.Literal(emit.LiteralString, "a"),
				wantOK: true,
			},
			{
				name:   "reads a member of a Literal of an enum",
				give:   ofStatus,
				text:   "Status.NEW",
				want:   emit.Name(newID),
				wantOK: true,
			},
			{name: "reads no float as an int", give: intRef, text: "1.5"},
			{name: "reads no name as the top type", give: &node.TypeRef{Spelling: "object"}, text: nameText},
			{name: "reads no value of no member of a union", give: union, text: "1.5"},
			{name: "reads no other value of a Literal", give: onlyA, text: `"b"`},
			{name: "reads no member of another enum of a Literal of an enum", give: ofStatus, text: "Color.NEW"},
			{name: "reads no value of a Literal of an argument that is no literal", give: ofExpression, text: "1"},
			{name: "reads no string as a value of a Literal of bytes", give: ofBytes, text: `"x"`},
			{
				name: "reads no value of a tuple",
				give: &node.TypeRef{Spelling: "tuple[int]", Form: symbol.FormTuple},
				text: "1",
			},
			{name: "reads no text that is no literal", give: intRef, text: "1 +"},
		}
		for _, tt := range built {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := pythonrules.Rules{}.LiteralFor(rules.Scope{}, tt.give, tt.text, rules.View{})
				expect.Equal(t, ok, tt.wantOK, "whether the text is a value of the type")
				expect.Equal(t, got, tt.want, "the value of the text")
			})
		}

		status := &node.TypeRef{Spelling: "Status", Target: statusID}
		ofPaid := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: []*node.TypeRef{
			{Spelling: "Status.PAID", Target: paidID},
		}}
		fixtures := []struct {
			name   string
			give   *node.TypeRef
			text   string
			want   emit.Value
			wantOK bool
		}{
			{
				name:   "reads the name of a member as a value of its enum",
				give:   status,
				text:   "NEW",
				want:   emit.Name(newID),
				wantOK: true,
			},
			{
				name:   "reads a qualified name of a member as a value of its enum",
				give:   status,
				text:   "Status.PAID",
				want:   emit.Name(paidID),
				wantOK: true,
			},
			{
				name:   "reads a name of a member through its module as a value of its enum",
				give:   status,
				text:   "models.Status.NEW",
				want:   emit.Name(newID),
				wantOK: true,
			},
			{
				name:   "reads a value of the Literal of an alias",
				give:   &node.TypeRef{Spelling: "Mode", Target: modeID},
				text:   `"fast"`,
				want:   emit.Literal(emit.LiteralString, "fast"),
				wantOK: true,
			},
			{
				name:   "reads a member of a Literal of the members of an enum",
				give:   ofPaid,
				text:   "PAID",
				want:   emit.Name(paidID),
				wantOK: true,
			},
			{
				name: "reads an integer as an int of typeshed",
				give: &node.TypeRef{Spelling: "int", Target: symbol.Identity{
					Lang: python.Lang, Package: builtinsModule, Name: "int", Kind: symbol.KindStruct,
				}},
				text:   "42",
				want:   emit.Number(emit.LiteralInt, "42", 0),
				wantOK: true,
			},
			{name: "reads no member of another enum", give: status, text: "Color.RED"},
			{name: "reads no name that no member has", give: status, text: "OTHER"},
			{name: "reads no integer as a value of an enum", give: status, text: "1"},
			{
				name: "reads no text through a cycle of aliases",
				give: &node.TypeRef{Spelling: "Ping", Target: pingID},
				text: "1",
			},
			{
				name: "reads no integer as a well-known type",
				give: &node.TypeRef{Spelling: "datetime", Target: timestampClass},
				text: "1",
			},
			{name: "reads no value of a class", give: &node.TypeRef{Spelling: "Order", Target: orderID}, text: "1"},
			{
				name: "reads no value of a type outside the view",
				give: &node.TypeRef{Spelling: "Gone", Target: goneID},
				text: "1",
			},
		}
		for _, tt := range fixtures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, ok := pythonrules.Rules{}.LiteralFor(rules.Scope{}, tt.give, tt.text, f.view)
				expect.Equal(t, ok, tt.wantOK, "whether the text is a value of the type")
				expect.Equal(t, got, tt.want, "the value of the text")
			})
		}

		t.Run("reads a value of the base of a NewType as its conversion", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			ref := &node.TypeRef{Spelling: "UserId", Target: userID}
			got, ok := pythonrules.Rules{}.LiteralFor(rules.Scope{}, ref, "42", f.view)
			assert.True(t, ok, "an integer is a value of the base")
			assert.Equal(t, got, emit.Conversion(rules.EmitRef(ref), emit.Number(emit.LiteralInt, "42", 0)),
				"the conversion of the integer")
			_, ok = pythonrules.Rules{}.LiteralFor(rules.Scope{}, ref, `"x"`, f.view)
			assert.False(t, ok, "a string is no value of the base")
		})
	})
}

// TestValuesAllocs checks the ceilings of the methods of values.go in the
// ordinary run, which does not run benchmarks.
func TestValuesAllocs(t *testing.T) {
	checkAllocs(t, valuesCalls(t))
}

// BenchmarkValues measures each method of values.go under its ceilings.
func BenchmarkValues(b *testing.B) {
	benchCalls(b, valuesCalls(b))
}

// valuesCalls returns a call of each method of values.go, and the pairs of a
// string and of a class.
func valuesCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := pythonrules.Rules{}
	intRef := &node.TypeRef{Spelling: "int"}
	strRef := &node.TypeRef{Spelling: "str"}
	item := &node.TypeRef{Spelling: "Item", Target: itemID}
	var (
		sample rules.Sample
		value  emit.Value
		ok     bool
	)
	return []allocCall{
		{
			name: "DeriveSamples", caseName: "an int", allocs: valuesAllocs,
			call:  func() { sample, _ = r.DeriveSamples(intRef, "", rules.View{}) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "an int derives") },
		},
		{
			name: "DeriveSamples", caseName: "a str", allocs: textPairAllocs,
			call:  func() { sample, _ = r.DeriveSamples(strRef, "name", rules.View{}) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "a str derives") },
		},
		{
			name: "DeriveSamples", caseName: "a dataclass", allocs: classPairAllocs,
			call:  func() { sample, _ = r.DeriveSamples(item, "", f.view) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "Item derives") },
		},
		{
			name: "ZeroValue", allocs: valuesAllocs,
			call:  func() { value, ok = r.ZeroValue(intRef, rules.View{}) },
			check: func(tb assert.TB) { assert.True(tb, ok, "an int has a zero") },
		},
		{
			name: "LiteralFor", allocs: valuesAllocs,
			call: func() { value, ok = r.LiteralFor(rules.Scope{}, intRef, decimalText, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value, emit.Number(emit.LiteralInt, decimalText, 0), "the integer")
			},
		},
	}
}
