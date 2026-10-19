// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/treesitter/java"
	"go.dokimi.dev/eidos/lang/treesitter/typescript"
	"go.dokimi.dev/eidos/sdk/position"
)

// The kinds of the comment cases beyond the shared kinds, which are a block
// comment and the body of a method.
const (
	blockCommentKind = "block_comment"
	methodBodyKind   = "block"
)

// The Java sources of the comment cases.
const (
	// runJava has a run of two line comments above its field.
	runJava = "class A {\n  // first\n  // second\n  int a;\n}\n"
	// gapJava has a blank line inside the comments above its field.
	gapJava = "class A {\n  // far\n\n  // near\n  int a;\n}\n"
	// tailJava has a field with a trailing comment before a second field.
	tailJava = "class A {\n  int a; // of a\n  int b;\n}\n"
	// passJava has a comment above a field that a case passes over.
	passJava = "class A {\n  // doc\n  int a;\n  int b;\n}\n"
	// stopJava has two line comments above its field.
	stopJava = "class A {\n  // module\n  // doc\n  int a;\n}\n"
	// innerJava has a block comment inside its field.
	innerJava = "class A {\n  int /* inside */ a;\n}\n"
	// nextJava has a comment on the line after its first field.
	nextJava = "class A {\n  int a;\n  // next\n  int b;\n}\n"
	// mixedJava has a block comment and a line comment above its field.
	mixedJava = "class A {\n  /* block */\n  // line\n  int a;\n}\n"
	// bodyJava has a comment inside the body of a method.
	bodyJava = "class A {\n  void run() {\n    // statement\n  }\n}\n"
)

// foldedTS is a TypeScript declaration without a semicolon, whose grammar
// folds the trailing comment into the declaration.
const foldedTS = "let a = 1 // tail\n"

// The TypeScript fixture of the folded case. The block has its path, the
// kind of its declaration and of its comments, and the marker of a line
// comment.
const (
	tsFile        = "src/a.ts"
	lexicalKind   = "lexical_declaration"
	tsCommentKind = "comment"
	tsLineMarker  = "//"
)

// The allocations of the comment walk over the run of runJava.
const (
	// leadingAllocs is the count of the run of two comments. Leading
	// allocates the list, which grows twice.
	leadingAllocs = 2
	// runsAllocs is the count of the text of a run of two lines, on a walk
	// whose buffer and set already contain the run.
	runsAllocs = 1
)

// run is one text that [treesitter.Comments.Runs] yields, and its position.
type run struct {
	text string
	at   position.Pos
}

