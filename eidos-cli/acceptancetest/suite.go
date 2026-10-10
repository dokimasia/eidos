// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package acceptancetest

import (
	"context"
	"io/fs"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
)

// Fixture is the binary of a consumer, and a tree that the binary runs
// over.
type Fixture struct {
	// Main is the import path of the main package of the binary. The go
	// command resolves it in the module of the package under test.
	Main string
	// Brand is the brand of the composition of the binary.
	Brand output.Brand
	// Prefix is the arguments before the name of a kernel command, such as
	// []string{"gen"} for a host that mounts the kernel commands under a
	// group. It is empty for a host that mounts them at the root.
	Prefix []string
	// Tree is a workspace tree with the config file of the brand at its
	// root, such as .acme.yaml. A check copies the tree into each directory
	// in which it runs the binary.
	Tree fs.FS
	// Panic is the arguments that run a command of the binary that panics.
	// The binary adds the command for the suite.
	Panic []string
	// Fail edits the copy of the tree in root, so that the next run reports
	// an Error.
	Fail func(root string) error
	// Compile compiles the output that a run generated in root.
	Compile func(ctx context.Context, root string) error
}

// RunAcceptanceSuite builds the binary of the fixture once, and checks it
// against the contract of the command line. It runs [AssertStatuses],
// [AssertNames], [AssertPanic], [AssertDiscovery], [AssertJSON],
// [AssertIdempotent], [AssertCompiles], [AssertLists] and [AssertLocked],
// each in a parallel subtest over a temporary directory of its own. It
// stops the test when the fixture has no main package, and when the main
// package does not compile.
func RunAcceptanceSuite(t *testing.T, f Fixture) {
	t.Helper()

	assert.NotEqual(t, f.Main, "", "the fixture has the main package of its binary")
	bin := Build(t, f.Main)
	checks := []struct {
		name  string
		check func(assert.TB, Fixture, string, string)
	}{
		{name: "exits with the statuses of the contract", check: AssertStatuses},
		{name: "runs each kernel command under its name", check: AssertNames},
		{name: "exits with status 2 for a panic", check: AssertPanic},
		{name: "finds the config file of the root", check: AssertDiscovery},
		{name: "writes JSON lines alone under --format=json", check: AssertJSON},
		{name: "rewrites no file on a second run", check: AssertIdempotent},
		{name: "compiles the generated output", check: AssertCompiles},
		{name: "runs each member of a list under a state directory of its own", check: AssertLists},
		{name: "exits with status 1 while another process has the lock", check: AssertLocked},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.check(t, f, bin, t.TempDir())
		})
	}
}
