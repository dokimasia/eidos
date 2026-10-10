// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package matrix

import (
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The document's title, and the sections of the read side, the render
// side and the sugar.
const (
	title       = "# Support matrix\n"
	readSection = "\n## Read side\n\nEach cell contains the verdict of a language's conformance corpus on " +
		"one feature of the inventory.\n\n"
	renderSection = "\n## Render side\n\nEach cell contains the verdict of a target's backend on one fact " +
		"of the model or on one kind of declaration. Where the verdict on a fact differs for some kinds, " +
		"the cell lists those kinds after the base verdict.\n\n"
	sugarSection = "\n## Sugar\n\nEach row contains the verdict of a language on a marker in its own syntax " +
		"for metadata, and the marker of the brand acme.\n\n"
)

// The headers of the tables' first columns, and of the sugar's two
// columns of values.
const (
	featureHeader  = "Feature"
	factHeader     = "Fact or kind"
	languageHeader = "Language"
	verdictHeader  = "Verdict"
	markerHeader   = "Marker"
)

// These constants build the cells of the tables. noneCell is the cell of
// a value that an entry does not have. codeOpen and codeClose quote code.
// A pipe ends a cell, so a pipe inside a cell takes the escape
// escapedPipe. rendersWord and refusesWord are the words of a kind that a
// backend renders or refuses. verdictSep, exceptOn and kindSep separate
// the verdicts that differ for some kinds.
const (
	noneCell    = "none"
	codeOpen    = "`"
	codeClose   = "`"
	pipe        = "|"
	escapedPipe = `\|`
	rendersWord = "renders"
	refusesWord = "refuses"
	verdictSep  = "; "
	exceptOn    = " on "
	kindSep     = ", "
)

// These constants build the rows of a table. rowOpen starts a row,
// cellSep separates two cells, and rowClose ends a row. ruleOpen and
// ruleCell build the rule below the header, and lineBreak ends the rule.
const (
	rowOpen   = "| "
	cellSep   = " | "
	rowClose  = " |\n"
	ruleOpen  = "|"
	ruleCell  = "---|"
	lineBreak = '\n'
)

// sugarFeature is the inventory feature of a marker in a language's own
// syntax for metadata.
const sugarFeature = "directive_sugar"

// verdictWords are the words of a backend's verdicts on a fact.
var verdictWords = map[render.Verdict]string{
	render.VerdictUndeclared: "undeclared",
	render.Renders:           rendersWord,
	render.Holds:             "holds",
	render.Refuses:           refusesWord,
}

// renderKinds are the kinds of declaration that a file can declare, in
// the order of the model's kinds.
var renderKinds = []symbol.Kind{
	symbol.KindFunction, symbol.KindMethod, symbol.KindEnum, symbol.KindSum, symbol.KindVariable,
	symbol.KindConstant, symbol.KindStruct, symbol.KindInterface, symbol.KindAlias,
}

// Markdown renders the support matrix of entries as a Markdown document
// of four tables:
//
//   - The read side has a row for each feature of the conformance
//     inventory and a column for each language. A cell contains the
//     verdict of the language's corpus.
//   - The render side has a row for each fact of the model and for each
//     kind of declaration, and a column for each target. A cell contains
//     the verdict of the target's backend. Where the verdict on a fact
//     differs for some kinds, the cell lists them after it.
//   - The hub has a row for each probe of a canonical form and for each
//     choice of a lowering policy, and a column for each target. A cell
//     contains the spelling of the target's spoke.
//   - The sugar has a row for each language. A row contains the verdict
//     of the language's corpus on a marker in its own syntax, and the
//     marker of the brand acme.
//
// A language without a backend has no column on the render side and in
// the hub. A cell of a value that an entry does not have contains none.
// A backend without a coverage has the verdict undeclared on every fact.
//
// Error modes: the error of a backend whose policies do not resolve, and
// an error for a policy without a probe of its shape.
func Markdown(entries []Entry) ([]byte, error) {
	var b strings.Builder
	b.WriteString(title)
	readSide(&b, entries)
	renderSide(&b, entries)
	if err := hub(&b, entries); err != nil {
		return nil, err
	}
	sugar(&b, entries)
	return []byte(b.String()), nil
}

// readSide writes the section of the read side.
func readSide(b *strings.Builder, entries []Entry) {
	header := []string{featureHeader}
	for _, e := range entries {
		header = append(header, string(e.Corpus.Frontend.Lang()))
	}
	inventory := conformance.Inventory()
	rows := make([][]string, 0, len(inventory))
	for _, f := range inventory {
		row := []string{f.ID}
		for _, e := range entries {
			row = append(row, e.Corpus.Coverage[f.ID].String())
		}
		rows = append(rows, row)
	}
	b.WriteString(readSection)
	table(b, header, rows)
}

// renderSide writes the section of the render side. A fact's cell is the
// cell that factCell returns. A kind's cell contains refuses for a kind
// that the backend refuses, and renders otherwise.
func renderSide(b *strings.Builder, entries []Entry) {
	backed := withBackends(entries)
	coverages := make([]render.Coverage, len(backed))
	refused := make([]map[symbol.Kind]string, len(backed))
	for i, e := range backed {
		if c, covers := e.Backend.(render.Coverer); covers {
			coverages[i] = c.Coverage()
		}
		if r, refuses := e.Backend.(render.Refuser); refuses {
			refused[i] = r.RefusedKinds()
		}
	}
	facts := symbol.Facts()
	rows := make([][]string, 0, len(facts)+len(renderKinds))
	for _, f := range facts {
		row := []string{f.String()}
		for _, c := range coverages {
			row = append(row, factCell(c, f))
		}
		rows = append(rows, row)
	}
	for _, k := range renderKinds {
		row := []string{k.String()}
		for _, r := range refused {
			cell := rendersWord
			if _, refuses := r[k]; refuses {
				cell = refusesWord
			}
			row = append(row, cell)
		}
		rows = append(rows, row)
	}
	b.WriteString(renderSection)
	table(b, append([]string{factHeader}, targetsOf(backed)...), rows)
}

// factCell returns the cell of the fact f in a backend's coverage c. The
// cell opens with the word of the base verdict. Each verdict that differs
// from it for some kinds follows, in the order of the verdicts, with
// those kinds in the order of the model's kinds, as in renders; refuses
// on Param, Field. An exception that repeats the base verdict is left
// out.
func factCell(c render.Coverage, f symbol.Fact) string {
	base := c.Facts[f]
	kindsOf := map[render.Verdict][]string{}
	for _, k := range slices.Sorted(maps.Keys(c.Except)) {
		if v, excepted := c.Except[k][f]; excepted && v != base {
			kindsOf[v] = append(kindsOf[v], k.String())
		}
	}
	verdicts := []string{verdictWords[base]}
	for _, v := range slices.Sorted(maps.Keys(kindsOf)) {
		verdicts = append(verdicts, verdictWords[v]+exceptOn+strings.Join(kindsOf[v], kindSep))
	}
	return strings.Join(verdicts, verdictSep)
}

// sugar writes the section of the sugar.
func sugar(b *strings.Builder, entries []Entry) {
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		marker := noneCell
		if e.Marker != "" {
			marker = codeOpen + e.Marker + codeClose
		}
		rows = append(rows, []string{
			string(e.Corpus.Frontend.Lang()), e.Corpus.Coverage[sugarFeature].String(), marker,
		})
	}
	b.WriteString(sugarSection)
	table(b, []string{languageHeader, verdictHeader, markerHeader}, rows)
}

// table writes one Markdown table: the header, the rule below it, and
// the rows.
func table(b *strings.Builder, header []string, rows [][]string) {
	writeRow(b, header)
	b.WriteString(ruleOpen)
	for range header {
		b.WriteString(ruleCell)
	}
	b.WriteByte(lineBreak)
	for _, row := range rows {
		writeRow(b, row)
	}
}

// writeRow writes one row of a table. A pipe ends a cell, so writeRow
// escapes each pipe inside a cell.
func writeRow(b *strings.Builder, cells []string) {
	b.WriteString(rowOpen)
	for i, cell := range cells {
		if i > 0 {
			b.WriteString(cellSep)
		}
		b.WriteString(strings.ReplaceAll(cell, pipe, escapedPipe))
	}
	b.WriteString(rowClose)
}

// withBackends returns the entries with a backend, in their order.
func withBackends(entries []Entry) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Backend != nil {
			out = append(out, e)
		}
	}
	return out
}

// targetsOf returns the targets of entries with a backend, in their
// order.
func targetsOf(backed []Entry) []string {
	out := make([]string, 0, len(backed))
	for _, e := range backed {
		out = append(out, string(e.Backend.Target()))
	}
	return out
}
