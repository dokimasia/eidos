// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// wireName is a name that is not an identifier, such as a header that a
// field maps, and constantName the name of a constant of a class.
const (
	wireName     = "content-type"
	constantName = "MAX_LINES"
)

// constantMember is a field at the level of the type of a class, whose name
// in UPPER_SNAKE_CASE is a constant of the class.
var constantMember = plugin.Member{Host: symbol.KindStruct, Kind: symbol.KindField, Level: symbol.LevelType}

// The allocations of the names that the allocation test and the benchmark
// measure.
const (
	// conversionAllocs is a name that the convention converts: the
	// converted spelling.
	conversionAllocs = 1
	// underscoreAllocs is a converted name that takes an underscore: the
	// converted spelling and the spelling with its underscore.
	underscoreAllocs = 2
)

// nameCall is one name that the allocation test and the benchmark measure:
// the arguments of the call, its ceiling and the spelling that it returns.
type nameCall struct {
	name   string
	host   symbol.Kind
	kind   symbol.Kind
	vis    symbol.Visibility
	give   string
	allocs uint64
	want   string
}

// The settle spells every declared name through the convention. Each
// mapping, and each name that Python refuses, is pinned byte for byte.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		type spelling struct {
			name string
			host symbol.Kind
			kind symbol.Kind
			vis  symbol.Visibility
			give string
			want string
		}
		// The kinds whose name passes through where it is not an identifier.
		passing := []symbol.Kind{symbol.KindField, symbol.KindEnumVariant}
		tests := slices.Grow([]spelling{
			{
				name: "returns the CapWords spelling of a class",
				kind: symbol.KindStruct, give: "order_item", want: "OrderItem",
			},
			{
				name: "returns a class with an acronym in capitals unchanged",
				kind: symbol.KindStruct, give: "HTTPServerError", want: "HTTPServerError",
			},
			{
				name: "returns a class with an acronym as a word unchanged",
				kind: symbol.KindInterface, give: "HttpClient", want: "HttpClient",
			},
			{
				name: "returns the snake-case spelling of a method",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "fetchRow", want: "fetch_row",
			},
			{
				name: "returns the snake-case spelling of a function",
				kind: symbol.KindFunction, vis: symbol.VisibilityPublic, give: "GetUser", want: "get_user",
			},
			{
				name: "returns the snake-case spelling of a parameter",
				host: symbol.KindFunction, kind: symbol.KindParam, give: "userID", want: "user_id",
			},
			{
				name: "returns the snake-case spelling of a field",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: "CreatedAt", want: "created_at",
			},
			{
				name: "returns the upper snake case of a constant",
				kind: symbol.KindConstant, vis: symbol.VisibilityPublic, give: "maxItems", want: "MAX_ITEMS",
			},
			{
				name: "returns the upper snake case of a member of an enum",
				host: symbol.KindEnum, kind: symbol.KindEnumVariant, give: "rowOpen", want: "ROW_OPEN",
			},
			{
				name: "returns the CapWords spelling of a type parameter",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "key", want: "Key",
			},
			{
				name: "returns a type parameter of one capital unchanged",
				host: symbol.KindFunction, kind: symbol.KindTypeParam, give: "T", want: "T",
			},
			{
				name: "returns a type variable tuple with its star unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "*Ts", want: "*Ts",
			},
			{
				name: "returns a parameter specification with its two stars unchanged",
				host: symbol.KindFunction, kind: symbol.KindTypeParam, give: "**P", want: "**P",
			},
			{
				name: "returns the CapWords spelling of a type variable tuple after its star",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "*key_types", want: "*KeyTypes",
			},
			{
				name: "returns a covariant type parameter with its suffix unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "T_co", want: "T_co",
			},
			{
				name: "returns a contravariant type parameter with its suffix unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "T_contra", want: "T_contra",
			},
			{
				name: "returns the CapWords spelling of a type parameter before its suffix of variance",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "item_key_co", want: "ItemKey_co",
			},
			{
				name: "returns the CapWords spelling of a variant of a sum",
				host: symbol.KindSum, kind: symbol.KindSumVariant, give: "card_payment", want: "CardPayment",
			},
			{
				name: "returns the snake-case spelling of a variable",
				kind: symbol.KindVariable,
				vis:  symbol.VisibilityPublic,
				give: "defaultTimeout",
				want: "default_timeout",
			},
			{
				name: "returns a variable in upper snake case unchanged",
				kind: symbol.KindVariable, vis: symbol.VisibilityPublic, give: "MAX_SIZE", want: "MAX_SIZE",
			},
			{
				name: "returns a variable in CapWords unchanged",
				kind: symbol.KindVariable, vis: symbol.VisibilityPublic, give: "UserId", want: "UserId",
			},
			{
				name: "returns the snake-case spelling of a capitalized variable with underscores",
				kind: symbol.KindVariable, vis: symbol.VisibilityPublic, give: "Default_client", want: "default_client",
			},
			{
				name: "returns a special name unchanged",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityPublic,
				give: "__init__", want: "__init__",
			},
			{
				name: "returns a name of underscores alone unchanged",
				host: symbol.KindFunction, kind: symbol.KindParam, give: "_", want: "_",
			},
			{
				name: "returns the trailing underscore of a name unchanged",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: "type_", want: "type_",
			},
			{
				name: "returns a keyword with a trailing underscore",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: "class", want: "class_",
			},
			{
				name: "returns a class named after a keyword with a trailing underscore",
				kind: symbol.KindStruct, vis: symbol.VisibilityPublic, give: "none", want: "None_",
			},
			{
				name: "returns a soft keyword unchanged",
				host: symbol.KindFunction, kind: symbol.KindParam, give: "match", want: "match",
			},
			{
				name: "returns one underscore before a protected member",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityProtected,
				give: "cache", want: "_cache",
			},
			{
				name: "returns one underscore before a member of package visibility",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityPackage,
				give: "Refresh", want: "_refresh",
			},
			{
				name: "returns one underscore before an internal member",
				host: symbol.KindStruct, kind: symbol.KindMethod, vis: symbol.VisibilityInternal,
				give: "reload", want: "_reload",
			},
			{
				name: "returns two underscores before a private member",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPrivate,
				give: "secret", want: "__secret",
			},
			{
				name: "returns a private member with two underscores unchanged",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPrivate,
				give: "__secret", want: "__secret",
			},
			{
				name: "returns a public member without its leading underscore",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic,
				give: "_cache", want: "cache",
			},
			{
				name: "returns one underscore before a declaration at file level that is not public",
				kind: symbol.KindFunction, vis: symbol.VisibilityPackage, give: "helper", want: "_helper",
			},
			{
				name: "returns one underscore before a private declaration at file level",
				kind: symbol.KindVariable, vis: symbol.VisibilityPrivate, give: "token", want: "_token",
			},
			{
				name: "returns the underscores of a declaration at file level that is not public unchanged",
				kind: symbol.KindVariable, vis: symbol.VisibilityPackage, give: "__registry", want: "__registry",
			},
			{
				name: "returns the underscore of a public declaration at file level unchanged",
				kind: symbol.KindFunction, vis: symbol.VisibilityPublic, give: "_compat", want: "_compat",
			},
			{
				name: "returns the underscore of a type parameter unchanged",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "_T", want: "_T",
			},
			{
				name: "returns the underscore of a parameter unchanged",
				host: symbol.KindFunction, kind: symbol.KindParam, give: "_unused", want: "_unused",
			},
			{
				name: "returns the CapWords spelling of a class with a letter outside ASCII",
				kind: symbol.KindStruct, give: "caf\U000000e9", want: "Caf\U000000e9",
			},
		}, len(passing))
		for _, kind := range passing {
			tests = append(tests, spelling{
				name: "returns a non-identifier name of a " + kind.String() + " unchanged",
				host: symbol.KindInterface, kind: kind, give: wireName, want: wireName,
			})
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Name(tt.host, tt.kind, tt.vis, tt.give)
				assert.NoError(t, err, "the name spells")
				assert.Equal(t, got, tt.want, "the spelling")
			})
		}

		type refusal struct {
			name string
			host symbol.Kind
			kind symbol.Kind
			vis  symbol.Visibility
			give string
		}
		// The kinds whose name Name refuses where it is not an identifier.
		refusing := []symbol.Kind{
			symbol.KindStruct, symbol.KindInterface, symbol.KindFunction, symbol.KindMethod, symbol.KindParam,
			symbol.KindAlias, symbol.KindConstant, symbol.KindVariable,
		}
		refusals := slices.Grow([]refusal{
			{name: "returns an error for __debug__", kind: symbol.KindVariable, give: "__debug__"},
			{
				name: "returns an error for a public member whose name starts with a digit after its underscore",
				host: symbol.KindStruct, kind: symbol.KindField, vis: symbol.VisibilityPublic, give: "_1st",
			},
			{
				name: "returns an error for a type parameter of three stars",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "***Ts",
			},
			{
				name: "returns an error for a type variable tuple whose name is no identifier",
				host: symbol.KindStruct, kind: symbol.KindTypeParam, give: "*1st",
			},
		}, len(refusing))
		for _, kind := range refusing {
			refusals = append(refusals, refusal{
				name: "returns an error for a name that is no identifier on the kind " + kind.String(), kind: kind,
				give: wireName,
			})
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Name(tt.host, tt.kind, tt.vis, tt.give)
				assert.HasError(t, err, "Python declares no such name")
				assert.Contains(t, err.Error(), tt.give, "the error names the name")
			})
		}

		t.Run("returns an error for a parameter specification named __debug__", func(t *testing.T) {
			t.Parallel()

			_, err := spell.Name(symbol.KindFunction, symbol.KindTypeParam, symbol.VisibilityUnknown, "**__debug__")
			assert.HasError(t, err, "Python binds __debug__ nowhere")
			assert.Equal(t, err.Error(), "python: no declaration can bind __debug__", "the error names the name")
		})
	})

	t.Run("Member", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Member
			want string
		}{
			{
				name: "returns a field of the type in UPPER_SNAKE_CASE unchanged",
				give: plugin.Member{Host: symbol.KindStruct, Kind: symbol.KindField, Level: symbol.LevelType},
				want: "MAX_LINES",
			},
			{
				name: "returns an immutable field in UPPER_SNAKE_CASE unchanged",
				give: plugin.Member{
					Host: symbol.KindStruct, Kind: symbol.KindField, Mutability: symbol.MutabilityImmutable,
				},
				want: "MAX_LINES",
			},
			{
				name: "returns the snake case of a field of each value in UPPER_SNAKE_CASE",
				give: plugin.Member{Host: symbol.KindStruct, Kind: symbol.KindField},
				want: "max_lines",
			},
			{
				name: "returns the snake case of a method of the type in UPPER_SNAKE_CASE",
				give: plugin.Member{Host: symbol.KindStruct, Kind: symbol.KindMethod, Level: symbol.LevelType},
				want: "max_lines",
			},
			{
				name: "returns one underscore before a protected field of the type in UPPER_SNAKE_CASE",
				give: plugin.Member{
					Host: symbol.KindStruct, Kind: symbol.KindField, Level: symbol.LevelType,
					Visibility: symbol.VisibilityProtected,
				},
				want: "_MAX_LINES",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Member(tt.give, constantName)
				assert.NoError(t, err, "Python spells the member")
				assert.Equal(t, got, tt.want, "the spelling of the member")
			})
		}

		t.Run("returns the snake case of a field of the type in camel case", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Member(
				plugin.Member{Host: symbol.KindStruct, Kind: symbol.KindField, Level: symbol.LevelType}, "maxLines")
			assert.NoError(t, err, "Python spells the member")
			assert.Equal(t, got, "max_lines", "a field of the type in another style takes snake case")
		})
	})

	t.Run("IsIdentifier", func(t *testing.T) {
		t.Parallel()

		identifiers := []struct {
			name string
			give string
		}{
			{name: "reports true for a letter", give: "a"},
			{name: "reports true for an underscore before a letter", give: "_x"},
			{name: "reports true for a digit after a letter", give: "row1"},
			{name: "reports true for a letter outside ASCII", give: "\U000000e9"},
			{name: "reports true for a letter number", give: "\U0000216b"},
			{name: "reports true for a character with the property Other_ID_Start", give: "\U00002118"},
			{name: "reports true for a combining mark after a letter", give: "a\U00000301"},
			{name: "reports true for a connector punctuation after a letter", give: "a\U0000203f"},
			{
				name: "reports true for a character with the property Other_ID_Continue after a letter",
				give: "a\U000000b7",
			},
			{name: "reports true for a decimal digit of another script after a letter", give: "a\U00000661"},
			{
				name: "reports true for a character that continues an identifier alone after a letter",
				give: "a\U00000e33",
			},
		}
		for _, tt := range identifiers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.True(t, spell.IsIdentifier(tt.give), "the name is an identifier")
			})
		}

		others := []struct {
			name string
			give string
		}{
			{name: "reports false for the empty name", give: ""},
			{name: "reports false for a digit first", give: "1row"},
			{name: "reports false for a hyphen", give: wireName},
			{name: "reports false for a dollar", give: "$"},
			{name: "reports false for a space", give: "a b"},
			{name: "reports false for a combining mark first", give: "\U00000301a"},
			{name: "reports false for a letter of the pattern syntax", give: "\U00002e2f"},
			{name: "reports false for a letter that the normal form NFKC leaves out", give: "a\U0000037a"},
			{name: "reports false for a character that continues an identifier alone first", give: "\U00000e33"},
			{name: "reports false for a line separator after a letter", give: "a\U00002028"},
			{name: "reports false for a symbol after a letter", give: "a\U000020ac"},
			{name: "reports false for a byte that is not UTF-8", give: "a\xff"},
		}
		for _, tt := range others {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.False(t, spell.IsIdentifier(tt.give), "the name is not an identifier")
			})
		}
	})

	t.Run("IsKeyword", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for a keyword", give: "def", want: true},
			{name: "reports true for a keyword in capitals", give: "None", want: true},
			{name: "reports false for a soft keyword", give: "match", want: false},
			{name: "reports false for a name", give: "row", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spell.IsKeyword(tt.give), tt.want, "IsKeyword reports whether the name is a keyword")
			})
		}
	})
}

