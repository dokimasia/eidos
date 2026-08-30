// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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
// machinery. [Symbol] in marker.go types the heterogeneous fields.
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
//   - owner: the generated RewireOwners pass fills each element's
//     Host with the declaring symbol.
//
// # Field conventions
//
// Five fields recur across the kinds and mean the same thing every
// time, so the per-kind documentation does not repeat them.
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
//   - Host: the owner back-pointer on an owned kind, filled by
//     RewireOwners. It is never walk-tagged.
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

//go:generate go run go.dokimi.dev/eidos/core/internal/gen
