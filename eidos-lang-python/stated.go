// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"slices"
	"strings"

	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Match is a match that reads the declarations of the graph through the
// tracked reader of its invocation. Every match of the kernel is one, such
// as [eidos.StructMatch] and [eidos.GraphMatch].
type Match interface {
	eidos.Matcher
	Reader() *store.Reader
}

// Stated returns the markers that restate the stamps of the Python frontend
// on the declaration id, in this order: the TypedDict marker of an
// interface, the bases of an enum, the keywords of a class statement, the
// metadata of an Annotated type and the InitVar marker of a field. A
// generator over Python sources appends them to the annotations of the
// emit declaration that it writes for id. The Python backend then writes
// the declaration as its source states it. A declaration that the frontend
// did not stamp has no markers, and Stated returns nil for it.
//
// Each read records on the invocation of m.
//
// # Allocation contract
//
// Stated allocates the list of markers, and the copy and the box of each
// list that a read returns. A declaration without stamps allocates nothing.
func Stated(m eidos.Matcher, id symbol.Identity) symbol.Annotations {
	var buf [5]symbol.Annotation
	marks := buf[:0]
	if typed, _ := eidos.FactOf(m, id, meta.Named[bool](TypedDictKey)); typed {
		marks = append(marks, symbol.Annotation{Name: string(TypedDictMarker)})
	}
	for _, list := range []struct {
		key    meta.KeyName
		marker Marker
	}{{BasesKey, BasesMarker}, {KeywordsKey, KeywordsMarker}, {AnnotatedKey, AnnotatedMarker}} {
		if args, stated := eidos.FactOf(m, id, meta.Named[[]string](list.key)); stated {
			marks = append(marks, symbol.Annotation{Name: string(list.marker), Args: args})
		}
	}
	if initVar, _ := eidos.FactOf(m, id, meta.Named[bool](InitVarKey)); initVar {
		marks = append(marks, symbol.Annotation{Name: string(InitVarMarker)})
	}
	if len(marks) == 0 {
		return nil
	}
	return slices.Clone(marks)
}

// The words of an import statement, which the arguments of [ImportMarker]
// spell and the backend writes: import a.b, import a.b as c,
// from a.b import c, from a.b import c as d, and from a.b import *.
const (
	ImportWord = "import"
	FromWord   = "from"
	AsWord     = "as"
	StarName   = "*"
)

// wordSep separates the words of an import statement.
const wordSep = " "

// Header returns the header of the file that declares d, and nil for a file
// without a docstring, an __all__, a future statement, a lazy import, a
// head comment and an import. d is a declaration or a file. The
// documentation of the header is the module docstring. Its annotations are
// the header markers of the other five, in this order: the names of
// __all__, the features of the future statements, the modules that the
// file imports lazily, the comments of the head, and the imports of the
// file. The import marker states one statement for each name that an
// import binds, in the order of the imports, with the absolute name of the
// module of a relative import. An __all__ without names states the all
// marker without arguments.
//
// A generator appends a header that is not nil to the accumulator of the file
// that it writes from the source, as in e.File().Append(h). The layout merges
// the headers that two handlers state alike.
//
// Each read records on the invocation of m.
//
// # Allocation contract
//
// Header allocates the header, a copy of its documentation, its list of
// annotations, the list of the names of __all__, and the list of the import
// statements with the text of each. Each read of a list allocates the copy
// and the box that it returns. A file without a header allocates the lists
// that its reads return alone.
func Header(m Match, d node.Declaration) *emit.File {
	file := symbol.Identity{Lang: Lang, Package: d.Identity().Package, Name: d.Position().File, Kind: symbol.KindFile}
	var (
		doc     []string
		all     []string
		imports []string
	)
	if found, held := m.Reader().Lookup(file); held {
		if f, is := found.(*node.File); is {
			doc = f.Doc
			for _, x := range f.Exports {
				if all == nil {
					all = make([]string, 0, len(x.Names))
				}
				for _, b := range x.Names {
					all = append(all, b.Name)
				}
			}
			imports = importStatements(f.Imports)
		}
	}
	var marks symbol.Annotations
	if all != nil {
		marks = append(marks, symbol.Annotation{Name: string(AllMarker), Args: all})
	}
	for _, list := range []struct {
		key    meta.KeyName
		marker Marker
	}{{FutureKey, FutureMarker}, {LazyKey, LazyMarker}, {HeadKey, HeadMarker}} {
		if args, stated := eidos.FactOf(m, file, meta.Named[[]string](list.key)); stated {
			marks = append(marks, symbol.Annotation{Name: string(list.marker), Args: args})
		}
	}
	if imports != nil {
		marks = append(marks, symbol.Annotation{Name: string(ImportMarker), Args: imports})
	}
	if len(doc) == 0 && len(marks) == 0 {
		return nil
	}
	return &emit.File{Doc: slices.Clone(doc), Annotations: marks}
}

// importStatements returns one import statement for each name that an
// import of imports binds, in order, and nil for no imports: import a.b or
// import a.b as c for a module, from a.b import c or from a.b import c as d
// for each name of a module, and from a.b import * for a star import.
func importStatements(imports []*node.Import) []string {
	var out []string
	for _, imp := range imports {
		module := strings.ReplaceAll(imp.Path, PackageSep, ModuleSep)
		from := FromWord + wordSep + module + wordSep + ImportWord + wordSep
		switch {
		case imp.Wildcard:
			out = append(out, from+StarName)
		case len(imp.Names) > 0:
			for _, b := range imp.Names {
				out = append(out, statement(from+b.Name, b.Alias))
			}
		default:
			out = append(out, statement(ImportWord+wordSep+module, imp.Alias))
		}
	}
	return out
}

// statement returns the import statement head, followed by the word as and
// alias where alias is not empty.
func statement(head, alias string) string {
	if alias == "" {
		return head
	}
	return head + wordSep + AsWord + wordSep + alias
}