// Name allocates a converted spelling and the spelling with its
// underscores, and nothing for a name that the convention spells as it is.
// Member allocates nothing for a constant of a class, and IsIdentifier
// allocates nothing. The ordinary run, which runs no benchmark, checks
// those ceilings here.
func TestNameAllocs(t *testing.T) {
	for _, tt := range nameCalls() {
		var (
			got string
			err error
		)
		assert.MaxAllocs(t, func() { got, err = spell.Name(tt.host, tt.kind, tt.vis, tt.give) }, tt.allocs,
			"Name of "+tt.name+" allocates within its ceiling")
		assert.NoError(t, err, "Name spells "+tt.name)
		assert.Equal(t, got, tt.want, "Name spells "+tt.name+" in Python")
	}
	var valid bool
	assert.MaxAllocs(
		t,
		func() { valid = spell.IsIdentifier("caf\U000000e9_count") },
		0,
		"IsIdentifier allocates nothing",
	)
	assert.True(t, valid, "IsIdentifier reports an identifier")
	var (
		member string
		err    error
	)
	assert.MaxAllocs(t, func() { member, err = spell.Member(constantMember, constantName) }, 0,
		"Member of a constant of a class allocates nothing")
	assert.NoError(t, err, "Member spells the constant")
	assert.Equal(t, member, constantName, "Member keeps the spelling of the constant")
}

