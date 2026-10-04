// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter_test

import (
	"context"
	"path"
	"runtime/debug"
	"slices"
	"testing"
	"unsafe"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsjava "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tsrust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/treesitter/java"
	"go.dokimi.dev/eidos/lang/treesitter/rust"
	"go.dokimi.dev/eidos/lang/treesitter/typescript"
)

// cleanJava declares a class with a two-name field and a method.
// brokenJava has an ERROR node on line 2 and a MISSING semicolon on
// line 3. In wideJava, the second field follows a two-byte character
// on its line.
const (
	javaFile   = "src/A.java"
	cleanJava  = "class A {\n  int a, b;\n  void run() {}\n}\n"
	brokenJava = "class A {\n  int = ;\n  void run() { int x = 1 }\n}\n"
	wideJava   = "class A {\n  String s = \"é\"; int b;\n}\n"
)

// The modules whose build versions the Java grammar's version names,
// and a module no build lists.
const (
	javaModule    = "github.com/tree-sitter/tree-sitter-java"
	runtimeModule = "github.com/tree-sitter/go-tree-sitter"
	absentModule  = "example.com/absent"
)

// The Java grammar's name, the node kinds and fields the cases walk
// by, and a kind id the grammar does not assign.
const (
	javaName        = "java"
	programKind     = "program"
	classKind       = "class_declaration"
	classBodyKind   = "class_body"
	fieldDeclKind   = "field_declaration"
	methodDeclKind  = "method_declaration"
	identifierKind  = "identifier"
	errorKind       = "ERROR"
	nameField       = "name"
	declaratorField = "declarator"
	superclassField = "superclass"
	absentName      = "no_such_node"
)

// unsupportedABI is an ABI version no runtime accepts, and fakeName
// the name the fake language loads under.
const (
	unsupportedABI = 99
	fakeName       = "fake"
)

// The allocations of a Load beside the names it copies from C: the
// language handle, the grammar, its version and its list of kinds, and
// the two maps, each its header, its directory, one table and the
// table's groups.
const (
	loadFixedAllocs = 4
	mapAllocs       = 4
)

// treeAllocs is a parse's [treesitter.Tree], its one allocation on the
// Go heap.
const treeAllocs = 1

// allocRuns is how many calls an allocation check makes: one to warm
// up and the hundred it counts.
const allocRuns = 101

// pinned is one grammar the layer pins, and the language its binding
// returns, which is the authority the grammar's tables are checked
// against.
type pinned struct {
	grammar  *treesitter.Grammar
	language *ts.Language
}

// allocCall is one call that an allocation test and a benchmark share:
// the method it calls, which names its benchmark, the case it measures
// where the method has more than one call, its allocation ceiling, the
// call, and the check of the result the call leaves. A case whose
// ceiling only a benchmark checks sets bench, which measures the case
// in place of the call, and no list an allocation test reads contains
// it.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
	bench    func(b *testing.B)
}

