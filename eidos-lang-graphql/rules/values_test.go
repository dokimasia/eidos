// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The values that the rules derive, pinned: the numbers, the truth values,
// the hint of a string where the caller names none with the suffixes of its
// two halves, and the widths.
const (
	derivedInt      = "42"
	derivedAltInt   = "7"
	derivedFloat    = "1.5"
	derivedAltFloat = "2.5"
	zeroText        = "0"
	trueText        = "true"
	falseText       = "false"
	defaultHint     = "sample"
	sampleSuffix    = "-a"
	alternateSuffix = "-b"
	bits32          = 32
	bits64          = 64
)

// fixturePlugin is the plugin that the authored stamps of the cases claim.
// An author states a string as its content, and a value of another type as
// a literal of GraphQL.
const (
	fixturePlugin    = "fixture"
	authoredTitle    = "Dune"
	authoredAltTitle = "Emma"
	authoredStatus   = "PUBLISHED"
	authoredAlt      = "ARCHIVED"
)

// The ceilings of the values. A pair of Author allocates the reference to
// the object type, the field list of each composite and the hinted texts of
// its first field, an ID. A pair of Status allocates the reference and the
// conversion of each value. The empty list allocates the reference to the
// list, its list of children and its child. A literal of an Int allocates
// the source of the lexer and the parsed value. A literal of AuthorFilter
// allocates the source, two parsed values, the parsed entry and its list,
// the reference to the input object and the field list of the composite.
const (
	objectSamplesAllocs = 1 + 2 + 2
	enumSamplesAllocs   = 1 + 2
	listZeroAllocs      = 1 + 1 + 1
	intLiteralAllocs    = 1 + 1
	inputLiteralAllocs  = 1 + 2 + 2 + 1 + 1
)

// The references to the declarations of the fixture tree and to the
// built-in scalars that the cases read.
var (
	authorRef = &node.TypeRef{
		Spelling: "Author",
		Target:   symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Author", Kind: symbol.KindStruct},
	}
	bookRef   = &node.TypeRef{Spelling: "Book", Target: bookID}
	statusRef = &node.TypeRef{Spelling: "Status", Target: statusID}
	singleRef = &node.TypeRef{
		Spelling: "Single",
		Target:   symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Single", Kind: symbol.KindEnum},
	}
	searchRef = &node.TypeRef{
		Spelling: "SearchResult",
		Target:   symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "SearchResult", Kind: symbol.KindAlias},
	}
	nodeRef = &node.TypeRef{
		Spelling: "Node",
		Target:   symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Node", Kind: symbol.KindInterface},
	}
	authorFilterRef = &node.TypeRef{
		Spelling: "AuthorFilter",
		Target:   symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "AuthorFilter", Kind: symbol.KindStruct},
	}
	bookFilterRef = &node.TypeRef{
		Spelling: "BookFilter",
		Target:   symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "BookFilter", Kind: symbol.KindStruct},
	}
	resultRef = &node.TypeRef{
		Spelling: "search",
		Target: symbol.Identity{
			Lang: graphql.Lang, Package: docsPkg, Owner: "SearchQuery", Name: "search", Kind: symbol.KindSum,
		},
	}
	attachmentRef = &node.TypeRef{
		Spelling: "Attachment",
		Target:   symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Attachment", Kind: symbol.KindAlias},
	}
	mediaRef = &node.TypeRef{
		Spelling: "attachment",
		Target: symbol.Identity{
			Lang: graphql.Lang, Package: docsPkg, Owner: "GetAttachmentQuery", Name: "attachment",
			Kind: symbol.KindSum,
		},
	}
	longRef   = &node.TypeRef{Spelling: "Long", Target: longID}
	base64Ref = &node.TypeRef{Spelling: "Base64String", Target: base64ID}
	dateRef   = &node.TypeRef{Spelling: "DateTime", Target: dateID}
	uploadRef = &node.TypeRef{Spelling: "Upload", Target: uploadID}
	ghostRef  = &node.TypeRef{
		Spelling: "Ghost",
		Target:   symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Ghost", Kind: symbol.KindStruct},
	}
	intRef      = &node.TypeRef{Spelling: graphql.IntScalar}
	floatRef    = &node.TypeRef{Spelling: graphql.FloatScalar}
	stringRef   = &node.TypeRef{Spelling: graphql.StringScalar}
	idRef       = &node.TypeRef{Spelling: graphql.IDScalar}
	booleanRef  = &node.TypeRef{Spelling: graphql.BooleanScalar}
	unknownRef  = &node.TypeRef{Spelling: "Unknown"}
	optionalInt = &node.TypeRef{Spelling: "Int", Form: symbol.FormOptional, Elems: []*node.TypeRef{intRef}}
	intList     = &node.TypeRef{Spelling: "[Int!]", Form: symbol.FormList, Elems: []*node.TypeRef{intRef}}
	stringList  = &node.TypeRef{Spelling: "[String!]", Form: symbol.FormList, Elems: []*node.TypeRef{stringRef}}
	ghostList   = &node.TypeRef{Spelling: "[Ghost!]", Form: symbol.FormList, Elems: []*node.TypeRef{ghostRef}}
	statusList  = &node.TypeRef{Spelling: "[Status!]", Form: symbol.FormList, Elems: []*node.TypeRef{statusRef}}
	bookStream  = &node.TypeRef{Spelling: "stream Book", Form: symbol.FormStream, Elems: []*node.TypeRef{bookRef}}
)