// BenchmarkName measures the spelling that the settle makes for every
// declared name, and the identifier check that the backend makes for every
// member key.
func BenchmarkName(b *testing.B) {
	b.Run("Name", func(b *testing.B) {
		for _, tt := range nameCalls() {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got string
					err error
				)
				for c.Loop() {
					got, err = spell.Name(tt.host, tt.kind, tt.vis, tt.give)
				}
				assert.NoError(b, err, "Name spells "+tt.name)
				assert.Equal(b, got, tt.want, "Name spells "+tt.name+" in Python")
			})
		}
	})

	b.Run("Member", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got string
			err error
		)
		for c.Loop() {
			got, err = spell.Member(constantMember, constantName)
		}
		assert.NoError(b, err, "Member spells the constant")
		assert.Equal(b, got, constantName, "Member keeps the spelling of the constant")
	})

	b.Run("IsIdentifier", func(b *testing.B) {
		for _, give := range []string{"row_count", "caf\U000000e9_count"} {
			b.Run(give, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(0)
				defer c.End()
				var valid bool
				for c.Loop() {
					valid = spell.IsIdentifier(give)
				}
				assert.True(b, valid, "IsIdentifier reports an identifier")
			})
		}
	})
}

// nameCalls returns the names that the allocation test and the benchmark
// measure: a name in another style, a name in its style, and a converted
// member that takes an underscore.
func nameCalls() []nameCall {
	return []nameCall{
		{
			name:   "a name in another style",
			kind:   symbol.KindStruct,
			give:   "order_item",
			allocs: conversionAllocs,
			want:   "OrderItem",
		},
		{name: "a name in its style", kind: symbol.KindStruct, give: "OrderItem", want: "OrderItem"},
		{
			name: "a member that takes an underscore", host: symbol.KindStruct, kind: symbol.KindMethod,
			vis: symbol.VisibilityPackage, give: "Refresh", allocs: underscoreAllocs, want: "_refresh",
		},
	}
}
