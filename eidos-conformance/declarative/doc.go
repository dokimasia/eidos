// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package declarative declares the conformance fixture of the declarative
// plugin host, and its tests run a declarative plugin end to end.
//
// The fixture is a Go tree under testdata. Its config file adds the plugin
// directory plugins/acme-audit to the two plans of [Compose]. The plugin
// implements each interface under its directive, mirrors each function
// under it, and writes a Go test of each implementation through the file
// template of its family test.
//
//   - The plan go-audit writes the implementation of Ledger in
//     svc/ledger_audit.go, the test of the implementation in
//     svc/ledger_audit_test.go, and the mirror of Transfer in
//     ops/transfer_audit.go.
//   - The plan ts-audit writes the mirror of Transfer in
//     ops/transfer.audit.ts. It reads no interface, because a TypeScript
//     implementation of Ledger would refer to a Go declaration that the plan
//     does not emit. No run of the fixture renders method.tmpl or test.tmpl
//     of the TypeScript tree for that reason.
//
// # Dependency position
//
// declarative imports core/workspace, the root, frontend, rules and backend
// packages of the Go satellite, the root and backend packages of the
// TypeScript satellite, and the output package of the SDK facade. Its tests
// also import cli, the declarative module, core/frontend/load, the assert
// module, the toolchain harness of the Go satellite, and the diag,
// directive, node, plugin, plugintest, position, rules, symbol and
// toolchain packages of the SDK facade.
package declarative
