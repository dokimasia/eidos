// Copyright ThesmOS B.V. 2026
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
// The inventory follows the node model, and no feature is ever removed
// from it, so totality covers every capability the model has had. The
// same expectations run over every language's tree. A contract defect
// then shows in the language the others do not share, while the
// contract can still change. The whole-composition acceptance runs and
// the end-to-end corpus benchmark belong in this module. Go's
// end-to-end fixtures run through the Go backend:
//
//   - the kernel's pipeline suite, over a stub generator and an audit
//     weaver;
//   - the kernel's workspace suite, over a plan scoped to svc that
//     doubles its interfaces and a plan scoped to admin that aliases
//     each double through the first plan's export;
//   - a scoped plan whose reader finds nothing outside its sources, a
//     workspace check that reads the doubles' record, and two
//     workspaces over sibling roots of one repository.
//
// # Dependency position
//
// conformance imports the kernel's two read-side kits,
// core/frontend/frontendtest and core/rules/rulestest, beside
// core/rules, core/store, core/node, core/directive, core/meta,
// core/plugin, core/symbol and the assert module. Its tests import the
// language satellites whose frontends exist. Go's end-to-end fixtures
// also import the Go backend, core/workspace, core/ledger,
// core/manifest and the kernel's pipeline and workspace kits,
// core/workspace/pipelinetest and core/workspace/workspacetest. No
// module imports conformance.
package conformance
