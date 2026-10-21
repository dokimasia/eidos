// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"io/fs"
	"iter"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixed spellings of the carrier marks. Under the brand acme, a carrier
// opens with acme:, +acme: or -acme:. The first two set a directive, and
// the minus sign negates it.
const (
	carrierSet     = "+"
	carrierNegated = "-"
	carrierClose   = ":"
)

// MaxSyntaxFindings is the number of syntax errors of one file that
// [SourceUnit.ReportSyntax] reports one by one.
const MaxSyntaxFindings = 10

// SourceUnit is one compilation unit that a frontend parses. A Parse call
// touches nothing else. Parse reads bytes only through Read, and Read
// refuses every path outside the files of the unit and their declared
// shared inputs. The load keys the unit by the digests of those files, so
// a frontend that reads around Read depends on bytes that the key does not
// cover.
type SourceUnit struct {
	files   []SourceRef
	allowed map[string]bool
	fsys    fs.FS
	depth   Depth
	syntax  CommentSyntax
	// brand is the name that opens every carrier mark under the load.
	brand  string
	sink   *diag.Sink
	origin diag.Origin
	graph  *GraphBuilder
	// uses are the bare identities that [SourceUnit.Use] recorded, nil
	// until the first use.
	uses map[symbol.Identity]struct{}
}

// NewSourceUnit builds a unit for the load driver and the conformance
// suite. files are the members of the unit, and reads resolve in fsys.
// syntax is the comment syntax of the language, and each carrier of the
// unit opens with brand. The unit reports its findings to sink under
// origin. NewSourceUnit panics on an empty brand, because a unit without a
// brand cannot tell a carrier from a comment.
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

// Files returns the members of the unit in partition order.
func (u *SourceUnit) Files() []SourceRef { return u.files }

// Read returns the bytes of one file. It returns an error that contains
// the path for a path outside the files of the unit and their declared
// shared inputs. It reads a qualified path from the store that the path
// qualifies, through the tree of the unit as a [StoreFS], as [ReadFile]
// does.
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

// Depth returns the depth at which the unit loads. Parse reads it on the
// code path that signature-only loading shares with full loading.
func (u *SourceUnit) Depth() Depth { return u.depth }

// Brand returns the brand of the composition. Every carrier and every
// marker of a directive opens with the brand. A frontend compares the
// first name of the path of a marker with it before it lifts the arguments
// of the marker, so the frontend allocates nothing for a marker outside
// the brand. Brand allocates nothing.
func (u *SourceUnit) Brand() string { return u.brand }

// Graph returns the write handle of the unit into the node model.
func (u *SourceUnit) Graph() *GraphBuilder { return u.graph }

// Use records that a reference of the unit can resolve to a declaration
// outside the workspace under the bare identity id, which has the
// language, the package and the name of the declaration and no kind. Use
// keeps those three parts of id. The load passes each use of a unit once
// to a dependency round of the frontend that parsed the unit, and the
// round places a unit that declares it.
//
// # Allocation contract
//
// Use allocates the set of uses and its first group at the first use of
// the unit, and the growth of the set for an identity that it records the
// first time. Use does not allocate for a use that it recorded before.
func (u *SourceUnit) Use(id symbol.Identity) {
	if u.uses == nil {
		u.uses = map[symbol.Identity]struct{}{}
	}
	u.uses[symbol.Identity{Lang: id.Lang, Package: id.Package, Name: id.Name}] = struct{}{}
}

// Uses returns the uses of the unit in identity order, without repeats,
// and nil for a unit that records none. It allocates the list that it
// returns, and nothing for a unit without uses.
func (u *SourceUnit) Uses() []symbol.Identity {
	if len(u.uses) == 0 {
		return nil
	}
	return slices.SortedFunc(maps.Keys(u.uses), symbol.Identity.Compare)
}

// Carrier is one directive payload without its marker, with its position
// and its mark. The kernel grammar parses the payload.
type Carrier struct {
	// Mark is the mark as the author wrote it: brand:, +brand: or
	// -brand:. The unit quotes a refused carrier as Mark followed by
	// Payload.
	Mark    string
	Payload string
	Pos     position.Pos
	// DirectiveShaped is true when the carrier line has the tool-directive
	// shape. The syntax declares the convention, the marker is directly
	// before the text, and the text has the form tool:name, as brand: has
	// under a lowercase brand. A formatter may move such a line to the end
	// of its doc comment, as gofmt does.
	DirectiveShaped bool
}

