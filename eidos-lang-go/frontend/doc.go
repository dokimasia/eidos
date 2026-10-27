// Copyright Dokimasia B.V. 2026
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
// identity. A file without a //go:build line reads its // +build lines
// as go/build reads them, outside the doc of the package. A file with a
// constraint has the annotation go:build, whose one argument is the
// constraint as one expression: its build lines and the tags of its name,
// joined, as the stamp golang.constraint has it. A generated file that
// derives from the file alone writes the constraint. Of an excluded file
// only the package clause and the imports parse, as the go command reads
// such a file, so a syntax error after its imports reports nothing. A
// generated-file marker classifies under golang.generated, and a file
// importing "C" under golang.cgo.
//
// # Comments, carriers and annotations
//
// Every comment splits through the unit's own pipeline into
// documentation, +-prefixed directive carriers, and the //go:
// directive family lowered as [symbol.Annotations], the shape the
// render side writes back, so the frontend reads what the backend
// spells. Carriers attach on packages, types, functions, methods,
// constants, variables, struct fields, embedded fields, interface
// methods, parameters and results, from leading docs, group docs and
// trailing comments alike. A carrier on a parameter or a result attaches
// to each name of its field. A carrier on a subject no rule takes, such
// as a receiver, an import, a constraint element, a function no code can
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
// A defined type promotes to the Enum that the schema names for a Go
// constant group where a constant of its package has the type as its
// declared type. Go gives a constant a boolean, numeric or string type,
// so the target of such a type can be any name. The variants come from
// every file of the package, and a constant of a test file joins only a
// type of a test file. The value spellings are kept verbatim, and the
// methods of the type fold in from every file of the package, as a
// struct's do.
//
// Resolve returns, as one tier, the candidates of the file's
// [golang.Bindings], which the parse derives from the file's import
// records. The rules resolve a directive's spelling through the same
// bindings, and a qualified reference's package is the import path its
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
// the replace directives of the go.mod that requires that version.
//
// A unit uses each declaration outside its package that a named type of
// its declarations or an argument of its directives can resolve to, the
// candidates that the bindings of the file return. A round places the
// package of each use. An import that no declaration and no directive
// names places nothing, so a package that only a function body or a value
// names does not load, and the rules give a reference into it a stand-in.
// A package places in the standard library when its first element has no
// dot, and otherwise in the module cache at the selected version, its
// hash record checked against go.sum, or in a workspace module's vendor
// tree when the cache lacks the version and the tree passes the go
// command's consistency checks. A dependency unit is one package
// directory without its tests, parsed signature-only, and loads under
// the import path the go command gives it: a replacement's tree under
// the original module path, and a vendored copy under its path under
// vendor/. A standard library file imports the modules the standard
// library vendors under vendor/.
//
// A used import that the build places nowhere, such as an import of a
// workspace module or an import no required module provides, yields no
// unit, and the round reports it with the reason. The import "C" names
// cgo's preamble and no package, so the round passes over it.
//
// A signature-only file records only the imports whose packages it uses,
// so the next round follows what exported signatures reference and not
// what function bodies call.
//
// A signature-only parse keeps an unexported type that an exported type of
// its package embeds or names as the target of an alias, because the
// exported type promotes the members of that type. It also keeps each
// unexported type that a kept type reaches the same way. testing.T embeds
// testing.common, so the member walk of a struct that embeds *testing.T
// lists the methods of testing.common. A kept type loads its exported
// fields and methods.
//
// A signature-only parse also lowers each unexported method of an
// interface that it loads, because a type implements the interface only
// where it has the method. Only a type of the package of the interface can
// declare such a method, so the parse keeps the method of that name of
// each type of the package. testing.TB declares the method private, and
// testing.common implements it, so the walk of testing.TB lists private
// and testing.T still implements testing.TB.
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
// underlying shape (on the enum when one replaces the type), a struct
// whose lowering left out a blank or an unexported field, and
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
//   - It reads a vendor tree only as the second source of a dependency.
//   - It reads go.mod and never go.work, because an eidos workspace
//     spans toolchain modules by configuration, so the module set a
//     go.work lists decides nothing about what loads, and bytes that
//     cannot change the graph must not key it.
//   - It lowers no init function and no blank declaration, a
//     function, method, field or value named _, because no code can
//     name one and a package may declare any number of each.
//   - It drops free-floating documentation between declarations,
//     which has no model home, and refuses its carriers.
//   - It drops the text of the leading comment of a parameter or a
//     result, which has no model home, and attaches its carriers.
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
