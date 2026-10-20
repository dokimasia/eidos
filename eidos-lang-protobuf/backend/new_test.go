// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// contractBrand is the brand of the end-to-end fixture, under which the
// output contract stamps.
const contractBrand = "protobuf"

// These constants are the allocations of a build and of the scaled
// corpus.
const (
	// newAllocs is a build of the backend, chiefly the parse of the file
	// template, the block template of each of the four kinds and the extend
	// template, and the function maps that text/template keeps for each
	// template.
	newAllocs = 1_027
	// renderAllocs is one render of the scaled corpus of 10,000 files. One
	// run counted 13,660,874: about 6.1 million in the parse that checks
	// each written file, and nearly all of the rest in text/template's
	// execution and its reflective calls of the vocabulary. The budget is a
	// round number about 5% above the count.
	renderAllocs = 14_350_000
	// settleAllocs is one settle of the scaled corpus. One run counted
	// 2,446,749: about 1.6 million in the lowering, which declares the
	// requests and the responses of the rpcs, the messages of the sums and
	// the members of their variants, and the rest in the plan of the names
	// and the respelling of every reference. The budget is a round number
	// about 5% above the count.
	settleAllocs = 2_570_000
)

// canonicalHead opens every file of the canonical fixture: proto3, and the
// clause of the directory of the fixture.
const canonicalHead = "syntax = \"proto3\";\n\npackage svc;\n\n"

// canonicalFiles are the files that the backend writes for the canonical
// fixture, by the line that opens the declaration of each. The generic
// siblings and the declarations of the kinds that protobuf refuses do not
// render.
var canonicalFiles = map[string]string{
	"enum Phase": canonicalHead +
		"// phase names a lifecycle step.\nenum Phase { // closed set\n" +
		"  // open admits writes.\n  OPEN = 0;\n  CLOSED = 1;\n}\n",
	"message Shape": canonicalHead + "import \"google/protobuf/empty.proto\";\n\n" +
		"// shape is one closed figure.\nmessage Shape { // tagged\n  oneof shape {\n" +
		"    // circle bounds by a radius.\n    int circle = 1; // round\n" +
		"    google.protobuf.Empty empty = 2;\n  }\n}\n",
	"message Row": canonicalHead +
		"// Row is one canonical record.\nmessage Row { // one per fetch\n" +
		"  // Name keys the row.\n  string name = 1; // unique per store\n}\n",
	"service Store": canonicalHead +
		"// Store reads rows back.\nservice Store { // read side\n" +
		"  // Get returns the row key names.\n  rpc Get(StoreGetRequest) returns (StoreGetResponse); // by key\n}\n\n" +
		"message StoreGetRequest {\n  string key = 1; // the row key\n}\n\n" +
		"message StoreGetResponse {\n  string result = 1; // the row\n}\n",
}

// allocCall is one call that an allocation test and a benchmark share.
// It has the method that it calls, whose name is the name of its
// benchmark, the case that it measures where the method has more than
// one call, its allocation ceiling, the call, and the check of the
// result of the call. A case whose ceiling only a benchmark checks sets
// bench, which measures the case in place of the call. No list that an
// allocation test reads contains such a case.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
	bench    func(b *testing.B)
}

// The backend is the write half of the module. The kernel suite runs the
// render checks over it on the canonical fixture, whose declarations take
// the numbers that protobuf needs, and the stamp check joins it to the
// output contract under the comment forms of the module.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		backendtest.RunBackendSuite(t, setup)

		t.Run("returns a backend named after the language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Name(), protobuf.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), protobuf.Target, "the rendering target")
		})

		t.Run("returns a backend whose spoke spells a type of another language", func(t *testing.T) {
			t.Parallel()

			got, err := backend.New().SpellType(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got.Spelling, protobuf.TypeString, "the backend's spoke is spell.Type")
		})

		t.Run("returns a backend whose policies are the encodings of the integers", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Policies(), protobuf.Policies(), "the policies of the satellite")
		})

		t.Run("returns a backend whose files stamp under the module's contract", func(t *testing.T) {
			t.Parallel()

			c, err := output.NewContract(contractBrand, protobuf.Syntax())
			assert.NoError(t, err, "the module contract composes")
			backendtest.AssertStamped(t, setup, c)
		})

		t.Run("returns a backend that writes each file of the canonical fixture", func(t *testing.T) {
			t.Parallel()

			files, _ := backendtest.Rendered(t, setup)
			got := map[string]string{}
			for _, f := range files {
				body := string(f.Body)
				for opening := range canonicalFiles {
					if strings.Contains(body, "\n"+opening+" ") {
						got[opening] = body
					}
				}
				assert.Equal(t, body, formatted(t, body), "the backend writes "+f.Path+" in the layout of buf format")
			}
			assert.Equal(t, got, canonicalFiles, "the file of each declaration that renders")
		})
	})
}

