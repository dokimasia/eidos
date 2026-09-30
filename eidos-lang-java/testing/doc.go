// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package testing is the Java language's toolchain harness: the
// adapter the kernel's assertion set drives over generated Java.
//
// [New] returns the adapter. It lays generated output out as a source
// tree in a scratch directory, parses it through javac's own parser,
// type-checks it by compiling it with javac, runs every test class
// through its main method, and asks javac whether a type is
// assignable to a contract.
//
// The JDK includes no test framework, so a test is a top-level class
// whose simple name ends in Test and which declares a main method: a
// return passes and a throw fails.
//
// Java has no parser in Go, so the parse runs the toolchain too.
// Every check needs it, and a caller passes
// [go.dokimi.dev/eidos/sdk/toolchain.Require] before any: absent
// locally, the check skips, and absent in CI, it fails.
//
// # Dependency position
//
// lang/java/testing imports the language root, the sdk's toolchain
// and symbol facades, and the Go stdlib, os/exec among it. It is a
// harness and runs javac and java on purpose.
package testing
