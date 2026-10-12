// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"os"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The files that the partition cases claim, out of path order.
const (
	firstFile  = "a/first.proto"
	secondFile = "b/second.proto"
	thirdFile  = "c/third.proto"
)

// The allocations of the construction and of the partition.
const (
	// newAllocs is the cost of New: two allocations of the syntax, which
	// the satellite root builds, and four of the kit with its list of key
	// registrations.
	newAllocs = 2 + 4
	// partitionAllocs is the cost of a partition of any number of files:
	// the sorted copy and the list of units.
	partitionAllocs = 2
)

// allocCall is one call that an allocation test and a benchmark share: the
// method that it calls, which is the name of its benchmark, the case that
// it measures where the method has more than one call, its allocation
// ceiling, the call, and the check of the result of the call.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
}

// New builds the frontend through the kit. The cases pin the identity, the
// claim, the roles and the unit grain of the frontend.
func TestFrontend(t *testing.T) {
	t.Parallel()

	f := protofrontend.New()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frontend that meets the conformance contract over the schema tree", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return protofrontend.New(), &frontendtest.Fixture{
					Sources: os.DirFS("testdata/schema"),
					Empty: fstest.MapFS{
						"empty/empty.proto": {Data: []byte("syntax = \"proto3\";\n\npackage empty;\n")},
					},
					// A schema has no bodies and no unexported names, so a
					// signature-only root loads what a full one loads, and
					// the fixture lists no dropped identity.
					Signatures: []string{"dep"},
					Schemas:    []directive.Schema{tableSchema()},
				}
			})
		})

		t.Run("returns a frontend in the importer role", func(t *testing.T) {
			t.Parallel()

			_, is := f.(plugin.Importer)
			assert.True(t, is, "a proto import names a file, so the frontend names the import of a declaring file")
		})

		t.Run("returns a frontend named after the satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Name(), protobuf.Name, "findings report under the identity of the satellite")
		})

		t.Run("returns a frontend that registers the protobuf keys under the identity of the language",
			func(t *testing.T) {
				t.Parallel()

				provider, provides := f.(plugin.KeyProvider)
				assert.True(t, provides, "the frontend registers keys")
				r := meta.NewRegistry()
				assert.NoError(t, provider.Keys(r.For(string(protofrontend.Lang))), "the keys register")
				_, held := r.Resolve(protobuf.FieldKey)
				assert.True(t, held, "the field key is registered")
			})

		t.Run("returns a frontend in the language of the satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Lang(), protobuf.Lang, "every loaded declaration is in the language of the satellite")
		})

		t.Run("returns a frontend in the language that the package restates", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Lang(), protofrontend.Lang, "a caller with the frontend reads Lang")
		})

		t.Run("returns a frontend with the comment forms of the satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Syntax(), protobuf.Syntax(), "the comment forms belong to the satellite")
		})

		t.Run("returns a frontend whose language cannot overload", func(t *testing.T) {
			t.Parallel()

			assert.False(t, f.Overloads(), "a service declares each RPC once by name")
		})

		t.Run("returns a frontend with the version of the satellite", func(t *testing.T) {
			t.Parallel()

			versioned, held := f.(plugin.Versioned)
			assert.True(t, held, "every unit key folds a version, so the frontend states one")
			assert.Equal(t, versioned.Version(), protobuf.Version, "the version belongs to the satellite")
		})

		t.Run("returns a frontend that claims every proto file", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Selection(), []string{"**/*.proto"}, "the claim negates nothing")
		})
	})

	t.Run("partition", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one unit per file", func(t *testing.T) {
			t.Parallel()

			units, err := f.Partition(t.Context(), unordered(), nil)
			assert.NoError(t, err, "the grain needs no bytes to settle")
			assert.Equal(t, []int{len(units[0]), len(units[1]), len(units[2])}, []int{1, 1, 1},
				"a file compiles alone")
		})

		t.Run("returns the units in path order", func(t *testing.T) {
			t.Parallel()

			units, _ := f.Partition(t.Context(), unordered(), nil)
			assert.Equal(t, []string{units[0][0].Path, units[1][0].Path, units[2][0].Path},
				[]string{firstFile, secondFile, thirdFile}, "two runs over one tree partition alike")
		})

		t.Run("returns units without a shared input", func(t *testing.T) {
			t.Parallel()

			units, _ := f.Partition(t.Context(), unordered(), nil)
			assert.Empty(t, units[0][0].Shared, "no file outside a unit contributes to it")
		})

		t.Run("returns units without spare capacity", func(t *testing.T) {
			t.Parallel()

			units, _ := f.Partition(t.Context(), unordered(), nil)
			assert.Equal(t, cap(units[0]), 1, "an append to a unit copies it, so the next unit stays as it is")
		})

		t.Run("leaves the claimed files in their order", func(t *testing.T) {
			t.Parallel()

			files := unordered()
			_, _ = f.Partition(t.Context(), files, nil)
			assert.Equal(t, files[0].Path, secondFile, "the partition sorts a copy")
		})

		t.Run("returns nothing for no files", func(t *testing.T) {
			t.Parallel()

			units, err := f.Partition(t.Context(), nil, nil)
			assert.NoError(t, err, "an empty claim partitions")
			assert.Empty(t, units, "there is no unit")
		})
	})
}

