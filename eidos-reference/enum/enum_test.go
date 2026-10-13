// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package enum_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/reference/enum"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with Status under the directive, Status, its
// variants with their texts, and the names that enum writes for it.
const (
	catalogPackage = "example.com/shop/catalog"
	statusName     = "Status"
	statusDraft    = "StatusDraft"
	statusListed   = "StatusListed"
	statusRetired  = "StatusRetired"
	draftText      = `"draft"`
	listedText     = `"listed"`
	retiredText    = `"retired"`
	parseStatus    = "ParseStatus"
	statusValues   = "StatusValues"
	unknownStatus  = "ErrUnknownStatus"
	statusUnknown  = `"????????"`
	statusOutside  = `""`
)

// The tree of the enumerations of each kind, its package, its numeric
// enumeration with its variants and their texts, and its enumeration with
// String and the one without methods.
const (
	kindsTree    = "testdata/kinds"
	kindsPackage = "example.com/kinds"
	levelName    = "Level"
	levelLow     = "LevelLow"
	levelHigh    = "LevelHigh"
	lowText      = `"Low"`
	highText     = `"High"`
	levelOutside = "2"
	modeName     = "Mode"
	quietName    = "Quiet"
)

// The tree of two variants of one text, its package, its enumeration and
// the second variant.
const (
	clashTree    = "testdata/clash"
	clashPackage = "example.com/clash"
	clashName    = "Clash"
	clashSecond  = "ClashSecond"
)

// The tree of a variant of text whose value another package decides, its
// package, its enumeration and that variant.
const (
	unknownTree    = "testdata/unknown"
	unknownPackage = "example.com/unknown"
	regionName     = "Region"
	regionHost     = "RegionHost"
)

// The templates of the two families.
const (
	enumTemplate = "enum.tmpl"
	testTemplate = "enum.test.tmpl"
)

// recorded is the data that enum records over one tree, by the name of the
// enumeration: into the family of the enumerations, and into the family of
// the tests.
type recorded struct {
	surfaces map[string]enum.Surface
	tests    map[string]enum.Surface
}

