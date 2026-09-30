// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package golang is the root of the Go satellite: the language's
// identity and comment forms, the golang.* fact keys, and the Go
// names the satellite's packages share.
//
// [Lang] is the language of every loaded Go declaration, and
// [CodePrefix] opens every diagnostic code the satellite registers.
// [Target] and [Name] are the spellings a plan resolves to select
// the backend, and [Version] is the backend's behavior version.
// [Syntax] returns Go's comment forms: the frontend strips comments
// with them, and the output contract writes the generated-file
// header through them. [Keys] registers the fact keys the frontend
// and the annotator stamp.
//
// [Predeclared] reports Go's predeclared type names, and [Basic] and
// [Ordered] the basic types among them, read off the universe scope
// of go/types. [ImportName] and [AssumedName] return the qualifier an
// import binds.
//
// [Scope] is what one file's imports bind, and [NewScope] derives it
// from the file's import records. The load's resolution step, the
// rules and the frontend's reference lowering all resolve a spelling
// through it, so Go probes one way everywhere: [Scope.Candidates]
// returns what a spelling may name in probe order, and
// [Scope.Import] returns the path a qualifier binds.
//
// [PointerReceiver] states a pointer receiver on an emit method that
// receives a type, named apart from the method's signature: what a
// generator writing Go stubs calls after the kernel's Mirror, which
// leaves the receiver to the target.
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
// and symbol facades and the Go stdlib, go/types among it. The
// module's other packages import the kernel's SPI through the sdk
// facade, the shared helpers of eidos-lang, and the Go stdlib. None
// imports eidos-lang's grammar packages: the frontend parses with
// the standard library, and the backend renders through the
// kernel's own pass.
package golang
