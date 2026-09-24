// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade

import (
	"path"
	"path/filepath"

	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// Generate renders the whole facade module from the kernel
// checkout under repoRoot.
//
// It returns the output in memory, keyed by repository-relative
// slash path, and writes nothing. A caller puts it on disk with
// [genfile.Write] or compares it with [genfile.Verify], which is
// what makes the mirror guard a plain test.
//
// Nothing is returned unless every package rendered and formatted,
// so a refused surface cannot leave a half-generated facade.
func Generate(repoRoot string) (genfile.Set, error) {
	surfaces, err := Lower(filepath.Join(repoRoot, KernelDir))
	if err != nil {
		return nil, err
	}

	cur := curated()
	return genfile.Render(surfaces, func(ps *PackageSurface) (string, []byte, error) {
		src, err := render(ps, cur)
		return path.Join(FacadeDir, ps.FacadeRel(), FileName), src, err
	})
}

// Regenerate renders the facade from the kernel module enclosing
// dir and writes it beside that kernel.
//
// The go:generate wrapper calls Regenerate and reports only its
// exit status. A directory outside the kernel module is refused
// before anything is written, and nothing is written unless every
// package rendered.
func Regenerate(dir string) error {
	kernelRoot, err := gosource.ModuleRoot(dir)
	if err != nil {
		return err
	}
	err = verifyKernel(kernelRoot)
	if err != nil {
		return err
	}
	return genfile.Regenerate(filepath.Dir(kernelRoot), Generate)
}
