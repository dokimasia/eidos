// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"context"

	"go.dokimi.dev/eidos/core/symbol"
)

// Depth is how deep one unit loads.
//
// Signature-only loading is the same Parse observing
// [DepthSignatures] and skipping bodies and unexported members:
// one code path, so a dependency package cannot drift from the
// in-scope parse.
type Depth uint8

const (
	// DepthFull loads everything the unit's source states.
	DepthFull Depth = iota
	// DepthSignatures loads the exported shape alone: no bodies,
	// no unexported members. The same bytes at the two depths
	// produce two graphs and key differently.
	DepthSignatures
)

// Frontend loads one language's source into the node graph. The
// kit lowers to this role and the exotic case implements it
// directly. The conformance suite runs the same checks over both.
type Frontend interface {
	// Name is the frontend's one identity: the origin its findings
	// report under and its classification stamps record.
	Name() ID

	// Lang is the source language of every declaration this
	// frontend loads.
	Lang() symbol.Lang

	// Syntax is the language's comment forms, shared with the
	// render side's output contract: the frontend strips comments
	// with it.
	Syntax() CommentSyntax

	// Selection is the file claim: gitignore-style globs against
	// workspace-relative paths, negations included, because a
	// testdata tree is not source by the language's own
	// definition and that belongs in the claim. Policy is left out
	// of it: whether test files take part is the consumer's call
	// through scopes, never a selection line.
	Selection() []string

	// Partition groups the selected files into units, the
	// language's own grain: package directories for Go, whatever
	// the language's own compilation unit is elsewhere. Every
	// selected file appears as a member of exactly one unit, and a
	// unit's refs list the shared inputs the frontend declares for
	// them, such as a module file or a config chain. The reader is
	// recorded and not jailed, because a grain can depend on bytes
	// the selection must not claim: a package clause is in a
	// selected file, a module boundary in go.mod. Every partition
	// read folds into every resulting unit's fingerprint, because
	// the partition decided their shape. A returned error is fatal
	// to the load: unit shape is structural, and a frontend that
	// cannot say what its units are has nothing to parse.
	Partition(ctx context.Context, files []SourceRef, r FileReader) ([][]SourceRef, error)

	// Parse loads one unit through its handle. A unit's problem
	// reports through the handle and parsing continues. A returned
	// error is fatal to the whole load, every frontend's, because
	// the resolution phase runs over the union of every frontend's
	// graph and a partial union resolves wrong. Units parse in
	// parallel, so Parse is called concurrently on one frontend:
	// per-unit state belongs on the unit, and a frontend keeping
	// its own is broken under any worker count. The context
	// cancels a long parse, and a frontend observing it returns
	// the error and no half-built unit, because a nil error reports
	// the unit whole.
	Parse(ctx context.Context, u *SourceUnit) error

	// Resolve returns what a spelling could mean in one file's
	// recorded import scope: the candidate identities in the
	// language's own probe order, grouped into shadowing tiers. The
	// resolution phase reads the first tier that names a
	// declaration the graph contains, takes that tier's first such
	// candidate as the target, and reports several such candidates
	// in the tier as an ambiguity. A reference no tier resolves
	// keeps its spelling alone: degradation a reader can ask about,
	// not a failure.
	Resolve(scope ImportScope, spelling string) Candidates
}

// Candidates is what one spelling could mean, in tiers: each tier is
// the candidate identities one scope offers, in probe order, and an
// earlier tier shadows every later one. A language whose scopes
// nest, such as protobuf declaring a message inside a message,
// returns one tier per scope, so an inner declaration takes the name
// from an outer one without an ambiguity. A language whose
// candidates compete, such as Go probing its own package and its dot
// imports, returns them in one tier, so two candidates the graph
// contains report as an ambiguity.
type Candidates [][]symbol.Identity

// SourceRef names a file without opening it: the
// workspace-relative path, and the shared inputs whose bytes fold
// into any dependent unit's fingerprint.
type SourceRef struct {
	// Path is the file's workspace-relative slash path.
	Path string

	// Shared lists the workspace-relative paths of the declared
	// inputs that apply to this file: a Go module file, a
	// TypeScript config chain, a proto root mapping. The shape is
	// the list, and the members are each language's own.
	Shared []string
}

// FileReader is the partition's recorded door: reads over the
// workspace tree, before units exist. It is not jailed to the
// selection, because a unit's shape can depend on a file the
// selection must not claim, such as a Go module file or a
// TypeScript config. The load is hermetic because every read folds
// into every resulting unit's fingerprint. It is not the graph's
// [go.dokimi.dev/eidos/core/store.Reader], and neither reads through
// the other.
type FileReader interface {
	// Read returns one file's bytes from the workspace tree, and
	// records the read into the fingerprints of the units the
	// partition produces.
	Read(path string) ([]byte, error)
}

// ImportScope is what the resolution phase hands a language's
// Resolve for one reference: the identity of the file it is written
// in, the declaration that encloses it, and the bindings the
// frontend recorded at parse time through [GraphBuilder.Scope], in
// the language's own form. The kernel stores the bindings and hands
// them back to that language's Resolve alone, which type-asserts
// its own shape: Go binds package aliases, TypeScript binds members
// with rename and form, proto scopes per declaration site, and a
// kernel that fixed one shape would fix one language's.
type ImportScope struct {
	// File is the identity of the file the scope belongs to.
	File symbol.Identity

	// Owner is the innermost enclosing declaration that can nest
	// types, a struct, an interface, an enum or a sum, and zero at
	// file level. A language whose scoping is lexical reads it: a
	// name written inside a message, a class or a module resolves
	// against that declaration's own members before it resolves
	// outward.
	Owner symbol.Identity

	// Bindings is the language's own record, opaque to the kernel.
	// A binding-shape mistake reports at resolution and not at
	// compile time, which the conformance suite's linked fixture
	// exercises per language.
	Bindings any
}
