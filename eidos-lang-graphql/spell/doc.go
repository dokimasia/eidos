// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes the GraphQL spellings of naming facts. [Filename]
// joins the file-key stem, the family word and the tag of a unit into a
// filename. [Package] names the one package of every GraphQL file of a
// plan, and [Name] spells a declared name in the convention of the guide to
// naming of graphql.org. [Type] spells the canonical shape of a type of
// another language in GraphQL under the policies of the target, as the
// spoke of the cross-language hub. The kernel calls Filename, Package, Name
// and Type through the naming, package, respelling and type steps of the
// backend kit.
//
// # Nullability
//
// A GraphQL type is non-null unless it is optional. A named reference has
// the bare name as its spelling, and the backend writes the mark ! after it.
// A structural reference has the type expression as its spelling: a list
// with its brackets and the marks of its elements, and an optional without
// the mark of its child.
//
// # Dependency position
//
// lang/graphql/spell imports the sdk's emit, plugin, rules and symbol
// facades, the module root for the language identity and the policy keys,
// the case conversion and the spoke helpers of eidos-lang, and the Go
// stdlib.
package spell
