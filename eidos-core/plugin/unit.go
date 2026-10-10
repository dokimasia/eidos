// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"io/fs"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The carrier marks' fixed spellings. Under the brand acme a carrier
// opens with acme:, +acme: or -acme:. The first two set a directive,
// and the minus sign negates it.
const (
	carrierSet     = "+"
	carrierNegated = "-"
	carrierClose   = ":"
)

// SourceUnit is one frontend compilation unit under parse: the only
// surface a Parse call touches. Bytes enter through Read alone,
// jailed to the unit's files and their declared shared inputs. The
// load keys the unit by the digests of those files, so a frontend
// that reads around Read depends on bytes the key does not cover.
type SourceUnit struct {
	files   []SourceRef
	allowed map[string]bool
	fsys    fs.FS
	depth   Depth
	syntax  CommentSyntax
	// brand is what a carrier line's mark spells under the load.
	brand  string
	sink   *diag.Sink
	origin diag.Origin
	graph  *GraphBuilder
}

// NewSourceUnit assembles a unit for the load driver and the
// conformance suite: the member files, the tree reads resolve in,
// the depth, the language's comment syntax, the composition's brand
// the unit's carriers open with, and the sink findings report to
// under the frontend's origin. An empty brand is a driver defect
// and panics, because a unit without a brand cannot tell a carrier
// from a comment.
func NewSourceUnit(
	files []SourceRef, fsys fs.FS, depth Depth,
	syntax CommentSyntax, brand string, sink *diag.Sink, origin diag.Origin,
) *SourceUnit {
	if brand == "" {
		panic("plugin: a unit without a brand cannot tell a carrier from a comment")
	}
	allowed := make(map[string]bool, len(files)*2)
	for _, f := range files {
		allowed[f.Path] = true
		for _, s := range f.Shared {
			allowed[s] = true
		}
	}
	return &SourceUnit{
		files:   files,
		allowed: allowed,
		fsys:    fsys,
		depth:   depth,
		syntax:  syntax,
		brand:   brand,
		sink:    sink,
		origin:  origin,
		graph:   newGraphBuilder(),
	}
}

// Files returns the unit's members, in partition order.
func (u *SourceUnit) Files() []SourceRef { return u.files }

// Read returns one file's bytes through the unit's one door. A
// path outside the unit's files and their declared shared inputs
// refuses, naming the path. A qualified path reads from the store it
// names, through the unit's tree as a [StoreFS], the way [ReadFile]
// resolves it.
func (u *SourceUnit) Read(path string) ([]byte, error) {
	if !u.allowed[path] {
		return nil, fmt.Errorf(
			"plugin: %s is outside the unit's files and shared inputs", path,
		)
	}
	b, err := ReadFile(u.fsys, path)
	if err != nil {
		return nil, fmt.Errorf("plugin: read %s: %w", path, err)
	}
	return b, nil
}

// Depth returns how deep this unit loads. Parse observes it on the
// one code path signature-only loading shares with the full one.
func (u *SourceUnit) Depth() Depth { return u.depth }

// Brand returns the composition's brand, which opens every carrier and
// every marker of a directive. A frontend compares the first name of a
// marker's path with it before it lifts the marker's arguments, so a
// marker outside the brand allocates nothing. Brand allocates nothing.
func (u *SourceUnit) Brand() string { return u.brand }

// Graph returns the unit's write handle into the node model.
func (u *SourceUnit) Graph() *GraphBuilder { return u.graph }

// Carrier is one directive payload and its line, marker stripped,
// ready for the kernel grammar, and the mark it opened with.
type Carrier struct {
	// Mark is the mark as the author wrote it: brand:, +brand: or
	// -brand:. A refusal quotes the carrier as Mark and Payload.
	Mark    string
	Payload string
	Pos     position.Pos
	// DirectiveShaped reports that the carrier line has the
	// tool-directive shape: the syntax declares the convention, the
	// marker is adjacent, and the text reads tool:name, as brand:
	// does under a lowercase brand. A formatter may move such a line
	// to the end of its doc comment, as gofmt does.
	DirectiveShaped bool
}

