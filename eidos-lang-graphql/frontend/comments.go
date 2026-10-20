// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	"github.com/dokimasia/gqlparser/v2/lexer"

	"go.dokimi.dev/eidos/lang/graphql/internal/document"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The joins of the comments that the attribution reads. The lines of a group
// of comments join into the text that the kit splits, and the documentation
// lines of a trailing comment join into the comment of a declaration.
const (
	commentLineSep = "\n"
	commentWordSep = " "
)

// unattributed describes a comment that the frontend attributes to no
// declaration, in the refusal of a carrier in it.
const unattributed = "a comment that the frontend attributes to no declaration"

// comments attributes the comments of one file to its declarations. A
// comment is neither documentation nor a directive in GraphQL, so the
// frontend reads only the carriers of a comment and the comment on the line
// of a declaration:
//
//   - The comments on the lines directly above the first token of a
//     declaration lead it. A comment on the line of the token before them
//     trails that token and leads nothing.
//   - The comment after the last token of a declaration, on the line where
//     the token ends, trails it. The last token of a definition with a body
//     is its opening brace, so a comment after the brace trails the
//     definition.
//
// The carriers of the comments that lead and trail a declaration attach to
// it. [comments.sweep] refuses the carriers of every other comment.
type comments struct {
	l     *document.Lexed
	unit  *plugin.SourceUnit
	path  string
	taken map[int]bool
}

// attach attributes the comments of the declaration subject. first is the
// index in Sig of its first token, and last the index of the token whose
// trailing comment is its comment. The carriers of the comments attach to
// subject. attach returns the text of the trailing comment without its
// carriers, which is the comment of the declaration.
func (c *comments) attach(subject symbol.Symbol, first, last int) string {
	if lead := c.leading(first); len(lead) > 0 {
		c.unit.AttachCarriers(subject, c.split(lead).Carriers, BadCarrier)
	}
	j, trails := c.trailing(last)
	if !trails {
		return ""
	}
	parts := c.split([]int{j})
	c.unit.AttachCarriers(subject, parts.Carriers, BadCarrier)
	return strings.TrimSpace(strings.Join(parts.Docs, commentWordSep))
}

// leading returns the indices of the comments that lead the significant
// token at first, in source order.
func (c *comments) leading(first int) []int {
	i := c.l.Sig[first]
	line := c.l.Tokens[i].Pos.Line
	var out []int
	for j := i - 1; j >= 0; j-- {
		tok := c.l.Tokens[j]
		// A comment on the line where the token before it ends trails that
		// token and leads nothing.
		trails := j > 0 && c.l.Tokens[j-1].Kind != lexer.Comment && c.l.Ends[j-1] == tok.Pos.Line
		if tok.Kind != lexer.Comment || tok.Pos.Line != line-1 || c.taken[j] || trails {
			break
		}
		out = append(out, j)
		line = tok.Pos.Line
	}
	slices.Reverse(out)
	return out
}

// trailing returns the index of the comment that trails the significant
// token at last, and reports false where none does. A comment trails the
// token where it is on the line on which the token ends.
func (c *comments) trailing(last int) (int, bool) {
	i := c.l.Sig[last]
	j := i + 1
	comment := j < len(c.l.Tokens) && c.l.Tokens[j].Kind == lexer.Comment && !c.taken[j]
	if comment && c.l.Ends[i] == c.l.Tokens[j].Pos.Line {
		return j, true
	}
	return 0, false
}

// split takes the comments at the token indices js apart through the kit,
// at the line of the first of them, and records each of them as taken. The
// comments are on consecutive lines, so each part is at its own line.
func (c *comments) split(js []int) plugin.CommentParts {
	raw := make([]string, len(js))
	for i, j := range js {
		c.taken[j] = true
		raw[i] = c.l.Tokens[j].Value
	}
	first := c.l.Tokens[js[0]].Pos
	return c.unit.Comment(strings.Join(raw, commentLineSep), position.Pos{
		File: c.path, Line: first.Line, Col: first.Column,
	})
}

// sweep refuses the carriers of every comment that no declaration took, so
// an authored directive in such a comment reports and does not disappear.
func (c *comments) sweep() {
	for j, tok := range c.l.Tokens {
		if tok.Kind != lexer.Comment || c.taken[j] {
			continue
		}
		parts := c.unit.Comment(tok.Value, position.Pos{File: c.path, Line: tok.Pos.Line, Col: tok.Pos.Column})
		c.unit.RefuseCarriers(UnaddressedCarrier, parts.Carriers, unattributed)
	}
}
