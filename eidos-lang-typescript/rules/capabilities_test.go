// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The type parameter that the substitution cases bind, the spelling of
// TypeScript's top type, and the name of a constructor.
const (
	paramName       = "T"
	unknownSpelling = "unknown"
	constructorName = "constructor"
)

// The allocations of the capabilities above zero.
const (
	// deriveAllocs counts the reference to unknown.
	deriveAllocs = 1
	// substituteAllocs counts the copy of the argument that replaces a
	// parameter.
	substituteAllocs = 1
	// propertiesAllocs counts the binding of the member walk, what the
	// walk allocates, and the list of properties, for a class with one
	// getter and its setter.
	propertiesAllocs = 4
	// constructorsAllocs counts the binding of the projection, the list
	// of callables, and the parameter list of each of the two overloads.
	constructorsAllocs = 4
)

// TestCapabilities checks each optional capability that the TypeScript
// rules implement.
func TestCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("Derive", func(t *testing.T) {
		t.Parallel()

		t.Run("returns unknown for a parameter without a bound", func(t *testing.T) {
			t.Parallel()

			w, ok := tsrules.Rules{}.Derive(&node.TypeParam{Name: paramName}, rules.View{})
			assert.True(t, ok, "a parameter without a bound derives a witness")
			assert.Equal(t, w.Spelling, unknownSpelling, "the witness is the top type, which admits every argument")
		})

		t.Run("returns the bound of a parameter with one bound", func(t *testing.T) {
			t.Parallel()

			bound := &node.TypeRef{Spelling: rowName}
			param := &node.TypeParam{Name: paramName, Bounds: []*node.TypeRef{bound}}
			w, ok := tsrules.Rules{}.Derive(param, rules.View{})
			assert.True(t, ok, "a parameter with one bound derives a witness")
			assert.Equal(t, w, bound, "the witness is the bound itself", assert.ByIdentity())
		})

		t.Run("returns unknown for a parameter whose one bound is nil", func(t *testing.T) {
			t.Parallel()

			param := &node.TypeParam{Name: paramName, Bounds: []*node.TypeRef{nil}}
			w, ok := tsrules.Rules{}.Derive(param, rules.View{})
			assert.True(t, ok, "a nil bound bounds nothing")
			assert.Equal(t, w.Spelling, unknownSpelling, "the witness is the top type")
		})

		refusals := []struct {
			name string
			give *node.TypeParam
		}{
			{name: "reports false for a nil parameter", give: nil},
			{name: "reports false for a parameter with two bounds", give: &node.TypeParam{
				Name: paramName, Bounds: []*node.TypeRef{{Spelling: rowName}, {Spelling: targetName}},
			}},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := tsrules.Rules{}.Derive(tt.give, rules.View{})
				assert.False(t, ok, "Derive reports that no witness derives")
			})
		}
	})

	t.Run("Substitute", func(t *testing.T) {
		t.Parallel()

		paramID := symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: paramName, Kind: symbol.KindTypeParam}
		param := &node.TypeParam{ID: paramID, Name: paramName}
		params := []*node.TypeParam{param}
		arg := &node.TypeRef{Spelling: stringName}
		args := []*node.TypeRef{arg}

		t.Run("replaces a reference that resolved to a parameter with a copy of its argument", func(t *testing.T) {
			t.Parallel()

			got := tsrules.Rules{}.Substitute(&node.TypeRef{Spelling: paramName, Target: param.ID}, params, args)
			expect.Equal(t, got.Spelling, stringName, "the copy has the argument's spelling")
			expect.NotEqual(t, got, arg, "the result is a copy of the argument", assert.ByIdentity())
		})

		t.Run("replaces an unresolved reference with a parameter's name", func(t *testing.T) {
			t.Parallel()

			got := tsrules.Rules{}.Substitute(&node.TypeRef{Spelling: paramName}, params, args)
			assert.Equal(t, got.Spelling, stringName, "the result has the argument's spelling")
		})

		t.Run("keeps a reference that resolved to another declaration of the parameter's name", func(t *testing.T) {
			t.Parallel()

			class := symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: paramName, Kind: symbol.KindStruct}
			ref := &node.TypeRef{Spelling: paramName, Target: class}
			assert.Equal(t, tsrules.Rules{}.Substitute(ref, params, args), ref, "the class T is not a parameter",
				assert.ByIdentity())
		})

		t.Run("restates the children and the arguments that refer to a parameter", func(t *testing.T) {
			t.Parallel()

			list := &node.TypeRef{Spelling: "T[]", Form: symbol.FormList, Elems: []*node.TypeRef{{Spelling: paramName}}}
			generic := &node.TypeRef{Spelling: "Box", Args: []*node.TypeRef{{Spelling: paramName}}}
			gotList := tsrules.Rules{}.Substitute(list, params, args)
			gotGeneric := tsrules.Rules{}.Substitute(generic, params, args)
			expect.Equal(t, gotList.Elems[0].Spelling, stringName, "the element is the argument")
			expect.Equal(t, gotGeneric.Args[0].Spelling, stringName, "the type argument is the argument")
			expect.Equal(t, list.Elems[0].Spelling, paramName, "the input reference is unchanged")
		})

		t.Run("restates the members of an inline object type that refer to a parameter", func(t *testing.T) {
			t.Parallel()

			inline := &node.TypeRef{
				Spelling: "{item:T;get():T}",
				Form:     symbol.FormInline,
				Fields: []*node.Field{
					nil,
					{Name: "item", Type: &node.TypeRef{Spelling: paramName}},
					{Name: "fixed", Type: arg},
				},
				Methods: []*node.Method{nil, {
					Name:    "get",
					Params:  []*node.Param{nil, {Name: "p", Type: &node.TypeRef{Spelling: paramName}}},
					Returns: []*node.Return{nil, {Type: &node.TypeRef{Spelling: paramName}}},
				}, {Name: "plain", Returns: []*node.Return{{Type: arg}}}},
			}
			got := tsrules.Rules{}.Substitute(inline, params, args)
			expect.Equal(t, got.Fields[1].Type.Spelling, stringName, "the field's type is the argument")
			expect.Equal(t, got.Fields[2], inline.Fields[2], "a field without a parameter is the same field",
				assert.ByIdentity())
			expect.Equal(t, got.Methods[1].Params[0].Type.Spelling, stringName,
				"the method's parameter has the argument's type")
			expect.Equal(t, got.Methods[1].Returns[0].Type.Spelling, stringName,
				"the method's result has the argument's type")
			expect.Equal(t, got.Methods[2], inline.Methods[2], "a method without a parameter is the same method",
				assert.ByIdentity())
			expect.Equal(t, inline.Fields[1].Type.Spelling, paramName, "the input reference is unchanged")
		})

		t.Run("returns a reference without a parameter as it is", func(t *testing.T) {
			t.Parallel()

			ref := &node.TypeRef{Spelling: "Box", Args: []*node.TypeRef{{Spelling: numberName}}}
			assert.Equal(t, tsrules.Rules{}.Substitute(ref, params, args), ref, "Substitute returns the same reference",
				assert.ByIdentity())
		})

		unchanged := []struct {
			name   string
			params []*node.TypeParam
			args   []*node.TypeRef
		}{
			{name: "returns the reference for no parameters", params: nil, args: nil},
			{name: "returns the reference for a count of arguments that differs", params: params, args: nil},
			{name: "returns the reference for a nil argument", params: params, args: []*node.TypeRef{nil}},
			{name: "returns the reference for a nil parameter", params: []*node.TypeParam{nil}, args: args},
		}
		for _, tt := range unchanged {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ref := &node.TypeRef{Spelling: paramName}
				assert.Equal(t, tsrules.Rules{}.Substitute(ref, tt.params, tt.args), ref,
					"Substitute returns the same reference", assert.ByIdentity())
			})
		}

		t.Run("returns nil for a nil reference", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, tsrules.Rules{}.Substitute(nil, params, args), "Substitute returns nil")
		})
	})

	t.Run("Reified", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false", func(t *testing.T) {
			t.Parallel()

			assert.False(t, tsrules.Rules{}.Reified(), "a generic value has no runtime type arguments")
		})
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()

		t.Run("pairs each instance getter with the setter of its name", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			props := tsrules.Rules{}.Properties(f.class(t, "Accessors"), f.view)
			assert.Length(t, props, 2,
				"the properties are size and readOnly, and neither writeOnly nor the static shared")
			expect.Equal(t, props[0].Name, "size", "the first property is size")
			expect.Equal(t, props[0].Type.Spelling, numberName, "the type of size is its getter's type")
			expect.NotNil(t, props[0].Setter, "size has its setter")
			expect.Equal(t, props[1].Name, "readOnly", "the second property is readOnly")
			expect.Nil(t, props[1].Setter, "readOnly has no setter")
		})

		t.Run("returns a property without a type for a getter without a return", func(t *testing.T) {
			t.Parallel()

			s := &node.Struct{Name: "Bare", Methods: []*node.Method{{Name: "x", Accessor: symbol.AccessorGet}}}
			props := tsrules.Rules{}.Properties(s, rules.View{})
			assert.Length(t, props, 1, "the getter is a property")
			assert.Nil(t, props[0].Type, "the property has no type")
		})
	})

	t.Run("Constructors", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each overload of a declared constructor", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got := tsrules.Rules{}.Constructors(f.class(t, "Overloaded"), f.view)
			assert.Length(t, got, 2, "Constructors returns the two overload signatures")
			expect.Equal(t, got[0].Params[0].Ref.Spelling, stringName, "the first takes a string")
			expect.Equal(t, got[1].Params[0].Ref.Spelling, numberName, "the second takes a number")
		})

		t.Run("returns the constructor of the class that a class extends", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got := tsrules.Rules{}.Constructors(f.class(t, derivedName), f.view)
			assert.Length(t, got, 1, "Constructors returns the constructor of Base")
			assert.Equal(t, got[0].Params[0].Name, "kind", "the constructor takes the kind")
		})

		t.Run("returns the constructor of a generic class with the arguments of the extends clause",
			func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got := tsrules.Rules{}.Constructors(f.class(t, "Concrete"), f.view)
				assert.Length(t, got, 1, "Constructors returns the constructor of Generic")
				assert.Equal(t, got[0].Params[0].Ref.Spelling, stringName,
					"the item has the type string that Concrete binds")
			})

		t.Run("returns the implicit constructor of a class without one", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			assert.Equal(t, tsrules.Rules{}.Constructors(f.class(t, rowName), f.view),
				[]rules.Callable{{Name: constructorName}}, "the implicit constructor has no parameters")
		})

		empty := []struct {
			name  string
			class string
		}{
			{name: "returns no constructor of a class whose constructor is private", class: "Hidden"},
			{name: "returns no constructor of a class that extends an interface", class: "OffReader"},
			{name: "returns no constructor below the default depth of a chain of classes", class: "L0"},
		}
		for _, tt := range empty {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				assert.Empty(t, tsrules.Rules{}.Constructors(f.class(t, tt.class), f.view),
					"the class has no public constructor")
			})
		}

		t.Run("returns the implicit constructor of a class with a nil base", func(t *testing.T) {
			t.Parallel()

			s := &node.Struct{Name: "Free", Extends: []*node.TypeRef{nil}}
			assert.Equal(t, tsrules.Rules{}.Constructors(s, rules.View{}), []rules.Callable{{Name: constructorName}},
				"a nil base is not a class")
		})
	})

	t.Run("Comparable", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for every type", func(t *testing.T) {
			t.Parallel()

			ok, problems := tsrules.Rules{}.Comparable(&node.TypeRef{Spelling: "string[]", Form: symbol.FormList},
				rules.View{})
			expect.True(t, ok, "=== compares any two values")
			expect.Empty(t, problems, "Comparable reports no problem")
		})
	})
}

