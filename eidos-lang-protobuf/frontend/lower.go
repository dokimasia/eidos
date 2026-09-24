// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strconv"
	"strings"

	"github.com/bufbuild/protocompile/ast"

	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The comment spellings the attribution reads.
const (
	// lineComment opens a line comment. Every other comment is a
	// block comment.
	lineComment = "//"
	// closers are the tokens that close a scope or a statement. A
	// comment alone before one of them trails the previous token,
	// because a closer takes no documentation.
	closers = "}]),;"
)

// at returns one node's position: the file the unit is loading,
// and the line and column protocompile recorded.
func (l *lowered) at(n ast.Node) position.Pos {
	if n == nil {
		return position.Pos{File: l.path, Line: 1, Col: 1}
	}
	start := l.tree.NodeInfo(n).Start()
	return position.Pos{File: l.path, Line: start.Line, Col: start.Col}
}

// text returns a node's source text, verbatim. A stamped residue
// keeps the spelling the schema wrote: a reserved range and an
// option value among them.
func (l *lowered) text(n ast.Node) string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(l.tree.NodeInfo(n).RawText())
}

// commentGroup is one run of comments protoc attributes as a unit:
// line comments on consecutive lines, or one block comment.
type commentGroup []ast.Comment

// isLine reports whether a comment is a line comment.
func isLine(c ast.Comment) bool { return strings.HasPrefix(c.RawText(), lineComment) }

// groupComments splits the comments before a token into protoc's
// groups: a block comment is a group of its own, and a line comment
// continues the group of a line comment on the line above it.
func groupComments(cmts ast.Comments) []commentGroup {
	if cmts.Len() == 0 {
		return nil
	}
	var out []commentGroup
	current := commentGroup{cmts.Index(0)}
	for i := 1; i < cmts.Len(); i++ {
		c := cmts.Index(i)
		prev := current[len(current)-1]
		if !isLine(c) || !isLine(prev) || c.Start().Line > prev.End().Line+1 {
			out = append(out, current)
			current = commentGroup{c}
			continue
		}
		current = append(current, c)
	}
	return append(out, current)
}

// attribute splits the comments between two adjacent tokens the way
// protoc's source info does, and returns the group trailing the
// previous token and the group leading the next one. The groups
// between the two are detached and document nothing. A previous
// token that is not valid, before the first token of a file, takes
// no trailing group.
func attribute(prev, next ast.NodeInfo) (trail, lead commentGroup) {
	groups := groupComments(next.LeadingComments())
	if prev.IsValid() {
		if own := prev.TrailingComments(); own.Len() > 0 {
			for i := range own.Len() {
				trail = append(trail, own.Index(i))
			}
		} else {
			trail, groups = donate(prev, next, groups)
		}
	}
	return trail, attach(prev, next, len(trail) > 0, groups)
}

// donate gives the previous token the first group before the next
// token as its trailing group, where protoc does: the group starts
// on the previous token's line or the one below it, and either
// another group follows it, a blank line separates it from the next
// token, or the next token is a closer or the end of the file. A
// group on the line of both tokens is not donated, because protoc
// leaves that attribution ambiguous.
func donate(prev, next ast.NodeInfo, groups []commentGroup) (commentGroup, []commentGroup) {
	if len(groups) == 0 {
		return nil, nil
	}
	first := groups[0][0]
	if first.Start().Line > prev.End().Line+1 {
		return nil, groups
	}
	if len(groups) > 1 {
		return groups[0], groups[1:]
	}
	only := groups[0]
	last := only[len(only)-1]
	if last.End().Line < next.Start().Line-1 {
		return only, nil
	}
	if text := next.RawText(); text == "" || len(text) == 1 && strings.ContainsAny(text, closers) {
		if text != "" && first.Start().Line == prev.End().Line && last.End().Line == next.Start().Line {
			return nil, groups
		}
		return only, nil
	}
	return nil, groups
}

// attach returns the last group before a token as its leading group
// where no blank line separates the two, and nil otherwise. One group
// on the line of both tokens, with no trailing group before it, leads
// neither, because protoc leaves it detached.
func attach(prev, next ast.NodeInfo, trailed bool, groups []commentGroup) commentGroup {
	if len(groups) == 0 {
		return nil
	}
	if len(groups) == 1 && !trailed && prev.IsValid() {
		only := groups[0]
		afterPrevious := only[0].Start().Line == prev.End().Line
		beforeNext := only[len(only)-1].End().Line == next.Start().Line
		if afterPrevious && beforeNext {
			return nil
		}
	}
	last := groups[len(groups)-1]
	if last[len(last)-1].End().Line >= next.Start().Line-1 {
		return last
	}
	return nil
}