// Negated reports whether the carrier opened with the negated mark.
func (c Carrier) Negated() bool { return strings.HasPrefix(c.Mark, carrierNegated) }

// Sugar is one marker that a frontend reads from the language's own syntax
// for metadata, such as a TypeScript decorator, a Rust attribute or a Java
// annotation.
type Sugar struct {
	// Path contains the parts of the name of the marker, split at the
	// separators of the language. The decorator @acme.stub has the path
	// acme, stub.
	Path []string
	// Args are the literal arguments of the marker in source order. An
	// argument is keyed where the language gives it a name, and positional
	// otherwise.
	Args []directive.RawArg
	// Refusal is the reason that the frontend could not lift an argument
	// of the marker, such as an argument that is not a literal. It is
	// empty when the frontend lifted every argument.
	Refusal string
	// Pos is the position of the marker.
	Pos position.Pos
}

// CommentParts contains the three parts of one raw comment. Docs are the
// documentation lines, Carriers are the carrier lines, and Annotations are
// the tool-directive lines, such as go:build. The frontend decides what
// each part means for its language. It filters its configuration lines
// out of the annotations before it attaches anything.
type CommentParts struct {
	Docs        []string
	Carriers    []Carrier
	Annotations symbol.Annotations
}

// Doc removes the markers of one raw comment by the syntax of the language
// and returns the clean lines. It drops line prefixes, block delimiters
// and gutters. Where the syntax declares the directive convention, Doc
// also drops each directive line that follows its marker directly, because
// a pragma is not documentation. Doc keeps carrier lines. A caller that
// needs the carriers apart uses [SourceUnit.Comment].
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

// Comment splits one raw comment into its parts by the syntax of the
// language. Each part is at its own line, counted from at. The parts are
// the documentation and the carriers, and Comment folds the continuation
// lines of a carrier into the carrier. Where the syntax declares the
// directive convention, Comment lowers each tool directive to an
// annotation. The annotation has the name without its marker and the
// arguments split at spaces, which is the spelling that the render side
// writes back.
//
// A carrier opens with one of the three marks of the brand, brand:,
// +brand: or -brand:, followed by a letter. The marker +name: of another
// tool and a markdown bullet therefore remain documentation. A line that
// opens with a mark of the brand is a carrier, also when the line has the
// tool-directive shape. A directive needs its marker directly before it,
// as the host toolchain requires, so prose after a marker and a space
// remains prose.
//
// Comment reports an Error under [ContinuedCarrier] for a continued
// carrier whose own line has the tool-directive shape, and returns no
// carrier for it, because a formatter may move such a line away from its
// continuation, as gofmt does. The Error quotes the carrier with the set
// mark. Comment reads the lines after it as they are written.
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

// AttachCarriers parses each carrier by the kernel grammar and attaches
// the directive to subject. The directive is at the line of the carrier.
// It is negated when the carrier opened with the negated mark, and
// directive-shaped when the line of the carrier has the tool-directive
// shape. AttachCarriers reports a carrier that the grammar refuses as an
// Error under refused, at the line of the carrier, and attaches nothing
// for it. The finding quotes the carrier as the author wrote it.
// AttachCarriers panics on a nil subject with a carrier to attach, as
// [GraphBuilder.Attach] does.
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
// whether the marker is a directive. AttachSugar reports false for a
// marker whose path does not start with the brand of the unit, because
// such a marker is ordinary metadata. The arguments of such a marker do
// not change the result.
//
// The path of a directive is the brand followed by a name, or by a plugin
// and a name. The first form is the bare directive name, and the second
// form is the name with the prefix of the plugin. AttachSugar reports a
// marker of the brand under refused, at the position of the marker, in
// these cases:
//
//   - the path has another form;
//   - the name is not a valid directive name;
//   - the frontend could not lift an argument, so the Refusal of the
//     marker is set.
//
// AttachSugar attaches nothing for such a marker, and reports true for it.
// An attached directive has the arguments and the position of the marker.
// It is set, because a marker has no negated form. AttachSugar panics on a
// nil subject with a marker of the brand, as [GraphBuilder.Attach] does.
//
// # Allocation contract
//
// AttachSugar allocates the prefixed name of a marker with a plugin and the
// growth of the list of attachments. It allocates nothing for a marker
// outside the brand.
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

