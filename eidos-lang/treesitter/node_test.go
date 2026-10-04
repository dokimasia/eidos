// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/treesitter/java"
	"go.dokimi.dev/eidos/sdk/position"
)

// The wide source's second field, which starts after the two bytes of
// "é" on line 2, as a count of bytes places it and a count of runes
// would not, and the byte after its semicolon.
var (
	wideFieldAt  = position.Pos{File: javaFile, Line: 2, Col: 20}
	wideFieldEnd = position.Pos{File: javaFile, Line: 2, Col: 26}
)

// The clean source's field, whose declarator field names two
// variables.
const (
	cleanField = "int a, b;"
	firstName  = "a"
	secondName = "b"
)

// The sources the extra and compact cases read: a field after a line
// comment, a generic field type spaced out around a block comment,
// identifiers that open with a dollar sign, and a string literal with
// two spaces inside it.
const (
	commentJava = "class A {\n  // note\n  int a;\n}\n"
	spacedJava  = "class A {\n  Map< String ,  /* keys */ List<T> > m;\n}\n"
	dollarJava  = "class A {\n  $A  $b;\n}\n"
	literalJava = "class A {\n  String s = \"a  b\";\n}\n"
)

// The kinds the extra and compact cases find: a line comment, a
// generic type, and a variable declarator.
const (
	lineCommentKind = "line_comment"
	genericKind     = "generic_type"
	declaratorKind  = "variable_declarator"
)

// The field the long compact case reads, of 21 declarators and 43
// tokens, more than Compact spells without allocating their spans, and
// its compact spelling.
var (
	longJava    = "class A {\n  int " + strings.Repeat("a, ", 20) + "a;\n}\n"
	longCompact = "int " + strings.Repeat("a,", 20) + "a;"
)

// The positions of the clean source's field: line 2 from its third
// byte, and the byte after its semicolon.
var (
	cleanFieldAt  = position.Pos{File: javaFile, Line: 2, Col: 3}
	cleanFieldEnd = position.Pos{File: javaFile, Line: 2, Col: 12}
)

// The allocations of an accessor that returns source text: the string,
// and the token spans of a node of more than 32 tokens.
const (
	textAllocs        = 1
	longCompactAllocs = textAllocs + 1
)

