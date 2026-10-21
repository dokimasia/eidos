// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"cmp"
	"io/fs"
	"iter"
	"strconv"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// unitCode is the code of the findings of the source unit fixtures.
var unitCode = diag.Code{Prefix: "tst", Number: 5}

// frontendOrigin is the origin under which the fixture unit reports.
const frontendOrigin = diag.Origin("golang")

// The brand of the carriers of the fixture unit, and its three marks as an
// author writes them.
const (
	unitBrand   = "fixture"
	bareMark    = "fixture:"
	setMark     = "+fixture:"
	negatedMark = "-fixture:"
)

// The payloads of the attachment cases. The first is a payload that the
// kernel grammar parses, and the second is a payload that it refuses. The
// block also has the directive name of the first payload and the line of
// both.
const (
	tablePayload  = "table name=rows"
	brokenPayload = "=rows"
	tableName     = directive.Name("table")
	carrierLine   = 7
)

// stubName is the directive name of the markers in the sugar cases.
// sugarRefusal is the reason that a frontend gives for a marker whose
// arguments it could not lift.
const (
	stubName     = directive.Name("stub")
	sugarRefusal = "the argument f() is a call, and a directive value is a literal"
)

// carrierAt is the position of the carriers of the attachment cases.
var carrierAt = position.Pos{File: "svc/store/row.go", Line: carrierLine}

// The uses of the use cases: two declarations of a package of a
// dependency, which a reference of the unit can resolve to.
var (
	usedRow = symbol.Identity{Lang: "golang", Package: "example.test/lib", Name: "Row"}
	usedCol = symbol.Identity{Lang: "golang", Package: "example.test/lib", Name: "Col"}
)

// The subject of the refusal cases, which the model cannot address, and
// the separator of the path of a marker in their findings.
const (
	unaddressed = "a parameter"
	markerSep   = "."
)

// The sentence of each error in the syntax report cases, and the number
// of errors past the cap.
const (
	syntaxMessage  = "an unexpected token"
	syntaxOverflow = 5
)

// The package of the module cases, the module that they stamp and the
// directory of its manifest, and a second module in another directory.
const (
	modulePackage = "svc/store"
	moduleName    = "example.test/svc"
	moduleRoot    = "."
	otherModule   = "example.test/tool"
	otherRoot     = "tool"
)

// The continued carrier of the refusal cases. The block has the payload of
// its first line, its continuation, and the directive that a formatter
// moves under it. It also has a brand with a hyphen, so the carriers of
// that brand do not have the tool-directive shape.
const (
	continuedPayload = "out user.go \\"
	continuationLine = "plugin=buildergen"
	indexPayload     = "index fields=[b]"
	hyphenBrand      = "fix-ture"
)

// continuedAt is the line at which the comments of the refusal cases open.
var continuedAt = position.Pos{File: "svc/store/row.go", Line: 4}

// The comments that the allocation checks and the benchmarks split. The
// first has three lines of documentation, and the second has documentation
// above a carrier.
const (
	docComment     = "// Row is one row.\n// It has fields.\n// It has a key."
	carrierComment = "// Row is one row.\n//fixture:table name=rows"
)

// The allocation counts of the methods of the source unit.
// TestSourceUnitAllocs checks them in the ordinary run, and
// BenchmarkSourceUnit checks them in a benchmark run.
const (
	// newSourceUnitAllocs is the count for a unit of one file with one
	// shared input. NewSourceUnit allocates the unit, its set of readable
	// paths with the storage of the set, and the graph builder with its map
	// of packages.
	newSourceUnitAllocs = 5
	// commentAllocs is the count for a comment of documentation lines.
	// Comment allocates the list of its lines and the list of its
	// documentation.
	commentAllocs = 2
	// carrierCommentAllocs is the count for a comment with a carrier.
	// Comment allocates what commentAllocs counts and the list of carriers.
	carrierCommentAllocs = commentAllocs + 1
	// docAllocs is the count for the documentation of a comment. Doc
	// allocates the list of its lines and the list of its documentation.
	docAllocs = 2
	// docLinesAllocs is the count of DocLines. DocLines allocates the
	// filtered list of clean lines.
	docLinesAllocs = 1
	// reportAllocs is the count for one finding. The report allocates the
	// formatted message.
	reportAllocs = 1
	// attachCarriersAllocs is the count for the first carrier that a unit
	// attaches. AttachCarriers allocates the parameters of the parsed
	// instance and the list of attachments.
	attachCarriersAllocs = 2
	// attachSugarAllocs is the count for the first marker of the brand and
	// a name that a unit attaches. AttachSugar allocates the list of
	// attachments.
	attachSugarAllocs = 1
	// attachPrefixedSugarAllocs is the count for the first marker of the
	// brand, a plugin and a name that a unit attaches. AttachSugar
	// allocates the prefixed name and the list of attachments.
	attachPrefixedSugarAllocs = 2
	// refuseCarrierAllocs is the count for one refused carrier.
	// RefuseCarriers allocates the quoted text, the boxing of the text and
	// of the subject, and the message.
	refuseCarrierAllocs = 4
	// refuseSugarAllocs is the count for one refused marker. RefuseSugar
	// allocates the joined path, the boxing of the path and of the subject,
	// and the message.
	refuseSugarAllocs = 4
	// reportSyntaxAllocs is the count for one syntax error. ReportSyntax
	// allocates the four objects of the loop state that the range over the
	// iterator moves to the heap, the boxing of the sentence and the
	// message.
	reportSyntaxAllocs = 6
	// stampModuleAllocs is the count for the first module of a unit.
	// StampModule allocates the boxing of the module and of its root, and
	// two growths of the list of stamps.
	stampModuleAllocs = 4
	// stampModuleAgainAllocs is the count for a second package of the same
	// module. StampModule allocates one growth of the list of stamps,
	// because the call reuses the boxing of the module and of its root.
	stampModuleAgainAllocs = 1
	// firstPackageAllocs is the count for the first package of a unit.
	// Package allocates the segments of the path, the package, the first
	// group of the map and the list of paths.
	firstPackageAllocs = 4
	// packagesAllocs is the count of Packages. Packages allocates the list
	// of packages.
	packagesAllocs = 1
	// firstRecordAllocs is the count for the first record of a scope, an
	// attachment or a stamp in a unit. The record allocates the list of
	// records.
	firstRecordAllocs = 1
	// firstUseAllocs is the count for the first use of a unit. Use
	// allocates the set of uses and its first group.
	firstUseAllocs = 2
)

// The fixtures that the allocation checks and the benchmarks read and
// record. They are the position at which a comment opens, and the subject,
// the file, the instance and the stamp that a builder records.
var (
	commentAt     = position.Pos{File: "svc/store/row.go", Line: 1}
	recordSubject = &node.Struct{Name: "Row"}
	recordFile    = &node.File{Path: "svc/store/row.go"}
	recordPackage = &node.Package{Files: []*node.File{recordFile}}
	secondPackage = &node.Package{Pos: position.Pos{File: "svc/store/col.go", Line: 1}}
	recordRaw     = directive.Raw{Name: tableName}
	recordStamp   = meta.RawStamp{Key: "fake.testFile", Value: true}
)

// The carrier, the marker and the syntax error that the allocation checks
// and the benchmarks refuse and report.
var (
	refusedCarriers = []plugin.Carrier{{Mark: setMark, Payload: tablePayload, Pos: carrierAt}}
	refusedSugar    = []plugin.Sugar{{Path: []string{unitBrand, string(stubName)}, Pos: carrierAt}}
	oneSyntaxError  = syntaxErrors(1)
)

// unitReport is one reporting method of the unit. It has the name of the
// method, a call that reports one finding through it, and the severity of
// the finding.
type unitReport struct {
	method string
	report func(*plugin.SourceUnit)
	want   diag.Severity
}

// unitRecord is one method that records into a builder. It has the name of
// the method and a call of it.
type unitRecord struct {
	method string
	record func(*plugin.GraphBuilder)
}

// unitRefusal is one call that refuses or reports through a unit. It has
// the name of its case, the call, and the allocation count of the call.
type unitRefusal struct {
	name   string
	refuse func(*plugin.SourceUnit)
	allocs uint64
}

