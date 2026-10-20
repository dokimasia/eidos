// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package python contains the identity of the Python satellite and the
// vocabulary that its packages share.
//
// [Lang] is the source language of every declaration that the frontend
// loads. [Name] is the plugin identity of the findings and the stamps of the
// satellite, and [CodePrefix] opens every diagnostic code that it registers.
// The frontend claims the files that end in [Extension] or [StubExtension].
// The package of a module is its dotted name joined with [PackageSep], and
// the module of a package directory is its [InitModule] file. Every unit key
// folds [FrontendVersion], and [Syntax] returns the comment forms. A plan
// selects the backend through [Target], and the composition fingerprint
// folds [BackendVersion].
//
// # Packages
//
// The module covers Python as a source and as a target:
//
//   - frontend loads modules and stub files into the node graph, and reads
//     the standard library from the stubs of typeshed.
//   - rules projects the declarations of Python for the walks of the
//     kernel.
//   - spell spells filenames, declared names, packages and the types of
//     other languages.
//   - backend renders emit declarations as Python modules and stub files.
//   - testing compiles, type-checks and tests written trees through
//     CPython and pyright.
//
// # Stamps
//
// The frontend stamps what Python states and the model has no field for,
// under the keys that [Keys] registers:
//
//   - [TestFileKey] marks a test of pytest, and [StubbedKey] a source file
//     that a stub file of its name states the types of.
//   - [TypedDictKey], [BasesKey] and [KeywordsKey] state the parts of a
//     class statement, and [InitVarKey] a field of a dataclass that its
//     instances do not keep.
//   - [ImplementationKey] marks the implementation of an overloaded
//     callable, and [GeneratorKey] a callable whose body yields.
//   - [TypeVarKey] marks the variable of a type variable, and
//     [AnnotatedKey] lists the metadata of an Annotated type.
//   - [LazyKey], [FutureKey], [HeadKey] and [ConditionKey] state the parts
//     of the head of a file and the checks that excluded a branch.
//
// # Restating a source
//
// A generator over Python sources restates the stamps of a declaration
// through [Stated], which returns its markers. [Header] returns the header
// of the file of a declaration, which states its docstring, its head and
// its imports. Both read the stamps through a [Match] of the generator.
//
// # Channels
//
// A generator states each construct of Python through one channel of the
// emit model: a field of the model that other languages share, an
// annotation for a decorator, or a [Marker] for a construct that is not a
// decorator. [Policies] returns the lowering policies of the target: the
// version that the written files run on, which [Minor] reads, and the type
// of a dynamic value.
//
// # Dependency position
//
// lang/python imports the sdk facade and the Go stdlib.
package python
