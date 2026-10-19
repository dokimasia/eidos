// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/lexer"
)

// The keywords that open a definition of GraphQL. The executable keywords
// and the opening brace of an anonymous query open an executable
// definition, and every other keyword opens a definition of the type
// system. KeywordImplements opens the interfaces of an object type and of
// an interface, which the frontend reads from the tokens.
const (
	keywordSchema       = "schema"
	keywordScalar       = "scalar"
	keywordType         = "type"
	keywordInterface    = "interface"
	keywordUnion        = "union"
	keywordEnum         = "enum"
	keywordInput        = "input"
	keywordDirective    = "directive"
	keywordExtend       = "extend"
	keywordQuery        = "query"
	keywordMutation     = "mutation"
	keywordSubscription = "subscription"
	keywordFragment     = "fragment"
	KeywordImplements   = "implements"
	keywordRepeatable   = "repeatable"
	keywordOn           = "on"
)

// blankRune replaces each rune of a definition that a copy of a file leaves
// out. A copy keeps every line terminator, so a position in the copy is the
// position in the file.
const blankRune = ' '

// LexError is one error of the lexer: its position in the file and the
// message of the lexer without a location, because a lexer that resumes
// after an earlier error counts its lines from the resume.
type LexError struct {
	At      ast.Position
	Message string
}

// span is a range of runes of a file, from start up to end.
type span struct{ start, end int }

// placed is the place that the lexer of gqlparser gives the string at the
// index token of the tokens: the line and the column after the opening
// quote of a string, and the last line of a block string with a column
// from the start of that line. The parser of gqlparser places an error at a
// string there.
type placed struct {
	token, line, column int
}

// Definition is one top-level definition of a file: the indices in
// [Lexed.Sig] of its first and its last token, whether it is executable,
// and whether it does not follow the grammar of a definition.
type Definition struct {
	First, Last int
	Executable  bool
	Broken      bool
}

// Lexed is the tokens of one file: every token other than the end of the
// file, comments included, in source order, and the errors of the lexer.
// The position of each token is its position in the file, with the line and
// the column of the rune where it starts.
//
// # Concurrency
//
// A Lexed is safe for concurrent reads. [Lex] writes it and no method
// changes it.
//
// # Allocation contract
//
// [Lex] allocates the lexed file, the source of each lexer, the list of the
// tokens as it grows, the lists of the significant tokens, of the pairs of
// their brackets and of the lines of the ends of the tokens once each, the
// errors and the skipped ranges of a file with errors of the lexer, and the
// places of the strings of a file with strings as the list grows. The
// methods allocate as each docblock states.
type Lexed struct {
	// Src is the source that Lex lexed.
	Src *ast.Source
	// Tokens are the tokens of the file in source order, comments included.
	Tokens []lexer.Token
	// Sig lists the indices in Tokens of the significant tokens, the tokens
	// that are not comments.
	Sig []int
	// Pair maps the index in Sig of each bracket to the index in Sig of the
	// bracket that pairs with it: the closing bracket of an opening bracket,
	// and the opening bracket of a closing bracket. An opening bracket that
	// does not close ends at the closing bracket of the group around it, or
	// at len(Sig) outside every group. A closing bracket that closes nothing
	// pairs with -1.
	Pair []int
	// Errors are the errors of the lexer in source order.
	Errors []LexError
	// skips are the rune ranges that an error of the lexer left out of the
	// tokens. Every copy of the file blanks them.
	skips []span
	// Ends is the line of the last rune of each token, which differs from
	// the line of its first rune for a block string over several lines.
	Ends []int
	// quoted are the places that the lexer of gqlparser gives the strings
	// and the block strings of the tokens, in source order.
	quoted []placed
}