// RefuseCarriers reports each carrier of cs as an Error under code. A
// frontend calls it for the carriers on a subject that the model cannot
// address, so the frontend drops no directive without a report. Each
// finding is at the line of its carrier, and it quotes the carrier as the
// author wrote it. The caller passes a description of the subject in what,
// such as "a parameter". RefuseCarriers reports nothing for an empty cs.
//
// # Allocation contract
//
// RefuseCarriers allocates nothing for an empty cs. A carrier costs its
// quoted text, the boxing of the two arguments and the message of the
// finding.
func (u *SourceUnit) RefuseCarriers(code diag.Code, cs []Carrier, what string) {
	for _, c := range cs {
		u.Errorf(
			code,
			c.Pos,
			"the model cannot address %s, so the directive %q is not attached. Move the carrier directly "+
				"above a declaration",
			what,
			c.Mark+c.Payload,
		)
	}
}

// RefuseSugar reports each marker of ss as an Error under code. A frontend
// calls it for the markers of the brand on a subject that the model cannot
// address. Each finding is at the position of its marker. The finding
// contains the path of the marker, joined by sep, such as "." for a
// TypeScript decorator. The caller passes a description of the subject in
// what. RefuseSugar reports nothing for an empty ss.
//
// # Allocation contract
//
// RefuseSugar allocates nothing for an empty ss. A marker costs its joined
// path, the boxing of the two arguments and the message of the finding.
func (u *SourceUnit) RefuseSugar(code diag.Code, ss []Sugar, sep, what string) {
	for _, s := range ss {
		u.Errorf(
			code,
			s.Pos,
			"the model cannot address %s, so the marker %s is not attached. Move the marker to a declaration",
			what,
			strings.Join(s.Path, sep),
		)
	}
}

// ReportSyntax reports the syntax errors of one file as Errors under code.
// errs yields the position and the description of each error in the order
// of the file. ReportSyntax reports the first [MaxSyntaxFindings] errors
// one by one. For the errors past the cap, it reports one finding at the
// first of them with their count. ReportSyntax reports nothing for an
// empty errs.
//
// # Allocation contract
//
// ReportSyntax allocates four objects for its loop state, because the
// range over errs moves the loop body and its counters to the heap. Each
// finding costs the boxing of its argument and its message.
func (u *SourceUnit) ReportSyntax(code diag.Code, errs iter.Seq2[position.Pos, string]) {
	var (
		reported, more int
		moreAt         position.Pos
	)
	for at, msg := range errs {
		if reported < MaxSyntaxFindings {
			u.Errorf(code, at, "%s", msg)
			reported++
			continue
		}
		if more == 0 {
			moreAt = at
		}
		more++
	}
	if more > 0 {
		u.Errorf(code, moreAt, "and %d more syntax errors", more)
	}
}

// DocLines filters lines that are already clean. Where the syntax declares
// the directive convention, it drops the lines of the tool:name directive
// shape, and it changes nothing else. A clean line has no marker, so
// DocLines keeps the legacy space forms line, extern and export, because
// those words also open ordinary prose. A caller with raw comments uses
// [SourceUnit.Doc], which sees the markers.
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

// Errorf reports a finding of Error severity at the given position under
// the origin of the frontend.
func (u *SourceUnit) Errorf(c diag.Code, at position.Pos, format string, a ...any) {
	u.sink.Errorf(c, at, u.origin, format, a...)
}

// Warnf reports a finding of Warning severity at the given position under
// the origin of the frontend.
func (u *SourceUnit) Warnf(c diag.Code, at position.Pos, format string, a ...any) {
	u.sink.Warnf(c, at, u.origin, format, a...)
}

// Infof reports a finding of Info severity at the given position under
// the origin of the frontend.
func (u *SourceUnit) Infof(c diag.Code, at position.Pos, format string, a ...any) {
	u.sink.Infof(c, at, u.origin, format, a...)
}

// directiveShaped reports whether a comment line has the tool-directive
// shape. A line has the shape when the syntax declares the convention, the
// marker is directly before the text, and the text has the form tool:name.
func (u *SourceUnit) directiveShaped(line commentLine) bool {
	return u.syntax.Directives && line.adjacent && directiveLine(line.text)
}

