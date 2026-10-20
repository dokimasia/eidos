// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// contractBrand is the brand of the end-to-end fixture, under which the
// output contract stamps.
const contractBrand = "python"

// The names of the function whose overloads the cases render, and of the
// class that extends [baseName].
const (
	loadName    = "load"
	derivedName = "Derived"
)

// These constants are the allocations of a build and of the scaled corpus.
const (
	// newAllocs is a build of the backend, chiefly the parse of the table of
	// the versions of the standard library, the file template and the seven
	// kind templates.
	newAllocs = 852
	// renderAllocs is one render of the scaled corpus of 10,000 files and
	// 200,000 declarations. One run counted 3,351,712: about seven in ten in
	// text/template's execution and its reflective calls of the vocabulary,
	// and the rest in the text of each declaration, the statements of the
	// bodies and the import blocks. The budget is a round number about 5%
	// above it.
	renderAllocs = 3_520_000
	// settleAllocs is one settle of the scaled corpus. One run counted
	// 1,249,943: about three fifths in the lowering, chiefly the classes and
	// the alias of each sum, and the rest in the respelling of every name, the
	// rewrite of references and the plan of the names. The budget is a round
	// number about 5% above it.
	settleAllocs = 1_310_000
)

// canonicalModules are the modules that the backend writes for the canonical
// fixture under the default version, Python 3.11, by their paths. The
// standalone methods do not render, because Python declares a method inside
// its class.
var canonicalModules = map[string]string{
	"svc/count_gen.py": "count: int = 0\n\"\"\"Count tracks fetches.\"\"\"\n",
	"svc/id_gen.py": "from typing import TypeAlias, TypeVar\n\nT = TypeVar(\"T\", bound=Codec)\n\n" +
		"Id: TypeAlias = string  # opaque\n\"\"\"ID names a row.\"\"\"\n\nMatch: TypeAlias = Keyed[T]\n" +
		"\"\"\"Match names a keyed lookup.\"\"\"\n",
	"svc/limit_gen.py": "from typing import Final\n\nLIMIT: Final[int32] = 8  # rows per call\n" +
		"\"\"\"Limit bounds one fetch.\"\"\"\n",
	"svc/phase_gen.py": "from enum import Enum, auto\n\n\nclass Phase(Enum):  # closed set\n" +
		"    \"\"\"phase names a lifecycle step.\"\"\"\n\n    OPEN = auto()\n    \"\"\"open admits writes.\"\"\"\n" +
		"    CLOSED = auto()\n",
	"svc/row_gen.py": "from typing import Generic, TypeVar\n\nfrom audit import Audit\n\nT = TypeVar(\"T\")\n" +
		"U = TypeVar(\"U\", bound=Codec)\n\n\nclass Box(Generic[T]):\n    \"\"\"Box wraps one item.\"\"\"\n\n" +
		"    item: T\n    \"\"\"Item is the wrapped value.\"\"\"\n\n    def map(self, item: U) -> U:\n" +
		"        \"\"\"Map rewraps the item.\"\"\"\n        ...\n\n\nclass Row:  # one per fetch\n" +
		"    \"\"\"Row is one canonical record.\"\"\"\n\n    name: string  # unique per store\n" +
		"    \"\"\"Name keys the row.\"\"\"\n\n    def boot(self) -> None: ...\n\n    def fetch(self) -> None:\n" +
		"        state = begin()\n        commit(state)\n        Audit.record(state)\n        return\n\n" +
		"    def push(self) -> None:\n        saving()\n        audit()\n\n    def trace(self) -> None:\n" +
		"        trace()\n",
	"svc/shape_gen.py": "from dataclasses import dataclass\nfrom typing import Generic, TypeAlias, TypeVar\n\n" +
		"T = TypeVar(\"T\")\n\n\n@dataclass(frozen=True)\nclass PackSome(Generic[T]):\n" +
		"    \"\"\"some contains the item.\"\"\"\n\n    item: T\n\n\n@dataclass(frozen=True)\n" +
		"class PackNone(Generic[T]): ...\n\n\nPack: TypeAlias = PackSome[T] | PackNone[T]\n" +
		"\"\"\"pack wraps one optional item.\"\"\"\n\n\n@dataclass(frozen=True)\nclass ShapeCircle:  # round\n" +
		"    \"\"\"circle bounds by a radius.\"\"\"\n\n    radius: int\n\n\n@dataclass(frozen=True)\n" +
		"class ShapeEmpty: ...\n\n\nShape: TypeAlias = ShapeCircle | ShapeEmpty  # tagged\n" +
		"\"\"\"shape is one closed figure.\"\"\"\n",
	"svc/store_gen.py": "from typing import Protocol, TypeVar\n\nK = TypeVar(\"K\", bound=Codec)\n\n\n" +
		"class Keyed(Protocol[K]):\n    \"\"\"Keyed looks rows up.\"\"\"\n\n    def pick(self, key: K) -> K:\n" +
		"        \"\"\"Pick returns the row at a key.\"\"\"\n        ...\n\n\nclass Store(Closer, Protocol):  # read side\n" +
		"    \"\"\"Store reads rows back.\"\"\"\n\n    def get(\n        self,\n        key: string,  # the row key\n" +
		"    ) -> string:  # by key\n        \"\"\"Get returns the row key names.\"\"\"\n        ...\n",
	"svc/task_gen.py": "from typing import TypeVar\n\nfrom audit import Audit\n\nT = TypeVar(\"T\", bound=Codec)\n\n\n" +
		"def boot() -> None: ...\n\n\ndef fetch() -> None:\n    state = begin()\n    commit(state)\n" +
		"    Audit.record(state)\n    return\n\n\ndef push() -> None:\n    saving()\n    audit()\n\n\n" +
		"def sort(items: T) -> T:  # stable\n    \"\"\"Sort orders items in place.\"\"\"\n    ...\n\n\n" +
		"def trace() -> None:\n    trace()\n",
}