// TestSourceUnit checks the source unit. A frontend reads every byte
// through the unit, so the cases pin the jail of Read, the fold of
// continued carriers and the comment pipeline.
func TestSourceUnit(t *testing.T) {
	t.Parallel()

	t.Run("NewSourceUnit", func(t *testing.T) {
		t.Parallel()

		t.Run("panics on an empty brand", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				plugin.NewSourceUnit([]plugin.SourceRef{unitFile()}, unitTree(), plugin.DepthFull,
					goSyntax(), "", diag.NewSink(), frontendOrigin)
			}, "a unit without a brand is a driver defect")
		})
	})

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the bytes of a member file", func(t *testing.T) {
			t.Parallel()

			b, err := unitOf(t, unitTree()).Read("svc/store/row.go")
			assert.NoError(t, err, "the member reads")
			assert.Equal(t, string(b), "package store\n", "the bytes are the file's")
		})

		t.Run("returns the bytes of a declared shared input", func(t *testing.T) {
			t.Parallel()

			b, err := unitOf(t, unitTree()).Read("go.mod")
			assert.NoError(t, err, "the shared input reads")
			assert.Equal(t, string(b), "module svc\n", "the bytes are the file's")
		})

		t.Run("returns an error for a path outside the unit", func(t *testing.T) {
			t.Parallel()

			_, err := unitOf(t, unitTree()).Read("svc/other/x.go")
			assert.HasError(t, err, "the read fails")
			assert.Contains(t, err.Error(), "svc/other/x.go", "the error names the path")
		})

		t.Run("returns an error for a declared file that the tree does not contain", func(t *testing.T) {
			t.Parallel()

			u := unitOver([]plugin.SourceRef{{Path: "svc/store/absent.go"}}, plugin.DepthFull, goSyntax())
			_, err := u.Read("svc/store/absent.go")
			assert.HasError(t, err, "the read fails")
			assert.Contains(t, err.Error(), "svc/store/absent.go", "the error names the path")
		})

		t.Run("returns the bytes of a qualified member from its store", func(t *testing.T) {
			t.Parallel()

			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: qualifiedFile}}, withCache(), plugin.DepthSignatures,
				goSyntax(), unitBrand, diag.NewSink(), frontendOrigin)
			b, err := u.Read(qualifiedFile)
			assert.NoError(t, err, "the dependency member reads")
			assert.Equal(t, string(b), "package lib\n", "the bytes are the store's")
		})

		t.Run("returns ErrStoreAbsent for a qualified member over a tree without stores", func(t *testing.T) {
			t.Parallel()

			u := unitOver([]plugin.SourceRef{{Path: qualifiedFile}}, plugin.DepthSignatures, goSyntax())
			_, err := u.Read(qualifiedFile)
			assert.ErrorIs(t, err, plugin.ErrStoreAbsent, "the unit's tree provides no store")
		})
	})

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the members in partition order", func(t *testing.T) {
			t.Parallel()

			members := []plugin.SourceRef{
				{Path: "svc/store/row.go", Shared: []string{"go.mod"}},
				{Path: "svc/store/col.go"},
			}
			assert.Equal(t, unitOver(members, plugin.DepthFull, goSyntax()).Files(), members,
				"the members are in partition order")
		})
	})

	t.Run("Depth", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Depth
		}{
			{name: "returns DepthSignatures for a signature-only unit", give: plugin.DepthSignatures},
			{name: "returns DepthFull for a full unit", give: plugin.DepthFull},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				u := unitOver([]plugin.SourceRef{unitFile()}, tt.give, goSyntax())
				assert.Equal(t, u.Depth(), tt.give, "the depth is the constructed one")
			})
		}
	})

	t.Run("Brand", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the brand of the unit", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, unitOf(t, unitTree()).Brand(), unitBrand, "the brand is the constructed one")
		})
	})

	t.Run("Use", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the language, the package and the name of a use", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			u.Use(symbol.Identity{
				Lang: usedRow.Lang, Package: usedRow.Package, Owner: "Table", Name: usedRow.Name,
				Kind: symbol.KindStruct, Disc: "(int)",
			})
			assert.Equal(t, u.Uses(), []symbol.Identity{usedRow}, "a use is a bare identity")
		})
	})

	t.Run("Uses", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the uses sorted without repeats", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			u.Use(usedRow)
			u.Use(usedCol)
			u.Use(usedRow)
			assert.Equal(t, u.Uses(), []symbol.Identity{usedCol, usedRow}, "each use once, in identity order")
		})

		t.Run("returns no use for a unit that records none", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, unitOf(t, unitTree()).Uses(), "the unit records no use")
		})
	})

	t.Run("Comment", func(t *testing.T) {
		t.Parallel()

		marks := []struct {
			name    string
			line    string
			negated bool
		}{
			{name: "returns a carrier opened by the bare brand mark", line: "// " + bareMark + tablePayload},
			{name: "returns a carrier opened by the set mark", line: "// " + setMark + tablePayload},
			{
				name: "returns a negated carrier opened by the negated mark",
				line: "// " + negatedMark + tablePayload, negated: true,
			},
		}
		for _, tt := range marks {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				parts := comment(t, tt.line)
				assert.Length(t, parts.Carriers, 1, "one carrier is returned")
				assert.Equal(t, parts.Carriers[0].Payload, tablePayload, "the payload follows the mark")
				assert.Equal(t, parts.Carriers[0].Negated(), tt.negated, "the polarity is the mark's")
				assert.Empty(t, parts.Docs, "no documentation is returned")
			})
		}

		t.Run("returns documentation for the mark of another brand", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "// +gen:table name=rows")
			assert.Empty(t, parts.Carriers, "no carrier is returned")
			assert.Equal(t, parts.Docs, []string{"+gen:table name=rows"}, "the line is documentation")
		})

		t.Run("returns documentation for a mark without a letter after it", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "// Options:\n// + item one\n// +1 point\n"+
				"// "+setMark+" spaced\n// "+setMark+tablePayload)
			assert.Length(t, parts.Carriers, 1, "only the lettered carrier is returned")
			assert.Equal(t, parts.Docs, []string{
				"Options:", "+ item one", "+1 point", setMark + " spaced",
			}, "every other line is documentation")
		})

		t.Run("returns the carriers of a block comment at their own lines", func(t *testing.T) {
			t.Parallel()

			parts := unitOf(t, unitTree()).Comment(
				"/**\n * Row is one record.\n * "+setMark+tablePayload+"\n * go:embed schema.sql\n */",
				position.Pos{File: "svc/store/row.go", Line: 3},
			)
			assert.Equal(t, parts.Docs, []string{"Row is one record.", "go:embed schema.sql"},
				"a block comment has no directives")
			assert.Length(t, parts.Carriers, 1, "the carrier is returned")
			assert.Equal(t, parts.Carriers[0].Pos.Line, 5, "the carrier is at its own line")
		})

		t.Run("returns no annotations for a block comment", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "/**\n * go:embed schema.sql\n */")
			assert.Empty(t, parts.Annotations, "nothing lowers off a block")
		})

		t.Run("returns annotations for directives with an adjacent marker", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "//go:embed schema.sql\n//export CFunc")
			assert.Equal(t, parts.Annotations, symbol.Annotations{
				{Name: "go:embed", Args: []string{"schema.sql"}},
				{Name: "export", Args: []string{"CFunc"}},
			}, "the colon form and the legacy space form lower as annotations")
		})

		t.Run("returns documentation for directives after a spaced marker", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "// go:generate reruns this\n// export is prose here")
			assert.Equal(t, parts.Docs, []string{
				"go:generate reruns this", "export is prose here",
			}, "the lines are documentation")
			assert.Empty(t, parts.Annotations, "no annotation is returned")
		})

		t.Run("folds a continued carrier into one payload at its opening line", func(t *testing.T) {
			t.Parallel()

			parts := unitOf(t, unitTree()).Comment(
				"// "+setMark+"out user.go \\\n// plugin=buildergen \\\n// pkg=usersx\n// Trailing doc.",
				position.Pos{File: "svc/store/row.go", Line: 4},
			)
			assert.Length(t, parts.Carriers, 1, "one carrier is returned")
			assert.Equal(t, parts.Carriers[0].Payload, "out user.go plugin=buildergen pkg=usersx",
				"the lines join with single spaces")
			assert.Equal(t, parts.Carriers[0].Pos.Line, 4, "the carrier is at its opening line")
			assert.Equal(t, parts.Docs, []string{"Trailing doc."}, "the continuation lines are consumed")
		})

		t.Run("folds a continued carrier after a spaced bare mark", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "// "+bareMark+continuedPayload+"\n// "+continuationLine)
			assert.Length(t, parts.Carriers, 1, "one carrier is returned")
			assert.Equal(t, parts.Carriers[0].Payload, "out user.go "+continuationLine,
				"a spaced marker keeps the line out of the tool-directive shape")
		})

		t.Run("folds a continued carrier after an adjacent bare mark without the directive convention",
			func(t *testing.T) {
				t.Parallel()

				u := unitOver([]plugin.SourceRef{unitFile()}, plugin.DepthFull,
					plugin.CommentSyntax{Line: []string{"//"}})
				parts := u.Comment("//"+bareMark+continuedPayload+"\n// "+continuationLine, continuedAt)
				assert.Length(t, parts.Carriers, 1, "one carrier is returned")
				assert.Equal(t, parts.Carriers[0].Payload, "out user.go "+continuationLine,
					"the syntax declares no tool directives")
			})

		t.Run("reports ContinuedCarrier for a continued carrier in the tool-directive shape", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.Comment("//"+bareMark+continuedPayload+"\n// "+continuationLine, continuedAt)
			coretest.AssertCodes(t, sink, plugin.ContinuedCarrier)
		})

		t.Run("reports the refusal at the line of the carrier", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.Comment("// Row is one record.\n//"+bareMark+continuedPayload+"\n// "+continuationLine, continuedAt)
			got := reported(sink)
			assert.Length(t, got, 1, "one finding is reported")
			assert.Equal(t, got[0].Pos.Line, continuedAt.Line+1, "the finding is at the carrier's line")
		})

		t.Run("quotes the carrier with the set mark in the refusal", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.Comment("//"+bareMark+continuedPayload+"\n// "+continuationLine, continuedAt)
			got := reported(sink)
			assert.Length(t, got, 1, "one finding is reported")
			assert.Contains(t, got[0].Msg, strconv.Quote(setMark+continuedPayload),
				"the refusal states the form that keeps its place")
		})

		t.Run("returns no carrier for a refused continued carrier", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "//"+bareMark+continuedPayload+"\n// "+continuationLine)
			assert.Empty(t, parts.Carriers, "the refused carrier is not returned")
		})

		t.Run("returns the line after a refused carrier as documentation", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "//"+bareMark+continuedPayload+"\n// "+continuationLine)
			assert.Equal(t, parts.Docs, []string{continuationLine}, "the continuation is read as written")
		})

		t.Run("returns the carrier that a formatter moved under a refused one", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "//"+bareMark+continuedPayload+"\n//"+bareMark+indexPayload)
			assert.Length(t, parts.Carriers, 1, "the moved carrier is its own")
			assert.Equal(t, parts.Carriers[0].Payload, indexPayload, "the refused carrier folds nothing into it")
		})

		shapes := []struct {
			name   string
			line   string
			syntax plugin.CommentSyntax
			want   bool
		}{
			{
				name: "marks a carrier after an adjacent bare mark directive-shaped",
				line: "//" + bareMark + tablePayload, syntax: goSyntax(), want: true,
			},
			{
				name: "leaves a carrier after a spaced bare mark unmarked",
				line: "// " + bareMark + tablePayload, syntax: goSyntax(),
			},
			{
				name: "leaves a carrier after an adjacent set mark unmarked",
				line: "//" + setMark + tablePayload, syntax: goSyntax(),
			},
			{
				name: "leaves a carrier after an adjacent negated mark unmarked",
				line: "//" + negatedMark + tablePayload, syntax: goSyntax(),
			},
			{
				name: "leaves a carrier unmarked under a syntax without the directive convention",
				line: "//" + bareMark + tablePayload, syntax: plugin.CommentSyntax{Line: []string{"//"}},
			},
		}
		for _, tt := range shapes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				u := unitOver([]plugin.SourceRef{unitFile()}, plugin.DepthFull, tt.syntax)
				parts := u.Comment(tt.line, continuedAt)
				assert.Length(t, parts.Carriers, 1, "one carrier is returned")
				assert.Equal(t, parts.Carriers[0].DirectiveShaped, tt.want, "the shape is the line's")
			})
		}

		t.Run("leaves a carrier under a hyphenated brand unmarked", func(t *testing.T) {
			t.Parallel()

			u := plugin.NewSourceUnit([]plugin.SourceRef{unitFile()}, unitTree(), plugin.DepthFull, goSyntax(),
				hyphenBrand, diag.NewSink(), frontendOrigin)
			parts := u.Comment("//"+hyphenBrand+":"+tablePayload, continuedAt)
			assert.Length(t, parts.Carriers, 1, "one carrier is returned")
			assert.False(t, parts.Carriers[0].DirectiveShaped, "a hyphen fails the tool-directive shape")
		})

		t.Run("returns no annotations for a syntax without the directive convention", func(t *testing.T) {
			t.Parallel()

			u := unitOver([]plugin.SourceRef{{Path: "svc/store/row.go"}}, plugin.DepthFull,
				plugin.CommentSyntax{Line: []string{"//"}})
			parts := u.Comment("// Row is one record.\n//go:embed schema.sql",
				position.Pos{File: "svc/store/row.go", Line: 1})
			assert.Equal(t, parts.Docs, []string{"Row is one record.", "go:embed schema.sql"},
				"the directive line is documentation")
			assert.Empty(t, parts.Annotations, "no annotation is returned")
		})

		t.Run("keeps a bare URL in the documentation", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "// Row is one record.\n// https://example.test/spec\n// note: keyed by name.")
			assert.Equal(t, parts.Docs, []string{
				"Row is one record.", "https://example.test/spec", "note: keyed by name.",
			}, "a URL's slash and a prose colon's space both fail the directive rule")
			assert.Empty(t, parts.Annotations, "no annotation is returned")
		})

		t.Run("keeps prose whose head is not a tool name", func(t *testing.T) {
			t.Parallel()

			parts := comment(t, "// Row is one record.\n// Note:keyed by name.\n// go_embed:schema.sql")
			assert.Equal(t, parts.Docs, []string{
				"Row is one record.", "Note:keyed by name.", "go_embed:schema.sql",
			}, "a tool name is lowercase alphanumeric throughout")
			assert.Empty(t, parts.Annotations, "no annotation is returned")
		})

		t.Run("returns the documentation after the longest line marker", func(t *testing.T) {
			t.Parallel()

			u := unitOver([]plugin.SourceRef{unitFile()}, plugin.DepthFull, nestedSyntax())
			parts := u.Comment("/// Row is one record.\n//! The module keeps rows.", continuedAt)
			assert.Equal(t, parts.Docs, []string{"Row is one record.", "The module keeps rows."},
				"no part of a longer marker is left in the text")
		})

		t.Run("returns a carrier after the longest line marker", func(t *testing.T) {
			t.Parallel()

			u := unitOver([]plugin.SourceRef{unitFile()}, plugin.DepthFull, nestedSyntax())
			parts := u.Comment("/// "+setMark+tablePayload, continuedAt)
			assert.Length(t, parts.Carriers, 1, "the carrier is returned")
			assert.Equal(t, parts.Carriers[0].Payload, tablePayload, "the payload follows the mark")
		})

		t.Run("strips the delimiters of the longest enclosing block", func(t *testing.T) {
			t.Parallel()

			u := unitOver([]plugin.SourceRef{unitFile()}, plugin.DepthFull, nestedSyntax())
			parts := u.Comment("/**\n * Row is one record.\n */", continuedAt)
			assert.Equal(t, parts.Docs, []string{"Row is one record."}, "the doc block's gutter is removed")
		})
	})

	t.Run("Negated", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			mark string
			want bool
		}{
			{name: "reports true for the negated mark", mark: negatedMark, want: true},
			{name: "reports false for the set mark", mark: setMark, want: false},
			{name: "reports false for the bare brand mark", mark: bareMark, want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c := plugin.Carrier{Mark: tt.mark, Payload: tablePayload}
				assert.Equal(t, c.Negated(), tt.want, "the polarity is the mark's")
			})
		}
	})

	t.Run("CutCarrier", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			line        string
			wantMark    string
			wantPayload string
			wantOK      bool
		}{
			{
				name: "cuts a line at the bare brand mark", line: bareMark + tablePayload,
				wantMark: bareMark, wantPayload: tablePayload, wantOK: true,
			},
			{
				name: "cuts a line at the set mark", line: setMark + tablePayload,
				wantMark: setMark, wantPayload: tablePayload, wantOK: true,
			},
			{
				name: "cuts a line at the negated mark", line: negatedMark + tablePayload,
				wantMark: negatedMark, wantPayload: tablePayload, wantOK: true,
			},
			{name: "reports false for the mark of another brand", line: "+gen:" + tablePayload},
			{name: "reports false for a mark without a payload", line: setMark},
			{name: "reports false for a mark before a digit", line: setMark + "1table"},
			{name: "reports false for a mark before a space", line: setMark + " " + tablePayload},
			{name: "reports false for the brand without its colon", line: "+" + unitBrand + tablePayload},
			{name: "reports false for documentation", line: "Row is one record."},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				mark, payload, ok := plugin.CutCarrier(tt.line, unitBrand)
				assert.Equal(t, ok, tt.wantOK, "the line opens a carrier or not")
				assert.Equal(t, mark, tt.wantMark, "the mark is the one written")
				assert.Equal(t, payload, tt.wantPayload, "the payload follows the mark")
			})
		}
	})

	t.Run("AttachCarriers", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches each carrier to the subject at its own line", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			row := &node.Struct{Name: "Row"}
			u.AttachCarriers(row, []plugin.Carrier{{Mark: setMark, Payload: tablePayload, Pos: carrierAt}}, unitCode)
			coretest.AssertCodes(t, sink)
			attached := u.Graph().Attachments()
			assert.Length(t, attached, 1, "the carrier attaches")
			assert.Equal(t, attached[0].Subject, symbol.Symbol(row), "the subject is the one passed in",
				assert.ByIdentity())
			assert.Equal(t, attached[0].Raw.Name, tableName, "the name is the payload's")
			assert.Equal(t, attached[0].Raw.Pos, carrierAt, "the instance is at the carrier's line")
			assert.False(t, attached[0].Raw.Negated, "the instance is set")
		})

		t.Run("attaches a negated carrier as a negated instance", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			u.AttachCarriers(&node.Struct{Name: "Row"},
				[]plugin.Carrier{{Mark: negatedMark, Payload: tablePayload, Pos: carrierAt}}, unitCode)
			attached := u.Graph().Attachments()
			assert.Length(t, attached, 1, "the carrier attaches")
			assert.True(t, attached[0].Raw.Negated, "the instance is negated")
		})

		t.Run("attaches a directive-shaped carrier as a directive-shaped instance", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			u.AttachCarriers(&node.Struct{Name: "Row"}, []plugin.Carrier{
				{Mark: bareMark, Payload: tablePayload, Pos: carrierAt, DirectiveShaped: true},
			}, unitCode)
			attached := u.Graph().Attachments()
			assert.Length(t, attached, 1, "the carrier attaches")
			assert.True(t, attached[0].Raw.DirectiveShaped, "validation reads the shape off the instance")
		})

		t.Run("reports a carrier outside the grammar under the given code", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.AttachCarriers(&node.Struct{Name: "Row"},
				[]plugin.Carrier{{Mark: setMark, Payload: brokenPayload, Pos: carrierAt}}, unitCode)
			coretest.AssertCodes(t, sink, unitCode)
			got := reported(sink)
			assert.Equal(t, got[0].Pos, carrierAt, "the finding is at the carrier's line")
			assert.Equal(t, got[0].Origin, frontendOrigin, "the finding is under the frontend's origin")
			assert.Empty(t, u.Graph().Attachments(), "the carrier attaches nothing")
		})

		t.Run("quotes a refused carrier with its mark", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.AttachCarriers(&node.Struct{Name: "Row"},
				[]plugin.Carrier{{Mark: negatedMark, Payload: brokenPayload, Pos: carrierAt}}, unitCode)
			got := reported(sink)
			assert.Length(t, got, 1, "one finding is reported")
			assert.Contains(t, got[0].Msg, negatedMark+brokenPayload, "the carrier is quoted as written")
		})

		t.Run("attaches the carriers after a refused one", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			u.AttachCarriers(&node.Struct{Name: "Row"}, []plugin.Carrier{
				{Mark: setMark, Payload: brokenPayload, Pos: carrierAt},
				{Mark: setMark, Payload: tablePayload, Pos: carrierAt},
			}, unitCode)
			assert.Length(t, u.Graph().Attachments(), 1, "the valid carrier attaches")
		})

		t.Run("panics on a nil subject with a carrier to attach", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			carriers := []plugin.Carrier{{Mark: setMark, Payload: tablePayload, Pos: carrierAt}}
			assert.Panics(t, func() {
				u.AttachCarriers(nil, carriers, unitCode)
			}, "a carrier on no subject is the frontend's defect")
		})
	})

	t.Run("AttachSugar", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches the bare directive of a path of the brand and a name", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			row := &node.Struct{Name: "Row"}
			args := []directive.RawArg{{Key: "tag", Value: directive.RawValue{Text: "test", Quoted: true}}}
			marker := plugin.Sugar{Path: []string{unitBrand, string(stubName)}, Args: args, Pos: carrierAt}
			assert.True(t, u.AttachSugar(row, marker, unitCode), "the marker is a directive")
			coretest.AssertCodes(t, sink)
			attached := u.Graph().Attachments()
			assert.Length(t, attached, 1, "the marker attaches")
			expect.Equal(t, attached[0].Subject, symbol.Symbol(row), "the subject is the one passed in",
				assert.ByIdentity())
			expect.Equal(t, attached[0].Raw, directive.Raw{Name: stubName, Args: args, Pos: carrierAt},
				"the instance is set, with the marker's name, arguments and position")
		})

		t.Run("attaches the prefixed directive of a path of the brand, a plugin and a name", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			marker := plugin.Sugar{Path: []string{unitBrand, "stubgen", string(stubName)}, Pos: carrierAt}
			assert.True(t, u.AttachSugar(&node.Struct{Name: "Row"}, marker, unitCode), "the marker is a directive")
			attached := u.Graph().Attachments()
			assert.Length(t, attached, 1, "the marker attaches")
			assert.Equal(t, attached[0].Raw.Name, directive.Name("stubgen:stub"), "the name has the plugin's prefix")
		})

		t.Run("reports false for a marker outside the brand", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			marker := plugin.Sugar{Path: []string{"log"}, Pos: carrierAt}
			assert.False(t, u.AttachSugar(&node.Struct{Name: "Row"}, marker, unitCode), "the marker is metadata")
			coretest.AssertCodes(t, sink)
			assert.Empty(t, u.Graph().Attachments(), "the marker attaches nothing")
		})

		t.Run("reports false for a marker without a path", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			assert.False(t, u.AttachSugar(&node.Struct{Name: "Row"}, plugin.Sugar{}, unitCode),
				"a marker without a name is not a directive")
		})

		t.Run("reports false for a marker outside the brand with a refusal", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			marker := plugin.Sugar{Path: []string{"log"}, Refusal: sugarRefusal, Pos: carrierAt}
			assert.False(t, u.AttachSugar(&node.Struct{Name: "Row"}, marker, unitCode), "the marker is metadata")
			coretest.AssertCodes(t, sink)
		})

		t.Run("reports a marker of the brand with a refusal", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			marker := plugin.Sugar{Path: []string{unitBrand, string(stubName)}, Refusal: sugarRefusal, Pos: carrierAt}
			assert.True(t, u.AttachSugar(&node.Struct{Name: "Row"}, marker, unitCode),
				"a marker of the brand is a directive")
			coretest.AssertCodes(t, sink, unitCode)
			got := reported(sink)
			expect.Equal(t, got[0].Pos, carrierAt, "the finding is at the marker")
			expect.Contains(t, got[0].Msg, sugarRefusal, "the finding contains the frontend's refusal")
			expect.Contains(t, got[0].Msg, string(stubName), "the finding contains the directive's name")
			expect.Empty(t, u.Graph().Attachments(), "the marker attaches nothing")
		})

		refusals := []struct {
			name string
			path []string
			want string
		}{
			{name: "reports a marker of the brand alone", path: []string{unitBrand}, want: "has 0 names"},
			{
				name: "reports a marker of three names after the brand",
				path: []string{unitBrand, "a", "b", "c"}, want: "has 3 names",
			},
			{
				name: "reports a name that is not a valid directive name",
				path: []string{unitBrand, "$stub"}, want: `the name "$stub"`,
			},
			{
				name: "reports a marker with an empty plugin",
				path: []string{unitBrand, "", string(stubName)}, want: `the name " stub"`,
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				u, sink := reporting(t, unitTree())
				marker := plugin.Sugar{Path: tt.path, Pos: carrierAt}
				assert.True(t, u.AttachSugar(&node.Struct{Name: "Row"}, marker, unitCode),
					"a marker of the brand is a directive")
				coretest.AssertCodes(t, sink, unitCode)
				got := reported(sink)
				expect.Equal(t, got[0].Pos, carrierAt, "the finding is at the marker")
				expect.Contains(t, got[0].Msg, tt.want, "the finding describes the fault")
				expect.Empty(t, u.Graph().Attachments(), "the marker attaches nothing")
			})
		}

		t.Run("panics on a nil subject with a marker of the brand", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			marker := plugin.Sugar{Path: []string{unitBrand, string(stubName)}, Pos: carrierAt}
			assert.Panics(t, func() { u.AttachSugar(nil, marker, unitCode) },
				"a marker on no subject is the frontend's defect")
		})
	})

	t.Run("RefuseCarriers", func(t *testing.T) {
		t.Parallel()

		t.Run("reports each carrier under the given code at its own line", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			next := carrierAt
			next.Line++
			u.RefuseCarriers(unitCode, []plugin.Carrier{
				{Mark: setMark, Payload: tablePayload, Pos: carrierAt},
				{Mark: negatedMark, Payload: tablePayload, Pos: next},
			}, unaddressed)
			coretest.AssertCodes(t, sink, unitCode, unitCode)
			got := reported(sink)
			expect.Equal(t, got[0].Pos, carrierAt, "the first finding is at the first carrier")
			expect.Equal(t, got[1].Pos, next, "the second finding is at the second carrier")
			expect.Equal(t, got[0].Severity, diag.SeverityError, "the finding is an Error")
		})

		t.Run("quotes the carrier with its mark", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.RefuseCarriers(unitCode, []plugin.Carrier{{Mark: negatedMark, Payload: tablePayload, Pos: carrierAt}},
				unaddressed)
			got := reported(sink)
			assert.Length(t, got, 1, "one finding is reported")
			assert.Contains(t, got[0].Msg, strconv.Quote(negatedMark+tablePayload), "the carrier is quoted as written")
		})

		t.Run("includes the description of the subject", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.RefuseCarriers(unitCode, []plugin.Carrier{{Mark: setMark, Payload: tablePayload, Pos: carrierAt}},
				unaddressed)
			got := reported(sink)
			assert.Length(t, got, 1, "one finding is reported")
			assert.Contains(t, got[0].Msg, unaddressed, "the finding contains the description of the subject")
		})

		t.Run("does not attach the carrier", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			u.RefuseCarriers(unitCode, []plugin.Carrier{{Mark: setMark, Payload: tablePayload, Pos: carrierAt}},
				unaddressed)
			assert.Empty(t, u.Graph().Attachments(), "a refused carrier attaches nowhere")
		})

		t.Run("does not report a finding for no carriers", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.RefuseCarriers(unitCode, nil, unaddressed)
			coretest.AssertCodes(t, sink)
		})
	})

	t.Run("RefuseSugar", func(t *testing.T) {
		t.Parallel()

		t.Run("reports each marker under the given code at its position", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			next := carrierAt
			next.Line++
			u.RefuseSugar(unitCode, []plugin.Sugar{
				{Path: []string{unitBrand, string(stubName)}, Pos: carrierAt},
				{Path: []string{unitBrand, string(stubName)}, Pos: next},
			}, markerSep, unaddressed)
			coretest.AssertCodes(t, sink, unitCode, unitCode)
			got := reported(sink)
			expect.Equal(t, got[0].Pos, carrierAt, "the first finding is at the first marker")
			expect.Equal(t, got[1].Pos, next, "the second finding is at the second marker")
			expect.Equal(t, got[0].Severity, diag.SeverityError, "the finding is an Error")
		})

		t.Run("joins the path of the marker with the separator", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.RefuseSugar(unitCode, []plugin.Sugar{{Path: []string{unitBrand, string(stubName)}, Pos: carrierAt}},
				markerSep, unaddressed)
			got := reported(sink)
			assert.Length(t, got, 1, "one finding is reported")
			assert.Contains(t, got[0].Msg, unitBrand+markerSep+string(stubName),
				"the finding contains the path as the author wrote it")
		})

		t.Run("includes the description of the subject", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.RefuseSugar(unitCode, []plugin.Sugar{{Path: []string{unitBrand, string(stubName)}, Pos: carrierAt}},
				markerSep, unaddressed)
			got := reported(sink)
			assert.Length(t, got, 1, "one finding is reported")
			assert.Contains(t, got[0].Msg, unaddressed, "the finding contains the description of the subject")
		})

		t.Run("does not report a finding for no markers", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.RefuseSugar(unitCode, nil, markerSep, unaddressed)
			coretest.AssertCodes(t, sink)
		})
	})

	t.Run("ReportSyntax", func(t *testing.T) {
		t.Parallel()

		t.Run("reports each error at its position", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.ReportSyntax(unitCode, syntaxErrors(2))
			coretest.AssertCodes(t, sink, unitCode, unitCode)
			got := reported(sink)
			expect.Equal(t, got[0].Pos, position.Pos{File: commentAt.File, Line: 1, Col: 1},
				"the first finding is at the first error")
			expect.Equal(t, got[1].Pos, position.Pos{File: commentAt.File, Line: 2, Col: 1},
				"the second finding is at the second error")
			expect.Equal(t, got[0].Msg, syntaxMessage, "the finding is the sentence of the error")
			expect.Equal(t, got[0].Severity, diag.SeverityError, "the finding is an Error")
		})

		t.Run("reports one finding past the cap", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.ReportSyntax(unitCode, syntaxErrors(plugin.MaxSyntaxFindings+syntaxOverflow))
			got := reported(sink)
			assert.Length(t, got, plugin.MaxSyntaxFindings+1, "the cap of findings and one more are reported")
			expect.Equal(t, got[plugin.MaxSyntaxFindings].Pos,
				position.Pos{File: commentAt.File, Line: plugin.MaxSyntaxFindings + 1, Col: 1},
				"the last finding is at the first error past the cap")
			expect.Equal(t, got[plugin.MaxSyntaxFindings].Msg,
				"and "+strconv.Itoa(syntaxOverflow)+" more syntax errors",
				"the last finding counts the errors past the cap")
		})

		t.Run("reports every error up to the cap one by one", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.ReportSyntax(unitCode, syntaxErrors(plugin.MaxSyntaxFindings))
			got := reported(sink)
			assert.Length(t, got, plugin.MaxSyntaxFindings, "each error up to the cap is a finding")
			expect.Equal(t, got[plugin.MaxSyntaxFindings-1].Msg, syntaxMessage,
				"the last finding is the sentence of the last error")
		})

		t.Run("does not report a finding for no errors", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, unitTree())
			u.ReportSyntax(unitCode, syntaxErrors(0))
			coretest.AssertCodes(t, sink)
		})
	})

	at := position.Pos{File: "svc/store/row.go", Line: 4, Col: 2}
	severities := []struct {
		method string
		name   string
		report func(*plugin.SourceUnit)
		want   diag.Severity
	}{
		{
			method: "Errorf", name: "reports an Error under the origin of the frontend",
			report: func(u *plugin.SourceUnit) { u.Errorf(unitCode, at, "the declaration names %s twice", "Row") },
			want:   diag.SeverityError,
		},
		{
			method: "Warnf", name: "reports a Warning under the origin of the frontend",
			report: func(u *plugin.SourceUnit) { u.Warnf(unitCode, at, "the declaration shadows %s", "Row") },
			want:   diag.SeverityWarning,
		},
		{
			method: "Infof", name: "reports an Info under the origin of the frontend",
			report: func(u *plugin.SourceUnit) { u.Infof(unitCode, at, "the declaration %s loaded", "Row") },
			want:   diag.SeverityInfo,
		},
	}
	for _, tt := range severities {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()

			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				u, sink := reporting(t, unitTree())
				tt.report(u)
				coretest.AssertCodes(t, sink, unitCode)
				got := reported(sink)
				assert.Equal(t, got[0].Severity, tt.want, "the severity is the method's")
				assert.Equal(t, got[0].Origin, frontendOrigin, "the origin is the frontend's")
				assert.Equal(t, got[0].Pos, at, "the position is the one passed in")
				assert.Contains(t, got[0].Msg, "Row", "the format arguments fill the message")
			})
		})
	}

	t.Run("Doc", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			raw  string
			want []string
		}{
			{
				name: "strips line markers",
				raw:  "// Row is one record.\n// It keys by name.",
				want: []string{"Row is one record.", "It keys by name."},
			},
			{
				name: "strips block delimiters with the gutter",
				raw:  "/**\n * Row is one record.\n */",
				want: []string{"Row is one record."},
			},
			{
				name: "drops a directive line",
				raw:  "// Row is one record.\n//go:embed schema.sql",
				want: []string{"Row is one record."},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, unitOf(t, unitTree()).Doc(tt.raw), tt.want, "the clean lines are pinned")
			})
		}
	})

	t.Run("DocLines", func(t *testing.T) {
		t.Parallel()

		t.Run("drops a directive line from clean lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, unitOf(t, unitTree()).DocLines([]string{"Clean.", "go:generate x"}),
				[]string{"Clean."}, "the directive line is dropped")
		})

		t.Run("returns every line for a syntax without the directive convention", func(t *testing.T) {
			t.Parallel()

			u := unitOver([]plugin.SourceRef{unitFile()}, plugin.DepthFull, plugin.CommentSyntax{Line: []string{"//"}})
			assert.Equal(t, u.DocLines([]string{"go:generate x"}), []string{"go:generate x"},
				"the directive line is kept")
		})
	})

	t.Run("Graph", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one builder for every call", func(t *testing.T) {
			t.Parallel()

			u := unitOf(t, unitTree())
			first, second := u.Graph(), u.Graph()
			assert.Equal(t, second, first, "the unit has one write handle", assert.ByIdentity())
		})
	})

	for _, tt := range unitReports() {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()

			t.Run("reports the finding under the origin of the frontend", func(t *testing.T) {
				t.Parallel()

				u, sink := reporting(t, unitTree())
				tt.report(u)
				got := reported(sink)
				assert.Length(t, got, 1, "one finding arrives")
				assert.Equal(t, got[0].Severity, tt.want, "at the method's severity")
				assert.Equal(t, got[0].Origin, frontendOrigin, "under the frontend's origin")
				assert.Equal(t, got[0].Msg, "a fault", "with the formatted message")
			})
		})
	}

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one package per path", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			a := gb.Package("svc/store")
			assert.Equal(t, gb.Package("svc/store"), a, "the second call returns the first package",
				assert.ByIdentity())
		})

		t.Run("returns the segments of the path", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			assert.Equal(t, gb.Package("svc/store").Path, []string{"svc", "store"}, "one segment per element")
		})

		t.Run("returns no segments for the empty path", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			assert.Empty(t, gb.Package("").Path, "the global package has no segments")
		})
	})

	t.Run("Packages", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the packages in first-touch order", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			a := gb.Package("svc/store")
			b := gb.Package("svc/api")
			gb.Package("svc/store")
			got := gb.Packages()
			assert.Length(t, got, 2, "each path is one package")
			expect.Equal(t, got[0], a, "the first-touched package comes first", assert.ByIdentity())
			expect.Equal(t, got[1], b, "the second-touched package comes second", assert.ByIdentity())
		})
	})

	t.Run("Scope", func(t *testing.T) {
		t.Parallel()

		t.Run("records the bindings under the file node", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			file := &node.File{Path: "svc/store/row.go"}
			gb.Scope(file, map[string]string{"emit": "core/emit"})
			scopes := gb.Scopes()
			assert.Length(t, scopes, 1, "one record is kept")
			assert.Equal(t, scopes[0].File, file, "the record is under its file node", assert.ByIdentity())
		})

		t.Run("panics on a nil file", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			assert.Panics(t, func() { gb.Scope(nil, nil) }, "a nil file is a frontend defect")
		})
	})

	t.Run("Attach", func(t *testing.T) {
		t.Parallel()

		t.Run("records the instance on its subject", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			row := &node.Struct{Name: "Row"}
			gb.Attach(row, directive.Raw{Name: tableName})
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "one attachment is kept")
			assert.Equal(t, attached[0].Subject, symbol.Symbol(row), "the attachment is on its subject",
				assert.ByIdentity())
			assert.Equal(t, attached[0].Raw.Name, tableName, "the attachment has the instance")
		})

		t.Run("panics on a nil subject", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			assert.Panics(t, func() { gb.Attach(nil, directive.Raw{}) }, "a nil subject is a frontend defect")
		})
	})

	t.Run("Stamp", func(t *testing.T) {
		t.Parallel()

		t.Run("records the stamp on its subject", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			file := &node.File{Path: "svc/store/row.go"}
			gb.Stamp(file, meta.RawStamp{Key: "fake.testFile", Value: true})
			stamps := gb.StampRecords()
			assert.Length(t, stamps, 1, "one stamp is kept")
			assert.Equal(t, stamps[0].Subject, symbol.Symbol(file), "the stamp is on its subject", assert.ByIdentity())
		})

		t.Run("panics on a nil subject", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			assert.Panics(t, func() { gb.Stamp(nil, meta.RawStamp{}) }, "a nil subject is a frontend defect")
		})
	})

	t.Run("StampModule", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the module and its root at the first file of the package", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			pkg := gb.Package(modulePackage)
			first := position.Pos{File: "svc/store/row.go", Line: 1, Col: 1}
			pkg.Files = []*node.File{{Path: first.File, Pos: first}, {Path: "svc/store/col.go"}}
			gb.StampModule(pkg, moduleName, moduleRoot)
			assert.Equal(t, gb.StampRecords(), []plugin.StampRecord{
				{Subject: pkg, Stamp: meta.RawStamp{Key: meta.ModuleKey, Value: moduleName, Pos: first}},
				{Subject: pkg, Stamp: meta.RawStamp{Key: meta.ModuleRootKey, Value: moduleRoot, Pos: first}},
			}, "the package has the module and the root of its manifest")
		})

		t.Run("stamps a package without files at its own position", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			pkg := gb.Package(modulePackage)
			pkg.Pos = position.Pos{File: "svc/store", Line: 1}
			gb.StampModule(pkg, moduleName, moduleRoot)
			stamps := gb.StampRecords()
			assert.Length(t, stamps, 2, "the module and its root are stamped")
			expect.Equal(t, stamps[0].Stamp.Pos, pkg.Pos, "the module is at the package's position")
			expect.Equal(t, stamps[1].Stamp.Pos, pkg.Pos, "the root is at the package's position")
		})

		t.Run("stamps the module of each call after a call with another module", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			gb.StampModule(recordPackage, moduleName, moduleRoot)
			gb.StampModule(secondPackage, otherModule, otherRoot)
			stamps := gb.StampRecords()
			assert.Length(t, stamps, 4, "each package has its module and its root")
			expect.Equal(t, stamps[2].Stamp.Value, any(otherModule), "the second package has the second module")
			expect.Equal(t, stamps[3].Stamp.Value, any(otherRoot), "the second package has the second root")
		})

		t.Run("does not record a stamp for an empty module", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			gb.StampModule(gb.Package(modulePackage), "", moduleRoot)
			assert.Empty(t, gb.StampRecords(), "a package outside every module has no module facts")
		})

		t.Run("panics on a nil package with a module", func(t *testing.T) {
			t.Parallel()

			gb := unitOf(t, unitTree()).Graph()
			assert.Panics(t, func() { gb.StampModule(nil, moduleName, moduleRoot) },
				"a module on no package is a frontend defect")
		})
	})

	t.Run("Rehome", func(t *testing.T) {
		t.Parallel()

		rehomed := func(tb assert.TB) (*plugin.GraphBuilder, symbol.Symbol, symbol.Symbol) {
			tb.Helper()

			gb := unitOf(tb, unitTree()).Graph()
			old := &node.Alias{Name: "Color"}
			replacement := &node.Enum{Name: "Color"}
			other := &node.Struct{Name: "Row"}
			gb.Attach(old, directive.Raw{Name: "stringer"})
			gb.Attach(other, directive.Raw{Name: tableName})
			gb.Stamp(old, meta.RawStamp{Key: "fake.underlying", Value: "basic"})
			gb.Rehome(old, replacement)
			return gb, replacement, other
		}

		t.Run("moves the attachments onto the replacement", func(t *testing.T) {
			t.Parallel()

			gb, replacement, _ := rehomed(t)
			assert.Equal(t, gb.Attachments()[0].Subject, replacement, "the attachment is on the replacement",
				assert.ByIdentity())
		})

		t.Run("keeps the attachment of an unrelated subject", func(t *testing.T) {
			t.Parallel()

			gb, _, other := rehomed(t)
			assert.Equal(t, gb.Attachments()[1].Subject, other, "the attachment is on its subject", assert.ByIdentity())
		})

		t.Run("moves the stamps onto the replacement", func(t *testing.T) {
			t.Parallel()

			gb, replacement, _ := rehomed(t)
			assert.Equal(t, gb.StampRecords()[0].Subject, replacement, "the stamp is on the replacement",
				assert.ByIdentity())
		})

		t.Run("panics on a nil source", func(t *testing.T) {
			t.Parallel()

			gb, replacement, _ := rehomed(t)
			assert.Panics(t, func() { gb.Rehome(nil, replacement) }, "a nil source is a frontend defect")
		})

		t.Run("panics on a nil destination", func(t *testing.T) {
			t.Parallel()

			gb, replacement, _ := rehomed(t)
			assert.Panics(t, func() { gb.Rehome(replacement, nil) }, "a nil destination is a frontend defect")
		})
	})
}

