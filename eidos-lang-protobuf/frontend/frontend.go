// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"slices"
	"strings"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language in the [symbol.Identity] of every
// declaration this frontend loads. It is [protobuf.Lang], restated
// here so a caller with the frontend need not import the satellite
// root.
const Lang = protobuf.Lang

// UnparsedFile reports a syntax error at the token that caused it.
//
// Severity is Error, so a run with one fails. The file still loads
// every declaration the parser recovered, because one bad token in
// one message must not erase a schema. The first ten errors of a
// file report one finding each, and one more finding counts the
// rest.
var UnparsedFile = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a proto file has a syntax error",
})

// RefusedExtension reports an extend block, positioned at the
// block.
//
// Severity is Error. An extension adds a field to a message another
// file declares, which the model has no shape for, so the block
// contributes nothing and the rest of the file loads. The finding
// names the extended message, whether or not this workspace
// declares it.
var RefusedExtension = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a proto file extends a message it does not declare",
})

// UnknownEdition reports a syntax or an edition that the frontend's
// table of versions does not contain, such as the edition 2025, which
// protobuf never released, or UNSTABLE, which protoc reserves for
// features in development. The finding is at the statement. The file
// does not load any declaration, because the defaults that define
// presence and the openness of enums are unknown.
//
// Severity is Error. The satellite does not reuse the number 5.
var UnknownEdition = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  6,
	Meaning: "a proto file has a syntax or an edition that the frontend does not know",
})

// BadCarrier reports a directive carrier the kernel grammar
// refused, positioned at the carrier's own line.
//
// Severity is Error. The carrier attaches nothing, and the
// declaration below it still loads. The finding states the grammar's
// reason and the line as written.
var BadCarrier = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a well-formed directive carrier whose
// subject the model cannot address, positioned at the carrier.
//
// Severity is Error. The carrier attaches nothing, and the finding
// names it, so an authored directive is never dropped without a
// report. protobuf raises it for a carrier in a comment protoc
// attributes to no declaration, such as one a blank line separates
// from the declaration below it, and for a carrier on an import,
// which has no identity.
var UnaddressedCarrier = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// New returns the protobuf frontend, the value a composition
// registers among its frontends.
//
// The returned value is stateless and takes no configuration:
// protobuf has no build-constraint equivalent, so one value serves
// every load and may be shared across compositions. The load calls
// Parse once per unit and does so concurrently, and each call writes
// only through the unit it is handed.
//
// The frontend does not return an error for a problem of a schema. It
// reports a syntax error under [UnparsedFile], an extension under
// [RefusedExtension] and a version that it does not know under
// [UnknownEdition], and the load continues. A returned error means that
// the load itself failed, for example because the tree does not contain
// the file of a unit. Such an error ends the load for every frontend.
//
// New allocates nothing: the frontend has no state.
func New() plugin.Frontend { return protoFrontend{} }

// protoFrontend loads proto schemas. It has no state: units parse in
// parallel, and each unit's state is on the unit. Each method states
// what it allocates.
type protoFrontend struct{}

// A proto import refers to a file, so the frontend implements the
// importer role for the resolution step. The frontend registers its
// language's keys through its role.
var (
	_ plugin.Importer    = protoFrontend{}
	_ plugin.KeyProvider = protoFrontend{}
)

// Name returns [protobuf.Name], the origin of this frontend's
// findings and classification stamps. It allocates nothing.
func (protoFrontend) Name() plugin.ID { return protobuf.Name }

// Lang returns [protobuf.Lang], the language in the identity of
// every declaration this frontend loads. It allocates nothing.
func (protoFrontend) Lang() symbol.Lang { return protobuf.Lang }

// Syntax returns protobuf's comment forms, which the kit strips
// documentation with and the directive grammar reads carriers from.
// They are C's: a line comment, a block comment with its gutter,
// and the directive convention. Syntax allocates what
// [protobuf.Syntax] allocates, two allocations.
func (protoFrontend) Syntax() plugin.CommentSyntax { return protobuf.Syntax() }

// Overloads reports false: a service declares each RPC once by name,
// so every method this frontend loads takes the empty discriminator.
// It allocates nothing.
func (protoFrontend) Overloads() bool { return false }

// Version returns [protobuf.Version], which every unit key folds.
// Bumping it invalidates every unit this frontend loaded before, so
// it changes with what the frontend produces and not with the
// module's release. It allocates nothing.
func (protoFrontend) Version() string { return protobuf.Version }

// Keys registers every protobuf key through the handle that the
// workspace binds to the language's spelling. It returns the error of
// [protobuf.Keys], and allocates what protobuf.Keys allocates.
func (protoFrontend) Keys(r *meta.Registry) error { return protobuf.Keys(r) }

// Selection claims every file under the workspace whose name ends in
// [protobuf.Extension].
//
// Nothing is negated. protobuf has no test-file or generated-file
// convention a claim could exclude by, so the composition decides
// through scopes which directories take part. The load drops a
// claimed file with the workspace's own provenance trailer after
// selection and before partitioning. Selection allocates the list of
// patterns, one allocation, which the caller keeps.
func (protoFrontend) Selection() []string { return []string{"**/*" + protobuf.Extension} }

// Partition returns one unit per file, which is protobuf's own
// compilation grain: a schema imports what it needs by path and
// compiles alone, so nothing outside a file decides its shape.
//
// Units are returned in path order, so two runs over one tree
// partition identically. No unit declares a shared input, because no
// file outside the unit contributes to it, and the reader is never
// consulted, so partitioning records no read and folds nothing extra
// into a unit key.
//
// It returns no error: the grain needs no bytes to settle.
//
// # Allocation contract
//
// Partition allocates the sorted copy of the files and the list of
// units, two allocations for any number of files: each unit is a
// one-element window on the sorted copy.
func (protoFrontend) Partition(
	_ context.Context, files []plugin.SourceRef, _ plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	ordered := slices.Clone(files)
	slices.SortFunc(ordered, func(a, b plugin.SourceRef) int { return strings.Compare(a.Path, b.Path) })
	units := make([][]plugin.SourceRef, len(ordered))
	for i := range ordered {
		units[i] = ordered[i : i+1 : i+1]
	}
	return units, nil
}
