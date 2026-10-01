// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"path"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// partition makes each claimed file a unit of its own, because
// TypeScript scopes a module's names to its file. Each member declares
// the tsconfig chain that governs it as its shared inputs: the nearest
// tsconfig.json above its directory, then every configuration that one
// extends. A file with no tsconfig above it declares none. A chain that
// does not read whole declares the configurations read before the
// fault, and the parse reports the fault. Every read folds into the
// unit keys, so a configuration appearing, changing or vanishing
// re-keys the files it governs.
func partition(_ context.Context, files []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	nearest := map[string]string{}
	chains := map[string][]string{}
	out := make([][]plugin.SourceRef, 0, len(files))
	for _, ref := range files {
		dir := path.Dir(ref.Path)
		config, probed := nearest[dir]
		if !probed {
			config, _ = governing(r.Read, dir)
			nearest[dir] = config
		}
		var shared []string
		if config != "" {
			cached, read := chains[config]
			if !read {
				cached, _, _ = readChain(r.Read, config)
				chains[config] = cached
			}
			shared = cached
		}
		out = append(out, []plugin.SourceRef{{Path: ref.Path, Shared: shared}})
	}
	return out, nil
}
