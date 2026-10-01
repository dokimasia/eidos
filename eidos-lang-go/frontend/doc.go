// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads Go source into the node graph.
//
// [New] builds the frontend through the kit. The claim is every .go
// file outside testdata and vendor trees: a testdata tree is not Go
// source by Go's own definition, and a vendor tree copies a
// dependency, which loads as one. Test files are claimed too
// and classify under golang.testFile, because whether they take part
// is the consumer's call. Units are package directories, and a
// directory with an external test package declares two packages,
// the second under the import path with a _test suffix.
// The governing go.mod is each unit's shared input: the partition
// probes upward for it, derives the import path from its module
// directive, and every probe that read folds into the unit keys,
// which also fold [golang.FrontendVersion].
//
// # Parsing
//
// Parsing is go/parser per file, with full error recovery: every
// syntax error reports positioned under GOLANG-0001, and every
// declaration the parser still recovered loads, because one bad
// token must not erase a file. Build constraints are configuration:
// [Options] states one tag set per load. A file outside it, by its
// build constraint line or by the GOOS and GOARCH suffixes its name
// implies under the go tool's own rule, contributes its file node, its
// imports and a golang.constraint stamp and no declarations, because
// two platform variants of one function share one canonical
// identity. Only its package clause and imports parse, as the go
// command reads such a file, so a syntax error after its imports
// reports nothing. A generated-file marker classifies under
// golang.generated, and a file importing "C" under golang.cgo.
//
// # Comments, carriers and annotations
//
// Every comment splits through the unit's own pipeline into
// documentation, +-prefixed directive carriers, and the //go:
// directive family lowered as [symbol.Annotations], the shape the
// render side writes back, so the frontend reads what the backend
// spells. Carriers attach on packages, types, functions, methods,
// constants, variables, struct fields, embedded fields and interface
// methods, from leading docs, group docs and trailing comments
// alike. A carrier on a subject no rule takes, such as a parameter,
// a result, an import, a constraint element, a function no code can
// name or a comment outside every declaration, refuses positioned
// under GOLANG-0003. Every declaration that ends a line keeps its trailing
// comment, a function or method the one after its closing brace, and
// a parameter's is shared by every name its field binds. The package
// clause's documentation belongs to the package: its text hoists to
// the first non-empty doc across the unit, and its carriers attach
// to the package. Tool directives above the clause, on an import or
// an import declaration, or floating between declarations are the
// file's annotations. A comment inside a function body belongs to
// the body's statements and is read by nothing, and a declaration
// signature depth leaves out takes its comments with it.
//
// # Types, enums and resolution
//
// A type expression lowers to a reference with its verbatim
// spelling and the structure Go's grammar states: a pointer as an
// optional, a slice as a list, a sized array as an array with its
// literal length in any integer form, a map, a channel as a stream,
// a function type with its parameters then results, and an inline
// body as inline, each child a reference in turn. Arguments split out
// for an explicit generic instantiation, and parentheses unwrap,
// which is the model's stated representation.
//
// An inline struct's named fields and an inline interface's methods
// are the reference's fields and methods, without identities and
// without their comments, so a carrier in the body refuses as one no
// declaration takes. The body's embedded fields, embedded interfaces
// and constraint elements remain in its spelling alone.
//
// A defined type over an ordered basic type, [golang.Ordered], whose
// constants in the same file name it as their type promotes to the
// Enum the schema names for a Go constant group. Its value spellings
// are kept verbatim, and its methods fold in from every file of the
// package, as a struct's do.
// Resolve returns, as one tier, the candidates of the file's
// [golang.Scope], which the parse derives from the file's import
// records. The rules resolve a directive's spelling through the same
// scope, and a qualified reference's package is the import path its
// qualifier binds there. A predeclared type, a constraint term and
// the shapes no single declaration declares (maps, funcs, channels,
// inline bodies) return no candidate, and their named element types
// remain spellings per the model's composite contract. An
// interface's constraint elements are not
// embeds: a union, an approximation, a predeclared basic type or a
// type literal stamps its verbatim spelling under golang.typeSet.
//
// # Dependencies
//
// The frontend is in the dependent role. [Stores] roots the module
// cache and the standard library where the go command finds them, and
// a round reads the build list from the workspace's go.mod files: each
// required module at the highest version any of them requires, after
// the replace directives of the go.mod that requires that version. A
// need places in the standard library when its first element has no
// dot, and otherwise in the module cache at the selected version, its
// hash record checked against go.sum, or in a workspace module's
// vendor tree when the cache lacks the version and the tree passes the
// go command's consistency checks. A dependency unit is one package
// directory without its tests, parsed signature-only, and loads under
// the import path the go command gives it: a replacement's tree under
// the original module path, and a vendored copy under its path under
// vendor/. A standard library file imports the modules the standard
// library vendors under vendor/.
//
// A need the build places nowhere, such as an import of a workspace
// module or an import no required module provides, yields no unit, and
// the round reports it with the reason. The import "C" names cgo's
// preamble and no package, so the round passes over it.
//
// A signature-only file records only the imports its retained
// declarations name, so the next round follows what exported
// signatures reference and not what function bodies call.
//
// # Stamps
//
// Every package a workspace unit declares is stamped with the kernel's
// neutral module identity: gen.module with the module path, and
// gen.moduleRoot with the directory its go.mod is in. A directory
// outside every module has neither, and so does a dependency package,
// which no workspace module governs. Beside the classifications, the parse
// stamps what it alone can see: a pointer receiver, the iterator
// return shapes, an empty or constraint interface, a defined type's
// underlying shape (on the enum when one replaces the type), and
// each constant's exact value where the package's own scope
// evaluates it, iota arithmetic included, through the checker's
// machinery with imports stubbed. What needs the whole graph, such
// as interface satisfaction, embedded interfaces and comparability,
// stamps from the annotate sibling over the sealed graph, where the
// proof is.
//
// # Stated refusals
//
// The frontend deliberately does none of the following:
//
//   - It keeps value spellings verbatim and implicit carriers empty.
//     The exact values are in the stamp, and a constant an import
//     feeds is left unstamped, absent over wrong.
//   - It reads no legacy +build form, and a vendor tree only as the
//     second source of a dependency.
//   - It reads go.mod and never go.work, because an eidos workspace
//     spans toolchain modules by configuration, so the module set a
//     go.work lists decides nothing about what loads, and bytes that
//     cannot change the graph must not key it.
//   - It lowers no init function and no blank declaration, a
//     function, method, field or value named _, because no code can
//     name one and a package may declare any number of each.
//   - It drops free-floating documentation between declarations,
//     which has no model home, and refuses its carriers.
//
// # Dependency position
//
// lang/go/frontend imports the sdk facade, the satellite root, the Go
// toolchain's own parsing and checking packages, and golang.org/x/mod's
// modfile, module and semver packages, and nothing that executes a
// process: a frontend reads what it can parse and never runs a build
// tool, which the package's own test pins over its whole import graph.
// The conformance corpus and the workspace composition import it, and
// no package beneath it does.
package frontend
