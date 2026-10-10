// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// namesOfAllocs is one NamesOf over the files of keyedFiles: the list of
// entries and the sorted list of the files' declarations.
const namesOfAllocs = 2

// keptNames is the name table of kept files over a list of entries:
// what a warm run hands a settle of part of a plan.
type keptNames []plugin.NameEntry

var _ plugin.Names = keptNames(nil)

// InPackage returns the first entry that a declaration of the package
// other than a method declares under the emitted name, ambiguous where
// another entry settles the name apart. It allocates nothing.
func (k keptNames) InPackage(pkg, emitted string) (plugin.NameEntry, bool) {
	var (
		out   plugin.NameEntry
		found bool
	)
	for _, e := range k {
		switch {
		case e.Package != pkg || e.Emitted != emitted || e.Kind == symbol.KindMethod:
		case !found:
			out, found = e, true
		case e.Settled != out.Settled:
			out.Ambiguous = true
		}
	}
	return out, found
}

// OfOrigin returns the first entry derived from the origin under the
// emitted name. It allocates nothing.
func (k keptNames) OfOrigin(origin symbol.Identity, emitted string) (plugin.NameEntry, bool) {
	for _, e := range k {
		if e.Origin == origin && e.Emitted == emitted {
			return e, true
		}
	}
	return plugin.NameEntry{}, false
}

// InScope returns the entries of the package and receiver, sorted by
// settled name. It allocates the list it returns.
func (k keptNames) InScope(pkg, receiver string) []plugin.NameEntry {
	var out []plugin.NameEntry
	for _, e := range k {
		if e.Package == pkg && e.Receiver == receiver {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b plugin.NameEntry) int { return strings.Compare(a.Settled, b.Settled) })
	return out
}

// A plan's name table lists what each file declares, under the names
// that a kept file's references resolve against.
func TestNames(t *testing.T) {
	t.Parallel()

	t.Run("NameKey", func(t *testing.T) {
		t.Parallel()

		t.Run("Compare", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				a, b plugin.NameKey
				want int
			}{
				{
					name: "returns a negative number for an earlier package",
					a:    plugin.NameKey{Package: "api", Emitted: "row"},
					b:    plugin.NameKey{Package: "svc", Emitted: "box"},
					want: -1,
				},
				{
					name: "returns a negative number for an earlier origin",
					a:    plugin.NameKey{Origin: settleOrigin("alpha", symbol.KindStruct), Emitted: "row"},
					b:    plugin.NameKey{Origin: settleOrigin("beta", symbol.KindStruct), Emitted: "box"},
					want: -1,
				},
				{
					name: "returns a positive number for a later emitted name",
					a:    plugin.NameKey{Package: "svc", Emitted: "row"},
					b:    plugin.NameKey{Package: "svc", Emitted: "box"},
					want: 1,
				},
				{
					name: "returns zero for an equal key",
					a:    plugin.NameKey{Package: "svc", Emitted: "row"},
					b:    plugin.NameKey{Package: "svc", Emitted: "row"},
					want: 0,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.a.Compare(tt.b), tt.want, "the package, then the origin, then the name")
				})
			}
		})
	})

	t.Run("NamesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a NameEntry for a file-level declaration", func(t *testing.T) {
			t.Parallel()

			got := namesAfterSettle(t, nil, &emit.Struct{Origin: settleOrigin("box", symbol.KindStruct), Name: "box"})
			assert.Equal(t, got, []plugin.NameEntry{{
				Package: "svc", Origin: settleOrigin("box", symbol.KindStruct), Kind: symbol.KindStruct,
				Emitted: "box", Settled: "Box", File: exportFile, FilePkg: exportPkg,
			}}, "the struct's entry names its file")
		})

		t.Run("returns no NameEntry for a member", func(t *testing.T) {
			t.Parallel()

			got := namesAfterSettle(t, nil, boxWithItem())
			assert.Length(t, got, 1, "the struct has an entry, and its field has none")
		})

		t.Run("sets Receiver to the emitted name of the method's type", func(t *testing.T) {
			t.Parallel()

			got := namesAfterSettle(t, nil, boxWithItem(), receiving("box"))
			assert.Length(t, got, 2, "the struct has an entry, and so does the method")
			assert.Equal(t, got[1].Receiver, "box", "the method attaches to the struct as it was emitted")
		})

		t.Run("sets Receiver to the emitted name that others list for the type", func(t *testing.T) {
			t.Parallel()

			kept := keptNames{{Package: "svc", Kind: symbol.KindStruct, Emitted: "box", Settled: "Box"}}
			got := namesAfterSettle(t, kept, receiving("box"))
			assert.Length(t, got, 1, "the method has an entry")
			assert.Equal(t, got[0].Receiver, "box", "a kept file declares the type")
		})

		t.Run("sets Receiver to the receiver's spelling where no file declares the type", func(t *testing.T) {
			t.Parallel()

			got := namesAfterSettle(t, keptNames{}, receiving(exportForeign))
			assert.Length(t, got, 1, "the method has an entry")
			assert.Equal(t, got[0].Receiver, exportForeign, "no file declares the type")
		})

		t.Run("sets Emitted to the declared name for a nil store", func(t *testing.T) {
			t.Parallel()

			box := boxWithItem()
			box.Name = "Box"
			file := plugin.File{
				Path:  exportFile,
				Pkg:   exportPkg,
				Units: []plugin.Unit{settleUnit("svc", "svc/a.src", box)},
			}
			got := plugin.NamesOf([]plugin.File{file}, nil, nil)
			assert.Length(t, got, 1, "the struct has an entry")
			assert.Equal(t, got[0].Emitted, "Box", "the file spells the name")
		})
	})
}