// canonicalStubs are the stub files that differ from the modules of
// [canonicalModules] beyond their extension and their blank lines, by their
// paths: the files with bodies, which a stub file writes as ..., and the
// variable, whose value a stub file writes as ....
var canonicalStubs = map[string]string{
	"svc/count_gen.pyi": "count: int = ...\n\"\"\"Count tracks fetches.\"\"\"\n",
	"svc/row_gen.pyi": "from typing import Generic, TypeVar\n\nT = TypeVar(\"T\")\nU = TypeVar(\"U\", bound=Codec)\n\n" +
		"class Box(Generic[T]):\n    \"\"\"Box wraps one item.\"\"\"\n\n    item: T\n    \"\"\"Item is the wrapped value.\"\"\"\n\n" +
		"    def map(self, item: U) -> U:\n        \"\"\"Map rewraps the item.\"\"\"\n        ...\n\n" +
		"class Row:  # one per fetch\n    \"\"\"Row is one canonical record.\"\"\"\n\n    name: string  # unique per store\n" +
		"    \"\"\"Name keys the row.\"\"\"\n\n    def boot(self) -> None: ...\n    def fetch(self) -> None: ...\n" +
		"    def push(self) -> None: ...\n    def trace(self) -> None: ...\n",
	"svc/task_gen.pyi": "from typing import TypeVar\n\nT = TypeVar(\"T\", bound=Codec)\n\n" +
		"def boot() -> None: ...\ndef fetch() -> None: ...\ndef push() -> None: ...\ndef sort(items: T) -> T:  # stable\n" +
		"    \"\"\"Sort orders items in place.\"\"\"\n    ...\n\ndef trace() -> None: ...\n",
}