// commentLine is one comment line without its markers. adjacent is true
// when a line marker directly precedes the text. The directive rule
// requires that adjacency, and a block form never has it, because the host
// toolchains read directives only from line comments.
type commentLine struct {
	text     string
	adjacent bool
}

// commentLines removes the markers of one comment and returns one entry
// for each source line, so an index maps to a line offset. It removes the
// delimiters and the gutter of a block, or the prefix of each line, and
// one space after either. The longest form that matches applies, because
// the markers of one language can begin with each other, as Rust's //, ///
// and //! do. The shorter marker would leave the rest of the longer one in
// the text. commentLines allocates only the returned list.
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

// enclosingBlock returns the block form whose opener and closer enclose
// the whole comment, with the longest opener where several forms do. It
// returns false when no form does.
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

// longestPrefix returns the longest marker at the start of the line, or
// the empty string when the line starts with no marker.
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
// brand and the payload after it. It tries the negated mark, then the
// explicit set mark, then the bare set mark. It reports false for a line
// that opens with none of them, and for a mark without a letter after it,
// so a mark alone never counts as a directive. The mark is a prefix of the
// line, so the cut allocates nothing.
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
// brand, the closing colon and a letter, at the letter. It reports false
// for any other line.
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

// CutCarrier splits a clean comment line into its carrier mark under a
// brand and the payload after it, as [SourceUnit.Comment] does for a
// carrier line. The marks brand: and +brand: set a directive, and -brand:
// negates one. CutCarrier reports false for a line that opens no carrier.
// Every language uses the same marks, so an author writes a directive the
// same way in the comments of each language. Two tools on the kernel in
// one repository each read only the carriers of their own brand.
// CutCarrier allocates nothing, because the mark and the payload share the
// bytes of the line.
func CutCarrier(line, brand string) (mark, payload string, ok bool) {
	return cutCarrier(line, brand)
}

// legacyDirective reports whether a line opens with one of the three
// space-form directives that the go toolchain accepts for compatibility.
// They are line, extern and export. Only a raw line directly after its
// marker can be one.
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

// directiveLine reports whether a clean line is a tool directive and not
// documentation. A tool directive is a line such as go:build. directiveLine
// applies the rule of go/ast. Every character up to the colon, and the
// character after the colon, is a lowercase letter or a digit. A doc line
// with a bare URL remains documentation, because the slash after "https:"
// fails the check.
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

// GraphBuilder is the write handle of a unit into the node model. A unit
// declares every package that its files declare. The splice merges two
// units that contribute the same package path, and appends their
// declarations in unit order. The splice validates what a unit built and
// panics on a structural defect, such as an emit-side symbol or a named
// kind without a name. A malformed graph that the resolution step finds
// would point away from the frontend that built it.
type GraphBuilder struct {
	packages    map[string]*node.Package
	order       []string
	scopes      []ScopeRecord
	attachments []Attachment
	stamps      []StampRecord
	// moduleValue and rootValue are the boxed module and root of the last
	// call of [GraphBuilder.StampModule]. A call with the same strings
	// reuses them, so the packages of one module share one boxing of each.
	moduleValue, rootValue any
}

// ScopeRecord pairs one parsed file with its import bindings in the form
// of its language. The file is the node that the unit built, because the
// splice assigns the canonical identities after the parse. The kernel
// derives the identity at the splice and passes the bindings only to the
// Resolve of that language.
type ScopeRecord struct {
	File     *node.File
	Bindings any
}

// Attachment is one raw directive instance on a declaration that this unit
// built. The subject is a pointer for the same reason as the file of a
// [ScopeRecord]. The splice resolves it to the assigned identity, so an
// attachment on a declaration that another unit also declared attaches to
// the identity that the splice keeps.
type Attachment struct {
	Subject symbol.Symbol
	Raw     directive.Raw
}

// StampRecord is one classification stamp on a declaration that this unit
// built. It is the second class of raw attachment, and the splice resolves
// it as it resolves an [Attachment]. The splice sets the origin of the
// stamp and overwrites the origin that a frontend set, so every stamp is
// attributed to the frontend that recorded it.
type StampRecord struct {
	Subject symbol.Symbol
	Stamp   meta.RawStamp
}

