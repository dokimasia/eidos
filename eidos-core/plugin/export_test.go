// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The plan every export case names, the plugin and the family its
// units come from, and the files it renders them into.
const (
	exportPlan              = "stubs"
	exportPlugin  plugin.ID = "gen"
	exportTag               = "test"
	exportFile              = "svc/a_gen.src"
	exportOther             = "svc/b_gen.src"
	exportForeign           = "external"
)

// exportPkg is the package the case files declare.
var exportPkg = symbol.Identity{Lang: "fixture", Package: "svc", Name: "svc", Kind: symbol.KindPackage}

// An export lists what a plan rendered, keyed the way a dependent
// knows a declaration before the run, and spelled the way the settle
// left it.
func TestExport(t *testing.T) {
	t.Parallel()

	t.Run("NewExport", func(t *testing.T) {
		t.Parallel()

		t.Run("names the producing plan", func(t *testing.T) {
			t.Parallel()

			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", boxWithItem()))
			assert.Equal(t, doc.Plan, exportPlan, "the export's plan")
		})

		t.Run("keys a file-level declaration by its emitted name", func(t *testing.T) {
			t.Parallel()

			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", boxWithItem()))
			box := exported(t, doc, "box")
			assert.Equal(t, box.ExportKey, plugin.ExportKey{
				Origin: settleOrigin("box", symbol.KindStruct), Plugin: exportPlugin, Name: "box",
			}, "the struct's key")
		})

		t.Run("spells a declaration as the settle respelled it", func(t *testing.T) {
			t.Parallel()

			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", boxWithItem()))
			assert.Equal(t, exported(t, doc, "box").Spelling, "Box", "the struct's settled spelling")
		})

		t.Run("records a declaration's kind", func(t *testing.T) {
			t.Parallel()

			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", boxWithItem()))
			assert.Equal(t, exported(t, doc, "item").Kind, symbol.KindField, "the field's kind")
		})

		t.Run("keys a member under its host's emitted name", func(t *testing.T) {
			t.Parallel()

			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", boxWithItem()))
			item := exported(t, doc, "item")
			assert.Equal(t, item.Host, "box", "the field's host")
			assert.Equal(t, item.Spelling, "Item", "the field's settled spelling")
		})

		t.Run("keys each member under the nearest declaration it is declared in", func(t *testing.T) {
			t.Parallel()

			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", shapeSum()))
			assert.Equal(t, exported(t, doc, "radius").Host, "circle", "the first variant's field")
			assert.Equal(t, exported(t, doc, "square").Host, "shape", "the variant after the first variant's field")
			assert.Equal(t, exported(t, doc, "side").Host, "square", "the second variant's field")
		})

		t.Run("keys a method under the emitted name of its receiver's type", func(t *testing.T) {
			t.Parallel()

			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", boxWithItem(), receiving("box")))
			assert.Equal(t, exported(t, doc, "fetch").Host, "box", "the method's host")
		})

		t.Run("keys a method under its receiver's spelling where no file declares the type", func(t *testing.T) {
			t.Parallel()

			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", receiving(exportForeign)))
			assert.Equal(t, exported(t, doc, "fetch").Host, exportForeign, "the method's host")
		})

		t.Run("lists no parameter, result or type parameter", func(t *testing.T) {
			t.Parallel()

			load := &emit.Function{
				Origin:     settleOrigin("load", symbol.KindFunction),
				Name:       "load",
				TypeParams: []*emit.TypeParam{{Name: "T"}},
				Params:     []*emit.Param{{Name: "count", Type: &emit.TypeRef{Spelling: "int"}}},
				Returns:    []*emit.Return{{Name: "err", Type: &emit.TypeRef{Spelling: "error"}}},
			}
			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", load))
			assert.Equal(t, emittedNames(doc), []string{"load"}, "the export's names")
		})

		t.Run("keys a declaration under its unit's plugin and family", func(t *testing.T) {
			t.Parallel()

			u := settleUnit("svc", "svc/a.src", boxWithItem())
			u.Tag = exportTag
			box := exported(t, exportOf(t, capitalizing(), u), "box")
			assert.Equal(t, box.Plugin, exportPlugin, "the struct's plugin")
			assert.Equal(t, box.Tag, exportTag, "the struct's family")
		})

		t.Run("records the file's path and package", func(t *testing.T) {
			t.Parallel()

			box := exported(t, exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", boxWithItem())), "box")
			assert.Equal(t, box.File, exportFile, "the struct's file")
			assert.Equal(t, box.Package, exportPkg, "the struct's package")
		})

		t.Run("records the zero origin for a declaration without one", func(t *testing.T) {
			t.Parallel()

			free := &emit.Constant{Name: "limit", Value: "1"}
			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", free))
			assert.True(t, exported(t, doc, "limit").Origin.IsZero(), "the constant's origin is zero")
		})

		t.Run("reads every name as emitted for a nil store", func(t *testing.T) {
			t.Parallel()

			box := boxWithItem()
			box.Name = "Box"
			u := settleUnit("svc", "svc/a.src", box)
			file := plugin.File{Path: exportFile, Pkg: exportPkg, Units: []plugin.Unit{u}}
			doc := plugin.NewExport(exportPlan, []plugin.File{file}, nil)
			assert.Equal(t, exported(t, doc, "Box").Spelling, "Box", "the struct's spelling")
		})

		t.Run("sorts the declarations by key", func(t *testing.T) {
			t.Parallel()

			late := &emit.Struct{Origin: settleOrigin("zeta", symbol.KindStruct), Name: "zeta"}
			early := &emit.Struct{Origin: settleOrigin("alpha", symbol.KindStruct), Name: "alpha"}
			doc := exportOf(t, capitalizing(), settleUnit("svc", "svc/a.src", late, early))
			assert.Equal(t, emittedNames(doc), []string{"alpha", "zeta"}, "the export's names")
		})

		t.Run("sorts one key's declarations by file", func(t *testing.T) {
			t.Parallel()

			doc := keyed(1)
			assert.Equal(t, []string{doc.Symbols[0].File, doc.Symbols[1].File},
				[]string{exportFile, exportOther}, "the files of one key")
		})
	})

	t.Run("Find", func(t *testing.T) {
		t.Parallel()

		key := plugin.ExportKey{Origin: settleOrigin("row1", symbol.KindStruct), Plugin: exportPlugin, Name: "row1"}

		t.Run("returns every declaration under a key in file order", func(t *testing.T) {
			t.Parallel()

			found := keyed(3).Find(key)
			files := make([]string, 0, len(found))
			for _, s := range found {
				files = append(files, s.File)
			}
			assert.Equal(t, files, []string{exportFile, exportOther}, "the files of the key's declarations")
		})

		t.Run("returns nothing for a key the export does not list", func(t *testing.T) {
			t.Parallel()

			missing := key
			missing.Name = "absent"
			assert.Length(t, keyed(3).Find(missing), 0, "the declarations under an unlisted key")
		})

		t.Run("returns a result whose capacity ends at its last match", func(t *testing.T) {
			t.Parallel()

			found := keyed(3).Find(key)
			assert.Equal(t, cap(found), len(found), "the result's capacity")
		})
	})
}

// NewExport allocates its result and its sorted declarations, and Find
// allocates nothing. The checks run alone, because AllocsPerRun counts
// every goroutine's allocations and refuses to run beside parallel
// tests.
func TestExportAllocs(t *testing.T) {
	files := keyedFiles(1000)
	built := testing.AllocsPerRun(10, func() {
		if len(plugin.NewExport(exportPlan, files, nil).Symbols) != 2000 {
			t.Fatal("NewExport lists other than the files' 2,000 declarations")
		}
	})
	assert.Equal(t, built, 2.0, "NewExport allocates its result and its sorted declarations")
	doc := plugin.NewExport(exportPlan, files, nil)
	key := plugin.ExportKey{Origin: settleOrigin("row500", symbol.KindStruct), Plugin: exportPlugin, Name: "row500"}
	found := testing.AllocsPerRun(100, func() {
		if len(doc.Find(key)) != 2 {
			t.Fatal("Find returns other than the key's two declarations")
		}
	})
	assert.Equal(t, found, 0.0, "Find allocates nothing")
}

// BenchmarkExport measures an export of 1,000 structs in each of two
// files: building it, which allocates its result and its sorted
// declarations, and finding one key in it, which allocates nothing.
func BenchmarkExport(b *testing.B) {
	files := keyedFiles(1000)
	key := plugin.ExportKey{Origin: settleOrigin("row500", symbol.KindStruct), Plugin: exportPlugin, Name: "row500"}

	b.Run("NewExport", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(2)
		defer c.End()
		var got plugin.ExportDoc
		for c.Loop() {
			got = plugin.NewExport(exportPlan, files, nil)
		}
		if len(got.Symbols) != 2000 {
			b.Fatalf("NewExport lists %d declarations", len(got.Symbols))
		}
	})

	b.Run("Find", func(b *testing.B) {
		doc := plugin.NewExport(exportPlan, files, nil)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []plugin.ExportedSymbol
		for c.Loop() {
			got = doc.Find(key)
		}
		if len(got) != 2 {
			b.Fatalf("Find returns %d declarations", len(got))
		}
	})
}