// TestSourceUnitAllocs checks that each method of the source unit
// allocates only what it returns or keeps. The ordinary run runs no
// benchmark, so this test checks the ceilings. Each call that records into
// its unit takes a new unit, which the setup builds outside the count. The
// check does not run in parallel, because the count includes the
// allocations of every goroutine.
func TestSourceUnitAllocs(t *testing.T) {
	refs, tree, syntax, sink := []plugin.SourceRef{unitFile()}, unitTree(), goSyntax(), diag.NewSink()
	var built *plugin.SourceUnit
	assert.MaxAllocs(t, func() {
		built = plugin.NewSourceUnit(refs, tree, plugin.DepthFull, syntax, unitBrand, sink, frontendOrigin)
	}, newSourceUnitAllocs, "NewSourceUnit allocates the unit, its readable paths and its graph builder")
	assert.Equal(t, built.Depth(), plugin.DepthFull, "NewSourceUnit returns a unit at the given depth")

	u := unitOf(t, unitTree())
	var parts plugin.CommentParts
	assert.MaxAllocs(t, func() { parts = u.Comment(docComment, commentAt) }, commentAllocs,
		"Comment allocates its lines and the documentation")
	assert.Length(t, parts.Docs, 3, "Comment returns the three documentation lines")
	assert.MaxAllocs(t, func() { parts = u.Comment(carrierComment, commentAt) }, carrierCommentAllocs,
		"Comment allocates the list of carriers beside the documentation")
	assert.Length(t, parts.Carriers, 1, "Comment returns the carrier")

	var lines []string
	assert.MaxAllocs(
		t,
		func() { lines = u.Doc(docComment) },
		docAllocs,
		"Doc allocates its lines and the documentation",
	)
	assert.Length(t, lines, 3, "Doc returns the three documentation lines")
	clean := []string{"Clean.", "go:generate x"}
	assert.MaxAllocs(t, func() { lines = u.DocLines(clean) }, docLinesAllocs, "DocLines allocates the filtered list")
	assert.Length(t, lines, 1, "DocLines drops the directive line")

	held := false
	line := setMark + tablePayload
	assert.MaxAllocs(t, func() { _, _, held = plugin.CutCarrier(line, unitBrand) }, 0, "CutCarrier allocates nothing")
	assert.True(t, held, "CutCarrier reads the set mark")
	negated := plugin.Carrier{Mark: negatedMark, Payload: tablePayload}
	assert.MaxAllocs(t, func() { held = negated.Negated() }, 0, "Negated allocates nothing")
	assert.True(t, held, "Negated reports true for the negated mark")
	var (
		members int
		depth   plugin.Depth
		graph   *plugin.GraphBuilder
		brand   string
	)
	assert.MaxAllocs(t, func() {
		members, depth, graph, brand = len(u.Files()), u.Depth(), u.Graph(), u.Brand()
	}, 0, "Files, Depth, Graph and Brand allocate nothing")
	expect.Equal(t, members, 1, "Files returns the unit's one member")
	expect.Equal(t, depth, plugin.DepthFull, "Depth returns the unit's depth")
	expect.NotNil(t, graph, "Graph returns the unit's builder")
	expect.Equal(t, brand, unitBrand, "Brand returns the unit's brand")

	read := unitRead(u)
	var err error
	assert.MaxAllocs(t, func() { err = cmp.Or(err, read.own()) }, plainAllocs(t, read),
		"Read allocates what the read of its tree allocates")
	assert.NoError(t, err, "Read reads the unit's member")
	for _, tt := range unitReports() {
		assert.MaxAllocs(t, func() { tt.report(u) }, reportAllocs, tt.method+" allocates the finding's message")
	}

	fresh := func() *plugin.SourceUnit { return unitOf(t, unitTree()) }
	carriers := []plugin.Carrier{{Mark: bareMark, Payload: tablePayload, Pos: carrierAt}}
	assert.MaxAllocsWithSetup(t, fresh,
		func(fu *plugin.SourceUnit) { fu.AttachCarriers(recordSubject, carriers, unitCode) },
		attachCarriersAllocs, "AttachCarriers allocates the parsed instance and the list of attachments")
	bare := plugin.Sugar{Path: []string{unitBrand, string(stubName)}, Pos: carrierAt}
	assert.MaxAllocsWithSetup(t, fresh,
		func(fu *plugin.SourceUnit) { fu.AttachSugar(recordSubject, bare, unitCode) },
		attachSugarAllocs, "AttachSugar allocates the list of attachments for a bare marker")
	prefixed := plugin.Sugar{Path: []string{unitBrand, "stubgen", string(stubName)}, Pos: carrierAt}
	assert.MaxAllocsWithSetup(t, fresh,
		func(fu *plugin.SourceUnit) { fu.AttachSugar(recordSubject, prefixed, unitCode) },
		attachPrefixedSugarAllocs, "AttachSugar allocates the prefixed name and the list of attachments")
	foreign := plugin.Sugar{Path: []string{"log"}, Pos: carrierAt}
	assert.MaxAllocs(t, func() { held = u.AttachSugar(recordSubject, foreign, unitCode) }, 0,
		"AttachSugar allocates nothing for a marker outside the brand")
	assert.False(t, held, "AttachSugar reports a marker outside the brand as metadata")

	assert.MaxAllocsWithSetup(t, fresh, func(fu *plugin.SourceUnit) { fu.Use(usedRow) }, firstUseAllocs,
		"Use allocates the set of uses at the first use of the unit")
	again := fresh()
	again.Use(usedRow)
	assert.MaxAllocs(t, func() { again.Use(usedRow) }, 0, "Use allocates nothing for a use that it recorded before")
	var uses []symbol.Identity
	assert.MaxAllocs(t, func() { uses = u.Uses() }, 0, "Uses allocates nothing for a unit without uses")
	assert.Nil(t, uses, "Uses returns no use for a unit that records none")

	assert.MaxAllocs(t, func() { u.RefuseCarriers(unitCode, nil, unaddressed) }, 0,
		"RefuseCarriers allocates nothing for no carriers")
	assert.MaxAllocs(t, func() { u.RefuseCarriers(unitCode, refusedCarriers, unaddressed) }, refuseCarrierAllocs,
		"RefuseCarriers allocates the quoted carrier, the boxed arguments and the message")
	assert.MaxAllocs(t, func() { u.RefuseSugar(unitCode, nil, markerSep, unaddressed) }, 0,
		"RefuseSugar allocates nothing for no markers")
	assert.MaxAllocs(t, func() { u.RefuseSugar(unitCode, refusedSugar, markerSep, unaddressed) }, refuseSugarAllocs,
		"RefuseSugar allocates the joined path, the boxed arguments and the message")
	assert.MaxAllocs(t, func() { u.ReportSyntax(unitCode, oneSyntaxError) }, reportSyntaxAllocs,
		"ReportSyntax allocates its loop state, the boxed sentence and the message of a finding")
	assert.MaxAllocsWithSetup(t, fresh,
		func(fu *plugin.SourceUnit) { fu.Graph().StampModule(recordPackage, moduleName, moduleRoot) },
		stampModuleAllocs, "StampModule allocates the boxed module and root, and the list of stamps")
	assert.MaxAllocsWithSetup(t, stampedModule(t),
		func(gb *plugin.GraphBuilder) { gb.StampModule(secondPackage, moduleName, moduleRoot) },
		stampModuleAgainAllocs, "StampModule reuses the boxed module and root for a second package of the module")

	assert.MaxAllocsWithSetup(t, fresh, func(fu *plugin.SourceUnit) { fu.Graph().Package("svc/store") },
		firstPackageAllocs, "Package allocates a unit's first package")
	gb := fresh().Graph()
	gb.Package("svc/store")
	assert.MaxAllocs(t, func() { gb.Package("svc/store") }, 0,
		"Package allocates nothing for a package it returned before")
	var pkgs []*node.Package
	assert.MaxAllocs(t, func() { pkgs = gb.Packages() }, packagesAllocs, "Packages allocates the list")
	assert.Length(t, pkgs, 1, "Packages returns the one package")

	for _, tt := range unitRecords() {
		assert.MaxAllocsWithSetup(t, fresh, func(fu *plugin.SourceUnit) { tt.record(fu.Graph()) },
			firstRecordAllocs, tt.method+" allocates a unit's list of its records")
	}
	full := recorded(t)
	replacement := &node.Struct{Name: "Row"}
	assert.MaxAllocs(t, func() { full.Rehome(recordSubject, replacement) }, 0, "Rehome allocates nothing")
	var scopes, attachments, stamps int
	assert.MaxAllocs(t, func() {
		scopes, attachments, stamps = len(full.Scopes()), len(full.Attachments()), len(full.StampRecords())
	}, 0, "Scopes, Attachments and StampRecords allocate nothing")
	expect.Equal(t, scopes, 1, "Scopes returns the builder's one scope")
	expect.Equal(t, attachments, 1, "Attachments returns the builder's one attachment")
	expect.Equal(t, stamps, 1, "StampRecords returns the builder's one stamp")
}

