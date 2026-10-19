// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package builder_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/builder"
	"go.dokimi.dev/eidos/reference/defaults"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with Address under the directive, the builder of
// Address, its constructor, the function of the param defaults, and the
// template of the family of the tests.
const (
	catalogPackage = "example.com/shop/catalog"
	addressBuilder = "AddressBuilder"
	newAddress     = "NewAddress"
	seedName       = "defaultAddress"
	testTemplate   = "builder.test.tmpl"
)

// The methods of the builder of Address, the methods of the record of its
// checks, and the nested functions of the record, joined by spaces.
const (
	addressMethods = "WithStreet WithCity WithCountry WithFloor WithLines AppendLines " +
		"WithLabels WithLabelsEntry WithLabelsEntries WithNotes WithNotesEntry WithNotesEntries " +
		"WithRaw WithRawString WithUnit Build"
	addressChecks = "Build WithStreet WithCity WithCountry WithFloor WithLines WithLabels WithNotes WithRaw WithUnit"
	addressTypes  = "NewAddress NewAddressFrom StreetDefault CityDefault CountryDefault"
)

// The assignments of the defaults of Address in the slot of NewAddress.
const (
	streetAssign  = "value.Street"
	cityAssign    = "value.City"
	countryAssign = "value.Country"
	floorAssign   = "value.Floor"
	valueLocal    = "value"
)

// The prefix of the name of a setter, and the prefix of a field that the
// seed of NewAddressFrom assigns.
const (
	setterPrefix = "With"
	seedPrefix   = "seed."
)

// The tree of the generic structs, its package, the builders of its two
// structs, the record of the checks of Page, and the assignment of the
// default of Page.
const (
	genericTree     = "testdata/generic"
	genericPackage  = "example.com/generic"
	pageBuilder     = "PageBuilder"
	newPage         = "NewPage"
	labelledBuilder = "LabelledBuilder"
	pageChecks      = "Build WithItems WithNext"
	pageTypes       = "NewPage NewPageFrom NextDefault"
	nextAssign      = "value.Next"
)

// The tree of a default that is no value of its type, its package, and the
// field of the default.
const (
	badTree    = "testdata/bad"
	badPackage = "example.com/bad"
	shelfName  = "Shelf"
	slotsName  = "Slots"
)

// The tree of the defaults that are names and a call, its package, the
// constructor of Policy, and the package outside the view that a default
// refers to.
const (
	namedTree    = "testdata/named"
	namedPackage = "example.com/named"
	newPolicy    = "NewPolicy"
	timePackage  = "time"
)

// generated is what builder writes over one tree: the builders and the
// constructors by name, and the records of the checks by the name of their
// builder.
type generated struct {
	builders map[string]*emit.Struct
	funcs    map[string]*emit.Function
	checks   map[string]*emit.Struct
}

