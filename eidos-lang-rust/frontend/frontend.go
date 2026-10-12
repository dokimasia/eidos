// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/rust"
	rustgrammar "go.dokimi.dev/eidos/lang/treesitter/rust"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration. It restates the
// constant of the satellite root for the callers of the frontend.
const Lang = rust.Lang

// targetDir is the directory into which Cargo builds, which the claim
// leaves out.
const targetDir = "target"

// UnparsedFile reports a syntax error at its position. The error is a
// problem of the source, and the load continues with every item that the
// parser recovered.
var UnparsedFile = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a Rust file has a syntax error",
})

// BadCarrier reports a comment line with the + prefix that the kernel
// grammar refused. The carrier attaches nothing, and the load continues.
var BadCarrier = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier or a marker on a subject
// that the model cannot address, such as a parameter, a macro or a comment
// that no item takes. The author learns that the directive attached
// nowhere.
var UnaddressedCarrier = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// UnlinkedFile reports a member of a crate that no mod item names. It
// loads under the module path of its place in the directories of the
// crate.
var UnlinkedFile = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a file no mod item names loads under its layout path",
})

// BadManifest reports a Cargo.toml that does not read or parse. Its
// package states no target, so each of its files loads as a crate of its
// own.
var BadManifest = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  5,
	Meaning: "a Cargo.toml does not parse",
})

// UnmodeledItem reports an associated constant for which the model has no
// member list: a constant of a data enum, and a constant of a type that
// the crate does not declare.
var UnmodeledItem = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  6,
	Meaning: "an associated constant has no member list in the model",
})

// ExcludedFile reports a member of a crate that a cfg predicate outside
// the set of the load keeps out with its module. The file loads nothing.
var ExcludedFile = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  7,
	Meaning: "a cfg predicate keeps a file's module out of the load",
})

// BadMarker reports an attribute of the brand that is not a valid
// directive. Either its path is not the brand followed by a name, or by a
// plugin and a name, or an argument is not a literal or a name. The
// attribute attaches nothing and remains an annotation.
var BadMarker = diag.MustRegister(rust.CodePrefix, diag.CodeSpec{
	Number:  8,
	Meaning: "an attribute of the brand is not a valid directive",
})

// Options is the declared configuration of the frontend: the cfg
// predicates that one load satisfies. The kit folds a cfg set into every
// unit key, because the set changes the graph without changing a read.
type Options struct {
	// Features are the Cargo features that the load enables, each of which
	// satisfies feature = "name".
	Features []string

	// Cfg are the other cfg options that the load sets, written as the
	// --cfg flag of rustc takes them, such as unix or target_os="linux".
	Cfg []string
}

// rustFrontend is the configuration of the load and the vocabulary of the
// grammar, which every parse reads and none writes.
type rustFrontend struct {
	opts *Options
	v    *vocabulary
}

// New builds the Rust frontend through the kit. Nil options load with no
// feature enabled and no cfg option set. The frontend implements the
// exporter role, so a reference through a pub use resolves to the
// declaration that the use publishes, and its version folds the version of
// the grammar. The frontend registers every rust key through its role,
// under the identity of the language.
//
// # Allocation contract
//
// New allocates the state of the frontend with its vocabulary and its
// parse hook, the six functions of the comment walk that capture the
// vocabulary, the version, two allocations of the syntax and four of the
// kit: 16 allocations, and 17 with the empty options that nil options
// take.
func New(opts *Options) plugin.Frontend {
	if opts == nil {
		opts = &Options{}
	}
	f := &rustFrontend{opts: opts, v: newVocabulary(rustgrammar.Grammar)}
	return frontend.New(rust.Name, Lang, rust.Syntax()).
		Version(rust.FrontendVersion+"; "+rustgrammar.Grammar.Version()).
		Match("**/*"+rust.Extension, "!**/"+targetDir+"/**").
		Units(partition).
		Parse(f.parse).
		Resolve(resolve).
		Exports(exports).
		Options(opts).
		Keys(rust.Keys).
		Build()
}
