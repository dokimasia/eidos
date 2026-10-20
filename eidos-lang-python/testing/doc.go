// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the toolchain harness of Python. It provides the
// adapter that the assertion set of the kernel drives over generated
// Python.
//
// [New] returns the adapter of one target version of the policy
// python.version. It lays generated output out as a project in a scratch
// directory, with a pyrightconfig.json at the root that checks the project
// in the strict mode of pyright for the target version, unless the output
// has one there. It parses every module and stub file with the ast module
// of python3 under the grammar of the target version, type-checks the
// project with pyright, and runs the test modules, the files test_*.py,
// with the discovery of unittest. It asks pyright whether a class is
// assignable to a contract, which is satisfaction under the nominal
// subtyping of a class and the structural subtyping of a Protocol.
//
// Python has no parser in Go that reads Python 3, so the parse runs
// python3 too. Every check needs the toolchain, and a caller passes
// [go.dokimi.dev/eidos/sdk/toolchain.Require] before any. The check skips
// where the toolchain is absent locally, and fails where it is absent in
// CI. python3 runs in isolated mode, so neither the environment nor the
// directory of the project changes the modules that it imports.
//
// # Dependency position
//
// lang/python/testing imports the language root, the plugin, symbol and
// toolchain facades of the SDK, and the Go stdlib, os/exec among it. It
// runs python3 and pyright through the toolchain facade.
package testing
