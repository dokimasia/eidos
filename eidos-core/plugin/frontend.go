// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"context"

	"go.dokimi.dev/eidos/core/symbol"
)

// Depth says how deep one unit loads.
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
// directly; either way the conformance suite holds both to the
// same checks.
type Frontend interface {
	// Name is the frontend's one identity: what its findings
	// report under and what its classification stamps carry as
	// origin.
	Name() ID

	// Lang is the source language every declaration this frontend
	// loads carries.
	Lang() symbol.Lang

	// Syntax is the language's comment forms, shared with the
	// render side's output contract: the frontend strips comments
	// with it.
	Syntax() CommentSyntax

	// Selection is the file claim: gitignore-style globs against
	// workspace-relative paths, negations included, because a
	// testdata tree is not source by the language's own
	// definition and that belongs in the claim. What stays out of
	// it is policy — whether test files take part is the
	// consumer's call through scopes, never a selection line.
	Selection() []string

	// Partition groups the selected files into units, the
	// language's own grain: package directories for Go, whatever
	// the language's own compilation unit is elsewhere. Every
	// selected file appears as a member of exactly one unit, and a
	// unit's refs carry the shared inputs the frontend declares for
	// them — a module file, a config chain. The reader is recorded
	// rather than jailed, because a grain can live inside bytes the
	// selection must not claim — a package clause sits in a
	// selected file, a module boundary in go.mod — and a partition
	// that cannot look would guess; every partition read folds into
	// every resulting unit's fingerprint, because the partition
	// decided their shape. A returned error is fatal to the load:
	// unit shape is structural, and a frontend that cannot say what
	// its units are has nothing to parse.
	Partition(ctx context.Context, files []SourceRef, r FileReader) ([][]SourceRef, error)

	// Parse loads one unit through its handle. A unit's problem
	// reports through the handle and parsing continues; a
	// returned error is fatal to the whole load, every
	// frontend's, because the resolution phase runs over the
	// union of every frontend's graph and a partial union
	// resolves wrong. The context carries cancellation into a
	// long parse.
	Parse(ctx context.Context, u *SourceUnit) error

	// Resolve says what a spelling could mean in one file's
	// recorded import scope: the candidate identities in the
	// language's own probe order. The resolution phase keeps the
	// first candidate the graph holds, reports several present
	// candidates as an ambiguity, and none leaves the reference
	// spelling only, which is degradation a reader can ask about
	// rather than failure.
	Resolve(scope ImportScope, spelling string) []symbol.Identity
}

// SourceRef names a file without opening it: the
// workspace-relative path, and the shared inputs whose bytes fold
// into any dependent unit's fingerprint.
type SourceRef struct {
	// Path is the file's workspace-relative slash path.
	Path string

	// Shared lists the workspace-relative paths of the declared
	// inputs that apply to this file — a Go module file, a
	// TypeScript config chain, a proto root mapping. The shape is
	// the list; the members are each language's own.
	Shared []string
}

// FileReader is the partition's recorded door: reads over the
// workspace tree, before units exist. It is not jailed to the
// selection, because a unit's shape can depend on a file the
// selection must not claim — a Go module file, a TypeScript config
// — and a partition that cannot look would guess; hermeticity holds
// because every read folds into every resulting unit's fingerprint
// instead. It is not the graph's
// [go.dokimi.dev/eidos/core/store.Reader], and the two never meet.
type FileReader interface {
	// Read returns one file's bytes from the workspace tree, and
	// records the read into the fingerprints of the units the
	// partition produces.
	Read(path string) ([]byte, error)
}

// ImportScope is what the resolution phase hands a language's
// Resolve for one file: the file's assigned identity, and the
// bindings the frontend recorded at parse time through
// [GraphBuilder.Scope], in the language's own form. The kernel
// stores the bindings and hands them back to that language's
// Resolve alone, which type-asserts its own shape: Go binds
// package aliases, TypeScript binds members with rename and form,
// proto scopes per declaration site, and a kernel that fixed one
// shape would fix one language's.
type ImportScope struct {
	// File is the identity of the file the scope belongs to.
	File symbol.Identity

	// Bindings is the language's own record, opaque to the
	// kernel. A binding-shape mistake reports at resolution
	// rather than compile, which the conformance suite's linked
	// fixture exercises per language.
	Bindings any
}