// BenchmarkSourceUnit measures the calls that the parse of a frontend
// makes on its unit. They are the construction of the unit, the comment
// pipeline once for each comment, the reads, the reports and the records.
// Each call that records into its unit runs on a unit that the benchmark
// builds outside the measurement.
func BenchmarkSourceUnit(b *testing.B) {
	b.Run("NewSourceUnit", func(b *testing.B) {
		refs, tree, syntax, sink := []plugin.SourceRef{unitFile()}, unitTree(), goSyntax(), diag.NewSink()
		c := bench.Start(b).MaxAllocs(newSourceUnitAllocs)
		defer c.End()
		var u *plugin.SourceUnit
		for c.Loop() {
			u = plugin.NewSourceUnit(refs, tree, plugin.DepthFull, syntax, unitBrand, sink, frontendOrigin)
		}
		assert.Length(b, u.Files(), 1, "NewSourceUnit returns a unit of the one member")
	})

	u := unitOf(b, unitTree())
	comments := []struct {
		name   string
		raw    string
		allocs uint64
	}{
		{name: "three lines of documentation", raw: docComment, allocs: commentAllocs},
		{name: "documentation above a carrier", raw: carrierComment, allocs: carrierCommentAllocs},
	}
	b.Run("Comment", func(b *testing.B) {
		for _, tt := range comments {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var parts plugin.CommentParts
				for c.Loop() {
					parts = u.Comment(tt.raw, commentAt)
				}
				assert.NotEmpty(b, parts.Docs, "Comment returns the documentation")
			})
		}
	})

	b.Run("Doc", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(docAllocs)
		defer c.End()
		var lines []string
		for c.Loop() {
			lines = u.Doc(docComment)
		}
		assert.Length(b, lines, 3, "Doc returns the three documentation lines")
	})

	b.Run("DocLines", func(b *testing.B) {
		clean := []string{"Clean.", "go:generate x"}
		c := bench.Start(b).MaxAllocs(docLinesAllocs)
		defer c.End()
		var lines []string
		for c.Loop() {
			lines = u.DocLines(clean)
		}
		assert.Length(b, lines, 1, "DocLines drops the directive line")
	})

	b.Run("CutCarrier", func(b *testing.B) {
		line := setMark + tablePayload
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			_, _, held = plugin.CutCarrier(line, unitBrand)
		}
		assert.True(b, held, "CutCarrier reads the set mark")
	})

	b.Run("Negated", func(b *testing.B) {
		negated := plugin.Carrier{Mark: negatedMark, Payload: tablePayload}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			held = negated.Negated()
		}
		assert.True(b, held, "Negated reports true for the negated mark")
	})

	b.Run("Files", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []plugin.SourceRef
		for c.Loop() {
			got = u.Files()
		}
		assert.Length(b, got, 1, "Files returns the one member")
	})

	b.Run("Depth", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got plugin.Depth
		for c.Loop() {
			got = u.Depth()
		}
		assert.Equal(b, got, plugin.DepthFull, "Depth returns the unit's depth")
	})

	b.Run("Brand", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = u.Brand()
		}
		assert.Equal(b, got, unitBrand, "Brand returns the unit's brand")
	})

	b.Run("Graph", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got *plugin.GraphBuilder
		for c.Loop() {
			got = u.Graph()
		}
		assert.Equal(b, got, u.Graph(), "Graph returns the unit's builder", assert.ByIdentity())
	})

	b.Run("Read", func(b *testing.B) {
		read := unitRead(u)
		c := bench.Start(b).MaxAllocs(plainAllocs(b, read))
		defer c.End()
		var err error
		for c.Loop() {
			err = read.own()
		}
		assert.NoError(b, err, "the member reads")
	})

	for _, tt := range unitReports() {
		b.Run(tt.method, func(b *testing.B) {
			reporter := unitOf(b, unitTree())
			c := bench.Start(b).MaxAllocs(reportAllocs)
			defer c.End()
			for c.Loop() {
				tt.report(reporter)
			}
		})
	}

	b.Run("AttachCarriers", func(b *testing.B) {
		b.Run("the first carrier of a unit", func(b *testing.B) {
			carriers := []plugin.Carrier{{Mark: bareMark, Payload: tablePayload, Pos: carrierAt}}
			var unit *plugin.SourceUnit
			fresh := func() { unit = unitOf(b, unitTree()) }
			c := bench.Start(b).MaxAllocs(attachCarriersAllocs)
			defer c.End()
			for c.Loop() {
				c.Excluding(fresh)
				unit.AttachCarriers(recordSubject, carriers, unitCode)
			}
			assert.Length(b, unit.Graph().Attachments(), 1, "the carrier is attached")
		})
	})

	b.Run("AttachSugar", func(b *testing.B) {
		markers := []struct {
			name   string
			marker plugin.Sugar
			allocs uint64
		}{
			{
				name:   "the first marker of the brand and a name",
				marker: plugin.Sugar{Path: []string{unitBrand, string(stubName)}, Pos: carrierAt},
				allocs: attachSugarAllocs,
			},
			{
				name:   "the first marker with a plugin",
				marker: plugin.Sugar{Path: []string{unitBrand, "stubgen", string(stubName)}, Pos: carrierAt},
				allocs: attachPrefixedSugarAllocs,
			},
		}
		for _, tt := range markers {
			b.Run(tt.name, func(b *testing.B) {
				var unit *plugin.SourceUnit
				fresh := func() { unit = unitOf(b, unitTree()) }
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				for c.Loop() {
					c.Excluding(fresh)
					unit.AttachSugar(recordSubject, tt.marker, unitCode)
				}
				assert.Length(b, unit.Graph().Attachments(), 1, "the marker is attached")
			})
		}
	})

	refusals := []struct {
		method string
		cases  []unitRefusal
	}{
		{method: "RefuseCarriers", cases: []unitRefusal{
			{
				name:   "no carriers",
				refuse: func(ru *plugin.SourceUnit) { ru.RefuseCarriers(unitCode, nil, unaddressed) },
			},
			{
				name: "one carrier", allocs: refuseCarrierAllocs,
				refuse: func(ru *plugin.SourceUnit) { ru.RefuseCarriers(unitCode, refusedCarriers, unaddressed) },
			},
		}},
		{method: "RefuseSugar", cases: []unitRefusal{
			{
				name:   "no markers",
				refuse: func(ru *plugin.SourceUnit) { ru.RefuseSugar(unitCode, nil, markerSep, unaddressed) },
			},
			{
				name: "one marker", allocs: refuseSugarAllocs,
				refuse: func(ru *plugin.SourceUnit) { ru.RefuseSugar(unitCode, refusedSugar, markerSep, unaddressed) },
			},
		}},
		{method: "ReportSyntax", cases: []unitRefusal{
			{
				name: "one error", allocs: reportSyntaxAllocs,
				refuse: func(ru *plugin.SourceUnit) { ru.ReportSyntax(unitCode, oneSyntaxError) },
			},
		}},
	}
	for _, method := range refusals {
		b.Run(method.method, func(b *testing.B) {
			for _, tt := range method.cases {
				b.Run(tt.name, func(b *testing.B) {
					reporter := unitOf(b, unitTree())
					c := bench.Start(b).MaxAllocs(tt.allocs)
					defer c.End()
					for c.Loop() {
						tt.refuse(reporter)
					}
				})
			}
		})
	}

	b.Run("StampModule", func(b *testing.B) {
		b.Run("the first module of a unit", func(b *testing.B) {
			var gb *plugin.GraphBuilder
			fresh := func() { gb = unitOf(b, unitTree()).Graph() }
			c := bench.Start(b).MaxAllocs(stampModuleAllocs)
			defer c.End()
			for c.Loop() {
				c.Excluding(fresh)
				gb.StampModule(recordPackage, moduleName, moduleRoot)
			}
			assert.Length(b, gb.StampRecords(), 2, "the module and its root are stamped")
		})

		b.Run("a second package of the module", func(b *testing.B) {
			stamped := stampedModule(b)
			var gb *plugin.GraphBuilder
			fresh := func() { gb = stamped() }
			c := bench.Start(b).MaxAllocs(stampModuleAgainAllocs)
			defer c.End()
			for c.Loop() {
				c.Excluding(fresh)
				gb.StampModule(secondPackage, moduleName, moduleRoot)
			}
			assert.Length(b, gb.StampRecords(), 4, "both packages have the module and its root")
		})
	})

	b.Run("Package", func(b *testing.B) {
		b.Run("the first package of a unit", func(b *testing.B) {
			var gb *plugin.GraphBuilder
			fresh := func() { gb = unitOf(b, unitTree()).Graph() }
			c := bench.Start(b).MaxAllocs(firstPackageAllocs)
			defer c.End()
			var pkg *node.Package
			for c.Loop() {
				c.Excluding(fresh)
				pkg = gb.Package("svc/store")
			}
			assert.Equal(b, pkg.Path, []string{"svc", "store"}, "Package returns the path's package")
		})

		b.Run("a package returned before", func(b *testing.B) {
			gb := unitOf(b, unitTree()).Graph()
			first := gb.Package("svc/store")
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var pkg *node.Package
			for c.Loop() {
				pkg = gb.Package("svc/store")
			}
			assert.Equal(b, pkg, first, "Package returns the first package", assert.ByIdentity())
		})
	})

	b.Run("Packages", func(b *testing.B) {
		gb := unitOf(b, unitTree()).Graph()
		gb.Package("svc/store")
		c := bench.Start(b).MaxAllocs(packagesAllocs)
		defer c.End()
		var pkgs []*node.Package
		for c.Loop() {
			pkgs = gb.Packages()
		}
		assert.Length(b, pkgs, 1, "Packages returns the one package")
	})

	for _, tt := range unitRecords() {
		b.Run(tt.method, func(b *testing.B) {
			b.Run("the first record of a unit", func(b *testing.B) {
				var gb *plugin.GraphBuilder
				fresh := func() { gb = unitOf(b, unitTree()).Graph() }
				c := bench.Start(b).MaxAllocs(firstRecordAllocs)
				defer c.End()
				for c.Loop() {
					c.Excluding(fresh)
					tt.record(gb)
				}
			})
		})
	}

	full := recorded(b)

	b.Run("Rehome", func(b *testing.B) {
		other := &node.Struct{Name: "Row"}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		from, to := symbol.Symbol(recordSubject), symbol.Symbol(other)
		for c.Loop() {
			full.Rehome(from, to)
			from, to = to, from
		}
		assert.Length(b, full.Attachments(), 1, "the attachment remains one")
	})

	b.Run("Scopes", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []plugin.ScopeRecord
		for c.Loop() {
			got = full.Scopes()
		}
		assert.Length(b, got, 1, "Scopes returns the record")
	})

	b.Run("Attachments", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []plugin.Attachment
		for c.Loop() {
			got = full.Attachments()
		}
		assert.Length(b, got, 1, "Attachments returns the record")
	})

	b.Run("StampRecords", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []plugin.StampRecord
		for c.Loop() {
			got = full.StampRecords()
		}
		assert.Length(b, got, 1, "StampRecords returns the record")
	})
}

