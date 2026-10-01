// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes Go's spellings of naming facts: [Filename]
// joins a unit's file-key stem, family word and tag into a filename,
// a test file for a unit of an external test package, [Package] names
// the package a file at a routed path declares, and [Name] spells a
// declared name in Go's convention, where case states visibility. The
// kernel consumes them through the backend kit's naming, package and
// respelling steps.
//
// # Dependency position
//
// lang/go/spell imports the sdk's plugin and symbol facades, the
// module root for the language identity, the test suffix and the
// import path derivation, the case conversion of eidos-lang, and the
// Go stdlib, go/token among it.
package spell
