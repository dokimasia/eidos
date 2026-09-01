// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads Go source into the node graph.
//
// [New] builds the frontend through the kit: the claim is every
// .go file with testdata carved out — a testdata tree is not Go
// source by Go's own definition — while test files stay claimed
// and classify under golang.testFile, because whether they take
// part is the consumer's call. Units are package directories; a
// directory holding an external test package declares two
// packages, the second under the import path with a _test suffix.
// The governing go.mod is each unit's shared input: the partition
// probes upward for it, derives the import path from its module
// directive, and every probe that read folds into the unit keys.
//
// Parsing is go/parser per file. A syntax error is the source's
// problem: it reports positioned under GOLANG-0001 and the load
// continues. Build constraints are configuration: [Options] states
// one tag set per load, a file outside it contributes its file
// node and a golang.constraint stamp and no declarations, because
// two platform variants of one function share one canonical
// identity. Doc comments strip through the unit's pipeline, and a
// +-prefixed doc line is a directive carrier: parsed under the
// kernel grammar, attached to its declaration, and kept out of the
// documentation.
//
// A type expression lowers to a reference carrying its verbatim
// spelling, with arguments split out for an explicit generic
// instantiation. Resolve owns Go's normalization: it strips
// pointer, slice, array and variadic decoration, probes a
// qualified spelling through the file's import bindings and a bare
// exported spelling through its own package, and answers nothing
// for builtins and the shapes no single declaration owns — maps,
// funcs, channels — which stay spellings.
//
// # Dependency position
//
// lang/go/frontend imports the sdk facade and the Go toolchain's
// own parsing packages; the conformance corpus and the workspace
// composition import it, and nothing beneath does.
package frontend
