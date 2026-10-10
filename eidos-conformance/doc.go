// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package conformance checks every language against one corpus: a
// neutral feature inventory, a tree per language that spells each
// feature the language's own way, and one set of expectations that the
// loaded graph meets whichever language parsed the tree.
//
// [Run] checks one language's entry in four steps:
//
//   - coverage totality: a verdict per inventory feature, so a language
//     cannot leave a capability without one;
//   - the frontend conformance suite over the whole tree;
//   - each covered feature's expectations against one load;
//   - each refused feature's absence from the tree, because a spelling
//     present under a refusal is a contradiction.
//
// The corpus convention places a feature's files under f/<id> and
// loads them under that package path. A language whose canonical paths
// differ states them through its own derivation.
//
// The inventory follows the node model and keeps every feature that it
// has had, so totality covers every capability that the model has had.
// The same expectations run over every language's tree. A contract
// defect then shows in the language that the others do not share, while
// the contract can still change.
//
// # Languages
//
// Each language's entry is a package under lang, at its satellite's
// import path below conformance: the entry of go.dokimi.dev/eidos/lang/java
// is go.dokimi.dev/eidos/conformance/lang/java. The packages are lang/go,
// lang/java, lang/protobuf, lang/rust and lang/typescript. Each declares
// its language's corpus entry, and its tests run [Run] over the tree
// under its testdata. lang/go also declares Go's whole-composition
// fixtures, the cross-language fixture of a Go and a TypeScript plan
// among them, and its tests run the kernel's pipeline and workspace
// suites over them and benchmark a load of this repository.
// lang/typescript declares the TypeScript pipeline fixture and a Go plan
// over TypeScript source, and its tests run the pipeline suite and the
// TypeScript toolchain over them. lang/protobuf declares the service
// fixture, a Go server plan and a TypeScript client plan over a proto
// service, and its tests run the workspace and warm suites over the
// service's schema in each protobuf version. The package matrix renders
// the support matrix of the satellites from their entries.
//
// # Dependency position
//
// conformance imports the kernel's two read-side kits,
// core/frontend/frontendtest and core/rules/rulestest, beside
// core/rules, core/store, core/node, core/directive, core/meta,
// core/plugin, core/symbol and the assert module. Its tests import
// conformance/lang/go and the Go satellite's root and frontend, for the
// level checks over Go's corpus. The packages under lang import
// conformance and their satellites. No module imports conformance.
package conformance