// goSyntax returns the comment forms of the fixture. They are the line form
// of Go and the C-family block with a star gutter, and the syntax declares
// the tool-directive convention.
func goSyntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line:       []string{"//"},
		Blocks:     []plugin.CommentBlock{{Open: "/**", Close: "*/", Gutter: "*"}},
		Directives: true,
	}
}

// nestedSyntax returns comment forms whose markers begin with each other,
// as the markers of Rust do. The plain line form is first, because the
// render side writes it, and the outer and inner doc line forms follow it.
// A plain block is before a doc block with a star gutter.
func nestedSyntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line: []string{"//", "///", "//!"},
		Blocks: []plugin.CommentBlock{
			{Open: "/*", Close: "*/"},
			{Open: "/**", Close: "*/", Gutter: "*"},
		},
	}
}

// unitTree returns the fixture tree. It contains the member of the unit,
// its shared module file, and a file outside the unit.
func unitTree() fstest.MapFS {
	return fstest.MapFS{
		"svc/store/row.go": {Data: []byte("package store\n")},
		"go.mod":           {Data: []byte("module svc\n")},
		"svc/other/x.go":   {Data: []byte("package other\n")},
	}
}

// unitFile returns the one member of the fixture unit, with the module file
// as its shared input.
func unitFile() plugin.SourceRef {
	return plugin.SourceRef{Path: "svc/store/row.go", Shared: []string{"go.mod"}}
}