// Negated reports whether the carrier opened with the negated mark.
func (c Carrier) Negated() bool { return strings.HasPrefix(c.Mark, carrierNegated) }

// Sugar is one marker that a frontend reads from the language's own
// syntax for metadata, such as a TypeScript decorator, a Rust attribute
// or a Java annotation.
type Sugar struct {
	// Path is the marker's name, split at the language's separators.
	// The decorator @acme.stub has the path acme, stub.
	Path []string
	// Args are the marker's literal arguments in source order. An
	// argument is keyed where the language gives it a name, and
	// positional otherwise.
	Args []directive.RawArg
	// Refusal is the reason that the frontend could not lift an argument
	// of the marker, such as an argument that is not a literal. It is
	// empty when the frontend lifted every argument.
	Refusal string
	// Pos is the position of the marker.
	Pos position.Pos
}

// CommentParts is one raw comment taken apart three ways: the
// documentation lines, the carrier lines, and the tool-directive
// lines, such as go:build, as annotations. The frontend decides what
// each part means for its language: it filters its configuration
// lines out of the annotations before attaching anything.
type CommentParts struct {
	Docs        []string
	Carriers    []Carrier
	Annotations symbol.Annotations
}

// Doc strips one raw comment's markers through the language's
// syntax and returns the clean lines: line prefixes dropped, and
// block delimiters and gutters removed. Where the syntax declares
// the directive convention, Doc also excludes marker-adjacent
// directive lines, because a pragma is not documentation and would
// render double-commented downstream. Carrier lines remain. A
// caller that splits carriers out uses [SourceUnit.Comment].
func (u *SourceUnit) Doc(raw string) []string {
	lines := commentLines(raw, u.syntax)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if u.syntax.Directives && line.adjacent &&
			(directiveLine(line.text) || legacyDirective(line.text)) {
			continue
		}
		out = append(out, line.text)
	}
	return trimBlank(out)
}

// Comment takes one raw comment apart through the language's
// syntax, each part positioned at its own line from the given
// base: documentation, and carriers with their continuations
// folded. Where the syntax declares the directive convention, tool
// directives lower as annotations: the name without its marker, and
// the arguments split on spaces, which is the spelling the render
// side writes back. A carrier opens with one of the brand's three marks,
// brand:, +brand: or -brand:, followed by a letter, so another
// tool's +name: marker and a markdown bullet remain documentation.
// A carrier line under the brand is a carrier even where it has the
// tool-directive shape. A directive needs its marker adjacent, the
// way the host toolchain reads it, so prose after a spaced marker
// remains prose.
//
// A continued carrier whose own line has the tool-directive shape
// reports an Error under [ContinuedCarrier] that quotes the carrier
// with the set mark, and returns no carrier: a formatter may move
// such a line away from its continuation, as gofmt does. The lines
// after it read as they are written.
func (u *SourceUnit) Comment(raw string, at position.Pos) CommentParts {
	var parts CommentParts
	lines := commentLines(raw, u.syntax)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		lineAt := at
		lineAt.Line += i
		mark, payload, carried := cutCarrier(line.text, u.brand)
		switch {
		case carried:
			shaped := u.directiveShaped(line)
			if strings.HasSuffix(payload, directive.Continuation) {
				if shaped {
					u.Errorf(ContinuedCarrier, lineAt,
						"%q continues onto the next line, and a formatter may move a carrier in this "+
							"form away from its continuation: write it as %q",
						mark+payload, carrierSet+u.brand+carrierClose+payload)
					continue
				}
				span := []string{payload}
				for strings.HasSuffix(span[len(span)-1], directive.Continuation) &&
					i+1 < len(lines) {
					i++
					span = append(span, lines[i].text)
				}
				payload = directive.Join(span)
			}
			parts.Carriers = append(parts.Carriers,
				Carrier{Mark: mark, Payload: payload, Pos: lineAt, DirectiveShaped: shaped})
		case u.syntax.Directives && line.adjacent &&
			(directiveLine(line.text) || legacyDirective(line.text)):
			name, rest, _ := strings.Cut(line.text, " ")
			annotation := symbol.Annotation{Name: name}
			if rest != "" {
				annotation.Args = strings.Fields(rest)
			}
			parts.Annotations = append(parts.Annotations, annotation)
		default:
			if parts.Docs == nil {
				// The lines left bound the documentation, so the list is
				// allocated once.
				parts.Docs = make([]string, 0, len(lines)-i)
			}
			parts.Docs = append(parts.Docs, line.text)
		}
	}
	parts.Docs = trimBlank(parts.Docs)
	return parts
}

