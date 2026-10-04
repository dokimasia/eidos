// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
)

// fakesDir is the directory the reference cases move RowStub into.
const fakesDir = "svc/store/fakes"

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
	})
}

// referring returns stubgen's primary unit of store.go: StoreStub, with
// a field of the given type.
func referring(ref *emit.TypeRef) plugin.Unit {
	return stubOf(storeFile, storePkg,
		generated(storeID, "StoreStub", &emit.Field{Name: "row", Type: ref}))
}