// exportOf settles the units under a backend, which reports nothing,
// and returns the plan's export over one file containing every unit.
func exportOf(t *testing.T, b plugin.Backend, units ...plugin.Unit) plugin.ExportDoc {
	t.Helper()

	e := storeOf(t, units...)
	coretest.AssertCodes(t, settled(t, e, b))
	file := plugin.File{Path: exportFile, Pkg: exportPkg, Units: slices.Collect(e.Units())}
	return plugin.NewExport(exportPlan, []plugin.File{file}, e)
}

// exported returns the export's one declaration under an emitted name,
// and fails the case where the export lists none or more than one.
func exported(t *testing.T, doc plugin.ExportDoc, name string) plugin.ExportedSymbol {
	t.Helper()

	var found []plugin.ExportedSymbol
	for _, s := range doc.Symbols {
		if s.Name == name {
			found = append(found, s)
		}
	}
	assert.Length(t, found, 1, "the export lists one declaration emitted as "+name)
	return found[0]
}

// emittedNames returns the emitted names an export lists, in its
// order.
func emittedNames(doc plugin.ExportDoc) []string {
	out := make([]string, 0, len(doc.Symbols))
	for _, s := range doc.Symbols {
		out = append(out, s.Name)
	}
	return out
}

// boxWithItem returns a struct named box whose field is named item.
func boxWithItem() *emit.Struct {
	box := &emit.Struct{Origin: settleOrigin("box", symbol.KindStruct), Name: "box"}
	box.Fields.Append(&emit.Field{
		Origin: settleOrigin("item", symbol.KindField),
		Name:   "item",
		Type:   &emit.TypeRef{Spelling: "int"},
	})
	return box
}

