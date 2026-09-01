// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// PackageSurface is one curated package as parsed: the syntax the
// renderer re-exports, with the positions refusals point at.
type PackageSurface struct {
	Surface

	// KernelName is the package name the kernel sources declare.
	KernelName string

	// KernelPath is the kernel package's import path.
	KernelPath string

	// Fset positions every parsed file, shared across the run so
	// one refusal format serves every package.
	Fset *token.FileSet

	// Files are the package's parsed sources, in file-name order,
	// generated files included: the models are generated, and a
	// facade that skipped them would re-export half the surface.
	Files []*ast.File
}

// Lower parses every curated package under the kernel module root
// and returns the surfaces in curated order.
//
// It refuses a root whose go.mod names another module, a package
// whose files disagree on the package name, and a dot import,
// which would make qualified references unresolvable from syntax.
func Lower(kernelRoot string) ([]*PackageSurface, error) {
	if err := verifyKernel(kernelRoot); err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	surfaces := make([]*PackageSurface, 0, len(Surfaces))
	for _, s := range Surfaces {
		dir := filepath.Join(kernelRoot, filepath.FromSlash(s.Rel))
		files, err := gosource.ParseDir(fset, dir, gosource.Complete)
		if err != nil {
			return nil, fmt.Errorf("facade: parse %s: %w", s.KernelImportPath(), err)
		}
		ps := &PackageSurface{
			Surface:    s,
			KernelName: files[0].Name.Name,
			KernelPath: s.KernelImportPath(),
			Fset:       fset,
			Files:      files,
		}
		if err := ps.validate(); err != nil {
			return nil, err
		}
		surfaces = append(surfaces, ps)
	}
	return surfaces, nil
}

// KernelImportPath returns the kernel import path of a curated
// entry.
func (s Surface) KernelImportPath() string {
	if s.Rel == "" {
		return KernelModule
	}
	return KernelModule + "/" + s.Rel
}

// FacadeImportPath returns the facade import path of a curated
// entry.
func (s Surface) FacadeImportPath() string {
	if s.Rel == "" {
		return FacadeModule
	}
	return FacadeModule + "/" + s.Rel
}

// validate refuses what re-export cannot survive: files disagreeing
// on the package name, and dot imports, which erase the qualifier
// syntax resolution depends on.
func (ps *PackageSurface) validate() error {
	for _, f := range ps.Files {
		if got := f.Name.Name; got != ps.KernelName {
			return ps.at(f.Name.Pos(), "package %s declared beside package %s", got, ps.KernelName)
		}
		for _, imp := range f.Imports {
			if imp.Name != nil && imp.Name.Name == "." {
				return ps.at(imp.Pos(), "dot import hides the qualifier re-export resolves by")
			}
		}
	}
	return nil
}

// at formats a refusal anchored to a kernel source position.
func (ps *PackageSurface) at(pos token.Pos, format string, a ...any) error {
	return fmt.Errorf("facade: %s: %s", ps.Fset.Position(pos), fmt.Sprintf(format, a...))
}

// verifyKernel confirms root holds the kernel module, so a
// generator pointed at the wrong tree refuses instead of emitting
// a facade over whatever it found.
func verifyKernel(kernelRoot string) error {
	got, err := gosource.ModulePath(kernelRoot)
	if err != nil {
		return fmt.Errorf("facade: %w", err)
	}
	if got != KernelModule {
		return fmt.Errorf("facade: %s holds module %s, not the kernel %s", kernelRoot, got, KernelModule)
	}
	return nil
}
