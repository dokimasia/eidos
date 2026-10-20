// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"path"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The names of the collision cases: a struct, a method and a field that
// two declarations share, the second receiver of the method and the field,
// the type of the field, the names of three groups whose findings the
// layout orders, and a function with the emitted name of a function that
// settles to it.
const (
	stubName      = "Stub"
	trackName     = "Track"
	tagName       = "Tag"
	boxName       = "Box"
	intName       = "int"
	aceName       = "Ace"
	bobName       = "Bob"
	zedName       = "Zed"
	findName      = "Find"
	lowerFindName = "find"
)

// The messages of two collisions: two structs of one routed package, and
// two methods on one receiver.
const (
	stubCollision = `svc/store.Stub in svc/store/row_stub.go and svc/store.Stub in svc/store/store_stub.go ` +
		`settle to "Stub" in one package`
	trackCollision = `svc/store.Track in svc/store/store_stub.go and svc/store.Track in svc/store/store_stub.go ` +
		`settle to "Track" on the receiver Row in one package`
)

// moduleFiles is the package half of a target in which each file is a
// module of its own: a file declares the package of its path.
type moduleFiles struct{}

// PackageAt returns the package of the file's path.
func (moduleFiles) PackageAt(p plugin.Placement) (symbol.Identity, error) {
	return symbol.Identity{Lang: coretest.Lang, Package: p.Path, Name: path.Base(p.Path), Kind: symbol.KindPackage}, nil
}

// namespaces is the package half of a target in which each file is a
// package of its own, and the files of one directory declare their names
// in one namespace, as the files of one protobuf package do.
type namespaces struct{ moduleFiles }

var _ plugin.NameScoper = namespaces{}

// NameScope returns the namespace of the directory of the package's path.
func (namespaces) NameScope(pkg symbol.Identity) symbol.Identity {
	return symbol.Identity{Lang: coretest.Lang, Package: path.Dir(pkg.Package), Kind: symbol.KindPackage}
}

// overloading is the package half of a target in which the files of one
// directory are one package, and whose language overloads a function where
// overloads is true, as Python's does.
type overloading struct {
	directories
	overloads bool
}

var _ plugin.Overloader = overloading{}

// Overloads reports whether the language of the target overloads a
// function.
func (o overloading) Overloads() bool { return o.overloads }

