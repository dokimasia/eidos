// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	java "go.dokimi.dev/eidos/lang/java"
	javagrammar "go.dokimi.dev/eidos/lang/treesitter/java"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration: the
// satellite root's constant, restated for the frontend's callers.
const Lang = java.Lang

// Keys registers every java key: the satellite root's one registration,
// restated here, where the corpus and the suite fixtures use it.
var Keys = java.Keys

// UnparsedFile reports a syntax error, positioned at it: the source's
// problem, and the load continues with every declaration the parser
// still recovered.
var UnparsedFile = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a Java file has a syntax error",
})

// BadCarrier reports a +-prefixed comment line the kernel grammar
// refused: the carrier attaches nothing and the load continues.
var BadCarrier = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier on a subject the model
// cannot address, such as a parameter, an initializer block or a
// comment no declaration takes, so the author learns that the directive
// attached nowhere.
var UnaddressedCarrier = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// BadPOM reports a pom.xml that does not read or parse. The packages of
// the directories it governs load without a module identity.
var BadPOM = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a pom.xml does not parse",
})

// UnmodeledItem reports a declaration the model has no place for: a
// type an enum declares, because an enum has no list of nested types.
var UnmodeledItem = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  5,
	Meaning: "a declaration has no place in the model",
})

// BadClassFile reports a class file, a JAR or a JAR's entry of a
// dependency unit that does not read or decode, at its file and entry.
// The load continues without that class file, JAR or entry, and a JAR
// whose manifest does not read loads its root entries.
var BadClassFile = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  6,
	Meaning: "a class file or a JAR does not decode",
})

// Options is the frontend's declared configuration, which every unit key
// folds.
type Options struct {
	// Release is the Java release the load reads: the ct.sym release,
	// and the version a multi-release JAR resolves for. Zero reads the
	// newest release ct.sym lists.
	Release int

	// Classpath lists the libraries the workspace compiles against,
	// group:artifact:version each, transitive ones included.
	Classpath []string
}

// javaFrontend is the grammar's vocabulary, which every parse reads and
// none writes, and the load's configuration.
type javaFrontend struct {
	v    *vocabulary
	opts *Options
}

// New builds the Java frontend through the kit. A nil options value
// reads the newest release and no library. It declares that the
// language overloads, so a callable's discriminator spells its
// parameters' types, and its version folds the grammar's.
//
// # Allocation contract
//
// New allocates the frontend's state with its vocabulary and two
// hooks, the version, the syntax, two allocations, and the kit's four:
// eleven allocations, and twelve with the empty options that nil
// options take.
func New(opts *Options) plugin.Frontend {
	if opts == nil {
		opts = &Options{}
	}
	f := &javaFrontend{v: newVocabulary(javagrammar.Grammar), opts: opts}
	return frontend.New(java.Name, Lang, java.Syntax()).
		Version(java.FrontendVersion + "; " + javagrammar.Grammar.Version()).
		Overloads().
		Match("**/*" + java.Extension).
		Units(partition).
		Parse(f.parse).
		Classify(markTests).
		Resolve(resolve).
		Dependencies(f.dependencies).
		Options(opts).
		Build()
}
