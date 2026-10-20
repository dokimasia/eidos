// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The name of the type parameter of the witness cases, the spelling of
// the top type of TypeScript, and the name of a constructor.
const (
	paramName       = "T"
	unknownSpelling = "unknown"
	constructorName = "constructor"
)

// The error fixture: its classes, and the fields that the error contract
// reads.
const (
	notFoundName = "NotFound"
	labelledName = "Labelled"
	domainName   = "Domain"
	rangeName    = "OutOfRange"
	plainName    = "Plain"
	keptName     = "Kept"
	deepName     = "E0"
	shallowName  = "E1"
	manyName     = "Many"
	causeName    = "cause"
	messageName  = "message"
	optionsName  = "options"
	errorsName   = "errors"
)

// errorTree is a module of error classes. NotFound extends Error and
// declares a cause, Labelled declares a message, Domain extends NotFound,
// OutOfRange extends RangeError, Many extends AggregateError, Plain extends
// nothing, and Kept extends Plain. E0 reaches Error through nine classes,
// and E1 through eight.
var errorTree = fstest.MapFS{
	aFile: {Data: []byte(`export class NotFound extends Error {
  cause?: Error;
}

export class Labelled extends Error {
  message = "labelled";
}

export class Domain extends NotFound {}

export class OutOfRange extends RangeError {}

export class Many extends AggregateError {}

export class Plain {}

export class Kept extends Plain {}

export class E0 extends E1 {}
export class E1 extends E2 {}
export class E2 extends E3 {}
export class E3 extends E4 {}
export class E4 extends E5 {}
export class E5 extends E6 {}
export class E6 extends E7 {}
export class E7 extends E8 {}
export class E8 extends Error {}
`)},
}