// The build allocates the kit's backend. The ordinary run, which runs
// no benchmark, checks that ceiling here.
func TestNewAllocs(t *testing.T) {
	checkAllocs(t, newCalls())
}

// BenchmarkNew measures the build of the backend, and the composed
// backend's settle and render of the suite's scaled corpus: the real
// templates, the extend clustering and the formatter. The settle leaves the
// corpus build out of its count. Only -bench checks the two corpus
// ceilings. No count of [assert.MaxAllocs] leaves the corpus build out, as
// the settle's count does, and one render takes seconds, so an allocation
// check's 101 calls would take minutes.
func BenchmarkNew(b *testing.B) {
	benchCalls(b, append(newCalls(),
		allocCall{name: "New", caseName: "the render of the scaled corpus", bench: func(b *testing.B) {
			b.Helper()
			backendtest.BenchRender(b, benchSetup, backendtest.Budget{MaxAllocs: renderAllocs})
		}},
		allocCall{name: "New", caseName: "the settle of the scaled corpus", bench: func(b *testing.B) {
			b.Helper()
			backendtest.BenchSettle(b, benchSetup, backendtest.Budget{MaxAllocs: settleAllocs})
		}},
	))
}

// newCalls returns a call of New.
func newCalls() []allocCall {
	var b plugin.Backend
	return []allocCall{
		{
			name: "New", caseName: "the backend", allocs: newAllocs,
			call:  func() { b = backend.New() },
			check: func(tb assert.TB) { assert.Equal(tb, b.Name(), protobuf.Name, "New returns the protobuf backend") },
		},
	}
}

// checkAllocs checks the ceiling of every call in the ordinary run, and
// the result each call leaves.
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

// benchCalls measures every call under the bench contract at its
// ceiling: one sub-benchmark for each method, in the order the methods
// first appear, and inside it one for each case of a method with cases.
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

// benchCall measures one call under the bench contract at its ceiling,
// and checks the result the last call leaves. The call runs once before
// the contract starts, so the count leaves out what the first call
// initialises. A case that sets bench runs it instead.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	if tt.bench != nil {
		tt.bench(b)
		return
	}
	tt.call()
	c := bench.Start(b).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}

// setup builds the backend over the kernel's canonical fixture, whose
// declarations take the numbers that protobuf needs. The fixture emits
// every file-level kind: the backend spells the message, the enum, the sum
// as a message with a oneof and the service, rejects their generic
// siblings, and refuses the other kinds.
func setup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	f := backendtest.CanonicalFixture(tb)
	numbered(f)
	return backend.New(), f
}

// benchSetup builds the backend over the suite's scaled corpus without
// the kinds that the backend refuses. Its declarations take the numbers
// that protobuf needs and lose their type parameters, so the render
// measures messages, enums and services and no rejection.
func benchSetup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	f := backendtest.ScaledFixture(tb, backend.RefusedKinds())
	numbered(f)
	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			switch x := d.(type) {
			case *emit.Struct:
				x.TypeParams = nil
			case *emit.Sum:
				x.TypeParams = nil
			case *emit.Interface:
				x.TypeParams = nil
			}
		}
	}
	return backend.New(), f
}

// numbered states in place what protobuf needs of the declarations of the
// fixture f and other languages leave out. Each field of a message takes
// the tag entry number of its position in place of its tag, each enum
// value takes its position as its number, and each member of a oneof takes
// the next number of the message of the oneof: a variant with one field
// through the tag of the field, and any other variant through the marker
// protobuf:number.
func numbered(f *backendtest.Fixture) {
	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			numberedDecl(d)
		}
	}
}

// numberedDecl states the numbers of the declaration d and of its nested
// declarations by the rules of [numbered].
func numberedDecl(d symbol.Symbol) {
	switch x := d.(type) {
	case *emit.Struct:
		for i, f := range x.Fields.Items() {
			f.Tags = emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: strconv.Itoa(i + 1)})
		}
		for _, t := range x.Types.Items() {
			numberedDecl(t)
		}
	case *emit.Enum:
		for i, v := range x.Variants.Items() {
			v.Value = strconv.Itoa(i)
		}
	case *emit.Sum:
		for i, v := range x.Variants.Items() {
			number := strconv.Itoa(i + 1)
			if fields := v.Fields.Items(); len(fields) == 1 {
				fields[0].Tags = emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: number})
				continue
			}
			v.Annotations = append(
				v.Annotations,
				symbol.Annotation{Name: string(protobuf.NumberMarker), Args: []string{number}},
			)
		}
	}
}