// TestCapabilitiesAllocs checks the allocation ceiling of each
// capability. The ordinary test run runs no benchmark, so this test
// checks the ceilings.
func TestCapabilitiesAllocs(t *testing.T) {
	checkAllocs(t, capabilitiesCalls(t))
}

// BenchmarkCapabilities measures each capability under its ceiling.
func BenchmarkCapabilities(b *testing.B) {
	benchCalls(b, capabilitiesCalls(b))
}

// capabilitiesCalls returns the measured calls of capabilities.go.
func capabilitiesCalls(tb assert.TB) []allocCall {
	f := loaded(tb)
	r := tsrules.Rules{}
	param := &node.TypeParam{Name: paramName}
	params := []*node.TypeParam{param}
	args := []*node.TypeRef{{Spelling: stringName}}
	ref := &node.TypeRef{Spelling: paramName}
	accessors := f.class(tb, "Accessors")
	overloaded := f.class(tb, "Overloaded")
	var (
		witness  *node.TypeRef
		ok       bool
		reified  bool
		bound    *node.TypeRef
		props    []rules.Property
		ctors    []rules.Callable
		problems []*node.TypeRef
	)
	return []allocCall{
		{
			name: "Derive", allocs: deriveAllocs,
			call: func() { witness, ok = r.Derive(param, f.view) },
			check: func(tb assert.TB) {
				assert.True(tb, ok, "Derive derives a witness")
				assert.Equal(tb, witness.Spelling, unknownSpelling, "Derive returns unknown")
			},
		},
		{
			name: "Substitute", allocs: substituteAllocs,
			call: func() { bound = r.Substitute(ref, params, args) },
			check: func(tb assert.TB) {
				assert.Equal(tb, bound.Spelling, stringName, "Substitute replaces the parameter")
			},
		},
		{
			name:  "Reified",
			call:  func() { reified = r.Reified() },
			check: func(tb assert.TB) { assert.False(tb, reified, "Reified reports false") },
		},
		{
			name: "Properties", allocs: propertiesAllocs,
			call:  func() { props = r.Properties(accessors, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, props, 2, "Properties returns the two properties") },
		},
		{
			name: "Constructors", allocs: constructorsAllocs,
			call:  func() { ctors = r.Constructors(overloaded, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, ctors, 2, "Constructors returns the two overloads") },
		},
		{
			name: "Comparable",
			call: func() { ok, problems = r.Comparable(ref, f.view) },
			check: func(tb assert.TB) {
				assert.True(tb, ok, "Comparable reports the type comparable")
				assert.Empty(tb, problems, "Comparable reports no problem")
			},
		},
	}
}
