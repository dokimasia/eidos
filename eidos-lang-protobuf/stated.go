// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import (
	"strconv"
	"strings"

	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of the stamps that [Stated] and [Header] split.
const (
	// listSep joins the entries of the reserved stamp and the ranges of an
	// extension range declaration.
	listSep = ", "
	// optionAssign joins the name of a stamped option to its value.
	optionAssign = "="
	// bracketOpen opens the options of an extension range declaration, and
	// optionsOpen separates them from its ranges in the extensions stamp.
	bracketOpen = "["
	optionsOpen = " " + bracketOpen
	// requiredLabel is the value of the label stamp of a required field.
	requiredLabel = "required"
	// jsonNameOption is the field option that renames a field in JSON.
	jsonNameOption = "json_name"
)

// Statement is the emit statement of the residue of one declaration.
type Statement struct {
	// Annotations are the markers and the options, in the order that
	// [Stated] states.
	Annotations symbol.Annotations
	// Number is the tag entry of the wire number of a field, and nil for
	// every other declaration.
	Number *emit.TagEntry
}

// Match is a match that reads the declarations of the graph through the
// tracked reader of its invocation. Every match of the kernel is one, such
// as [eidos.StructMatch] and [eidos.GraphMatch].
type Match interface {
	eidos.Matcher
	Reader() *store.Reader
}

// Stated returns the emit statement of the residue that the protobuf
// frontend stamped on the declaration id. A generator over protobuf
// sources appends it to the emit declaration that it writes for id, so
// the protobuf backend writes the declaration as its source states it.
//
// The annotations are in this order: the required, the delimited, the
// closed and the local markers, the reserved marker with every reserved
// entry, one extensions marker for each extension range declaration, the
// json_name annotation with its value quoted, the options and then the
// features as written. A oneof, a map field and a stream are stamped too,
// and the shapes of the model state them, so Stated restates none of
// them. A declaration that the frontend did not stamp has the zero
// statement.
//
// Each read records on the invocation of m.
//
// # Allocation contract
//
// Stated allocates the list of annotations, the arguments of each
// annotation and the tag entry of a number. The read of a list stamp, such
// as the extension ranges and the options, allocates the copy of the stamp
// that the read returns and its box. The value of a read string stamp is
// shared.
func Stated(m eidos.Matcher, id symbol.Identity) Statement {
	var s Statement
	if n, stated := eidos.FactOf(m, id, meta.Named[string](FieldKey)); stated {
		s.Number = &emit.TagEntry{Key: NumberTag, Value: n}
	}
	if label, stated := eidos.FactOf(m, id, meta.Named[string](LabelKey)); stated && label == requiredLabel {
		s.Annotations = append(s.Annotations, symbol.Annotation{Name: string(RequiredMarker)})
	}
	for _, mark := range [...]struct {
		key    meta.KeyName
		marker Marker
	}{
		{key: DelimitedKey, marker: DelimitedMarker},
		{key: ClosedKey, marker: ClosedMarker},
		{key: LocalKey, marker: LocalMarker},
		{key: BodyKey, marker: BodyMarker},
	} {
		if _, stated := eidos.FactOf(m, id, meta.Named[string](mark.key)); stated {
			s.Annotations = append(s.Annotations, symbol.Annotation{Name: string(mark.marker)})
		}
	}
	if reserved, stated := eidos.FactOf(m, id, meta.Named[string](ReservedKey)); stated {
		s.Annotations = append(s.Annotations, symbol.Annotation{
			Name: string(ReservedMarker), Args: strings.Split(reserved, listSep),
		})
	}
	extensions, _ := eidos.FactOf(m, id, meta.Named[[]string](ExtensionsKey))
	for _, entry := range extensions {
		ranges, options, bracketed := strings.Cut(entry, optionsOpen)
		args := strings.Split(ranges, listSep)
		if bracketed {
			args = append(args, bracketOpen+options)
		}
		s.Annotations = append(s.Annotations, symbol.Annotation{Name: string(ExtensionsMarker), Args: args})
	}
	if name, stated := eidos.FactOf(m, id, meta.Named[string](JSONNameKey)); stated {
		s.Annotations = append(s.Annotations, symbol.Annotation{
			Name: jsonNameOption, Args: []string{strconv.Quote(name)},
		})
	}
	s.Annotations = appendOptions(m, id, s.Annotations)
	if r, reads := m.(Match); reads {
		if after, nested := follows(r, id); nested {
			s.Annotations = append(s.Annotations, symbol.Annotation{Name: string(FollowsMarker), Args: []string{after}})
		}
	}
	return s
}

