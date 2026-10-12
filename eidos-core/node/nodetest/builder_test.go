// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package nodetest_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/node/nodetest"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The package paths, the files, the names and the scripted source of the
// fixtures.
const (
	storePath = "svc/store"
	cachePath = "svc/cache"
	svcName   = "svc"
	storeName = "store"
	cacheName = "cache"
	storeFile = "svc/store/store.zz"
	cacheFile = "svc/cache/cache.zz"
	otherFile = "svc/store/other.zz"
	badFile   = "/svc/store/bad.zz"

	rowName    = "Row"
	entryName  = "Entry"
	outerName  = "Outer"
	innerName  = "Inner"
	ghostName  = "Ghost"
	loadName   = "Load"
	getName    = "Get"
	limitName  = "limit"
	limitValue = "0"
	field0     = "f0"
	field1     = "f1"
	paramT     = "T"
	intName    = "int"
	presetName = "ints"
	twinName   = "X"

	cacheSource = "package svc/cache\ntype Entry\n"
	storeSource = "package svc/store\nimport cache svc/cache\ntype Row int cache.Entry\n" +
		"typeparam T\nmethod Get T int\nconst limit\n"
)

// The contracts of the failures of Build.
const (
	langContract     = "the language of the fixture has an identity"
	claimedContract  = "no file path belongs to two packages"
	repeatedContract = "no two files of the fixture have one path"
	invalidContract  = "every file path of the fixture is a valid path of a file system"
	twiceContract    = "no declaration or reference appears in two places of the fixture"
	loadContract     = "the load builds the fixture"
	findingContract  = "the load of the fixture reports no finding"
)

// errRefused is the error of the language that refuses every form.
var errRefused = errors.New("nodetest_test: the language refuses the form")

// scripted is the language of the kernel's scripted frontend, which
// overloads.
var scripted = nodetest.Neutral{Language: frontendtest.ScriptedLang, Overloading: true}

// refusing is a language that refuses every structural form.
type refusing struct{ nodetest.Neutral }

// Spell refuses every form with errRefused.
func (refusing) Spell(*node.TypeRef) (string, error) { return "", errRefused }

// entry is one declaration or reference of a walk of a graph: the
// identity of a declaration, the host of a member, and the target of a
// type reference.
type entry struct {
	id, host, target symbol.Identity
}

