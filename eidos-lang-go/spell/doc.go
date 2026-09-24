// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes Go's spellings of naming facts: [Filename]
// joins a unit's routing-key stem, family word and tag into a
// filename, and [Name] spells a declared name in Go's convention,
// where case states visibility. The kernel consumes both through
// the backend kit's naming and respelling steps.
//
// # Dependency position
//
// lang/go/spell imports the sdk's plugin and symbol facades, the
// module root for the language identity, the case conversion of
// eidos-lang, and the Go stdlib, go/token among it.
package spell
