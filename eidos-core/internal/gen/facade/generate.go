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
	set := make(genfile.Set, len(surfaces))
	for _, ps := range surfaces {
		rendered, err := render(ps, cur)
		if err != nil {
			return nil, err
		}
		p := path.Join(FacadeDir, ps.FacadeRel(), FileName)
		formatted, err := genfile.Format(p, rendered)
		if err != nil {
			return nil, err
		}
		set[p] = formatted
	}
	return set, nil
}

// Regenerate renders the facade from the kernel module enclosing
// dir and writes it beside that kernel.
//
// It is the whole of what the go:generate wrapper does, so the
// wrapper holds nothing but the exit status. A directory outside
// the kernel module is refused rather than generated into, and
// nothing is written unless every package rendered.
func Regenerate(dir string) error {
	kernelRoot, err := gosource.ModuleRoot(dir)
	if err != nil {
		return err
	}
	err = verifyKernel(kernelRoot)
	if err != nil {
		return err
	}

	repoRoot := filepath.Dir(kernelRoot)
	set, err := Generate(repoRoot)
	if err != nil {
		return err
	}
	return genfile.Write(repoRoot, set)
}
