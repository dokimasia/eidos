// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package backend renders emit declarations as Go source.
//
// [New] composes the language pieces through the kernel's backend
// kit: the file skeleton, the kind templates, the shared template
// vocabulary in [Funcs], the statement printer [Scaffold], the
// import-block renderer [Imports], and go/format as the finalising
// step, returning the Backend and Renderer roles both.
//
// # Dependency position
//
// lang-go/backend imports the kernel's root authoring package and
// the SPI packages beneath it, the module root for the language
// identity, lang-go/spell for filename spelling, the shared
// expression writer of eidos-lang, and the Go stdlib.
package backend