// Lex lexes the source of one file through the lexer of gqlparser. The
// lexer stops at its first error, so Lex records the error and continues
// with a new lexer at the start of the next line. The tokens of the new
// lexer have the positions in the file. The skip of an error runs from the
// end of the token before it, because the lexer starts the token of a
// string that does not close after its opening quotes.
func Lex(src *ast.Source) *Lexed {
	l := &Lexed{Src: src}
	text := src.Input
	var base ast.Position
	for {
		lx := lexer.New(&ast.Source{Name: src.Name, Input: text})
		resumed := false
		for {
			tok, err := lx.ReadToken()
			tok.Pos = shifted(tok.Pos, base, src)
			if err != nil {
				message := err.Error()
				if ge, ok := errors.AsType[*gqlerror.Error](err); ok {
					message = ge.Message
				}
				l.Errors = append(l.Errors, LexError{At: tok.Pos, Message: message})
				rest, skipped, end := nextLine(text, tok.Pos.Line-base.Line)
				from := base.Start
				if n := len(l.Tokens); n > 0 {
					from = max(from, l.Tokens[n-1].Pos.End)
				}
				l.skips = append(l.skips, span{start: from, end: base.Start + skipped})
				if !end {
					text = rest
					base = ast.Position{Start: base.Start + skipped, Line: tok.Pos.Line}
					resumed = true
				}
				break
			}
			if tok.Kind == lexer.EOF {
				break
			}
			if tok.Kind == lexer.String || tok.Kind == lexer.BlockString {
				l.quoted = append(l.quoted, placed{token: len(l.Tokens), line: tok.Pos.Line, column: tok.Pos.Column})
			}
			l.Tokens = append(l.Tokens, tok)
		}
		if !resumed {
			break
		}
	}
	l.index()
	return l
}

// At returns the significant token at the index k of Sig, and the end of
// the file for an index past the last token. It allocates nothing.
func (l *Lexed) At(k int) lexer.Token {
	if k >= len(l.Sig) {
		return lexer.Token{Kind: lexer.EOF}
	}
	return l.Tokens[l.Sig[k]]
}

// Find returns the index in Sig of the significant token that starts at the
// rune offset start, and reports false where no token starts there. It
// allocates nothing.
func (l *Lexed) Find(start int) (int, bool) {
	return slices.BinarySearchFunc(l.Sig, start, func(i, offset int) int { return l.Tokens[i].Pos.Start - offset })
}

// Is reports whether the significant token at the index k of Sig is the
// name word. It allocates nothing.
func (l *Lexed) Is(k int, word string) bool {
	tok := l.At(k)
	return tok.Kind == lexer.Name && tok.Value == word
}

// Definitions splits the significant tokens into the top-level definitions
// of the file, by the grammar of GraphQL. A run of tokens that does not
// follow the grammar is a broken definition up to the next keyword of a
// definition at the top level. Definitions allocates the list of the
// definitions as it grows, and nothing for a file without tokens.
func (l *Lexed) Definitions() []Definition {
	var out []Definition
	for k := 0; k < len(l.Sig); {
		end, executable, ok := l.skipDefinition(k)
		if !ok {
			end = l.recover(max(end, k+1))
		}
		out = append(out, Definition{First: k, Last: end - 1, Executable: executable, Broken: !ok})
		k = end
	}
	return out
}

// Text returns the text of the file from the start of the significant token
// at the index from of Sig up to the end of the significant token at the
// index to, as written. The positions of the tokens count runes, so a file
// with runes outside ASCII maps them to bytes first. Text allocates
// nothing.
func (l *Lexed) Text(from, to int) string {
	start, end := l.At(from).Pos.Start, l.At(to).Pos.End
	input := l.Src.Input
	if len(input) == utf8.RuneCountInString(input) {
		return input[start:end]
	}
	lo, hi, at := len(input), len(input), 0
	for i := range input {
		if at == start {
			lo = i
		}
		if at == end {
			hi = i
			break
		}
		at++
	}
	return input[lo:hi]
}

