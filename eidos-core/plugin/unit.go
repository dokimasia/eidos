// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io/fs"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// SourceUnit is one frontend compilation unit under parse: the only
// surface a Parse call touches. Bytes enter through Read alone,
// jailed to the unit's files and their declared shared inputs,
// and every accepted read folds into the unit's fingerprint, so a
// frontend cannot depend on bytes the cache does not know about.
type SourceUnit struct {
	files   []SourceRef
	allowed map[string]bool
	fsys    fs.FS
	depth   Depth
	syntax  CommentSyntax
	sink    *diag.Sink
	origin  diag.Origin
	graph   *GraphBuilder
	reads   hash.Hash
}

// NewSourceUnit assembles a unit for the load driver and the
// conformance suite: the member files, the tree reads resolve in,
// the depth, the language's comment syntax, and the sink findings
// report to under the frontend's origin.
func NewSourceUnit(
	files []SourceRef, fsys fs.FS, depth Depth,
	syntax CommentSyntax, sink *diag.Sink, origin diag.Origin,
) *SourceUnit {
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
		sink:    sink,
		origin:  origin,
		graph:   newGraphBuilder(),
		reads:   sha256.New(),
	}
}

// Files returns the unit's members, in partition order.
func (u *SourceUnit) Files() []SourceRef { return u.files }

// Read returns one file's bytes through the unit's one door. A
// path outside the unit's files and their declared shared inputs
// refuses, naming the path, and every accepted read folds path
// and content into the unit's fingerprint.
func (u *SourceUnit) Read(path string) ([]byte, error) {
	if !u.allowed[path] {
		return nil, fmt.Errorf(
			"plugin: %s is outside the unit's files and shared inputs", path,
		)
	}
	b, err := fs.ReadFile(u.fsys, path)
	if err != nil {
		return nil, fmt.Errorf("plugin: read %s: %w", path, err)
	}
	u.reads.Write([]byte(path))
	u.reads.Write([]byte{0})
	u.reads.Write(b)
	u.reads.Write([]byte{0})
	return b, nil
}

// Depth says how deep this unit loads; Parse observes it on the
// one code path signature-only loading shares with the full one.
func (u *SourceUnit) Depth() Depth { return u.depth }

// Graph returns the unit's write handle into the node model.
func (u *SourceUnit) Graph() *GraphBuilder { return u.graph }

// Doc strips one raw comment's markers through the language's
// syntax and returns the clean lines: line prefixes dropped,
// block delimiters and gutters removed, directive lines — the
// //go:build kin — excluded, because a pragma is not
// documentation and would render double-commented downstream.
func (u *SourceUnit) Doc(raw string) []string {
	return u.DocLines(stripComment(raw, u.syntax))
}

// DocLines filters lines the author already holds clean: the
// directive-line rule applies, nothing else changes.
func (*SourceUnit) DocLines(lines []string) []string {
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

// ReadSum returns the fold of every read the unit accepted, in
// read order: the reads' half of the unit key. The driver folds
// the rest — partition reads, depth, versions, configuration —
// and the load report carries the finished key.
func (u *SourceUnit) ReadSum() []byte { return u.reads.Sum(nil) }

// stripComment removes one comment's markers: the first matching
// line prefix, or a block's delimiters and per-line gutter, one
// leading space tolerated after either.
func stripComment(raw string, syntax CommentSyntax) []string {
	text := raw
	for _, b := range syntax.Blocks {
		if strings.HasPrefix(text, b.Open) && strings.HasSuffix(text, b.Close) {
			body := strings.TrimSuffix(strings.TrimPrefix(text, b.Open), b.Close)
			lines := strings.Split(body, "\n")
			out := make([]string, 0, len(lines))
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if b.Gutter != "" {
					trimmed = strings.TrimPrefix(trimmed, b.Gutter)
					trimmed = strings.TrimPrefix(trimmed, " ")
				}
				out = append(out, trimmed)
			}
			return trimBlank(out)
		}
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, p := range syntax.Line {
			if rest, marked := strings.CutPrefix(trimmed, p); marked {
				trimmed = strings.TrimPrefix(rest, " ")
				break
			}
		}
		out = append(out, trimmed)
	}
	return trimBlank(out)
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
// rather than documentation: the go:build kin, spelled
// tool:name with no space before the colon.
func directiveLine(line string) bool {
	head, _, found := strings.Cut(line, ":")
	if !found || head == "" || strings.ContainsAny(head, " \t") {
		return false
	}
	for _, r := range head {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// GraphBuilder is the unit's write handle into the node model. A
// unit declares as many packages as its bytes do; two units
// contributing one package path merge at the splice, declarations
// appended in unit order. The splice validates what a unit built
// and panics on a structural defect — an emit-side symbol, a named
// kind without a name — because a malformed graph discovered at the
// resolution step points away from the frontend that built it.
type GraphBuilder struct {
	packages    map[string]*node.Package
	order       []string
	scopes      []ScopeRecord
	attachments []Attachment
}

// ScopeRecord pairs one parsed file with its import bindings, in
// the language's own form. The file is the node the unit built,
// because canonical identities do not exist until the splice
// assigns them, and a derivation spelled twice would drift; the
// kernel derives the identity there and hands the bindings back to
// that language's Resolve alone.
type ScopeRecord struct {
	File     *node.File
	Bindings any
}

// Attachment is one raw directive instance on a declaration this
// unit built. The subject is a pointer for the reason
// [ScopeRecord]'s file is: the splice resolves it to the assigned
// identity, so an attachment on a declaration another unit already
// declared attaches to the identity that stands.
type Attachment struct {
	Subject symbol.Symbol
	Raw     directive.Raw
}

// newGraphBuilder returns an empty builder; the unit owns it.
func newGraphBuilder() *GraphBuilder {
	return &GraphBuilder{packages: map[string]*node.Package{}}
}

// Package returns the package for one path, created on first
// touch: the container every file of that path joins.
func (gb *GraphBuilder) Package(path string) *node.Package {
	if p, held := gb.packages[path]; held {
		return p
	}
	p := &node.Package{
		ID:   symbol.Identity{Package: path},
		Path: strings.Split(path, "/"),
	}
	gb.packages[path] = p
	gb.order = append(gb.order, path)
	return p
}

// Scope records one file's import bindings: what the owning
// language's Resolve reads at the resolution phase. A nil file is a
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