// The backends are the write half of the module. The kernel suite runs the
// render checks over each on the canonical fixture, in the indentation of
// Python, and the stamp check joins each to the output contract under the
// comment forms of the module.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		backendtest.RunBackendSuite(t, setup)

		t.Run("returns a backend named after the language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Name(), python.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), python.Target, "the rendering target")
		})

		t.Run("returns a backend whose spoke spells a type of another language", func(t *testing.T) {
			t.Parallel()

			got, err := backend.New().SpellType(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got.Spelling, "str", "the backend's spoke is spell.Type")
		})

		t.Run("returns a backend whose policies are the choices of the spoke", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Policies(), python.Policies(), "the policies of the satellite")
		})

		t.Run("returns a backend that overloads a function", func(t *testing.T) {
			t.Parallel()

			assert.True(t, backend.New().Overloads(), "a module declares the overloads of a function")
		})

		t.Run("returns a backend whose files stamp under the module's contract", func(t *testing.T) {
			t.Parallel()

			c, err := output.NewContract(contractBrand, python.Syntax())
			assert.NoError(t, err, "the module contract composes")
			backendtest.AssertStamped(t, setup, c)
		})

		t.Run("returns a backend that writes each module of the canonical fixture", func(t *testing.T) {
			t.Parallel()

			files, _ := backendtest.Rendered(t, setup)
			got := map[string]string{}
			for _, f := range files {
				got[f.Path] = string(f.Body)
			}
			assert.Equal(t, got, canonicalModules, "the module of each unit that renders")
		})

		t.Run("returns a backend that writes the syntax of the version of the policy", func(t *testing.T) {
			t.Parallel()

			files, _ := backendtest.Rendered(t, func(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
				b, f := setup(tb)
				f.Policy = versionPolicy(tb, python.Version312)
				return b, f
			})
			got := map[string]string{}
			for _, f := range files {
				got[f.Path] = string(f.Body)
			}
			assert.Equal(t, got["svc/id_gen.py"], "type Id = string  # opaque\n\"\"\"ID names a row.\"\"\"\n\n"+
				"type Match[T: Codec] = Keyed[T]\n\"\"\"Match names a keyed lookup.\"\"\"\n", "the type statements")
		})

		t.Run("returns a backend that writes the overload signatures of a function before its implementation",
			func(t *testing.T) {
				t.Parallel()

				body, diags := rendered(t, backend.New(), versionPolicy(t, python.Version311),
					loader("object", false), loader("int", true))
				assert.Empty(t, diags, "the render reports nothing")
				assert.ContainsInOrder(t, body, []string{
					"@overload\ndef load(id: int) -> None: ...\n", "def load(id: object) -> None: ...\n",
				}, "the signature before the implementation")
			})

		t.Run("returns a backend that skips the overload signatures of a function without an implementation",
			func(t *testing.T) {
				t.Parallel()

				body, diags := rendered(t, backend.New(), versionPolicy(t, python.Version311),
					&emit.Constant{Name: "LIMIT", Value: "8"}, loader("int", true))
				assert.Length(t, diags, 1, "one finding")
				expect.Equal(t, diags[0].Code, render.UnimplementedOverload, "Python requires the implementation")
				expect.NotContains(t, body, "def "+loadName, "the signature does not render")
			})

		t.Run("returns a backend that writes a class after the class that it extends", func(t *testing.T) {
			t.Parallel()

			base := &emit.Struct{Name: baseName}
			derived := &emit.Struct{Name: derivedName, Extends: []*emit.TypeRef{{Spelling: baseName}}}
			body, diags := rendered(t, backend.New(), versionPolicy(t, python.Version311), derived, base)
			assert.Empty(t, diags, "the render reports nothing")
			head := "class " + derivedName + "(" + baseName + ")"
			assert.ContainsInOrder(t, body, []string{"class " + baseName, head},
				"Python defines a base before a class names it")
		})
	})

	t.Run("Stubs", func(t *testing.T) {
		t.Parallel()

		backendtest.RunBackendSuite(t, stubSetup)

		t.Run("returns a backend whose version differs from the version of the backend of modules", func(t *testing.T) {
			t.Parallel()

			stubs, versioned := backend.Stubs().(plugin.Versioned)
			assert.True(t, versioned, "the backend of stub files has a version")
			modules, versioned := backend.New().(plugin.Versioned)
			assert.True(t, versioned, "the backend of modules has a version")
			assert.NotEqual(t, stubs.Version(), modules.Version(),
				"the fingerprint of a composition tells the two backends apart")
		})

		t.Run("returns a backend whose files stamp under the module's contract", func(t *testing.T) {
			t.Parallel()

			c, err := output.NewContract(contractBrand, python.Syntax())
			assert.NoError(t, err, "the module contract composes")
			backendtest.AssertStamped(t, stubSetup, c)
		})

		t.Run("returns a backend that writes each stub file of the canonical fixture", func(t *testing.T) {
			t.Parallel()

			files, _ := backendtest.Rendered(t, stubSetup)
			got := map[string]string{}
			for _, f := range files {
				got[f.Path] = string(f.Body)
			}
			assert.Length(t, got, len(canonicalModules), "one stub file for each module")
			for path, want := range canonicalStubs {
				assert.Equal(t, got[path], want, "the stub file "+path)
			}
		})

		t.Run("returns a backend that writes the overload signatures of a function without an implementation",
			func(t *testing.T) {
				t.Parallel()

				body, diags := rendered(t, backend.Stubs(), versionPolicy(t, python.Version311),
					loader("int", true), loader("str", true))
				assert.Empty(t, diags, "the render reports nothing")
				assert.ContainsInOrder(t, body, []string{
					"@overload\ndef load(id: int) -> None: ...\n", "@overload\ndef load(id: str) -> None: ...\n",
				}, "a stub file states the signatures alone")
			})

		t.Run("returns a backend that keeps the order of the flush", func(t *testing.T) {
			t.Parallel()

			base := &emit.Struct{Name: baseName}
			derived := &emit.Struct{Name: derivedName, Extends: []*emit.TypeRef{{Spelling: baseName}}}
			body, diags := rendered(t, backend.Stubs(), versionPolicy(t, python.Version311), derived, base)
			assert.Empty(t, diags, "the render reports nothing")
			head := "class " + derivedName + "(" + baseName + ")"
			assert.ContainsInOrder(t, body, []string{head, "class " + baseName},
				"a type checker reads a stub file without running it")
		})
	})
}

