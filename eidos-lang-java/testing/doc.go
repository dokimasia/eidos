// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the toolchain harness of Java and the spelling adapter
// of its fixtures. It provides the adapter that the assertion set of the
// kernel drives over generated Java, and the Language of the fixture
// builder of nodetest.
//
// [New] returns the adapter. It lays generated output out as a source tree
// in a scratch directory and parses the tree through the parser of javac.
// It type-checks the tree by compiling it with javac, runs every test class
// through its main method, and asks javac whether a type is assignable to a
// contract.
//
// The JDK includes no test framework, so a test is a top-level class whose
// simple name ends in Test and which declares a main method. A return
// passes and a throw fails.
//
// Java has no parser in Go, so the parse runs the toolchain too. Every
// check needs it, and a caller passes
// [go.dokimi.dev/eidos/sdk/toolchain.Require] before any. The check skips
// where the toolchain is absent locally, and fails where it is absent in
// CI.
//
// [Language] writes the compact spelling of a structural reference of a
// fixture, as the Java frontend records the same type. Its test pins each
// form to the frontend.
//
// # Dependency position
//
// lang/java/testing imports the language root, the node, nodetest,
// toolchain and symbol facades of the SDK, and the Go stdlib. It runs javac
// and java through the toolchain facade.
package testing