// newGraphBuilder returns an empty builder for one unit.
func newGraphBuilder() *GraphBuilder {
	return &GraphBuilder{packages: map[string]*node.Package{}}
}

// Package returns the package of one path, and creates it on the first
// call for the path. Every file of that path joins the package. The empty
// path is the package of the global scope or of the unnamed package of a
// language, and it has no segments.
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

// Scope records the import bindings of one file. The Resolve of the
// language of the file reads them at the resolution phase. Scope panics on
// a nil file, because no parsed file could ever join the record.
func (gb *GraphBuilder) Scope(file *node.File, bindings any) {
	if file == nil {
		panic("plugin: a scope on a nil file records nothing")
	}
	gb.scopes = append(gb.scopes, ScopeRecord{File: file, Bindings: bindings})
}

// Attach records one raw directive instance on a declaration that this
// unit built. Attach panics on a nil subject.
func (gb *GraphBuilder) Attach(subject symbol.Symbol, raw directive.Raw) {
	if subject == nil {
		panic("plugin: a directive on a nil subject cannot be attached")
	}
	gb.attachments = append(gb.attachments, Attachment{Subject: subject, Raw: raw})
}

// Stamp records one classification stamp on a declaration that this unit
// built. The stamp is what a classifier saw. The load moves it into the
// fact store at plugin authority once the registry of the run exists.
// Stamp panics on a nil subject.
func (gb *GraphBuilder) Stamp(subject symbol.Symbol, s meta.RawStamp) {
	if subject == nil {
		panic("plugin: a stamp on a nil subject indexes nowhere")
	}
	gb.stamps = append(gb.stamps, StampRecord{Subject: subject, Stamp: s})
}

// StampModule records the module of package p. It stamps [meta.ModuleKey]
// with module, and [meta.ModuleRootKey] with root, which is the
// workspace-relative directory of the manifest of the module. Both stamps
// are at the position of the first file of p, or at the position of p when
// p has no file. StampModule records nothing for an empty module, because
// a package outside every module has no module facts. A frontend calls it
// once for each package that it declares. StampModule panics on a nil p
// with a module to record, as [GraphBuilder.Stamp] does.
//
// # Allocation contract
//
// StampModule boxes module and root where they differ from the strings of
// the call before it, so the packages of one module share one boxing of
// each. It also allocates the growth of the list of stamps.
func (gb *GraphBuilder) StampModule(p *node.Package, module, root string) {
	if module == "" {
		return
	}
	if p == nil {
		panic("plugin: a module on a nil package indexes nowhere")
	}
	at := p.Pos
	if len(p.Files) > 0 {
		at = p.Files[0].Pos
	}
	if last, ok := gb.moduleValue.(string); !ok || last != module {
		gb.moduleValue = module
	}
	if last, ok := gb.rootValue.(string); !ok || last != root {
		gb.rootValue = root
	}
	gb.Stamp(p, meta.RawStamp{Key: meta.ModuleKey, Value: gb.moduleValue, Pos: at})
	gb.Stamp(p, meta.RawStamp{Key: meta.ModuleRootKey, Value: gb.rootValue, Pos: at})
}

// Rehome moves every recorded attachment and stamp from one subject to
// another. A frontend calls it when a rewrite pass replaces a declaration
// that already has attachments. The directives of the author then apply to
// the replacement, and never to a declaration that the graph does not
// identify. Rehome panics on a nil subject on either side.
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

// Packages returns the packages of the unit in the order of their first
// call of [GraphBuilder.Package]. The splice appends them in that order.
// Partition order fixes the order of the calls, so the result is
// deterministic.
func (gb *GraphBuilder) Packages() []*node.Package {
	out := make([]*node.Package, 0, len(gb.order))
	for _, path := range gb.order {
		out = append(out, gb.packages[path])
	}
	return out
}

// Scopes returns the recorded bindings in record order. One goroutine
// parses the unit, so the order is the order in which the frontend
// recorded them.
func (gb *GraphBuilder) Scopes() []ScopeRecord { return gb.scopes }

// Attachments returns the recorded attachments in record order.
func (gb *GraphBuilder) Attachments() []Attachment { return gb.attachments }

// StampRecords returns the recorded stamps in record order.
func (gb *GraphBuilder) StampRecords() []StampRecord { return gb.stamps }
