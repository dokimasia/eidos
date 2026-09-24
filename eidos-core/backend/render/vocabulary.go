// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"maps"
	"slices"
	"text/template"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
)

// mergeVocabulary folds the plugins' helpers over the shared
// bucket: schedule order, latest wins, and plugins the schedule
// does not hold merge first, in name order, so a fixture without a
// schedule stays deterministic. A shared name shadowed without a
// declared override is refused and reported, and a reserved name
// is refused the same way, so the shared helper and the builtin
// stand.
func (f *frame) mergeVocabulary(ctx *plugin.RenderContext) template.FuncMap {
	merged := template.FuncMap{}
	scheduled := map[plugin.ID]bool{}
	order := make([]plugin.ID, 0, len(ctx.Funcs))
	for _, id := range ctx.Schedule {
		scheduled[id] = true
	}
	for _, id := range slices.Sorted(maps.Keys(ctx.Funcs)) {
		if !scheduled[id] {
			order = append(order, id)
		}
	}
	for _, id := range ctx.Schedule {
		if _, held := ctx.Funcs[id]; held {
			order = append(order, id)
		}
	}
	at := position.Pos{File: string(f.pass.name)}
	owners := map[string]plugin.ID{}
	for _, id := range order {
		declared := map[string]bool{}
		for _, name := range ctx.Overrides[id] {
			declared[name] = true
		}
		fm := ctx.Funcs[id]
		for _, name := range slices.Sorted(maps.Keys(fm)) {
			_, shared := f.pass.shared[name]
			switch admit(name, shared, declared[name]) {
			case claimsBuiltin:
				f.sink.Errorf(UndeclaredOverride, at, f.origin,
					"%s claims %q, which is a builtin, and the builtin stands",
					id, name)
			case shadowsUndeclared:
				f.sink.Errorf(UndeclaredOverride, at, f.origin,
					"%s shadows the shared helper %q without declaring the override, and the shared helper stands",
					id, name)
			case admitted:
				if !shared {
					if owner, taken := owners[name]; taken {
						f.sink.Errorf(HelperCollision, at, f.origin,
							"%s and %s both register the helper %q, and %s's stands",
							owner, id, name, owner)
						continue
					}
					owners[name] = id
				}
				merged[name] = fm[name]
			}
		}
	}
	return merged
}

// admission classifies one plugin helper against the shared
// vocabulary. The render's merge and the lint read the one rule, so
// a helper the lint admits is a helper the render binds.
type admission uint8

// admit classifies a helper name: whether the shared vocabulary
// defines the name, and whether the plugin declared the override.
func admit(name string, shared, declared bool) admission {
	switch {
	case reserved(name):
		return claimsBuiltin
	case shared && !declared:
		return shadowsUndeclared
	default:
		return admitted
	}
}