// AttachCarriers parses each carrier under the kernel grammar and
// attaches it to subject, positioned at the carrier's own line,
// negated where the carrier opened with the negated mark, and
// directive-shaped where its line has the tool-directive shape. A
// carrier the grammar refuses reports under the code the frontend
// states for it, at the carrier's line, quoting the carrier as
// written, and attaches nothing. A nil subject with a carrier to
// attach is a frontend defect and panics, as [GraphBuilder.Attach]
// does.
func (u *SourceUnit) AttachCarriers(subject symbol.Symbol, cs []Carrier, refused diag.Code) {
	for _, c := range cs {
		raw, err := directive.Parse(c.Payload)
		if err != nil {
			u.Errorf(refused, c.Pos, "%q: %v", c.Mark+c.Payload, err)
			continue
		}
		raw.Pos = c.Pos
		raw.Negated = c.Negated()
		raw.DirectiveShaped = c.DirectiveShaped
		u.graph.Attach(subject, raw)
	}
}

// AttachSugar attaches the directive of a marker to subject, and reports
// whether the marker is a directive. A marker whose path does not start
// with the unit's brand is ordinary metadata, and AttachSugar reports
// false for it, whatever its arguments are.
//
// The path of a directive is the brand followed by a name, or by a
// plugin and a name. The first form gives the bare directive name, and
// the second form gives the name with the plugin's prefix. AttachSugar
// reports a marker of the brand under refused, at the marker's
// position, in these cases:
//
//   - the path has another form;
//   - the name is not a valid directive name;
//   - the frontend could not lift an argument, so the marker's Refusal
//     is set.
//
// Such a marker attaches nothing, and AttachSugar reports true for it.
// An attached directive has the marker's arguments and position. It is
// set, because a marker has no negated form. A nil subject with a marker
// of the brand is a frontend defect, and AttachSugar panics, as
// [GraphBuilder.Attach] does.
//
// # Allocation contract
//
// AttachSugar allocates the prefixed name of a marker with a plugin, and
// the growth of the unit's attachment list. A marker outside the brand
// allocates nothing.
func (u *SourceUnit) AttachSugar(subject symbol.Symbol, s Sugar, refused diag.Code) bool {
	if len(s.Path) == 0 || s.Path[0] != u.brand {
		return false
	}
	var name directive.Name
	switch len(s.Path) {
	case 2:
		name = directive.Name(s.Path[1])
	case 3:
		// The canonical name of a schema without a plugin is bare, so an
		// empty plugin leaves the name empty. The empty name is not valid.
		if s.Path[1] != "" {
			name = directive.Schema{Plugin: s.Path[1], Name: directive.Name(s.Path[2])}.Canonical()
		}
	default:
		u.Errorf(refused, s.Pos,
			"a marker of the brand %s has %d names after the brand, but a directive has a name, "+
				"or a plugin and a name", u.brand, len(s.Path)-1)
		return true
	}
	if !name.Valid() {
		u.Errorf(refused, s.Pos,
			"a marker of the brand %s has the name %q, but a directive name is a letter followed by letters, "+
				"digits, hyphens and underscores", u.brand, strings.Join(s.Path[1:], " "))
		return true
	}
	if s.Refusal != "" {
		u.Errorf(refused, s.Pos,
			"the marker of the directive %s has an argument that is not a directive value: %s", name, s.Refusal)
		return true
	}
	u.graph.Attach(subject, directive.Raw{Name: name, Args: s.Args, Pos: s.Pos})
	return true
}

