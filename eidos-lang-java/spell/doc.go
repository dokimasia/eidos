// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes Java's spellings of naming facts: a file
// is named after the lone public type its unit holds, and a
// typeless unit falls back to the routing-key stem, family word
// and tag joined as one Pascal name. It is the module's half of
// the lowering, and the kernel consumes it through the backend
// kit's naming step.
//
// # Dependency position
//
// lang-java/spell imports core/plugin, the module root for the
// language identity, the case conversion of eidos-lang, and the
// Go stdlib.
package spell
