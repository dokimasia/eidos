// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Artifact is the record of one file that a plan generated: the file's
// manifest entry, the package it declares, the group that generates it,
// where a finding about the file is positioned, the export rows of its
// declarations and the file-level names it declares. The artifacts table
// keeps each artifact under its path. It keeps a second row under the
// path in lower case, through which [PhaseState.Clashes] finds the paths
// that one tree cannot contain beside a new path.
type Artifact struct {
	// Entry is the file's manifest entry, and its path is the key of the
	// artifact's row.
	Entry manifest.Entry
	// Pkg is the package the file declares, and the zero identity where
	// the target derives none.
	Pkg symbol.Identity
	// Group is the key unit of the file's group.
	Group plugin.UnitRef
	// At is where a finding about the file is positioned: the position of
	// the origin of the file's first declaration, or the file itself
	// where that origin has none. First names that declaration by its kind
	// and its name, and is empty for a file without a declaration.
	At    position.Pos
	First string
	// Export lists the export rows of the file's declarations, sorted, for
	// a plan that a dependent plan or a check reads, and none for any
	// other plan. Each row's File and Package are the artifact's path and
	// package.
	Export []plugin.ExportedSymbol
	// Names lists the file-level names the file declares, in the order of
	// its declarations. Each entry's File and FilePkg are the artifact's
	// path and package, and no entry is Ambiguous.
	Names []plugin.NameEntry
}

// Clash is one recorded path that one tree cannot contain beside a path
// that a run routes, with the plan that wrote it.
type Clash struct {
	Path string
	Plan string
}

// Artifact returns the record of the file at path, and false where the
// generation has none. The lookup allocates nothing for a path that fits
// in 256 bytes. The decoded record allocates its strings and its lists.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not read
// whole or does not decode.
func (s *PhaseState) Artifact(at string) (Artifact, bool, error) {
	var key [spellingCap]byte
	row, held, err := s.g.readers[TableArtifacts].get(s.ctx, append(key[:0], at...))
	if err != nil || !held {
		return Artifact{}, false, err
	}
	d := newDecoder(row, nil)
	a := decodeArtifact(d, at)
	if err := d.Err(); err != nil {
		return Artifact{}, false, fmt.Errorf("%w: the artifacts row of %s does not decode: %w", ErrDamaged, at, err)
	}
	return a, true, nil
}

// Recorded reports whether the generation records the file at path. It
// reads the artifact's row and decodes nothing, and the lookup allocates
// nothing for a path that fits in 256 bytes.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not read
// whole.
func (s *PhaseState) Recorded(at string) (bool, error) {
	var key [spellingCap]byte
	_, held, err := s.g.readers[TableArtifacts].get(s.ctx, append(key[:0], at...))
	return held, err
}

// Clashes returns the recorded paths that one tree cannot contain beside
// p, each with the plan that wrote it, sorted by path: p itself, a path
// that differs from p only in case, a path that p needs as a directory,
// and a path that needs p as a directory. These are the clashes of an
// output's staging, which compares paths in lower case. Clashes scans the
// rows under the path in lower case, under the lower case of each of its
// directories, and under the lower case of p as a directory.
//
// Error modes: an error wrapping [ErrDamaged] for a table that does not
// read whole, and for a row that does not decode.
func (s *PhaseState) Clashes(p string) ([]Clash, error) {
	lower := strings.ToLower(p)
	var key [spellingCap]byte
	out, err := s.clashesUnder(foldedPrefix(key[:0], lower, true), nil)
	if err == nil {
		out, err = s.clashesUnder(append(foldedPrefix(key[:0], lower, false), '/'), out)
	}
	for dir := path.Dir(lower); err == nil && dir != "." && dir != "/"; dir = path.Dir(dir) {
		out, err = s.clashesUnder(foldedPrefix(key[:0], dir, true), out)
	}
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b Clash) int { return cmp.Compare(a.Path, b.Path) })
	return out, nil
}

// clashesUnder appends to out the path and the plan of each folded row
// whose key begins with prefix.
//
// Error modes: an error wrapping [ErrDamaged] for a table that does not
// read whole, and for a row that does not decode.
func (s *PhaseState) clashesUnder(prefix []byte, out []Clash) ([]Clash, error) {
	rows, err := s.g.readers[TableArtifacts].scan(s.ctx, prefix)
	if err != nil {
		return out, err
	}
	for _, e := range rows {
		at, plan, decoded := foldedRow(e)
		if !decoded {
			return out, fmt.Errorf("%w: a folded artifacts row does not decode", ErrDamaged)
		}
		out = append(out, Clash{Path: at, Plan: plan})
	}
	return out, nil
}

