// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads Rust source into the node graph.
//
// [New] builds the frontend through the kit. The frontend claims every .rs
// file outside a target directory, which contains the build output of
// Cargo. A unit is one crate target, because an inherent impl block
// anywhere in a crate adds methods to a type of the crate, and the module
// tree starts at the crate root. The shared input of a unit is the
// Cargo.toml of the package, the nearest one above each file. Every unit
// key folds the version of the frontend, the version of the grammar and
// the [Options].
//
// # Crates and packages
//
// The layout of Cargo and the manifest place each file in a target: the
// library at src/lib.rs or at its [lib] path, the binaries at src/main.rs
// and under src/bin, and the integration tests, examples and benches under
// tests, examples and benches. A file below the directory of the root of
// the library, src in the layout of Cargo, belongs to the library, and a
// file under src belongs to the src/main.rs binary, because Cargo finds
// the modules of a crate below the directory of its root. Where both
// directories contain a file, the target of the deeper directory takes it,
// and the library takes a tie. Neither takes a file under src/bin, which
// the layout of Cargo reserves for binaries. A directory under tests,
// examples or benches that no target roots, such as tests/common, is a
// unit of its own. A file without a Cargo.toml above it is a crate of its
// own, and so is each file of a package whose manifest does not parse,
// which reports under [BadManifest].
//
// The package path of a library is its crate name: the [lib] name, or the
// package name with hyphens replaced by underscores. The package path of
// any other target is its own name. The package path of a module is the
// path of the crate followed by the module path, as in mycrate/store/table.
// The parse walks the module tree from the crate root through every mod
// item, with #[path] honoured, and an inline mod block is a nested
// package. A member that no mod item names loads under the module path of
// its place in the directories of the crate, and reports under
// [UnlinkedFile].
//
// Every package of a unit has the module facts of the kernel at the
// position of its first file: gen.module with the crate name of the
// target, and gen.moduleRoot with the directory of its Cargo.toml, or the
// directory of the root file for a crate of its own.
//
// # Declarations
//
// A struct is a Struct, and a union is a Struct with the stamp rust.union.
// An enum whose variants have no payload is an Enum with each discriminant
// verbatim, and an enum where any variant has one is a Sum, with the
// fields of a tuple variant unnamed. A trait is an Interface with its
// supertraits in Extends. A method of a trait without a body is abstract,
// a method with a body has a default, and an associated type is an Alias
// without a target among the Types of the Interface. A fn is a Function, a
// const a Constant, a static a Variable, mutable for static mut, and a
// type item an Alias. An attribute is an annotation of its declaration
// with its arguments verbatim, and a #[doc] attribute adds documentation.
// rust.lifetimeParams stamps the lifetime parameters of a declaration, for
// which the model has no parameter. A macro declares nothing, because its
// expansion is outside the syntax.
//
// An inherent impl block folds its methods onto its type, with a function
// without self at the type level, and its associated constants onto the
// fields of the type. A trait impl adds its trait to the Implements of a
// struct. An impl of a type that the crate does not declare declares its
// methods outside their type, which receives them. An associated constant
// without a field list reports under [UnmodeledItem].
//
// pub is public, pub(crate) internal, and no modifier private. Every other
// restriction is internal, and rust.visibility stamps its spelling.
//
// # Conditional compilation
//
// The [Options] state one set of cfg predicates per load: the features
// that the load enables and the cfg options that it sets. The test
// predicate is always true, so a test item loads, and #[test] and
// #[cfg(test)] stamp rust.test on it. An item that any other predicate
// outside the set keeps out declares nothing, and rust.cfg stamps its file
// with the predicate, because two variants of one item share one identity.
// A module that a predicate keeps out leaves out every file below it, and
// so does a file whose inner #![cfg] predicate is false.
//
// # Types
//
// The spelling of a reference is its tokens without the whitespace between
// them, with one space kept between two identifier tokens. &T is a Borrow,
// [T; N] an Array, with N as its length where N is a literal, [T] a List,
// a tuple a Tuple, and a function pointer type a Func. dyn Trait, impl
// Trait and dyn A + B are an Intersection of their trait bounds, with the
// dyn or the impl in the spelling. A child that the source does not state
// is an empty reference. A generic type keeps its bare name or path in the
// spelling and its arguments in Args. Every other type is Named with its
// spelling. A path, or a name that a use declaration binds, records its
// module as its package. Rust cannot overload, and every callable takes
// the empty discriminator.
//
// # Resolution and re-exports
//
// The parse records the child modules, the use bindings, the glob imports
// and the extern crates of each module before its items lower. Resolve
// returns the own item of the module and its use binding of a name in one
// tier, and its glob imports in the next. crate, self and super start a
// path at the crate root, the module and its parent. Any other path starts
// at the child module of its first segment, at the target of the use
// binding of that segment, or at an external crate, and then at a child
// module that no mod item that the parse read declares. The items of a
// module are not in scope in its child modules. The frontend has the
// exporter role: a module publishes the names that its use declarations
// bind and the names that its glob imports import, so a reference through
// a pub use resolves to the declaration that the use publishes. The
// Imports of a File record every use and extern crate declaration, and its
// Exports record every pub use declaration.
//
// # Comments and carriers
//
// ///, /** */ and #[doc] document the item after them, past the
// attributes between them, and //! and /*! */ document their module. A run
// of line comments reads as one text, so the continuation of a carrier
// folds across the run. The plain comment on the last line of an item is
// its trailing comment, and its carriers attach too. A carrier that no
// declaration takes, or that is on a subject that the model cannot
// address, reports under [UnaddressedCarrier], and the finding describes
// the subject. A carrier that the kernel grammar refuses reports under
// [BadCarrier]. The comments in a function body belong to its statements,
// which the model does not contain.
//
// # Markers
//
// An attribute whose path starts with the brand is a marker of a
// directive, as #[acme::stub(tag = "test")] and #[acme::gen::table] are.
// The frontend lifts its arguments from the syntax: key = value as a keyed
// argument, and a literal or a name as a positional one. A literal is a
// string, an integer or a float with at most one minus sign, true or
// false, and a number lifts as its decimal text. The marker attaches its
// directive to the item, field, variant or module that it marks, and it
// remains an annotation. A marker whose path is not a directive name, or
// whose argument does not lift, reports under [BadMarker]. A marker on an
// item that the model does not contain reports under [UnaddressedCarrier].
//
// # Classification
//
// rust.test stamps every file of an integration test and of a shared
// module under the tests directory, and an item that #[test] or
// #[cfg(test)] marks. rust.generated stamps a file whose first comment
// contains [rust.GeneratedMark], with the comment as written.
//
// # Syntax errors
//
// A syntax error is a problem of the source. The frontend reports the
// first [plugin.MaxSyntaxFindings] errors of a file under [UnparsedFile],
// each at its position, and one finding that counts the rest. Every item
// that the parser recovered loads.
//
// # Signature depth
//
// A load at signature depth leaves out every item and field without pub,
// with its comments. A module loads at every depth, because a pub use can
// publish the pub items of a private module.
//
// # Dependency position
//
// lang/rust/frontend imports the sdk facade, the satellite root,
// lang/treesitter and its Rust grammar, lang/numeric,
// github.com/BurntSushi/toml for the manifest, and the Go standard
// library. It runs no tool and reads nothing outside the doors of its
// units. The conformance corpus and a composition import it.
package frontend