// CopyWithout returns a copy of the file without the definitions defs whose
// entries in out are true and without the ranges that the lexer skipped.
// Each rune of a left-out range is a space, and a line terminator stays as
// it is, so a position in the copy is the position in the file. The copy is
// the text of the file where it leaves nothing out. CopyWithout allocates
// the copy, and the list of the ranges that it leaves out where the list
// does not fit on the stack, as three ranges do not. It allocates nothing
// where it leaves nothing out.
func (l *Lexed) CopyWithout(defs []Definition, out []bool) string {
	blanks := slices.Clone(l.skips)
	for i, d := range defs {
		if out[i] {
			blanks = append(blanks, span{start: l.At(d.First).Pos.Start, end: l.At(d.Last).Pos.End})
		}
	}
	if len(blanks) == 0 {
		return l.Src.Input
	}
	slices.SortFunc(blanks, func(a, b span) int { return a.start - b.start })
	var b strings.Builder
	b.Grow(len(l.Src.Input))
	at, next := 0, 0
	for _, r := range l.Src.Input {
		for next < len(blanks) && blanks[next].end <= at {
			next++
		}
		if next < len(blanks) && at >= blanks[next].start && r != '\n' && r != '\r' {
			b.WriteRune(blankRune)
		} else {
			b.WriteRune(r)
		}
		at++
	}
	return b.String()
}

// filePosition returns the line and the column in the file of the token
// that the parser of gqlparser places at line and column. The parser places
// a string after its opening quote and a block string on its last line, so
// filePosition returns the first rune of such a string, and every other
// place as it is. No other token starts at the place of a string, because
// the place is inside the string or before the start of its line.
// filePosition allocates nothing.
func (l *Lexed) filePosition(line, column int) (int, int) {
	for _, q := range l.quoted {
		if q.line == line && q.column == column {
			pos := l.Tokens[q.token].Pos
			return pos.Line, pos.Column
		}
	}
	return line, column
}