// follows returns the name of the member of its message that the source
// declares right before the nested declaration id, a message, an enum, a
// oneof or an extension: a field, a oneof, a nested message or enum, or an
// extension. It returns the empty name for a declaration before every
// member. It returns false for a declaration at file level, for a field of
// its message, for a declaration without a position, and for a message
// that the reader does not hold.
func follows(m Match, id symbol.Identity) (string, bool) {
	if id.Owner == "" {
		return "", false
	}
	owner := symbol.Identity{Lang: id.Lang, Package: id.Package, Name: id.Owner, Kind: symbol.KindStruct}
	if at := strings.LastIndex(id.Owner, symbol.OwnerSep); at >= 0 {
		owner.Owner, owner.Name = id.Owner[:at], id.Owner[at+1:]
	}
	held, found := m.Reader().Lookup(owner)
	s, message := held.(*node.Struct)
	if !found || !message {
		return "", false
	}
	var (
		self    position.Pos
		nested  bool
		after   string
		closest position.Pos
	)
	for _, t := range s.Types {
		if d, named := t.(node.Declaration); named && d.Identity() == id {
			self, nested = d.Position(), true
		}
	}
	if !nested || self.Line == 0 {
		return "", false
	}
	consider := func(name string, at position.Pos) {
		if before(at, self) && (closest.Line == 0 || before(closest, at)) {
			after, closest = name, at
		}
	}
	for _, f := range s.Fields {
		consider(f.Name, f.Pos)
	}
	for _, t := range s.Types {
		switch n := t.(type) {
		case *node.Struct:
			consider(n.Name, n.Pos)
		case *node.Enum:
			consider(n.Name, n.Pos)
		case *node.Sum:
			consider(n.Name, n.Pos)
		case *node.Field:
			consider(n.Name, n.Pos)
		}
	}
	return after, true
}

// before reports whether a starts before b in one file.
func before(a, b position.Pos) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Col < b.Col)
}

// Header returns the header of the file that declares d. The header has
// the documentation of the syntax statement of the file and then the
// documentation of its package statement. Its annotations are the syntax
// marker, the package marker and one import marker for each import in
// source order, and then the options and the features of the file as
// written. A file without a syntax statement is proto2, so its header has
// the syntax marker of proto2. A file without a package statement has no
// package marker.
//
// A generator appends the header to the accumulator of the file that it
// writes from the source, as in e.File().Append(protobuf.Header(m, d)),
// and the layout merges the headers that two handlers state alike.
//
// Each read records on the invocation of m.
//
// # Allocation contract
//
// Header allocates the header, its joined documentation, its list of
// annotations and the arguments of each annotation. The read of a list
// stamp, such as the imports and the options, allocates the copy of the
// stamp that the read returns and its box. The value of a read string stamp
// is shared.
func Header(m Match, d node.Declaration) *emit.File {
	file := symbol.Identity{Lang: Lang, Package: d.Identity().Package, Name: d.Position().File, Kind: symbol.KindFile}
	h := &emit.File{}
	r := m.Reader()
	if f, held := r.Lookup(file); held {
		h.Doc = append(h.Doc, f.Docs()...)
	}
	if p, held := r.Lookup(file.PackageIdentity()); held {
		h.Doc = append(h.Doc, p.Docs()...)
	}
	syntax, stated := eidos.FactOf(m, file, meta.Named[string](SyntaxKey))
	if !stated {
		syntax = Proto2
	}
	h.Annotations = append(h.Annotations, symbol.Annotation{Name: string(SyntaxMarker), Args: []string{syntax}})
	if pkg, stated := eidos.FactOf(m, file, meta.Named[string](PackageKey)); stated {
		h.Annotations = append(h.Annotations, symbol.Annotation{Name: string(PackageMarker), Args: []string{pkg}})
	}
	imports, _ := eidos.FactOf(m, file, meta.Named[[]string](ImportKey))
	for _, imported := range imports {
		h.Annotations = append(h.Annotations, symbol.Annotation{Name: string(ImportMarker), Args: []string{imported}})
	}
	h.Annotations = appendOptions(m, file, h.Annotations)
	return h
}

// appendOptions appends to dst one annotation for each option and then for
// each feature that the frontend stamped on id, in the order of the
// stamps. The frontend stamps an option as its name and its value as
// written, joined by an equals sign. The name of an option has no equals
// sign, so the first one ends the name.
func appendOptions(m eidos.Matcher, id symbol.Identity, dst symbol.Annotations) symbol.Annotations {
	for _, key := range [...]meta.KeyName{OptionsKey, FeaturesKey} {
		entries, _ := eidos.FactOf(m, id, meta.Named[[]string](key))
		for _, entry := range entries {
			name, value, _ := strings.Cut(entry, optionAssign)
			dst = append(dst, symbol.Annotation{Name: name, Args: []string{value}})
		}
	}
	return dst
}