// A grammar resolves its tables once and parses any input, so its
// lookups are pinned against the trees its own parser returns.
func TestGrammar(t *testing.T) {
	t.Parallel()

	g := java.Grammar

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("panics on a language whose ABI version the runtime does not accept", func(t *testing.T) {
			t.Parallel()

			// The runtime reads a language's ABI version from its first
			// field, and Load reads nothing else before it refuses.
			fake := [16]uint32{unsupportedABI}
			got := assert.Panics(t, func() {
				treesitter.Load(fakeName, javaModule, unsafe.Pointer(&fake))
			}, "a language the runtime cannot drive")
			assert.Contains(t, got, "ABI version 99", "the panic names the version")
		})

		t.Run("panics on a module the build information does not list", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				treesitter.Load(javaName, absentModule, tsjava.Language())
			}, "a grammar whose version no key could fold")
			assert.Contains(t, got, absentModule, "the panic names the module")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the grammar's name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, g.Name(), javaName, "the grammar package loads under its language's name")
		})
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the grammar's module before the runtime's with their build versions", func(t *testing.T) {
			t.Parallel()

			want := path.Base(javaModule) + " " + buildVersion(t, javaModule) + ", " +
				path.Base(runtimeModule) + " " + buildVersion(t, runtimeModule)
			assert.Equal(t, g.Version(), want, "both modules and their versions, grammar first")
		})
	})

	t.Run("Kind", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the id every named node of a parsed tree reports", func(t *testing.T) {
			t.Parallel()

			for _, src := range []string{cleanJava, wideJava} {
				named(parse(t, src).Root(), func(n treesitter.Node) {
					assert.Equal(t, g.Kind(g.KindName(n.Kind())), n.Kind(),
						"the name of a node's kind resolves to the node's own id")
				})
			}
		})

		t.Run("returns the id an ERROR node reports for ERROR", func(t *testing.T) {
			t.Parallel()

			errs := slices.Collect(parse(t, brokenJava).Errors())
			assert.True(t, errs[0].IsError(), "the first error is an ERROR node")
			assert.Equal(t, errs[0].Kind(), g.Kind(errorKind), "the runtime's error kind")
		})

		t.Run("panics on a name the grammar does not declare", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() { g.Kind(absentName) }, "a kind of another grammar")
			assert.Contains(t, got, absentName, "the panic names the kind")
		})
	})

	t.Run("KindName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name of a node's kind", func(t *testing.T) {
			t.Parallel()

			class := first(parse(t, cleanJava).Root(), g.Kind(classKind))
			assert.Equal(t, g.KindName(class.Kind()), classKind, "the name the grammar spells")
		})

		t.Run("returns ERROR for the error kind", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, g.KindName(g.Kind(errorKind)), errorKind, "the runtime's name for its error kind")
		})

		t.Run("returns the runtime's name for every id of every grammar", func(t *testing.T) {
			t.Parallel()

			for _, p := range allPinned() {
				for id := range uint16(p.language.NodeKindCount()) {
					assert.Equal(t, p.grammar.KindName(treesitter.Kind(id)), p.language.NodeKindForId(id),
						"the name the runtime's table spells")
				}
			}
		})

		t.Run("returns nothing for the first id past a grammar's kinds", func(t *testing.T) {
			t.Parallel()

			for _, p := range allPinned() {
				past := treesitter.Kind(p.language.NodeKindCount())
				assert.Empty(t, p.grammar.KindName(past), "no kind has the id")
			}
		})
	})

	t.Run("Field", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the runtime's id for every field name of every grammar", func(t *testing.T) {
			t.Parallel()

			for _, p := range allPinned() {
				for id := range uint16(p.language.FieldCount()) {
					field := id + 1
					assert.Equal(t, p.grammar.Field(p.language.FieldNameForId(field)), treesitter.Field(field),
						"field ids count from one, as the runtime's table does")
				}
			}
		})

		t.Run("returns the id a child reports under its field name", func(t *testing.T) {
			t.Parallel()

			class := first(parse(t, cleanJava).Root(), g.Kind(classKind))
			assert.Equal(t, class.Child(g.Field(nameField)).Text(), "A", "the class's name child")
		})

		t.Run("panics on a name the grammar does not declare", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() { g.Field(absentName) }, "a field of another grammar")
			assert.Contains(t, got, absentName, "the panic names the field")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the context's error for a done context", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			tree, err := g.Parse(ctx, javaFile, []byte(cleanJava))
			assert.ErrorIs(t, err, context.Canceled, "a done context parses nothing")
			assert.Nil(t, tree, "no tree is returned")
		})

		t.Run("returns a tree for source outside the grammar", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, brokenJava)
			assert.Equal(t, tree.Root().Kind(), g.Kind(programKind), "the root is the grammar's own")
		})

		t.Run("returns a tree without errors for an empty source", func(t *testing.T) {
			t.Parallel()

			tree, err := g.Parse(context.Background(), javaFile, nil)
			assert.NoError(t, err, "an empty source parses")
			t.Cleanup(tree.Close)
			assert.Equal(t, tree.Root().Kind(), g.Kind(programKind), "an empty program")
			assert.Empty(t, slices.Collect(tree.Errors()), "nothing is outside the grammar")
		})

		t.Run("returns a tree with the file in every position", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, parse(t, cleanJava).Root().Pos().File, javaFile, "the path the parse was given")
		})
	})
}

// Load allocates the grammar's tables, Parse its tree, and every lookup
// nothing. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestGrammarAllocs(t *testing.T) {
	checkAllocs(t, grammarCalls(t))
	ctx, src := context.Background(), []byte(cleanJava)
	var (
		tree *treesitter.Tree
		err  error
	)
	parsed := func() {
		tree, err = java.Grammar.Parse(ctx, javaFile, src)
		tree.Close()
	}
	assert.MaxAllocs(t, parsed, treeAllocs, "Parse allocates the tree")
	assert.NoError(t, err, "Parse parses the source")
}

