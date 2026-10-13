// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package matrix_test

import (
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance"
	gocorpus "go.dokimi.dev/eidos/conformance/lang/go"
	javacorpus "go.dokimi.dev/eidos/conformance/lang/java"
	"go.dokimi.dev/eidos/conformance/matrix"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// artifactName is the file of the support matrix in the test's artifact
// directory.
const artifactName = "support-matrix.md"

// The target and the plugin's name of the test's backends.
const (
	stubTarget plugin.Target = "stub"
	stubID     plugin.ID     = "stub"
)

// The headings of the document's sections, in their order.
var headings = []string{
	"# Support matrix\n", "\n## Read side\n", "\n## Projection side\n", "\n## Render side\n", "\n## Hub\n",
	"\n## Sugar\n",
}

// The rows that the cases expect, the headers of the satellites' columns
// among them.
const (
	readHeader       = "| Feature | golang | typescript | java | rust | protobuf |\n"
	projectionHeader = "| Capability | golang | typescript | java | rust | protobuf |\n"
	renderHeader     = "| Fact or kind | golang | typescript | java | rust |\n"
	exceptedRow      = "| Value | renders; refuses on Param, Field |\n"
	undeclaredRow    = "| Comment | undeclared |\n"
	refusedRow       = "| Function | refuses |\n"
	renderedRow      = "| Method | renders |\n"
	escapedRow       = "| Bool | `A\\|B` |\n"
	unmarkedRow      = "| golang | refuses | none |\n"
	markedRow        = "| golang | refuses | `@acme.stub` |\n"
	implementedRow   = "| EnumRules | implements |\n"
	absentRow        = "| PresenceRules | absent |\n"
	rulelessRow      = "| EnumRules | none |\n"
)

// The package that declares the optional capabilities, the suffix of the
// name of each, and the one interface with the suffix that is not
// optional. The projection section opens with projectionHeading and ends
// at the next heading, and its rows open with rowOpen.
const (
	rulesPackage      = "go.dokimi.dev/eidos/core/rules"
	capabilitySuffix  = "Rules"
	contractInterface = "SourceRules"
	projectionHeading = "\n## Projection side\n"
	nextHeading       = "\n## "
	rowOpen           = "| "
	cellSep           = " | "
)

// The spelling of the spelling backend, with a pipe, and the marker of
// the marked entry.
const (
	pipedSpelling = "A|B"
	marker        = "@acme.stub"
	refusalReason = "the stub has no functions"
)

// errUnspelled is the refusal of the spoke of the bare backend.
var errUnspelled = errors.New("the stub target spells no type of another language")

// bareBackend is a backend of the target stub. Its spoke refuses every
// shape, and it declares no policy, coverage or refused kind. The methods
// of a rendering backend belong to the nil interface that it embeds,
// which the matrix never calls.
type bareBackend struct{ plugin.Backend }

// Name returns the plugin's name, stub.
func (bareBackend) Name() plugin.ID {
	return stubID
}

// Target returns the target stub.
func (bareBackend) Target() plugin.Target {
	return stubTarget
}

// SpellType refuses every shape.
func (bareBackend) SpellType(rules.TypeShape, plugin.Policy) (*emit.TypeRef, error) {
	return nil, errUnspelled
}

// Policies returns nil.
func (bareBackend) Policies() []plugin.PolicySpec { return nil }

// coveredBackend is a backend of the target stub that declares a
// coverage and the kinds that it refuses.
type coveredBackend struct {
	bareBackend
	coverage render.Coverage
	refused  map[symbol.Kind]string
}

// Coverage returns the backend's coverage.
func (b coveredBackend) Coverage() render.Coverage {
	return b.coverage
}

// RefusedKinds returns the kinds that the backend refuses.
func (b coveredBackend) RefusedKinds() map[symbol.Kind]string {
	return b.refused
}

// spellingBackend is a backend of the target stub whose spoke spells
// every shape as spelling.
type spellingBackend struct {
	bareBackend
	spelling string
}

// SpellType returns a reference with the backend's spelling.
func (b spellingBackend) SpellType(rules.TypeShape, plugin.Policy) (*emit.TypeRef, error) {
	return &emit.TypeRef{Spelling: b.spelling}, nil
}

func TestMarkdown(t *testing.T) {
	t.Parallel()

	t.Run("Markdown", func(t *testing.T) {
		t.Parallel()

		t.Run("renders the matrix of the satellites", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			files.Write(t, t.ArtifactDir(), files.Tree{artifactName: files.Text(string(doc))})
			assert.ContainsInOrder(t, doc, headings, "the document has the five sections in their order")
		})
		t.Run("renders a row for each optional capability of package rules in its order", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Equal(t, projectionRows(t, string(doc)), declaredCapabilities(t),
				"the projection side lists each optional capability that package rules declares")
		})
		t.Run("renders a column for each language on the projection side", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Contains(t, doc, projectionHeader, "the projection side has a column for each satellite")
		})
		t.Run("writes implements for a capability that the rules of a language satisfy", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil)}})
			assert.NoError(t, err, "the matrix of one language renders")
			assert.Contains(t, doc, implementedRow, "the Go rules project an enumeration")
		})
		t.Run("writes absent for a capability that the rules of a language do not satisfy", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil)}})
			assert.NoError(t, err, "the matrix of one language renders")
			assert.Contains(t, doc, absentRow, "the Go rules report no presence")
		})
		t.Run("writes none for a language without rules", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: javacorpus.Corpus(nil)}})
			assert.NoError(t, err, "the matrix of one language renders")
			assert.Contains(t, doc, rulelessRow, "the Java corpus has no rules")
		})
		t.Run("renders a row for each feature of the inventory", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			for _, f := range conformance.Inventory() {
				expect.Contains(t, doc, "| "+f.ID+" | ", "the read side has a row for "+f.ID)
			}
		})
		t.Run("renders a row for each fact of the model", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			for _, f := range symbol.Facts() {
				expect.Contains(t, doc, "| "+f.String()+" | ", "the render side has a row for "+f.String())
			}
		})
		t.Run("renders a column for each language on the read side", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Contains(t, doc, readHeader, "the read side has a column for each satellite")
		})
		t.Run("renders a column for each target with a backend on the render side", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Contains(t, doc, renderHeader, "the render side has a column for each satellite but protobuf")
		})
		t.Run("lists the kinds of each verdict that differs from the base verdict", func(t *testing.T) {
			t.Parallel()

			backend := coveredBackend{coverage: render.Coverage{
				Facts: map[symbol.Fact]render.Verdict{symbol.FactValue: render.Renders},
				Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
					symbol.KindField:    {symbol.FactValue: render.Refuses},
					symbol.KindParam:    {symbol.FactValue: render.Refuses},
					symbol.KindVariable: {symbol.FactValue: render.Renders},
				},
			}}
			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Backend: backend}})
			assert.NoError(t, err, "the matrix of a covered backend renders")
			assert.Contains(t, doc, exceptedRow, "the cell lists the refusing kinds in the model's order")
		})
		t.Run("writes undeclared for a fact without a verdict", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Backend: bareBackend{}}})
			assert.NoError(t, err, "the matrix of a backend without a coverage renders")
			assert.Contains(t, doc, undeclaredRow, "a backend without a coverage does not take a stance on a fact")
		})
		t.Run("writes refuses for a kind that the backend refuses", func(t *testing.T) {
			t.Parallel()

			backend := coveredBackend{refused: map[symbol.Kind]string{symbol.KindFunction: refusalReason}}
			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Backend: backend}})
			assert.NoError(t, err, "the matrix of a refusing backend renders")
			expect.That(t, doc).
				Contains(refusedRow, "the backend refuses functions").
				Contains(renderedRow, "the backend renders methods")
		})
		t.Run("escapes a pipe inside a cell", func(t *testing.T) {
			t.Parallel()

			backend := spellingBackend{spelling: pipedSpelling}
			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Backend: backend}})
			assert.NoError(t, err, "the matrix of a spelling backend renders")
			assert.Contains(t, doc, escapedRow, "a pipe inside a spelling does not end its cell")
		})
		t.Run("writes none as the marker of a language without markers", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil)}})
			assert.NoError(t, err, "the matrix of a language without a backend renders")
			assert.Contains(t, doc, unmarkedRow, "the sugar has no marker for the language")
		})
		t.Run("writes the marker of a language as code", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Marker: marker}})
			assert.NoError(t, err, "the matrix of a language with a marker renders")
			assert.Contains(t, doc, markedRow, "the sugar writes the marker as code")
		})
	})
}

