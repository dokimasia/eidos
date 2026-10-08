// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	rustgrammar "go.dokimi.dev/eidos/lang/treesitter/rust"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The whole-contract fixture's signature root and its crate, the one
// item the crate does not publish, which a signature-only load drops,
// and the declaration a reference names only through a pub use.
const (
	depRoot       = "dep"
	depCrate      = "dep"
	hiddenName    = "hidden"
	personPackage = "app/models/person"
	personName    = "Person"
)

// The spellings of the codes the frontend registers, which findings
// persist.
const (
	unparsedCode = "RUST-0001"
	targetClaim  = "!**/target/**"
)

// newAllocs is the frontend New returns for nil options: the empty
// options, the frontend's state with its vocabulary and its parse hook,
// the version, the syntax's two, and the kit's three.
const newAllocs = 1 + 3 + 1 + 2 + 3

// allocCall is one call that an allocation test and a benchmark share:
// the method it calls, which names its benchmark, the case it measures
// where the method has more than one call, its allocation ceiling, the
// call, and the check of the result the call leaves.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb testing.TB)
}

// The frontend runs the conformance suite every frontend runs, over real
// Rust source.
func TestFrontend(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frontend that passes the conformance suite", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, setup)
		})

		t.Run("returns a frontend in the exporter role", func(t *testing.T) {
			t.Parallel()

			_, exports := frontend.New(nil).(plugin.Exporter)
			assert.True(t, exports, "a Rust module publishes the names its use declarations bind")
		})

		t.Run("returns a frontend whose language cannot overload", func(t *testing.T) {
			t.Parallel()

			assert.False(t, frontend.New(nil).Overloads(), "every Rust callable takes the empty discriminator")
		})

		t.Run("returns a version that folds the frontend's version", func(t *testing.T) {
			t.Parallel()

			assert.HasPrefix(t, versionOf(t), rust.FrontendVersion, "a graph change bumps what the unit keys fold")
		})

		t.Run("returns a version that folds the grammar's version", func(t *testing.T) {
			t.Parallel()

			assert.HasSuffix(t, versionOf(t), rustgrammar.Grammar.Version(),
				"a grammar upgrade bumps what the unit keys fold")
		})

		t.Run("returns a frontend whose options are the ones it was built with", func(t *testing.T) {
			t.Parallel()

			opts := &frontend.Options{Features: []string{"x"}}
			op, provides := frontend.New(opts).(plugin.OptionsProvider)
			assert.True(t, provides, "the frontend declares its configuration")
			assert.Equal(t, op.Options(), any(opts), "so every unit key folds the cfg set")
		})

		t.Run("returns a frontend that reports a syntax error as RUST-0001", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub struct {\n")
			assert.NotEmpty(t, found, "the syntax error reports")
			assert.Equal(t, found[0].Code.String(), unparsedCode, "under the satellite's prefix and the first number")
		})

		t.Run("returns a frontend that claims no file under a target directory", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, frontend.New(nil).Selection(), targetClaim, "Cargo's build output is not source")
		})
	})
}

// The construction allocates the frontend the kit builds. The ordinary
// run, which runs no benchmark, checks that ceiling here.
func TestFrontendAllocs(t *testing.T) {
	checkAllocs(t, frontendCalls())
}

// BenchmarkFrontend measures the construction a composition makes once
// per load.
func BenchmarkFrontend(b *testing.B) {
	benchCalls(b, frontendCalls())
}

// frontendCalls returns a call of New with nil options.
func frontendCalls() []allocCall {
	var f plugin.Frontend
	return []allocCall{
		{
			name: "New", allocs: newAllocs,
			call: func() { f = frontend.New(nil) },
			check: func(tb testing.TB) {
				tb.Helper()

				assert.Equal(tb, f.Name(), rust.Name, "New returns the Rust frontend")
			},
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
// and checks the result the last call leaves. The contract warms up with
// one call, so what the first call initialises stays out of the count.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}

// rustTree is the whole-contract fixture: a crate whose modules
// reference each other, a carrier, an integration test, a signature
// root, a file that does not parse, and a module that publishes through
// a pub use what its private child module declares.
func rustTree() fstest.MapFS {
	return fstest.MapFS{
		manifestPath: {Data: []byte("[package]\nname = \"app\"\n")},
		libRoot:      {Data: []byte("pub mod api;\npub mod models;\npub mod store;\npub mod uses;\n")},
		"src/api.rs": {Data: []byte("pub struct User {\n    pub name: String,\n}\n")},
		"src/store.rs": {Data: []byte("use crate::api::User;\n\n/// Row is one record.\n" +
			"/// +fixture:gen:table name=rows\npub struct Row {\n    pub owner: User,\n    pub count: u32,\n}\n\n" +
			"impl Row {\n    pub fn get(&self, n: u32) -> u32 {\n        self.count + n\n    }\n}\n")},
		"src/models/mod.rs":    {Data: []byte("mod person;\npub use person::Person;\n")},
		"src/models/person.rs": {Data: []byte("pub struct Person {\n    pub id: String,\n}\n")},
		"src/uses.rs": {Data: []byte("use crate::models::Person;\n\npub struct Session {\n" +
			"    pub user: Person,\n}\n")},
		"tests/it.rs":    {Data: []byte("#[test]\nfn probe() {}\n")},
		"dep/Cargo.toml": {Data: []byte("[package]\nname = \"dep\"\n")},
		"dep/src/lib.rs": {Data: []byte("pub struct Dep;\nfn hidden() {}\n")},
		"bad/oops.rs":    {Data: []byte("pub struct {\n")},
	}
}

// setup is the suite's entry over the whole-contract fixture.
func setup(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
	return frontend.New(nil), &frontendtest.Fixture{
		Sources:    rustTree(),
		Signatures: []string{depRoot},
		Dropped: []symbol.Identity{
			{Lang: frontend.Lang, Package: depCrate, Name: hiddenName, Kind: symbol.KindFunction},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    frontend.Keys,
		Reexported: []symbol.Identity{
			{Lang: frontend.Lang, Package: personPackage, Name: personName, Kind: symbol.KindStruct},
		},
	}
}

// versionOf returns the version of a frontend built without options.
func versionOf(tb testing.TB) string {
	tb.Helper()

	v, versioned := frontend.New(nil).(plugin.Versioned)
	assert.True(tb, versioned, "the frontend states a version")
	return v.Version()
}
