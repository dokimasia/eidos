// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"maps"
	"slices"
	"text/template"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
)

// The names of the vocabulary helpers that every backend declares under the
// same name, so one template text serves each target. Each backend binds
// its own function under each name.
const (
	// HelperDocs writes the documentation of a declaration in the comment
	// form of the target.
	HelperDocs = "docs"
	// HelperSpell writes a type reference as the target spells it.
	HelperSpell = "spell"
	// HelperSpellValue writes a type reference in a value position, where
	// the declaration must exist at run time, such as the class of a
	// construction. A target that imports a declaration for its type
	// checker alone imports it for its value there.
	HelperSpellValue = "spellvalue"
	// HelperTypeParams writes the type parameter list of a declaration.
	HelperTypeParams = "typeparams"
	// HelperTypeArgs writes a list of type parameters or of type
	// references as the type arguments of a reference, and nothing for an
	// empty list.
	HelperTypeArgs = "typeargs"
	// HelperParams writes the parameter list of a callable.
	HelperParams = "params"
	// HelperResults writes the results of a callable.
	HelperResults = "results"
)

// mergeVocabulary folds the helpers of the plugins over the shared
// vocabulary. It merges in schedule order, so the helper of a later
// plugin replaces the helper of an earlier one. Plugins that the
// schedule does not list merge first, in name order, so a fixture
// without a schedule merges in one order. The merge reports and leaves
// out three kinds of helper:
//
//   - A helper that shadows a shared name without a declared override.
//     The shared helper keeps the name.
//   - A helper that claims the name of a builtin. The builtin keeps the
//     name.
//   - A helper that text/template cannot call.
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
			switch admit(name, fm[name], shared, declared[name]) {
			case claimsBuiltin:
				f.sink.Errorf(UndeclaredOverride, at, f.origin,
					"%s claims %q, which is a builtin, so the builtin keeps the name",
					id, name)
			case shadowsUndeclared:
				f.sink.Errorf(UndeclaredOverride, at, f.origin,
					"%s shadows the shared helper %q without declaring the override, "+
						"so the shared helper keeps the name",
					id, name)
			case unbindable:
				f.sink.Errorf(UnbindableHelper, at, f.origin,
					"%s registers the helper %q, which text/template cannot call, so the render leaves it out",
					id, name)
			case admitted:
				if !shared {
					if owner, taken := owners[name]; taken {
						f.sink.Errorf(HelperCollision, at, f.origin,
							"%s and %s both register the helper %q, so the helper of %s keeps the name",
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
// vocabulary. The merge of the render and the lint apply the same
// rule, so the render binds every helper that the lint admits.
type admission uint8

// admit classifies the helper fn under name. shared reports that the
// shared vocabulary defines the name, and declared reports that the
// plugin declared the override.
func admit(name string, fn any, shared, declared bool) admission {
	switch {
	case reserved(name):
		return claimsBuiltin
	case shared && !declared:
		return shadowsUndeclared
	case !binds(name, fn):
		return unbindable
	default:
		return admitted
	}
}

// binds reports whether text/template binds fn as a template function
// under name. text/template panics on a name that is not an
// identifier, on a value that is not a function, and on a function
// whose results a template cannot use. binds recovers that panic, so
// the caller reports the helper instead of failing.
func binds(name string, fn any) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	template.New(name).Funcs(template.FuncMap{name: fn})
	return true
}