// index records the indices of the tokens that are not comments, the pairs
// of the brackets among them, and the lines of the first and the last rune
// of each token. The lexer of gqlparser gives a block string the line where
// it ends and a string the column after its opening quote, so index sets the
// line and the column of each token from the rune where it starts.
func (l *Lexed) index() {
	n := len(l.Tokens)
	l.Ends = make([]int, n)
	line, lineStart, first, last := 1, 0, 0, 0
	text := l.Src.Input
	for i, at := 0, 0; last < n && i < len(text); at++ {
		for ; first < n && l.Tokens[first].Pos.Start <= at; first++ {
			l.Tokens[first].Pos.Line = line
			l.Tokens[first].Pos.Column = l.Tokens[first].Pos.Start - lineStart + 1
		}
		for ; last < n && l.Tokens[last].Pos.End-1 <= at; last++ {
			l.Ends[last] = line
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		// A carriage return and a line feed end one line, and the lexer
		// counts both as runes.
		switch {
		case r == '\r' && i < len(text) && text[i] == '\n':
			i++
			at++
			line++
			lineStart = at + 1
		case r == '\n' || r == '\r':
			line++
			lineStart = at + 1
		default:
			// Every other rune continues its line.
		}
	}
	l.Sig = make([]int, 0, n)
	for i, tok := range l.Tokens {
		if tok.Kind != lexer.Comment {
			l.Sig = append(l.Sig, i)
		}
	}
	l.Pair = make([]int, len(l.Sig))
	var open []int
	for k := range l.Sig {
		switch l.At(k).Kind {
		case lexer.BraceL, lexer.ParenL, lexer.BracketL:
			open = append(open, k)
		case lexer.BraceR:
			open = l.close(open, k, lexer.BraceL)
		case lexer.ParenR:
			open = l.close(open, k, lexer.ParenL)
		case lexer.BracketR:
			open = l.close(open, k, lexer.BracketL)
		default:
			// A token that is not a bracket opens and closes nothing.
		}
	}
	for _, k := range open {
		l.Pair[k] = len(l.Sig)
	}
}

// close pairs the closing bracket at the index k of Sig with the innermost
// open bracket of the kind opener, and returns the brackets that stay open.
// An open bracket inside the pair ends at k. A closing bracket without an
// open bracket of its kind closes nothing, so a stray bracket does not end
// the group around it.
func (l *Lexed) close(open []int, k int, opener lexer.Type) []int {
	n := len(open) - 1
	for n >= 0 && l.At(open[n]).Kind != opener {
		n--
	}
	if n < 0 {
		l.Pair[k] = -1
		return open
	}
	for _, inner := range open[n+1:] {
		l.Pair[inner] = k
	}
	l.Pair[open[n]] = k
	l.Pair[k] = open[n]
	return open[:n]
}

// recover returns the index of the first token from k on that can open a
// definition at the top level: a keyword of a definition outside every
// bracket.
func (l *Lexed) recover(k int) int {
	depth := 0
	for ; k < len(l.Sig); k++ {
		tok := l.At(k)
		switch {
		case tok.Kind == lexer.BraceL || tok.Kind == lexer.ParenL || tok.Kind == lexer.BracketL:
			depth++
		case tok.Kind == lexer.BraceR || tok.Kind == lexer.ParenR || tok.Kind == lexer.BracketR:
			depth = max(depth-1, 0)
		case depth == 0 && tok.Kind == lexer.Name && opens(tok.Value):
			return k
		default:
			// Every other token continues the broken definition.
		}
	}
	return k
}

// skipDefinition returns the index after the definition at k, whether the
// definition is executable, and whether it follows the grammar.
func (l *Lexed) skipDefinition(k int) (int, bool, bool) {
	if kind := l.At(k).Kind; kind == lexer.String || kind == lexer.BlockString {
		k++
	}
	tok := l.At(k)
	if tok.Kind == lexer.BraceL {
		return min(l.Pair[k]+1, len(l.Sig)), true, true
	}
	if tok.Kind != lexer.Name {
		return k, false, false
	}
	switch tok.Value {
	case keywordQuery, keywordMutation, keywordSubscription:
		k++
		if l.At(k).Kind == lexer.Name {
			k++
		}
		k = l.skipGroup(k, lexer.ParenL)
		k = l.skipDirectives(k)
		end, ok := l.skipBody(k)
		return end, true, ok
	case keywordFragment:
		if l.At(k+1).Kind != lexer.Name {
			return k + 1, true, false
		}
		k = l.skipGroup(k+2, lexer.ParenL)
		if !l.Is(k, keywordOn) || l.At(k+1).Kind != lexer.Name {
			return k, true, false
		}
		end, ok := l.skipBody(l.skipDirectives(k + 2))
		return end, true, ok
	case keywordExtend:
		end, ok := l.skipTypeSystem(k + 1)
		return end, false, ok
	default:
		end, ok := l.skipTypeSystem(k)
		return end, false, ok
	}
}

// skipTypeSystem returns the index after the definition of the type system
// that opens with the keyword at k, and whether it follows the grammar.
func (l *Lexed) skipTypeSystem(k int) (int, bool) {
	tok := l.At(k)
	if tok.Kind != lexer.Name {
		return k, false
	}
	switch tok.Value {
	case keywordSchema:
		return l.skipGroup(l.skipDirectives(k+1), lexer.BraceL), true
	case keywordScalar, keywordType, keywordInterface, keywordUnion, keywordEnum, keywordInput:
		return l.skipNamed(tok.Value, k+1)
	case keywordDirective:
		if l.At(k+1).Kind != lexer.At || l.At(k+2).Kind != lexer.Name {
			return k + 1, false
		}
		k = l.skipGroup(k+3, lexer.ParenL)
		if l.Is(k, keywordRepeatable) {
			k++
		}
		if !l.Is(k, keywordOn) {
			return k, false
		}
		return l.skipList(k+1, lexer.Pipe), true
	default:
		return k, false
	}
}

// skipNamed returns the index after a named type definition that opens with
// keyword, from the token at k after the keyword on, and reports false
// where the definition has no name.
func (l *Lexed) skipNamed(keyword string, k int) (int, bool) {
	if l.At(k).Kind != lexer.Name {
		return k, false
	}
	k++
	switch keyword {
	case keywordScalar:
		return l.skipDirectives(k), true
	case keywordUnion:
		k = l.skipDirectives(k)
		if l.At(k).Kind == lexer.Equals {
			k = l.skipList(k+1, lexer.Pipe)
		}
		return k, true
	case keywordType, keywordInterface:
		if l.Is(k, KeywordImplements) {
			k = l.skipList(k+1, lexer.Amp)
		}
	default:
		// An enum and an input object have directives and a body alone.
	}
	return l.skipGroup(l.skipDirectives(k), lexer.BraceL), true
}

// skipNames returns the index after the name at k, and k where the token
// at k is not a name.
func (l *Lexed) skipNames(k int) int {
	if l.At(k).Kind == lexer.Name {
		return k + 1
	}
	return k
}

// skipList returns the index after a list of names that sep separates, with
// an optional leading sep, as an implements clause, the members of a union
// and the locations of a directive write it.
func (l *Lexed) skipList(k int, sep lexer.Type) int {
	if l.At(k).Kind == sep {
		k++
	}
	k = l.skipNames(k)
	for l.At(k).Kind == sep {
		k = l.skipNames(k + 1)
	}
	return k
}

// skipDirectives returns the index after the applied directives at k.
func (l *Lexed) skipDirectives(k int) int {
	for l.At(k).Kind == lexer.At {
		k = l.skipGroup(l.skipNames(k+1), lexer.ParenL)
	}
	return k
}

// skipGroup returns the index after the bracketed group at k where the
// token at k opens one of kind open, and k otherwise. The index after a
// group that does not close is the end of the tokens, because [Lexed.Pair]
// pairs such a bracket with the end of the group around it.
func (l *Lexed) skipGroup(k int, open lexer.Type) int {
	if l.At(k).Kind != open {
		return k
	}
	return min(l.Pair[k]+1, len(l.Sig))
}

// skipBody returns the index after the selection set at k, and reports
// false where k opens no selection set. The index after a set that does not
// close is the end of the tokens, as for skipGroup.
func (l *Lexed) skipBody(k int) (int, bool) {
	if l.At(k).Kind != lexer.BraceL {
		return k, false
	}
	return min(l.Pair[k]+1, len(l.Sig)), true
}

// shifted returns the position p of a token of a lexer that started at
// base, as a position in the file of src. A lexer starts at the start of a
// line, so a column needs no shift.
func shifted(p, base ast.Position, src *ast.Source) ast.Position {
	p.Start += base.Start
	p.End += base.Start
	p.Line += base.Line
	p.Src = src
	return p
}

// nextLine returns the text after the line of an error, the number of
// runes that it skips and whether the text ends on the line. line is the
// line of the error, counted from 1, in the text that the failed lexer
// read. A line ends at a line feed, a carriage return or both together, as
// the lexer of GraphQL ends a line.
func nextLine(text string, line int) (string, int, bool) {
	runes := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		runes++
		if r != '\n' && r != '\r' {
			continue
		}
		if r == '\r' && i < len(text) && text[i] == '\n' {
			i++
			runes++
		}
		if line--; line == 0 {
			return text[i:], runes, false
		}
	}
	return "", runes, true
}

// opens reports whether a keyword opens a definition.
func opens(word string) bool {
	switch word {
	case keywordSchema, keywordScalar, keywordType, keywordInterface, keywordUnion, keywordEnum,
		keywordInput, keywordDirective, keywordExtend, keywordQuery, keywordMutation,
		keywordSubscription, keywordFragment:
		return true
	default:
		return false
	}
}
