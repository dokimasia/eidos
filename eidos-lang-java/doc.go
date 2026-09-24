// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package java is the root of the Java satellite: the language's
// identity and comment forms, which the satellite's packages share.
//
// [Lang] is the language of every Java declaration. [Target] and
// [Name] are the spellings a plan resolves to reach the backend,
// [Extension] is the suffix of every Java file, and [Version] is the
// backend's behavior version. [Syntax] returns Java's comment forms,
// which the output contract writes the generated-file header
// through.
//
// # Packages
//
// The module covers Java as a target:
//
//   - spell spells filenames and declared names.
//   - backend renders emit values as Java source.
//
// # Dependency position
//
// The root package imports the sdk's plugin and symbol facades. The
// module's other packages import the kernel's SPI through the sdk
// facade, the shared helpers of eidos-lang, and the Go stdlib.
package java
