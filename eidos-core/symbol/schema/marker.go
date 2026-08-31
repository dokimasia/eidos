// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package schema

// Symbol marks a field that holds any declaration kind.
//
// The generator maps it to symbol.Symbol on both model sides, so a
// field declared as Symbol accepts every kind and a field declared
// as a concrete kind accepts one. Use it where the language
// genuinely admits anything, as [File.Decls] does, and for the Host
// back-pointers, whose declaring kind varies.
//
// It is a marker: it declares no methods, and the generator reads
// it by name rather than by structure.
type Symbol any

// Body marks a field that holds a callable's emit-side content.
//
// The generator carries the field through to the emit model, where
// the spelling resolves to that package's own Body value; the node
// model never sees the field, because it is declared emit-side and
// parsed bodies are out of scope. Like [Symbol], it is a marker
// read by name.
type Body any

// Annotations marks a field that holds the structured markers a
// generated declaration writes: Java annotations, Rust attributes,
// TypeScript and Python decorators, C# attributes.
//
// The generator carries the field through to the emit model, where
// the spelling resolves to that package's own Annotations value.
// The node model never sees the field, because a source
// declaration's annotations are lifted into the frontend's own
// metadata namespace, where authority and overrides apply; a
// generated declaration has no metadata bag, so what it must carry
// rides the model. Like [Symbol], it is a marker read by name.
type Annotations any
