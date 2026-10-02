// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
)

// packageHash is the digest of "package p\n", computed outside this
// package, so a case pins the bytes and does not mirror the code.
const packageHash = "sha256:0ad6261536f6380b14ade1a508ac911b8c48230731746e0c76abd26bf3e3a15d"

// every returns one constructor per shipped sink, so the rules
// they share are checked against each of them, not against
// whichever one was convenient.
func every(t *testing.T) map[string]func() output.Sink {
	t.Helper()

	return map[string]func() output.Sink{
		"mem": func() output.Sink { return output.NewMem() },
		"disk": func() output.Sink {
			d, err := output.NewDisk(t.TempDir(), diskBrand)
			assert.NoError(t, err, "the fixture root opens")
			return d
		},
	}
}

// Staging is what every sink shares: what it refuses to stage, and
// that one staging serves one commit. How the bytes are written to
// the destination is each sink's own.
func TestSink(t *testing.T) {
	t.Parallel()

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		paths := []struct {
			name string
			path string
		}{
			{name: "returns an error for the empty path", path: ""},
			{name: "returns an error for an absolute path", path: "/etc/passwd"},
			{name: "returns an error for a path that climbs out of the root", path: "../outside.go"},
			{name: "returns an error for a path that climbs out part-way", path: "svc/../../outside.go"},
			{name: "returns an error for the root itself", path: "."},
			{name: "returns an error for a path with a trailing slash", path: "svc/"},
			{name: "returns an error for a path with a backslash", path: `svc\store.go`},
			{name: "returns an error for a path with the reserved staging suffix", path: "svc/store.go.stage"},
		}
		pairs := []struct {
			name          string
			first, second string
		}{
			{
				name:  "returns an error for a file where a directory is needed",
				first: "svc/store.go", second: "svc/store.go/row.go",
			},
			{
				name:  "returns an error for a directory where a file is staged",
				first: "svc/store.go", second: "svc",
			},
			{
				name:  "returns an error for two names that differ only in case",
				first: "svc/store.go", second: "svc/Store.go",
			},
			{
				name:  "returns an error for a file whose name differs from a needed directory only in case",
				first: "gen/a.go", second: "Gen",
			},
			{
				name:  "returns an error for a path under a file whose name differs only in case",
				first: "Gen", second: "gen/a.go",
			},
		}
		for name, open := range every(t) {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				for _, tt := range paths {
					t.Run(tt.name, func(t *testing.T) {
						t.Parallel()

						assert.HasError(t, open().Write(tt.path, []byte("x\n")), "the staging fails")
					})
				}

				for _, tt := range pairs {
					t.Run(tt.name, func(t *testing.T) {
						t.Parallel()

						s := open()
						assert.NoError(t, s.Write(tt.first, []byte("a\n")), "the first path stages")
						assert.HasError(t, s.Write(tt.second, []byte("b\n")), "the second path fails")
					})
				}

				t.Run("returns an error for one path staged twice", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Write("svc/store.go", []byte("first\n")), "the first staging succeeds")
					assert.HasError(t, s.Write("svc/store.go", []byte("second\n")), "the second staging fails")
				})

				t.Run("returns an error for a path staged for removal", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Delete("svc/store.go"), "the removal stages")
					assert.HasError(t, s.Write("svc/store.go", []byte("x\n")), "the write fails")
				})

				t.Run("returns an error for a write after the preparation", func(t *testing.T) {
					t.Parallel()

					s := open()
					_, err := s.Prepare()
					assert.NoError(t, err, "the staging prepares")
					assert.HasError(t, s.Write("svc/store.go", []byte("x\n")), "a prepared staging is closed")
				})

				t.Run("stages two files in one directory", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Write("svc/store.go", []byte("a\n")), "the first path stages")
					assert.NoError(t, s.Write("svc/store_test.go", []byte("b\n")), "the sibling stages")
				})

				t.Run("returns ErrFinished for a write after the commit", func(t *testing.T) {
					t.Parallel()

					s := open()
					_, err := s.Commit()
					assert.NoError(t, err, "the commit succeeds")
					assert.ErrorIs(t, s.Write("b.go", []byte("b\n")), output.ErrFinished, "a sink serves one staging")
				})

				t.Run("returns ErrFinished for a write after the discard", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Discard(), "the discard succeeds")
					assert.ErrorIs(t, s.Write("b.go", []byte("b\n")), output.ErrFinished, "a sink serves one staging")
				})
			})
		}
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()

		for name, open := range every(t) {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				t.Run("returns an error for a path that climbs out of the root", func(t *testing.T) {
					t.Parallel()

					assert.HasError(t, open().Delete("../outside.go"), "the removal fails")
				})

				t.Run("returns an error for a path staged for writing", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Write("svc/store.go", []byte("x\n")), "the write stages")
					assert.HasError(t, s.Delete("svc/store.go"), "the removal fails")
				})

				t.Run("returns an error for one path staged for removal twice", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Delete("svc/store.go"), "the first removal stages")
					assert.HasError(t, s.Delete("svc/store.go"), "the second removal fails")
				})

				t.Run("returns an error for a removal after the preparation", func(t *testing.T) {
					t.Parallel()

					s := open()
					_, err := s.Prepare()
					assert.NoError(t, err, "the staging prepares")
					assert.HasError(t, s.Delete("svc/store.go"), "a prepared staging is closed")
				})

				t.Run("returns ErrFinished for a removal after the commit", func(t *testing.T) {
					t.Parallel()

					s := open()
					_, err := s.Commit()
					assert.NoError(t, err, "the commit succeeds")
					assert.ErrorIs(t, s.Delete("b.go"), output.ErrFinished, "a sink serves one staging")
				})
			})
		}
	})

	t.Run("Prepare", func(t *testing.T) {
		t.Parallel()

		for name, open := range every(t) {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				t.Run("returns every staged path in path order", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Write("svc/store.go", []byte("package p\n")), "the file stages")
					assert.NoError(t, s.Delete("a.go"), "the removal stages")
					got, err := s.Prepare()
					assert.NoError(t, err, "the staging prepares")
					assert.Equal(t, got, []output.Change{
						{Path: "a.go", Action: output.ActionUnchanged, Found: output.FoundNothing},
						{
							Path: "svc/store.go", Action: output.ActionCreated,
							Found: output.FoundNothing, Hash: packageHash,
						},
					}, "a removal of nothing, then a new file with its digest")
				})

				t.Run("returns an error for a second preparation", func(t *testing.T) {
					t.Parallel()

					s := open()
					_, err := s.Prepare()
					assert.NoError(t, err, "the first preparation succeeds")
					_, err = s.Prepare()
					assert.HasError(t, err, "a sink prepares once")
				})

				t.Run("returns ErrFinished after the commit", func(t *testing.T) {
					t.Parallel()

					s := open()
					_, err := s.Commit()
					assert.NoError(t, err, "the commit succeeds")
					_, err = s.Prepare()
					assert.ErrorIs(t, err, output.ErrFinished, "a sink serves one staging")
				})
			})
		}
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		for name, open := range every(t) {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				t.Run("records every staged path in path order", func(t *testing.T) {
					t.Parallel()

					s := open()
					for _, p := range []string{"svc/store.go", "a.go", "svc/api.go"} {
						assert.NoError(t, s.Write(p, []byte("package p\n")), "the fixture stages "+p)
					}
					got, err := s.Commit()
					assert.NoError(t, err, "the commit succeeds")
					assert.Equal(t, []string{got[0].Path, got[1].Path, got[2].Path},
						[]string{"a.go", "svc/api.go", "svc/store.go"},
						"the records are sorted by path")
					for _, w := range got {
						assert.Equal(t, w.Action, output.ActionCreated, "nothing existed before")
						assert.Equal(t, w.Hash, packageHash, "the hash is the digest of the bytes as written")
					}
				})

				t.Run("records nothing for a removal of a path that contains nothing", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Delete("svc/gone.go"), "the removal stages")
					got, err := s.Commit()
					assert.NoError(t, err, "the commit succeeds")
					assert.Empty(t, got, "nothing was removed")
				})

				t.Run("returns ErrFinished for a second commit", func(t *testing.T) {
					t.Parallel()

					s := open()
					_, err := s.Commit()
					assert.NoError(t, err, "the first commit succeeds")
					_, err = s.Commit()
					assert.ErrorIs(t, err, output.ErrFinished, "a sink commits once")
				})
			})
		}
	})

	t.Run("Action.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give output.Action
			want string
		}{
			{name: "returns created for ActionCreated", give: output.ActionCreated, want: "created"},
			{name: "returns updated for ActionUpdated", give: output.ActionUpdated, want: "updated"},
			{name: "returns unchanged for ActionUnchanged", give: output.ActionUnchanged, want: "unchanged"},
			{name: "returns deleted for ActionDeleted", give: output.ActionDeleted, want: "deleted"},
			{name: "returns the number of an undeclared action", give: output.Action(7), want: "Action(7)"},
			{name: "returns every digit of an undeclared action's number", give: output.Action(12), want: "Action(12)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("Found.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give output.Found
			want string
		}{
			{name: "returns nothing for FoundNothing", give: output.FoundNothing, want: "nothing"},
			{name: "returns same for FoundSame", give: output.FoundSame, want: "same"},
			{name: "returns intact for FoundIntact", give: output.FoundIntact, want: "intact"},
			{name: "returns drifted for FoundDrifted", give: output.FoundDrifted, want: "drifted"},
			{name: "returns foreign for FoundForeign", give: output.FoundForeign, want: "foreign"},
			{name: "returns the number of the zero verdict", give: output.Found(0), want: "Found(0)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})
}