// TestComments checks that the comment walk attributes the comments of a
// tree by one algorithm, and that each parameter changes one rule of it.
func TestComments(t *testing.T) {
	t.Parallel()

	g := java.Grammar

	t.Run("Leading", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the run of comments above a declaration", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, runJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, texts(javaComments().Leading(field)), []string{"// first", "// second"},
				"the run is the two line comments in source order")
		})

		t.Run("ends the run at a blank line", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, gapJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, texts(javaComments().Leading(field)), []string{"// near"},
				"the comment above the blank line is not in the run")
		})

		t.Run("leaves out the trailing comment of the sibling before", func(t *testing.T) {
			t.Parallel()

			second := first(parse(t, tailJava).Root(), g.Kind(fieldDeclKind)).NextNamedSibling()
			assert.Empty(t, javaComments().Leading(second), "the comment on the line of the first field is its own")
		})

		t.Run("passes over a sibling that Passes reports", func(t *testing.T) {
			t.Parallel()

			second := first(parse(t, passJava).Root(), g.Kind(fieldDeclKind)).NextNamedSibling()
			c := javaComments()
			c.Passes = func(n treesitter.Node) bool { return n.Kind() == g.Kind(fieldDeclKind) }
			assert.Equal(t, texts(c.Leading(second)), []string{"// doc"},
				"the comment above the passed field joins the run of the second field")
		})

		t.Run("ends the run at a comment that Stops reports", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, stopJava).Root(), g.Kind(fieldDeclKind))
			c := javaComments()
			c.Stops = func(n treesitter.Node) bool { return n.Text() == "// module" }
			assert.Equal(t, texts(c.Leading(field)), []string{"// doc"}, "the run ends below the stopping comment")
		})

		t.Run("ends the run at a comment that another subject took", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, stopJava)
			c := javaComments()
			c.Take(commentsOf(c, first(tree.Root(), g.Kind(classBodyKind)))[0])
			assert.Equal(t, texts(c.Leading(first(tree.Root(), g.Kind(fieldDeclKind)))), []string{"// doc"},
				"the taken comment belongs to its subject")
		})

		t.Run("uses LastLine for the line on which a comment ends", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, gapJava).Root(), g.Kind(fieldDeclKind))
			c := javaComments()
			c.LastLine = func(n treesitter.Node) int { return n.End().Line + 1 }
			assert.Equal(t, texts(c.Leading(field)), []string{"// far", "// near"},
				"a comment that ends one line lower closes the gap")
		})

		t.Run("joins the comments of the child that Inner returns", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, innerJava).Root(), g.Kind(fieldDeclKind))
			c := javaComments()
			c.Inner = func(n treesitter.Node) treesitter.Node { return n }
			assert.Equal(t, texts(c.Leading(field)), []string{"/* inside */"},
				"the comment inside the declaration follows the run")
		})
	})

	t.Run("Trailing", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the comment after a declaration on its last line", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, tailJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, javaComments().Trailing(field).Text(), "// of a", "the comment is the trailing comment")
		})

		t.Run("returns the zero Node for a comment on the next line", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, nextJava).Root(), g.Kind(fieldDeclKind))
			assert.True(t, javaComments().Trailing(field).IsZero(), "a comment on the next line does not trail")
		})

		t.Run("returns the zero Node for a comment that Trails rejects", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, tailJava).Root(), g.Kind(fieldDeclKind))
			c := javaComments()
			c.Trails = func(treesitter.Node) bool { return false }
			assert.True(t, c.Trailing(field).IsZero(), "the rejected comment does not trail")
		})

		t.Run("returns a comment that the grammar folds into the declaration", func(t *testing.T) {
			t.Parallel()

			tree, err := typescript.TypeScript.Parse(t.Context(), tsFile, []byte(foldedTS))
			assert.NoError(t, err, "the source parses")
			t.Cleanup(tree.Close)
			declaration := tree.Root().FirstOf(typescript.TypeScript.Kind(lexicalKind))
			c := tsComments()
			c.Folded = true
			assert.Equal(t, c.Trailing(declaration).Text(), "// tail", "the folded comment is the trailing comment")
		})
	})

	t.Run("Runs", func(t *testing.T) {
		t.Parallel()

		t.Run("joins line comments on consecutive lines into one text", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, runJava).Root(), g.Kind(fieldDeclKind))
			c := javaComments()
			got := runsOf(c, c.Leading(field))
			assert.Equal(
				t,
				got,
				[]run{{text: "// first\n// second", at: position.Pos{File: javaFile, Line: 2, Col: 3}}},
				"the run is one text at the position of its first comment",
			)
		})

		t.Run("yields a block comment as a text of its own", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, mixedJava).Root(), g.Kind(fieldDeclKind))
			c := javaComments()
			got := runsOf(c, c.Leading(field))
			assert.Length(t, got, 2, "the block comment and the line comment are two texts")
			expect.Equal(t, got[0].text, "/* block */", "the block comment is the first text")
			expect.Equal(t, got[1].text, "// line", "the line comment is the second text")
		})

		t.Run("yields two texts for line comments that a blank line separates", func(t *testing.T) {
			t.Parallel()

			body := first(parse(t, gapJava).Root(), g.Kind(classBodyKind))
			c := javaComments()
			got := runsOf(c, commentsOf(c, body))
			assert.Length(t, got, 2, "the blank line ends the first run")
		})

		t.Run("uses Text for the lines of a run", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, runJava).Root(), g.Kind(fieldDeclKind))
			c := javaComments()
			c.Text = func(n treesitter.Node) string { return strings.ToUpper(n.Text()) }
			got := runsOf(c, c.Leading(field))
			assert.Length(t, got, 1, "the two lines are one run")
			assert.Equal(t, got[0].text, "// FIRST\n// SECOND", "the run joins the texts that Text returns")
		})
	})

	t.Run("Sweep", func(t *testing.T) {
		t.Parallel()

		t.Run("reports every comment that no run took", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, runJava)
			assert.Equal(t, swept(javaComments(), tree.Root()), []string{"// first", "// second"},
				"both comments are untaken")
		})

		t.Run("passes over a comment that a run took", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, gapJava)
			c := javaComments()
			runsOf(c, c.Leading(first(tree.Root(), g.Kind(fieldDeclKind))))
			assert.Equal(t, swept(c, tree.Root()), []string{"// far"}, "the run took the near comment")
		})

		t.Run("passes over a node that Take marks", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, innerJava)
			c := javaComments()
			c.Take(first(tree.Root(), g.Kind(fieldDeclKind)))
			assert.Empty(t, swept(c, tree.Root()), "the sweep passes over the comment inside the taken field")
		})

		t.Run("passes over a node that Opaque reports", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, bodyJava)
			c := javaComments()
			c.Opaque = func(n treesitter.Node) bool { return n.Kind() == g.Kind(methodBodyKind) }
			assert.Empty(t, swept(c, tree.Root()), "the comment of a statement is in the opaque body")
		})
	})
}