// DocLines filters lines the author already has clean: the
// tool:name directive shape is excluded under the syntax's
// declaration, and nothing else changes. Marker adjacency is gone
// from a clean line, so the legacy space forms line, extern and
// export remain, because those words open ordinary prose too. A
// caller with raw comments uses [SourceUnit.Doc], which still
// knows the marker.
func (u *SourceUnit) DocLines(lines []string) []string {
	if !u.syntax.Directives {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if directiveLine(line) {
			continue
		}
		out = append(out, line)
	}
	return out
}

// Errorf reports at Error severity at the given position, under
// the frontend's origin.
func (u *SourceUnit) Errorf(c diag.Code, at position.Pos, format string, a ...any) {
	u.sink.Errorf(c, at, u.origin, format, a...)
}

// Warnf reports at Warning severity at the given position, under
// the frontend's origin.
func (u *SourceUnit) Warnf(c diag.Code, at position.Pos, format string, a ...any) {
	u.sink.Warnf(c, at, u.origin, format, a...)
}

// Infof reports at Info severity at the given position, under the
// frontend's origin.
func (u *SourceUnit) Infof(c diag.Code, at position.Pos, format string, a ...any) {
	u.sink.Infof(c, at, u.origin, format, a...)
}

// directiveShaped reports whether a comment line has the
// tool-directive shape: the syntax declares the convention, the
// marker is adjacent, and the text reads tool:name.
func (u *SourceUnit) directiveShaped(line commentLine) bool {
	return u.syntax.Directives && line.adjacent && directiveLine(line.text)
}

// commentLine is one comment line with its markers stripped: the
// clean text, and whether a line marker immediately precedes it.
// The directive rule requires that adjacency, and a block form
// never has it, because the host toolchains read directives off
// line comments alone.
type commentLine struct {
	text     string
	adjacent bool
}

// commentLines removes one comment's markers, one entry per source
// line so an index maps back to a line offset: a block's delimiters
// and per-line gutter, or each line's line prefix, one leading space
// tolerated after either. The longest form that matches decides,
// because one language's markers can begin with each other, as Rust's
// //, /// and //! do, and the shorter marker would leave the rest of
// the longer one in the text. It allocates the returned list alone.
func commentLines(raw string, syntax CommentSyntax) []commentLine {
	if b, enclosed := enclosingBlock(raw, syntax.Blocks); enclosed {
		body := strings.TrimSuffix(strings.TrimPrefix(raw, b.Open), b.Close)
		out := make([]commentLine, 0, strings.Count(body, "\n")+1)
		for line := range strings.SplitSeq(body, "\n") {
			trimmed := strings.TrimSpace(line)
			if b.Gutter != "" {
				trimmed = strings.TrimPrefix(trimmed, b.Gutter)
				trimmed = strings.TrimPrefix(trimmed, " ")
			}
			out = append(out, commentLine{text: trimmed})
		}
		return out
	}
	out := make([]commentLine, 0, strings.Count(raw, "\n")+1)
	for line := range strings.SplitSeq(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		adjacent := false
		if marker := longestPrefix(trimmed, syntax.Line); marker != "" {
			rest := trimmed[len(marker):]
			adjacent = rest != "" && rest[0] != ' ' && rest[0] != '\t'
			trimmed = strings.TrimPrefix(rest, " ")
		}
		out = append(out, commentLine{text: trimmed, adjacent: adjacent})
	}
	return out
}

// enclosingBlock returns the block form whose opener and closer
// enclose the whole comment, the longest opener among several, and
// false when no form does.
func enclosingBlock(raw string, blocks []CommentBlock) (CommentBlock, bool) {
	var best CommentBlock
	found := false
	for _, b := range blocks {
		if (!found || len(b.Open) > len(best.Open)) &&
			strings.HasPrefix(raw, b.Open) && strings.HasSuffix(raw, b.Close) {
			best, found = b, true
		}
	}
	return best, found
}

// longestPrefix returns the longest marker the line starts with, and
// the empty string when it starts with none.
func longestPrefix(line string, markers []string) string {
	best := ""
	for _, m := range markers {
		if len(m) > len(best) && strings.HasPrefix(line, m) {
			best = m
		}
	}
	return best
}

