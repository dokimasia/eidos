// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package java is the root of the Java satellite: the language's
// identity and comment forms, which the satellite's packages share.
//
// [Lang] is the language of every Java declaration, and [CodePrefix]
// opens every diagnostic code the satellite registers. [Target] and
// [Name] are the spellings a plan resolves to select the backend,
// [Extension] is the suffix of every Java file, [Version] is the
// backend's behavior version, and [FrontendVersion] the frontend's.
// [Syntax] returns Java's comment forms, which the output contract
// writes the generated-file header through. [Keys] registers the
// java.* keys a frontend stamps.
//
// # Packages
//
// The module covers Java as a source and as a target:
//
//   - frontend loads Java source, and the class files of the JDK's ct.sym
//     and of classpath JARs, into the node graph.
//   - frontend/classfile decodes class files.
//   - spell spells filenames and declared names.
//   - backend renders emit values as Java source.
//   - testing runs javac and java over generated output.
//
// # Dependency position
//
// The root package imports the sdk's diag, meta, plugin and symbol
// facades and the Go stdlib. The module's other packages import the
// kernel's SPI through the sdk facade, the shared helpers of eidos-lang,
// the Java grammar of lang/treesitter, and the Go stdlib.
package java
