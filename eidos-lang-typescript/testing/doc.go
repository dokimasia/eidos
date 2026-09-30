// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the TypeScript language's toolchain harness: the
// adapter the kernel's assertion set drives over generated
// TypeScript.
//
// [New] returns the adapter. It lays generated output out as a
// project in a scratch directory with a strict tsconfig.json, parses
// and type-checks it with tsc, emits it as CommonJS and runs every
// *.test.ts module with node's test runner, and asks tsc whether a
// type is assignable to a contract, which is satisfaction under
// structural typing.
//
// TypeScript has no parser in Go, so the parse runs tsc too. Every
// check needs the toolchain, and a caller passes
// [go.dokimi.dev/eidos/sdk/toolchain.Require] before any: absent
// locally, the check skips, and absent in CI, it fails.
//
// # Dependency position
//
// lang/typescript/testing imports the language root, the sdk's
// toolchain and symbol facades, and the Go stdlib, os/exec among it.
// It is a harness and runs tsc and node on purpose.
package testing