// cutCarrier splits a clean comment line into its carrier mark under a
// brand and the payload that follows. It tries the negated mark, then
// the explicit set mark, then the bare set mark, and reports false for
// a line that opens with none of them, and for a mark followed by
// anything but a letter, so a bare mark never reads as authored intent.
// The mark is the line's own leading bytes, so the cut allocates
// nothing.
func cutCarrier(line, brand string) (mark, payload string, ok bool) {
	// Both signs are one byte.
	if line != "" && (line[0] == carrierNegated[0] || line[0] == carrierSet[0]) {
		if mark, payload, ok = cutMark(line, 1, brand); ok {
			return mark, payload, true
		}
	}
	return cutMark(line, 0, brand)
}

// cutMark splits a line whose bytes after the first sign bytes are the
// brand, the closing colon and a letter, at the letter. It reports
// false for any other line.
func cutMark(line string, sign int, brand string) (mark, payload string, ok bool) {
	rest, branded := strings.CutPrefix(line[sign:], brand)
	if !branded {
		return "", "", false
	}
	rest, closed := strings.CutPrefix(rest, carrierClose)
	if !closed || rest == "" {
		return "", "", false
	}
	if c := rest[0]; ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') {
		return line[:len(line)-len(rest)], rest, true
	}
	return "", "", false
}

// CutCarrier splits a clean comment line into the carrier mark it
// opens with under a brand and the payload that follows, the way
// [SourceUnit.Comment] reads a carrier line. The marks are brand:
// and +brand:, which set a directive, and -brand:, which negates
// one. It reports false for a line that opens no carrier. The marks
// are the kit's one cross-language convention, so a directive is
// spelled the same way in every language's comments, and two tools
// built on the kernel and run in one repository read only their own
// carriers. It allocates nothing, because the mark and the payload
// share the line's bytes.
func CutCarrier(line, brand string) (mark, payload string, ok bool) {
	return cutCarrier(line, brand)
}

// legacyDirective reports whether a line opens with one of the
// three space-form directives the go toolchain accepts for
// compatibility: line, extern and export. Only a marker-adjacent
// raw line can be one.
func legacyDirective(line string) bool {
	return strings.HasPrefix(line, "line ") ||
		strings.HasPrefix(line, "extern ") ||
		strings.HasPrefix(line, "export ")
}

// trimBlank drops leading and trailing empty lines, which comment
// delimiters leave behind.
func trimBlank(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && lines[start] == "" {
		start++
	}
	for end > start && lines[end-1] == "" {
		end--
	}
	return lines[start:end]
}

