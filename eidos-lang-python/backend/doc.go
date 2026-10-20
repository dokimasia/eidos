// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package backend renders emit declarations as Python modules and stub
// files.
//
// [New] composes the backend of modules and [Stubs] the backend of stub
// files through the kernel's backend kit. The pieces are the file skeleton
// and the kind templates, the fact [Coverage], the template vocabulary of
// the [Writer], the filename, the package, the name and the type spelling
// of the spell package, the lowering of the constructs that Python states
// in other declarations, the statement printer, the import block, and
// [Layout.Format] as the finalising step.
//
// # Versions
//
// The policy python.version selects the version of Python that a file runs
// on, from 3.11 to 3.15. From 3.12 a type parameter has the syntax of PEP
// 695 and an alias is a type statement. For 3.11 a type parameter is a type
// variable of the module and an alias an assignment annotated with
// TypeAlias. A name of typing that the version does not have imports from
// typing_extensions. An annotation of a module for 3.11 to 3.13 that names
// a declaration of the file that has not finished rendering is a string.
// Lazy imports need 3.15.
//
// # Layout
//
// The templates and [Layout.Format] write the layout of ruff's formatter:
// four spaces of indentation, double quotes, two blank lines around a
// definition at the top level of a module and one around a method, the
// blank lines of the stub style in a stub file, and a signature, the bases
// of a class and the arguments of a decorator on one line up to 88
// characters, on one indented line where they fit, and one item on each
// line with a trailing comma otherwise. The import block groups and sorts
// the imports as isort's defaults do. A value, a default and a body are
// Python source that a generator states, and the backend writes them as
// they are.
//
// # Markers
//
// An annotation whose name starts with python: is a marker of the
// satellite, which states a construct of Python without a field in the
// model: a TypedDict, the bases of an enum, the keywords of a class
// statement, the metadata of an Annotated type, an InitVar field, and the
// header of a file with its __all__, its lazy and its future imports, its
// head comments and its imports.
//
// # Dependency position
//
// lang/python/backend imports the sdk's backend, emit, plugin, render and
// symbol facades, the module root for the language identity and its
// markers, lang/python/spell for the spellings, lang/python/frontend/
// typeshed for the table of the standard library, the grammar of Python
// for the check of the texts of declarations, the lowering, scaffold,
// spellref and treesitter helpers of eidos-lang, and the Go stdlib.
package backend
