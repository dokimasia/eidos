// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	tsgrammar "go.dokimi.dev/eidos/lang/treesitter/typescript"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration: the constant
// of the satellite root, restated for the callers of the frontend.
const Lang = typescript.Lang

// UnparsedFile reports a syntax error at its position. It is a problem of
// the source, and the load continues with every declaration that the
// parser recovered.
var UnparsedFile = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a TypeScript file has a syntax error",
})

// BadCarrier reports a directive carrier that the kernel grammar refused.
// The carrier attaches nothing, and the load continues.
var BadCarrier = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier on a subject that the
// model cannot address, such as a parameter, a statement or a comment that
// no declaration takes, so the author learns that the directive attached
// nowhere.
var UnaddressedCarrier = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// BadConfig reports a tsconfig.json that does not parse. The files that it
// governs load without its baseUrl and paths.
var BadConfig = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a tsconfig.json does not parse",
})

// BadMarker reports a decorator of the brand that is not a valid
// directive. Either its path is not the brand followed by a name, or by
// a plugin and a name, or an argument is not a literal that the frontend
// lifts. The decorator attaches nothing and remains an annotation.
var BadMarker = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  5,
	Meaning: "a decorator of the brand is not a valid directive",
})

// BadPackage reports a package.json that does not read or parse, at its
// first line. Severity is Warning. The packages of the files that it
// governs load without a module.
var BadPackage = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  6,
	Meaning: "a package.json does not parse",
})

// ReadonlyRest reports a rest parameter of a readonly array, at the
// parameter. Severity is Warning. The model types a rest parameter as one
// argument that it takes, so the parameter loads without the readonly.
var ReadonlyRest = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  7,
	Meaning: "a rest parameter of a readonly array loads without its readonly",
})

// RelativeAugmentation reports a declare module block whose name is a
// relative specifier, at the block. Severity is Warning. The block
// augments the module of a file that the parse of its own file cannot
// name, so it loads nothing.
var RelativeAugmentation = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  8,
	Meaning: "a module augmentation of a relative module loads nothing",
})

// UnknownLibrary reports an entry of the lib option, a target, or a lib
// directive of a source file with a name that TypeScript 7.0.2 does not
// know, at the configuration or at the directive. Severity is Warning. The
// entry and the directive select no file, and the target is ES2025.
var UnknownLibrary = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  9,
	Meaning: "a library or a target is unknown to the embedded release of TypeScript",
})

// nodeModules is the directory that a package manager installs packages
// into, which the claim leaves out.
const nodeModules = "node_modules"

// New builds the TypeScript frontend through the kit. It claims the
// extension of TypeScript, its ES module and CommonJS forms, and TSX,
// which parses with the TSX grammar. It declares that the language
// overloads and implements the exporter role, so a reference through a
// re-export resolves to the declaration that it publishes. It implements
// the dependent role over the library store of [Stores], so a global name
// resolves to the declaration of the library of its project. Its version
// folds the version of the grammar, so an upgrade of the grammar re-keys
// every unit. The kit stamps typescript.testFile on every test file, and
// registers every typescript key as the key provider of the frontend.
//
// # Allocation contract
//
// New allocates the state of the frontend with its two vocabularies and
// its parse hook, the three closures of the comment walk of each
// vocabulary, the version, the syntax, two allocations, the test-file
// classifier and the five allocations of the kit: nineteen allocations.
func New() plugin.Frontend {
	f := &tsFrontend{
		ts:  newVocabulary(tsgrammar.TypeScript),
		tsx: newVocabulary(tsgrammar.TSX),
	}
	return frontend.New(typescript.Name, Lang, typescript.Syntax()).
		Version(typescript.FrontendVersion+"; "+tsgrammar.TypeScript.Version()).
		Overloads().
		Match("**/*"+typescript.Extension, "**/*"+typescript.ExtensionTSX,
			"**/*"+typescript.ExtensionMTS, "**/*"+typescript.ExtensionCTS,
			"!**/"+nodeModules+"/**").
		Units(partition).
		Parse(f.parse).
		Classify(frontend.TestFiles(typescript.TestFileKey, testFile)).
		Resolve(resolve).
		Exports(exports).
		Dependencies(dependencies).
		Stores(Stores).
		Keys(typescript.Keys).
		Build()
}

// tsFrontend is the vocabularies of the two grammars, which every parse
// reads and none writes.
type tsFrontend struct {
	ts, tsx *vocabulary
}

// vocabularyOf returns the vocabulary of the grammar that parses a file:
// TSX for a .tsx file, and TypeScript for every other.
func (f *tsFrontend) vocabularyOf(path string) *vocabulary {
	if strings.HasSuffix(path, typescript.ExtensionTSX) {
		return f.tsx
	}
	return f.ts
}