// directiveLine reports whether a clean line is a tool directive
// and not documentation: a tool:name line such as go:build. The
// rule is go/ast's own: everything up to and including the
// character after the colon is lowercase alphanumeric. A doc line
// with a bare URL therefore remains documentation, because the
// slash after "https:" fails the check.
func directiveLine(line string) bool {
	head, rest, found := strings.Cut(line, ":")
	if !found || head == "" || rest == "" {
		return false
	}
	for _, r := range head {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	next := rest[0]
	return (next >= 'a' && next <= 'z') || (next >= '0' && next <= '9')
}

// GraphBuilder is the unit's write handle into the node model. A
// unit declares as many packages as its bytes do. Two units that
// contribute one package path merge at the splice, declarations
// appended in unit order. The splice validates what a unit built and
// panics on a structural defect, such as an emit-side symbol or a
// named kind without a name, because a malformed graph found at the
// resolution step points away from the frontend that built it.
type GraphBuilder struct {
	packages    map[string]*node.Package
	order       []string
	scopes      []ScopeRecord
	attachments []Attachment
	stamps      []StampRecord
}

// ScopeRecord pairs one parsed file with its import bindings, in
// the language's own form. The file is the node the unit built,
// because canonical identities do not exist until the splice
// assigns them, and a derivation spelled twice would drift. The
// kernel derives the identity at the splice and hands the bindings
// back to that language's Resolve alone.
type ScopeRecord struct {
	File     *node.File
	Bindings any
}

// Attachment is one raw directive instance on a declaration this
// unit built. The subject is a pointer for the reason
// [ScopeRecord]'s file is: the splice resolves it to the assigned
// identity, so an attachment on a declaration another unit already
// declared attaches to the identity the splice keeps.
type Attachment struct {
	Subject symbol.Symbol
	Raw     directive.Raw
}

// StampRecord is one classification stamp on a declaration this
// unit built: the second raw attachment class, resolved at the
// splice the way [Attachment] is. The splice sets the stamp's
// origin and overwrites a frontend's own value, so every stamp is
// attributed to the frontend that recorded it.
type StampRecord struct {
	Subject symbol.Symbol
	Stamp   meta.RawStamp
}

// newGraphBuilder returns an empty builder for one unit.
func newGraphBuilder() *GraphBuilder {
	return &GraphBuilder{packages: map[string]*node.Package{}}
}

// Package returns the package for one path, created on first
// touch: the container every file of that path joins. The empty path
// is the package a language's global scope or unnamed package
// declares into, and it has no segments.
func (gb *GraphBuilder) Package(path string) *node.Package {
	if p, held := gb.packages[path]; held {
		return p
	}
	var segments []string
	if path != "" {
		segments = strings.Split(path, "/")
	}
	p := &node.Package{
		ID:   symbol.Identity{Package: path},
		Path: segments,
	}
	gb.packages[path] = p
	gb.order = append(gb.order, path)
	return p
}

// Scope records one file's import bindings: what the file's
// language reads in its Resolve at the resolution phase. A nil file is a
// frontend defect and panics, because nothing could ever join the
// record to a parsed file.
func (gb *GraphBuilder) Scope(file *node.File, bindings any) {
	if file == nil {
		panic("plugin: a scope on a nil file records nothing")
	}
	gb.scopes = append(gb.scopes, ScopeRecord{File: file, Bindings: bindings})
}

// Attach records one raw directive instance on a declaration this
// unit built. A nil subject is a frontend defect and panics.
func (gb *GraphBuilder) Attach(subject symbol.Symbol, raw directive.Raw) {
	if subject == nil {
		panic("plugin: a directive on a nil subject attaches nowhere")
	}
	gb.attachments = append(gb.attachments, Attachment{Subject: subject, Raw: raw})
}

// Stamp records one classification stamp on a declaration this
// unit built: what a classifier saw, bound for the fact store at
// plugin authority once the run's registry is in hand. A nil
// subject is a frontend defect and panics.
func (gb *GraphBuilder) Stamp(subject symbol.Symbol, s meta.RawStamp) {
	if subject == nil {
		panic("plugin: a stamp on a nil subject indexes nowhere")
	}
	gb.stamps = append(gb.stamps, StampRecord{Subject: subject, Stamp: s})
}

// Rehome moves every recorded attachment and stamp from one
// subject onto another: what a frontend calls when a rewrite pass
// replaces a declaration it already attached to, so authored
// intent follows the replacement and never dangles on a
// declaration the graph does not identify. A nil subject on either
// side is a frontend defect and panics.
func (gb *GraphBuilder) Rehome(from, to symbol.Symbol) {
	if from == nil || to == nil {
		panic("plugin: a rehoming between nil subjects moves nothing")
	}
	for i := range gb.attachments {
		if gb.attachments[i].Subject == from {
			gb.attachments[i].Subject = to
		}
	}
	for i := range gb.stamps {
		if gb.stamps[i].Subject == from {
			gb.stamps[i].Subject = to
		}
	}
}

// Packages returns the unit's packages in first-touch order: what
// the splice appends, deterministically, because partition order
// fixed the touches.
func (gb *GraphBuilder) Packages() []*node.Package {
	out := make([]*node.Package, 0, len(gb.order))
	for _, path := range gb.order {
		out = append(out, gb.packages[path])
	}
	return out
}

// Scopes returns the recorded bindings in record order, which the
// unit's single parse goroutine makes the frontend's own.
func (gb *GraphBuilder) Scopes() []ScopeRecord { return gb.scopes }

// Attachments returns the recorded attachments, in record order.
func (gb *GraphBuilder) Attachments() []Attachment { return gb.attachments }

// StampRecords returns the recorded stamps, in record order.
func (gb *GraphBuilder) StampRecords() []StampRecord { return gb.stamps }