// The allocations of the capabilities above zero.
const (
	// deriveAllocs counts the reference to unknown.
	deriveAllocs = 1
	// propertiesAllocs counts the binding of the member walk, what the
	// walk allocates, and the list of properties, for a class with one
	// getter and its setter.
	propertiesAllocs = 4
	// constructorsAllocs counts the binding of the projection, the list
	// of callables, and the parameter list of each of the two overloads.
	constructorsAllocs = 4
	// errorConstructorAllocs counts the binding of the projection, the
	// list of one callable, and the parameter list of the constructor of
	// Error.
	errorConstructorAllocs = 3
	// errorContractAllocs counts the binding of the member walk and what
	// the walk of NotFound allocates.
	errorContractAllocs = 3
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
			{name: "does not return a constructor of a class whose constructor is private", class: "Hidden"},
			{name: "does not return a constructor of a class that extends an interface", class: "OffReader"},
			{name: "does not return a constructor below the default depth of a chain of classes", class: "L0"},
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

		errorConstructors := []struct {
			name      string
			giveClass string
		}{
			{name: "returns the constructor of Error for a class that extends Error", giveClass: notFoundName},
			{name: "returns the constructor of Error at the end of a chain of classes", giveClass: domainName},
			{name: "returns the constructor of a native error class that a class extends", giveClass: rangeName},
		}
		for _, tt := range errorConstructors {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loadedFrom(t, errorTree)
				got := tsrules.Rules{}.Constructors(f.class(t, tt.giveClass), f.view)
				assert.Length(t, got, 1, "the error class declares one constructor")
				assert.Length(t, got[0].Params, 2, "the constructor takes a message and the options")
				expect.Equal(t, got[0].Params[0].Name, messageName, "the first parameter is the message")
				expect.Equal(t, got[0].Params[0].Ref.Spelling, typescript.TypeString, "the message is a string")
				expect.True(t, got[0].Params[0].Optional, "a call can leave the message out")
				expect.Equal(t, got[0].Params[1].Name, optionsName, "the second parameter is the options")
				expect.Equal(t, got[0].Params[1].Ref.Spelling, typescript.TypeErrorOptions,
					"the options are ErrorOptions")
				expect.True(t, got[0].Params[1].Optional, "a call can leave the options out")
			})
		}

		t.Run("returns the constructor of AggregateError for a class that extends it", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, errorTree)
			got := tsrules.Rules{}.Constructors(f.class(t, manyName), f.view)
			assert.Length(t, got, 1, "AggregateError declares one constructor")
			assert.Length(t, got[0].Params, 3, "the constructor takes the errors, a message and the options")
			errors := got[0].Params[0]
			expect.Equal(t, errors.Name, errorsName, "the first parameter is the errors")
			expect.Equal(t, errors.Ref.Spelling, typescript.TypeIterable, "the errors are an Iterable")
			expect.Equal(t, errors.Ref.Args, []*node.TypeRef{{Spelling: typescript.TypeAny}},
				"of any value")
			expect.False(t, errors.Optional, "a call passes the errors")
			expect.Equal(t, got[0].Params[1].Name, messageName, "the second parameter is the message")
			expect.Equal(t, got[0].Params[2].Name, optionsName, "the third parameter is the options")
		})

		t.Run("does not return a constructor of an Error that a module exports", func(t *testing.T) {
			t.Parallel()

			s := &node.Struct{Name: plainName, Extends: []*node.TypeRef{{Spelling: "Error", Package: outsideLib}}}
			assert.Empty(t, tsrules.Rules{}.Constructors(s, rules.View{}),
				"the Error of a module is not a global error class")
		})

		t.Run("returns the constructor of Error for a class that extends the Error of a library", func(t *testing.T) {
			t.Parallel()

			s := &node.Struct{Name: plainName, Extends: []*node.TypeRef{{Spelling: "Error", Target: libraryError}}}
			got := tsrules.Rules{}.Constructors(s, rules.View{})
			assert.Length(t, got, 1, "the Error of the library declares one constructor")
			assert.Length(t, got[0].Params, 2, "the constructor takes a message and the options")
		})

		t.Run("does not return a constructor of a class that extends an interface of a library", func(t *testing.T) {
			t.Parallel()

			s := &node.Struct{Name: plainName, Extends: []*node.TypeRef{{Spelling: "Map", Target: libraryMap}}}
			assert.Empty(t, tsrules.Rules{}.Constructors(s, rules.View{}), "the Map of the library is an interface")
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

	t.Run("ErrorContract", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the field cause of a class that extends Error", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, errorTree)
			c, isError := tsrules.Rules{}.ErrorContract(f.class(t, notFoundName), f.view)
			assert.True(t, isError, "NotFound is an error")
			expect.That(t, c.Cause).NotNil("NotFound declares a cause")
			expect.Nil(t, c.Message, "NotFound inherits its message from Error")
			expect.Nil(t, c.Unwrap, "TypeScript has no hooks")
		})

		t.Run("returns the field message that a class declares", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, errorTree)
			c, _ := tsrules.Rules{}.ErrorContract(f.class(t, labelledName), f.view)
			message, isField := c.Message.(*node.Field)
			assert.True(t, isField, "the message is a field")
			assert.Equal(t, message.Name, messageName, "the field message")
		})

		t.Run("returns the cause that a class inherits", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, errorTree)
			c, isError := tsrules.Rules{}.ErrorContract(f.class(t, domainName), f.view)
			assert.True(t, isError, "Domain is an error through NotFound")
			assert.Equal(t, c.Cause.Name, causeName, "the field cause of NotFound")
		})

		t.Run("returns the contract of a class that extends another global error class", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, errorTree)
			_, isError := tsrules.Rules{}.ErrorContract(f.class(t, rangeName), f.view)
			assert.True(t, isError, "OutOfRange extends RangeError")
		})

		t.Run("returns the contract of a class at the default depth", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, errorTree)
			_, isError := tsrules.Rules{}.ErrorContract(f.class(t, shallowName), f.view)
			assert.True(t, isError, "E1 reaches Error through eight classes")
		})

		refusals := []struct {
			name string
			give string
		}{
			{name: "reports false for a class that extends nothing", give: plainName},
			{name: "reports false for a class that extends a class that is not an error", give: keptName},
			{name: "reports false for a chain deeper than the default depth", give: deepName},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loadedFrom(t, errorTree)
				_, isError := tsrules.Rules{}.ErrorContract(f.class(t, tt.give), f.view)
				assert.False(t, isError, "the class is not an error type")
			})
		}

		constructed := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "reports false for a declaration that is not a class", give: &node.Interface{Name: plainName}},
			{name: "reports false for a nil base", give: &node.Struct{Name: plainName, Extends: []*node.TypeRef{nil}}},
			{
				name: "reports false for an Error that a module exports",
				give: &node.Struct{Name: plainName, Extends: []*node.TypeRef{{Spelling: "Error", Package: outsideLib}}},
			},
			{
				name: "reports false for a base that the view does not declare",
				give: &node.Struct{Name: plainName, Extends: []*node.TypeRef{{Spelling: rowName, Target: rowID}}},
			},
			{
				name: "reports false for a declaration of a library that is no error class",
				give: &node.Struct{Name: plainName, Extends: []*node.TypeRef{{Spelling: "Map", Target: libraryMap}}},
			},
		}
		for _, tt := range constructed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, isError := tsrules.Rules{}.ErrorContract(tt.give, rules.View{})
				assert.False(t, isError, "the declaration is not an error type")
			})
		}

		t.Run("returns the contract of a class that extends the Error of a library", func(t *testing.T) {
			t.Parallel()

			f := loadedWithLibrary(t, fstest.MapFS{"fault.ts": {Data: []byte(
				"export class Fault extends Error {\n  cause?: unknown;\n}\n",
			)}})
			class, isClass := f.decl(t, "fault", "Fault", symbol.KindStruct).(*node.Struct)
			assert.True(t, isClass, "Fault is a class")
			assert.True(t, typescript.Library(class.Extends[0].Target.Package), "Error resolves to the library")
			c, isError := tsrules.Rules{}.ErrorContract(class, f.view)
			assert.True(t, isError, "Fault is an error")
			expect.Nil(t, c.Message, "Fault inherits its message from the Error of the library")
			assert.NotNil(t, c.Cause, "Fault declares a cause")
			expect.Equal(t, c.Cause.Name, causeName, "the cause of Fault, not the cause of the library")
			expect.Equal(t, c.Cause.Pos.File, "fault.ts", "the field of the class")
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
	ref := &node.TypeRef{Spelling: paramName}
	accessors := f.class(tb, "Accessors")
	overloaded := f.class(tb, "Overloaded")
	ef := loadedFrom(tb, errorTree)
	notFound := ef.class(tb, notFoundName)
	var (
		witness  *node.TypeRef
		ok       bool
		reified  bool
		props    []rules.Property
		ctors    []rules.Callable
		problems []*node.TypeRef
		contract rules.ErrorContract
	)
	return []allocCall{
		{
			name: "ErrorContract", allocs: errorContractAllocs,
			call: func() { contract, ok = r.ErrorContract(notFound, ef.view) },
			check: func(tb assert.TB) {
				assert.True(tb, ok, "ErrorContract takes NotFound as an error")
				assert.NotNil(tb, contract.Cause, "ErrorContract finds the cause")
			},
		},
		{
			name: "Derive", allocs: deriveAllocs,
			call: func() { witness, ok = r.Derive(param, f.view) },
			check: func(tb assert.TB) {
				assert.True(tb, ok, "Derive derives a witness")
				assert.Equal(tb, witness.Spelling, unknownSpelling, "Derive returns unknown")
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
			name: "Constructors", caseName: "two overloads", allocs: constructorsAllocs,
			call:  func() { ctors = r.Constructors(overloaded, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, ctors, 2, "Constructors returns the two overloads") },
		},
		{
			name: "Constructors", caseName: "the constructor of Error", allocs: errorConstructorAllocs,
			call:  func() { ctors = r.Constructors(notFound, ef.view) },
			check: func(tb assert.TB) { assert.Length(tb, ctors, 1, "Constructors returns the constructor of Error") },
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
