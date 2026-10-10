// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// The spellings of vendor/modules.txt: the directory a module vendors
// into, the file inside it, a module line's and an annotation line's
// openings, the arrow that opens a replacement, and the annotation
// that marks a module its go.mod requires.
const (
	vendorDir        = "vendor"
	vendorModules    = "modules.txt"
	vendorModuleLine = "# "
	vendorAnnotation = "## "
	vendorArrow      = "=>"
	vendorExplicit   = "explicit"
)

// vendorList is what one vendor/modules.txt records: the modules its
// packages come from, in file order, the annotations and replacement
// of every module it lists, and the modules it lists as replaced, in
// file order.
type vendorList struct {
	provided []module.Version
	meta     map[module.Version]vendorMeta
	replaced []module.Version
}

// vendorMeta is one listed module's annotations: whether its go.mod
// requires it, and what replaces it, the zero Version for nothing.
type vendorMeta struct {
	explicit    bool
	replacement module.Version
}

// parseVendor reads a vendor/modules.txt the way the go command reads
// it. A "# path version" line opens a module, "# path => new" opens a
// replacement of every version, and either can end in a replacement,
// "=> dir" or "=> path version". A "## " line annotates the open
// module, and a line with one import path lists a package the module
// provides. A line of another shape is left out.
func parseVendor(data []byte) vendorList {
	list := vendorList{meta: map[module.Version]vendorMeta{}}
	var mod module.Version
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, vendorModuleLine) {
			mod = list.openModule(strings.Fields(line))
			continue
		}
		if mod.Path == "" {
			continue
		}
		if annotations, annotated := strings.CutPrefix(line, vendorAnnotation); annotated {
			meta := list.meta[mod]
			for entry := range strings.SplitSeq(annotations, ";") {
				if strings.TrimSpace(entry) == vendorExplicit {
					meta.explicit = true
				}
			}
			list.meta[mod] = meta
			continue
		}
		if f := strings.Fields(line); len(f) == 1 && module.CheckImportPath(f[0]) == nil {
			list.provide(mod)
		}
	}
	return list
}

// openModule reads a module line's fields, the leading mark first,
// records its replacement, and returns the module it opens, the zero
// Version for a line the go command reads as no module.
func (list *vendorList) openModule(f []string) module.Version {
	if len(f) < 3 {
		return module.Version{}
	}
	var mod module.Version
	switch {
	case semver.IsValid(f[2]):
		mod, f = module.Version{Path: f[1], Version: f[2]}, f[3:]
	case f[2] == vendorArrow:
		mod, f = module.Version{Path: f[1]}, f[2:]
	default:
		return module.Version{}
	}
	if len(f) < 2 || f[0] != vendorArrow {
		return mod
	}
	var replacement module.Version
	switch {
	case len(f) == 2:
		replacement = module.Version{Path: f[1]}
	case len(f) == 3 && semver.IsValid(f[2]):
		replacement = module.Version{Path: f[1], Version: f[2]}
	default:
		return mod
	}
	meta := list.meta[mod]
	meta.replacement = replacement
	list.meta[mod] = meta
	list.replaced = append(list.replaced, mod)
	return mod
}

// provide records that a module provides a package, once per module.
func (list *vendorList) provide(mod module.Version) {
	if !slices.Contains(list.provided, mod) {
		list.provided = append(list.provided, mod)
	}
}

// checkVendor runs the go command's four consistency checks of a
// vendor/modules.txt against the go.mod beside its vendor directory,
// and returns an error naming the first mismatch: a requirement the
// list does not mark explicit, a replacement the list does not record
// or records otherwise, an explicit module the go.mod does not
// require, and a replacement the go.mod does not state.
func checkVendor(f *modfile.File, list vendorList) error {
	required := map[module.Version]bool{}
	for _, r := range f.Require {
		required[r.Mod] = true
		if !list.meta[r.Mod].explicit {
			return fmt.Errorf("%s is explicitly required in go.mod, but not marked as explicit in %s",
				r.Mod, vendorModules)
		}
	}
	for _, r := range f.Replace {
		recorded := list.meta[r.Old].replacement
		if recorded == (module.Version{}) {
			return fmt.Errorf("%s is replaced in go.mod, but not marked as replaced in %s", r.Old, vendorModules)
		}
		if recorded != r.New {
			return fmt.Errorf("%s is replaced by %s in go.mod, but marked as replaced by %s in %s",
				r.Old, r.New, recorded, vendorModules)
		}
	}
	for _, mod := range list.provided {
		if list.meta[mod].explicit && !required[mod] {
			return fmt.Errorf("%s is marked as explicit in %s, but not explicitly required in go.mod",
				mod, vendorModules)
		}
	}
	for _, mod := range list.replaced {
		if !replaces(f, mod) {
			return fmt.Errorf("%s is marked as replaced in %s, but not replaced in go.mod", mod, vendorModules)
		}
	}
	return nil
}

// replaces reports whether a go.mod states a replacement of a module
// at a version, or of every version of it.
func replaces(f *modfile.File, mod module.Version) bool {
	for _, r := range f.Replace {
		if r.Old.Path == mod.Path && (r.Old.Version == "" || r.Old.Version == mod.Version) {
			return true
		}
	}
	return false
}
