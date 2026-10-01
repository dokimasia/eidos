// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	tsgrammar "go.dokimi.dev/eidos/lang/treesitter/typescript"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language of every loaded declaration: the
// satellite root's constant, restated for the frontend's callers.
const Lang = typescript.Lang

// Keys registers every typescript key: the satellite root's one
// registration, restated here, where the corpus and the suite
// fixtures use it.
var Keys = typescript.Keys

// UnparsedFile reports a syntax error, positioned at it: the source's
// problem, and the load continues with every declaration the parser
// still recovered.
var UnparsedFile = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a TypeScript file has a syntax error",
})

// BadCarrier reports a +-prefixed comment line the kernel grammar
// refused: the carrier attaches nothing and the load continues.
var BadCarrier = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a directive carrier on a subject the
// model cannot address, such as a parameter, a statement or a comment
// no declaration takes, so the author learns that the directive
// attached nowhere.
var UnaddressedCarrier = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// BadConfig reports a tsconfig.json that does not parse. The files it
// governs load without its baseUrl and paths.
var BadConfig = diag.MustRegister(typescript.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a tsconfig.json does not parse",
})

// The extensions the frontend claims: TypeScript's own and its ES
// module and CommonJS forms, and TSX, which parses with the TSX
// grammar.
const (
	extensionTS  = ".ts"
	extensionTSX = ".tsx"
	extensionMTS = ".mts"
	extensionCTS = ".cts"
)

// nodeModules is the directory a package manager installs packages
// into, which the claim leaves out.
const nodeModules = "node_modules"

// New builds the TypeScript frontend through the kit. It declares that
// the language overloads and implements the exporter role, so a
// reference through a re-export resolves to the declaration it
// publishes. Its version folds the grammar's, so an upgrade of the
// grammar re-keys every unit.
func New() plugin.Frontend {
	f := &tsFrontend{
		ts:  newVocabulary(tsgrammar.TypeScript),
		tsx: newVocabulary(tsgrammar.TSX),
	}
	return frontend.New(typescript.Name, Lang, typescript.Syntax()).
		Version(typescript.FrontendVersion+"; "+tsgrammar.TypeScript.Version()).
		Overloads().
		Match("**/*"+extensionTS, "**/*"+extensionTSX, "**/*"+extensionMTS, "**/*"+extensionCTS,
			"!**/"+nodeModules+"/**").
		Units(partition).
		Parse(f.parse).
		Classify(markTests).
		Resolve(resolve).
		Exports(exports).
		Build()
}

// tsFrontend is the two grammars' vocabularies, which every parse reads
// and none writes.
type tsFrontend struct {
	ts, tsx *vocabulary
}

// vocabularyOf returns the vocabulary of the grammar that parses a
// file: TSX for a .tsx file, and TypeScript for every other.
func (f *tsFrontend) vocabularyOf(path string) *vocabulary {
	if strings.HasSuffix(path, extensionTSX) {
		return f.tsx
	}
	return f.ts
}
