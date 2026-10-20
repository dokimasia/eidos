// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of PEP 681 that make a class a dataclass: the decorator
// dataclass_transform of typing, its argument of the field specifiers, the
// suffix of its arguments that state the default of an option, and the
// keyword of the metaclass of a class statement.
const (
	transformDecorator = "dataclass_transform"
	specifiersArg      = "field_specifiers"
	defaultSuffix      = "_default"
	metaclassKeyword   = "metaclass"
)

// qualified is a declaration of Python by its module and its name, as
// [canonical] returns them.
type qualified struct{ module, name string }

// dataclassForm is what makes a class a dataclass: the options of its
// decorator or its class statement, followed by the defaults of the
// dataclass_transform that applies as options of the same names, and the
// field specifiers of the transform. [option] reads the first option of a
// name, so an option of the class overrides a default of the transform. A
// field whose value calls field of dataclasses or a field specifier states
// the options of the field.
type dataclassForm struct {
	options    []string
	specifiers []qualified
}

// dataclass returns the form of the class s as a dataclass, and reports
// whether s is one:
//
//   - a class under the decorator dataclass of dataclasses;
//   - a class under a decorator whose function dataclass_transform marks,
//     with the options of the call of the decorator;
//   - a class whose metaclass the transform marks, a class with a base that
//     the transform marks, and a class with a base that is such a class, to
//     the default depth of the kernel, with the keywords of its class
//     statement as its options. The class that the transform marks is no
//     dataclass itself.
//
// It allocates the options of a class that a transform makes a dataclass,
// and the field specifiers of the transform.
func (Rules) dataclass(s *node.Struct, v rules.View) (dataclassForm, bool) {
	scope := v.ScopeOf(s.ID)
	pkg, _ := v.PackageOf(s.ID)
	for _, a := range s.Annotations {
		module, name := canonical(pkg, scope.File, a.Name)
		if module == dataclassesModule && name == dataclassDecorator {
			return dataclassForm{options: a.Args}, true
		}
		if t, marked := decoratorTransform(module, name, v); marked {
			return dataclassForm{options: slices.Concat(a.Args, t.options), specifiers: t.specifiers}, true
		}
	}
	keywords, _ := rules.Fact(v, s.ID, meta.Named[[]string](python.KeywordsKey))
	t, marked := inheritedTransform(s, keywords, v, 0)
	if !marked {
		return dataclassForm{}, false
	}
	own := slices.DeleteFunc(slices.Clone(keywords), func(k string) bool {
		key, _, _ := strings.Cut(k, optionSep)
		return strings.TrimSpace(key) == metaclassKeyword
	})
	return dataclassForm{options: slices.Concat(own, t.options), specifiers: t.specifiers}, true
}

// transformOf returns the transform that the annotations of a declaration of
// the file f of the package pkg state: the defaults of dataclass_transform of
// typing or typing_extensions, each as an option of its name without the
// suffix _default, and its field specifiers, each a name of f. It reports
// false where no annotation is the transform.
func transformOf(annotations symbol.Annotations, pkg *node.Package, f *node.File) (dataclassForm, bool) {
	for _, a := range annotations {
		module, name := canonical(pkg, f, a.Name)
		if name != transformDecorator || module != typingModule && module != typingExtensions {
			continue
		}
		var t dataclassForm
		for _, arg := range a.Args {
			key, value, _ := strings.Cut(arg, optionSep)
			key, value = strings.TrimSpace(key), strings.TrimSpace(value)
			if key == specifiersArg {
				for spec := range arguments(strings.TrimSuffix(strings.TrimPrefix(value, callOpen), callClose)) {
					m, n := canonical(pkg, f, spec)
					t.specifiers = append(t.specifiers, qualified{module: m, name: n})
				}
				continue
			}
			if option, isDefault := strings.CutSuffix(key, defaultSuffix); isDefault {
				t.options = append(t.options, option+optionSep+value)
			}
		}
		return t, true
	}
	return dataclassForm{}, false
}

// decoratorTransform returns the transform of the function name of the
// module, which a decorator names, and reports false where the module
// declares no function of the name that the transform marks. A function of
// overloads has the transform where one of its declarations states it.
func decoratorTransform(module, name string, v rules.View) (dataclassForm, bool) {
	sym, _ := v.Lookup(symbol.Identity{Lang: python.Lang, Package: module, Kind: symbol.KindPackage})
	pkg, isPackage := sym.(*node.Package)
	if !isPackage {
		return dataclassForm{}, false
	}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			fn, isFunction := d.(*node.Function)
			if !isFunction || fn.Name != name {
				continue
			}
			if t, marked := transformOf(fn.Annotations, pkg, f); marked {
				return t, true
			}
		}
	}
	return dataclassForm{}, false
}

// inheritedTransform returns the transform that the class s takes from its
// class statement, whose keywords are keywords, and from its bases: the
// transform of its metaclass, of a base that the transform marks, and the
// transform that a base takes in turn, to the default depth of the kernel
// at depth.
func inheritedTransform(s *node.Struct, keywords []string, v rules.View, depth int) (dataclassForm, bool) {
	scope := v.ScopeOf(s.ID)
	pkg, _ := v.PackageOf(s.ID)
	if metaclass := option(slices.Values(keywords), metaclassKeyword); metaclass != "" {
		module, name := canonical(pkg, scope.File, metaclass)
		id := symbol.Identity{Lang: python.Lang, Package: module, Name: name, Kind: symbol.KindStruct}
		if mc, isClass := lookupClass(id, v); isClass {
			if t, marked := transformOf(mc.Annotations, mc.pkg, mc.file); marked {
				return t, true
			}
		}
	}
	if depth >= rules.DefaultDepth {
		return dataclassForm{}, false
	}
	for _, ext := range s.Extends {
		if ext == nil {
			continue
		}
		base, isClass := lookupClass(ext.Target, v)
		if !isClass {
			continue
		}
		if t, marked := transformOf(base.Annotations, base.pkg, base.file); marked {
			return t, true
		}
		baseKeywords, _ := rules.Fact(v, base.ID, meta.Named[[]string](python.KeywordsKey))
		if t, marked := inheritedTransform(base.Struct, baseKeywords, v, depth+1); marked {
			return t, true
		}
	}
	return dataclassForm{}, false
}

// placedClass is a class of the view with the package and the file that
// declare it, whose names its annotations spell.
type placedClass struct {
	*node.Struct
	pkg  *node.Package
	file *node.File
}

// lookupClass returns the class of the identity id with its package and its
// file, and reports false where the view has no class of the identity.
func lookupClass(id symbol.Identity, v rules.View) (placedClass, bool) {
	sym, _ := v.Lookup(id)
	s, isClass := sym.(*node.Struct)
	if !isClass {
		return placedClass{}, false
	}
	pkg, _ := v.PackageOf(id)
	return placedClass{Struct: s, pkg: pkg, file: v.ScopeOf(id).File}, true
}
