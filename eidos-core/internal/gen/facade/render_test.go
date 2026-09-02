// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
)

// poisonRel is where every injected file lands, and emitPath the
// facade file the injected package renders into.
const (
	poisonRel = "emit/poison.go"
	emitPath  = "eidos-sdk/emit/facade.gen.go"
)

// unexportedType is the declaration each refusal case reaches for
// when it needs a name with no facade spelling.
const unexportedType = "\ntype row struct{}\n"

// mini copies the testdata mini kernel into a fresh repository
// root, so a case can poison one package without touching the
// committed fixture.
func mini(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	src := filepath.Join("testdata", "mini")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	assert.NoError(t, err, "the mini kernel copies")
	return root
}

// poison writes one file into a mini kernel copy.
func poison(t *testing.T, root, rel, content string) {
	t.Helper()

	target := filepath.Join(root, "eidos-core", filepath.FromSlash(rel))
	assert.NoError(t, os.WriteFile(target, []byte(content), 0o644), "the poison writes")
}

// poisoned generates a mini kernel holding one injected file and
// returns the facade package the injection rendered into.
func poisoned(t *testing.T, content string) string {
	t.Helper()

	root := mini(t)
	poison(t, root, poisonRel, content)
	set, err := facade.Generate(root)
	assert.NoError(t, err, "the poisoned mini kernel generates")
	return string(set[emitPath])
}

// refused generates a mini kernel holding one injected file and
// returns the refusal it produced.
func refused(t *testing.T, content string) error {
	t.Helper()

	root := mini(t)
	poison(t, root, poisonRel, content)
	_, err := facade.Generate(root)
	assert.HasError(t, err, "the injected surface is refused")
	return err
}