// foldedPrefix appends the prefix of the folded rows of a path in lower
// case to dst: the separator that opens every folded key, and the path.
// whole closes the prefix with the separator, so it matches the rows of
// that path alone. Without it, the prefix also matches every longer path.
func foldedPrefix(dst []byte, lower string, whole bool) []byte {
	dst = append(append(dst, keySep), lower...)
	if whole {
		dst = append(dst, keySep)
	}
	return dst
}

// foldedKey appends the key of an artifact's folded row to dst: the
// separator, the path in lower case, the separator and the path. No path
// starts with the separator, so the folded rows sort before the rows keyed
// by path.
func foldedKey(dst []byte, p string) []byte {
	return append(foldedPrefix(dst, strings.ToLower(p), true), p...)
}

// foldedRow returns the path and the plan of a folded row, and reports
// false for a row whose key or plan does not decode.
func foldedRow(e entry) (string, string, bool) {
	rest, folded := bytes.CutPrefix(e.key, []byte{keySep})
	_, at, split := bytes.Cut(rest, []byte{keySep})
	d := wire.NewDecoder(e.row)
	plan := d.Bytes()
	if !folded || !split || d.Err() != nil || d.Len() != 0 {
		return "", "", false
	}
	return string(at), string(plan), true
}

// appendArtifact appends an artifact's row to dst. The row opens with the
// plan, so a commit decides from the plan alone which prior rows it keeps.
// The digest, the plugins and the sources of the manifest entry follow,
// then the package, the group, the position of its findings, the export
// rows and the names.
func appendArtifact(dst []byte, a *Artifact) []byte {
	e := encoder{buf: dst}
	e.text(a.Entry.Plan)
	e.text(a.Entry.Hash)
	e.uvarint(uint64(len(a.Entry.Plugins)))
	for _, p := range a.Entry.Plugins {
		e.text(string(p))
	}
	e.texts(a.Entry.Sources)
	e.identity(a.Pkg)
	e.buf = appendUnitRef(e.buf, a.Group)
	e.pos(a.At)
	e.text(a.First)
	e.uvarint(uint64(len(a.Export)))
	for i := range a.Export {
		x := &a.Export[i]
		e.identity(x.Origin)
		e.text(string(x.Plugin))
		e.text(x.Tag)
		e.text(x.Host)
		e.text(x.Name)
		e.uvarint(uint64(x.Kind))
		e.text(x.Spelling)
	}
	e.uvarint(uint64(len(a.Names)))
	for i := range a.Names {
		n := &a.Names[i]
		e.text(n.Package)
		e.text(n.Receiver)
		e.identity(n.Origin)
		e.uvarint(uint64(n.Kind))
		e.text(n.Emitted)
		e.text(n.Settled)
	}
	return e.buf
}

// decodeArtifact decodes the row of the artifact at path. Each export row
// and each name takes the artifact's path and package.
func decodeArtifact(d *decoder, at string) Artifact {
	a := Artifact{Entry: manifest.Entry{Path: at, Plan: d.text(), Hash: d.text()}}
	if n := d.Count(); n > 0 {
		a.Entry.Plugins = make([]diag.Origin, n)
		for i := range a.Entry.Plugins {
			a.Entry.Plugins[i] = diag.Origin(d.text())
		}
	}
	a.Entry.Sources = d.texts()
	a.Pkg = d.identity()
	a.Group = d.unitRef()
	a.At = d.pos()
	a.First = d.text()
	if n := d.Count(); n > 0 {
		a.Export = make([]plugin.ExportedSymbol, n)
		for i := range a.Export {
			key := plugin.ExportKey{Origin: d.identity(), Plugin: plugin.ID(d.text()), Tag: d.text()}
			key.Host, key.Name = d.text(), d.text()
			a.Export[i] = plugin.ExportedSymbol{
				ExportKey: key, Kind: symbol.Kind(d.Uvarint()), Spelling: d.text(), Package: a.Pkg, File: at,
			}
		}
	}
	if n := d.Count(); n > 0 {
		a.Names = make([]plugin.NameEntry, n)
		for i := range a.Names {
			a.Names[i] = plugin.NameEntry{
				Package: d.text(), Receiver: d.text(), Origin: d.identity(), Kind: symbol.Kind(d.Uvarint()),
				Emitted: d.text(), Settled: d.text(), File: at, FilePkg: a.Pkg,
			}
		}
	}
	return a
}