// unitOf builds a source unit over the given tree. The unit has one file
// with one shared input, and it loads at full depth.
func unitOf(tb assert.TB, tree fstest.MapFS) *plugin.SourceUnit {
	tb.Helper()

	u, _ := reporting(tb, tree)
	return u
}

// reporting builds the fixture unit together with the sink of its
// findings, so a case can read what the unit reported.
func reporting(tb assert.TB, tree fstest.MapFS) (*plugin.SourceUnit, *diag.Sink) {
	tb.Helper()

	sink := diag.NewSink()
	return plugin.NewSourceUnit(
		[]plugin.SourceRef{unitFile()}, tree, plugin.DepthFull, goSyntax(),
		unitBrand, sink, frontendOrigin,
	), sink
}

// unitOver builds a unit over the fixture tree with the given
// members, depth and syntax.
func unitOver(
	refs []plugin.SourceRef, depth plugin.Depth, syntax plugin.CommentSyntax,
) *plugin.SourceUnit {
	return plugin.NewSourceUnit(refs, unitTree(), depth, syntax, unitBrand, diag.NewSink(), frontendOrigin)
}

// reported returns the findings of a sink in report order. A case that
// compares severities and origins reads them.
func reported(s *diag.Sink) []diag.Diag {
	var out []diag.Diag
	for d := range s.All() {
		out = append(out, d)
	}
	return out
}

