// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
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

// chainRead is what the partition keeps of one read of a tsconfig chain:
// the paths of its configurations in the order read, and the options of its
// library.
type chainRead struct {
	paths   []string
	library libraryOptions
}

// partition makes each claimed file a unit of its own, because
// TypeScript scopes the names of a module to its file. Each member
// declares its shared inputs. They are the tsconfig chain that governs it,
// which is the nearest tsconfig.json above its directory and every
// configuration that it extends, then the first root file of its library
// where the library store has it, and then the nearest package.json above
// its directory. A file without a tsconfig.json or a package.json above it
// declares no such input, and a file without a tsconfig.json has the default
// library. A chain that does not read whole declares the configurations
// read before the fault, and the parse reports the fault. Every read folds
// into the unit keys, so a configuration, a manifest or a library that
// appears, changes or vanishes re-keys the files that it governs.
//
// Error modes: partition returns the error of a probe of the library store
// that fails for another cause than its absence.
func partition(_ context.Context, files []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	configs, manifests := map[string]string{}, map[string]string{}
	chains := map[string]chainRead{}
	libraries := map[string]string{}
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
			var opts libraryOptions
			if g.config != "" {
				c, read := chains[g.config]
				if !read {
					// The parse reports a chain that does not read whole, so
					// the partition keeps what read before the fault.
					var s settings
					c.paths, s, _ = readChain(r.Read, g.config)
					c.library = s.library
					chains[g.config] = c
				}
				inputs, opts = c.paths, c.library
			}
			library, err := libraryInput(r, opts, libraries)
			if err != nil {
				return nil, err
			}
			if library != "" {
				inputs = append(slices.Clip(inputs), library)
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

// libraryInput returns the shared input that names the library of the
// options of a chain: the store path of the first root file of the library,
// inside the key of the library. It returns the empty path for an empty
// library and for a load without the library store. probed maps each key
// that the partition probed onto its input, so each key reads once.
//
// Error modes: libraryInput returns the error of a probe that fails for
// another cause than an absent store or file.
func libraryInput(r plugin.FileReader, opts libraryOptions, probed map[string]string) (string, error) {
	roots, _, _ := rootsOf(opts)
	if len(roots) == 0 {
		return "", nil
	}
	key := strings.Join(roots, typescript.LibraryKeySep)
	if input, met := probed[key]; met {
		return input, nil
	}
	input := plugin.StorePath(typescript.LibraryStore, key+libraryStoreSep+roots[0])
	_, err := r.Read(input)
	switch {
	case plugin.Absent(err):
		input = ""
	case err != nil:
		return "", fmt.Errorf("frontend: probe the library %s: %w", key, err)
	}
	probed[key] = input
	return input, nil
}
