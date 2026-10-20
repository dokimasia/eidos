// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The paths of the entries of an import set that are no imports: the
// entries that name the public declarations of the file, which the head of
// the file reads for its __all__, the entries that name the modules that
// the file imports lazily, which the import block reads, the entries that
// name the functions of the file without the mark Overload, which
// [Writer.Function] reads, the entries that name the bound names of the
// imports that the file does not use, which the import block leaves out,
// and the entries that name the bound names of the re-exports of the
// header, import a as a and from a import b as b, which the import block
// writes with their redundant aliases and never leaves out.
const (
	publicPath      = specialMark + "public"
	lazyPath        = specialMark + "lazy"
	implementedPath = specialMark + "implemented"
	unusedPath      = specialMark + "unused"
	reexportPath    = specialMark + "reexport"
)

// The parts of the head of a module: the mark of a comment and the
// assignment of __all__.
const (
	headComment = "#"
	allOpen     = "__all__ = ["
	allClose    = "]"
	dunder      = "__"
)

// Head writes the start of a module from its header h: the comments of the
// head marker, each after its #, and the docstring of the header. It binds
// the imports of the header and the features of its future marker in the
// import set, and records the modules of its lazy marker there, so the
// import block that follows the head writes them.
//
// Error modes: an annotation of the header that is no marker, a marker
// that is no marker of a header, the lazy marker for a version before
// Python 3.15, which adds lazy imports, and the errors of [Writer.Bind].
func (w *Writer) Head(h *emit.File) (string, error) {
	if h == nil {
		return "", nil
	}
	minor := minorOf(w.set)
	for _, a := range h.Annotations {
		m := python.Marker(a.Name)
		switch {
		case !m.Header():
			return "", refuse("a Python file states the header markers alone, and its header states %s", a.Name)
		case m == python.LazyMarker && minor < minor315:
			return "", refuse("lazy imports are a construct of Python 3.15, and the file runs on Python 3.%d", minor)
		case m == python.LazyMarker:
			for _, module := range a.Args {
				w.set.AddNamed(lazyPath, module)
			}
		case m == python.FutureMarker:
			for _, feature := range a.Args {
				w.set.AddNamed(futureModule, feature)
			}
		}
	}
	if _, err := w.Bind(h); err != nil {
		return "", err
	}
	comments, _ := markerArgs(h.Annotations, python.HeadMarker)
	size := docstringSize(h.Doc, "")
	for _, comment := range comments {
		size += len(headComment) + len(comment) + len(newline)
	}
	var b strings.Builder
	b.Grow(size)
	for _, comment := range comments {
		b.WriteString(headComment)
		b.WriteString(comment)
		b.WriteString(newline)
	}
	writeDocstring(&b, h.Doc, "")
	return b.String(), nil
}

// All writes the __all__ of the file with the header h: the names of the all
// marker of the header in their order, and the empty list for the marker
// without names. Without the marker, it writes the public names of the file
// in the order of isort where a public name starts with an underscore, which
// Python makes private without __all__, and nothing otherwise. The list
// wraps to one name on each line where it is longer than the line.
//
// # Allocation contract
//
// All allocates the list, the entries of the import set where the header
// states no all marker, the public names where one starts with an
// underscore, and the escaped text of a name that is no identifier.
func (w *Writer) All(h *emit.File) string {
	var names []string
	stated := false
	if h != nil {
		names, stated = markerArgs(h.Annotations, python.AllMarker)
	}
	if !stated {
		names = w.publicNames()
	}
	if !stated && len(names) == 0 {
		return ""
	}
	size := len(allOpen) + len(listOpen) + len(groupClose) + len(allClose) + len(newline)
	for _, n := range names {
		size += len(n) + 2*len(doubleQuote) + len(itemSep)
	}
	var b strings.Builder
	b.Grow(size)
	b.WriteString(allOpen + listOpen)
	for i, n := range names {
		if i > 0 {
			b.WriteString(itemSep)
		}
		if spell.IsIdentifier(n) {
			b.WriteString(doubleQuote)
			b.WriteString(n)
			b.WriteString(doubleQuote)
		} else {
			b.WriteString(python.Quote(n))
		}
	}
	b.WriteString(groupClose + allClose + newline)
	return b.String()
}

// publicNames returns the public names of the file where one starts with an
// underscore, and nil otherwise.
func (w *Writer) publicNames() []string {
	entries := w.set.Entries()
	hidden := slices.ContainsFunc(entries, func(e render.Entry) bool {
		return e.Path == publicPath && strings.HasPrefix(e.Name, discardName)
	})
	if !hidden {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.Path == publicPath {
			names = append(names, e.Name)
		}
	}
	slices.SortFunc(names, func(a, b string) int { return compareMembers(member{name: a}, member{name: b}) })
	return names
}

// declare starts the declaration d of the file with the header h, which
// every kind writer does before it writes: it ends the scope of the
// declaration before d, binds the imports of h, as [Writer.Bind] binds
// them, and records the name of d where d is a public declaration of the
// module. The head of the file lists the recorded names in __all__ where
// one public name starts with an underscore. A special name, which starts
// and ends with two underscores, is no name of __all__, and neither is the
// name of a nested type, which the module does not declare.
//
// Error modes: the errors of [Writer.Bind].
func (w *Writer) declare(h *emit.File, d symbol.Symbol) error {
	w.scope = w.scope[:0]
	if _, err := w.Bind(h); err != nil {
		return err
	}
	name := emit.DeclaredName(d)
	special := len(name) > 2*len(dunder) && strings.HasPrefix(name, dunder) && strings.HasSuffix(name, dunder)
	v := visibilityOf(d)
	if !special && w.set.Reserved(name) && (v == symbol.VisibilityUnknown || v == symbol.VisibilityPublic) {
		w.set.AddNamed(publicPath, name)
	}
	return nil
}

// visibilityOf returns the visibility of a declaration of a module.
func visibilityOf(d symbol.Symbol) symbol.Visibility {
	switch t := d.(type) {
	case *emit.Struct:
		return t.Visibility
	case *emit.Interface:
		return t.Visibility
	case *emit.Enum:
		return t.Visibility
	case *emit.Function:
		return t.Visibility
	case *emit.Alias:
		return t.Visibility
	case *emit.Constant:
		return t.Visibility
	case *emit.Variable:
		return t.Visibility
	default:
		return symbol.VisibilityUnknown
	}
}
