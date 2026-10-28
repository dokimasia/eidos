// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"io/fs"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
)

// lineFamily is the family of a file of a line-set family: the generator
// that declares the family, its tag, and the [plugin.LineSetReader] of the
// generator. The zero value, with a nil reader, marks a file whose removed
// lines the run does not read.
type lineFamily struct {
	plugin plugin.ID
	tag    string
	reader plugin.LineSetReader
}

// lineFamilyOf returns the family of a routed data file whose family is a
// line set, for a generator of the plan that implements
// [plugin.LineSetReader]. It returns the zero family for every other file.
// It allocates nothing.
func lineFamilyOf(pl *compiledPlan, f *plugin.File) lineFamily {
	if !f.Data || len(f.Units) == 0 {
		return lineFamily{}
	}
	first := f.Units[0]
	for _, o := range pl.outputs[first.Plugin] {
		if o.Tag != first.Tag || !o.LineSet {
			continue
		}
		for _, e := range pl.entries {
			if r, reads := e.run.(plugin.LineSetReader); reads && e.name == first.Plugin {
				return lineFamily{plugin: first.Plugin, tag: first.Tag, reader: r}
			}
		}
	}
	return lineFamily{}
}

// removedLines returns each line of committed that fresh does not
// contain, in the order of committed, with its number from 1 and without
// its line ending. The order and the repetition of the lines of either
// file do not matter. A final line ending ends the last line and starts
// no empty one.
func removedLines(committed, fresh []byte) []plugin.RemovedLine {
	kept := map[string]struct{}{}
	for line := range bytes.Lines(fresh) {
		kept[string(bytes.TrimRight(line, "\r\n"))] = struct{}{}
	}
	var removed []plugin.RemovedLine
	n := 0
	for line := range bytes.Lines(committed) {
		n++
		text := bytes.TrimRight(line, "\r\n")
		if _, held := kept[string(text)]; !held {
			removed = append(removed, plugin.RemovedLine{Line: n, Text: string(text)})
		}
	}
	return removed
}

// removals reports the finding of each line that the committed file of a
// line-set file of a plan has and its fresh bytes lack, through the
// [plugin.LineSetReader] of the generator that declares the family. It
// reads the committed file from tree, and reports nothing for a nil tree,
// a plan without such a file and a file that tree does not contain. A
// finding without a position is at the line, and a finding without an
// origin has the origin of the generator.
func removals(p *planRun, tree fs.FS, sink *diag.Sink) {
	if tree == nil {
		return
	}
	for i := range p.files {
		f := &p.files[i]
		if f.lines.reader == nil {
			continue
		}
		committed, err := fs.ReadFile(tree, f.path)
		if err != nil {
			continue
		}
		for _, r := range removedLines(committed, f.body) {
			r.Family, r.Path = f.lines.tag, f.path
			d, report := f.lines.reader.Removed(r)
			if !report {
				continue
			}
			if d.Pos.IsZero() {
				d.Pos = position.Pos{File: f.path, Line: r.Line}
			}
			if d.Origin == "" {
				d.Origin = f.lines.plugin
			}
			sink.Report(d)
		}
	}
}
