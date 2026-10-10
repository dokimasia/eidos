// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes TypeScript's spellings of naming facts:
// [Filename] joins a unit's file-key stem, family word and tag into a
// filename, [Package] names the module a file at a routed path is,
// [Name] spells a declared name in TypeScript's convention, and
// [IsIdentifier] reports whether a name spells bare. [Type] spells the
// canonical shape of a type of another language in TypeScript under the
// target's lowering policies, as the spoke of the cross-language hub.
// The kernel consumes Filename, Package, Name and Type through the
// backend kit's naming, package, respelling and type steps.
//
// # Dependency position
//
// lang/typescript/spell imports the sdk's emit, plugin, rules and symbol
// facades, the module root for the language identity, the module path
// and the policy keys, the case conversion, the reference spelling and
// the spoke helpers of eidos-lang, and the Go stdlib.
package spell
