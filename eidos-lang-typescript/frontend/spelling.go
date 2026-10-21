// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/typescript"
)

// The punctuation of a type that [lowering.spelling] lays out.
const (
	commaMark     = ","
	semicolonMark = ";"
	colonMark     = ":"
	questionMark  = "?"
	braceOpen     = "{"
	braceClose    = "}"
	barMark       = "|"
	ampersandMark = "&"
)

// spacedKeywords are the keywords of a type that a space separates from
// the token on each side, also from a bracket or a parenthesis after
// them, as in keyof (A | B) and readonly [A, B].
var spacedKeywords = map[string]bool{
	"abstract": true, "as": true, "asserts": true, "const": true, "extends": true, "get": true, "in": true,
	"infer": true, "is": true, "keyof": true, "new": true, "out": true, "readonly": true, "set": true,
	"typeof": true, "unique": true,
}

// spelling returns the spelling of the type n: its tokens in the layout in
// which Biome and Prettier write the type on one line. The layout depends
// on the tokens alone, so a reformatted signature keeps one spelling:
//
//   - a space around |, &, => and =, around the ? and the : of a
//     conditional type, and around a keyword of a type, such as keyof,
//     readonly or extends;
//   - a space after a comma, a colon and the separator of the members of
//     an object type, and inside the braces of an object type and of an
//     object pattern;
//   - one space between two tokens that would otherwise read as one
//     identifier, and no space anywhere else;
//   - the tokens of a string and of a template literal type as the source
//     writes them.
//
// The members of an object type are separated by semicolons. A leading |
// or & of a union or an intersection, a separator of an object type after
// its opening brace or before its closing brace, and a comma before a
// closing bracket are left out. A comment is left out too.
//
// spelling allocates the string. The buffer of tokens of the lowering grows
// where a type has more tokens than any type before it.
func (l *lowering) spelling(n treesitter.Node) string {
	l.tokens = n.Tokens(l.tokens[:0])
	size := 0
	for _, t := range l.tokens {
		size += len(t.Text) + 1
	}
	var b strings.Builder
	b.Grow(size)
	var prev *treesitter.Token
	for i := range l.tokens {
		t := &l.tokens[i]
		if l.dropped(t, prev, i) {
			continue
		}
		if prev != nil && l.v.spaced(prev, t) {
			b.WriteByte(' ')
		}
		if t.Parent == l.v.objectType && string(t.Text) == commaMark {
			b.WriteString(semicolonMark)
		} else {
			b.Write(t.Text)
		}
		prev = t
	}
	return b.String()
}

// dropped reports whether the spelling leaves out the token t, the token
// at the index i of the tokens of the lowering. prev is the last token that
// the spelling writes before it, and nil for none. The spelling leaves out
// a | or & that opens its union or intersection, and a comma or a
// semicolon after the opening brace of an object type or before a closing
// bracket, brace or parenthesis.
func (l *lowering) dropped(t, prev *treesitter.Token, i int) bool {
	switch string(t.Text) {
	case barMark, ampersandMark:
		return t.Lead && (t.Parent == l.v.unionType || t.Parent == l.v.intersectionType)
	case commaMark, semicolonMark:
		if prev != nil && string(prev.Text) == braceOpen && prev.Parent == l.v.objectType {
			return true
		}
		var next string
		if i+1 < len(l.tokens) {
			next = string(l.tokens[i+1].Text)
		}
		switch next {
		case ")", "]", ">", braceClose:
			return true
		}
	}
	return false
}

// spaced reports whether one space separates the token t from the token p
// before it in a spelling, by the rules of [lowering.spelling].
func (v *vocabulary) spaced(p, t *treesitter.Token) bool {
	if v.glued(p, t) {
		return false
	}
	switch string(t.Text) {
	case commaMark, semicolonMark, ")", "]", ">", ".", "?:", "-?:", "+?:":
		return false
	case braceClose:
		return string(p.Text) != braceOpen
	case colonMark, questionMark:
		return t.Parent == v.conditionalType
	}
	switch string(p.Text) {
	case "(", "[", "<", ".", "...", "-", "+":
		return false
	case braceOpen, commaMark, semicolonMark, colonMark, "?:", "-?:", "+?:", "=>", "=", barMark, ampersandMark:
		return true
	case questionMark:
		return p.Parent == v.conditionalType
	}
	switch string(t.Text) {
	case barMark, ampersandMark, "=>", "=":
		return true
	}
	if !p.Named && spacedKeywords[string(p.Text)] || !t.Named && spacedKeywords[string(t.Text)] {
		return true
	}
	last, _ := utf8.DecodeLastRune(p.Text)
	first, _ := utf8.DecodeRune(t.Text)
	return typescript.IsIdentifierPart(last) && typescript.IsIdentifierPart(first)
}

// glued reports whether the token t continues a string or a template
// literal type after the token p, as the source writes it. Every token of
// a string continues it but its opening quote, and so does every token of
// a template literal type but its opening quote. A substitution opens and
// closes without a space. A token after the closing quote of a string or a
// template literal type takes the spaces of the layout.
func (v *vocabulary) glued(p, t *treesitter.Token) bool {
	switch {
	case t.Parent == v.stringNode, t.Parent == v.templateLiteralType:
		return !t.Lead
	case p.Parent == v.stringNode, p.Parent == v.templateLiteralType:
		return p.Lead || p.Named
	case t.Parent == v.templateType:
		return true
	default:
		return p.Parent == v.templateType && p.Lead
	}
}