// shapeSum returns a sum named shape over two variants: circle, with the
// field radius, then square, with the field side.
func shapeSum() *emit.Sum {
	circle := &emit.SumVariant{Name: "circle"}
	circle.Fields.Append(&emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: "int"}})
	square := &emit.SumVariant{Name: "square"}
	square.Fields.Append(&emit.Field{Name: "side", Type: &emit.TypeRef{Spelling: "int"}})
	shape := &emit.Sum{Origin: settleOrigin("shape", symbol.KindSum), Name: "shape"}
	shape.Variants.Append(circle, square)
	return shape
}

// receiving returns a file-level method named fetch whose receiver names
// a type spelled host.
func receiving(host string) *emit.Method {
	return &emit.Method{
		Origin:   settleOrigin("fetch", symbol.KindMethod),
		Name:     "fetch",
		Receives: &emit.TypeRef{Spelling: host},
	}
}

// keyedFiles returns two files that each declare the same n structs,
// so Find meets a key in more than one file.
func keyedFiles(n int) []plugin.File {
	files := make([]plugin.File, 0, 2)
	for _, path := range []string{exportFile, exportOther} {
		u := plugin.Unit{Plugin: exportPlugin, Per: plugin.PerSource, Word: "gen", Key: path, Pkg: exportPkg}
		for i := range n {
			name := "row" + strconv.Itoa(i)
			u.Decls = append(u.Decls, &emit.Struct{Origin: settleOrigin(name, symbol.KindStruct), Name: name})
		}
		files = append(files, plugin.File{Path: path, Pkg: exportPkg, Units: []plugin.Unit{u}})
	}
	return files
}

// keyed returns the export of the files keyedFiles returns.
func keyed(n int) plugin.ExportDoc { return plugin.NewExport(exportPlan, keyedFiles(n), nil) }