// TestValues pins the samples, the zeros and the literals of each type of
// GraphQL. A check generator writes these values.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("DeriveSamples", func(t *testing.T) {
		t.Parallel()

		deep := intRef
		for range 10 {
			deep = &node.TypeRef{Spelling: "Int", Form: symbol.FormOptional, Elems: []*node.TypeRef{deep}}
		}
		tests := []struct {
			name          string
			give          *node.TypeRef
			giveHint      string
			wantSample    emit.Value
			wantAlternate emit.Value
		}{
			{
				name:          "returns integers at 32 bits for Int",
				give:          intRef,
				wantSample:    emit.Number(emit.LiteralInt, derivedInt, bits32),
				wantAlternate: emit.Number(emit.LiteralInt, derivedAltInt, bits32),
			},
			{
				name:          "returns floats at 64 bits for Float",
				give:          floatRef,
				wantSample:    emit.Number(emit.LiteralFloat, derivedFloat, bits64),
				wantAlternate: emit.Number(emit.LiteralFloat, derivedAltFloat, bits64),
			},
			{
				name:          "returns true beside false for Boolean",
				give:          booleanRef,
				wantSample:    emit.Literal(emit.LiteralBool, trueText),
				wantAlternate: emit.Literal(emit.LiteralBool, falseText),
			},
			{
				name:          "returns the hint with a suffix for each half of a String",
				give:          stringRef,
				giveHint:      "title",
				wantSample:    emit.Literal(emit.LiteralString, "title"+sampleSuffix),
				wantAlternate: emit.Literal(emit.LiteralString, "title"+alternateSuffix),
			},
			{
				name:          "returns the default hint with a suffix for each half of an ID",
				give:          idRef,
				wantSample:    emit.Literal(emit.LiteralString, defaultHint+sampleSuffix),
				wantAlternate: emit.Literal(emit.LiteralString, defaultHint+alternateSuffix),
			},
			{
				name:          "returns the pair of the type of a nullable type",
				give:          optionalInt,
				wantSample:    emit.Number(emit.LiteralInt, derivedInt, bits32),
				wantAlternate: emit.Number(emit.LiteralInt, derivedAltInt, bits32),
			},
			{
				name:     "returns lists of one element that differ in the element",
				give:     stringList,
				giveHint: "tag",
				wantSample: emit.Composite(rules.EmitRef(stringList),
					emit.Element(emit.Literal(emit.LiteralString, "tag"+sampleSuffix))),
				wantAlternate: emit.Composite(rules.EmitRef(stringList),
					emit.Element(emit.Literal(emit.LiteralString, "tag"+alternateSuffix))),
			},
			{
				name: "returns composites that set the first field of an object type",
				give: authorRef,
				wantSample: emit.Composite(rules.EmitRef(authorRef),
					emit.NamedField("id", emit.Literal(emit.LiteralString, "id"+sampleSuffix))),
				wantAlternate: emit.Composite(rules.EmitRef(authorRef),
					emit.NamedField("id", emit.Literal(emit.LiteralString, "id"+alternateSuffix))),
			},
			{
				name: "returns composites that set the first field of an input object",
				give: authorFilterRef,
				wantSample: emit.Composite(rules.EmitRef(authorFilterRef),
					emit.NamedField("name", emit.Literal(emit.LiteralString, "name"+sampleSuffix))),
				wantAlternate: emit.Composite(rules.EmitRef(authorFilterRef),
					emit.NamedField("name", emit.Literal(emit.LiteralString, "name"+alternateSuffix))),
			},
			{
				name:          "returns the first two values of an enum converted to the enum",
				give:          statusRef,
				wantSample:    emit.Conversion(rules.EmitRef(statusRef), emit.Literal(emit.LiteralString, "DRAFT")),
				wantAlternate: emit.Conversion(rules.EmitRef(statusRef), emit.Literal(emit.LiteralString, "PUBLISHED")),
			},
			{
				name: "returns the pair of the first member of a union",
				give: searchRef,
				wantSample: emit.Composite(rules.EmitRef(bookRef),
					emit.NamedField("id", emit.Literal(emit.LiteralString, "id"+sampleSuffix))),
				wantAlternate: emit.Composite(rules.EmitRef(bookRef),
					emit.NamedField("id", emit.Literal(emit.LiteralString, "id"+alternateSuffix))),
			},
			{
				name: "returns composites that set a field of the first variant of a sum",
				give: resultRef,
				wantSample: emit.Composite(rules.EmitRef(resultRef),
					emit.NamedField("title", emit.Literal(emit.LiteralString, "title"+sampleSuffix))),
				wantAlternate: emit.Composite(rules.EmitRef(resultRef),
					emit.NamedField("title", emit.Literal(emit.LiteralString, "title"+alternateSuffix))),
			},
			{
				name:          "returns integers at 64 bits for the scalar of a long",
				give:          longRef,
				wantSample:    emit.Number(emit.LiteralInt, derivedInt, bits64),
				wantAlternate: emit.Number(emit.LiteralInt, derivedAltInt, bits64),
			},
			{
				name:          "returns the hinted strings for the scalar of a base64 string",
				give:          base64Ref,
				giveHint:      "cover",
				wantSample:    emit.Literal(emit.LiteralString, "cover"+sampleSuffix),
				wantAlternate: emit.Literal(emit.LiteralString, "cover"+alternateSuffix),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, a := graphqlrules.New().DeriveSamples(tt.give, tt.giveHint, loaded(t).view)
				assert.True(t, s.OK(), "the sample derives")
				assert.True(t, a.OK(), "the alternate derives")
				assert.Equal(t, s.Value, tt.wantSample, "the sample")
				assert.Equal(t, a.Value, tt.wantAlternate, "the alternate")
			})
		}

		refusals := []struct {
			name string
			give *node.TypeRef
			want rules.Refusal
		}{
			{
				name: "returns RefusedUnresolved for a spelling that names no built-in scalar",
				give: unknownRef,
				want: rules.RefusedUnresolved,
			},
			{
				name: "returns RefusedUnresolved for a type that the view does not contain",
				give: ghostRef,
				want: rules.RefusedUnresolved,
			},
			{name: "returns the refusal of the element of a list", give: ghostList, want: rules.RefusedUnresolved},
			{name: "returns RefusedNoLiteral for a stream", give: bookStream, want: rules.RefusedNoLiteral},
			{name: "returns RefusedNoLiteral for a nil reference", want: rules.RefusedNoLiteral},
			{name: "returns RefusedNoLiteral for an enum of one value", give: singleRef, want: rules.RefusedNoLiteral},
			{name: "returns RefusedNoLiteral for an interface", give: nodeRef, want: rules.RefusedNoLiteral},
			{
				name: "returns RefusedNoLiteral for a scalar of a well-known shape",
				give: dateRef,
				want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a custom scalar outside the table",
				give: uploadRef,
				want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a union of members without a pair",
				give: attachmentRef,
				want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a sum of variants without a pair",
				give: mediaRef,
				want: rules.RefusedNoLiteral,
			},
			{name: "returns RefusedDepth for a type past the budget of the walk", give: deep, want: rules.RefusedDepth},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, a := graphqlrules.New().DeriveSamples(tt.give, "", loaded(t).view)
				assert.Equal(t, s.Refusal, tt.want, "the refusal of the sample")
				assert.Equal(t, a.Refusal, tt.want, "the refusal of the alternate")
			})
		}

		t.Run("returns the values authored on a field for an object type", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			id := symbol.Identity{
				Lang:    graphql.Lang,
				Package: shopPkg,
				Owner:   "Book",
				Name:    "id",
				Kind:    symbol.KindField,
			}
			stampOn(t, f, f.Keys.Sample, id, authoredTitle)
			stampOn(t, f, f.Keys.Alternate, id, authoredAltTitle)
			s, a := graphqlrules.New().DeriveSamples(bookRef, "", f.view)
			assert.Equal(t, s.Value, emit.Composite(rules.EmitRef(bookRef),
				emit.NamedField("id", emit.Literal(emit.LiteralString, authoredTitle))),
				"the sample sets the authored id")
			assert.Equal(t, a.Value, emit.Composite(rules.EmitRef(bookRef),
				emit.NamedField("id", emit.Literal(emit.LiteralString, authoredAltTitle))),
				"the alternate sets the authored id")
		})

		t.Run("returns the values authored on the type of the element of a list", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			stampOn(t, f, f.Keys.Sample, statusID, authoredStatus)
			stampOn(t, f, f.Keys.Alternate, statusID, authoredAlt)
			s, a := graphqlrules.New().DeriveSamples(statusList, "", f.view)
			assert.Equal(t, s.Value, emit.Composite(rules.EmitRef(statusList), emit.Element(
				emit.Conversion(rules.EmitRef(statusRef), emit.Literal(emit.LiteralString, authoredStatus)))),
				"the element takes the value that the author stated on Status")
			assert.Equal(t, a.Value, emit.Composite(rules.EmitRef(statusList), emit.Element(
				emit.Conversion(rules.EmitRef(statusRef), emit.Literal(emit.LiteralString, authoredAlt)))),
				"the element takes the alternate that the author stated on Status")
		})
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		zeros := []struct {
			name string
			give *node.TypeRef
			want emit.Value
		}{
			{name: "returns null for a nullable type", give: optionalInt, want: emit.Literal(emit.LiteralNil, "")},
			{name: "returns the empty list for a list", give: intList, want: emit.Composite(rules.EmitRef(intList))},
			{name: "returns 0 at 32 bits for Int", give: intRef, want: emit.Number(emit.LiteralInt, zeroText, bits32)},
			{
				name: "returns 0 at 64 bits for Float",
				give: floatRef,
				want: emit.Number(emit.LiteralFloat, zeroText, bits64),
			},
			{name: "returns false for Boolean", give: booleanRef, want: emit.Literal(emit.LiteralBool, falseText)},
			{name: "returns the empty string for String", give: stringRef, want: emit.Literal(emit.LiteralString, "")},
			{name: "returns the empty string for ID", give: idRef, want: emit.Literal(emit.LiteralString, "")},
			{
				name: "returns 0 at 64 bits for the scalar of a long",
				give: longRef,
				want: emit.Number(emit.LiteralInt, zeroText, bits64),
			},
			{
				name: "returns the empty string for the scalar of a base64 string",
				give: base64Ref,
				want: emit.Literal(emit.LiteralString, ""),
			},
		}
		for _, tt := range zeros {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, exists := graphqlrules.New().ZeroValue(tt.give, loaded(t).view)
				assert.True(t, exists, "the type has a zero")
				assert.Equal(t, got, tt.want, "the zero of the type")
			})
		}

		none := []struct {
			name string
			give *node.TypeRef
		}{
			{name: "reports false for a nil reference"},
			{name: "reports false for a stream", give: bookStream},
			{name: "reports false for a spelling that names no built-in scalar", give: unknownRef},
			{name: "reports false for an object type", give: bookRef},
			{name: "reports false for an enum", give: statusRef},
			{name: "reports false for a union", give: searchRef},
			{name: "reports false for a scalar of a well-known shape", give: dateRef},
			{name: "reports false for a custom scalar outside the table", give: uploadRef},
			{name: "reports false for a type that the view does not contain", give: ghostRef},
		}
		for _, tt := range none {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, exists := graphqlrules.New().ZeroValue(tt.give, loaded(t).view)
				assert.False(t, exists, "GraphQL gives the type no zero")
			})
		}
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give *node.TypeRef
			text string
			want emit.Value
		}{
			{
				name: "returns an integer at 32 bits for Int",
				give: intRef,
				text: "-7",
				want: emit.Number(emit.LiteralInt, "-7", bits32),
			},
			{
				name: "returns an integer of Float as a float",
				give: floatRef,
				text: "1",
				want: emit.Number(emit.LiteralFloat, "1", bits64),
			},
			{
				name: "returns the shortest text of a float",
				give: floatRef,
				text: "1.5e3",
				want: emit.Number(emit.LiteralFloat, "1500", bits64),
			},
			{
				name: "returns a string for String",
				give: stringRef,
				text: `"a"`,
				want: emit.Literal(emit.LiteralString, "a"),
			},
			{
				name: "returns a block string for String",
				give: stringRef,
				text: `"""b"""`,
				want: emit.Literal(emit.LiteralString, "b"),
			},
			{name: "returns a string for ID", give: idRef, text: `"x"`, want: emit.Literal(emit.LiteralString, "x")},
			{
				name: "returns an integer of ID as its text",
				give: idRef,
				text: "7",
				want: emit.Literal(emit.LiteralString, "7"),
			},
			{
				name: "returns true for Boolean",
				give: booleanRef,
				text: "true",
				want: emit.Literal(emit.LiteralBool, trueText),
			},
			{
				name: "returns null for a nullable type",
				give: optionalInt,
				text: "null",
				want: emit.Literal(emit.LiteralNil, ""),
			},
			{
				name: "returns the value of the type of a nullable type",
				give: optionalInt,
				text: "5",
				want: emit.Number(emit.LiteralInt, "5", bits32),
			},
			{
				name: "returns a list of its items",
				give: intList,
				text: "[1, 2]",
				want: emit.Composite(rules.EmitRef(intList),
					emit.Element(emit.Number(emit.LiteralInt, "1", bits32)),
					emit.Element(emit.Number(emit.LiteralInt, "2", bits32))),
			},
			{
				name: "returns a single value of a list as a list of one item",
				give: intList,
				text: "3",
				want: emit.Composite(rules.EmitRef(intList), emit.Element(emit.Number(emit.LiteralInt, "3", bits32))),
			},
			{
				name: "returns a value of an enum converted to the enum",
				give: statusRef,
				text: "PUBLISHED",
				want: emit.Conversion(rules.EmitRef(statusRef), emit.Literal(emit.LiteralString, "PUBLISHED")),
			},
			{
				name: "returns an object of an input object",
				give: authorFilterRef,
				text: `{name: "Ada"}`,
				want: emit.Composite(rules.EmitRef(authorFilterRef),
					emit.NamedField("name", emit.Literal(emit.LiteralString, "Ada"))),
			},
			{
				name: "returns an empty object of an input object without a required field",
				give: bookFilterRef,
				text: "{}",
				want: emit.Composite(rules.EmitRef(bookFilterRef)),
			},
			{
				name: "returns an integer at 64 bits for the scalar of a long",
				give: longRef,
				text: "9007199254740993",
				want: emit.Number(emit.LiteralInt, "9007199254740993", bits64),
			},
			{
				name: "returns a string for the scalar of a base64 string",
				give: base64Ref,
				text: `"aGk="`,
				want: emit.Literal(emit.LiteralString, "aGk="),
			},
			{
				name: "returns a literal of a custom scalar outside the table as its own kind",
				give: uploadRef,
				text: "1",
				want: emit.Literal(emit.LiteralInt, "1"),
			},
			{
				name: "returns a literal of a spelling that names no built-in scalar as its own kind",
				give: unknownRef,
				text: "1.5",
				want: emit.Number(emit.LiteralFloat, "1.5", 0),
			},
			{
				name: "returns a string of a type that the view does not contain as its own kind",
				give: ghostRef,
				text: `"s"`,
				want: emit.Literal(emit.LiteralString, "s"),
			},
			{
				name: "returns a truth value without a type as its own kind",
				text: "false",
				want: emit.Literal(emit.LiteralBool, falseText),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, valid := graphqlrules.New().LiteralFor(rules.Scope{}, tt.give, tt.text, loaded(t).view)
				assert.True(t, valid, "the text is a value of the type")
				assert.Equal(t, got, tt.want, "the value", assert.EquateEmpty())
			})
		}

		invalid := []struct {
			name string
			give *node.TypeRef
			text string
		}{
			{name: "reports false for an integer outside 32 bits", give: intRef, text: "2147483648"},
			{name: "reports false for a float of Int", give: intRef, text: "1.5"},
			{name: "reports false for an integer of String", give: stringRef, text: "1"},
			{name: "reports false for a truth value of ID", give: idRef, text: "true"},
			{name: "reports false for an integer of Boolean", give: booleanRef, text: "1"},
			{name: "reports false for null of a type that is not nullable", give: intRef, text: "null"},
			{name: "reports false for null of a list", give: intList, text: "null"},
			{name: "reports false for a list with an item of another type", give: intList, text: `[1, "a"]`},
			{name: "reports false for a name that the enum does not declare", give: statusRef, text: "GHOST"},
			{name: "reports false for a string of an enum", give: statusRef, text: `"PUBLISHED"`},
			{name: "reports false for an object without a required field", give: authorFilterRef, text: "{}"},
			{
				name: "reports false for an object with a field that the input object does not declare",
				give: authorFilterRef,
				text: `{name: "Ada", ghost: 1}`,
			},
			{
				name: "reports false for an object with a repeated field",
				give: authorFilterRef,
				text: `{name: "a", name: "b"}`,
			},
			{
				name: "reports false for an object with a field of another type",
				give: authorFilterRef,
				text: "{name: 1}",
			},
			{name: "reports false for an object of an object type", give: bookRef, text: `{id: "1"}`},
			{name: "reports false for a string of an input object", give: authorFilterRef, text: `"Ada"`},
			{name: "reports false for a literal of a union", give: searchRef, text: "{}"},
			{name: "reports false for a literal of an interface", give: nodeRef, text: "{}"},
			{name: "reports false for a float of the scalar of a long", give: longRef, text: "1.5"},
			{
				name: "reports false for a literal of a scalar of a well-known shape",
				give: dateRef,
				text: `"2026-10-19T00:00:00Z"`,
			},
			{name: "reports false for a literal of a stream", give: bookStream, text: "{}"},
			{name: "reports false for an enum value without a type", give: unknownRef, text: "RED"},
			{name: "reports false for text that does not parse", give: intRef, text: "$id"},
		}
		for _, tt := range invalid {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, valid := graphqlrules.New().LiteralFor(rules.Scope{}, tt.give, tt.text, loaded(t).view)
				assert.False(t, valid, "the text is no value of the type")
			})
		}
	})
}

