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
// # Parsing
//
// Parsing is go/parser per file, with full error recovery: every
// syntax error reports positioned under GOLANG-0001 and every
// declaration the parser still recovered loads, because one bad
// token must not erase a file. Build constraints are
// configuration: [Options] states one tag set per load, and a file
// outside it — by go:build line or by filename-implied GOOS and
// GOARCH suffixes — contributes its file node, its imports and a
// golang.constraint stamp and no declarations, because two
// platform variants of one function share one canonical identity.
// A generated-file marker classifies under golang.generated, and a
// file importing "C" under golang.cgo.
//
// # Comments, carriers and annotations
//
// Every comment splits through the unit's own pipeline into
// documentation, +-prefixed directive carriers, and the //go:
// directive kin lowered as [symbol.Annotations] — the same shape
// the render side writes back, so what the backend spells the
// frontend reads. Carriers attach on packages, types, functions,
// methods, constants, variables, struct fields and interface
// methods, from leading docs, group docs and trailing comments
// alike; a carrier on a subject the model cannot address — an
// embedded field, a constraint element, a comment no declaration
// owns, which is where the parser leaves parameter comments —
// refuses positioned under GOLANG-0003 rather than vanishing. The
// package clause's documentation belongs to the package: its text
// hoists to the first non-empty doc across the unit and its
// carriers attach to the package.
//
// # Types, enums and resolution
//
// A type expression lowers to a reference carrying its verbatim
// spelling, arguments split out for an explicit generic
// instantiation and parentheses unwrapped, which is the model's
// stated representation. A defined type over a basic underlying
// whose constants carry its spelling promotes to the Enum the
// schema names for a Go constant group, methods folded in and
// value spellings verbatim. Resolve owns Go's probing: decoration
// strips — pointer, slice, array, variadic, parentheses, a
// trailing instantiation — then a qualified spelling probes the
// import its qualifier binds, an unbound qualifier probes every
// import in source order because a package's clause can differ
// from its path, and a bare spelling probes the file's own package
// and then, exported, each dot-imported package. Builtins,
// constraint terms and the shapes no single declaration owns —
// maps, funcs, channels, inline bodies — answer nothing, their
// named element types staying spellings per the model's composite
// contract. An interface's constraint elements are not embeds:
// their verbatim spellings stamp under golang.typeSet.
//
// # Stamps
//
// Every package a unit declares carries the kernel's neutral
// module identity — gen.module with the module path, gen.moduleRoot
// with the directory its go.mod sits in — and a directory outside
// every module carries neither. Beside the classifications, the
// parse stamps what it alone can see: a pointer receiver, the iterator return shapes, an empty or
// constraint interface, a defined type's underlying shape — on the
// enum when one stands for it — and each constant's exact value
// where the package's own scope evaluates it, iota arithmetic
// included, through the checker's machinery with imports stubbed.
// What needs the whole graph — interface satisfaction, embedded
// interfaces, comparability — stamps from the annotate sibling
// over the sealed graph, where the proof is.
//
// # Stated refusals
//
// What this frontend deliberately does not do, so silence is
// never the answer: value spellings in the model stay verbatim
// and implicit carriers empty, the exact values living in the
// stamp; a constant an import feeds stays unstamped, absent over
// wrong; it reads neither the legacy +build form nor vendor
// directories; it reads go.mod and never go.work, because an
// eidos workspace spans toolchain modules by configuration, so the
// module set a go.work lists decides nothing about what loads, and
// bytes that cannot change the graph must not key it; an import's
// local name defaults to the path's last segment, the
// unbound-qualifier probe catching the mismatch for workspace
// packages. Free-floating documentation and tool directives
// between declarations have no model home and drop; their
// carriers refuse.
//
// # Dependency position
//
// lang/go/frontend imports the sdk facade and the Go toolchain's
// own parsing packages, and nothing that executes a process: a
// frontend reads what it can parse and never runs a build tool,
// which the package's own test holds over its whole import graph.
// The conformance corpus and the workspace composition import it,
// and nothing beneath does.
package frontend