func TestBuilder(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		t.Run("creates a package with the segments and the last segment of its path", func(t *testing.T) {
			t.Parallel()

			p := nodetest.New(scripted).Package(storePath)
			expect.Equal(t, p.Path, []string{svcName, storeName}, "the package has the segments of its path")
			expect.Equal(t, p.Name, storeName, "the package has the last segment as its name")
		})

		t.Run("returns the package of an earlier call for the path", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			assert.Equal(t, b.Package(storePath), b.Package(storePath), "both calls return one package",
				assert.ByIdentity())
		})

		t.Run("creates the package of the empty path without segments", func(t *testing.T) {
			t.Parallel()

			p := nodetest.New(scripted).Package("")
			expect.Empty(t, p.Path, "the package has no segments")
			expect.Empty(t, p.Name, "the package has no name")
		})
	})

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		t.Run("places the file in its package", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			f := b.File(storePath, storeFile)
			files := b.Package(storePath).Files
			assert.Length(t, files, 1, "the package has one file")
			assert.Equal(t, files[0], f, "the package has the file", assert.ByIdentity())
		})

		t.Run("appends the declarations of a later call", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			row, entry := &node.Struct{Name: rowName}, &node.Struct{Name: entryName}
			b.File(storePath, storeFile, row)
			f := b.File(storePath, storeFile, entry)
			assert.Length(t, f.Decls, 2, "the file has two declarations")
			expect.Equal(t, f.Decls[0], symbol.Symbol(row), "the first declaration is the first one placed",
				assert.ByIdentity())
			expect.Equal(t, f.Decls[1], symbol.Symbol(entry), "the second declaration is the later one placed",
				assert.ByIdentity())
		})
	})

	t.Run("Ref", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a named reference without a spelling", func(t *testing.T) {
			t.Parallel()

			arg := nodetest.Named(intName)
			ref := nodetest.New(scripted).Ref(&node.Struct{Name: rowName}, arg)
			expect.Equal(t, ref.Form, symbol.FormNamed, "the reference is named")
			expect.Empty(t, ref.Spelling, "the reference has no spelling before Build")
			sameRefs(t, ref.Args, []*node.TypeRef{arg}, "the reference keeps its arguments")
		})
	})

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("assigns what the load of the scripted frontend assigns", func(t *testing.T) {
			t.Parallel()

			loaded := rulestest.Loaded(t, frontendtest.NewScripted(), fstest.MapFS{
				cacheFile: {Data: []byte(cacheSource)},
				storeFile: {Data: []byte(storeSource)},
			})
			b := nodetest.New(scripted)
			cached := &node.Struct{Name: entryName}
			b.File(cachePath, cacheFile, cached)
			row := &node.Struct{
				Name:       rowName,
				TypeParams: []*node.TypeParam{{Name: paramT}},
				Fields: []*node.Field{
					{Name: field0, Type: nodetest.Named(intName)},
					{Name: field1, Type: b.Ref(cached)},
				},
				Methods: []*node.Method{{
					Name:   getName,
					Params: []*node.Param{{Type: nodetest.Named(paramT)}, {Type: nodetest.Named(intName)}},
				}},
			}
			f := b.File(storePath, storeFile, row, &node.Constant{Name: limitName, Value: limitValue})
			f.Imports = []*node.Import{{Path: cachePath, Alias: cacheName}}
			var want []*node.Package
			for p := range loaded.Graph.Packages() {
				want = append(want, p)
			}
			assert.Equal(t, walked(b.Build(t)), walked(want),
				"the builder assigns the identities, the hosts and the targets of the scripted load")
		})

		t.Run("resolves a reference to a declaration of another package", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			cached := &node.Struct{Name: entryName}
			b.File(cachePath, cacheFile, cached)
			ref := b.Ref(cached)
			b.File(storePath, storeFile, &node.Struct{Name: rowName, Fields: []*node.Field{{Name: field0, Type: ref}}})
			b.Build(t)
			expect.Equal(t, ref.Spelling, cacheName+symbol.OwnerSep+entryName, "the reference has a qualified name")
			expect.Equal(t, ref.Package, cachePath, "the reference has the path of the package of its target")
			expect.Equal(t, ref.Target, cached.ID, "the reference targets the declaration")
		})

		t.Run("resolves a reference to a nested type", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			inner := &node.Struct{Name: innerName}
			ref := b.Ref(inner)
			b.File(storePath, storeFile,
				&node.Struct{Name: outerName, Types: node.Symbols{inner}},
				&node.Struct{Name: rowName, Fields: []*node.Field{{Name: field0, Type: ref}}})
			b.Build(t)
			expect.Equal(t, ref.Spelling, outerName+symbol.OwnerSep+innerName,
				"the reference has the names of the enclosing types")
			expect.Equal(t, ref.Target, inner.ID, "the reference targets the nested type")
		})

		t.Run("resolves a named reference to a type parameter in scope", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			param := &node.TypeParam{Name: paramT}
			ref := nodetest.Named(paramT)
			b.File(storePath, storeFile, &node.Struct{
				Name:       rowName,
				TypeParams: []*node.TypeParam{param},
				Fields:     []*node.Field{{Name: field0, Type: ref}},
			})
			b.Build(t)
			assert.Equal(t, ref.Target, param.ID, "the reference targets the type parameter")
		})

		t.Run("leaves a named reference to a builtin without a target", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			ref := nodetest.Named(intName)
			b.File(storePath, storeFile, &node.Struct{Name: rowName, Fields: []*node.Field{{Name: field0, Type: ref}}})
			b.Build(t)
			assert.Equal(t, ref.Target, symbol.Identity{}, "the reference has no target")
		})

		t.Run("gives each declaration a line of its own", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			field := &node.Field{Name: field0, Type: nodetest.Named(intName)}
			row := &node.Struct{Name: rowName, Fields: []*node.Field{field}}
			f := b.File(storePath, storeFile, row)
			b.Build(t)
			expect.Equal(t, f.Pos, position.Pos{File: storeFile, Line: 1, Col: 1}, "the file is at line 1")
			expect.Equal(t, row.Pos, position.Pos{File: storeFile, Line: 2, Col: 1}, "the struct is at line 2")
			expect.Equal(t, field.Pos, position.Pos{File: storeFile, Line: 3, Col: 1}, "the field is at line 3")
		})

		t.Run("gives a reference the position of the declaration that contains it", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			supertype := nodetest.Named(entryName)
			row := &node.Struct{
				Name:    rowName,
				Fields:  []*node.Field{{Name: field0, Type: nodetest.Named(intName)}},
				Extends: []*node.TypeRef{supertype},
			}
			b.File(storePath, storeFile, row)
			b.Build(t)
			assert.Equal(t, supertype.Pos, row.Pos, "a supertype after the members has the position of the struct")
		})

		t.Run("keeps a position that the test sets", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			at := position.Pos{File: storeFile, Line: 40, Col: 2}
			row := &node.Struct{Name: rowName, Pos: at}
			b.File(storePath, storeFile, row)
			b.Build(t)
			assert.Equal(t, row.Pos, at, "the struct keeps its position")
		})

		t.Run("spells a structural reference through the language", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			ref := nodetest.List(nodetest.Named(intName))
			b.File(storePath, storeFile, &node.Struct{Name: rowName, Fields: []*node.Field{{Name: field0, Type: ref}}})
			b.Build(t)
			assert.Equal(t, ref.Spelling, "list(int)", "the reference has the spelling of the language")
		})

		t.Run("keeps a spelling that the test sets", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			ref := nodetest.List(nodetest.Named(intName))
			ref.Spelling = presetName
			b.File(storePath, storeFile, &node.Struct{Name: rowName, Fields: []*node.Field{{Name: field0, Type: ref}}})
			b.Build(t)
			assert.Equal(t, ref.Spelling, presetName, "the reference keeps its spelling")
		})

		t.Run("spells the discriminator of an overloaded method", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			get := &node.Method{Name: getName, Params: []*node.Param{{Type: nodetest.Named(intName)}}}
			b.File(storePath, storeFile, &node.Struct{Name: rowName, Methods: []*node.Method{get}})
			b.Build(t)
			assert.Equal(t, get.ID.Disc, intName, "the discriminator has the spelling of the parameter")
		})

		t.Run("returns the packages in identity order", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			b.File(storePath, storeFile, &node.Struct{Name: rowName})
			b.File(cachePath, cacheFile, &node.Struct{Name: entryName})
			pkgs := b.Build(t)
			assert.Length(t, pkgs, 2, "the fixture has two packages")
			assert.Pairwise(t, pkgs, func(earlier, later *node.Package) bool {
				return earlier.ID.Compare(later.ID) < 0
			}, "each package precedes the packages after it in identity order")
		})

		t.Run("gives each package of the builder the identity of its copy", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			b.File(storePath, storeFile, &node.Struct{Name: rowName})
			pkgs := b.Build(t)
			assert.Length(t, pkgs, 1, "the fixture has one package")
			assert.Equal(t, b.Package(storePath).ID, pkgs[0].ID, "the package has the identity of its copy")
		})

		t.Run("returns the same packages on a second call", func(t *testing.T) {
			t.Parallel()

			b := nodetest.New(scripted)
			b.File(storePath, storeFile, &node.Struct{Name: rowName})
			first := b.Build(t)
			assert.Equal(t, b.Build(t), first, "the second call returns the packages of the first",
				assert.ByIdentity())
		})

		t.Run("fails for a language without an identity", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a fixture in no language", func(tb assert.TB) {
				nodetest.New(nodetest.Neutral{}).Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{langContract}, "the failure names the language")
		})

		t.Run("fails for a file path that two packages claim", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "one path in two packages", func(tb assert.TB) {
				b := nodetest.New(scripted)
				b.File(storePath, storeFile, &node.Struct{Name: rowName})
				b.File(cachePath, storeFile)
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{claimedContract}, "the failure names the claim")
		})

		t.Run("fails for two files with one path", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "two files with one path", func(tb assert.TB) {
				b := nodetest.New(scripted)
				b.File(storePath, storeFile, &node.Struct{Name: rowName})
				b.File(storePath, otherFile).Path = storeFile
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{repeatedContract}, "the failure names the path")
		})

		t.Run("fails for a file path that a file system does not admit", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "an absolute file path", func(tb assert.TB) {
				b := nodetest.New(scripted)
				b.File(storePath, badFile, &node.Struct{Name: rowName})
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{invalidContract}, "the failure names the path")
		})

		t.Run("fails for a declaration in two places", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "one struct in two files", func(tb assert.TB) {
				b := nodetest.New(scripted)
				row := &node.Struct{Name: rowName}
				b.File(storePath, storeFile, row)
				b.File(storePath, otherFile, row)
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{twiceContract}, "the failure names the place")
		})

		t.Run("fails for a reference to a declaration outside the fixture", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a reference to a struct of no file", func(tb assert.TB) {
				b := nodetest.New(scripted)
				ref := b.Ref(&node.Struct{Name: ghostName})
				row := &node.Struct{Name: rowName, Fields: []*node.Field{{Name: field0, Type: ref}}}
				b.File(storePath, storeFile, row)
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the reference to " + symbol.KindStruct.String() + " " + ghostName + " targets a declaration of the fixture",
			}, "the failure names the target")
		})

		t.Run("fails for a reference to a function", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a reference to a function", func(tb assert.TB) {
				b := nodetest.New(scripted)
				load := &node.Function{Name: loadName}
				ref := b.Ref(load)
				row := &node.Struct{Name: rowName, Fields: []*node.Field{{Name: field0, Type: ref}}}
				b.File(storePath, storeFile, load, row)
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the load resolves the reference to " + symbol.KindFunction.String() + " " + loadName +
					" to the identity of the declaration",
			}, "the failure names the target that the load does not resolve")
		})

		t.Run("fails for one spelling of two targets in one file", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "one spelling of two structs", func(tb assert.TB) {
				b := nodetest.New(scripted)
				row, cached := &node.Struct{Name: rowName}, &node.Struct{Name: entryName}
				first, second := b.Ref(row), b.Ref(cached)
				first.Spelling, second.Spelling = twinName, twinName
				b.File(storePath, storeFile, row, cached, &node.Struct{Name: outerName, Fields: []*node.Field{
					{Name: field0, Type: first}, {Name: field1, Type: second},
				}})
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the spelling " + twinName + " names one target in " + storeFile,
			}, "the failure names the spelling and the file")
		})

		t.Run("fails for a form that the language refuses", func(t *testing.T) {
			t.Parallel()

			at := position.Pos{File: storeFile, Line: 3, Col: 1}
			records := assert.Rejects(t, "a list in a language without lists", func(tb assert.TB) {
				b := nodetest.New(refusing{scripted})
				b.File(storePath, storeFile, &node.Struct{Name: rowName, Fields: []*node.Field{
					{Name: field0, Type: nodetest.List(nodetest.Named(intName))},
				}})
				b.Build(tb)
			})
			want := []string{"the language spells the reference at " + at.String()}
			assert.Equal(t, coretest.Contracts(records), want, "the failure names the position of the reference")
		})

		t.Run("fails for a duplicate declaration", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "two structs of one name", func(tb assert.TB) {
				b := nodetest.New(scripted)
				b.File(storePath, storeFile, &node.Struct{Name: rowName}, &node.Struct{Name: rowName})
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{findingContract}, "the failure names the finding")
		})

		t.Run("fails for a declaration without a name", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a struct without a name", func(tb assert.TB) {
				b := nodetest.New(scripted)
				b.File(storePath, storeFile, &node.Struct{})
				b.Build(tb)
			})
			assert.Equal(t, coretest.Contracts(records), []string{loadContract}, "the failure names the load")
		})
	})
}

// walked returns the identity of each declaration, the host of each method
// and each field, and the target of each type reference of the packages,
// in the order of a walk of each.
func walked(pkgs []*node.Package) []entry {
	var out []entry
	for _, p := range pkgs {
		node.Walk(p, func(s symbol.Symbol) bool {
			switch x := s.(type) {
			case *node.TypeRef:
				out = append(out, entry{target: x.Target})
			case *node.Method:
				out = append(out, entry{id: x.ID, host: x.Host})
			case *node.Field:
				out = append(out, entry{id: x.ID, host: x.Host})
			case node.Declaration:
				out = append(out, entry{id: x.Identity()})
			}
			return true
		})
	}
	return out
}
