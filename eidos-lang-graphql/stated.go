// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"path"
	"slices"
	"strings"

	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// packageSep separates the segments of a package path, which is
// slash-separated.
const packageSep = "/"

// rootMarkers are the header markers of the root operation types by the
// kind of their operations.
var rootMarkers = map[Operation]Marker{
	OperationQuery:        QueryMarker,
	OperationMutation:     MutationMarker,
	OperationSubscription: SubscriptionMarker,
}

// Statement is the emit statement of the stamps of one declaration.
type Statement struct {
	// Annotations are the markers, in the order that [Stated] states.
	Annotations symbol.Annotations
	// Body is the selection set of an operation or a fragment as its
	// verbatim body, and the zero Body for every other declaration.
	Body emit.Body
}

// Match is a match that reads the declarations of the graph through the
// tracked reader of its invocation. Every match of the kernel is one, such
// as [eidos.StructMatch] and [eidos.GraphMatch].
type Match interface {
	eidos.Matcher
	Reader() *store.Reader
}

// Stated returns the emit statement of the stamps that the GraphQL frontend
// stamped on the declaration id. A generator over GraphQL sources appends the
// markers to the annotations of the emit declaration that it writes for id,
// and sets the body of its function to the body of the statement. The
// GraphQL backend then writes the declaration as its source states it.
//
// The markers are in this order: the input marker of an input object, the
// extend marker of an extension of a type outside its schema, the operation
// marker with the kind of the operation, the anonymous marker, the fragment
// marker with the type condition, the directive marker with the locations
// of a directive definition, and the repeatable marker. The frontend stamps
// the kind of an operation on a root operation type and on the function of
// an operation. The body of an operation and of a fragment is its selection
// set as written. A declaration that the frontend did not stamp has the
// zero statement.
//
// Each read records on the invocation of m.
//
// # Allocation contract
//
// Stated allocates the list of markers, and the list of arguments of an
// operation marker and of a fragment marker. The arguments of a directive
// marker are the copy of the stamp of the locations that the read returns,
// which allocates the copy and its box. A declaration without stamps
// allocates nothing. The text of the body is the value of its stamp, which
// Stated shares.
func Stated(m eidos.Matcher, id symbol.Identity) Statement {
	var buf [7]symbol.Annotation
	marks := buf[:0]
	if _, stated := eidos.FactOf(m, id, meta.Named[string](InputKey)); stated {
		marks = append(marks, symbol.Annotation{Name: string(InputMarker)})
	}
	if _, stated := eidos.FactOf(m, id, meta.Named[string](ExtensionKey)); stated {
		marks = append(marks, symbol.Annotation{Name: string(ExtendMarker)})
	}
	if kind, stated := eidos.FactOf(m, id, meta.Named[string](OperationKey)); stated {
		marks = append(marks, symbol.Annotation{Name: string(OperationMarker), Args: []string{kind}})
	}
	if _, stated := eidos.FactOf(m, id, meta.Named[string](AnonymousKey)); stated {
		marks = append(marks, symbol.Annotation{Name: string(AnonymousMarker)})
	}
	if condition, stated := eidos.FactOf(m, id, meta.Named[string](FragmentKey)); stated {
		marks = append(marks, symbol.Annotation{Name: string(FragmentMarker), Args: []string{condition}})
	}
	if locations, stated := eidos.FactOf(m, id, meta.Named[[]string](DirectiveKey)); stated {
		marks = append(marks, symbol.Annotation{Name: string(DirectiveMarker), Args: locations})
	}
	if _, stated := eidos.FactOf(m, id, meta.Named[string](RepeatableKey)); stated {
		marks = append(marks, symbol.Annotation{Name: string(RepeatableMarker)})
	}
	var s Statement
	if len(marks) > 0 {
		s.Annotations = slices.Clone(marks)
	}
	s.Body.Verbatim, _ = eidos.FactOf(m, id, meta.Named[string](SelectionKey))
	return s
}

// Header returns the header of the file that declares d, and nil for a file
// without a schema statement. d is a declaration or a file. The documentation
// of the header is the description of the schema definition of the file. Its
// annotations are the directives of the schema statements of the file, a
// root marker for each root operation type that the statements name, and the
// extend marker for a file with extensions of the schema alone.
//
// The frontend loads the executable definitions of a file into the package of
// the documents of its schema. For a declaration of that package, Header
// returns the header of the file in the package of the schema, so an
// operation and a fragment have the header of their file.
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
// annotations and the list of arguments of each root marker. The read of the
// roots allocates the copy of the stamp that the read returns and its box. A
// file without a schema statement allocates nothing. Each directive shares
// its list of arguments with the annotation of the file.
func Header(m Match, d node.Declaration) *emit.File {
	pkg := d.Identity().Package
	file := symbol.Identity{Lang: Lang, Package: pkg, Name: d.Position().File, Kind: symbol.KindFile}
	statement, stated := eidos.FactOf(m, file, meta.Named[string](SchemaKey))
	if !stated && path.Base(pkg) == DocumentsSegment {
		file.Package = strings.TrimSuffix(strings.TrimSuffix(pkg, DocumentsSegment), packageSep)
		statement, stated = eidos.FactOf(m, file, meta.Named[string](SchemaKey))
	}
	if !stated {
		return nil
	}
	roots, _ := eidos.FactOf(m, file, meta.Named[[]string](RootsKey))
	h := &emit.File{}
	var directives symbol.Annotations
	if found, held := m.Reader().Lookup(file); held {
		if f, is := found.(*node.File); is {
			h.Doc = slices.Clone(f.Doc)
			directives = f.Annotations
		}
	}
	h.Annotations = make(symbol.Annotations, 0, len(directives)+len(roots)+1)
	h.Annotations = append(h.Annotations, directives...)
	for _, entry := range roots {
		op, root, _ := strings.Cut(entry, NameSep)
		if marker, valid := rootMarkers[Operation(op)]; valid {
			h.Annotations = append(h.Annotations, symbol.Annotation{Name: string(marker), Args: []string{root}})
		}
	}
	if statement != SchemaStatement {
		h.Annotations = append(h.Annotations, symbol.Annotation{Name: string(ExtendMarker)})
	}
	return h
}
