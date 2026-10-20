// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/rust"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The ceilings of the capabilities: the ownership of a parameter, the
// equality of a primitive and the witness of a default allocate nothing.
const (
	ownershipAllocs  = 0
	comparableAllocs = 0
	deriveAllocs     = 0
)

// The declarations of the fixture that the capabilities read.
var (
	loadErrID = symbol.Identity{Lang: rust.Lang, Package: errorsPkg, Name: "LoadError", Kind: symbol.KindStruct}
	cacheID   = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "Cache", Kind: symbol.KindStruct}
	showID    = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "show", Kind: symbol.KindFunction}
)

// TestCapabilities pins the constructors, the error contracts, the
// ownership, the equality and the witnesses that the rules state for Rust.
func TestCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("Constructors", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			give       symbol.Identity
			wantNames  []string
			wantParams []int
		}{
			{
				name: "returns the associated functions that build the struct and then its literal", give: orderID,
				wantNames: []string{"new", "parse", "Order"}, wantParams: []int{2, 1, 14},
			},
			{
				name: "returns the literal of a struct after its constructor without parameters", give: memoryID,
				wantNames: []string{"new", "Memory"}, wantParams: []int{0, 1},
			},
			{
				name: "returns the literal of a tuple struct", give: idID,
				wantNames: []string{"Id"}, wantParams: []int{1},
			},
			{
				name: "returns no literal of a struct with a private field", give: settingsID,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				s, is := f.decl(t, tt.give).(*node.Struct)
				assert.True(t, is, tt.give.Name+" is a struct")
				cs := rustrules.Rules{}.Constructors(s, f.view)
				var names []string
				var params []int
				for _, c := range cs {
					names = append(names, c.Name)
					params = append(params, len(c.Params))
				}
				expect.Equal(t, names, tt.wantNames, "the names of the constructors")
				expect.Equal(t, params, tt.wantParams, "the number of the parameters of each constructor")
			})
		}

		t.Run("returns the value and the failure of a constructor that returns a result", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, is := f.decl(t, orderID).(*node.Struct)
			assert.True(t, is, "Order is a struct")
			cs := rustrules.Rules{}.Constructors(s, f.view)
			assert.Length(t, cs, 3, "Order has three constructors")
			assert.Length(t, cs[1].Returns, 2, "parse returns a value and a failure")
			expect.Equal(t, cs[1].Returns[1].Role, rules.ReturnError, "the second view is the failure")
		})

		t.Run("passes a field of a tuple struct by position", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, is := f.decl(t, idID).(*node.Struct)
			assert.True(t, is, "Id is a struct")
			cs := rustrules.Rules{}.Constructors(s, f.view)
			assert.Length(t, cs, 1, "Id has its literal")
			assert.Length(t, cs[0].Params, 1, "the literal has one parameter")
			expect.Equal(t, cs[0].Params[0].Passing, symbol.PassingPositional, "a positional field passes by position")
		})

		t.Run("returns nothing for a nil struct", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, rustrules.Rules{}.Constructors(nil, rules.View{}), "a nil struct has no constructor")
		})

		t.Run("returns an associated function inside an option", func(t *testing.T) {
			t.Parallel()

			rowID := symbol.Identity{Lang: rust.Lang, Package: modelsPkg, Name: "Row", Kind: symbol.KindStruct}
			option := &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{{Spelling: "Self", Target: rowID}}}
			s := &node.Struct{ID: rowID, Name: "Row", Methods: []*node.Method{
				nil,
				{
					Name: "find", Level: symbol.LevelType, Visibility: symbol.VisibilityPublic,
					Returns: []*node.Return{{Type: option}},
				},
				{
					Name: "count", Level: symbol.LevelType, Visibility: symbol.VisibilityPublic,
					Returns: []*node.Return{{Type: &node.TypeRef{Spelling: u8Name}}},
				},
				{
					Name:       "none",
					Level:      symbol.LevelType,
					Visibility: symbol.VisibilityPublic,
					Returns:    []*node.Return{nil},
				},
				{
					Name: "deep", Level: symbol.LevelType, Visibility: symbol.VisibilityPublic,
					Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "(u8,)", Form: symbol.FormTuple}}},
				},
			}}
			cs := rustrules.Rules{}.Constructors(s, rules.View{})
			names := make([]string, 0, len(cs))
			for _, c := range cs {
				names = append(names, c.Name)
			}
			assert.Equal(t, names, []string{"find", "Row"}, "find builds the struct inside an option")
		})
	})

	t.Run("ErrorContract", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the contract of a data enum that derives Error of thiserror", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			c, is := rustrules.Rules{}.ErrorContract(f.decl(t, storeErrID), f.view)
			assert.True(t, is, "StoreError is an error type")
			expect.Equal(t, c, rules.ErrorContract{}, "the variants of a data enum state their own causes")
		})

		tests := []struct {
			name      string
			give      symbol.Identity
			wantCause string
		}{
			{
				name: "returns the field named source as the cause of a struct that implements the error trait",
				give: parseErrID, wantCause: "source",
			},
			{
				name: "returns the field that #[source] marks as the cause", give: loadErrID, wantCause: "cause",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				c, is := rustrules.Rules{}.ErrorContract(f.decl(t, tt.give), f.view)
				assert.True(t, is, tt.give.Name+" is an error type")
				expect.Nil(t, c.Message, "the message is in an impl of Display")
				assert.NotNil(t, c.Cause, "the error has a cause")
				expect.Equal(t, c.Cause.Name, tt.wantCause, "the cause of the error")
			})
		}

		implemented := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "returns the contract of an enum that implements the error trait",
				give: &node.Enum{Name: "Code", Implements: []*node.TypeRef{{Spelling: errorName}}},
			},
			{
				name: "returns the contract of a data enum that implements the error trait",
				give: &node.Sum{Name: "Failure", Implements: []*node.TypeRef{{Spelling: errorName}}},
			},
		}
		for _, tt := range implemented {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c, is := rustrules.Rules{}.ErrorContract(tt.give, rules.View{})
				assert.True(t, is, "the declaration is an error type")
				expect.Equal(t, c, rules.ErrorContract{}, "the variants state their own causes")
			})
		}

		t.Run("returns the contract of a struct that derives the path of thiserror", func(t *testing.T) {
			t.Parallel()

			s := &node.Struct{Name: "Failure", Annotations: symbol.Annotations{
				{Name: "doc"}, {Name: "derive", Args: []string{"Debug", "thiserror::Error"}},
			}, Fields: []*node.Field{nil, {Name: "LIMIT", Level: symbol.LevelType}, {Name: "source"}, {
				Name: "inner", Annotations: symbol.Annotations{{Name: "from"}},
			}}}
			c, is := rustrules.Rules{}.ErrorContract(s, rules.View{})
			assert.True(t, is, "the struct is an error type")
			assert.NotNil(t, c.Cause, "the error has a cause")
			expect.Equal(t, c.Cause.Name, "inner", "the marked field comes before the field named source")
		})

		others := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "reports false for a struct that is no error", give: &node.Struct{Name: "Row"}},
			{name: "reports false for an enum that is no error", give: &node.Enum{Name: "Color"}},
			{name: "reports false for a data enum that is no error", give: &node.Sum{Name: "Shape"}},
			{name: "reports false for a function", give: &node.Function{Name: "open"}},
			{name: "reports false for a nil struct", give: (*node.Struct)(nil)},
			{name: "reports false for a nil enum", give: (*node.Enum)(nil)},
			{name: "reports false for a nil data enum", give: (*node.Sum)(nil)},
		}
		for _, tt := range others {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, is := rustrules.Rules{}.ErrorContract(tt.give, rules.View{})
				assert.False(t, is, "the declaration is no error type")
			})
		}
	})

	t.Run("Ownership", func(t *testing.T) {
		t.Parallel()

		u8 := &node.TypeRef{Spelling: u8Name}
		tests := []struct {
			name string
			give *node.TypeRef
			want rules.Ownership
		}{
			{name: "moves a parameter without a type", want: rules.OwnByValue},
			{name: "moves a value", give: u8, want: rules.OwnByValue},
			{
				name: "lends a shared borrow",
				give: &node.TypeRef{Spelling: "&u8", Form: symbol.FormBorrow, Elems: []*node.TypeRef{u8}},
				want: rules.OwnBorrow,
			},
			{
				name: "lends a mutable borrow for mutation",
				give: &node.TypeRef{
					Spelling: "&mut u8", Form: symbol.FormBorrow, Mutability: symbol.MutabilityMutable,
					Elems: []*node.TypeRef{u8},
				},
				want: rules.OwnBorrowMut,
			},
			{name: "moves the receiver self", give: &node.TypeRef{Spelling: "Self"}, want: rules.OwnByValue},
			{
				name: "moves a receiver of a Box",
				give: &node.TypeRef{
					Spelling: boxName,
					Args:     []*node.TypeRef{{Spelling: "Self"}},
				},
				want: rules.OwnByValue,
			},
			{
				name: "moves a tuple",
				give: &node.TypeRef{Spelling: "(u8,)", Form: symbol.FormTuple},
				want: rules.OwnByValue,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := rustrules.Rules{}.Ownership(rules.ParamView{Name: "p", Ref: tt.give})
				assert.Equal(t, got, tt.want, "the ownership of the parameter")
			})
		}

		t.Run("lends the receiver of a method that saves through it for mutation", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			c, is := rules.NewBound(rustrules.New(), f.view, nil).CallableOf(f.method(t, storeID, "save"))
			assert.True(t, is, "save is callable")
			assert.NotNil(t, c.Receiver, "save has a receiver")
			expect.Equal(t, rustrules.Rules{}.Ownership(*c.Receiver), rules.OwnBorrowMut, "the receiver &mut self")
			assert.Length(t, c.Params, 1, "save takes one parameter")
			expect.Equal(t, rustrules.Rules{}.Ownership(c.Params[0]), rules.OwnBorrow, "the parameter &Order")
		})
	})

	t.Run("Comparable", func(t *testing.T) {
		t.Parallel()

		u8 := &node.TypeRef{Spelling: u8Name}
		guarded := &node.TypeRef{Spelling: "Guarded", Target: guardedID}
		mutex := &node.TypeRef{Spelling: "Mutex", Package: "std/sync", Args: []*node.TypeRef{u8}}
		ioError := &node.TypeRef{Spelling: "io::Error", Package: "std/io"}
		dyn := &node.TypeRef{
			Spelling: "dyn Store", Form: symbol.FormIntersection,
			Elems: []*node.TypeRef{{Spelling: "Store", Target: storeID}},
		}
		iter := &node.TypeRef{
			Spelling: iteratorName,
			Args:     []*node.TypeRef{{Spelling: "Item=u8", Args: []*node.TypeRef{u8}}},
		}
		dynError := &node.TypeRef{
			Spelling: "dyn Error", Form: symbol.FormIntersection, Elems: []*node.TypeRef{{Spelling: errorName}},
		}
		boxedError := &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{dynError}}
		trait := &node.TypeRef{Spelling: "Store", Target: storeID}
		fn := &node.TypeRef{Spelling: "fn()", Form: symbol.FormFunc}
		tests := []struct {
			name string
			give *node.TypeRef
			want []*node.TypeRef
		}{
			{name: "compares a nil reference"},
			{
				name: "compares a form that Rust does not write",
				give: &node.TypeRef{Spelling: "A|B", Form: symbol.FormUnion},
			},
			{name: "compares a primitive", give: u8},
			{name: "compares the unit type", give: &node.TypeRef{Spelling: "()"}},
			{name: "compares fmt::Error", give: &node.TypeRef{Spelling: "fmt::Error", Package: "std/fmt"}},
			{name: "compares a struct that derives PartialEq", give: &node.TypeRef{Spelling: "Order", Target: orderID}},
			{
				name: "compares an enum that derives PartialEq",
				give: &node.TypeRef{Spelling: "Status", Target: statusID},
			},
			{
				name: "compares a map of comparable members",
				give: &node.TypeRef{Spelling: "HashMap", Package: collectionsPkg, Args: []*node.TypeRef{
					{Spelling: stringName}, u8,
				}},
			},
			{name: "compares the target of an alias", give: &node.TypeRef{Spelling: "Ids", Target: idsID}},
			{name: "compares a type of another crate", give: &node.TypeRef{Spelling: "Utc", Package: "chrono"}},
			{name: "compares a declaration outside the view", give: &node.TypeRef{Spelling: "Gone", Target: goneID}},
			{
				name: "compares a type of the crate past its lifetime arguments",
				give: &node.TypeRef{Spelling: "Page", Target: pageID, Args: []*node.TypeRef{{Spelling: "'a"}, u8}},
			},
			{
				name: "does not compare a struct that does not derive PartialEq",
				give: guarded,
				want: []*node.TypeRef{guarded},
			},
			{name: "does not compare a Mutex", give: mutex, want: []*node.TypeRef{mutex}},
			{name: "does not compare io::Error", give: ioError, want: []*node.TypeRef{ioError}},
			{name: "does not compare a trait object", give: dyn, want: []*node.TypeRef{dyn}},
			{name: "does not compare an iterator", give: iter, want: []*node.TypeRef{iter}},
			{
				name: "does not compare a Box of a trait object of the error trait", give: boxedError,
				want: []*node.TypeRef{dynError},
			},
			{name: "does not compare a trait", give: trait, want: []*node.TypeRef{trait}},
			{name: "does not compare a function", give: fn, want: []*node.TypeRef{fn}},
			{
				name: "does not compare a vector of a member that does not compare",
				give: &node.TypeRef{Spelling: vecName, Args: []*node.TypeRef{guarded}}, want: []*node.TypeRef{guarded},
			},
			{
				name: "does not compare a generic struct of an argument that does not compare",
				give: &node.TypeRef{Spelling: "Page", Target: pageID, Args: []*node.TypeRef{mutex}},
				want: []*node.TypeRef{mutex},
			},
			{
				name: "does not compare a result of a failure that does not compare",
				give: &node.TypeRef{
					Spelling: resultName,
					Args:     []*node.TypeRef{u8, ioError},
				},
				want: []*node.TypeRef{ioError},
			},
			{
				name: "does not compare a tuple of a member that does not compare",
				give: &node.TypeRef{
					Spelling: "(u8,Mutex<u8>)",
					Form:     symbol.FormTuple,
					Elems:    []*node.TypeRef{u8, mutex},
				},
				want: []*node.TypeRef{mutex},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				ok, problems := rustrules.Rules{}.Comparable(tt.give, f.view)
				expect.Equal(t, ok, len(tt.want) == 0, "whether the type compares")
				expect.Equal(t, problems, tt.want, "the references that break the equality")
			})
		}

		t.Run("compares a type parameter that a bound extends PartialEq for", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			entries := f.field(t, cacheID, "entries").Type
			assert.Length(t, entries.Args, 2, "the map of the cache has two type arguments")
			ok, _ := rustrules.Rules{}.Comparable(entries.Args[0], f.view)
			expect.True(t, ok, "K: Hash + Eq compares")
			ok, problems := rustrules.Rules{}.Comparable(entries.Args[1], f.view)
			expect.False(t, ok, "V has no bound that compares")
			assert.Length(t, problems, 1, "one reference breaks the equality")
			expect.Equal(t, problems[0], entries.Args[1], "the parameter breaks the equality", assert.ByIdentity())
		})

		t.Run("compares nothing past the budget of the walk", func(t *testing.T) {
			t.Parallel()

			ref := &node.TypeRef{Spelling: "fn()", Form: symbol.FormFunc}
			for range 10 {
				ref = &node.TypeRef{Spelling: "&T", Form: symbol.FormBorrow, Elems: []*node.TypeRef{ref}}
			}
			ok, _ := rustrules.Rules{}.Comparable(ref, rules.View{})
			assert.True(t, ok, "the walk stops before the function")
		})
	})

	t.Run("Derive", func(t *testing.T) {
		t.Parallel()

		unit := &node.TypeRef{Spelling: "()"}
		owned := &node.TypeRef{Spelling: stringName}
		tests := []struct {
			name string
			give *node.TypeParam
			want *node.TypeRef
		}{
			{
				name: "returns the unit type for a parameter without a bound",
				give: &node.TypeParam{Name: "T"},
				want: unit,
			},
			{
				name: "returns the unit type for bounds that the unit type implements",
				give: &node.TypeParam{Name: "T", Bounds: []*node.TypeRef{
					{Spelling: "Clone"}, {Spelling: "std::fmt::Debug", Package: "std/fmt"}, nil,
				}},
				want: unit,
			},
			{
				name: "returns String for a bound that only String implements",
				give: &node.TypeParam{Name: "T", Bounds: []*node.TypeRef{{Spelling: "Display", Package: "std/fmt"}}},
				want: owned,
			},
			{
				name: "returns String for a bound with the argument that String implements it for",
				give: &node.TypeParam{Name: "T", Bounds: []*node.TypeRef{
					{Spelling: "AsRef", Args: []*node.TypeRef{{Spelling: strName}}},
				}},
				want: owned,
			},
			{
				name: "reads past the relaxation ?Sized",
				give: &node.TypeParam{Name: "T", Bounds: []*node.TypeRef{{Spelling: "?Sized"}}},
				want: unit,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, derived := rustrules.Rules{}.Derive(tt.give, rules.View{})
				assert.True(t, derived, "the parameter has a witness")
				assert.Equal(t, got, tt.want, "the witness of the parameter")
			})
		}

		t.Run("returns the default of a parameter", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, is := f.decl(t, cacheID).(*node.Struct)
			assert.True(t, is, "Cache is a struct")
			assert.Length(t, s.TypeParams, 2, "Cache has two type parameters")
			got, derived := rustrules.Rules{}.Derive(s.TypeParams[1], f.view)
			assert.True(t, derived, "V has a witness")
			assert.Equal(t, got, s.TypeParams[1].Default, "the witness is the default", assert.ByIdentity())
		})

		t.Run("returns String for the parameter of a function that displays it", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			fn, is := f.decl(t, showID).(*node.Function)
			assert.True(t, is, "show is a function")
			assert.Length(t, fn.TypeParams, 1, "show has one type parameter")
			got, derived := rustrules.Rules{}.Derive(fn.TypeParams[0], f.view)
			assert.True(t, derived, "T has a witness")
			assert.Equal(t, got.Spelling, stringName, "String implements Display and Clone")
		})

		refusals := []struct {
			name string
			give *node.TypeParam
		}{
			{name: "reports false for a nil parameter"},
			{name: "reports false for a const parameter", give: &node.TypeParam{Name: "N", Const: true}},
			{
				name: "reports false for a bound of a trait of the crate",
				give: &node.TypeParam{Name: "T", Bounds: []*node.TypeRef{{Spelling: "Store", Target: storeID}}},
			},
			{
				name: "reports false for a bound of another crate",
				give: &node.TypeParam{Name: "T", Bounds: []*node.TypeRef{{Spelling: "Serialize", Package: "serde"}}},
			},
			{
				name: "reports false for a bound that neither implements",
				give: &node.TypeParam{Name: "T", Bounds: []*node.TypeRef{{Spelling: "Copy"}, {Spelling: "Display"}}},
			},
			{
				name: "reports false for a bound with another argument",
				give: &node.TypeParam{Name: "T", Bounds: []*node.TypeRef{
					{Spelling: "AsRef", Args: []*node.TypeRef{{Spelling: u8Name}}},
				}},
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, derived := rustrules.Rules{}.Derive(tt.give, rules.View{})
				assert.False(t, derived, "the parameter has no witness")
				assert.Nil(t, got, "no witness is returned")
			})
		}
	})

	t.Run("Reified", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true", func(t *testing.T) {
			t.Parallel()

			assert.True(t, rustrules.Rules{}.Reified(), "Rust keeps the type arguments of an instantiation")
		})
	})
}

