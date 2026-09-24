// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes TypeScript's spellings of naming facts:
// [Filename] joins a unit's routing-key stem, family word and tag
// into a filename, [Name] spells a declared name in TypeScript's
// convention, and [IsIdentifier] reports whether a name spells bare.
// The kernel consumes the first two through the backend kit's naming
// and respelling steps.
//
// # Dependency position
//
// lang/typescript/spell imports the sdk's plugin and symbol facades,
// the module root for the language identity, the case conversion of
// eidos-lang, and the Go stdlib.
package spell