// Routed paths that one tree cannot contain refuse the declarations
// routed to them, and every other file renders. Two file-level names of
// one routed scope with one settled name fail the plan.
func TestCollision(t *testing.T) {
	t.Parallel()

	t.Run("Route", func(t *testing.T) {
		t.Parallel()

		collisions := []struct {
			name string
			give func() *fixture
		}{
			{
				name: "reports PathCollision for declarations of two packages routed to one file",
				give: func() *fixture {
					return newFixture(storeStub(), cacheStub()).
						on(storeID, written(stubDirective, 1, keyOut, "../shared/stub.go")).
						on(cacheID, written(stubDirective, 1, keyOut, "../shared/stub.go"))
				},
			},
			{
				name: "reports PathCollision for two paths that differ only in case",
				give: func() *fixture {
					return newFixture(storeStub(), rowStub()).
						on(storeID, written(stubDirective, 1, keyOut, "Stub.go")).
						on(rowID, written(stubDirective, 1, keyOut, "stub.go"))
				},
			},
			{
				name: "reports PathCollision for a file under a path routed as a file",
				give: func() *fixture {
					return newFixture(storeStub(), rowStub()).
						on(storeID, written(stubDirective, 1, keyOut, "../mocks")).
						on(rowID, written(stubDirective, 1, keyOut, "../mocks/"))
				},
			},
			{
				name: "reports PathCollision for a file at a path another file needs as a directory",
				give: func() *fixture {
					return newFixture(storeStub(), rowStub()).
						on(storeID, written(stubDirective, 1, keyOut, "../Mocks/")).
						on(rowID, written(stubDirective, 1, keyOut, "../mocks"))
				},
			},
		}
		for _, tt := range collisions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := tt.give().route(t)
				assert.Empty(t, files, "the declarations of both files are refused")
				coretest.AssertCodes(t, sink, layout.PathCollision)
			})
		}

		t.Run("returns the files beside a collision", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(storeStub(), rowStub(), cacheStub()).
				on(storeID, written(stubDirective, 1, keyOut, "Stub.go")).
				on(rowID, written(stubDirective, 1, keyOut, "stub.go")).route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/cache/cache_stub.go: CacheStub"},
				"the file outside the collision routes")
			coretest.AssertCodes(t, sink, layout.PathCollision)
		})

		t.Run("reports a clash at the second file's origin with the first's as related", func(t *testing.T) {
			t.Parallel()

			_, sink := newFixture(storeStub(), rowStub()).
				on(storeID, written(stubDirective, 1, keyOut, "Stub.go")).
				on(rowID, written(stubDirective, 1, keyOut, "stub.go")).route(t)
			for d := range sink.All() {
				assert.Equal(t, d.Pos, at(rowFile), "svc/store/stub.go sorts second")
				assert.Equal(t, d.Related, []position.Pos{at(storeFile)}, "svc/store/Stub.go sorts first")
			}
		})

		t.Run("reports two packages at the second unit's origin with the first's as related", func(t *testing.T) {
			t.Parallel()

			_, sink := newFixture(storeStub(), cacheStub()).
				on(storeID, written(stubDirective, 1, keyOut, "../shared/stub.go")).
				on(cacheID, written(stubDirective, 1, keyOut, "../shared/stub.go")).route(t)
			for d := range sink.All() {
				assert.Equal(t, d.Pos, at(storeFile), "the store unit sorts after the cache unit")
				assert.Equal(t, d.Related, []position.Pos{at(cacheFile)}, "the cache unit sorts first")
			}
		})

		t.Run("returns a plan unit and a package's unit at one path as one file", func(t *testing.T) {
			t.Parallel()

			f := newFixture(storeStub(),
				unitOf(stubgen, families()[stubgen][3], "", symbol.Identity{}, generated(storeID, "StoreIndex")))
			f.config = layout.Config{Families: map[layout.Family]layout.Refinement{
				{Plugin: stubgen, Tag: tagPlan}: {Dir: storePkg, File: "store_stub.go"},
			}}
			files, sink := f.route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/store/store_stub.go: StoreStub StoreIndex"},
				"a plan unit derives from no package")
			coretest.AssertCodes(t, sink)
		})

		names := []struct {
			name string
			give func() *fixture
			want []diag.Code
		}{
			{
				name: "reports CollidingNames for two file-level names of one routed package",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg, generated(storeID, stubName)),
						stubOf(rowFile, storePkg, generated(rowID, stubName)))
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports CollidingNames for two source packages that route into one package",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, stubName)),
						stubOf(cacheFile, cachePkg, generated(cacheID, stubName))).
						on(cacheID, written(stubDirective, 1, keyOut, "../store/"))
					f.packager = directories{}
					return f
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports no CollidingNames for one name in two routed packages",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, stubName)),
						stubOf(rowFile, storePkg, generated(rowID, stubName)))
					f.packager = moduleFiles{}
					return f
				},
			},
			{
				name: "reports CollidingNames for two packages of one namespace",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, stubName)),
						stubOf(rowFile, storePkg, generated(rowID, stubName)))
					f.packager = namespaces{}
					return f
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports no CollidingNames for two files without a package",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, stubName)),
						stubOf(rowFile, storePkg, generated(rowID, stubName)))
					f.packager = directories{refuse: storePkg}
					return f
				},
			},
			{
				name: "reports CollidingNames for two declarations of one name in one file",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg,
						generated(storeID, stubName), generated(storeID, stubName)))
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports no CollidingNames for a record of a family that claims its files",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, stubName)),
						unitOf(stubgen, families()[stubgen][1], storeFile, coretest.PackageID(storePkg),
							generated(storeID, stubName)))
					f.outputs[stubgen][1].Template = claimTemplate
					return f
				},
			},
			{
				name: "reports CollidingNames for two methods of one name on one receiver",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg,
						&emit.Method{Origin: storeID, Name: trackName, Receives: &emit.TypeRef{Spelling: rowName}},
						&emit.Method{Origin: rowID, Name: trackName, Receives: &emit.TypeRef{Spelling: rowName}}))
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports no CollidingNames for one method name on two receivers",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg,
						&emit.Method{Origin: storeID, Name: trackName, Receives: &emit.TypeRef{Spelling: rowName}},
						&emit.Method{Origin: rowID, Name: trackName, Receives: &emit.TypeRef{Spelling: boxName}}))
				},
			},
			{
				name: "reports CollidingNames for two fields of one name on one receiving type",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg,
						&emit.Field{
							Origin: storeID, Name: tagName, Type: &emit.TypeRef{Spelling: intName},
							Receives: &emit.TypeRef{Spelling: rowName},
						},
						&emit.Field{
							Origin: rowID, Name: tagName, Type: &emit.TypeRef{Spelling: intName},
							Receives: &emit.TypeRef{Spelling: rowName},
						}))
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports no CollidingNames for one field name on two receiving types",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg,
						&emit.Field{
							Origin: storeID, Name: tagName, Type: &emit.TypeRef{Spelling: intName},
							Receives: &emit.TypeRef{Spelling: rowName},
						},
						&emit.Field{
							Origin: rowID, Name: tagName, Type: &emit.TypeRef{Spelling: intName},
							Receives: &emit.TypeRef{Spelling: boxName},
						}))
				},
			},
			{
				name: "reports CollidingNames for a name that a kept file of its routed package declares",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, rowStubName)))
					f.packager = directories{}
					f.others = keptFiles{keptRowStub(storePkg)}
					return f
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports no CollidingNames for a kept name of another routed package",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg, generated(storeID, rowStubName)))
					f.packager = directories{}
					f.others = keptFiles{keptRowStub(fakesDir)}
					return f
				},
			},
			{
				name: "reports no CollidingNames for the overloads of one function of a target that overloads",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg,
						&emit.Function{Origin: storeID, Name: findName}, &emit.Function{Origin: rowID, Name: findName}))
					f.packager = overloading{overloads: true}
					return f
				},
			},
			{
				name: "reports CollidingNames for two functions of one name of a target that does not overload",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg,
						&emit.Function{Origin: storeID, Name: findName}, &emit.Function{Origin: rowID, Name: findName}))
					f.packager = overloading{}
					return f
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports CollidingNames for a struct with the name of a function of a target that overloads",
				give: func() *fixture {
					f := newFixture(stubOf(storeFile, storePkg,
						&emit.Function{Origin: storeID, Name: stubName}, generated(rowID, stubName)))
					f.packager = overloading{overloads: true}
					return f
				},
				want: []diag.Code{plugin.CollidingNames},
			},
			{
				name: "reports CollidingNames for a function whose emitted name settles to the name of another",
				give: func() *fixture {
					kept := keptRowStub(storePkg)
					kept.Kind, kept.Emitted, kept.Settled = symbol.KindFunction, lowerFindName, findName
					f := newFixture(stubOf(storeFile, storePkg, &emit.Function{Origin: storeID, Name: findName}))
					f.packager = overloading{overloads: true}
					f.others = keptFiles{kept}
					return f
				},
				want: []diag.Code{plugin.CollidingNames},
			},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := tt.give().route(t)
				coretest.AssertCodes(t, sink, tt.want...)
			})
		}

		t.Run("returns the files of a collision of names", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(stubOf(storeFile, storePkg, generated(storeID, stubName)),
				stubOf(rowFile, storePkg, generated(rowID, stubName))).route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/store/row_stub.go: Stub", "svc/store/store_stub.go: Stub"},
				"the Error fails the plan, so the layout refuses no declaration")
			coretest.AssertCodes(t, sink, plugin.CollidingNames)
		})

		t.Run("reports CollidingNames at the second declaration with the first as related", func(t *testing.T) {
			t.Parallel()

			_, sink := newFixture(stubOf(storeFile, storePkg, generated(storeID, stubName)),
				stubOf(rowFile, storePkg, generated(rowID, stubName))).route(t)
			coretest.AssertCodes(t, sink, plugin.CollidingNames)
			for d := range sink.All() {
				expect.Equal(t, d.Pos, at(storeFile), "svc/store/store_stub.go sorts second")
				expect.Equal(t, d.Related, []position.Pos{at(rowFile)}, "svc/store/row_stub.go sorts first")
				expect.Equal(t, d.Msg, stubCollision, "the finding names each declaration with its file")
			}
		})

		t.Run("reports CollidingNames with the receiver of two methods", func(t *testing.T) {
			t.Parallel()

			_, sink := newFixture(stubOf(storeFile, storePkg,
				&emit.Method{Origin: storeID, Name: trackName, Receives: &emit.TypeRef{Spelling: rowName}},
				&emit.Method{Origin: rowID, Name: trackName, Receives: &emit.TypeRef{Spelling: rowName}})).route(t)
			coretest.AssertCodes(t, sink, plugin.CollidingNames)
			for d := range sink.All() {
				expect.Equal(t, d.Pos, at(rowFile), "the method of Row is the second declaration")
				expect.Equal(t, d.Msg, trackCollision, "the finding names the receiver")
			}
		})

		t.Run("reports CollidingNames at the routed declaration with the kept one as related", func(t *testing.T) {
			t.Parallel()

			f := newFixture(stubOf(storeFile, storePkg, generated(storeID, rowStubName)))
			f.packager = directories{}
			f.others = keptFiles{keptRowStub(storePkg)}
			_, sink := f.route(t)
			coretest.AssertCodes(t, sink, plugin.CollidingNames)
			for d := range sink.All() {
				expect.Equal(t, d.Pos, at(storeFile), "the routed file sorts after the kept file")
				expect.Equal(t, d.Related, []position.Pos{at(rowFile)}, "the kept name is at its origin")
			}
		})

		t.Run("positions a kept name without an origin at its file", func(t *testing.T) {
			t.Parallel()

			kept := keptRowStub(storePkg)
			kept.Origin = symbol.Identity{}
			f := newFixture(stubOf(storeFile, storePkg, generated(storeID, rowStubName)))
			f.packager = directories{}
			f.others = keptFiles{kept}
			_, sink := f.route(t)
			coretest.AssertCodes(t, sink, plugin.CollidingNames)
			for d := range sink.All() {
				assert.Equal(t, d.Related, []position.Pos{{File: kept.File}}, "the kept file positions the name")
			}
		})

		t.Run("reports the collisions in the order of their routed scopes", func(t *testing.T) {
			t.Parallel()

			_, sink := newFixture(
				stubOf(storeFile, storePkg, generated(storeID, bobName), generated(storeID, aceName)),
				stubOf(rowFile, storePkg, generated(rowID, bobName), generated(rowID, aceName)),
				stubOf(cacheFile, cachePkg, generated(cacheID, zedName), generated(cacheID, zedName)),
			).route(t)
			var got []string
			for d := range sink.All() {
				got = append(got, d.Msg)
			}
			assert.Length(t, got, 3, "the layout reports one finding for each group")
			expect.Contains(t, got[0], strconv.Quote(zedName), "the package svc/cache sorts first")
			expect.Contains(t, got[1], strconv.Quote(aceName), "Ace sorts first in svc/store")
			expect.Contains(t, got[2], strconv.Quote(bobName), "Bob sorts after Ace")
		})
	})
}

// rowStub returns stubgen's primary unit of row.go with one stub of
// Row.
func rowStub() plugin.Unit {
	return stubOf(rowFile, storePkg, generated(rowID, "RowStub"))
}

// cacheStub returns stubgen's primary unit of cache.go with one stub
// of Cache.
func cacheStub() plugin.Unit {
	return stubOf(cacheFile, cachePkg, generated(cacheID, "CacheStub"))
}
