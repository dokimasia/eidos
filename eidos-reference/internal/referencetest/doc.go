// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package referencetest builds the fixtures that the tests of the reference
// plugins run over.
//
// [New] loads a workspace tree with the Go and the TypeScript frontends
// under [Brand], as a run of the composition loads it. It returns a
// [Fixture] over the packages that a [Spec] lists. The fixture has the
// rules of both languages, the target of the plan, the keys and the
// classification stamps of both languages, and the keys of the plugins of
// the spec. It validates the carriers of the tree against the directives
// of those plugins, as a run validates them before the first phase. The
// annotators of the spec stamp the fixture before the plugin under test
// runs, and the units of the hosts of the spec are seeded into it. So a
// plugin of the ensemble runs over a fixture as it runs in the
// composition, after the plugins that it builds on.
//
// [Tree] is the workspace tree of the module, which the golden files of
// the composition describe. A plugin package keeps a tree of its own under
// testdata for the cases that the workspace tree does not contain.
//
// # Dependency position
//
// The package imports core/frontend/load, the root, frontend and rules
// packages of the Go satellite, the frontend and rules packages of the
// TypeScript satellite, the diag, directive, meta, node, output, plugin,
// plugintest, rules, store and symbol packages of the SDK facade, the
// assert module and the Go stdlib. It is the one package of a test of a
// plugin that imports the kernel, because the facade does not re-export
// the load.
package referencetest
