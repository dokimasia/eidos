// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package nodetest builds the declaration graph of a test through the
// kernel's load.
//
// A test writes its declarations as literals of the node model without
// identities, places them in files with [Builder.File], and links them
// with [Builder.Ref]. [Builder.Build] loads the files through the kernel's
// load, so every identity, host and target of the fixture is the one that
// the load assigns. After Build, a test reads the identity of a
// declaration from the declaration, and passes the returned packages to
// the Load method of a plugintest fixture or to the AddPackage method of a
// store graph.
//
// # Spelling
//
// A [Language] writes the source spelling of each structural reference
// whose spelling the test leaves empty. The testing package of each
// satellite provides the Language of its language, and [Neutral] serves a
// language without a satellite. [Optional], [List], [Map] and the other
// constructors build a reference of their form without a spelling.
//
// # Failures
//
// Build fails the test through an assertion, so assert.Rejects returns a
// record of each failure. Build fails for a fixture that the load cannot
// build as the test wrote it: a file path that two files have, a
// declaration or a reference in two places, a reference whose target the
// load does not resolve, a form that the language does not spell, and an
// error, a finding or a panic of the load.
//
// # Allocation contract
//
// The package states no allocation contract. Build runs once for each
// fixture, on the cold path of a test.
//
// # Dependency position
//
// nodetest imports the kernel's node, symbol, position, plugin, diag,
// output and store packages, the load of the frontend package, and
// go.dokimi.dev/assert.
package nodetest
