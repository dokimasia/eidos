// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language in the identity of every declaration that
// this frontend loads. It restates [protobuf.Lang], so a caller with the
// frontend does not need to import the satellite root.
const Lang = protobuf.Lang

// UnparsedFile reports a syntax error at the token that caused it.
//
// Severity is Error, so a run with one fails. The file still loads every
// declaration that the parser recovered, because one bad token in one
// message must not erase a schema. The frontend reports the first
// [plugin.MaxSyntaxFindings] errors of a file one finding each, and one
// more finding counts the rest.
var UnparsedFile = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a proto file has a syntax error",
})

// UnknownEdition reports a syntax or an edition that the table of versions
// of the frontend does not contain, such as the edition 2025, which
// protobuf never released, or UNSTABLE, which protoc reserves for features
// in development. The finding is at the statement. The file does not load
// any declaration, because the defaults that define presence and the
// openness of enums are unknown.
//
// Severity is Error. The satellite does not reuse the numbers 2 and 5.
var UnknownEdition = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  6,
	Meaning: "a proto file has a syntax or an edition that the frontend does not know",
})

// BadCarrier reports a directive carrier that the kernel grammar refused,
// at the line of the carrier.
//
// Severity is Error. The carrier attaches nothing, and the declaration
// below it still loads. The finding states the reason of the grammar and
// the line as written.
var BadCarrier = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a well-formed directive carrier whose subject
// the model cannot address, at the carrier.
//
// Severity is Error. The carrier attaches nothing, and the finding quotes
// it, so an authored directive is never dropped without a report.
// protobuf raises it for a carrier in a comment that protoc attributes to
// no declaration, such as a comment that a blank line separates from the
// declaration below it, and for a carrier on an import, which has no
// identity.
var UnaddressedCarrier = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// New builds the protobuf frontend through the kit, the value that a
// composition registers among its frontends. The frontend claims every
// file whose name ends in [protobuf.Extension], partitions one unit per
// file, and folds [protobuf.Version] into every unit key. A service
// declares each RPC once by name, so the language does not overload. A
// proto import refers to a file, so the frontend has the importer role. The
// frontend registers the protobuf keys through its role.
//
// The frontend takes no configuration: protobuf has no equivalent of a
// build constraint, so one value serves every load and compositions can
// share it. The load calls Parse once per unit and concurrently, and each
// call writes only through its own unit.
//
// The frontend does not return an error for a problem of a schema. It
// reports a syntax error under [UnparsedFile] and a version that it does
// not know under [UnknownEdition], and the load continues. A returned error
// means that the load itself failed, for example because the tree does not
// contain the file of a unit. Such an error ends the load for every
// frontend.
//
// # Allocation contract
//
// New allocates two allocations of the syntax and four of the kit with its
// list of key registrations: 6 allocations.
func New() plugin.Frontend {
	return frontend.New(protobuf.Name, Lang, protobuf.Syntax()).
		Version(protobuf.Version).
		Match("**/*" + protobuf.Extension).
		Units(partition).
		Parse(parse).
		Resolve(Resolve).
		Imports(ImportPath).
		Keys(protobuf.Keys).
		Build()
}

// partition returns one unit per file, which is the compilation grain of
// protobuf: a schema imports what it needs by path and compiles alone, so
// nothing outside a file decides its shape.
//
// The units are in path order, so two runs over one tree partition alike.
// No unit declares a shared input, because no file outside the unit
// contributes to it. The partition never consults the reader, so it
// records no read and folds nothing more into a unit key. It returns no
// error, because the grain needs no bytes to settle.
//
// # Allocation contract
//
// partition allocates the sorted copy of the files and the list of units,
// two allocations for any number of files. Each unit is a one-element
// window on the sorted copy.
func partition(_ context.Context, files []plugin.SourceRef, _ plugin.FileReader) ([][]plugin.SourceRef, error) {
	ordered := slices.Clone(files)
	slices.SortFunc(ordered, func(a, b plugin.SourceRef) int { return strings.Compare(a.Path, b.Path) })
	units := make([][]plugin.SourceRef, len(ordered))
	for i := range ordered {
		units[i] = ordered[i : i+1 : i+1]
	}
	return units, nil
}
