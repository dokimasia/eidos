// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spell writes Python's spellings of naming facts: [Filename] and
// [StubFilename] join a unit's file-key stem, family word and tag into the
// name of a module or of a stub file, [Package] names the module of a file
// at a routed path, [Name] spells a declared name in Python's convention,
// [IsIdentifier] reports whether a name is a Python identifier, and
// [IsKeyword] reports whether it is a keyword. [Type]
// spells the canonical shape of a type of another language in Python under
// the target's lowering policies, as the spoke of the cross-language hub.
// The kernel consumes the filenames, Package, Name and Type through the
// backend kit's naming, package, respelling and type steps.
//
// # Conventions
//
// The spellings follow PEP 8. A class takes CapWords, a function snake
// case and a constant upper snake case. A module is a file in snake case,
// and its package path is its dotted name with slashes, as shop/models for
// shop.models. A leading underscore marks a name that is not public, and a
// keyword of Python takes a trailing underscore.
//
// # Dependency position
//
// lang/python/spell imports the sdk's emit, plugin, rules and symbol
// facades, the module root for the language identity, the extensions, the
// module of a package directory and the policies, the case conversion, the
// reference spelling and the spoke helpers of eidos-lang, and the Go
// stdlib.
package spell