// BenchmarkGrammar measures the load a grammar package makes once, each
// lookup a frontend makes when it is built, and a parse of one file.
func BenchmarkGrammar(b *testing.B) {
	benchCalls(b, grammarCalls(b))

	b.Run("Parse", func(b *testing.B) {
		ctx, src := context.Background(), []byte(cleanJava)
		c := bench.Start(b).MaxAllocs(treeAllocs)
		defer c.End()
		var (
			tree *treesitter.Tree
			err  error
		)
		for c.Loop() {
			tree, err = java.Grammar.Parse(ctx, javaFile, src)
			c.Excluding(tree.Close)
		}
		assert.NoError(b, err, "Parse parses the source")
		assert.True(b, tree.Root().IsZero(), "the last tree is closed")
	})
}

// grammarCalls returns a call of Load and of every lookup of the Java
// grammar.
func grammarCalls(tb testing.TB) []allocCall {
	tb.Helper()

	g := java.Grammar
	loadCeiling := loadAllocs(tb, ts.NewLanguage(tsjava.Language()))
	var (
		loaded *treesitter.Grammar
		text   string
		kind   treesitter.Kind
		field  treesitter.Field
	)
	return []allocCall{
		{
			name: "Load", allocs: loadCeiling,
			call: func() { loaded = treesitter.Load(javaName, javaModule, tsjava.Language()) },
			check: func(tb assert.TB) {
				assert.Equal(tb, loaded.Version(), g.Version(), "Load reads the versions the package read")
			},
		},
		{
			name: "Name", call: func() { text = g.Name() },
			check: func(tb assert.TB) { assert.Equal(tb, text, javaName, "Name returns the name") },
		},
		{
			name: "Version", call: func() { text = g.Version() },
			check: func(tb assert.TB) { assert.Contains(tb, text, path.Base(javaModule), "Version names the module") },
		},
		{
			name: "Kind", call: func() { kind = g.Kind(classKind) },
			check: func(tb assert.TB) { assert.Equal(tb, g.KindName(kind), classKind, "Kind returns the class's id") },
		},
		{
			name: "KindName", call: func() { text = g.KindName(kind) },
			check: func(tb assert.TB) { assert.Equal(tb, text, classKind, "KindName returns the class's name") },
		},
		{
			name: "Field", call: func() { field = g.Field(nameField) },
			check: func(tb assert.TB) { assert.NotEqual(tb, field, 0, "Field returns the name field's id") },
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
// the contract starts, so what the first call initialises stays out of
// the count. A case that sets bench runs it instead.
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

// loadAllocs returns the allocations of a Load of a language: one copy
// of each of its kind names and field names that is not empty, and the
// allocations every Load makes.
func loadAllocs(tb testing.TB, lang *ts.Language) uint64 {
	tb.Helper()

	names := uint64(0)
	for id := range uint16(lang.NodeKindCount()) {
		if lang.NodeKindForId(id) != "" {
			names++
		}
	}
	for id := range uint16(lang.FieldCount()) {
		if lang.FieldNameForId(id+1) != "" {
			names++
		}
	}
	return names + loadFixedAllocs + 2*mapAllocs
}

// allPinned returns every grammar the layer pins, each with its
// binding's language.
func allPinned() []pinned {
	return []pinned{
		{grammar: java.Grammar, language: ts.NewLanguage(tsjava.Language())},
		{grammar: rust.Grammar, language: ts.NewLanguage(tsrust.Language())},
		{grammar: typescript.TypeScript, language: ts.NewLanguage(tsts.LanguageTypescript())},
		{grammar: typescript.TSX, language: ts.NewLanguage(tsts.LanguageTSX())},
	}
}

// parse parses one Java source under javaFile and closes the tree
// when the test ends.
func parse(tb testing.TB, src string) *treesitter.Tree {
	tb.Helper()

	tree, err := java.Grammar.Parse(context.Background(), javaFile, []byte(src))
	assert.NoError(tb, err, "the source parses")
	tb.Cleanup(tree.Close)
	return tree
}

// first returns the first node of a kind in document order below and
// including n, and the zero Node when there is none.
func first(n treesitter.Node, kind treesitter.Kind) treesitter.Node {
	if n.Kind() == kind {
		return n
	}
	for child := range n.NamedChildren() {
		if found := first(child, kind); !found.IsZero() {
			return found
		}
	}
	return treesitter.Node{}
}

// named yields every named node below and including n, in document
// order.
func named(n treesitter.Node, yield func(treesitter.Node)) {
	yield(n)
	for child := range n.NamedChildren() {
		named(child, yield)
	}
}

// buildVersion returns a module's build version, as the build
// information of the test binary records it.
func buildVersion(tb testing.TB, module string) string {
	tb.Helper()

	info, built := debug.ReadBuildInfo()
	assert.True(tb, built, "the test binary records its build information")
	for _, dep := range info.Deps {
		if dep.Path == module {
			return dep.Version
		}
	}
	tb.Fatalf("the build information does not list %s", module)
	return ""
}
