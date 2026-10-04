// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package golang declares Go's conformance fixtures, and its tests grade
// Go against them: the corpus entry the shared feature inventory checks,
// the end-to-end fixture the kernel's pipeline suite runs, and the
// two-plan fixture the kernel's workspace suite runs.
//
// # Fixtures
//
//   - [Corpus] is Go's entry: the Go frontend and rules over a tree that
//     spells every inventory feature but method overloads, which Go does
//     not have.
//   - [ComposeStubs] composes the end-to-end fixture: stubgen doubles an
//     interface under //+acme:stub tag=test, an audit weaver calls audit
//     first in each method of the double, and the Go backend writes the
//     double beside its source.
//   - [ComposeWorkspace] and [WorkspacePlans] compose the workspace
//     fixture: [StubsPlan], scoped to svc, doubles the interfaces under
//     the stub directive, and the registry plan, scoped to admin, aliases
//     each double through the export of [StubsPlan].
//   - [Stubbed] is the workspace check that reads the record of
//     [StubsPlan], and reports each interface under the stub directive
//     without a double under [Unstubbed].
//
// # Dependency position
//
// golang imports the conformance package, core/frontend/frontendtest,
// core/symbol, core/workspace and core/ledger, the Go satellite's root,
// frontend, rules and backend packages, and the SDK facade. Its tests
// also import the assert module, core/frontend/load, core/manifest and
// the kernel's pipeline and workspace kits, core/workspace/pipelinetest
// and core/workspace/workspacetest. Beside its own tests, the
// conformance package's level checks import golang for Go's entry.
package golang
