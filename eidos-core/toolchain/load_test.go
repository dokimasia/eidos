// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"context"
	"errors"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/node"
	frame "go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/toolchain"
)

// loadBrand is the brand under which the checks load generated output.
const loadBrand frame.Brand = "toolchain"

// errParse is the error of a parse that fails.
var errParse = errors.New("toolchain_test: the parse fails")

// These constants are the paths of the fixtures of the loaded checks: two
// generated files and a source file of one package, and a file that the
// scripted language refuses.
const (
	apiFile    = "gen/api.zz"
	extraFile  = "gen/extra.zz"
	genSource  = "gen/source.zz"
	badFile    = "gen/bad.zz"
	genPackage = "gen"
)

// literal is the scripted language whose parse lowers each file of a unit
// to the declarations that decls builds from the path and the content of
// the file, in the package of the directory of the file. decls builds new
// declarations on each call, because the load assigns their identities
// and their hosts. Where warn is set, the parse reports one Warning for
// each file. Where fail is set, the parse returns it.
type literal struct {
	*frontendtest.Scripted
	decls func(path, content string) []symbol.Symbol
	warn  bool
	fail  error
}

// Parse returns the error of the case, or reads each file of the unit and
// lowers it to its declarations.
func (l literal) Parse(_ context.Context, u *plugin.SourceUnit) error {
	if l.fail != nil {
		return l.fail
	}
	for _, ref := range u.Files() {
		b, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		at := position.Pos{File: ref.Path, Line: 1}
		if l.warn {
			u.Warnf(frontendtest.ScriptedBadFile, at, "%s is a fixture", ref.Path)
		}
		dir := path.Dir(ref.Path)
		pkg := u.Graph().Package(dir)
		pkg.Name = path.Base(dir)
		pkg.Files = append(pkg.Files, &node.File{Path: ref.Path, Pos: at, Decls: l.decls(ref.Path, string(b))})
	}
	return nil
}

// The checks load generated output with its sources, as a toolchain run
// lays it out, under a brand of their own.
func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("Surface", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a generated file in place of the source file at its path", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{
				Files:   map[string][]byte{apiFile: []byte("New")},
				Sources: fstest.MapFS{apiFile: {Data: []byte("Old")}},
			}
			got, err := toolchain.Surface(t.Context(), literal{Scripted: frontendtest.NewScripted(), decls: named}, g)
			assert.NoError(t, err, "the output loads")
			assert.Equal(t, got, "package "+genPackage+"\nStruct New\n", "the generated file replaces the source file")
		})

		t.Run("reads the declarations of the generated files alone", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{
				Files:   map[string][]byte{apiFile: []byte("Api")},
				Sources: fstest.MapFS{genSource: {Data: []byte("Source")}},
			}
			got, err := toolchain.Surface(t.Context(), literal{Scripted: frontendtest.NewScripted(), decls: named}, g)
			assert.NoError(t, err, "the output loads")
			assert.Equal(t, got, "package "+genPackage+"\nStruct Api\n",
				"the surface has no declaration of a source file")
		})

		t.Run("leaves out a file in the frame of the brand of the load", func(t *testing.T) {
			t.Parallel()

			c, err := frame.NewContract(loadBrand, frontendtest.NewScripted().Syntax())
			assert.NoError(t, err, "the contract of the brand of the load composes")
			extra := plugin.RenderedFile{Path: extraFile, Plugins: []plugin.ID{"gen"}, Body: []byte("Extra\n")}
			framed, err := c.Stamp(extra)
			assert.NoError(t, err, "the extra file stamps")
			g := toolchain.Generated{Files: map[string][]byte{apiFile: []byte("Api"), extraFile: framed}}
			got, err := toolchain.Surface(t.Context(), literal{Scripted: frontendtest.NewScripted(), decls: named}, g)
			assert.NoError(t, err, "the output loads")
			assert.Equal(t, got, "package "+genPackage+"\nStruct Api\n", "the load leaves out the framed file")
		})

		t.Run("returns the error of a load that fails", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{apiFile: []byte("Api")}}
			_, err := toolchain.Surface(t.Context(),
				literal{Scripted: frontendtest.NewScripted(), decls: named, fail: errParse}, g)
			assert.ErrorIs(t, err, errParse, "the error of the parse fails the load")
		})

		t.Run("accepts a Warning of the load", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{apiFile: []byte("Api")}}
			got, err := toolchain.Surface(t.Context(),
				literal{Scripted: frontendtest.NewScripted(), decls: named, warn: true}, g)
			assert.NoError(t, err, "a Warning fails no load")
			assert.Equal(t, got, "package "+genPackage+"\nStruct Api\n", "the surface has the declaration of the file")
		})
	})
}

// named returns a public struct whose name is the content of the file at
// path, without its surrounding space.
func named(path, content string) []symbol.Symbol {
	return []symbol.Symbol{&node.Struct{
		Name: strings.TrimSpace(content), Pos: position.Pos{File: path, Line: 1}, Visibility: symbol.VisibilityPublic,
	}}
}
