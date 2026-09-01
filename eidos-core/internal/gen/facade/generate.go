// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package facade

import (
	"path"
	"path/filepath"

	"go.dokimi.dev/eidos/core/internal/genfile"
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
		p := path.Join(FacadeDir, ps.Rel, FileName)
		formatted, err := genfile.Format(p, rendered)
		if err != nil {
			return nil, err
		}
		set[p] = formatted
	}
	return set, nil
}
