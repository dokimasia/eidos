// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package backend renders emit declarations as Go source.
//
// [New] composes the backend through the kernel's backend kit and
// returns the Backend and Renderer roles both. The pieces are the
// file skeleton, the kind templates, the fact [Coverage], the
// template vocabulary in [Funcs], the filename and name spelling of
// the spell package, the enum and throw lowering [Lower], the
// statement printer [Scaffold], the import-block renderer
// [Imports], and go/format as the finalising step.
//
// # Dependency position
//
// lang/go/backend imports the sdk's backend, emit, plugin, render
// and symbol facades, the module root for the language identity,
// lang/go/spell for filename and name spelling, the lowering,
// naming, scaffold, spellref and textfmt helpers of eidos-lang, and
// the Go stdlib, go/format and go/scanner among it.
package backend
