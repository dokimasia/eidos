// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package backend renders emit declarations as Java source.
//
// [New] composes the language pieces through the kernel's backend
// kit: the file skeleton, the class and interface templates, the
// shared template vocabulary in [Funcs], the statement printer
// [Scaffold], the import renderer [Imports], and a pass-through
// finalising step, returning the Backend and Renderer roles both.
//
// # Dependency position
//
// lang-java/backend imports the kernel's root authoring package
// and the SPI packages beneath it, the module root for the
// language identity, lang-java/spell for filename spelling, the
// shared expression writer of eidos-lang, and the Go stdlib.
package backend