// enum records the text of each variant of an enumeration under its
// directive and the methods that it writes, and passes the plugin suite.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := enum.New()
				return p, fixture(tb, referencetest.Tree, catalogPackage, p).Fixture
			})
		})

		t.Run("records the declared values of an enumeration of text as its texts", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage).surfaces[statusName]
			expect.Equal(t, got.Variants, []enum.Variant{
				{Name: statusDraft, Text: draftText},
				{Name: statusListed, Text: listedText},
				{Name: statusRetired, Text: retiredText},
			}, "each variant has its declared value as its text")
			expect.True(t, got.Textual, "the variants have text values")
		})

		t.Run(
			"records the names without the name of the type as the texts of a numeric enumeration",
			func(t *testing.T) {
				t.Parallel()

				got := record(t, kindsTree, kindsPackage).surfaces[levelName]
				expect.Equal(t, got.Variants, []enum.Variant{
					{Name: levelLow, Text: lowText},
					{Name: levelHigh, Text: highText},
				}, "each variant has its name without the name of the type as its text")
				expect.False(t, got.Textual, "the variants have numeric values")
			},
		)

		t.Run("records the names of the parser, the list and the error of the enumeration", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage).surfaces[statusName]
			expect.Equal(t, got.Parse, parseStatus, "the parser joins Parse onto the name of the type")
			expect.Equal(t, got.Values, statusValues, "the list joins Values onto the name of the type")
			expect.Equal(t, got.Unknown, unknownStatus, "the error joins ErrUnknown onto the name of the type")
		})

		t.Run("records a text longer than the text of each variant as the unknown text", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage).surfaces[statusName]
			assert.Equal(t, got.UnknownText, statusUnknown, "the unknown text is one mark longer than retired")
		})

		t.Run("records the text that no variant has as the value outside an enumeration of text", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage).surfaces[statusName]
			assert.Equal(t, got.OutOfRange, statusOutside, "no variant of Status has the empty text")
		})

		t.Run("records the number above the variants as the value outside a numeric enumeration", func(t *testing.T) {
			t.Parallel()

			got := record(t, kindsTree, kindsPackage).surfaces[levelName]
			assert.Equal(t, got.OutOfRange, levelOutside, "the largest level is 1")
		})

		t.Run("writes each method of an enumeration that declares none", func(t *testing.T) {
			t.Parallel()

			got := record(t, kindsTree, kindsPackage).surfaces[levelName]
			expect.True(t, got.String, "enum writes String")
			expect.True(t, got.Valid, "enum writes IsValid")
			expect.True(t, got.Marshal, "enum writes MarshalText")
			expect.True(t, got.Unmarshal, "enum writes UnmarshalText")
		})

		t.Run("withholds String and the methods of text of an enumeration that declares String", func(t *testing.T) {
			t.Parallel()

			got := record(t, kindsTree, kindsPackage).surfaces[modeName]
			expect.False(t, got.String, "enum withholds String")
			expect.True(t, got.Valid, "enum writes IsValid")
			expect.False(t, got.Marshal, "enum withholds MarshalText")
			expect.False(t, got.Unmarshal, "enum withholds UnmarshalText")
		})

		t.Run("records no declarations of an enumeration whose param methods is false", func(t *testing.T) {
			t.Parallel()

			got := record(t, kindsTree, kindsPackage)
			expect.NotContains(t, got.surfaces, quietName, "enum records nothing for Quiet into its family")
			expect.Contains(t, got.tests, quietName, "enum records the checks of Quiet")
		})

		t.Run("records the data of the declarations into the family of the tests", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage)
			assert.Equal(t, got.tests[statusName], got.surfaces[statusName],
				"the checks read the data of the declarations")
		})

		t.Run("reports DuplicateText at the second variant of one text", func(t *testing.T) {
			t.Parallel()

			p := enum.New()
			f := fixture(t, clashTree, clashPackage, p)
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			found := slices.Collect(res.Sink.All())
			assert.Length(t, found, 1, "enum reports the one variant")
			expect.That(t, found[0].Code).Equal(enum.DuplicateText, "the finding is DuplicateText")
			expect.That(t, found[0].Pos.Line).Equal(
				f.Find(t, symbol.KindEnumVariant, clashName, clashSecond).Position().Line,
				"the finding is at the second variant")
			expect.Empty(t, slices.Collect(res.Emit.Units()), "enum records nothing for Clash")
		})

		t.Run("reports UnknownText at a variant of text without a value", func(t *testing.T) {
			t.Parallel()

			p := enum.New()
			f := fixture(t, unknownTree, unknownPackage, p)
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			found := slices.Collect(res.Sink.All())
			assert.Length(t, found, 1, "enum reports the one variant")
			expect.That(t, found[0].Code).Equal(enum.UnknownText, "the finding is UnknownText")
			expect.That(t, found[0].Pos.Line).Equal(
				f.Find(t, symbol.KindEnumVariant, regionName, regionHost).Position().Line,
				"the finding is at the variant without a value")
			expect.Empty(t, slices.Collect(res.Emit.Units()), "enum records nothing for Region")
		})

		t.Run("reports NoProjection at an enumeration of a language without the projection", func(t *testing.T) {
			t.Parallel()

			p := enum.New()
			f := plugintest.Loaded(t, plugintest.Source{
				Tree:      os.DirFS(filepath.FromSlash(kindsTree)),
				Brand:     referencetest.Brand,
				Frontends: []plugin.Frontend{gofrontend.New(nil)},
				Plugins:   []plugin.Plugin{p},
				Packages:  []string{kindsPackage},
			})
			f.Target = golang.Target
			res := f.Generate(t, p)
			assert.NoError(t, res.Err, "the generator runs whole")
			var codes []diag.Code
			for d := range res.Sink.All() {
				codes = append(codes, d.Code)
			}
			assert.Equal(t, codes,
				[]diag.Code{rules.AbsentRules, enum.NoProjection, enum.NoProjection, enum.NoProjection},
				"the kernel reports the absent rules once, and enum reports each of the three enumerations")
			expect.Empty(t, slices.Collect(res.Emit.Units()), "enum records nothing")
		})
	})
}

// fixture returns a fixture over the package pkg of the tree dir, loaded
// under the context of tb, with the target golang and the plugin enum p.
func fixture(tb assert.TB, dir, pkg string, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(tb, referencetest.Spec{
		Dir:      dir,
		Packages: []string{pkg},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// record runs enum over the package pkg of the tree dir, and returns the
// data that it records. It checks that each family claims its files
// through its template, and that each record is an enumeration.
func record(t *testing.T, dir, pkg string) recorded {
	t.Helper()

	p := enum.New()
	res := fixture(t, dir, pkg, p).Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	got := recorded{surfaces: map[string]enum.Surface{}, tests: map[string]enum.Surface{}}
	for u := range res.Emit.Units() {
		into, want := got.surfaces, enumTemplate
		if u.Tag == enum.TestTag {
			into, want = got.tests, testTemplate
		}
		expect.That(t, u.Template).Equal(want, "the family claims its files through its template")
		for _, d := range u.Decls {
			record := d.(*emit.Enum)
			into[record.Name] = u.Data[record.Origin].(enum.Surface)
		}
	}
	return got
}