// The renderer owns the re-export forms, so each one is pinned
// over the mini kernel: the committed fixture holds every form,
// and each way a surface can defeat re-export is injected and has
// to refuse at its position.
func TestRender(t *testing.T) {
	t.Parallel()

	t.Run("re-exports every declaration form", func(t *testing.T) {
		t.Parallel()

		set, err := facade.Generate(mini(t))
		assert.NoError(t, err, "the mini kernel generates")

		root := string(set["eidos-sdk/facade.gen.go"])
		assert.Contains(t, root, "type Rule = core.Rule",
			"a type re-exports as an alias")
		assert.Contains(t, root, "const Weight = core.Weight",
			"a constant re-declares against the kernel's")
		assert.Contains(t, root, "var Default = core.Default",
			"a variable re-declares against the kernel's")
		assert.Contains(t, root,
			"func On(k symbol.Kind, hs ...func(r *Rule) error) (Rule, error) {",
			"a function wraps under its own signature")
		assert.Contains(t, root, "return core.On(k, hs...)",
			"forwarding the variadic tail")
		assert.Contains(t, root, `core "go.dokimi.dev/eidos/core"`,
			"the kernel counterpart imports under the fixed alias")
		assert.Contains(t, root, `"go.dokimi.dev/eidos/sdk/symbol"`,
			"a qualified kernel reference respells to the facade sibling")
		assert.Contains(t, root, "[go.dokimi.dev/eidos/sdk/symbol.Kind]",
			"carried documentation respells kernel paths")

		sym := string(set["eidos-sdk/symbol/facade.gen.go"])
		assert.Contains(t, sym, "KindStruct = core.KindStruct",
			"an iota block re-declares name by name")
		assert.Contains(t, sym, "Generic[T any] = core.Generic[T]",
			"a parameterized type re-exports as a generic alias")
		assert.Contains(t, sym, "// sdk/symbol imports core/symbol.",
			"the package documentation states the computed dependency position")

		kit := string(set["eidos-sdk/plugintest/facade.gen.go"])
		assert.Contains(t, kit, `"example.test/dep"`,
			"an external import carries verbatim")
		assert.Contains(t, kit, "func Check(d dep.T) {",
			"and its qualifier stays the source's")
	})

	t.Run("carries every type expression form", func(t *testing.T) {
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
			"// Two carries two type parameters.\ntype Two[A any, B any] struct{}\n\n"+
			"// Use takes an instantiation with two arguments.\nfunc Use(p Two[int, string]) {}\n\n"+
			"// Tilde constrains by a union of underlying types.\n"+
			"func Tilde[T interface{ ~int | ~string }](v T) {}\n")

		assert.Contains(t, emit, "fixed [4]int",
			"a fixed-length array carries its length literal")
		assert.Contains(t, emit, "sl []dep.T",
			"a slice carries its element")
		assert.Contains(t, emit, "m map[string]dep.T",
			"a map carries its key and its value")
		assert.Contains(t, emit, "send chan<- int",
			"a send-only channel keeps its direction")
		assert.Contains(t, emit, "recv <-chan int",
			"a receive-only channel keeps its direction")
		assert.Contains(t, emit, "both chan int",
			"a bidirectional channel prints bare")
		assert.Contains(t, emit, "paren *int",
			"a parenthesized type re-exports as the type it wraps")
		assert.Contains(t, emit, "fn func(a int, b string) (n int, err error)",
			"a function type carries its parameter and result names")
		assert.Contains(t, emit, "empty interface{}",
			"an interface declaring nothing re-exports as itself")
		assert.Contains(t, emit, "type Two[A any, B any] = core.Two[A, B]",
			"a two-parameter generic type re-exports as a generic alias")
		assert.Contains(t, emit, "func Use(p Two[int, string])",
			"an instantiation with two type arguments carries both")
		assert.Contains(t, emit, "func Tilde[T interface{ ~int | ~string }](v T)",
			"a constraint interface carries its union of underlying types")
		assert.Contains(t, emit, "core.Tilde[T](v)",
			"and the wrapper instantiates explicitly, so inference decides nothing")
	})

	t.Run("names a parameter the kernel left without one", func(t *testing.T) {
		t.Parallel()

		emit := poisoned(t, "package emit\n\n"+
			"// Unnamed takes positions rather than names.\nfunc Unnamed(int, string) {}\n\n"+
			"// Blank discards both its parameters.\nfunc Blank(_ int, _ string) {}\n\n"+
			"// Pair returns two results under one type.\nfunc Pair() (a, b int) { return 0, 0 }\n")

		assert.Contains(t, emit, "func Unnamed(a0 int, a1 string)",
			"an unnamed parameter gets a fresh name")
		assert.Contains(t, emit, "core.Unnamed(a0, a1)",
			"because the wrapper has to forward it")
		assert.Contains(t, emit, "func Blank(a0 int, a1 string)",
			"a blank parameter gets a fresh name for the same reason")
		assert.Contains(t, emit, "core.Blank(a0, a1)",
			"and forwards under it")
		assert.Contains(t, emit, "func Pair() (a, b int)",
			"results grouped under one type keep their names")
	})

	t.Run("carries a grouped declaration and its trailing comments", func(t *testing.T) {
		t.Parallel()

		emit := poisoned(t, "package emit\n\n"+
			"// The shapes the group declares.\ntype (\n"+
			"\t// First is the first shape.\n\tFirst struct{} // trailing the type\n"+
			"\t// Second is the second shape.\n\tSecond struct{}\n)\n\n"+
			"// The counts the group declares.\nconst (\n"+
			"\t// Low is the low count.\n\tLow = 1 // trailing the constant\n"+
			"\tHigh = 2\n)\n")

		assert.Contains(t, emit, "type (", "a grouped type declaration re-exports as a group")
		assert.Contains(t, emit, "First = core.First // trailing the type",
			"an alias carries the type spec's trailing comment")
		assert.Contains(t, emit, "Second = core.Second",
			"and the spec beside it re-exports too")
		assert.Contains(t, emit, "= core.Low // trailing the constant",
			"a re-declared constant carries the value spec's trailing comment")
		assert.Contains(t, emit, "// Low is the low count.",
			"beside the spec's own documentation")
	})

	t.Run("carries an import's name and drops a blank one", func(t *testing.T) {
		t.Parallel()

		emit := poisoned(t, "package emit\n\nimport (\n"+
			"\txdep \"example.test/dep\"\n\n\t_ \"example.test/unused\"\n)\n\n"+
			"// Named takes a renamed import's type.\nfunc Named(v xdep.T) {}\n")

		assert.Contains(t, emit, "xdep \"example.test/dep\"",
			"an import the kernel renamed keeps its name in the facade")
		assert.Contains(t, emit, "func Named(v xdep.T)",
			"so the qualifier the signature spells still resolves")
		assert.NotContains(t, emit, "example.test/unused",
			"a blank import names nothing a signature can spell, so the facade drops it")
	})

	t.Run("refuses what re-export cannot carry", func(t *testing.T) {
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

				err := refused(t, "package emit\n\n"+tt.poison)
				for _, want := range tt.wants {
					assert.Contains(t, err.Error(), want,
						"the refusal names what defeated re-export")
				}
				assert.Contains(t, err.Error(), "poison.go:",
					"at the kernel position an author can go and fix")
				assert.HasPrefix(t, err.Error(), "facade: ", "under the package prefix")
			})
		}
	})

	t.Run("refuses a surface re-exporting nothing", func(t *testing.T) {
		t.Parallel()

		root := mini(t)
		poison(t, root, "plugin/plugin.go", "package plugin\n\ntype marker struct{}\n")
		_, err := facade.Generate(root)
		assert.HasError(t, err, "an empty facade package proves the list wrong")
		assert.Contains(t, err.Error(), "re-exports nothing", "the refusal says why")
		assert.Contains(t, err.Error(), "go.dokimi.dev/eidos/core/plugin",
			"and names the kernel package that re-exported none of itself")
	})
}
