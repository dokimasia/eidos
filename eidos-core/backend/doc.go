// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package backend declares backends: [New] accumulates one
// target's write-side declaration — identity, comment syntax, and
// the language pieces the render pass varies in — and
// [Builder.Build] lowers it to the [plugin.Backend] and
// [plugin.Renderer] roles over a composed [render.Pass]. The kit
// adds nothing the roles do not state, so a kit backend and a
// hand-rolled pass over the same language return the same bytes.
//
// # The hub
//
// [Builder.Types] declares the target's spoke of the cross-language
// hub, which the built backend serves as [plugin.TypeSpeller], and
// [Builder.Policies] declares the lowering policies the spoke reads,
// served as [plugin.PolicyProvider]. A backend without a spoke refuses
// every shape of another language.
//
// # Failure semantics
//
// A declaration defect panics at [Builder.Build], before any run
// exists. A built backend declaring a settle seam through
// [Builder.Lower] or [Builder.Respell] refuses to render an
// unsettled store, as an error rather than wrong bytes.
//
// # Dependency position
//
// core/backend imports core/backend/render, core/emit,
// core/plugin, core/rules, core/symbol and the Go stdlib. Nothing
// beneath it imports it back.
package backend
