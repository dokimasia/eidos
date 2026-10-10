// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package golang declares the conformance fixtures of Go, and its tests
// check Go against them.
//
// The shared feature inventory checks the corpus entry of Go. The pipeline
// suite of the kernel runs the end-to-end fixture, and the workspace suite
// of the kernel runs the fixture of two plans. The acceptance suite of the
// command line runs two binaries over the fixture of two plans.
//
// # Fixtures
//
//   - [Corpus] is the entry of Go. It has the Go frontend and rules over a
//     tree that spells every feature of the inventory except method
//     overloads, which Go does not have.
//   - [ComposeStubs] composes the end-to-end fixture. stubgen doubles an
//     interface under //+acme:stub tag=test, an audit weaver calls audit
//     first in each method of the double, and the Go backend writes the
//     double beside its source.
//   - [ComposeWorkspace] and [WorkspacePlans] compose the workspace
//     fixture. [StubsPlan], scoped to svc, doubles the interfaces under the
//     stub directive. The registry plan, scoped to admin, aliases each
//     double through the export of [StubsPlan].
//   - [EditWorkspace] is the edit of the workspace fixture. It renames the
//     parameter of Get in the stubbed interface, which changes the double
//     and leaves the export of [StubsPlan] unchanged.
//   - [Stubbed] is the workspace check that reads the record of
//     [StubsPlan]. It reports each interface under the stub directive
//     without a double under [Unstubbed].
//   - [ComposeAcceptance] composes the binaries of the acceptance fixture
//     from the plans of the workspace fixture and [Stubbed]. [Crash] is a
//     command of the binaries that panics, and [CompileAcceptance] compiles
//     the output of a run with go build.
//   - [ComposeHub] and [HubPlans] compose the cross-language fixture. One
//     run over a Go workspace writes the double of Store in
//     svc/store_stub_test.go and a TypeScript client of Session and Store
//     in svc/store.ts. The client generator translates every type through
//     the Emitter. [EditHub] renames the parameter of Get, which changes
//     both files. [StoreClient] writes the client of each interface alone,
//     so the client of Store refers to a Session that the plan does not
//     emit.
//
// # Dependency position
//
// golang imports cli, the conformance package, core/frontend/frontendtest,
// core/symbol, core/workspace, core/layout and core/ledger, the root,
// frontend, rules and backend packages of the Go satellite, the root and
// backend packages of the TypeScript satellite, and the SDK facade. Its
// tests also import the assert module, cli/acceptancetest,
// core/frontend/load, core/manifest, sdk/frontendtest, sdk/toolchain, the
// toolchain adapter of the TypeScript satellite, and the pipeline and
// workspace kits of the kernel, core/workspace/pipelinetest and
// core/workspace/workspacetest. The level checks of the conformance
// package import golang for the entry of Go. The binaries under
// testdata/acceptance import cli and golang.
package golang
