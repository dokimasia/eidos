// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the toolchain harness of TypeScript and the spelling
// adapter of its fixtures. It provides the adapter that the assertion set
// of the kernel drives over generated TypeScript, and the Language of the
// fixture builder of nodetest.
//
// [New] returns the adapter. It lays generated output out as a project in a
// scratch directory, and parses and type-checks the project with tsc. A
// project without a tsconfig.json at its root gets one that sets strict,
// noUnusedLocals, noUnusedParameters and exactOptionalPropertyTypes, so
// the output compiles under the strictest settings that a consumer
// commonly sets. It emits the project as CommonJS and
// runs every *.test.ts module with the test runner of node. It asks tsc
// whether a type is assignable to a contract, which is satisfaction under
// structural typing.
//
// TypeScript has no parser in Go, so the parse runs tsc too. Every check
// needs the toolchain, and a caller passes
// [go.dokimi.dev/eidos/sdk/toolchain.Require] before any. The check skips
// where the toolchain is absent locally, and fails where it is absent in
// CI.
//
// [Language] writes the compact spelling of a structural reference of a
// fixture, as the TypeScript frontend records the same type. Its test pins
// each form to the frontend.
//
// # Dependency position
//
// lang/typescript/testing imports the language root, the node, nodetest,
// toolchain and symbol facades of the SDK, and the Go stdlib, os/exec
// among it. It runs tsc and node through the toolchain facade.
package testing
