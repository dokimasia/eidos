// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package schema is the hand-written source of truth for the
// declaration kinds.
//
// One exported struct defines one kind. The generator parses this
// package as source and writes the node and emit models, the walk
// and rewire passes, the JSON codecs and the [symbol.Kind]
// constants. Nothing imports it at run time, and adding a kind or a
// field is an edit here plus a regeneration.
//
// Kinds group by family, one file each: containers, structural type
// declarations, enumerations, callables, members, and type
// machinery. The markers in marker.go are read by name: [Symbol]
// types the heterogeneous fields, and [Body] types a callable's
// emit-side content.
//
// # Annotations
//
// Every field carries an `eidos` tag from a closed vocabulary. The
// generator refuses a token it does not know, naming the schema
// position.
//
//   - side, one of "node", "emit" or "both": which model carries
//     the field.
//   - walk: the field takes part in the generated traversal. Only
//     containment edges carry it, which is what keeps the walk a
//     tree over a cyclic graph.
//   - slot=<name>: on the emit side the field becomes slot storage
//     with typed accessors instead of a plain slice.
//   - name: the field is the declaration's own name, which the
//     identity assignment and the settle's respelling read.
//   - fact=<Fact>: the field states one fact of the coverage
//     vocabulary, which is what a backend declares a verdict
//     against and what the render guard reads.
//
// # Field conventions
//
// Seven fields recur across the kinds and mean the same thing
// every time, so the per-kind documentation does not repeat them.
//
//   - Id: the node-side [symbol.Identity]. It is zero until a
//     frontend assigns it, and zero claims nothing.
//   - Origin: the emit-side identity of the node symbol a generated
//     value derives from. Origin points one way; no node field
//     refers to emit.
//   - Pos: the node-side source position. Synthesized emit values
//     have none, so the field is node-only.
//   - Doc: the declaration's documentation, one entry per line,
//     with the comment markers already stripped.
//   - Comment: the trailing text on the declaration's own line,
//     markers stripped, "" when none. Doc sits above and Comment
//     beside, which is the split a parser makes.
//   - Annotations: the structured markers a generated declaration
//     writes, emit-side through the [Annotations] marker. A source
//     declaration's annotations are the frontend's own metadata
//     instead, where authority and overrides apply.
//   - Host: the identity of the declaration that owns an owned
//     kind, set when a frontend creates the child. It is an
//     identity rather than a pointer, so it can be stored,
//     compared and carried across runs, and reaching the owner
//     goes through a tracked read like any other cross-reference.
//
// # Emptiness
//
// A kind carries what any language in scope needs, and languages
// leave the rest empty. Emptiness never claims anything: an empty
// Extends means "this language does not have nominal supertypes, or
// this declaration has none", never "this type extends nothing".
// Member lists are slices rather than maps keyed by name, because
// Java, Kotlin, C# and TypeScript overload.
//
// # Dependency position
//
// core/symbol/schema imports only core/symbol and core/position.
package schema

//go:generate go run go.dokimi.dev/eidos/core/internal/gen/model/cmd