// comment splits one raw comment through the fixture unit at line 1 of the
// member.
func comment(tb assert.TB, raw string) plugin.CommentParts {
	tb.Helper()

	return unitOf(tb, unitTree()).Comment(raw, position.Pos{File: "svc/store/row.go", Line: 1})
}

// unitReports returns each reporting method of the unit. Each call reports
// one finding with an argument at the line of the carrier fixture, and each
// entry has the severity of its method.
func unitReports() []unitReport {
	return []unitReport{
		{
			method: "Errorf", want: diag.SeverityError,
			report: func(u *plugin.SourceUnit) { u.Errorf(unitCode, carrierAt, "a %s", "fault") },
		},
		{
			method: "Warnf", want: diag.SeverityWarning,
			report: func(u *plugin.SourceUnit) { u.Warnf(unitCode, carrierAt, "a %s", "fault") },
		},
		{
			method: "Infof", want: diag.SeverityInfo,
			report: func(u *plugin.SourceUnit) { u.Infof(unitCode, carrierAt, "a %s", "fault") },
		},
	}
}

// unitRecords returns one call of each method that records into a
// builder. Each call records one of the record fixtures.
func unitRecords() []unitRecord {
	return []unitRecord{
		{method: "Scope", record: func(gb *plugin.GraphBuilder) { gb.Scope(recordFile, nil) }},
		{method: "Attach", record: func(gb *plugin.GraphBuilder) { gb.Attach(recordSubject, recordRaw) }},
		{method: "Stamp", record: func(gb *plugin.GraphBuilder) { gb.Stamp(recordFile, recordStamp) }},
	}
}

