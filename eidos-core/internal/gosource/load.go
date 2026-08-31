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

// Mode says which of a package's files a loader reads.
type Mode uint8

const (
	// HandWritten skips generated files and reports every
	// type-checking error. A generator reads its own input this
	// way, so its output can never feed it and a stale generated
	// file cannot change what it produces.
	HandWritten Mode = iota
	// Complete reads generated files and returns whatever resolved,
	// tolerating type-checking errors. A dependency is read this
	// way: the caller wants its names, and a dependency that does
	// not compile must not stop the generator that would fix it.
	Complete
)

// ParseDir parses a directory's Go files, in name order, with
// comments attached.
//
// Test files are always skipped; mode decides whether generated
// files are. Name order is what makes a caller's output depend on
// the package's bytes rather than on the filesystem's iteration
// order.
//
// A directory holding no readable Go file is an error.
func ParseDir(fset *token.FileSet, dir string, mode Mode) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("gosource: read %s: %w", dir, err)
	}
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !readable(entry.Name(), mode) {
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
		return nil, fmt.Errorf("gosource: no readable Go file in %s", dir)
	}
	return files, nil
}

// readable reports whether a filename is a Go source file the
// loader reads under mode.
func readable(name string, mode Mode) bool {
	if !strings.HasSuffix(name, extGo) || strings.HasSuffix(name, extTest) {
		return false
	}
	return mode == Complete || !strings.HasSuffix(name, extGenerated)
}

// Load parses and type-checks the package in dir.
//
// mode decides which of the package's own files are read; its
// imports always resolve in [Complete] mode, because hand-written
// code in a dependency may refer to what that dependency's own
// generator produced.
//
// modRoot roots the importer that resolves the package's imports;
// see [NewImporter]. A package importing nothing loads with an
// empty modRoot. The type-checked package and the parsed files are
// both returned, because a caller that walks declarations wants the
// syntax and the caller that resolves names wants the types.
//
// pkgPath names the package under type-checking and appears in type
// spellings; it need not match the directory.
func Load(
	fset *token.FileSet,
	dir, pkgPath, modRoot string,
	mode Mode,
) (*types.Package, []*ast.File, error) {
	files, err := ParseDir(fset, dir, mode)
	if err != nil {
		return nil, nil, err
	}
	imp, err := NewImporter(fset, modRoot)
	if err != nil {
		return nil, nil, err
	}
	conf := types.Config{Importer: imp}
	if mode == Complete {
		// Collect errors instead of stopping at the first, so a
		// dependency that does not compile still returns its names.
		conf.Error = func(error) {}
	}
	pkg, err := conf.Check(pkgPath, fset, files, nil)
	if err != nil && mode == HandWritten {
		return nil, nil, fmt.Errorf("gosource: type-check %s: %w", dir, err)
	}
	return pkg, files, nil
}
