// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"path"
	"slices"

	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// packageJSON is the manifest of an npm package, whose name is the module
// of the packages that its files declare.
const packageJSON = "package.json"

// governed is the pair of manifests that govern a file: the tsconfig.json
// nearest above it and the package.json nearest above it.
type governed struct {
	config   string
	manifest string
}

// partition makes each claimed file a unit of its own, because
// TypeScript scopes the names of a module to its file. Each member
// declares its shared inputs. They are the tsconfig chain that governs it,
// which is the nearest tsconfig.json above its directory and every
// configuration that it extends, and then the nearest package.json above
// its directory. A file without a tsconfig.json or a package.json above
// it declares no such input. A chain that does not read whole declares the
// configurations read before the fault, and the parse reports the fault.
// Every read folds into the unit keys, so a configuration or a manifest
// that appears, changes or vanishes re-keys the files that it governs.
func partition(_ context.Context, files []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	configs, manifests := map[string]string{}, map[string]string{}
	chains := map[string][]string{}
	shared := map[governed][]string{}
	out := make([][]plugin.SourceRef, 0, len(files))
	for _, ref := range files {
		dir := path.Dir(ref.Path)
		g := governed{
			config:   frontend.Nearest(r, dir, tsconfigName, configs),
			manifest: frontend.Nearest(r, dir, packageJSON, manifests),
		}
		inputs, known := shared[g]
		if !known {
			if g.config != "" {
				chain, read := chains[g.config]
				if !read {
					chain, _, _ = readChain(r.Read, g.config)
					chains[g.config] = chain
				}
				inputs = chain
			}
			if g.manifest != "" {
				inputs = append(slices.Clip(inputs), g.manifest)
			}
			shared[g] = inputs
		}
		out = append(out, []plugin.SourceRef{{Path: ref.Path, Shared: inputs}})
	}
	return out, nil
}
