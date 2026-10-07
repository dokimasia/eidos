// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pathset_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/pathset"
)

// The fixture paths the cases add and check.
const (
	storeFile = "svc/store.go"
	storeTest = "svc/store_test.go"
	storeDir  = "svc"
	rowFile   = "svc/store.go/row.go"
	genFile   = "gen/a.go"
	genDir    = "Gen"
)

// A set admits the paths one filesystem can contain together, on a
// case-insensitive filesystem too.
func TestSet(t *testing.T) {
	t.Parallel()

	t.Run("Clash", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			set       []string
			give      string
			wantClash pathset.Clash
			wantOther string
		}{
			{
				name: "returns ClashNone for a path in an empty set",
				give: storeFile, wantClash: pathset.ClashNone,
			},
			{
				name: "returns ClashNone for a sibling of a path in the set",
				set:  []string{storeFile}, give: storeTest, wantClash: pathset.ClashNone,
			},
			{
				name: "returns ClashNone for a path the set contains in the same spelling",
				set:  []string{storeFile}, give: storeFile, wantClash: pathset.ClashNone,
			},
			{
				name: "returns ClashCase for a filename that differs from a path of the set only in case",
				set:  []string{storeFile}, give: "svc/Store.go",
				wantClash: pathset.ClashCase, wantOther: storeFile,
			},
			{
				name: "returns ClashCase for a directory that differs from one of the set only in case",
				set:  []string{storeFile}, give: "Svc/store.go",
				wantClash: pathset.ClashCase, wantOther: storeFile,
			},
			{
				name: "returns ClashDirectory for a path the set needs as a directory",
				set:  []string{storeFile}, give: storeDir,
				wantClash: pathset.ClashDirectory, wantOther: storeFile,
			},
			{
				name: "returns ClashDirectory for a path the set needs as a directory in another case",
				set:  []string{genFile}, give: genDir,
				wantClash: pathset.ClashDirectory, wantOther: genFile,
			},
			{
				name: "returns ClashDirectory with the first path that needs the directory",
				set:  []string{storeFile, storeTest}, give: storeDir,
				wantClash: pathset.ClashDirectory, wantOther: storeFile,
			},
			{
				name: "returns ClashFile for a path under a path of the set",
				set:  []string{storeFile}, give: rowFile,
				wantClash: pathset.ClashFile, wantOther: storeFile,
			},
			{
				name: "returns ClashFile for a path under a path of the set in another case",
				set:  []string{genDir}, give: genFile,
				wantClash: pathset.ClashFile, wantOther: genDir,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var s pathset.Set
				for _, p := range tt.set {
					s.Add(p)
				}
				clash, other := s.Clash(tt.give)
				assert.Equal(t, clash, tt.wantClash, "the clash")
				assert.Equal(t, other, tt.wantOther, "the path it clashes with")
			})
		}
	})

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the first spelling of a path added in two cases", func(t *testing.T) {
			t.Parallel()

			var s pathset.Set
			s.Add("svc/Store.go")
			s.Add(storeFile)
			_, other := s.Clash("svc/STORE.go")
			assert.Equal(t, other, "svc/Store.go", "the first spelling")
		})

		t.Run("records every directory a path needs", func(t *testing.T) {
			t.Parallel()

			var s pathset.Set
			s.Add("a/b/c.go")
			for _, dir := range []string{"a", "a/b"} {
				clash, other := s.Clash(dir)
				assert.Equal(t, clash, pathset.ClashDirectory, "the clash of "+dir)
				assert.Equal(t, other, "a/b/c.go", "the path that needs "+dir)
			}
		})

		t.Run("records the directories below a directory already recorded", func(t *testing.T) {
			t.Parallel()

			var s pathset.Set
			s.Add("a/b/c.go")
			s.Add("a/d/e.go")
			clash, other := s.Clash("a/d")
			assert.Equal(t, clash, pathset.ClashDirectory, "the clash of the new directory")
			assert.Equal(t, other, "a/d/e.go", "the path that needs the new directory")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give pathset.Clash
			want string
		}{
			{name: "returns none for ClashNone", give: pathset.ClashNone, want: "none"},
			{name: "returns case for ClashCase", give: pathset.ClashCase, want: "case"},
			{name: "returns directory for ClashDirectory", give: pathset.ClashDirectory, want: "directory"},
			{name: "returns file for ClashFile", give: pathset.ClashFile, want: "file"},
			{name: "returns the number of an undeclared clash", give: pathset.Clash(7), want: "Clash(7)"},
			{name: "returns every digit of an undeclared clash's number", give: pathset.Clash(12), want: "Clash(12)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})
}

// A lowercase path is its own folded spelling, so the checks over one
// allocate nothing in the ordinary run, which runs no benchmark. A
// path with a capital allocates its folded copy.
func TestSetAllocs(t *testing.T) {
	s := filled()
	var clash pathset.Clash
	assert.MaxAllocs(t, func() { clash, _ = s.Clash(storeTest) }, 0, "Clash allocates nothing for a lowercase path")
	assert.Equal(t, clash, pathset.ClashNone, "a sibling fits")
	assert.MaxAllocs(t, func() { clash, _ = s.Clash("svc/Store.go") }, 1,
		"Clash allocates the folded spelling of a path with a capital")
	assert.Equal(t, clash, pathset.ClashCase, "a path in another case clashes by case")
	assert.MaxAllocs(t, func() { s.Add(storeFile) }, 0, "Add allocates nothing for a path the set contains")
	var spelt string
	assert.MaxAllocs(
		t,
		func() { spelt = pathset.ClashFile.String() },
		0,
		"String allocates nothing for a declared clash",
	)
	assert.Equal(t, spelt, "file", "String spells ClashFile")
}

// BenchmarkSet measures the checks a layout runs for every file it
// routes, over a set of the fixture's paths.
func BenchmarkSet(b *testing.B) {
	s := filled()

	b.Run("Clash", func(b *testing.B) {
		b.Run("a lowercase path that fits", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got pathset.Clash
			for c.Loop() {
				got, _ = s.Clash(storeTest)
			}
			assert.Equal(b, got, pathset.ClashNone, "Clash reports a sibling as fitting")
		})

		b.Run("a path that differs only in case", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var got pathset.Clash
			for c.Loop() {
				got, _ = s.Clash("svc/Store.go")
			}
			assert.Equal(b, got, pathset.ClashCase, "Clash reports the case clash")
		})
	})

	b.Run("Add", func(b *testing.B) {
		b.Run("a path the set contains", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			for c.Loop() {
				s.Add(storeFile)
			}
			clash, _ := s.Clash(storeFile)
			assert.Equal(b, clash, pathset.ClashNone, "Add keeps the one spelling")
		})
	})

	b.Run("Clash.String", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = pathset.ClashFile.String()
		}
		assert.Equal(b, got, "file", "String spells the clash")
	})
}

// filled returns a set of the fixture's store and generated paths.
func filled() *pathset.Set {
	var s pathset.Set
	for _, p := range []string{storeFile, genFile, "a/b/c.go"} {
		s.Add(p)
	}
	return &s
}