// The construction allocates the frontend that the kit builds, and a
// partition allocates its two lists. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestFrontendAllocs(t *testing.T) {
	checkAllocs(t, frontendCalls(t))
}

// BenchmarkFrontend measures the construction and the partition, which the
// load calls before it parses.
func BenchmarkFrontend(b *testing.B) {
	benchCalls(b, frontendCalls(b))
}

// frontendCalls returns a call of New and a call of the partition of the
// frontend, under the context of the test.
func frontendCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := protofrontend.New()
	files := unordered()
	var (
		built plugin.Frontend
		units [][]plugin.SourceRef
		err   error
	)
	return []allocCall{
		{
			name: "New", allocs: newAllocs,
			call:  func() { built = protofrontend.New() },
			check: func(tb assert.TB) { assert.Equal(tb, built.Name(), protobuf.Name, "New returns the frontend") },
		},
		{
			name: "partition", allocs: partitionAllocs,
			call: func() { units, err = f.Partition(tb.Context(), files, nil) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "the partition settles")
				assert.Length(tb, units, 3, "the partition returns one unit per file")
			},
		},
	}
}

// checkAllocs checks the ceiling of every call in the ordinary run, and the
// result of each call.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		if c.caseName != "" {
			msg = c.name + " for " + c.caseName + " allocates within its ceiling"
		}
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its ceiling:
// one sub-benchmark for each method, in the order of the first call of
// each method, and inside it one for each case of a method with cases.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	var methods []string
	byMethod := map[string][]allocCall{}
	for _, c := range calls {
		if _, seen := byMethod[c.name]; !seen {
			methods = append(methods, c.name)
		}
		byMethod[c.name] = append(byMethod[c.name], c)
	}
	for _, name := range methods {
		cases := byMethod[name]
		b.Run(name, func(b *testing.B) {
			if len(cases) == 1 && cases[0].caseName == "" {
				benchCall(b, cases[0])
				return
			}
			for _, tt := range cases {
				b.Run(tt.caseName, func(b *testing.B) { benchCall(b, tt) })
			}
		})
	}
}

// benchCall measures one call under the bench contract at its ceiling, and
// checks the result of the last call. The contract warms up with one call,
// so the count leaves out what the first call initialises.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}

// unordered returns three claimed files out of path order.
func unordered() []plugin.SourceRef {
	return []plugin.SourceRef{{Path: secondFile}, {Path: firstFile}, {Path: thirdFile}}
}

// tableSchema is the directive that the carrier of the fixture schema
// writes.
func tableSchema() directive.Schema {
	return directive.Schema{
		Plugin: "gen", Name: "table",
		Params: []directive.ParamSpec{{
			Key: "name", Type: directive.TypeString, Required: true,
			Doc: "the table the message maps to",
		}},
		Doc: "maps a message onto a table",
	}
}
