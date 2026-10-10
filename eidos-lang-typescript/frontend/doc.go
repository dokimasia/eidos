// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads TypeScript source into the node graph.
//
// [New] builds the frontend through the kit. The claim is every .ts,
// .tsx, .mts and .cts file outside a node_modules directory,
// declaration files included. A unit is one file, because TypeScript
// scopes a module's names to its file. Its shared inputs are the
// tsconfig chain that governs it: the nearest tsconfig.json above the
// file, then every configuration that one extends, read as JSON with
// comments. Every unit key folds the frontend's version and the
// grammar's.
//
// # Parsing
//
// The frontend parses with tree-sitter-typescript through
// lang/treesitter, and a .tsx file with its TSX grammar. Every ERROR and
// MISSING node reports positioned under [UnparsedFile], ten per file and
// the remainder counted once, and every declaration the parser still
// recovered loads. A tsconfig chain that does not read whole reports
// under [BadConfig].
//
// # Packages
//
// A module's package is its path without its extension, a declaration
// file's whole .d.ts included. A namespace A.B is the package below the
// module's, stamped typescript.namespace with A.B, and a declare module
// 'x' block is the package x. A script, a file without a top-level
// import or export, declares into the package with the empty path,
// which is TypeScript's global scope, and so does a declare global
// block. A file contributes one File node to each package it declares
// into, so two declarations of one namespace share one, and a second
// declaration of an interface in one package of a file merges into the
// first.
//
// # Declarations
//
// A class is a Struct, abstract where it states it, with its superclass
// and interfaces. An interface is an Interface, an enum an Enum, const
// where it states it, and a type alias an Alias. A function is one
// Function per overload signature, and an overloaded function's
// implementation declares nothing, because TypeScript hides it from
// callers. A const is a Constant, and let and var are mutable
// Variables. A decorator is an annotation of its declaration, its
// arguments verbatim. An anonymous default-exported class or function
// declares under the name default. typescript.generator stamps a
// generator function, and typescript.ambient a declaration whose
// implementation is elsewhere: one a declare statement, a declare
// block or an ambient module states, and every declaration of a
// declaration file.
//
// A callable that returns Promise<T> is async with the result T, and one
// that returns Promise<void> is async without a result. A callable that
// returns void has no result. A callable without a return type has one
// result without a type, except a constructor and a setter, which have
// none.
//
// A method's accessibility, static level, getter and setter, abstract,
// async and override marks lower to the model's fields, and a # name is
// hard and private. A constructor is a method named constructor that
// constructs, and its parameter properties declare fields, each field
// and its parameter stamped typescript.parameterProperty. An index
// signature is a method named [], a construct signature a method named
// new that constructs, and an object type's call signatures stamp
// typescript.callSignature on the interface or alias that declares
// them. typescript.generator stamps a generator method,
// typescript.optional a method declared with ?, typescript.readonly a
// readonly index signature, and typescript.definiteAssignment a
// property declared with !. A declaration another module can import is
// public, and every other is package-visible.
//
// # Types
//
// A reference spells its tokens without the whitespace between them,
// one space kept between two identifier tokens, so reformatting a
// signature changes no identity. The frontend declares that TypeScript
// overloads, and a callable's discriminator spells its parameters'
// types. A rest parameter is typed as one argument it takes, the
// element of the array it collects into. The structural forms are the
// ones the syntax states: T[] is a List, a tuple a Tuple, a readonly
// array or tuple the List or Tuple it reads, T | undefined and T | null
// an Optional, any other union a Union, an intersection an
// Intersection, a function type and a constructor type a Func, an
// object type of one index signature a Map, and any other object type
// Inline. A generic instantiation keeps its bare name in the spelling
// and its arguments in Args. Every other type is Named with its
// spelling, a name an import binds recording the import's module
// specifier as its package.
//
// An Inline reference records its object type's members as its fields
// and methods, lowered as an interface's members are. They have no
// identity, so no stamp names them, and a carrier on one reports under
// [UnaddressedCarrier].
//
// # Resolution and re-exports
//
// Resolve returns tiers in TypeScript's scope order: the enclosing
// namespaces innermost first, the module's own package, the module an
// import binds the name from, and the global package. A relative
// specifier names its file before the directory's index, an emitted
// JavaScript extension names the source file, a tsconfig paths pattern
// and baseUrl place a bare specifier, and any other specifier names
// the ambient module of its name. The frontend is in the exporter role:
// a module publishes an export clause's names, its default export and
// what its export-star modules publish, and the resolution step follows
// them to the declaration they name.
//
// # Comments and carriers
//
// A declaration's documentation and carriers are the comments directly
// above it, or above the export or declare statement that wraps it,
// with a member's decorators skipped. A run of line comments reads as
// one text, so a carrier's continuation folds across them. The comment
// on a declaration's last line is its trailing comment, and its
// carriers attach too. A carrier no declaration takes reports under
// [UnaddressedCarrier], and one the kernel grammar refuses under
// [BadCarrier]. The comments in a function's body belong to its
// statements, which the model does not contain.
//
// # Markers
//
// A decorator whose path starts with the brand is a marker of a
// directive, as @acme.stub and @acme.gen.table are. The frontend lifts
// its arguments from the syntax: a string, a number with at most one
// minus sign, true, false and an array of them as positional arguments,
// and an object literal in the last position as keyed ones. The marker
// attaches its directive to the class or the member that it decorates,
// and it remains an annotation. A marker whose path is not a directive
// name, or whose argument does not lift, reports under [BadMarker]. A
// marker on a parameter or on an overloaded method's implementation
// reports under [UnaddressedCarrier].
//
// # Classification
//
// typescript.testFile stamps a file Jest's default match names a test:
// one under a __tests__ directory, one named test or spec, and one whose
// name ends in .test or .spec before its extension.
//
// # Signature depth
//
// A load at signature depth leaves out a declaration the module does not
// export, a namespace member it does not export, and a private or
// #-named member, each with its comments.
//
// # Dependency position
//
// lang/typescript/frontend imports the sdk facade, the satellite root,
// lang/treesitter and its TypeScript grammar, lang/numeric,
// github.com/tailscale/hujson for the tsconfig chain, and the Go stdlib. It runs no tool and reads
// nothing outside its units' doors. The conformance corpus and a
// composition import it.
package frontend
