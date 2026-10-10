// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"github.com/bufbuild/protocompile/experimental/ast"
	"github.com/bufbuild/protocompile/experimental/source"
	"github.com/bufbuild/protocompile/experimental/source/length"
	"github.com/bufbuild/protocompile/experimental/token"

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
	// commentLineSep joins the comments of one group into the text that
	// the kit splits.
	commentLineSep = "\n"
)

// at returns the position of a span: the file the unit is loading, and
// the line and the column of the span's first byte. The column counts
// runes. The zero span is at the start of the file. The callers pass the
// span of a node, not the node, because a node converted to
// [source.Spanner] costs one allocation.
func (l *lowered) at(span source.Span) position.Pos {
	loc := span.Location(span.Start, length.Runes)
	return position.Pos{File: l.path, Line: loc.Line, Col: loc.Column}
}

// commentGroup is one run of comments protoc attributes as a unit:
// line comments on consecutive lines, or one block comment.
type commentGroup []token.Token

// isLine reports whether a comment is a line comment.
func isLine(c token.Token) bool { return strings.HasPrefix(c.Text(), lineComment) }

// startLine returns the line a token starts on. The zero token stands for
// the end of the file, and starts on the line of the file's last byte.
func (l *lowered) startLine(t token.Token) int {
	if t.IsZero() {
		return l.stream.LineByOffset(len(l.stream.Text()))
	}
	return l.stream.LineByOffset(t.LeafSpan().Start)
}

// endLine returns the line of a token's last byte.
func (l *lowered) endLine(t token.Token) int {
	span := t.LeafSpan()
	return l.stream.LineByOffset(max(span.End-1, span.Start))
}

// attribute attributes every comment of the file the way protoc's source
// info does, one run of comments between two adjacent tokens at a time.
// The tokens are the file's tokens other than comments and white space,
// in source order, and the opening and the closing brace of a body are
// two tokens. It records the group that trails a token and the group that
// leads a token, so [lowered.commentsOf] reads a declaration's comments
// at its first and its last token. It also records the file's first token
// as l.head.
func (l *lowered) attribute() {
	var (
		prev    token.Token
		between []token.Token
	)
	for tok := range l.stream.All() {
		if tok.IsSynthetic() {
			break
		}
		switch tok.Kind() {
		case token.Comment:
			between = append(between, tok)
		case token.Space, token.Unrecognized:
		default:
			if prev.IsZero() {
				l.head = tok
			}
			l.pair(prev, tok, between)
			prev, between = tok, nil
		}
	}
	l.pair(prev, token.Zero, between)
}

// pair attributes the comments between two adjacent tokens, prev and
// next. A zero prev is the start of the file, and a zero next is its end.
//
// The first comment trails prev where each of these is true:
//
//   - The comment starts on prev's line.
//   - next starts on a later line.
//   - The comment is a line comment, another comment follows it, or it
//     ends on a line before next.
//
// The end of the file counts as a line below prev. The other comments
// form groups as protoc groups them, and [lowered.donate] and
// [lowered.attach] split the groups between prev's trailing group and
// next's leading group.
func (l *lowered) pair(prev, next token.Token, comments []token.Token) {
	if len(comments) == 0 {
		return
	}
	var trail commentGroup
	if !prev.IsZero() {
		prevEnd, nextStart := l.endLine(prev), l.startLine(next)
		if next.IsZero() && nextStart == prevEnd {
			nextStart++
		}
		first := comments[0]
		if nextStart > prevEnd && l.startLine(first) == prevEnd &&
			(isLine(first) || len(comments) > 1 || l.endLine(first) < nextStart) {
			trail, comments = commentGroup{first}, comments[1:]
		}
	}
	groups := l.groupComments(comments)
	if !prev.IsZero() && len(trail) == 0 {
		trail, groups = l.donate(prev, next, groups)
	}
	if len(trail) > 0 {
		if l.trailing == nil {
			l.trailing = map[token.ID]commentGroup{}
		}
		l.trailing[prev.ID()] = trail
	}
	if lead := l.attach(prev, next, len(trail) > 0, groups); len(lead) > 0 && !next.IsZero() {
		if l.leading == nil {
			l.leading = map[token.ID]commentGroup{}
		}
		l.leading[next.ID()] = lead
	}
}

// groupComments splits a run of comments into protoc's groups: a block
// comment is a group of its own, and a line comment continues the group
// of a line comment on the line above it.
func (l *lowered) groupComments(comments []token.Token) []commentGroup {
	if len(comments) == 0 {
		return nil
	}
	var out []commentGroup
	current := commentGroup{comments[0]}
	for _, c := range comments[1:] {
		prev := current[len(current)-1]
		if !isLine(c) || !isLine(prev) || l.startLine(c) > l.endLine(prev)+1 {
			out = append(out, current)
			current = commentGroup{c}
			continue
		}
		current = append(current, c)
	}
	return append(out, current)
}