// Every accessor of a Node works on the absent node too. Each accessor
// is pinned on a parsed tree and on the zero Node.
func TestNode(t *testing.T) {
	t.Parallel()

	g := java.Grammar
	var absent treesitter.Node

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.True(t, absent.IsZero(), "the zero Node is absent")
		})

		t.Run("reports false for a node of an open tree", func(t *testing.T) {
			t.Parallel()

			assert.False(t, parse(t, cleanJava).Root().IsZero(), "the root of an open tree is present")
		})
	})

	t.Run("Kind", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the zero Kind for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, absent.Kind(), 0, "an absent node has no kind")
		})
	})

	t.Run("Named", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a node of a named kind", func(t *testing.T) {
			t.Parallel()

			assert.True(t, parse(t, cleanJava).Root().Named(), "the root is a named rule")
		})

		t.Run("reports false for punctuation", func(t *testing.T) {
			t.Parallel()

			errs := slices.Collect(parse(t, brokenJava).Errors())
			assert.False(t, errs[1].Named(), "the missing semicolon is punctuation")
		})

		t.Run("reports false for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.False(t, absent.Named(), "an absent node names nothing")
		})
	})

	t.Run("IsError", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for an ERROR node", func(t *testing.T) {
			t.Parallel()

			errs := slices.Collect(parse(t, brokenJava).Errors())
			assert.True(t, errs[0].IsError(), "the field the parser could not fit is an ERROR node")
		})

		t.Run("reports false for a MISSING node", func(t *testing.T) {
			t.Parallel()

			errs := slices.Collect(parse(t, brokenJava).Errors())
			assert.False(t, errs[1].IsError(), "an inserted node is no ERROR node")
		})

		t.Run("reports false for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.False(t, absent.IsError(), "an absent node is no error")
		})
	})

	t.Run("IsMissing", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a node the parser inserted", func(t *testing.T) {
			t.Parallel()

			errs := slices.Collect(parse(t, brokenJava).Errors())
			assert.True(t, errs[1].IsMissing(), "the semicolon the source lacks")
		})

		t.Run("reports false for an ERROR node", func(t *testing.T) {
			t.Parallel()

			errs := slices.Collect(parse(t, brokenJava).Errors())
			assert.False(t, errs[0].IsMissing(), "an ERROR node spans source")
		})

		t.Run("reports false for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.False(t, absent.IsMissing(), "an absent node was not inserted")
		})
	})

	t.Run("IsExtra", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a comment", func(t *testing.T) {
			t.Parallel()

			comment := first(parse(t, commentJava).Root(), g.Kind(lineCommentKind))
			assert.True(t, comment.IsExtra(), "the grammar admits a comment between any two tokens")
		})

		t.Run("reports false for a declaration", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, commentJava).Root(), g.Kind(fieldDeclKind))
			assert.False(t, field.IsExtra(), "a declaration is where the grammar states it")
		})

		t.Run("reports false for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.False(t, absent.IsExtra(), "an absent node is no extra")
		})
	})

	t.Run("Text", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the node's source bytes", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, field.Text(), cleanField, "the declaration as written")
		})

		t.Run("returns nothing for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, absent.Text(), "an absent node spans no source")
		})
	})

	t.Run("TextThrough", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the source from one node through another as written", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			declarators := slices.Collect(field.Children(g.Field(declaratorField)))
			assert.Equal(t, declarators[0].TextThrough(declarators[1]), "a, b", "the separator and its space included")
		})

		t.Run("returns nothing for a last node that ends before the node starts", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			declarators := slices.Collect(field.Children(g.Field(declaratorField)))
			assert.Empty(t, declarators[1].TextThrough(declarators[0]), "the span would run backwards")
		})

		t.Run("returns nothing for nodes of two trees", func(t *testing.T) {
			t.Parallel()

			one := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			other := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			assert.Empty(t, one.TextThrough(other), "a span is inside one source")
		})

		t.Run("returns nothing for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, absent.TextThrough(absent), "an absent node spans no source")
		})
	})

	t.Run("Compact", func(t *testing.T) {
		t.Parallel()

		t.Run("removes the whitespace between two tokens", func(t *testing.T) {
			t.Parallel()

			generic := first(parse(t, spacedJava).Root(), g.Kind(genericKind))
			assert.Equal(t, generic.Compact(), "Map<String,List<T>>", "the spaces around the brackets are gone")
		})

		t.Run("keeps one space between two identifier tokens", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, field.Compact(), "int a,b;", "the type and the name are two tokens")
		})

		t.Run("keeps one space before an identifier that opens with a dollar sign", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, dollarJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, field.Compact(), "$A $b;", "a dollar sign can open a Java identifier")
		})

		t.Run("keeps the whitespace inside a token", func(t *testing.T) {
			t.Parallel()

			declarator := first(parse(t, literalJava).Root(), g.Kind(declaratorKind))
			assert.Equal(t, declarator.Compact(), `s="a  b"`, "the string literal's spaces are its value")
		})

		t.Run("spells every token of a node of more than 32 tokens", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, longJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, field.Compact(), longCompact, "the type, 21 names, 20 commas and the semicolon")
		})

		t.Run("returns nothing for an extra", func(t *testing.T) {
			t.Parallel()

			comment := first(parse(t, commentJava).Root(), g.Kind(lineCommentKind))
			assert.Empty(t, comment.Compact(), "an extra has no token Compact writes")
		})

		t.Run("returns nothing for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, absent.Compact(), "an absent node has no token")
		})
	})

	t.Run("Pos", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a position counted from one", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, wideJava).Root(), g.Kind(fieldDeclKind)).NextNamedSibling()
			assert.Equal(t, field.Pos(), wideFieldAt, "the second field starts at byte 19 of line 2")
		})

		t.Run("returns the zero position for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, absent.Pos(), position.Pos{}, "an absent node starts nowhere")
		})
	})

	t.Run("End", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the position of the byte after the node", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, wideJava).Root(), g.Kind(fieldDeclKind)).NextNamedSibling()
			assert.Equal(t, field.End(), wideFieldEnd, "the byte after the second field's semicolon")
		})

		t.Run("returns the zero position for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, absent.End(), position.Pos{}, "an absent node ends nowhere")
		})
	})

	t.Run("Child", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the first child under a field", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, field.Child(g.Field(declaratorField)).Text(), firstName, "the first declarator")
		})

		t.Run("returns the zero Node for a field without a child", func(t *testing.T) {
			t.Parallel()

			class := first(parse(t, cleanJava).Root(), g.Kind(classKind))
			assert.True(t, class.Child(g.Field(superclassField)).IsZero(), "the class extends nothing")
		})

		t.Run("returns the zero Node for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.True(t, absent.Child(g.Field(nameField)).IsZero(), "an absent node has no child")
		})
	})

	t.Run("Children", func(t *testing.T) {
		t.Parallel()

		t.Run("yields every child under a field in source order", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			var names []string
			for d := range field.Children(g.Field(declaratorField)) {
				names = append(names, d.Text())
			}
			assert.Equal(t, names, []string{firstName, secondName}, "both declarators, in order")
		})

		t.Run("yields nothing for the zero Field", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			assert.Empty(t, slices.Collect(field.Children(0)), "the zero Field names no field")
		})

		t.Run("yields nothing for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(absent.Children(g.Field(nameField))), "an absent node has no child")
		})

		t.Run("stops when the consumer stops", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			seen := 0
			for d := range field.Children(g.Field(declaratorField)) {
				seen++
				if d.Text() == firstName {
					break
				}
			}
			assert.Equal(t, seen, 1, "the walk ends at the first break")
		})
	})

	t.Run("AllChildren", func(t *testing.T) {
		t.Parallel()

		t.Run("yields every child in source order with the unnamed ones", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, kindsOf(g, slices.Collect(field.AllChildren())),
				[]string{"integral_type", declaratorKind, ",", declaratorKind, ";"},
				"the separator and the terminator are children too")
		})

		t.Run("yields an extra among the children", func(t *testing.T) {
			t.Parallel()

			body := first(parse(t, commentJava).Root(), g.Kind(classBodyKind))
			assert.Equal(t, kindsOf(g, slices.Collect(body.AllChildren())),
				[]string{"{", lineCommentKind, fieldDeclKind, "}"}, "the comment is where the source writes it")
		})

		t.Run("yields nothing for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(absent.AllChildren()), "an absent node has no child")
		})

		t.Run("stops when the consumer stops", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			seen := 0
			for range field.AllChildren() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the walk ends at the first break")
		})
	})

	t.Run("NamedChildren", func(t *testing.T) {
		t.Parallel()

		t.Run("yields every named child in source order", func(t *testing.T) {
			t.Parallel()

			body := first(parse(t, cleanJava).Root(), g.Kind(classBodyKind))
			assert.Equal(t, kindsOf(g, slices.Collect(body.NamedChildren())),
				[]string{fieldDeclKind, methodDeclKind}, "the braces are unnamed and left out")
		})

		t.Run("yields nothing for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(absent.NamedChildren()), "an absent node has no child")
		})

		t.Run("stops when the consumer stops", func(t *testing.T) {
			t.Parallel()

			body := first(parse(t, cleanJava).Root(), g.Kind(classBodyKind))
			seen := 0
			for member := range body.NamedChildren() {
				seen++
				if member.Kind() == g.Kind(fieldDeclKind) {
					break
				}
			}
			assert.Equal(t, seen, 1, "the walk ends at the first break")
		})
	})

	t.Run("Parent", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the node's parent", func(t *testing.T) {
			t.Parallel()

			name := first(parse(t, cleanJava).Root(), g.Kind(identifierKind))
			assert.Equal(t, name.Parent().Kind(), g.Kind(classKind), "the class declares its name")
		})

		t.Run("returns the zero Node for the root", func(t *testing.T) {
			t.Parallel()

			assert.True(t, parse(t, cleanJava).Root().Parent().IsZero(), "the root has no parent")
		})

		t.Run("returns the zero Node for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.True(t, absent.Parent().IsZero(), "an absent node has no parent")
		})
	})

	t.Run("PrevNamedSibling", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the named sibling before the node", func(t *testing.T) {
			t.Parallel()

			method := first(parse(t, cleanJava).Root(), g.Kind(methodDeclKind))
			assert.Equal(t, method.PrevNamedSibling().Kind(), g.Kind(fieldDeclKind), "the field precedes the method")
		})

		t.Run("returns the zero Node for a first named child", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			assert.True(t, field.PrevNamedSibling().IsZero(), "the brace before it is unnamed")
		})

		t.Run("returns the zero Node for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.True(t, absent.PrevNamedSibling().IsZero(), "an absent node has no sibling")
		})
	})

	t.Run("NextNamedSibling", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the named sibling after the node", func(t *testing.T) {
			t.Parallel()

			field := first(parse(t, cleanJava).Root(), g.Kind(fieldDeclKind))
			assert.Equal(t, field.NextNamedSibling().Kind(), g.Kind(methodDeclKind), "the method follows the field")
		})

		t.Run("returns the zero Node for a last named child", func(t *testing.T) {
			t.Parallel()

			method := first(parse(t, cleanJava).Root(), g.Kind(methodDeclKind))
			assert.True(t, method.NextNamedSibling().IsZero(), "the brace after it is unnamed")
		})

		t.Run("returns the zero Node for the zero Node", func(t *testing.T) {
			t.Parallel()

			assert.True(t, absent.NextNamedSibling().IsZero(), "an absent node has no sibling")
		})
	})
}

