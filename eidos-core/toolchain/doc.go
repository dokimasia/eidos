// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package toolchain holds the skeleton every language's toolchain
// harness is built on: the generated-output fixture, the assertion
// set, and the small [Adapter] a satellite implements over its own
// compiler.
//
// The assertions and their failure wording live here, once. A
// satellite states how to lay its language out, how to parse it,
// how to type-check it, how to run its tests and how to ask whether
// a type satisfies a contract, and gets the whole set. Without a
// shared skeleton every language grows its own assertion vocabulary
// and they drift apart by hand.
//
// [RunToolchainSuite] is the floor a satellite runs: the generated
// output parses, type-checks and its tests pass. A satellite adds
// assertions of its own on top, exported from its own harness under
// the same naming scheme, for a check that means something under
// one toolchain alone.
//
// A toolchain the machine does not have is a skip locally and a
// failure in CI, so a regression cannot hide behind a missing
// compiler. [RequiredInCI] decides which, and the individual
// assertions never skip: a caller reaching for one has already
// decided the toolchain is there.
//
// # Dependency position
//
// core/toolchain imports core/symbol and the Go stdlib, the testing
// package among it. It runs no compiler itself, which is what keeps
// the kernel free of every language's tooling.
package toolchain
