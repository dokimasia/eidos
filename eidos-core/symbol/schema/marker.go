// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

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

