// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads Rust source into the node graph.
//
// [New] builds the frontend through the kit. The claim is every .rs file
// outside a target directory, which is Cargo's build output. A unit is
// one crate target, because an inherent impl block anywhere in a crate
// adds methods to a type the crate declares, and the module tree starts
// at the crate root. Its shared input is the package's Cargo.toml, the
// nearest one above each file. Every unit key folds the frontend's
// version, the grammar's and the [Options].
//
// # Crates and packages
//
// Cargo's layout and the manifest place each file in a target: the
// library at src/lib.rs or its [lib] path, the binaries at src/main.rs
// and under src/bin, and the integration tests, examples and benches
// under tests, examples and benches. A file below the directory of the
// library's root, src in Cargo's layout, belongs to the library, and a
// file under src to the src/main.rs binary, because Cargo finds a crate's
// modules below its root's directory. Where both directories contain a
// file, the deeper one's target takes it, and the library takes a tie.
// Neither takes a file under src/bin, which Cargo's layout reserves for
// binaries. A directory under tests, examples or benches that no target
// roots, such as tests/common, is a unit of its own. A file
// without a Cargo.toml above it is a crate of its own, and so is each
// file of a package whose manifest does not parse, which reports under
// [BadManifest].
//
// A library's package path is its crate name: [lib] name, or the
// package name with hyphens replaced by underscores. Any other target's
// is its own name. A module's package path is the crate's followed by
// the module path, as in mycrate/store/table. The parse walks the module
// tree from the crate root through every mod item, honouring #[path],
// and an inline mod block is a nested package. A member no mod item
// names loads under the module path its place in the crate's
// directories spells, and reports under [UnlinkedFile].
//
// # Declarations
//
// A struct is a Struct, and a union a Struct stamped rust.union. An enum
// whose variants state no payload is an Enum with each discriminant
// verbatim, and one where any does is a Sum, a tuple variant's fields
// unnamed. A trait is an Interface with its supertraits in Extends, a
// method without a body abstract and one with a body a default, and an
// associated type an Alias without a target among its Types. A fn is a
// Function, a const a Constant, a static a Variable, mutable for static
// mut, and a type item an Alias. An attribute is an annotation of its
// declaration with its arguments verbatim, a #[doc] attribute adds
// documentation, and rust.lifetimeParams stamps a declaration's lifetime
// parameters, which the model has no parameter for. A macro declares
// nothing, because its expansion is outside what the syntax states.
//
// An inherent impl block folds its methods onto the type it names, a
// function without self at the type level, and its associated constants
// onto the type's fields. A trait impl adds its trait to a struct's
// Implements. An impl of a type the crate does not declare declares its
// methods outside their type, which receives them, and an associated
// constant without a field list to join reports under [UnmodeledItem].
//
// pub is public, pub(crate) internal, and no modifier private. Every
// other restriction is internal, and rust.visibility stamps its
// spelling.
//
// # Conditional compilation
//
// The [Options] state one set of cfg predicates per load: the features
// the load enables and the cfg options it sets. The test predicate is
// always true, so a test item loads, and #[test] and #[cfg(test)] stamp
// it rust.test. An item that any other predicate outside the set keeps
// out declares nothing, and rust.cfg stamps its file with the predicate,
// because two variants of one item share one identity. A module kept out
// leaves out every file below it, and so does a file whose inner
// #![cfg] predicate is false.
//
// # Types
//
// A reference spells its tokens without the whitespace between them,
// one space kept between two identifier tokens. &T is a Borrow, [T; N]
// an Array, with N as its length where N is a literal, [T] a List, a
// tuple a Tuple, and a function pointer type a Func. A generic type
// keeps its bare name or path in the spelling and its arguments in
// Args. Every other type is Named with its spelling, and a path, or a
// name a use declaration binds, records the module it names as its
// package. Rust cannot overload, and every callable takes the empty
// discriminator.
//
// # Resolution and re-exports
//
// The parse records each module's child modules, use bindings, glob
// imports and extern crates before its items lower. Resolve returns the
// module's own item and its use binding of a name in one tier, and its
// glob imports in the next. crate, self and super spell a path from the
// crate root, the module and its parent. Any other path names its first
// segment's child module, the module a use declaration binds it to, or
// an external crate, and then a child module no mod item the parse read
// declares. A module's items are not in scope in its child modules. The
// frontend is in the exporter role: a module publishes the names its
// use declarations bind and what its glob imports name, so a reference
// through a pub use resolves to the declaration it names. File Imports
// record every use and extern crate declaration, and File Exports every
// pub use declaration.
//
// # Comments and carriers
//
// ///, /** */ and #[doc] document the item after them, with the
// attributes between skipped, and //! and /*! */ document the module
// they are in. A run of line comments reads as one text, so a carrier's
// continuation folds across them. The plain comment on an item's last
// line is its trailing comment, and its carriers attach too. A carrier
// no declaration takes reports under [UnaddressedCarrier], and one the
// kernel grammar refuses under [BadCarrier]. The comments in a function
// body belong to its statements, which the model does not contain.
//
// # Classification
//
// rust.test stamps every file of an integration test and of a shared
// module under the tests directory, and an item #[test] or #[cfg(test)]
// marks.
//
// # Signature depth
//
// A load at signature depth leaves out every item and field without
// pub, with its comments. A module loads at every depth, because a pub
// use can publish the pub items of a private module.
//
// # Dependency position
//
// lang/rust/frontend imports the sdk facade, the satellite root,
// lang/treesitter and its Rust grammar, github.com/BurntSushi/toml for
// the manifest, and the Go stdlib. It runs no tool and reads nothing
// outside its units' doors. The conformance corpus and a composition
// import it.
package frontend
