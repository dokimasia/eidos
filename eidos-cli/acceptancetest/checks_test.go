// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package acceptancetest_test

import (
	"context"
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli/acceptancetest"
)

// errEdit is the error of a Fail that does not edit the tree.
var errEdit = errors.New("acceptancetest_test: the tree does not take the edit")

// The checks exist to catch binaries that break the contract. Each check
// runs over a host that breaks one rule of it, and over fixtures that lack
// what the check needs, and each test asserts the failure of the check.
func TestChecks(t *testing.T) {
	t.Parallel()

	bin := acceptancetest.Build(t, hostMain)
	remapped := acceptancetest.Build(t, remappedMain)

	t.Run("AssertStatuses", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a host that exits 1 for a usage error", func(t *testing.T) {
			t.Parallel()

			assert.HasPrefix(t, rejected(t, acceptancetest.AssertStatuses, fixture(), remapped),
				"run exits 64 for a flag that no command defines", "the check fails at the status of the flag")
		})

		t.Run("rejects a Fail after which a run succeeds", func(t *testing.T) {
			t.Parallel()

			f := fixture()
			f.Fail = func(string) error { return nil }
			assert.HasPrefix(t, rejected(t, acceptancetest.AssertStatuses, f, bin),
				"run exits 1 after the Fail of the fixture", "the check fails at the status after the Fail")
		})

		t.Run("rejects a Fail that returns an error", func(t *testing.T) {
			t.Parallel()

			f := fixture()
			f.Fail = func(string) error { return errEdit }
			assert.Equal(t, rejected(t, acceptancetest.AssertStatuses, f, bin),
				"the Fail of the fixture edits the tree", "the check fails at the Fail")
		})

		fixtures := []struct {
			name string
			edit func(f *acceptancetest.Fixture)
			want string
		}{
			{
				name: "rejects a fixture without a Fail",
				edit: func(f *acceptancetest.Fixture) { f.Fail = nil },
				want: "the fixture has a Fail",
			},
			{
				name: "rejects a fixture without a valid brand",
				edit: func(f *acceptancetest.Fixture) { f.Brand = "Acme" },
				want: "the fixture has a valid brand",
			},
			{
				name: "rejects a fixture without a tree",
				edit: func(f *acceptancetest.Fixture) { f.Tree = nil },
				want: "the fixture has a tree",
			},
		}
		for _, tt := range fixtures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := fixture()
				tt.edit(&f)
				assert.Equal(t, rejected(t, acceptancetest.AssertStatuses, f, bin), tt.want,
					"the check fails at what the fixture lacks")
			})
		}
	})

	t.Run("AssertNames", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a host that mounts run under another name", func(t *testing.T) {
			t.Parallel()

			renamed := acceptancetest.Build(t, renamedMain)
			assert.HasPrefix(t, rejected(t, acceptancetest.AssertNames, fixture(), renamed), "run -h exits 0",
				"the check fails at the name run")
		})
	})

	t.Run("AssertPanic", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a host that recovers a panic", func(t *testing.T) {
			t.Parallel()

			recovering := acceptancetest.Build(t, recoveringMain)
			assert.HasPrefix(t, rejected(t, acceptancetest.AssertPanic, fixture(), recovering),
				"the command that panics exits 2", "the check fails at the status of the panic")
		})

		t.Run("rejects a fixture without the arguments of a command that panics", func(t *testing.T) {
			t.Parallel()

			f := fixture()
			f.Panic = nil
			assert.Equal(t, rejected(t, acceptancetest.AssertPanic, f, bin),
				"the fixture has the arguments of a command that panics",
				"the check fails at what the fixture lacks")
		})
	})

	t.Run("AssertDiscovery", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a host that passes the config file of the working directory", func(t *testing.T) {
			t.Parallel()

			pinned := acceptancetest.Build(t, pinnedMain)
			assert.HasPrefix(t, rejected(t, acceptancetest.AssertDiscovery, fixture(), pinned),
				"a run from a directory below the root exits 0", "the check fails at the run below the root")
		})
	})

	t.Run("AssertJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a host that writes a banner to standard output", func(t *testing.T) {
			t.Parallel()

			banner := acceptancetest.Build(t, bannerMain)
			assert.HasPrefix(t, rejected(t, acceptancetest.AssertJSON, fixture(), banner),
				"each line of run is a JSON object", "the check fails at the banner")
		})
	})

	t.Run("AssertIdempotent", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a binary that writes other bytes on every run", func(t *testing.T) {
			t.Parallel()

			clock := acceptancetest.Build(t, clockMain)
			assert.Equal(t, rejected(t, acceptancetest.AssertIdempotent, fixture(), clock),
				"the second run leaves "+storeDir+"/"+mirrorName+" unchanged",
				"the check fails at the rewritten mirror")
		})
	})

	t.Run("AssertCompiles", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects output that does not compile", func(t *testing.T) {
			t.Parallel()

			f := fixture()
			f.Compile = func(context.Context, string) error { return errUncompiled }
			assert.Equal(t, rejected(t, acceptancetest.AssertCompiles, f, bin), "the generated output compiles",
				"the check fails at the compilation")
		})

		t.Run("rejects a fixture without a Compile", func(t *testing.T) {
			t.Parallel()

			f := fixture()
			f.Compile = nil
			assert.Equal(t, rejected(t, acceptancetest.AssertCompiles, f, bin),
				"the fixture has a Compile", "the check fails at what the fixture lacks")
		})
	})

	t.Run("AssertLists", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a host that exits 1 for a config error", func(t *testing.T) {
			t.Parallel()

			assert.HasPrefix(t, rejected(t, acceptancetest.AssertLists, fixture(), remapped),
				"a run over a list whose roots nest exits 64", "the check fails at the status of the nested list")
		})
	})

	t.Run("AssertLocked", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a host that discards standard error", func(t *testing.T) {
			t.Parallel()

			muted := acceptancetest.Build(t, mutedMain)
			assert.Equal(t, rejected(t, acceptancetest.AssertLocked, fixture(), muted),
				"the output of the run has the process ID of the holder of the lock",
				"the check fails at the process ID")
		})
	})
}

// rejected runs check over the binary bin and the fixture f in a new
// temporary directory under assert.Rejects, and returns the contract of the
// first failure of the check. It stops the test when the check passes.
func rejected(
	t *testing.T, check func(assert.TB, acceptancetest.Fixture, string, string), f acceptancetest.Fixture, bin string,
) string {
	t.Helper()

	root := t.TempDir()
	records := assert.Rejects(t, "the check rejects the binary", func(tb assert.TB) { check(tb, f, bin, root) })
	assert.NotEmpty(t, records, "the check fails at an assertion")
	return records[0].Contract
}
