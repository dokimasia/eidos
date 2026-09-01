// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes TypeScript's spellings of naming facts:
// the filename a unit's routing-key stem, family word and tag
// join into. It is the module's half of the lowering, and the
// kernel consumes it through the backend kit's naming step.
//
// # Dependency position
//
// lang-typescript/spell imports core/plugin, the module root for
// the language identity, the case conversion of eidos-lang, and
// the Go stdlib.
package spell
