// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"github.com/bufbuild/protocompile/experimental/ast/printer"
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/lang/protobuf/internal/parse"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the declarations that two or more specs lower and render.
const (
	rowName   = "row"
	phaseName = "phase"
	storeName = "store"
	otherName = "Other"
)

// The names of the members that two or more specs declare.
const (
	nameField = "name"
	idField   = "id"
	extName   = "ext"
)

// The spellings of the types that two or more specs give their fields.
const (
	int32Type         = "int32"
	stringType        = protobuf.TypeString
	optionalInt32Type = "optional int32"
)

// syntaxName is the name of the header marker of the version of a file.
const syntaxName = string(protobuf.SyntaxMarker)

// writtenPath is the path under which the oracle of the layout parses a
// written file.
const writtenPath = "written.proto"

// The tags of the wire numbers that the specs give their fields. The
// lowering and the render read a tag and never write one, so the fields of
// parallel cases share them.
var (
	tag1   = emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "1"})
	tag2   = emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "2"})
	tag3   = emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "3"})
	tag100 = emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "100"})
)

// The types that the specs of the vocabulary give their fields. The
// vocabulary reads a type and never writes one, so parallel cases share
// them. The settle rewrites the spelling of each type that it settles, so a
// spec that settles builds its own types.
var (
	int32Ref      = &emit.TypeRef{Spelling: int32Type}
	stringRef     = &emit.TypeRef{Spelling: stringType}
	optionalInt32 = &emit.TypeRef{
		Spelling: optionalInt32Type,
		Form:     symbol.FormOptional,
		Elems:    []*emit.TypeRef{int32Ref},
	}
	repeatedInt32 = &emit.TypeRef{Spelling: "repeated int32", Form: symbol.FormList, Elems: []*emit.TypeRef{int32Ref}}
)

// The headers of a file of each version. The specs read a header and never
// write one, so parallel cases share them.
var (
	proto2File      = &emit.File{Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{protobuf.Proto2}}}}
	proto3File      = &emit.File{Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{protobuf.Proto3}}}}
	edition2023File = &emit.File{Annotations: symbol.Annotations{
		{Name: syntaxName, Args: []string{protobuf.Edition2023}},
	}}
	edition2024File = &emit.File{Annotations: symbol.Annotations{
		{Name: syntaxName, Args: []string{protobuf.Edition2024}},
	}}
	edition2026File = &emit.File{Annotations: symbol.Annotations{
		{Name: syntaxName, Args: []string{protobuf.Edition2026}},
	}}
)

// fileScope returns the scope of the top level of the file of the header
// h, and fails the test where the header states no version.
func fileScope(tb assert.TB, h *emit.File) backend.Scope {
	tb.Helper()

	s, err := backend.FileScope(h)
	assert.NoError(tb, err, "the header states a version")
	return s
}

// formatted returns the written file text in the layout that buf format
// gives it, through the printer of protocompile under its preset of buf
// format. A spec compares a written file with it, which proves that the
// backend writes the layout of buf format. A file of edition 2026 prints from
// a copy that states 2024, and formatted writes 2026 back into its edition
// statement.
func formatted(tb assert.TB, text string) string {
	tb.Helper()

	tree, r, patched := parse.File(writtenPath, text)
	assert.Empty(tb, r.Diagnostics, "the written file parses")
	out, err := printer.PrintFile(printer.Options{Format: true, Formatting: printer.Legacy()}, tree)
	assert.NoError(tb, err, "the printer prints the written file")
	if patched {
		again, _, _ := parse.File(writtenPath, out)
		content := again.Syntax().Value().AsLiteral().AsString().RawContent()
		out = out[:content.Start] + protobuf.Edition2026 + out[content.End:]
	}
	return out
}