// TestCommentsAllocs checks that the walk allocates the list of a run and
// the text that joins it, and nothing for the trailing comment and the
// sweep. The ordinary run runs no benchmark, so this test checks the
// ceilings.
func TestCommentsAllocs(t *testing.T) {
	checkAllocs(t, commentCalls(t))
}

// BenchmarkComments measures each step of the walk that a frontend makes
// once for each declaration of a file.
func BenchmarkComments(b *testing.B) {
	benchCalls(b, commentCalls(b))
}

// commentCalls returns a call of each step of the walk over the run of
// runJava and the trailing comment of tailJava. Leading runs on a walk
// that took no comment, because a taken comment ends the run.
func commentCalls(tb testing.TB) []allocCall {
	tb.Helper()

	g := java.Grammar
	field := first(parse(tb, runJava).Root(), g.Kind(fieldDeclKind))
	tailed := first(parse(tb, tailJava).Root(), g.Kind(fieldDeclKind))
	c, untaken := javaComments(), javaComments()
	comments := c.Leading(field)
	runs := 0
	count := func(string, position.Pos) { runs++ }
	c.Runs(comments, count)
	var (
		got    []treesitter.Node
		tail   treesitter.Node
		untook int
	)
	return []allocCall{
		{
			name: "Leading", allocs: leadingAllocs,
			call:  func() { got = untaken.Leading(field) },
			check: func(tb assert.TB) { assert.Length(tb, got, 2, "Leading returns the run of two comments") },
		},
		{
			name: "Trailing",
			call: func() { tail = c.Trailing(tailed) },
			check: func(tb assert.TB) {
				assert.Equal(tb, tail.Text(), "// of a", "Trailing returns the comment of the field")
			},
		},
		{
			name: "Runs", allocs: runsAllocs,
			call:  func() { c.Runs(comments, count) },
			check: func(tb assert.TB) { assert.NotEqual(tb, runs, 0, "Runs yields the text of the run") },
		},
		{
			name: "Sweep",
			call: func() {
				untook = 0
				c.Sweep(field.Parent(), func(treesitter.Node) { untook++ })
			},
			check: func(tb assert.TB) { assert.Equal(tb, untook, 0, "Sweep finds every comment taken") },
		},
	}
}

// javaComments returns the parameters of the comment walk over the Java
// grammar, which has line and block comments.
func javaComments() *treesitter.Comments {
	line, block := java.Grammar.Kind(lineCommentKind), java.Grammar.Kind(blockCommentKind)
	return &treesitter.Comments{
		IsComment: func(n treesitter.Node) bool { return n.Kind() == line || n.Kind() == block },
		IsBlock:   func(n treesitter.Node) bool { return n.Kind() == block },
	}
}

// tsComments returns the parameters of the comment walk over the
// TypeScript grammar. Its one comment kind is a block comment where the text
// does not open with the marker of a line comment.
func tsComments() *treesitter.Comments {
	comment := typescript.TypeScript.Kind(tsCommentKind)
	return &treesitter.Comments{
		IsComment: func(n treesitter.Node) bool { return n.Kind() == comment },
		IsBlock:   func(n treesitter.Node) bool { return !strings.HasPrefix(n.Text(), tsLineMarker) },
	}
}

// texts returns the source of each node.
func texts(nodes []treesitter.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Text())
	}
	return out
}

// runsOf returns the text of every run of comments.
func runsOf(c *treesitter.Comments, comments []treesitter.Node) []run {
	var out []run
	c.Runs(comments, func(text string, at position.Pos) { out = append(out, run{text: text, at: at}) })
	return out
}

// commentsOf returns the comments among the named children of a node.
func commentsOf(c *treesitter.Comments, n treesitter.Node) []treesitter.Node {
	var out []treesitter.Node
	for child := range n.NamedChildren() {
		if c.IsComment(child) {
			out = append(out, child)
		}
	}
	return out
}

// swept returns the source of every comment that a sweep from root
// reports.
func swept(c *treesitter.Comments, root treesitter.Node) []string {
	var out []string
	c.Sweep(root, func(n treesitter.Node) { out = append(out, n.Text()) })
	return out
}
