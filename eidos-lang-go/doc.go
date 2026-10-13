// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package golang is the root of the Go satellite: the language's
// identity and comment forms, the golang.* fact keys, and the Go
// names the satellite's packages share.
//
// [Lang] is the language of every loaded Go declaration, and
// [CodePrefix] opens every diagnostic code the satellite registers.
// [Target] and [Name] are the spellings a plan resolves to select
// the backend, and [Version] is the backend's behavior version.
// [AnnotatorName] is the plugin identity of the annotator.
// [Syntax] returns Go's comment forms: the frontend strips comments
// with them, and the output contract writes the generated-file
// header through them. [Keys] registers the fact keys the frontend
// and the annotator stamp, and [Lookup] returns the annotator's
// handles from a registry that contains them.
//
// [Predeclared] reports Go's predeclared type names, and [Basic] and
// [Ordered] the basic types among them, read off the universe scope
// of go/types. [NumericOf] returns the class and the width of a
// predeclared numeric type, which the rules classify, and
// [NumericName] returns the name of a class and a width, which the
// spoke spells. [ImportName] and [AssumedName] return the qualifier an
// import binds, and [Qualifier] returns the qualifier under which a
// generated file imports a package. [ImportPath] returns the import
// path under which the package of a workspace directory loads. The
// frontend loads the package under it, and the backend uses it as the
// package of a file that it routes into the directory. [Standard]
// reports whether an import path names a package of the standard
// library, for the frontend's load of a dependency and the backend's
// import groups. [TestSuffix] ends a test file's stem and the import
// path the frontend loads an external test package under, so the
// backend spells a file of that package as a test file.
//
// [Bindings] contains what the imports of one file bind, and
// [NewBindings] derives the bindings from the import records of the
// file. The resolution step of the load, the rules and the reference
// lowering of the frontend resolve a spelling through them, so Go
// probes one way everywhere. [Bindings.Candidates] returns the
// declarations that a spelling can refer to in probe order, and
// [Bindings.Import] returns the path that a qualifier binds.
//
// [PointerReceiver] states a pointer receiver on an emit method that
// receives a type, named apart from the method's signature: what a
// generator writing Go stubs calls after the kernel's Mirror, which
// leaves the receiver to the target.
//
// [Restate] writes the syntax of a Go composite type from the spellings
// of its children: a pointer, a slice, an array, a map, a channel and a
// function type. The spoke spells the composite types of a translation
// through it, and the backend writes a composite again through it where
// a child spells otherwise than written.
//
// # Packages
//
// The module's packages cover Go as a source and as a target:
//
//   - frontend loads Go source into the node graph.
//   - rules projects the graph through Go's decisions.
//   - annotate stamps what the sealed graph proves.
//   - spell spells filenames and declared names.
//   - backend renders emit values as Go source.
//   - testing runs the Go toolchain over generated output.
//
// # Projection facts
//
// The language fixes these facts, and no configuration changes them:
//
//   - Callables are Sync always, because Go's concurrency is
//     caller-side and never appears in a signature.
//   - The error model is LastReturn. The error-value rules read the
//     sentinel convention as a name and predicate pair.
//   - Composition is Embeds, never Extends. The members projection
//     and the promotion rules read promotion.
//   - Optionality projects from pointers. The equality rules read
//     comparability and name the members that break it: slices,
//     maps and funcs.
//   - The tag rules read struct tags, and the enum rules read
//     const-group enums, iota arithmetic included.
//
// # Parsing
//
// The frontend parses with the standard library's go/parser, a
// pure-Go library pinned by the module's toolchain version, never a
// toolchain installed on the machine. Positions and comment
// attachment are exact, and one workspace parses alike on every
// machine.
//
// # Dependency position
//
// The root package imports the sdk's diag, emit, meta, node, plugin
// and symbol facades, eidos-lang's naming and spellref, and the Go
// stdlib, go/types among it. The
// module's other packages import the kernel's SPI through the sdk
// facade, the shared helpers of eidos-lang, and the Go stdlib. None
// imports eidos-lang's grammar packages: the frontend parses with
// the standard library, and the backend renders through the
// kernel's own pass.
package golang