// TestValuesAllocs checks the ceilings of the values in the ordinary run,
// which does not run benchmarks.
func TestValuesAllocs(t *testing.T) {
	checkAllocs(t, valuesCalls(t))
}

// BenchmarkValues measures the samples, the zeros and the literals that a
// check generator asks for each field.
func BenchmarkValues(b *testing.B) {
	benchCalls(b, valuesCalls(b))
}

// valuesCalls returns the calls of the methods of values.go that
// TestValuesAllocs and BenchmarkValues measure.
func valuesCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := graphqlrules.New()
	var (
		sample, alternate rules.Sample
		zero, lit         emit.Value
		valid             bool
	)
	return []allocCall{
		{
			name: "DeriveSamples", caseName: "an object type", allocs: objectSamplesAllocs,
			call: func() { sample, alternate = r.DeriveSamples(authorRef, "", f.view) },
			check: func(tb assert.TB) {
				assert.True(tb, sample.OK(), "DeriveSamples derives the sample of Author")
				assert.True(tb, alternate.OK(), "DeriveSamples derives the alternate of Author")
			},
		},
		{
			name: "DeriveSamples", caseName: "an enum", allocs: enumSamplesAllocs,
			call: func() { sample, alternate = r.DeriveSamples(statusRef, "", f.view) },
			check: func(tb assert.TB) {
				assert.True(tb, sample.OK(), "DeriveSamples derives the sample of Status")
				assert.True(tb, alternate.OK(), "DeriveSamples derives the alternate of Status")
			},
		},
		{
			name: "ZeroValue", caseName: "a list", allocs: listZeroAllocs,
			call:  func() { zero, valid = r.ZeroValue(intList, f.view) },
			check: func(tb assert.TB) { assert.True(tb, valid, "ZeroValue returns the empty list") },
		},
		{
			name: "ZeroValue", caseName: "an Int",
			call:  func() { zero, valid = r.ZeroValue(intRef, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, zero.Text, zeroText, "ZeroValue returns 0") },
		},
		{
			name: "LiteralFor", caseName: "an Int", allocs: intLiteralAllocs,
			call:  func() { lit, valid = r.LiteralFor(rules.Scope{}, intRef, derivedInt, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, lit.Text, derivedInt, "LiteralFor returns 42") },
		},
		{
			name: "LiteralFor", caseName: "an input object", allocs: inputLiteralAllocs,
			call:  func() { lit, valid = r.LiteralFor(rules.Scope{}, authorFilterRef, `{name: "Ada"}`, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, lit.Fields, 1, "LiteralFor returns the object") },
		},
	}
}

// stampOn stamps the value that an author states for subject under the key
// k of the kernel, as a plugin of the fixture.
func stampOn(tb assert.TB, f *fixture, k meta.Key[string], subject symbol.Identity, text string) {
	tb.Helper()

	err := meta.Stamp(f.Facts, k, text, meta.Claim{
		Subject: subject, Authority: meta.AuthorityPlugin, Plugin: fixturePlugin,
	})
	assert.NoError(tb, err, "the author states a value")
}
