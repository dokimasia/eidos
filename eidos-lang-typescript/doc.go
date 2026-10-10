// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package typescript is the root of the TypeScript satellite: the
// language's identity and comment forms, which the satellite's
// packages share.
//
// [Lang] is the language of every TypeScript declaration, and
// [CodePrefix] opens every diagnostic code the satellite registers.
// [Target] and [Name] are the spellings a plan resolves to select the
// backend, [Extension] is the suffix of every TypeScript file the
// backend writes, [Version] is the backend's behavior version, and
// [FrontendVersion] the frontend's. [Syntax] returns TypeScript's
// comment forms: the frontend strips comments with them, and the
// output contract writes the generated-file header through them.
// [Keys] registers the typescript.* keys the frontend stamps.
//
// [ModulePath] returns the module a file or a specifier names, which
// is the package the frontend loads a file into and the package the
// backend names a written file's package by. [DeclarationFile]
// reports a file whose every declaration is implemented elsewhere.
//
// [Policies] returns the target's lowering policies. Their keys are
// [Int64], [Absent], [Timestamp], [Bytes] and [Duration], and each choice
// is a constant, such as [BigInt].
//
// [Unquote], [ParseNumber] and [ParseBigInt] decode TypeScript's string,
// numeric and bigint literals. The frontend decodes the arguments of a
// decorator with them. The rules decode the literal of a directive and
// the value of an enum member with them.
//
// # Packages
//
// The module covers TypeScript as a source and as a target:
//
//   - frontend loads TypeScript source into the node graph.
//   - rules projects TypeScript's declarations for the kernel's walks.
//   - spell spells filenames, declared names and the types of other
//     languages.
//   - backend renders emit values as TypeScript source.
//   - testing runs tsc and node over generated output.
//
// # Dependency position
//
// The root package imports the sdk's diag, meta, plugin and symbol
// facades and the Go stdlib, math/big among it. The module's other
// packages import the kernel's SPI through the sdk facade, the shared
// helpers of eidos-lang, the TypeScript grammar of lang/treesitter, and
// the Go stdlib.
package typescript
