// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes Java's spellings of naming facts. [Filename]
// names a file after the lone public type its unit declares, and a
// unit without a type after the file-key stem, family word and tag
// joined as one Pascal name. [Package] names the package a file at a
// routed path declares, and [Name] spells a declared name in Java's
// convention. The kernel consumes them through the backend kit's
// naming, package and respelling steps.
//
// # Dependency position
//
// lang/java/spell imports the sdk's emit, plugin and symbol facades,
// the module root for the language identity, the case conversion of
// eidos-lang, and the Go stdlib.
package spell