// leadingGroup returns the comment group leading a node's first
// token.
func (l *lowered) leadingGroup(n ast.Node) commentGroup {
	start := n.Start()
	var prev ast.NodeInfo
	if before, held := l.tree.Tokens().Previous(start); held {
		prev = l.tree.TokenInfo(before)
	}
	_, lead := attribute(prev, l.tree.TokenInfo(start))
	return lead
}

// trailingGroup returns the comment group trailing a node's last
// token.
func (l *lowered) trailingGroup(n ast.Node) commentGroup {
	end := n.End()
	after, held := l.tree.Tokens().Next(end)
	if !held {
		return nil
	}
	trail, _ := attribute(l.tree.TokenInfo(end), l.tree.TokenInfo(after))
	return trail
}

// split takes one group apart through the kit, positioned at the
// group's first line, and records every comment in it as taken, so
// the sweep reads only the comments no declaration took.
func (l *lowered) split(g commentGroup) plugin.CommentParts {
	if len(g) == 0 {
		return plugin.CommentParts{}
	}
	raw := make([]string, len(g))
	for i, c := range g {
		l.consumed[c.AsItem()] = true
		raw[i] = c.RawText()
	}
	start := g[0].Start()
	return l.unit.Comment(strings.Join(raw, "\n"),
		position.Pos{File: l.path, Line: start.Line, Col: start.Col})
}

// commented is one declaration's comments, split: the documentation
// above it, the annotations above and beside it, and the comment
// beside it.
type commented struct {
	doc         []string
	annotations symbol.Annotations
	comment     string
}

// commentsOf attributes a declaration's comments as protoc does and
// splits them. The leading group is the documentation, and the
// trailing group is the comment: the one after the last token, or
// after the opening brace for a block declaration, which is brace. A
// carrier in either group attaches to the subject, and a nil
// subject reports it as unaddressed.
func (l *lowered) commentsOf(n, brace ast.Node, subject symbol.Symbol) commented {
	lead := l.split(l.leadingGroup(n))
	end := n
	if brace != nil {
		end = brace
	}
	trail := l.split(l.trailingGroup(end))
	l.carriers(subject, lead.Carriers)
	l.carriers(subject, trail.Carriers)
	return commented{
		doc:         lead.Docs,
		annotations: append(lead.Annotations, trail.Annotations...),
		comment:     strings.TrimSpace(strings.Join(trail.Docs, " ")),
	}
}

// carriers attaches every well-formed carrier to its subject. A
// carrier the grammar refused reports under [BadCarrier], and one
// with no addressable subject under [UnaddressedCarrier].
func (l *lowered) carriers(subject symbol.Symbol, carriers []plugin.Carrier) {
	for _, c := range carriers {
		raw, err := directive.Parse(c.Payload)
		if err != nil {
			l.unit.Errorf(BadCarrier, c.Pos, "%q: %v", plugin.CarrierMark+c.Payload, err)
			continue
		}
		if subject == nil {
			l.unit.Errorf(UnaddressedCarrier, c.Pos,
				"%q is on a subject the model cannot address; move it above a declaration",
				plugin.CarrierMark+c.Payload)
			continue
		}
		raw.Pos = c.Pos
		l.unit.Graph().Attach(subject, raw)
	}
}

// identOf returns a name node's dotted spelling. A reference keeps
// it verbatim until resolution binds it.
func identOf(n ast.IdentValueNode) string {
	if n == nil {
		return ""
	}
	return string(n.AsIdentifier())
}

// uintText spells an unsigned literal, which is what a field's
// wire number is.
func uintText(n *ast.UintLiteralNode) string {
	if n == nil {
		return ""
	}
	return strconv.FormatUint(n.Val, 10)
}

// commentAt returns the comment one item is, and false where the
// item is a token.
func (l *lowered) commentAt(item ast.Item) (ast.Comment, bool) {
	_, comment := l.tree.GetItem(item)
	if !comment.IsValid() {
		return ast.Comment{}, false
	}
	return comment, true
}
