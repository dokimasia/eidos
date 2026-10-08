// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/frontend"
	javagrammar "go.dokimi.dev/eidos/lang/treesitter/java"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The whole-contract fixture's signature root and its package, and the
// one type the package does not publish, which a signature-only load
// drops.
const (
	depRoot    = "dep"
	depPackage = "com/acme/dep"
	hiddenName = "Hidden"
)

// unparsedCode is the spelling of the code a syntax error reports
// under, which findings persist.
const unparsedCode = "JAVA-0001"

// release is the Java release the options cases build a frontend for.
const release = 21

// newAllocs is the frontend New returns for nil options: the empty
// options, the frontend's state with its vocabulary and two hooks, the
// version, the syntax's two, and the kit's four.
const newAllocs = 1 + 4 + 1 + 2 + 4

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
// Java source.
func TestFrontend(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frontend that passes the conformance suite", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, setup)
		})

		t.Run("returns a frontend whose language overloads", func(t *testing.T) {
			t.Parallel()

			assert.True(t, frontend.New(nil).Overloads(), "Java declares overloaded methods")
		})

		t.Run("returns a version that folds the frontend's version", func(t *testing.T) {
			t.Parallel()

			assert.HasPrefix(t, versionOf(t), java.FrontendVersion, "a graph change bumps what the unit keys fold")
		})

		t.Run("returns a version that folds the grammar's version", func(t *testing.T) {
			t.Parallel()

			assert.HasSuffix(t, versionOf(t), javagrammar.Grammar.Version(),
				"a grammar upgrade bumps what the unit keys fold")
		})

		t.Run("returns a frontend whose options are the ones it was built with", func(t *testing.T) {
			t.Parallel()

			opts := &frontend.Options{Release: release}
			op, provides := frontend.New(opts).(plugin.OptionsProvider)
			assert.True(t, provides, "the frontend declares its configuration")
			assert.Equal(t, op.Options(), any(opts), "so every unit key folds the release")
		})

		t.Run("returns a frontend whose options default to the zero options", func(t *testing.T) {
			t.Parallel()

			op, _ := frontend.New(nil).(plugin.OptionsProvider)
			assert.Equal(t, op.Options(), any(&frontend.Options{}), "the newest release and no library")
		})

		t.Run("returns a frontend that locates its stores through Stores", func(t *testing.T) {
			t.Parallel()

			locator, locates := frontend.New(nil).(plugin.StoreLocator)
			assert.True(t, locates, "the dependency rounds read ct.sym and the repositories of libraries")
			unset := func(string) string { return "" }
			_, want := frontend.Stores(unset)
			assert.HasError(t, want, "Stores needs JAVA_HOME")
			_, err := locator.Stores(unset)
			assert.Equal(t, err, want, "the frontend returns the error of Stores")
		})

		t.Run("returns a frontend that reports a syntax error as JAVA-0001", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class {\n")
			assert.NotEmpty(t, found, "the syntax error reports")
			assert.Equal(t, found[0].Code.String(), unparsedCode, "under the satellite's prefix and the first number")
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

				assert.Equal(tb, f.Name(), java.Name, "New returns the Java frontend")
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

// setup is the suite's entry over the whole-contract fixture, with a
// ct.sym whose java.lang the first dependency round loads.
func setup(tb assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
	return frontend.New(nil), &frontendtest.Fixture{
		Sources:    javaTree(),
		Signatures: []string{depRoot},
		Dropped: []symbol.Identity{
			{Lang: frontend.Lang, Package: depPackage, Name: hiddenName, Kind: symbol.KindStruct},
		},
		Stores: map[string]fs.FS{
			frontend.JDKStore: fstest.MapFS{lang9To25: {Data: fixtureClass(tb, circleName)}},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    frontend.Keys,
	}
}

// javaTree is the whole-contract fixture: two packages and a reference
// between them, a carrier, a test file, a signature root and a file
// that does not parse.
func javaTree() fstest.MapFS {
	return fstest.MapFS{
		pomFile: {Data: []byte(pomSrc)},
		"src/main/java/com/acme/api/User.java": {Data: []byte("package com.acme.api;\n\n" +
			"public class User {\n    public String name;\n}\n")},
		"src/main/java/com/acme/store/Row.java": {Data: []byte("package com.acme.store;\n\n" +
			"import com.acme.api.User;\n\n/** Row is one record. */\n// +fixture:gen:table name=rows\n" +
			"public class Row {\n    public User owner;\n    public int count;\n\n" +
			"    public int get(int n) {\n        return count + n;\n    }\n}\n")},
		"src/test/java/com/acme/store/RowTest.java": {Data: []byte("package com.acme.store;\n\n" +
			"class RowTest {\n    void probe() {}\n}\n")},
		"dep/src/com/acme/dep/Dep.java": {Data: []byte("package com.acme.dep;\n\npublic class Dep {}\n\n" +
			"class Hidden {}\n")},
		"bad/Oops.java": {Data: []byte("public class {\n")},
	}
}

// versionOf returns the version of a frontend built without options.
func versionOf(tb testing.TB) string {
	tb.Helper()

	v, versioned := frontend.New(nil).(plugin.Versioned)
	assert.True(tb, versioned, "the frontend states a version")
	return v.Version()
}