// donate gives the previous token the first group before the next
// token as its trailing group, where protoc does: the group starts
// on the previous token's line or the one below it, and either
// another group follows it, a blank line separates it from the next
// token, or the next token is a closer or the end of the file. A
// group on the line of both tokens is not donated, because protoc
// leaves that attribution ambiguous.
func (l *lowered) donate(prev, next token.Token, groups []commentGroup) (commentGroup, []commentGroup) {
	if len(groups) == 0 {
		return nil, nil
	}
	first := groups[0][0]
	if l.startLine(first) > l.endLine(prev)+1 {
		return nil, groups
	}
	if len(groups) > 1 {
		return groups[0], groups[1:]
	}
	only := groups[0]
	last := only[len(only)-1]
	if l.endLine(last) < l.startLine(next)-1 {
		return only, nil
	}
	if text := next.Text(); text == "" || len(text) == 1 && strings.ContainsAny(text, closers) {
		if text != "" && l.startLine(first) == l.endLine(prev) && l.endLine(last) == l.startLine(next) {
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
func (l *lowered) attach(prev, next token.Token, trailed bool, groups []commentGroup) commentGroup {
	if len(groups) == 0 {
		return nil
	}
	if len(groups) == 1 && !trailed && !prev.IsZero() {
		only := groups[0]
		afterPrevious := l.startLine(only[0]) == l.endLine(prev)
		beforeNext := l.endLine(only[len(only)-1]) == l.startLine(next)
		if afterPrevious && beforeNext {
			return nil
		}
	}
	last := groups[len(groups)-1]
	if l.endLine(last[len(last)-1]) >= l.startLine(next)-1 {
		return last
	}
	return nil
}

// split takes one group apart through the kit, positioned at the
// group's first line, and records every comment in it as taken, so
// the sweep reads only the comments no declaration took.
func (l *lowered) split(g commentGroup) plugin.CommentParts {
	if len(g) == 0 {
		return plugin.CommentParts{}
	}
	if l.consumed == nil {
		l.consumed = map[token.ID]bool{}
	}
	raw := make([]string, len(g))
	for i, c := range g {
		l.consumed[c.ID()] = true
		raw[i] = c.Text()
	}
	return l.unit.Comment(strings.Join(raw, commentLineSep), l.at(g[0].LeafSpan()))
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
// splits them. The leading group of the declaration's first token is
// the documentation, and the trailing group of its last token is the
// comment. The last token of a block declaration is its opening brace,
// after which protoc takes the trailing comment. A carrier in either
// group attaches to the subject, and a nil subject reports it as
// unaddressed. A zero token has no group.
func (l *lowered) commentsOf(first, last token.Token, subject symbol.Symbol) commented {
	lead := l.split(l.leading[first.ID()])
	trail := l.split(l.trailing[last.ID()])
	l.carriers(subject, lead.Carriers)
	l.carriers(subject, trail.Carriers)
	return commented{
		doc:         lead.Docs,
		annotations: append(lead.Annotations, trail.Annotations...),
		comment:     strings.TrimSpace(strings.Join(trail.Docs, " ")),
	}
}

// bounds returns a definition's first token and the token whose trailing
// group is its comment. The first token is the first token of its type,
// which is the keyword of a message, an enum, a service, a oneof, a group
// and an rpc, or its name, because an enum value has no type. The last
// is the opening brace of its body, or its semicolon. A definition that
// lacks both, which the parser reports, has the zero last token.
func (l *lowered) bounds(def ast.DeclDef) (first, last token.Token) {
	var start int
	if ty := def.Type(); !ty.IsZero() {
		start = ty.Span().Start
	} else {
		start = def.Name().Span().Start
	}
	_, first = l.stream.Around(start)
	if body := def.Body(); !body.IsZero() {
		return first, body.Braces()
	}
	return first, def.Semicolon()
}

// carriers attaches every well-formed carrier to its subject. A
// carrier the grammar refused reports under [BadCarrier], and one
// with no addressable subject under [UnaddressedCarrier], each
// quoted as the author wrote it.
func (l *lowered) carriers(subject symbol.Symbol, carriers []plugin.Carrier) {
	if subject != nil {
		l.unit.AttachCarriers(subject, carriers, BadCarrier)
		return
	}
	for _, c := range carriers {
		if _, err := directive.Parse(c.Payload); err != nil {
			l.unit.Errorf(BadCarrier, c.Pos, "%q: %v", c.Mark+c.Payload, err)
			continue
		}
		l.unit.Errorf(UnaddressedCarrier, c.Pos,
			"%q is on a subject the model cannot address. Move it directly above a declaration",
			c.Mark+c.Payload)
	}
}
