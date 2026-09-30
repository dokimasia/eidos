// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
)

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
						assert.Equal(t, w.Hash, "sha256:"+
							"0ad6261536f6380b14ade1a508ac911b8c48230731746e0c76abd26bf3e3a15d",
							"the hash is the digest of the bytes as written")
					}
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

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give output.Action
			want string
		}{
			{name: "returns created for ActionCreated", give: output.ActionCreated, want: "created"},
			{name: "returns updated for ActionUpdated", give: output.ActionUpdated, want: "updated"},
			{name: "returns unchanged for ActionUnchanged", give: output.ActionUnchanged, want: "unchanged"},
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
}