// declaredCapabilities returns the optional capabilities that package
// rules declares: each exported interface whose name ends in Rules, other
// than SourceRules. They are in the order of the files of the package and
// of the declarations of each file.
func declaredCapabilities(tb assert.TB) []string {
	tb.Helper()

	pkg, err := build.Import(rulesPackage, ".", 0)
	assert.NoError(tb, err, "the go command finds package rules")
	fset := token.NewFileSet()
	var names []string
	for _, name := range pkg.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(pkg.Dir, name), nil, parser.SkipObjectResolution)
		assert.NoError(tb, err, "a file of package rules parses")
		for _, decl := range f.Decls {
			gen, isGen := decl.(*ast.GenDecl)
			if !isGen || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, isType := spec.(*ast.TypeSpec)
				if !isType {
					continue
				}
				name := ts.Name.Name
				_, isInterface := ts.Type.(*ast.InterfaceType)
				optional := name != contractInterface && strings.HasSuffix(name, capabilitySuffix)
				if isInterface && ts.Name.IsExported() && optional {
					names = append(names, name)
				}
			}
		}
	}
	assert.NotEmpty(tb, names, "package rules declares optional capabilities")
	return names
}

// projectionRows returns the first cell of each row of the projection side
// of doc, in their order, without the header.
func projectionRows(tb assert.TB, doc string) []string {
	tb.Helper()

	_, section, found := strings.Cut(doc, projectionHeading)
	assert.True(tb, found, "the document has the projection side")
	section, _, _ = strings.Cut(section, nextHeading)
	var rows []string
	for line := range strings.Lines(section) {
		if !strings.HasPrefix(line, rowOpen) {
			continue
		}
		first, _, _ := strings.Cut(strings.TrimPrefix(line, rowOpen), cellSep)
		rows = append(rows, first)
	}
	assert.NotEmpty(tb, rows, "the projection side has rows")
	return rows[1:]
}
