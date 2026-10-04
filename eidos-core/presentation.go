// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"text/template"

	"go.dokimi.dev/eidos/core/plugin"
)

// TargetOption is one presentation declaration for one target, the
// argument [Builder.For] takes: a template tree, helpers, or
// overrides. [Templates], [Funcs] and [Overrides] construct one.
// The zero value declares nothing, and Build panics on it.
type TargetOption struct {
	kind  optionKind
	tree  fs.FS
	funcs template.FuncMap
}

// Templates declares a target's template tree: the tree the
// target's plans resolve this plugin's template references in, in
// place of the plugin-level tree. A nil tree panics at Build.
// Templates returns the option by value and allocates nothing.
func Templates(tree fs.FS) TargetOption {
	return TargetOption{kind: optionTemplates, tree: tree}
}

// Funcs declares helpers for a target's plans, layered over the
// plugin-level helpers: a name declared at both levels takes this
// function in that target's plans. A nil map, and a function
// text/template refuses, panic at Build. Funcs returns the option by
// value and allocates nothing.
func Funcs(fm template.FuncMap) TargetOption {
	return TargetOption{kind: optionFuncs, funcs: fm}
}

// Overrides declares replacements for names in the target's shared
// vocabulary, which is the replace verb: in every plan of that
// target, each function replaces the shared helper of its name in
// every template the plan renders, the backend's kind templates
// included. A name the shared vocabulary lacks fails the template
// lint, and where two plugins override one name, the one later in
// the schedule takes effect. A nil map, and a function
// text/template refuses, panic at Build. Overrides returns the option
// by value and allocates nothing.
func Overrides(fm template.FuncMap) TargetOption {
	return TargetOption{kind: optionOverrides, funcs: fm}
}

// optionKind names what a [TargetOption] declares.
type optionKind uint8

const (
	// optionTemplates declares a template tree.
	optionTemplates optionKind = 1
	// optionFuncs declares helpers beside the shared vocabulary.
	optionFuncs optionKind = 2
	// optionOverrides declares replacements for names in the shared
	// vocabulary.
	optionOverrides optionKind = 3
)

// presentation is a plugin's resolved presentation: what the
// [plugin.TemplateProvider] surface returns, computed once at Build.
type presentation struct {
	// tree and funcs are the plugin-level declarations, which serve
	// every target without a declaration of its own.
	tree  fs.FS
	funcs template.FuncMap
	// trees maps each target [Builder.For] declared a tree for to
	// that tree, and targets lists those targets, sorted.
	trees   map[plugin.Target]fs.FS
	targets []plugin.Target
	// layered maps each [Builder.For] target to its helpers: the
	// plugin-level helpers, then the target's helpers, then its
	// overrides. overrides maps each target to the names it
	// replaces, sorted.
	layered   map[plugin.Target]template.FuncMap
	overrides map[plugin.Target][]string
}

// level is one level's declarations after resolution: the
// plugin-level ones or one target's.
type level struct {
	tree      fs.FS
	funcs     template.FuncMap
	overrides template.FuncMap
}

// resolvePresentation resolves the plugin-level declarations and
// every target layer, and panics on a declaration defect with the
// plugin's name and the level it found the defect at.
func resolvePresentation(name string, defaults []TargetOption, layers []targetLayer) presentation {
	base := resolveLevel(name, "at plugin level", defaults)
	p := presentation{tree: base.tree, funcs: base.funcs}
	for _, l := range layers {
		if l.target == "" {
			panic("eidos: " + name + " declares presentation for the zero target")
		}
		where := "for target " + strconv.Quote(string(l.target))
		if _, taken := p.layered[l.target]; taken {
			panic("eidos: " + name + " declares presentation " + where + " twice")
		}
		if len(l.opts) == 0 {
			panic("eidos: " + name + " declares no presentation option " + where)
		}
		own := resolveLevel(name, where, l.opts)
		if p.layered == nil {
			p.layered = map[plugin.Target]template.FuncMap{}
		}
		p.layered[l.target] = layer(base.funcs, own.funcs, own.overrides)
		if own.tree != nil {
			if p.trees == nil {
				p.trees = map[plugin.Target]fs.FS{}
			}
			p.trees[l.target] = own.tree
			p.targets = append(p.targets, l.target)
		}
		if len(own.overrides) > 0 {
			if p.overrides == nil {
				p.overrides = map[plugin.Target][]string{}
			}
			p.overrides[l.target] = slices.Sorted(maps.Keys(own.overrides))
		}
	}
	slices.Sort(p.targets)
	return p
}

// resolveLevel folds one level's options: one tree at most, and
// every helper and override name declared once across the level.
func resolveLevel(name, where string, opts []TargetOption) level {
	var l level
	declared := map[string]bool{}
	for _, o := range opts {
		switch o.kind {
		case optionTemplates:
			if o.tree == nil {
				panic("eidos: " + name + " declares a nil template tree " + where)
			}
			if l.tree != nil {
				panic("eidos: " + name + " declares two template trees " + where)
			}
			l.tree = o.tree
		case optionFuncs, optionOverrides:
			if o.funcs == nil {
				panic("eidos: " + name + " declares a nil helper map " + where)
			}
			admitHelpers(name, where, o.funcs)
			into := &l.funcs
			if o.kind == optionOverrides {
				into = &l.overrides
			}
			if *into == nil {
				*into = template.FuncMap{}
			}
			for _, helper := range slices.Sorted(maps.Keys(o.funcs)) {
				if declared[helper] {
					panic("eidos: " + name + " declares the helper " +
						strconv.Quote(helper) + " twice " + where)
				}
				declared[helper] = true
				(*into)[helper] = o.funcs[helper]
			}
		default:
			panic("eidos: " + name + " declares the zero presentation option " + where)
		}
	}
	return l
}

// admitHelpers panics unless text/template accepts every function
// of fm under its name, so a helper it refuses fails at Build and
// not when a plan first renders.
func admitHelpers(name, where string, fm template.FuncMap) {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Sprintf("eidos: %s declares a helper %s that text/template refuses: %v",
				name, where, r))
		}
	}()
	template.New(name).Funcs(fm)
}

// layer returns the plugin-level helpers with a target's helpers and
// overrides over them, nil where all three are empty.
func layer(base, funcs, overrides template.FuncMap) template.FuncMap {
	if len(base)+len(funcs)+len(overrides) == 0 {
		return nil
	}
	out := make(template.FuncMap, len(base)+len(funcs)+len(overrides))
	maps.Copy(out, base)
	maps.Copy(out, funcs)
	maps.Copy(out, overrides)
	return out
}
