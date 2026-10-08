// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"path"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// fakesDir is the directory the reference cases move RowStub into.
const fakesDir = "svc/store/fakes"

// keptFiles is the name table of kept files over a list of entries: what
// a warm run hands the layout of part of a plan.
type keptFiles []plugin.NameEntry

var _ plugin.Names = keptFiles(nil)

// InPackage returns the first entry of the package under the emitted
// name that is not a method's. It allocates nothing.
func (k keptFiles) InPackage(pkg, emitted string) (plugin.NameEntry, bool) {
	for _, e := range k {
		if e.Package == pkg && e.Emitted == emitted && e.Kind != symbol.KindMethod {
			return e, true
		}
	}
	return plugin.NameEntry{}, false
}

// OfOrigin returns the first entry derived from the origin under the
// emitted name. It allocates nothing.
func (k keptFiles) OfOrigin(origin symbol.Identity, emitted string) (plugin.NameEntry, bool) {
	for _, e := range k {
		if e.Origin == origin && e.Emitted == emitted {
			return e, true
		}
	}
	return plugin.NameEntry{}, false
}

// InScope returns the entries of the package and receiver, sorted by
// settled name. It allocates the list it returns.
func (k keptFiles) InScope(pkg, receiver string) []plugin.NameEntry {
	var out []plugin.NameEntry
	for _, e := range k {
		if e.Package == pkg && e.Receiver == receiver {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b plugin.NameEntry) int { return strings.Compare(a.Settled, b.Settled) })
	return out
}

// A bare reference to a declaration routed into another package takes
// that package, so the target qualifies it.
func TestReference(t *testing.T) {
	t.Parallel()

	t.Run("Route", func(t *testing.T) {
		t.Parallel()

		packages := []struct {
			name string
			give *emit.TypeRef
			out  string
			want string
		}{
			{
				name: "qualifies a bare reference into a file of another package",
				give: &emit.TypeRef{Spelling: "RowStub"}, out: "fakes/", want: fakesDir,
			},
			{
				name: "leaves a bare reference within one package bare",
				give: &emit.TypeRef{Spelling: "RowStub"}, out: "", want: "",
			},
			{
				name: "leaves a reference with a target as written",
				give: &emit.TypeRef{Spelling: "RowStub", Target: rowID}, out: "fakes/", want: "",
			},
			{
				name: "leaves a reference with a package as written",
				give: &emit.TypeRef{Spelling: "RowStub", Package: "example.com/row"}, out: "fakes/",
				want: "example.com/row",
			},
			{
				name: "leaves a reference to a name no declaration of the plan declares bare",
				give: &emit.TypeRef{Spelling: "string"}, out: "fakes/", want: "",
			},
		}
		for _, tt := range packages {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := newFixture(referring(tt.give), rowStub())
				if tt.out != "" {
					f.on(rowID, written(stubDirective, 1, keyOut, tt.out))
				}
				f.packager = directories{}
				_, sink := f.route(t)
				assert.Equal(t, tt.give.Package, tt.want, "the reference's package")
				coretest.AssertCodes(t, sink)
			})
		}

		t.Run("reports UnderivedPackage for a reference into a file whose package derives none", func(t *testing.T) {
			t.Parallel()

			f := newFixture(referring(&emit.TypeRef{Spelling: "RowStub"}), rowStub()).
				on(rowID, written(stubDirective, 1, keyOut, "fakes/"))
			f.packager = directories{refuse: fakesDir}
			files, sink := f.route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/store/fakes/row_stub.go: RowStub"},
				"the referencing declaration is refused")
			coretest.AssertCodes(t, sink, layout.UnderivedPackage)
			for d := range sink.All() {
				assert.Equal(t, d.Pos, at(storeFile), "the finding is at the referencing declaration's origin")
			}
		})

		t.Run("leaves a reference to a refused declaration as written", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: "RowStub"}
			f := newFixture(referring(ref), rowStub()).
				on(rowID, written(stubDirective, 1, keyOut, "/fakes/"))
			f.packager = directories{}
			files, sink := f.route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/store/store_stub.go: StoreStub"},
				"the escaping declaration is refused")
			assert.Equal(t, ref.Package, "", "the reference is bare")
			coretest.AssertCodes(t, sink, layout.EscapingPath)
		})

		t.Run("qualifies a bare reference into a kept file of another package", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: "RowStub"}
			f := newFixture(referring(ref))
			f.packager = directories{}
			f.others = keptFiles{keptRowStub(fakesDir)}
			_, sink := f.route(t)
			assert.Equal(t, ref.Package, fakesDir, "the reference takes the package of the kept file")
			coretest.AssertCodes(t, sink)
		})

		t.Run("leaves a bare reference into a kept file of its own package bare", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: "RowStub"}
			f := newFixture(referring(ref))
			f.packager = directories{}
			f.others = keptFiles{keptRowStub(storePkg)}
			_, sink := f.route(t)
			assert.Equal(t, ref.Package, "", "the kept file declares the package of the reference")
			coretest.AssertCodes(t, sink)
		})

		t.Run("reports UnderivedPackage for a reference into a kept file without a package", func(t *testing.T) {
			t.Parallel()

			kept := keptRowStub(fakesDir)
			kept.FilePkg = symbol.Identity{}
			f := newFixture(referring(&emit.TypeRef{Spelling: "RowStub"}))
			f.packager = directories{}
			f.others = keptFiles{kept}
			files, sink := f.route(t)
			assert.Empty(t, files, "the referencing declaration is refused")
			coretest.AssertCodes(t, sink, layout.UnderivedPackage)
		})
	})
}

// referring returns stubgen's primary unit of store.go: StoreStub, with
// a field of the given type.
func referring(ref *emit.TypeRef) plugin.Unit {
	return stubOf(storeFile, storePkg,
		generated(storeID, "StoreStub", &emit.Field{Name: "row", Type: ref}))
}

// keptRowStub returns the entry of a kept file in dir that declares the
// struct RowStub of the store package, the file declaring the package of
// dir as the [directories] target names it.
func keptRowStub(dir string) plugin.NameEntry {
	return plugin.NameEntry{
		Package: storePkg, Origin: rowID, Kind: symbol.KindStruct, Emitted: "RowStub", Settled: "RowStub",
		File:    dir + "/row_stub.go",
		FilePkg: symbol.Identity{Lang: coretest.Lang, Package: dir, Name: path.Base(dir), Kind: symbol.KindPackage},
	}
}
