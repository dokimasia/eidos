// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/gen/facade"
)

// poisonRel is the path of every injected file, and emitPath is the
// facade file the injected package renders into.
const (
	poisonRel = "emit/poison.go"
	emitPath  = "eidos-sdk/emit/facade.gen.go"
)

// unexportedType is the declaration a refusal case appends when it
// needs a name with no facade spelling.
const unexportedType = "\ntype row struct{}\n"

// renamedImport is a package that imports one package under a name of
// its own, beside a blank import, and spells the renamed one in a
// signature.
const renamedImport = "package emit\n\nimport (\n" +
	"\txdep \"example.test/dep\"\n\n\t_ \"example.test/unused\"\n)\n\n" +
	"// Named takes a renamed import's type.\nfunc Named(v xdep.T) {}\n"

// Each re-export form the renderer prints is pinned over the mini
// kernel. The committed fixture contains every form, and each way a
// surface can defeat re-export is injected and has to refuse at its
// position.
func TestRender(t *testing.T) {
	t.Parallel()

	t.Run("re-exports every declaration form", func(t *testing.T) {
		t.Parallel()

		set, err := facade.Generate(mini(t))
		assert.NoError(t, err, "the mini kernel generates")

		assert.That(t, string(set["eidos-sdk/facade.gen.go"])).
			Contains("type Rule = core.Rule", "a type re-exports as an alias").
			Contains("const Weight = core.Weight", "a constant re-declares against the kernel's").
			Contains("var Default = core.Default", "a variable re-declares against the kernel's").
			Contains("func On(k symbol.Kind, hs ...func(r *Rule) error) (Rule, error) {",
				"a function wraps under its own signature").
			Contains("return core.On(k, hs...)", "forwarding the variadic tail").
			Contains(`core "go.dokimi.dev/eidos/core"`, "the kernel counterpart imports under the fixed alias").
			Contains(`"go.dokimi.dev/eidos/sdk/symbol"`, "a qualified kernel reference respells to the facade sibling").
			Contains("[go.dokimi.dev/eidos/sdk/symbol.Kind]", "copied documentation respells kernel paths")

		assert.That(t, string(set["eidos-sdk/symbol/facade.gen.go"])).
			Contains("KindStruct = core.KindStruct", "an iota block re-declares name by name").
			Contains("Generic[T any] = core.Generic[T]", "a parameterized type re-exports as a generic alias").
			Contains("// sdk/symbol imports core/symbol.",
				"the package documentation states the computed dependency position")

		assert.That(t, string(set["eidos-sdk/plugintest/facade.gen.go"])).
			Contains(`"example.test/dep"`, "an external import is copied verbatim").
			Contains("func Check(d dep.T) {", "and its qualifier is the source's")
	})

	t.Run("prints every type expression form", func(t *testing.T) {
		t.Parallel()

		emit := poisoned(t, "package emit\n\nimport \"example.test/dep\"\n\n"+
			"// Shapes takes one of every form a signature spells.\n"+
			"func Shapes(\n"+
			"\tfixed [4]int,\n"+
			"\tsl []dep.T,\n"+
			"\tm map[string]dep.T,\n"+
			"\tsend chan<- int,\n"+
			"\trecv <-chan int,\n"+
			"\tboth chan int,\n"+
			"\tparen (*int),\n"+
			"\tfn func(a int, b string) (n int, err error),\n"+
			"\tempty interface{},\n"+
			") {\n}\n\n"+
			"// Two has two type parameters.\ntype Two[A any, B any] struct{}\n\n"+
			"// Use takes an instantiation with two arguments.\nfunc Use(p Two[int, string]) {}\n\n"+
			"// Tilde constrains by a union of underlying types.\n"+
			"func Tilde[T interface{ ~int | ~string }](v T) {}\n")

		assert.That(t, emit).
			Contains("fixed [4]int", "a fixed-length array keeps its length literal").
			Contains("sl []dep.T", "a slice keeps its element type").
			Contains("m map[string]dep.T", "a map keeps its key and value types").
			Contains("send chan<- int", "a send-only channel keeps its direction").
			Contains("recv <-chan int", "a receive-only channel keeps its direction").
			Contains("both chan int", "a bidirectional channel prints bare").
			Contains("paren *int", "a parenthesized type re-exports as the type it wraps").
			Contains("fn func(a int, b string) (n int, err error)", "a function type keeps its parameter and result names").
			Contains("empty interface{}", "an interface declaring nothing re-exports as itself").
			Contains("type Two[A any, B any] = core.Two[A, B]",
				"a two-parameter generic type re-exports as a generic alias").
			Contains("func Use(p Two[int, string])", "an instantiation with two type arguments keeps both").
			Contains("func Tilde[T interface{ ~int | ~string }](v T)",
				"a constraint interface keeps its union of underlying types").
			Contains("core.Tilde[T](v)", "and the wrapper instantiates explicitly, so inference decides nothing")
	})

	t.Run("names a parameter the kernel left without one", func(t *testing.T) {
		t.Parallel()

		emit := poisoned(t, "package emit\n\n"+
			"// Unnamed takes parameters without names.\nfunc Unnamed(int, string) {}\n\n"+
			"// Blank discards both its parameters.\nfunc Blank(_ int, _ string) {}\n\n"+
			"// Pair returns two results under one type.\nfunc Pair() (a, b int) { return 0, 0 }\n")

		assert.That(t, emit).
			Contains("func Unnamed(a0 int, a1 string)", "an unnamed parameter gets a fresh name").
			Contains("core.Unnamed(a0, a1)", "because the wrapper has to forward it").
			Contains("func Blank(a0 int, a1 string)", "a blank parameter gets a fresh name for the same reason").
			Contains("core.Blank(a0, a1)", "and forwards under it").
			Contains("func Pair() (a, b int)", "results grouped under one type keep their names")
	})

	t.Run("re-exports a grouped declaration with its trailing comments", func(t *testing.T) {
		t.Parallel()

		emit := poisoned(t, "package emit\n\n"+
			"// The shapes the group declares.\ntype (\n"+
			"\t// First is the first shape.\n\tFirst struct{} // trailing the type\n"+
			"\t// Second is the second shape.\n\tSecond struct{}\n)\n\n"+
			"// The counts the group declares.\nconst (\n"+
			"\t// Low is the low count.\n\tLow = 1 // trailing the constant\n"+
			"\tHigh = 2\n)\n")

		assert.That(t, emit).
			Contains("type (", "a grouped type declaration re-exports as a group").
			Contains("First = core.First // trailing the type", "an alias keeps the type spec's trailing comment").
			Contains("Second = core.Second", "and the spec beside it re-exports too").
			Contains("= core.Low // trailing the constant",
				"a re-declared constant keeps the value spec's trailing comment").
			Contains("// Low is the low count.", "beside the spec's own documentation")
	})

	t.Run("respells documentation through the curated table", func(t *testing.T) {
		t.Parallel()

		emit := poisoned(t, "package emit\n\n"+
			"// Linked pairs with [go.dokimi.dev/eidos/core/backend/render.Pass], runs\n"+
			"// inside [go.dokimi.dev/eidos/core/workspace] and belongs to\n"+
			"// [go.dokimi.dev/eidos/core].\n"+
			"type Linked struct{} // see go.dokimi.dev/eidos/core/frontend/frontendtest/\n")

		assert.That(t, emit).
			Contains("[go.dokimi.dev/eidos/sdk/render.Pass]",
				"a nested curated package respells to its flat facade path").
			Contains("[go.dokimi.dev/eidos/core/workspace]",
				"an uncurated package keeps its kernel path, because no facade path exists for it").
			Contains("[go.dokimi.dev/eidos/sdk].", "the kernel root respells to the facade root").
			Contains("// see go.dokimi.dev/eidos/sdk/frontendtest/",
				"a trailing comment respells the same way, its closing slash kept")
	})

	t.Run("keeps a renamed import's name", func(t *testing.T) {
		t.Parallel()

		assert.That(t, poisoned(t, renamedImport)).
			Contains("xdep \"example.test/dep\"", "an import the kernel renamed keeps its name in the facade").
			Contains("func Named(v xdep.T)", "so the qualifier the signature spells still resolves")
	})

	t.Run("drops a blank import", func(t *testing.T) {
		t.Parallel()

		assert.NotContains(t, poisoned(t, renamedImport), "example.test/unused",
			"a blank import names nothing a signature can spell, so the facade drops it")
	})

	t.Run("returns an error for what re-export cannot print", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			poison string
			wants  []string
		}{
			{
				name:   "an unexported type in an exported signature",
				poison: "// Fetch returns a row.\nfunc Fetch() row { return row{} }\n" + unexportedType,
				wants:  []string{"unexported row"},
			},
			{
				name: "a kernel package outside the curated list",
				poison: "import \"go.dokimi.dev/eidos/core/workspace\"\n\n" +
					"// Run runs w.\nfunc Run(w workspace.Workspace) {}\n",
				wants: []string{"leaks", "go.dokimi.dev/eidos/core/workspace"},
			},
			{
				name:   "an anonymous struct",
				poison: "// Shape takes an anonymous struct.\nfunc Shape(x struct{ N int }) {}\n",
				wants:  []string{"unsupported", "*ast.StructType"},
			},
			{
				name:   "an anonymous interface declaring a method",
				poison: "// Read takes a reader.\nfunc Read(r interface{ Read() error }) {}\n",
				wants:  []string{"anonymous interface with methods"},
			},
			{
				name:   "a qualifier resolving to no import",
				poison: "// Q takes an unresolvable qualifier.\nfunc Q(v nosuch.T) {}\n",
				wants:  []string{"qualifier nosuch", "no import"},
			},
			{
				name: "one import name serving two packages",
				poison: "import \"example.test/core\"\n\n" +
					"// C collides with the kernel counterpart's alias.\nfunc C(v core.T) {}\n",
				wants: []string{
					"import name core", "example.test/core", "go.dokimi.dev/eidos/core",
				},
			},
			{
				name:   "an unexported constraint on a function",
				poison: "// G is constrained by an unexported name.\nfunc G[T row]() {}\n" + unexportedType,
				wants:  []string{"unexported row"},
			},
			{
				name:   "an unexported constraint on a type",
				poison: "// G is constrained by an unexported name.\ntype G[T row] struct{}\n" + unexportedType,
				wants:  []string{"unexported row"},
			},
			{
				name:   "an unexported slice element",
				poison: "// A returns rows.\nfunc A() []row { return nil }\n" + unexportedType,
				wants:  []string{"unexported row"},
			},
			{
				name:   "an unexported map key",
				poison: "// M takes a map keyed by a row.\nfunc M(m map[row]int) {}\n" + unexportedType,
				wants:  []string{"unexported row"},
			},
			{
				name:   "an unexported generic base",
				poison: "// B takes an instantiation of an unexported type.\nfunc B(v row[int]) {}\n" + unexportedType,
				wants:  []string{"unexported row"},
			},
			{
				name: "an unexported type argument",
				poison: "import \"go.dokimi.dev/eidos/core/symbol\"\n\n" +
					"// I takes a kernel generic over an unexported type.\n" +
					"func I(v symbol.Generic[row]) {}\n" + unexportedType,
				wants: []string{"unexported row"},
			},
			{
				name:   "an unexported parameter of a function type",
				poison: "// P takes a callback over a row.\nfunc P(f func(row)) {}\n" + unexportedType,
				wants:  []string{"unexported row"},
			},
			{
				name:   "an unexported result of a function type",
				poison: "// R takes a callback returning a row.\nfunc R(f func() row) {}\n" + unexportedType,
				wants:  []string{"unexported row"},
			},
			{
				name: "an unexported element in a constraint union",
				poison: "// U is constrained by an unexported underlying type.\n" +
					"func U[T interface{ ~row | ~int }]() {}\n" + unexportedType,
				wants: []string{"unexported row"},
			},
			{
				name: "a qualified expression syntax cannot resolve",
				poison: "import \"example.test/dep\"\n\n" +
					"// N takes an array sized by a nested selector.\nfunc N(v [dep.Sub.N]int) {}\n",
				wants: []string{"qualifier of .N", "not a package name"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				text := refused(t, "package emit\n\n"+tt.poison).Error()
				for _, want := range tt.wants {
					assert.Contains(t, text, want, "the refusal names what defeated re-export")
				}
				assert.That(t, text).
					Contains("poison.go:", "at the kernel position an author can go and fix").
					HasPrefix("facade: ", "under the package prefix")
			})
		}
	})

	t.Run("returns an error for a surface re-exporting nothing", func(t *testing.T) {
		t.Parallel()

		root := mini(t)
		files.Write(t, filepath.Join(root, facade.KernelDir),
			files.Tree{"plugin/plugin.go": files.Text("package plugin\n\ntype marker struct{}\n")})
		_, err := facade.Generate(root)
		assert.HasError(t, err, "an empty facade package proves the list wrong")
		assert.That(t, err.Error()).
			Contains("re-exports nothing", "the refusal states the reason").
			Contains("go.dokimi.dev/eidos/core/plugin", "and names the kernel package that re-exported none of itself")
	})
}

// mini copies the testdata mini kernel into a fresh repository
// root, so a case can poison one package without touching the
// committed fixture.
func mini(tb testing.TB) string {
	tb.Helper()

	return coretest.CopyTree(tb, filepath.Join("testdata", "mini"))
}

// poisoned generates a mini kernel with one injected file and
// returns the source of the facade file the injection rendered into.
func poisoned(t *testing.T, content string) string {
	t.Helper()

	root := mini(t)
	files.Write(t, filepath.Join(root, facade.KernelDir), files.Tree{poisonRel: files.Text(content)})
	set, err := facade.Generate(root)
	assert.NoError(t, err, "the poisoned mini kernel generates")
	return string(set[emitPath])
}

// refused generates a mini kernel with one injected file and
// returns the refusal it produced.
func refused(t *testing.T, content string) error {
	t.Helper()

	root := mini(t)
	files.Write(t, filepath.Join(root, facade.KernelDir), files.Tree{poisonRel: files.Text(content)})
	_, err := facade.Generate(root)
	assert.HasError(t, err, "the injected surface is refused")
	return err
}
