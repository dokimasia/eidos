// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
)

// every returns one constructor per shipped sink, so the laws
// they share are checked against each of them rather than against
// whichever one was convenient.
func every(t *testing.T) map[string]func() output.Sink {
	t.Helper()

	return map[string]func() output.Sink{
		"mem": func() output.Sink { return output.NewMem() },
		"disk": func() output.Sink {
			d, err := output.NewDisk(t.TempDir())
			assert.NoError(t, err, "the fixture root opens")
			return d
		},
	}
}

// Staging is what every sink shares: what it refuses to stage, and
// that one staging serves one commit. The behaviour after the
// bytes land is each sink's own.
func TestSink(t *testing.T) {
	t.Parallel()

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a path no sink may take", func(t *testing.T) {
			t.Parallel()

			paths := []struct {
				name string
				path string
				why  string
			}{
				{"empty", "", "a file needs a path"},
				{"absolute", "/etc/passwd", "a staged path is workspace-relative"},
				{"parent", "../outside.go", "a path may not climb out of the root"},
				{"interior parent", "svc/../../outside.go", "nor climb out part-way"},
				{"the root itself", ".", "a directory is not a file"},
				{"trailing slash", "svc/", "nor is a directory with a slash"},
				{"backslash", `svc\store.go`, "paths are slash-separated on every platform"},
				{"the reserved suffix", "svc/store.go.stage", "the commit stages through it"},
			}
			for name, open := range every(t) {
				for _, tt := range paths {
					t.Run(name+"/"+tt.name, func(t *testing.T) {
						t.Parallel()
						assert.HasError(t, open().Write(tt.path, []byte("x\n")), tt.why)
					})
				}
			}
		})

		t.Run("refuses one path staged twice", func(t *testing.T) {
			t.Parallel()

			for name, open := range every(t) {
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Write("svc/store.go", []byte("first\n")),
						"the first staging takes")
					assert.HasError(t, s.Write("svc/store.go", []byte("second\n")),
						"one sink writes each path once, so the second refuses "+
							"rather than deciding which wins")
				})
			}
		})

		t.Run("refuses paths that cannot coexist on a filesystem", func(t *testing.T) {
			t.Parallel()

			pairs := []struct {
				name          string
				first, second string
			}{
				{"a file where a directory is needed", "svc/store.go", "svc/store.go/row.go"},
				{"a directory where a file is staged", "svc/store.go", "svc"},
				{"two names differing only in case", "svc/store.go", "svc/Store.go"},
			}
			for name, open := range every(t) {
				for _, tt := range pairs {
					t.Run(name+"/"+tt.name, func(t *testing.T) {
						t.Parallel()

						s := open()
						assert.NoError(t, s.Write(tt.first, []byte("a\n")), "the first path stages")
						assert.HasError(t, s.Write(tt.second, []byte("b\n")),
							"the second cannot exist beside it on a filesystem")
					})
				}
				t.Run(name+"/two files in one directory", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Write("svc/store.go", []byte("a\n")), "the first path stages")
					assert.NoError(t, s.Write("svc/store_test.go", []byte("b\n")),
						"and a sibling beside it")
				})
			}
		})

		t.Run("refuses everything after the sink finished", func(t *testing.T) {
			t.Parallel()

			for name, open := range every(t) {
				t.Run(name+"/committed", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Write("a.go", []byte("a\n")), "the staging takes")
					_, err := s.Commit()
					assert.NoError(t, err, "the commit runs")
					assert.ErrorIs(t, s.Write("b.go", []byte("b\n")), output.ErrFinished,
						"a sink serves one staging")
					_, err = s.Commit()
					assert.ErrorIs(t, err, output.ErrFinished, "and commits once")
				})
				t.Run(name+"/discarded", func(t *testing.T) {
					t.Parallel()

					s := open()
					assert.NoError(t, s.Write("a.go", []byte("a\n")), "the staging takes")
					assert.NoError(t, s.Discard(), "the discard runs")
					assert.ErrorIs(t, s.Write("b.go", []byte("b\n")), output.ErrFinished,
						"a discarded sink takes nothing more")
				})
			}
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("spells what a commit did", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, output.ActionCreated.String(), "created",
				"a path that was not there")
			assert.Equal(t, output.ActionUpdated.String(), "updated",
				"a path that held different bytes")
			assert.Equal(t, output.ActionUnchanged.String(), "unchanged",
				"a path the commit left alone")
		})

		t.Run("spells an action nothing declares by its number", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, output.Action(7).String(), "Action(7)",
				"a diagnostic over an unknown action still names it, "+
					"rather than reading as one of the three")
			assert.Equal(t, output.Action(12).String(), "Action(12)",
				"in every digit its number has")
		})
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		t.Run("records every staged path in path order", func(t *testing.T) {
			t.Parallel()

			for name, open := range every(t) {
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					s := open()
					for _, p := range []string{"svc/store.go", "a.go", "svc/api.go"} {
						assert.NoError(t, s.Write(p, []byte("package p\n")),
							"the fixture stages "+p)
					}
					got, err := s.Commit()
					assert.NoError(t, err, "the commit runs")
					assert.Equal(t, []string{got[0].Path, got[1].Path, got[2].Path},
						[]string{"a.go", "svc/api.go", "svc/store.go"},
						"sorted by path, whatever order they were staged in")
					for _, w := range got {
						assert.Equal(t, w.Action, output.ActionCreated,
							"nothing was there before")
						assert.Equal(t, w.Hash, "sha256:"+
							"0ad6261536f6380b14ade1a508ac911b8c48230731746e0c76abd26bf3e3a15d",
							"the digest of the bytes as written")
					}
				})
			}
		})
	})
}
