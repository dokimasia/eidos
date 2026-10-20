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
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The ceilings of capabilities.go. The hash of a builtin, the witness of a
// default and Reified allocate nothing, and the witness object allocates its
// reference. The contract of OutOfStock allocates the binding of the member
// walk, the members and the path through each of its three bases of
// builtins. The properties of Account allocate the binding, the members, the
// list of the members of the class and the list of properties. The
// constructor of Item allocates its parameters and the list of callables.
const (
	capabilityAllocs   = 0
	witnessAllocs      = 1
	errorAllocs        = 5
	propertiesAllocs   = 4
	constructorsAllocs = 2
)

// outOfStockID is the exception class of the fixture that writes its own
// message.
var outOfStockID = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "OutOfStock", Kind: symbol.KindStruct}

// TestCapabilities pins the error types, the properties, the constructors,
// the hashes and the generics of Python. A case names a class of the module
// of the models, or of the module of the classes with cycles of bases.
func TestCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("ErrorContract", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			give       string
			pkg        string
			kind       symbol.Kind
			wantOK     bool
			wantSource string
		}{
			{
				name:       "reads the __str__ of an exception class",
				give:       "OutOfStock",
				wantOK:     true,
				wantSource: "OutOfStock",
			},
			{
				name:       "reads the __str__ that a base exception declares",
				give:       "SoldOut",
				wantOK:     true,
				wantSource: "OutOfStock",
			},
			{name: "reports false for a class without an exception base", give: "Order"},
			{name: "reports false for a class whose bases form a cycle", give: "Up", pkg: brokenPkg},
			{name: "reports false for an enum", give: "Status", kind: symbol.KindEnum},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{
					Lang: python.Lang, Package: cmp.Or(tt.pkg, modelsPkg), Name: tt.give,
					Kind: cmp.Or(tt.kind, symbol.KindStruct),
				}
				c, ok := pythonrules.Rules{}.ErrorContract(f.decl(t, id), f.view)
				assert.Equal(t, ok, tt.wantOK, "whether the class is an exception")
				if !tt.wantOK {
					return
				}
				message, is := c.Message.(*node.Method)
				assert.True(t, is, "the message is a method")
				expect.Equal(t, message.Name, "__str__", "the method that writes the message")
				expect.Equal(t, message.ID.Owner, tt.wantSource, "the class that declares the method")
			})
		}

		t.Run("returns no message for an exception that inherits the __str__ of BaseException", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			silent := symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Silent", Kind: symbol.KindStruct}
			c, ok := pythonrules.Rules{}.ErrorContract(f.decl(t, silent), f.view)
			assert.True(t, ok, "Silent is an exception")
			expect.Nil(t, c.Message, "the message of the error of the language")
			expect.Nil(t, c.Cause, "the cause of the error of the language")
		})

		t.Run("reads the __cause__ that an exception class declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			chained := symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Chained", Kind: symbol.KindStruct}
			c, ok := pythonrules.Rules{}.ErrorContract(f.decl(t, chained), f.view)
			assert.True(t, ok, "Chained is an exception")
			assert.NotNil(t, c.Cause, "the class declares its cause")
			expect.Equal(t, c.Cause.Name, "__cause__", "the field of the cause")
			expect.Equal(t, c.Cause.ID.Owner, "Chained", "the class that declares the field")
		})

		built := []struct {
			name   string
			give   symbol.Symbol
			wantOK bool
		}{
			{
				name:   "reads an exception base of builtins without a target",
				give:   &node.Struct{Extends: []*node.TypeRef{nil, {Spelling: "ValueError"}}},
				wantOK: true,
			},
			{
				name: "reports false for a base of builtins that is no exception",
				give: &node.Struct{Extends: []*node.TypeRef{{Spelling: "int"}}},
			},
			{
				name: "reports false for a dotted base whose first name nothing binds",
				give: &node.Struct{Extends: []*node.TypeRef{{Spelling: "lib.Error"}}},
			},
			{name: "reports false for a nil class", give: (*node.Struct)(nil)},
			{name: "reports false for a symbol that is no class", give: &node.Enum{}},
		}
		for _, tt := range built {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c, ok := pythonrules.Rules{}.ErrorContract(tt.give, rules.View{})
				expect.Equal(t, ok, tt.wantOK, "whether the class is an exception")
				expect.Nil(t, c.Message, "the class declares no __str__")
			})
		}
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()

		t.Run("pairs a getter with the setter of its name", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			account, _ := f.decl(t, accountID).(*node.Struct)
			got := pythonrules.Rules{}.Properties(account, f.view)
			assert.Length(t, got, 1, "Account has one property")
			expect.Equal(t, got[0].Name, "limit", "the name of the property")
			expect.Equal(t, got[0].Getter.ID, limitGetID, "the getter")
			assert.NotNil(t, got[0].Setter, "the property has a setter")
			expect.Equal(t, got[0].Setter.ID.Disc, "int", "the setter takes an int")
			assert.NotNil(t, got[0].Type, "the getter states its type")
			expect.Equal(t, got[0].Type.Spelling, "int", "the type of the getter")
		})

		t.Run("reads a getter without a type as a property without a type", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			gauge, _ := f.decl(t, symbol.Identity{
				Lang: python.Lang, Package: modelsPkg, Name: "Gauge", Kind: symbol.KindStruct,
			}).(*node.Struct)
			got := pythonrules.Rules{}.Properties(gauge, f.view)
			assert.Length(t, got, 1, "Gauge has one property")
			expect.Nil(t, got[0].Type, "the getter states no type")
			expect.Nil(t, got[0].Setter, "the property is read-only")
		})

		t.Run("returns no property of a class without getters", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			order, _ := f.decl(t, orderID).(*node.Struct)
			assert.Empty(t, pythonrules.Rules{}.Properties(order, f.view), "Order has no property")
		})
	})

	t.Run("Constructors", func(t *testing.T) {
		t.Parallel()

		const (
			keyword    = symbol.PassingKeyword
			positional = symbol.PassingDefault
		)
		tests := []struct {
			name         string
			give         string
			pkg          string
			wantName     string
			wantParams   []string
			wantOptional []bool
			wantPassing  []symbol.Passing
		}{
			{
				name: "generates the __init__ of a dataclass from its fields", give: "Item", wantName: "__init__",
				wantParams: []string{"sku", "quantity", "tags"}, wantOptional: []bool{false, true, true},
				wantPassing: []symbol.Passing{positional, positional, positional},
			},
			{
				name: "passes every field of a dataclass of kw_only=True by keyword", give: "Address",
				wantName: "__init__", wantParams: []string{"street", "city"}, wantOptional: []bool{false, false},
				wantPassing: []symbol.Passing{keyword, keyword},
			},
			{
				name: "passes the fields after KW_ONLY by keyword", give: "Order", wantName: "__init__",
				wantParams:   []string{"id", "customer", "items", "status", "note"},
				wantOptional: []bool{false, false, false, true, true},
				wantPassing:  []symbol.Passing{positional, positional, positional, keyword, keyword},
			},
			{
				name: "puts the fields of a base dataclass first and the keyword fields last", give: "Rush",
				wantName:     "__init__",
				wantParams:   []string{"id", "customer", "items", "deadline", "status", "note"},
				wantOptional: []bool{false, false, false, true, true, true},
				wantPassing:  []symbol.Passing{positional, positional, positional, positional, keyword, keyword},
			},
			{
				name: "passes a field of field(kw_only=True) by keyword", give: "Mixed", wantName: "__init__",
				wantParams: []string{"d", "a", "b"}, wantOptional: []bool{false, true, true},
				wantPassing: []symbol.Passing{positional, positional, keyword},
			},
			{
				name: "reads the arguments of field outside the brackets and the strings of a value", give: "Tricky",
				wantName: "__init__", wantParams: []string{"a", "b", "c"}, wantOptional: []bool{true, true, true},
				wantPassing: []symbol.Passing{positional, positional, positional},
			},
			{
				name: "keeps the place of a field that a dataclass declares again", give: "Child1",
				wantName: "__init__", wantParams: []string{"x", "y"}, wantOptional: []bool{true, true},
				wantPassing: []symbol.Passing{positional, positional},
			},
			{
				name: "reads the declared __init__ of a dataclass of init=False", give: "Manual",
				wantName: "__init__", wantParams: []string{"y"}, wantOptional: []bool{false},
				wantPassing: []symbol.Passing{positional},
			},
			{
				name: "generates the __new__ of a named tuple from its fields", give: "Point", wantName: "__new__",
				wantParams: []string{"x", "y"}, wantOptional: []bool{false, true},
				wantPassing: []symbol.Passing{positional, positional},
			},
			{
				name: "reads the __init__ of a class without its receiver", give: "Account", wantName: "__init__",
				wantParams: []string{"owner", "limit"}, wantOptional: []bool{false, true},
				wantPassing: []symbol.Passing{positional, keyword},
			},
			{
				name: "reads the __new__ of a class without __init__", give: "Token", wantName: "__new__",
				wantParams: []string{"text"}, wantOptional: []bool{false}, wantPassing: []symbol.Passing{positional},
			},
			{
				name: "reads the __init__ that a base of builtins declares", give: "OutOfStock",
				wantName: "__init__", wantParams: []string{"args"}, wantOptional: []bool{true},
				wantPassing: []symbol.Passing{positional},
			},
			{name: "constructs a class without a constructor through object", give: "Plain", wantName: "__init__"},
			{name: "constructs a class whose base is a Protocol through object", give: "Impl", wantName: "__init__"},
			{
				name: "ends the walk of the bases of a dataclass at a cycle", give: "First", pkg: brokenPkg,
				wantName: "__init__",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{
					Lang: python.Lang, Package: cmp.Or(tt.pkg, modelsPkg), Name: tt.give, Kind: symbol.KindStruct,
				}
				s, _ := f.decl(t, id).(*node.Struct)
				cs := pythonrules.Rules{}.Constructors(s, f.view)
				assert.Length(t, cs, 1, "the class constructs through one signature")
				expect.Equal(t, cs[0].Name, tt.wantName, "the method that constructs")
				expect.Equal(t, paramNames(cs[0]), tt.wantParams, "the parameters in the order of the signature",
					assert.EquateEmpty())
				expect.Equal(t, paramOptional(cs[0]), tt.wantOptional, "the parameters with a default",
					assert.EquateEmpty())
				expect.Equal(t, paramPassing(cs[0]), tt.wantPassing, "the way a call passes each argument",
					assert.EquateEmpty())
				expect.Nil(t, cs[0].Receiver, "a constructor has no receiver")
			})
		}

		t.Run("returns one signature for each overload of __init__", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, _ := f.decl(t, symbol.Identity{
				Lang: python.Lang, Package: modelsPkg, Name: "Parsed", Kind: symbol.KindStruct,
			}).(*node.Struct)
			cs := pythonrules.Rules{}.Constructors(s, f.view)
			assert.Length(t, cs, 2, "the implementation is no signature")
			assert.Length(t, cs[0].Params, 1, "the first overload takes one parameter")
			expect.Equal(t, cs[0].Params[0].Ref.Spelling, "int", "the first overload takes an int")
			assert.Length(t, cs[1].Params, 1, "the second overload takes one parameter")
			expect.Equal(t, cs[1].Params[0].Ref.Spelling, "str", "the second overload takes a str")
		})

		t.Run("skips a nil base and a nil field of a dataclass", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s := &node.Struct{
				ID: itemID, Annotations: symbol.Annotations{{Name: "dataclass"}}, Extends: []*node.TypeRef{nil},
				Fields: []*node.Field{nil, {Name: "x", Type: &node.TypeRef{Spelling: "int"}}},
			}
			cs := pythonrules.Rules{}.Constructors(s, f.view)
			assert.Length(t, cs, 1, "the dataclass generates one constructor")
			expect.Equal(t, paramNames(cs[0]), []string{"x"}, "the one field is the one parameter")
		})

		t.Run("skips a nil base and a nil field of a named tuple", func(t *testing.T) {
			t.Parallel()

			namedTuple := symbol.Identity{
				Lang:    python.Lang,
				Package: typingModule,
				Name:    "NamedTuple",
				Kind:    symbol.KindStruct,
			}
			s := &node.Struct{
				Extends: []*node.TypeRef{nil, {Spelling: "NamedTuple", Target: namedTuple}},
				Fields:  []*node.Field{nil, {Name: "x", Type: &node.TypeRef{Spelling: "int"}}},
			}
			cs := pythonrules.Rules{}.Constructors(s, rules.View{})
			assert.Length(t, cs, 1, "the named tuple generates one constructor")
			expect.Equal(t, cs[0].Name, "__new__", "a named tuple constructs through __new__")
			expect.Equal(t, paramNames(cs[0]), []string{"x"}, "the one field is the one parameter")
		})

		t.Run("skips a nil base of a class without a constructor", func(t *testing.T) {
			t.Parallel()

			cs := pythonrules.Rules{}.Constructors(&node.Struct{Extends: []*node.TypeRef{nil}}, rules.View{})
			assert.Length(t, cs, 1, "the class constructs through object")
			expect.Empty(t, cs[0].Params, "the __init__ of object takes nothing")
		})

		t.Run("returns no signature for a chain of bases deeper than the depth of the kernel", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, _ := f.decl(t, symbol.Identity{
				Lang: python.Lang, Package: brokenPkg, Name: "Up", Kind: symbol.KindStruct,
			}).(*node.Struct)
			assert.Empty(t, pythonrules.Rules{}.Constructors(s, f.view), "a cycle has no constructor")
		})
	})

	t.Run("Comparable", func(t *testing.T) {
		t.Parallel()

		intRef := &node.TypeRef{Spelling: "int"}
		listRef := &node.TypeRef{Spelling: "list", Args: []*node.TypeRef{intRef}}
		built := []struct {
			name        string
			give        *node.TypeRef
			wantProblem *node.TypeRef
		}{
			{name: "hashes an int", give: intRef},
			{
				name: "hashes a tuple of hashable members",
				give: &node.TypeRef{Form: symbol.FormTuple, Elems: []*node.TypeRef{intRef, {Spelling: "str"}}},
			},
			{name: "hashes a frozenset", give: &node.TypeRef{Spelling: "frozenset", Args: []*node.TypeRef{intRef}}},
			{
				name: "hashes an immutable list of hashable elements",
				give: &node.TypeRef{
					Form: symbol.FormList, Mutability: symbol.MutabilityImmutable, Elems: []*node.TypeRef{intRef},
				},
			},
			{
				name: "hashes a Mapping of hashable arguments",
				give: &node.TypeRef{Spelling: "Mapping", Package: abcModule, Args: []*node.TypeRef{intRef, intRef}},
			},
			{name: "hashes a function type", give: &node.TypeRef{Form: symbol.FormFunc}},
			{name: "hashes a nil reference"},
			{name: "breaks on a list", give: listRef, wantProblem: listRef},
			{
				name:        "breaks on a list member of a tuple",
				give:        &node.TypeRef{Form: symbol.FormTuple, Elems: []*node.TypeRef{intRef, listRef}},
				wantProblem: listRef,
			},
			{
				name:        "breaks on a list inside an optional",
				give:        &node.TypeRef{Form: symbol.FormOptional, Elems: []*node.TypeRef{listRef}},
				wantProblem: listRef,
			},
			{
				name: "breaks on a list element of an immutable list",
				give: &node.TypeRef{
					Form: symbol.FormList, Mutability: symbol.MutabilityImmutable, Elems: []*node.TypeRef{listRef},
				},
				wantProblem: listRef,
			},
			{
				name:        "breaks on a list argument of a Sequence",
				give:        &node.TypeRef{Spelling: "Sequence", Package: abcModule, Args: []*node.TypeRef{listRef}},
				wantProblem: listRef,
			},
		}
		for _, tt := range built {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ok, problems := pythonrules.Rules{}.Comparable(tt.give, rules.View{})
				if tt.wantProblem == nil {
					expect.True(t, ok, "the type hashes")
					expect.Empty(t, problems, "nothing breaks the hash")
					return
				}
				expect.False(t, ok, "the type has no hash")
				assert.Length(t, problems, 1, "one reference breaks the hash")
				expect.Equal(t, problems[0], tt.wantProblem, "the reference that breaks the hash", assert.ByIdentity())
			})
		}

		unhashable := []struct {
			name string
			give *node.TypeRef
		}{
			{
				name: "breaks on a mutable list form",
				give: &node.TypeRef{Form: symbol.FormList, Elems: []*node.TypeRef{intRef}},
			},
			{name: "breaks on a set", give: &node.TypeRef{Spelling: "set", Args: []*node.TypeRef{intRef}}},
			{name: "breaks on a bytearray", give: &node.TypeRef{Spelling: "bytearray"}},
			{
				name: "breaks on Set of typing",
				give: &node.TypeRef{Spelling: "Set", Package: typingModule, Args: []*node.TypeRef{intRef}},
			},
			{
				name: "breaks on MutableSet of collections.abc",
				give: &node.TypeRef{Spelling: "MutableSet", Package: abcModule, Args: []*node.TypeRef{intRef}},
			},
			{name: "breaks on a dict", give: &node.TypeRef{Spelling: "dict", Args: []*node.TypeRef{intRef, intRef}}},
		}
		for _, tt := range unhashable {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ok, problems := pythonrules.Rules{}.Comparable(tt.give, rules.View{})
				expect.False(t, ok, "the type has no hash")
				assert.Length(t, problems, 1, "the type alone breaks the hash")
				expect.Equal(t, problems[0], tt.give, "the type breaks the hash", assert.ByIdentity())
			})
		}

		classes := []struct {
			name string
			give string
			pkg  string
			kind symbol.Kind
			want bool
		}{
			{name: "hashes a class that declares __eq__ and __hash__", give: "Both", want: true},
			{name: "hashes a subclass that declares __hash__ again", give: "Rehashed", want: true},
			{name: "hashes a frozen dataclass of hashable fields", give: "Money", want: true},
			{name: "hashes a dataclass of unsafe_hash=True", give: "Keyed", want: true},
			{name: "hashes a dataclass of eq=False by its identity", give: "Ident", want: true},
			{name: "hashes a named tuple of hashable fields", give: "Point", want: true},
			{name: "hashes a frozen dataclass whose field of InitVar has no hash", give: "Seeded", want: true},
			{name: "hashes an enum", give: "Status", kind: symbol.KindEnum, want: true},
			{name: "hashes a Protocol", give: "Named", kind: symbol.KindInterface, want: true},
			{name: "hashes a class whose bases form a cycle", give: "Up", pkg: brokenPkg, want: true},
			{name: "hashes a type outside the view", give: "Gone", pkg: "nowhere", want: true},
			{name: "hashes through a cycle of aliases", give: "Ping", kind: symbol.KindAlias, want: true},
			{name: "breaks on a class that declares __eq__ without __hash__", give: "Account"},
			{name: "breaks on a subclass of a class without a hash", give: "Derived"},
			{name: "breaks on a dataclass of eq=False whose base has no hash", give: "Plainly"},
			{name: "breaks on a subclass of list of typeshed", give: "Numbers"},
			{name: "breaks on a class that sets __hash__ to None", give: "NoHash"},
			{name: "breaks on a dataclass that compares its fields", give: "Item"},
			{name: "breaks on a TypedDict", give: "Payload", kind: symbol.KindInterface},
			{name: "breaks on an alias of a list", give: "Tags", kind: symbol.KindAlias},
		}
		for _, tt := range classes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{
					Lang: python.Lang, Package: cmp.Or(tt.pkg, modelsPkg), Name: tt.give,
					Kind: cmp.Or(tt.kind, symbol.KindStruct),
				}
				ok, problems := pythonrules.Rules{}.Comparable(&node.TypeRef{Spelling: tt.give, Target: id}, f.view)
				expect.Equal(t, ok, tt.want, "whether the type hashes")
				expect.Equal(t, len(problems) == 0, tt.want, "a type that hashes has no problem")
			})
		}

		t.Run("hashes a type parameter", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			ok, _ := pythonrules.Rules{}.Comparable(&node.TypeRef{Spelling: "T", Target: boxParam}, f.view)
			assert.True(t, ok, "the hash of a type parameter is unknown")
		})

		fields := []struct {
			name string
			give string
		}{
			{name: "breaks on a list field of a frozen dataclass", give: "Holder"},
			{name: "breaks on a list field that a base of a frozen dataclass declares", give: "Grown"},
			{name: "breaks on a list field of a named tuple", give: "Row"},
		}
		for _, tt := range fields {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: tt.give, Kind: symbol.KindStruct}
				ok, problems := pythonrules.Rules{}.Comparable(&node.TypeRef{Spelling: tt.give, Target: id}, f.view)
				expect.False(t, ok, "the type has no hash")
				assert.Length(t, problems, 1, "one field breaks the hash")
				expect.Equal(t, problems[0].Spelling, "list", "the list type of the field")
			})
		}
	})

	t.Run("Derive", func(t *testing.T) {
		t.Parallel()

		bound := &node.TypeRef{Spelling: "int"}
		unionBound := &node.TypeRef{
			Spelling: "int | str", Form: symbol.FormUnion, Elems: []*node.TypeRef{bound, {Spelling: "str"}},
		}
		tests := []struct {
			name   string
			give   *node.TypeParam
			want   *node.TypeRef
			wantOK bool
		}{
			{
				name: "derives the default of a type parameter",
				give: &node.TypeParam{
					Default: bound,
					Bounds:  []*node.TypeRef{{Spelling: "str"}},
				},
				want:   bound,
				wantOK: true,
			},
			{
				name: "derives the first bound of a type parameter without a default",
				give: &node.TypeParam{Bounds: []*node.TypeRef{bound}}, want: bound, wantOK: true,
			},
			{
				name: "derives the first constraint of a type variable with constraints",
				give: &node.TypeParam{Bounds: []*node.TypeRef{{
					Spelling: "(int, str)", Form: symbol.FormUnion, Elems: []*node.TypeRef{bound, {Spelling: "str"}},
				}}},
				want: bound, wantOK: true,
			},
			{
				name: "derives a bound of the form Union whole",
				give: &node.TypeParam{Bounds: []*node.TypeRef{unionBound}}, want: unionBound, wantOK: true,
			},
			{name: "reports false for a nil type parameter"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := pythonrules.Rules{}.Derive(tt.give, rules.View{})
				expect.Equal(t, ok, tt.wantOK, "whether a witness derives")
				expect.Equal(t, got, tt.want, "the witness", assert.ByIdentity())
			})
		}

		objects := []struct {
			name string
			give *node.TypeParam
		}{
			{name: "derives object for a type parameter without a bound", give: &node.TypeParam{Name: "T"}},
			{
				name: "derives object for a type parameter of a nil bound",
				give: &node.TypeParam{Bounds: []*node.TypeRef{nil}},
			},
		}
		for _, tt := range objects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := pythonrules.Rules{}.Derive(tt.give, rules.View{})
				assert.True(t, ok, "object is a witness")
				assert.Equal(t, got, &node.TypeRef{Spelling: "object"}, "the top type of Python")
			})
		}
	})

	t.Run("Reified", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false", func(t *testing.T) {
			t.Parallel()

			assert.False(t, pythonrules.Rules{}.Reified(), "Python erases type arguments")
		})
	})
}