// The accessors that return source text allocate the string, and every
// other accessor and walk allocates nothing. The ordinary run, which
// runs no benchmark, checks those ceilings here.
func TestNodeAllocs(t *testing.T) {
	checkAllocs(t, nodeCalls(t))
}

// BenchmarkNode measures every accessor and walk a frontend calls on
// the nodes of a parsed file.
func BenchmarkNode(b *testing.B) {
	benchCalls(b, nodeCalls(b))
}

// nodeCalls returns a call of every accessor and walk of a node of the
// clean source, and a compact spelling of the long field.
func nodeCalls(tb testing.TB) []allocCall {
	tb.Helper()

	g := java.Grammar
	root := parse(tb, cleanJava).Root()
	field, body := first(root, g.Kind(fieldDeclKind)), first(root, g.Kind(classBodyKind))
	method := first(root, g.Kind(methodDeclKind))
	long := first(parse(tb, longJava).Root(), g.Kind(fieldDeclKind))
	decl := g.Field(declaratorField)
	declarators := slices.Collect(field.Children(decl))
	var (
		reported bool
		kind     treesitter.Kind
		text     string
		at       position.Pos
		node     treesitter.Node
		count    int
	)
	return []allocCall{
		{
			name:  "IsZero",
			call:  func() { reported = field.IsZero() },
			check: func(tb assert.TB) { assert.False(tb, reported, "IsZero reports a parsed node present") },
		},
		{
			name:  "Kind",
			call:  func() { kind = field.Kind() },
			check: func(tb assert.TB) { assert.Equal(tb, kind, g.Kind(fieldDeclKind), "Kind returns the field's kind") },
		},
		{
			name:  "Named",
			call:  func() { reported = field.Named() },
			check: func(tb assert.TB) { assert.True(tb, reported, "Named reports a declaration named") },
		},
		{
			name:  "IsError",
			call:  func() { reported = field.IsError() },
			check: func(tb assert.TB) { assert.False(tb, reported, "IsError reports a clean node") },
		},
		{
			name:  "IsMissing",
			call:  func() { reported = field.IsMissing() },
			check: func(tb assert.TB) { assert.False(tb, reported, "IsMissing reports a written node") },
		},
		{
			name:  "IsExtra",
			call:  func() { reported = field.IsExtra() },
			check: func(tb assert.TB) { assert.False(tb, reported, "IsExtra reports a declaration") },
		},
		{
			name:   "Text",
			allocs: textAllocs,
			call:   func() { text = field.Text() },
			check:  func(tb assert.TB) { assert.Equal(tb, text, cleanField, "Text returns the declaration") },
		},
		{
			name:   "TextThrough",
			allocs: textAllocs,
			call:   func() { text = declarators[0].TextThrough(declarators[1]) },
			check:  func(tb assert.TB) { assert.Equal(tb, text, "a, b", "TextThrough returns both declarators") },
		},
		{
			name:   "Compact",
			allocs: textAllocs,
			call:   func() { text = field.Compact() },
			check:  func(tb assert.TB) { assert.Equal(tb, text, "int a,b;", "Compact returns the tokens") },
		},
		{
			name:   "Compact/a node of more than 32 tokens",
			allocs: longCompactAllocs,
			call:   func() { text = long.Compact() },
			check:  func(tb assert.TB) { assert.Equal(tb, text, longCompact, "Compact returns every token") },
		},
		{
			name:  "Pos",
			call:  func() { at = field.Pos() },
			check: func(tb assert.TB) { assert.Equal(tb, at, cleanFieldAt, "Pos returns where the field starts") },
		},
		{
			name:  "End",
			call:  func() { at = field.End() },
			check: func(tb assert.TB) { assert.Equal(tb, at, cleanFieldEnd, "End returns where the field ends") },
		},
		{
			name:  "Child",
			call:  func() { node = field.Child(decl) },
			check: func(tb assert.TB) { assert.Equal(tb, node.Text(), firstName, "Child returns the first declarator") },
		},
		{
			name: "Children",
			call: func() {
				count = 0
				for range field.Children(decl) {
					count++
				}
			},
			check: func(tb assert.TB) { assert.Equal(tb, count, 2, "Children yields both declarators") },
		},
		{
			name: "AllChildren",
			call: func() {
				count = 0
				for range field.AllChildren() {
					count++
				}
			},
			check: func(tb assert.TB) { assert.Equal(tb, count, 5, "AllChildren yields the separator and the terminator") },
		},
		{
			name: "NamedChildren",
			call: func() {
				count = 0
				for range body.NamedChildren() {
					count++
				}
			},
			check: func(tb assert.TB) { assert.Equal(tb, count, 2, "NamedChildren yields the field and the method") },
		},
		{
			name: "Parent",
			call: func() { node = field.Parent() },
			check: func(tb assert.TB) {
				assert.Equal(tb, node.Kind(), g.Kind(classBodyKind), "Parent returns the class body")
			},
		},
		{
			name: "PrevNamedSibling",
			call: func() { node = method.PrevNamedSibling() },
			check: func(tb assert.TB) {
				assert.Equal(tb, node.Kind(), g.Kind(fieldDeclKind), "PrevNamedSibling returns the field")
			},
		},
		{
			name: "NextNamedSibling",
			call: func() { node = field.NextNamedSibling() },
			check: func(tb assert.TB) {
				assert.Equal(tb, node.Kind(), g.Kind(methodDeclKind), "NextNamedSibling returns the method")
			},
		},
	}
}

// kindsOf returns the names of the kinds a sequence of nodes reports.
func kindsOf(g *treesitter.Grammar, nodes []treesitter.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = g.KindName(n.Kind())
	}
	return out
}