// NamesOf allocates its list of entries and its sorted declarations, and
// NameKey.Compare allocates nothing, in the ordinary run, which runs no
// benchmark. The check runs alone, because the count includes every
// goroutine's allocations.
func TestNamesAllocs(t *testing.T) {
	files := keyedFiles(1000)
	var got []plugin.NameEntry
	assert.MaxAllocs(t, func() { got = plugin.NamesOf(files, nil, nil) }, namesOfAllocs,
		"NamesOf allocates its entries and its sorted declarations")
	assert.Length(t, got, 2000, "NamesOf lists the files' 2,000 declarations")
	a, b := plugin.NameKey{Package: "svc", Emitted: "row"}, plugin.NameKey{Package: "svc", Emitted: "box"}
	order := 0
	assert.MaxAllocs(t, func() { order = a.Compare(b) }, 0, "Compare allocates nothing")
	assert.InRange(t, order, 1, math.Inf(1), "row sorts after box")
}

// BenchmarkNames measures the names of 1,000 structs in each of two
// files, and the comparison of two keys.
func BenchmarkNames(b *testing.B) {
	b.Run("NameKey", func(b *testing.B) {
		b.Run("Compare", func(b *testing.B) {
			x, y := plugin.NameKey{Package: "svc", Emitted: "row"}, plugin.NameKey{Package: "svc", Emitted: "box"}
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			order := 0
			for c.Loop() {
				order = x.Compare(y)
			}
			assert.InRange(b, order, 1, math.Inf(1), "row sorts after box")
		})
	})

	b.Run("NamesOf", func(b *testing.B) {
		files := keyedFiles(1000)
		c := bench.Start(b).MaxAllocs(namesOfAllocs)
		defer c.End()
		var got []plugin.NameEntry
		for c.Loop() {
			got = plugin.NamesOf(files, nil, nil)
		}
		assert.Length(b, got, 2000, "NamesOf lists the files' 2,000 declarations")
	})
}

// namesAfterSettle settles one unit of the declarations under the
// capitalizing hook against kept, and returns the names of one file
// containing the unit.
func namesAfterSettle(t *testing.T, kept plugin.Names, decls ...symbol.Symbol) []plugin.NameEntry {
	t.Helper()

	e := storeOf(t, settleUnit("svc", "svc/a.src", decls...))
	sink := diag.NewSink()
	_, err := plugin.SettleWith(e, capitalizing(), nil, sink, kept)
	assert.NoError(t, err, "the settle completes")
	coretest.AssertCodes(t, sink)
	file := plugin.File{Path: exportFile, Pkg: exportPkg, Units: slices.Collect(e.Units())}
	return plugin.NamesOf([]plugin.File{file}, e, kept)
}
