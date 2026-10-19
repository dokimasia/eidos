// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/plugin"
)

// TemplatesDir is the directory of the templates of a plugin, which
// [TemplatesOf] reads. The tree of each target is the directory below it
// that has the name of the target, such as templates/golang. A plugin
// embeds the directory with //go:embed templates.
const TemplatesDir = "templates"

// TargetOption is one declaration of the presentation of one target, which
// [Builder.For] takes. It declares a template tree, helpers or overrides,
// and [Templates], [TemplatesOf], [Funcs] and [Overrides] construct it. The
// zero value declares nothing, and a typed build panics on it.
type TargetOption struct {
	kind  optionKind
	tree  fs.FS
	funcs template.FuncMap
	// defect is the defect of the tree that [TemplatesOf] found, which the
	// typed build reports, and empty for a tree without one.
	defect string
}

// Templates declares the template tree of a target. The plans of the
// target resolve the template references of the plugin in this tree, in
// place of the tree of the plugin level. A typed build panics on a nil
// tree. Templates returns the option by value and allocates nothing.
func Templates(tree fs.FS) TargetOption {
	return TargetOption{kind: optionTemplates, tree: tree}
}

// TemplatesOf declares the template tree of the target t from the files of
// a plugin, as [Templates] declares a tree: the directory templates/<t> of
// files, such as the files that a plugin embeds with //go:embed templates.
// A name of the tree is the path of a template below that directory, so a
// plugin of several targets declares each target in one call.
//
// A typed build panics where the name of t is no name of a directory,
// where files has no directory templates/<t>, and on nil files. Each
// defect comes from the source of the plugin, so the first build in any
// test reports it.
//
// # Allocation contract
//
// TemplatesOf returns the option by value. It allocates the joined path,
// what [fs.Stat] allocates for the directory, and the tree of [fs.Sub]. A
// refused declaration allocates the text of its defect.
func TemplatesOf(files fs.FS, t plugin.Target) TargetOption {
	opt := TargetOption{kind: optionTemplates}
	if files == nil {
		return opt
	}
	if name := string(t); strings.Contains(name, "/") || !fs.ValidPath(name) {
		opt.defect = "the target " + strconv.Quote(name) + " is no name of a directory"
		return opt
	}
	dir := path.Join(TemplatesDir, string(t))
	info, err := fs.Stat(files, dir)
	switch {
	case err != nil:
		opt.defect = err.Error()
	case !info.IsDir():
		opt.defect = dir + " is no directory"
	default:
		tree, err := fs.Sub(files, dir)
		if err != nil {
			opt.defect = err.Error()
			return opt
		}
		opt.tree = tree
	}
	return opt
}

// Funcs declares helpers for the plans of a target, layered over the
// helpers of the plugin level. A name declared at both levels takes this
// function in the plans of the target. A typed build panics on a nil map
// and on a function that text/template refuses. Funcs returns the option
// by value and allocates nothing.
func Funcs(fm template.FuncMap) TargetOption {
	return TargetOption{kind: optionFuncs, funcs: fm}
}

// Overrides declares replacements for names in the shared vocabulary of
// the target. In every plan of the target, each function replaces the
// shared helper of its name in every template that the plan renders, the
// kind templates of the backend included. A name that the shared
// vocabulary lacks fails the template lint, and where two plugins override
// one name, the override later in the schedule takes effect. A typed build
// panics on a nil map and on a function that text/template refuses.
// Overrides returns the option by value and allocates nothing.
func Overrides(fm template.FuncMap) TargetOption {
	return TargetOption{kind: optionOverrides, funcs: fm}
}

// optionKind is what a [TargetOption] declares.
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

// presentation is the resolved presentation of a plugin, which the
// [plugin.TemplateProvider] methods return. A typed build computes it
// once.
type presentation struct {
	// tree and funcs are the declarations of the plugin level, which serve
	// every target without a declaration of its own.
	tree  fs.FS
	funcs template.FuncMap
	// trees maps each target for which [Builder.For] declared a tree to
	// that tree, and targets lists those targets, sorted.
	trees   map[plugin.Target]fs.FS
	targets []plugin.Target
	// layered maps each target of [Builder.For] to its helpers, which are
	// the helpers of the plugin level, then the helpers of the target, and
	// then its overrides. overrides maps each target to the sorted names
	// that it replaces.
	layered   map[plugin.Target]template.FuncMap
	overrides map[plugin.Target][]string
}

// level is the resolved declarations of one level, the plugin level or
// one target.
type level struct {
	tree      fs.FS
	funcs     template.FuncMap
	overrides template.FuncMap
}

// resolvePresentation resolves the declarations of the plugin level and
// every target layer. It panics on a declaration defect with the name of
// the plugin and the level of the defect.
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

// resolveLevel folds the options of one level. A level has one tree at
// most, and declares every helper and override name once.
func resolveLevel(name, where string, opts []TargetOption) level {
	var l level
	declared := map[string]bool{}
	for _, o := range opts {
		switch o.kind {
		case optionTemplates:
			if o.defect != "" {
				panic("eidos: " + name + " declares the template tree " + where + ", and " + o.defect)
			}
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

// admitHelpers panics unless text/template accepts every function of fm
// under its name, so a helper that it refuses fails at the typed build and
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

// layer returns the helpers of the plugin level with the helpers and the
// overrides of a target over them, and nil where all three are empty.
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
