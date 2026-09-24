// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package conformance holds every language to one corpus: a
// neutral feature inventory, a per-language tree spelling each
// feature the language's own way, and one set of expectations the
// loaded graph must meet whichever language answered.
//
// [Run] drives one language's entry: coverage totality first — a
// verdict per inventory feature, so a language cannot go silent on
// a capability — then the frontend conformance suite over the
// whole tree, then each covered feature's expectations against one
// load, and each refused feature's absence from the tree, because
// a spelling present under a refusal is a contradiction. The
// corpus convention places a feature's files under f/<id> and
// loads them under that package path; a language whose canonical
// paths differ says so through its own derivation.
//
// The inventory grows with the wave, the node model its source; a
// feature is never removed, because totality is what keeps the
// growth honest. Cross-language equivalence is this package's
// point: the same expectations run over every language's tree, so
// a contract defect arrives through the language the others do not
// share while the contract is still free to change. The
// whole-composition acceptance runs and the end-to-end corpus
// bench live here too, as the languages arrive.
//
// # Dependency position
//
// conformance imports the kernel's two read-side kits,
// core/frontend/frontendtest and core/rules/rulestest, beside
// core/rules, core/store, core/node, core/directive, core/meta,
// core/plugin, core/symbol and the assert module. Its tests import
// the language satellites whose frontends exist. Nothing imports it
// back: it is the module that drives the others.
package conformance