// builder writes a builder of each struct under its directive and the
// checks of each builder, and passes the plugin suite.
func TestBuilder(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := builder.New()
				return p, fixture(tb, referencetest.Tree, catalogPackage, p).Fixture
			})
		})

		t.Run("writes the setters of each field by the form of its type", func(t *testing.T) {
			t.Parallel()

			got := generate(t, referencetest.Tree, catalogPackage)
			b := got.builders[addressBuilder]
			assert.NotNil(t, b, "builder writes the builder of Address")
			assert.Equal(t, methodNames(b.Methods.Items()), addressMethods, "the builder has the setters of each form")
		})

		t.Run("writes NewAddress that seeds the value through the function of the param defaults", func(t *testing.T) {
			t.Parallel()

			got := generate(t, referencetest.Tree, catalogPackage)
			ctor := got.funcs[newAddress]
			assert.NotNil(t, ctor, "builder writes the constructor of Address")
			slot, declared := ctor.Body.Slot(builder.DefaultsSlot)
			assert.True(t, declared, "the constructor declares the slot of the defaults")
			stmts := slot.Items()
			assert.Length(t, stmts, 5, "the slot calls the function and assigns four defaults")
			expect.That(t, stmts[0].Names).Equal([]string{valueLocal}, "the first statement declares the value")
			expect.True(t, stmts[0].Declare, "the first statement declares the value")
			expect.That(t, stmts[0].Value.Fn.Ref.Name).Equal(seedName, "the value is the result of the function")
			expect.Equal(t, assigned(stmts[1:]), []string{streetAssign, cityAssign, countryAssign, floorAssign},
				"the slot assigns the defaults in the order of the fields")
		})

		t.Run("writes NewPage that declares the value where the directive sets no function", func(t *testing.T) {
			t.Parallel()

			got := generate(t, genericTree, genericPackage)
			ctor := got.funcs[newPage]
			assert.NotNil(t, ctor, "builder writes the constructor of Page")
			slot, _ := ctor.Body.Slot(builder.DefaultsSlot)
			assert.Equal(t, assigned(slot.Items()), []string{nextAssign}, "the slot assigns the one default alone")
		})

		t.Run("records a check of each default and of each setter with a sample", func(t *testing.T) {
			t.Parallel()

			got := generate(t, referencetest.Tree, catalogPackage)
			record := got.checks[addressBuilder]
			assert.NotNil(t, record, "builder records the checks of the builder of Address")
			expect.That(t, methodNames(record.Methods.Items())).Equal(addressChecks,
				"the record has Build and the setters with a sample")
			expect.That(t, typeNames(record.Types.Items())).Equal(addressTypes,
				"the record has the constructors and the defaults other than zero")
		})

		t.Run("records the check of each setter with a value that the seed does not have", func(t *testing.T) {
			t.Parallel()

			got := generate(t, referencetest.Tree, catalogPackage)
			record := got.checks[addressBuilder]
			assert.NotNil(t, record, "builder records the checks of the builder of Address")
			types := record.Types.Items()
			assert.Length(t, types, 5, "the record has the two constructors and three defaults")
			seeded := map[string]emit.Value{}
			for _, s := range types[1].(*emit.Function).Body.Stmts {
				seeded[strings.TrimPrefix(s.Names[0], seedPrefix)] = *s.Value.Val
			}
			for _, m := range record.Methods.Items()[1:] {
				field := strings.TrimPrefix(m.Name, setterPrefix)
				expect.NotEqual(t, *m.Body.Stmts[0].Value.Val, seeded[field],
					"the check of "+m.Name+" sets the alternate sample over the seed")
			}
		})

		t.Run("records a check of a field whose type refers to a type parameter", func(t *testing.T) {
			t.Parallel()

			got := generate(t, genericTree, genericPackage)
			record := got.checks[pageBuilder]
			assert.NotNil(t, record, "builder records the checks of the builder of Page")
			expect.That(t, methodNames(record.Methods.Items())).Equal(pageChecks,
				"the record has the setter of Items at the witness of its parameter")
			expect.That(t, typeNames(record.Types.Items())).Equal(pageTypes, "the record has the default of Next")
		})

		t.Run("records no checks of a generic struct whose type parameter has no witness", func(t *testing.T) {
			t.Parallel()

			got := generate(t, genericTree, genericPackage)
			expect.NotNil(t, got.builders[labelledBuilder], "builder writes the builder of Labelled")
			expect.NotContains(t, got.checks, labelledBuilder, "builder records no checks of Labelled")
		})

		t.Run("assigns the defaults that refer to declarations in the scope of the struct", func(t *testing.T) {
			t.Parallel()

			got := generate(t, namedTree, namedPackage)
			ctor := got.funcs[newPolicy]
			assert.NotNil(t, ctor, "builder writes the constructor of Policy")
			slot, _ := ctor.Body.Slot(builder.DefaultsSlot)
			stmts := slot.Items()
			assert.Length(t, stmts, 3, "the slot assigns the three defaults")
			second := symbol.Identity{
				Lang: golang.Lang, Package: timePackage, Name: "Second", Kind: symbol.KindConstant,
			}
			retries := symbol.Identity{
				Lang: golang.Lang, Package: namedPackage, Name: "Retries", Kind: symbol.KindConstant,
			}
			clock := symbol.Identity{
				Lang: golang.Lang, Package: namedPackage, Name: "SystemClock", Kind: symbol.KindFunction,
			}
			expect.Equal(t, *stmts[0].Value.Val, emit.Name(second), "Timeout takes the constant time.Second")
			expect.Equal(t, *stmts[1].Value.Val, emit.Name(retries), "Attempts takes the constant Retries")
			expect.Equal(t, *stmts[2].Value.Val, emit.Call(clock), "Clock takes the call of SystemClock")
		})

		t.Run("reports BadDefault at a field whose default is no value of its type", func(t *testing.T) {
			t.Parallel()

			p := builder.New()
			f := fixture(t, badTree, badPackage, p)
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			found := slices.Collect(res.Sink.All())
			assert.Length(t, found, 1, "builder reports the one field")
			expect.That(t, found[0].Code).Equal(builder.BadDefault, "the finding is BadDefault")
			expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindField, shelfName, slotsName).Position().Line,
				"the finding is at the field")
			expect.Empty(t, slices.Collect(res.Emit.Units()), "builder writes nothing for Shelf")
		})
	})
}

// fixture returns a fixture over the package pkg of the tree dir, loaded
// under the context of tb and stamped by defaults, with the target golang
// and the plugin builder p.
func fixture(tb assert.TB, dir, pkg string, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(tb, referencetest.Spec{
		Dir:        dir,
		Packages:   []string{pkg},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{p},
		Annotators: []plugin.Annotator{defaults.New()},
	})
}

// generate runs builder over the package pkg of the tree dir, and returns
// what it writes. It checks that the family of the tests claims its files
// through its template, and that the family of the builders claims none.
func generate(t *testing.T, dir, pkg string) generated {
	t.Helper()

	p := builder.New()
	res := fixture(t, dir, pkg, p).Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	got := generated{
		builders: map[string]*emit.Struct{},
		funcs:    map[string]*emit.Function{},
		checks:   map[string]*emit.Struct{},
	}
	for u := range res.Emit.Units() {
		if u.Tag == builder.TestTag {
			expect.That(t, u.Template).Equal(testTemplate, "the family of the tests claims its files")
			for _, d := range u.Decls {
				record := d.(*emit.Struct)
				got.checks[record.Name] = record
			}
			continue
		}
		expect.That(t, u.Template).Equal("", "the family of the builders claims no file")
		for _, d := range u.Decls {
			switch decl := d.(type) {
			case *emit.Struct:
				got.builders[decl.Name] = decl
			case *emit.Function:
				got.funcs[decl.Name] = decl
			}
		}
	}
	return got
}

// methodNames returns the names of the methods, joined by spaces.
func methodNames(ms []*emit.Method) string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return strings.Join(out, " ")
}

// typeNames returns the names of the nested functions of a record, joined
// by spaces.
func typeNames(ss []symbol.Symbol) string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.(*emit.Function).Name)
	}
	return strings.Join(out, " ")
}

// assigned returns the names that the statements assign, in order.
func assigned(stmts []emit.Stmt) []string {
	out := make([]string, 0, len(stmts))
	for _, s := range stmts {
		out = append(out, s.Names...)
	}
	return out
}
