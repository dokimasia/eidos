// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes protobuf's spellings of naming facts: [Filename]
// joins a unit's file-key stem, family word and tag into a filename,
// [Package] names the package and the import path of a file at a routed
// path, and [Name] spells a declared name in the convention of protobuf's
// style guide. [Type] spells the canonical shape of a type of another
// language in protobuf under the target's lowering policies, as the spoke
// of the cross-language hub. The kernel consumes Filename, Package, Name
// and Type through the backend kit's naming, package, respelling and type
// steps.
//
// # Dependency position
//
// lang/protobuf/spell imports the sdk's emit, plugin, rules and symbol
// facades, the module root for the language identity, the type names, the
// markers and the policy keys, the case conversion and the spoke helpers
// of eidos-lang, and the Go stdlib.
package spell