// TestCapabilitiesAllocs checks the ceilings of the methods of
// capabilities.go in the ordinary run, which does not run benchmarks.
func TestCapabilitiesAllocs(t *testing.T) {
	checkAllocs(t, capabilityCalls(t))
}

// BenchmarkCapabilities measures each method of capabilities.go under its
// ceiling.
func BenchmarkCapabilities(b *testing.B) {
	benchCalls(b, capabilityCalls(b))
}

// capabilityCalls returns a call of each method of capabilities.go.
func capabilityCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := pythonrules.Rules{}
	outOfStock := f.decl(tb, outOfStockID)
	account, _ := f.decl(tb, accountID).(*node.Struct)
	item, _ := f.decl(tb, itemID).(*node.Struct)
	intRef := &node.TypeRef{Spelling: "int"}
	param := &node.TypeParam{Name: "T"}
	defaulted := &node.TypeParam{Name: "T", Default: intRef}
	var (
		contract rules.ErrorContract
		props    []rules.Property
		cs       []rules.Callable
		ok       bool
		witness  *node.TypeRef
	)
	return []allocCall{
		{
			name: "ErrorContract", allocs: errorAllocs,
			call:  func() { contract, _ = r.ErrorContract(outOfStock, f.view) },
			check: func(tb assert.TB) { assert.NotNil(tb, contract.Message, "OutOfStock writes its message") },
		},
		{
			name: "Properties", allocs: propertiesAllocs,
			call:  func() { props = r.Properties(account, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, props, 1, "Account has one property") },
		},
		{
			name: "Constructors", allocs: constructorsAllocs,
			call:  func() { cs = r.Constructors(item, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, cs, 1, "Item has one constructor") },
		},
		{
			name: "Comparable", allocs: capabilityAllocs,
			call:  func() { ok, _ = r.Comparable(intRef, rules.View{}) },
			check: func(tb assert.TB) { assert.True(tb, ok, "an int hashes") },
		},
		{
			name: "Derive", caseName: "a default", allocs: capabilityAllocs,
			call:  func() { witness, _ = r.Derive(defaulted, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, witness, intRef, "the default", assert.ByIdentity()) },
		},
		{
			name: "Derive", caseName: "object", allocs: witnessAllocs,
			call:  func() { witness, _ = r.Derive(param, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, witness.Spelling, "object", "the top type") },
		},
		{
			name: "Reified", allocs: capabilityAllocs,
			call:  func() { ok = r.Reified() },
			check: func(tb assert.TB) { assert.False(tb, ok, "Python erases type arguments") },
		},
	}
}