// TestCapabilitiesAllocs checks the ceilings of the capabilities in the
// ordinary run, which does not run benchmarks.
func TestCapabilitiesAllocs(t *testing.T) {
	checkAllocs(t, capabilitiesCalls())
}

// BenchmarkCapabilities measures the ownership, the equality and the witness
// under their ceilings.
func BenchmarkCapabilities(b *testing.B) {
	benchCalls(b, capabilitiesCalls())
}

// capabilitiesCalls returns the calls of the capabilities that the
// allocation test and the benchmark measure.
func capabilitiesCalls() []allocCall {
	r := rustrules.Rules{}
	u8 := &node.TypeRef{Spelling: u8Name}
	borrow := rules.ParamView{Ref: &node.TypeRef{Spelling: "&u8", Form: symbol.FormBorrow, Elems: []*node.TypeRef{u8}}}
	param := &node.TypeParam{Name: "T", Default: u8}
	var (
		owned    rules.Ownership
		compares bool
		witness  *node.TypeRef
	)
	return []allocCall{
		{
			name: "Ownership", allocs: ownershipAllocs,
			call:  func() { owned = r.Ownership(borrow) },
			check: func(tb assert.TB) { assert.Equal(tb, owned, rules.OwnBorrow, "Ownership of a borrow") },
		},
		{
			name: "Comparable", allocs: comparableAllocs,
			call:  func() { compares, _ = r.Comparable(u8, rules.View{}) },
			check: func(tb assert.TB) { assert.True(tb, compares, "Comparable of a primitive") },
		},
		{
			name:   "Derive",
			allocs: deriveAllocs,
			call:   func() { witness, _ = r.Derive(param, rules.View{}) },
			check:  func(tb assert.TB) { assert.Equal(tb, witness, u8, "Derive returns the default", assert.ByIdentity()) },
		},
	}
}