// The build allocates the kit's backend. The ordinary run, which runs no
// benchmark, checks that ceiling here.
func TestNewAllocs(t *testing.T) {
	checkAllocs(t, newCalls())
}

// BenchmarkNew measures the build of the backend, and the composed
// backend's settle and render of the suite's scaled corpus: the real
// templates, the lowering and the layout. The settle leaves the corpus
// build out of its count. Only -bench checks the two corpus ceilings,
// because no count of [assert.MaxAllocs] leaves the corpus build out, as
// the settle's count does.
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
			check: func(tb assert.TB) { assert.Equal(tb, b.Name(), python.Name, "New returns the Python backend") },
		},
	}
}

// setup builds the backend of modules over the kernel's canonical fixture
// in the indentation of Python.
func setup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	f := backendtest.CanonicalFixture(tb)
	adapted(tb, f)
	return backend.New(), f
}

// stubSetup builds the backend of stub files over the kernel's canonical
// fixture in the indentation of Python.
func stubSetup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	f := backendtest.CanonicalFixture(tb)
	adapted(tb, f)
	return backend.Stubs(), f
}

// benchSetup builds the backend of modules over the suite's scaled corpus
// without the kinds that the backend refuses, in the indentation of Python.
func benchSetup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	f := backendtest.ScaledFixture(tb, backend.RefusedKinds())
	adapted(tb, f)
	return backend.New(), f
}

// loader returns the function load of the module of the unit, which takes
// an id of the type param, marked as an overload signature where signature
// is set.
func loader(param string, signature bool) *emit.Function {
	return &emit.Function{
		Origin: symbol.Identity{
			Lang: python.Lang, Package: unitPkg, Name: loadName, Kind: symbol.KindFunction, Disc: param,
		},
		Name: loadName, Overload: signature,
		Params: emit.SlotOf(&emit.Param{Name: idField, Type: &emit.TypeRef{Spelling: param}}),
	}
}

// adapted states the bodies of the fixture f in the indentation of Python,
// which a body states as Python source: each tab of a verbatim body and of
// a template of the tree of a plugin is four spaces, where the kernel's
// fixture indents with a tab, as Go does.
func adapted(tb assert.TB, f *backendtest.Fixture) {
	tb.Helper()

	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			switch x := d.(type) {
			case *emit.Function:
				x.Body.Verbatim = strings.ReplaceAll(x.Body.Verbatim, "\t", python.Indent)
			case *emit.Struct:
				for _, m := range x.Methods.Items() {
					m.Body.Verbatim = strings.ReplaceAll(m.Body.Verbatim, "\t", python.Indent)
				}
			}
		}
	}
	for id, tree := range f.Trees {
		indented := fstest.MapFS{}
		err := fs.WalkDir(tree, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(tree, path)
			indented[path] = &fstest.MapFile{Data: bytes.ReplaceAll(data, []byte("\t"), []byte(python.Indent))}
			return err
		})
		assert.NoError(tb, err, "the tree of the fixture reads")
		f.Trees[id] = indented
	}
}
