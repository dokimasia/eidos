// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// link resolves every type reference in the graph, nested type
// arguments included, through the owning language's own bindings:
// the frontend's Resolve names the candidates in probe order, the
// first one the graph holds becomes the target, several held ones
// report under [AmbiguousReference], and none leaves the spelling
// alone — degradation a reader can ask about rather than failure.
//
// A file without a recorded scope resolves nothing: there are no
// bindings to resolve through, and its spellings stay spellings.
func link(packages []*spliced, scopes []scopeEntry, ix *index, sink *diag.Sink) {
	byFile := make(map[*node.File]scopeEntry, len(scopes))
	for _, s := range scopes {
		byFile[s.file] = s
	}
	for _, sp := range packages {
		for _, f := range sp.pkg.Files {
			entry, held := byFile[f]
			if !held {
				continue
			}
			scope := plugin.ImportScope{File: f.ID, Bindings: entry.bindings}
			node.Walk(f, func(s symbol.Symbol) bool {
				if ref, is := s.(*node.TypeRef); is && ref.Spelling != "" && ref.Target.IsZero() {
					resolve(ref, entry.frontend, scope, ix, sink)
				}
				return true
			})
		}
	}
}

// resolve settles one reference: candidates in probe order, held
// ones kept in that order, the first standing as the target.
func resolve(
	ref *node.TypeRef, f plugin.Frontend, scope plugin.ImportScope,
	ix *index, sink *diag.Sink,
) {
	var hits []symbol.Identity
	for _, c := range f.Resolve(scope, ref.Spelling) {
		for _, full := range ix.lookup(c) {
			if !slices.Contains(hits, full) {
				hits = append(hits, full)
			}
		}
	}
	if len(hits) == 0 {
		return
	}
	ref.Target = hits[0]
	if len(hits) > 1 {
		names := make([]string, len(hits))
		for i, h := range hits {
			names[i] = h.String()
		}
		sink.Warnf(AmbiguousReference, ref.Pos, f.Name(),
			"%q resolves to %s; the first stands", ref.Spelling, strings.Join(names, " and "))
	}
}