// syntaxErrors returns n syntax errors of the fixture member. They are on
// the lines from line 1, one on each line, and each has the sentence of the
// fixture.
func syntaxErrors(n int) iter.Seq2[position.Pos, string] {
	return func(yield func(position.Pos, string) bool) {
		for i := range n {
			if !yield(position.Pos{File: commentAt.File, Line: i + 1, Col: 1}, syntaxMessage) {
				return
			}
		}
	}
}

// stampedModule returns a setup that builds a fresh unit whose builder has
// stamped the module of the fixtures on recordPackage, and returns the
// builder.
func stampedModule(tb assert.TB) func() *plugin.GraphBuilder {
	return func() *plugin.GraphBuilder {
		gb := unitOf(tb, unitTree()).Graph()
		gb.StampModule(recordPackage, moduleName, moduleRoot)
		return gb
	}
}

// recorded returns a builder with one record of each kind.
func recorded(tb assert.TB) *plugin.GraphBuilder {
	tb.Helper()

	gb := unitOf(tb, unitTree()).Graph()
	for _, r := range unitRecords() {
		r.record(gb)
	}
	return gb
}

// unitRead returns the read of the member of the unit and the read of an
// equal tree. Read allocates no more than the read of the tree.
func unitRead(u *plugin.SourceUnit) storeRead {
	var tree fs.FS = unitTree()
	member := unitFile().Path
	return storeRead{
		name:  "Read",
		own:   func() error { _, err := u.Read(member); return err },
		plain: func() error { _, err := fs.ReadFile(tree, member); return err },
	}
}
