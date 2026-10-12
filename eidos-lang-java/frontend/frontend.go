// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/java"
	javagrammar "go.dokimi.dev/eidos/lang/treesitter/java"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration. It restates the
// constant of the satellite root for the callers of the frontend.
const Lang = java.Lang

// UnparsedFile reports a syntax error at its position. The error is a
// problem of the source, and the load continues with every declaration
// that the parser recovered.
var UnparsedFile = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a Java file has a syntax error",
})

// BadCarrier reports a comment line with the + prefix that the kernel
// grammar refused. The carrier attaches nothing, and the load continues.
var BadCarrier = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier or a marker on a subject
// that the model cannot address, such as a parameter, an initializer block
// or a comment that no declaration takes. The author learns that the
// directive attached nowhere.
var UnaddressedCarrier = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// BadPOM reports a pom.xml that does not read or parse. The packages of
// the directories that it governs load without a module identity.
var BadPOM = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a pom.xml does not parse",
})

// UnmodeledItem reports a declaration for which the model has no place: a
// type that an enum declares, because an enum has no list of nested
// types.
var UnmodeledItem = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  5,
	Meaning: "a declaration has no place in the model",
})

// BadClassFile reports a class file, a JAR or an entry of a JAR of a
// dependency unit that does not read or decode, at its file and entry. The
// load continues without that class file, JAR or entry. A JAR whose
// manifest does not read loads its root entries.
var BadClassFile = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  6,
	Meaning: "a class file or a JAR does not decode",
})

// BadMarker reports an annotation of the brand that is not a valid
// directive. Either its name is not the brand followed by a name, or by a
// plugin and a name, or an element value is not a literal or an array of
// literals. The annotation attaches nothing and remains an annotation of
// its declaration.
var BadMarker = diag.MustRegister(java.CodePrefix, diag.CodeSpec{
	Number:  7,
	Meaning: "an annotation of the brand is not a valid directive",
})

// Options is the declared configuration of the frontend, which every unit
// key folds.
type Options struct {
	// Release is the Java release that the load reads: the release of
	// ct.sym, and the version for which a multi-release JAR resolves. Zero
	// reads the newest release that ct.sym lists.
	Release int

	// Classpath lists the libraries that the workspace compiles against,
	// each as group:artifact:version, the transitive ones included.
	Classpath []string
}

// javaFrontend is the vocabulary of the grammar, which every parse reads
// and none writes, and the configuration of the load.
type javaFrontend struct {
	v    *vocabulary
	opts *Options
}

// New builds the Java frontend through the kit. Nil options read the
// newest release and no library. The frontend declares that the language
// overloads, so the discriminator of a callable contains the types of its
// parameters, and its version folds the version of the grammar. The
// frontend registers every java key through its role, under the identity
// of the language. Its classifier stamps java.testFile on every test file.
//
// # Allocation contract
//
// New allocates the state of the frontend with its vocabulary and two
// hooks, the four functions of the comment walk, the classifier of test
// files, the version, two allocations of the syntax and five of the kit:
// 17 allocations, and 18 with the empty options that nil options take.
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
		Classify(frontend.TestFiles(java.TestFileKey, testFile)).
		Resolve(resolve).
		Dependencies(f.dependencies).
		Stores(Stores).
		Options(opts).
		Keys(java.Keys).
		Build()
}
