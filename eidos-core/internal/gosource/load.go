// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package gosource

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

// The file suffixes that decide what a loader reads.
const (
	extGo        = ".go"
	extGenerated = ".gen.go"
	extTest      = "_test.go"
)

// ParseDir parses a directory's hand-written Go files, in name
// order, with comments attached.
//
// Generated files and test files are skipped, so the result is
// closed over hand-written source. Name order is what makes a
// caller's output depend on the schema's bytes rather than on the
// filesystem's iteration order.
//
// A directory holding no hand-written Go file is an error.
func ParseDir(fset *token.FileSet, dir string) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("gosource: read %s: %w", dir, err)
	}
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !readable(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("gosource: parse %s: %w", path, err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("gosource: no hand-written Go file in %s", dir)
	}
	return files, nil
}

// readable reports whether a filename is a hand-written Go source
// file the loaders read.
func readable(name string) bool {
	return strings.HasSuffix(name, extGo) &&
		!strings.HasSuffix(name, extGenerated) &&
		!strings.HasSuffix(name, extTest)
}

// Load parses and type-checks the package in dir.
//
// modRoot roots the importer that resolves the package's imports;
// see [NewImporter]. A package importing nothing loads with an
// empty modRoot. The type-checked package and the parsed files are
// both answered, because a caller that walks declarations wants the
// syntax and the caller that resolves names wants the types.
//
// pkgPath names the package under type-checking and appears in type
// spellings; it need not match the directory.
func Load(fset *token.FileSet, dir, pkgPath, modRoot string) (*types.Package, []*ast.File, error) {
	files, err := ParseDir(fset, dir)
	if err != nil {
		return nil, nil, err
	}
	imp, err := NewImporter(fset, modRoot)
	if err != nil {
		return nil, nil, err
	}
	conf := types.Config{Importer: imp}
	pkg, err := conf.Check(pkgPath, fset, files, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("gosource: type-check %s: %w", dir, err)
	}
	return pkg, files, nil
}
