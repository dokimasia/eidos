// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	rust "go.dokimi.dev/eidos/lang/rust"
	rustgrammar "go.dokimi.dev/eidos/lang/treesitter/rust"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration: the
// satellite root's constant, restated for the frontend's callers.
const Lang = rust.Lang

// targetDir is the directory Cargo builds into, which the claim leaves
// out.
const targetDir = "target"

// Keys registers every rust key: the satellite root's one registration,
// restated here, where the corpus and the suite fixtures use it.
var Keys = rust.Keys

// UnparsedFile reports a syntax error, positioned at it: the source's
// problem, and the load continues with every item the parser still
// recovered.
var UnparsedFile = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a Rust file has a syntax error",
})

// BadCarrier reports a +-prefixed comment line the kernel grammar
// refused: the carrier attaches nothing and the load continues.
var BadCarrier = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier on a subject the model
// cannot address, such as a parameter, a macro or a comment no item
// takes, so the author learns that the directive attached nowhere.
var UnaddressedCarrier = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// UnlinkedFile reports a member of a crate that no mod item names. It
// loads under the module path its place in the crate's directories
// spells.
var UnlinkedFile = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a file no mod item names loads under its layout path",
})

// BadManifest reports a Cargo.toml that does not read or parse. Its
// package states no target, so each of its files loads as a crate of
// its own.
var BadManifest = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  5,
	Meaning: "a Cargo.toml does not parse",
})

// UnmodeledItem reports an associated constant the model has no member
// list for: one of a data enum, and one of a type the crate does not
// declare.
var UnmodeledItem = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  6,
	Meaning: "an associated constant has no member list in the model",
})

// ExcludedFile reports a member of a crate that a cfg predicate outside
// the load's set keeps out, with its module: the file loads nothing.
var ExcludedFile = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  7,
	Meaning: "a cfg predicate keeps a file's module out of the load",
})

// Options is the frontend's declared configuration: the cfg predicates
// one load satisfies. A cfg set is in every unit key by the kit's
// contract, because it changes the graph without changing a read.
type Options struct {
	// Features are the Cargo features the load enables, each of which
	// satisfies feature = "name".
	Features []string

	// Cfg are the other cfg options the load sets, spelled as rustc's
	// --cfg takes them, such as unix or target_os="linux".
	Cfg []string
}

// rustFrontend is the load's configuration and the grammar's
// vocabulary, which every parse reads and none writes.
type rustFrontend struct {
	opts *Options
	v    *vocabulary
}

// New builds the Rust frontend through the kit. A nil options value
// loads with no feature enabled and no cfg option set. It implements
// the exporter role, so a reference through a pub use resolves to the
// declaration it publishes, and its version folds the grammar's.
//
// # Allocation contract
//
// New allocates the frontend's state with its vocabulary and its parse
// hook, the version, the syntax, two allocations, and the kit's three:
// nine allocations, and ten with the empty options that nil options
// take.
func New(opts *Options) plugin.Frontend {
	if opts == nil {
		opts = &Options{}
	}
	f := &rustFrontend{opts: opts, v: newVocabulary(rustgrammar.Grammar)}
	return frontend.New(rust.Name, Lang, rust.Syntax()).
		Version(rust.FrontendVersion+"; "+rustgrammar.Grammar.Version()).
		Match("**/*"+rustExtension, "!**/"+targetDir+"/**").
		Units(partition).
		Parse(f.parse).
		Resolve(resolve).
		Exports(exports).
		Options(opts).
		Build()
}
