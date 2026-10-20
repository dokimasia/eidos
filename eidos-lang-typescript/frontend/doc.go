// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads TypeScript source into the node graph.
//
// [New] builds the frontend through the kit. The claim is every .ts,
// .tsx, .mts and .cts file outside a node_modules directory, declaration
// files included. A unit is one file, because TypeScript scopes the names
// of a module to its file. The shared inputs of a unit are the tsconfig
// chain that governs it and the nearest package.json above it. The chain
// is the nearest tsconfig.json above the file and every configuration that
// it extends, read as JSON with comments. Every unit key folds the version
// of the frontend and the version of the grammar.
//
// # Parsing
//
// The frontend parses with tree-sitter-typescript through lang/treesitter,
// and a .tsx file with its TSX grammar. Every ERROR and MISSING node
// reports at its position under [UnparsedFile], ten for each file and one
// finding that counts the rest, and every declaration that the parser
// recovered loads. A tsconfig chain that does not read whole reports under
// [BadConfig], and a package.json that does not read or parse under
// [BadPackage]. [BadConfig], [BadPackage], [ReadonlyRest] and
// [RelativeAugmentation] are warnings.
//
// # Packages
//
// The package of a module is its path without its extension, with the
// whole .d.ts of a declaration file. A namespace A.B is the package below
// the package of the module, stamped typescript.namespace with A.B, and a
// declare module 'x' block is the package x. A declare module block with a
// relative specifier augments the module of another file, which the parse
// of one file cannot name, so it reports under [RelativeAugmentation] and
// loads nothing. A script, which is a file
// without a top-level import or export, declares into the package with the
// empty path, which is the global scope of TypeScript, and so does a
// declare global block. A file contributes one File node to each package
// that it declares into, so two declarations of one namespace share one,
// and a second declaration of an interface in one package of a file merges
// into the first.
//
// Every package of a unit has the module of the nearest package.json: the
// name of the package under gen.module, and the directory of the file
// under gen.moduleRoot. A file without a package.json above it, and a
// package.json without a name, has no module.
//
// # Declarations
//
// A class is a Struct, abstract where it states it, with its superclass
// and interfaces. An interface is an Interface, an enum an Enum, const
// where it states it, and a type alias an Alias. A function is a Function.
// A function signature of a name that its scope declares more than once is
// an overload signature with the mark Overload, and the implementation
// after the signatures is a Function without the mark. A method signature
// of a class is an overload signature in the same way. A const is a
// Constant, and let and var are mutable Variables. A
// decorator is an annotation of its declaration, with its arguments
// verbatim. An anonymous default-exported class or function declares
// under the name default. typescript.generator stamps a generator
// function, and typescript.ambient a declaration whose implementation is
// elsewhere: one that a declare statement, a declare block or an ambient
// module states, and every declaration of a declaration file.
//
// A callable that returns Promise<T> is async with the result T, and one
// that returns Promise<void> is async without a result. A callable that
// returns void has no result. A callable without a return type has one
// result without a type, except a constructor and a setter, which have
// none.
//
// The accessibility, the static level, the getter and the setter, and the
// abstract, async, optional and override marks of a method lower to the
// fields of the model, and a # name is hard and private. A member named in
// brackets is computed, and it keeps the brackets in its name. A
// constructor is a method named constructor that constructs, and its
// parameter properties declare fields. The frontend stamps
// typescript.parameterProperty on each such field and on its parameter.
// An index signature is a method named [], which is immutable where the
// signature is readonly. A construct signature is a method named new that
// constructs. The frontend stamps the call signatures of an object type as
// typescript.callSignature on the interface or the alias that declares
// them. It stamps typescript.generator on a generator method,
// typescript.definiteAssignment on a property declared with !, and
// typescript.abstract on an abstract property. An alias whose target is a
// conditional type gets the spelling of the target under
// typescript.conditionalType. A declaration that another module can import
// is public, and every other is visible in its package.
//
// # Types
//
// The spelling of a reference is its tokens without the whitespace between
// them, with one space between two identifier tokens, so a reformatted
// signature has the same identities. The frontend declares that TypeScript
// overloads, and the discriminator of a callable spells the types of its
// parameters. A rest parameter has the type of one argument that it takes,
// which is the element of the array that it collects into. A rest
// parameter of a readonly array reports under [ReadonlyRest], because the
// element has no readonly. The structural forms are the ones that the
// syntax states:
//
//   - T[] is a mutable List, and a tuple a mutable Tuple. A readonly array
//     or tuple is an immutable List or Tuple.
//   - T | undefined and T | null are an Optional, whose Absent lists the
//     undefined and null members. Any other union is a Union.
//   - An intersection is an Intersection, and a function type and a
//     constructor type are a Func.
//   - An object type of one index signature is a Map, which is immutable
//     where the signature is readonly. Any other object type is Inline.
//
// A generic instantiation keeps its bare name in the spelling and its
// arguments in Args. Every other type is Named with its spelling, and a
// name that an import binds records the module specifier of the import as
// its package.
//
// An Inline reference records the members of its object type as its
// fields and methods, lowered as the members of an interface. They have
// no identity, so no stamp addresses them, and a carrier on one reports
// under [UnaddressedCarrier].
//
// # Resolution and re-exports
//
// Resolve returns tiers in the scope order of TypeScript: the enclosing
// namespaces innermost first, the package of the module, the module that
// an import binds the name from, and the global package. A relative
// specifier refers to its file before the index of the directory, an
// emitted JavaScript extension refers to the source file, a tsconfig paths
// pattern and baseUrl place a bare specifier, and any other specifier
// refers to the ambient module of its name. The frontend is in the
// exporter role. A module publishes the names of an export clause, its
// default export and what its export-star modules publish, and the
// resolution step follows them to the declaration that they publish.
//
// # Comments and carriers
//
// The documentation and the carriers of a declaration are the comments
// directly above it, or above the export or declare statement that wraps
// it, past the decorators of a member. The comment walk of lang/treesitter
// attributes them. A run of line comments reads as one text, so the
// continuation of a carrier folds across them. The comment on the last
// line of a declaration is its trailing comment, and its carriers attach
// too. An import has the documentation and the trailing comment of its
// statement, and an export that wraps no declaration the documentation of
// its statement. A carrier in them, and a carrier that no declaration
// takes, reports under [UnaddressedCarrier], and one that the kernel
// grammar refuses under [BadCarrier]. The comments in the body of a
// function belong to its statements, which the model does not contain.
//
// The header of a module is every comment above its first statement, and
// below its hashbang line where it has one. The run directly above a first
// declaration or export is not part of it, because that statement takes it. The run directly above a first import is part of the
// header. The carriers of the header attach to the package of the module,
// so a directive can address a whole module, as one on the package clause
// of a Go file does. The header documents nothing, because the first
// comment of a module is often a license header. A script has no header.
//
// # Markers
//
// A decorator whose path starts with the brand is a marker of a directive,
// as @acme.stub and @acme.gen.table are. The frontend lifts its arguments
// from the syntax: a string, a number with at most one minus sign, true,
// false and an array of them as positional arguments, and an object
// literal in the last position as keyed ones. The marker attaches its
// directive to the class or the member that it decorates, and it remains
// an annotation. A marker whose path is not a directive name, or whose
// argument does not lift, reports under [BadMarker]. A marker on a
// parameter reports under [UnaddressedCarrier].
//
// # Classification
//
// typescript.testFile stamps a file that the default match of Jest finds
// as a test: one under a __tests__ directory, one named test or spec, and
// one whose name ends in .test or .spec before its extension.
// typescript.generated stamps a file whose first comment contains
// @generated, with that comment as written.
//
// # Signature depth
//
// A load at signature depth leaves out a declaration that the module does
// not export, a namespace member that it does not export, and a private
// member or a member with a # name, each with its comments.
//
// # Dependency position
//
// lang/typescript/frontend imports the sdk facade, the satellite root,
// lang/treesitter and its TypeScript grammar, lang/numeric,
// github.com/tailscale/hujson for the tsconfig chain, and the Go standard
// library. It runs no tool and reads nothing outside the doors of its
// units. The conformance corpus and a composition import it.
package frontend
