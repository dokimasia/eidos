// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"runtime"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// splitVariable is the environment variable that turns on the split of long
// lines of gofumpt, and splitOn is the value that turns it on.
const (
	splitVariable = "GOFUMPT_SPLIT_LONG_LINES"
	splitOn       = "on"
)

// The allocations of a format. formatAllocs is the format of a file without
// a long line, which gofumpt allocates for its parse, its rules and its
// print. The parse and the print are 71 of them, as go/format allocates
// them. wrapAllocs is the format of wideSource at 100 columns. golines
// allocates about 2,100 of them for the syntax tree of dst, the annotations
// of the long lines and its two formats of the file.
const (
	formatAllocs = 100
	wrapAllocs   = 2_461
)

// groupedSource is a file whose signature repeats the type of two adjacent
// parameters, and groupedFormat is its format.
const (
	groupedSource = "package svc\n\nfunc Join(a string, b string) string {\n\treturn a + b\n}\n"
	groupedFormat = "package svc\n\nfunc Join(a, b string) string {\n\treturn a + b\n}\n"
)

// The parts of the wide source: the file up to the comment of the first
// method, the body of each method, and the signature of each method on one
// line and wrapped. The comment line has 109 columns. The signatures have
// 112, 89 and 123 columns on one line.
const (
	stubHead    = "package ledger\n\nimport \"context\"\n\ntype Stub struct {\n" + stubTags + "}\n\n" + stubComment
	stubComment = "// RecordDifference records a difference between the balance of a ledger and the balance " +
		"of its counterparty.\n"
	stubTags = "\tID   int    `json:\"id\" yaml:\"id\"`\n\tName string `json:\"name\" yaml:\"name\"`\n"
	stubBody = "\treturn \"\", nil\n}\n"

	differenceLine = "func (s *Stub) RecordDifference(ctx context.Context, ledger, counterparty string, " +
		"units int64) (string, error) {\n"
	differenceWrapped = "func (s *Stub) RecordDifference(\n\tctx context.Context,\n" +
		"\tledger, counterparty string,\n\tunits int64,\n) (string, error) {\n"
	recordLine    = "func (s *Stub) Record(ctx context.Context, ledger, counterparty string) (string, error) {\n"
	recordWrapped = "func (s *Stub) Record(\n\tctx context.Context,\n\tledger, counterparty string,\n" +
		") (string, error) {\n"
	outstandingLine = "func (s *Stub) RecordOutstandingDifference(ctx context.Context, ledger, counterparty " +
		"string, units int64) (string, error) {\n"
	outstandingWrapped = "func (s *Stub) RecordOutstandingDifference(\n\tctx context.Context,\n" +
		"\tledger, counterparty string,\n\tunits int64,\n) (string, error) {\n"
)

// wideSource is a file with three methods whose signatures pass 80, 100 or
// 120 columns, a comment line that passes 100 columns and a struct whose
// tags gofumpt and golines leave as they are. Its format without a width is
// wideSource itself. wideAt80, wideAt100 and wideAt120 are its formats at
// those widths, as gofumpt v0.12.0 and golines v0.16.0 write them.
const (
	wideSource = stubHead + differenceLine + stubBody + "\n" + recordLine + stubBody + "\n" +
		outstandingLine + stubBody
	wideAt80 = stubHead + differenceWrapped + stubBody + "\n" + recordWrapped + stubBody + "\n" +
		outstandingWrapped + stubBody
	wideAt100 = stubHead + differenceWrapped + stubBody + "\n" + recordLine + stubBody + "\n" +
		outstandingWrapped + stubBody
	wideAt120 = stubHead + differenceLine + stubBody + "\n" + recordLine + stubBody + "\n" +
		outstandingWrapped + stubBody
)

// tabbedSource is a file whose return statement has 100 bytes and 103
// columns, because golines counts its tab as four columns. tabbedFormat is
// its format at 100 columns.
const (
	tabbedHead = "package svc\n\nimport \"strings\"\n\n" +
		"func Join(first, second, third, fourth, fifth, sixth, separator string) string {\n"
	tabbedSource = tabbedHead +
		"\treturn strings.Join([]string{first, second, third, fourth, fifth, sixth, \"the-seventh\"}, separator)\n}\n"
	tabbedFormat = tabbedHead + "\treturn strings.Join(\n" +
		"\t\t[]string{first, second, third, fourth, fifth, sixth, \"the-seventh\"},\n\t\tseparator,\n\t)\n}\n"
)

// brokenSource is a file that does not parse.
const brokenSource = "package svc\n\nfunc {\n"

// wordWidth is a width that is no number of columns, which a policy of
// another spec than the Go target's selects.
const wordWidth plugin.Choice = "wide"

// TestFormat pins the format of a Go file: the rules of gofumpt with its
// three extra rules, then the wrap of golines at the width of the plan.
func TestFormat(t *testing.T) {
	t.Parallel()

	t.Run("Format", func(t *testing.T) {
		t.Parallel()

		rules := []struct {
			name string
			give string
			want string
		}{
			{
				name: "groups adjacent parameters of one type",
				give: groupedSource,
				want: groupedFormat,
			},
			{
				name: "writes the elements of a composite literal without their type",
				give: "package svc\n\nvar lines = []Line{Line{Text: \"a\"}}\n",
				want: "package svc\n\nvar lines = []Line{{Text: \"a\"}}\n",
			},
			{
				name: "writes an empty struct type on one line",
				give: "package svc\n\ntype Empty struct {\n}\n",
				want: "package svc\n\ntype Empty struct{}\n",
			},
			{
				name: "writes the results of a naked return",
				give: "package svc\n\nfunc Size() (n int) {\n\tn = 1\n\treturn\n}\n",
				want: "package svc\n\nfunc Size() (n int) {\n\tn = 1\n\treturn n\n}\n",
			},
			{
				name: "writes the closing parenthesis of a call on its own line after its arguments",
				give: "package svc\n\nfunc Call() {\n\tprintln(\n\t\t1, 2)\n}\n",
				want: "package svc\n\nfunc Call() {\n\tprintln(\n\t\t1, 2,\n\t)\n}\n",
			},
			{
				name: "writes an octal literal with the prefix 0o",
				give: "package svc\n\nconst mode = 0755\n",
				want: "package svc\n\nconst mode = 0o755\n",
			},
			{
				name: "wraps a line that its tab takes past the width",
				give: tabbedSource,
				want: tabbedFormat,
			},
			{
				name: "wraps each line of code that passes 100 columns for a policy without the width",
				give: wideSource,
				want: wideAt100,
			},
		}
		for _, tt := range rules {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Format([]byte(tt.give), plugin.Policy{})
				assert.NoError(t, err, "the source parses")
				assert.Equal(t, string(got), tt.want, "Format writes the format of gofumpt and golines")
			})
		}

		widths := []struct {
			name      string
			giveWidth plugin.Choice
			want      string
		}{
			{
				name:      "wraps each line of code that passes 80 columns under the width 80",
				giveWidth: golang.Width80,
				want:      wideAt80,
			},
			{
				name:      "wraps each line of code that passes 100 columns under the width 100",
				giveWidth: golang.Width100,
				want:      wideAt100,
			},
			{
				name:      "wraps each line of code that passes 120 columns under the width 120",
				giveWidth: golang.Width120,
				want:      wideAt120,
			},
			{
				name:      "keeps every line under the width none",
				giveWidth: golang.WidthNone,
				want:      wideSource,
			},
		}
		for _, tt := range widths {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Format([]byte(wideSource), widthPolicy(t, tt.giveWidth))
				assert.NoError(t, err, "the source parses")
				assert.Equal(t, string(got), tt.want, "Format wraps at the width of the policy")
			})
		}

		t.Run("keeps a comment line that passes the width", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Format([]byte(wideSource), widthPolicy(t, golang.Width80))
			assert.NoError(t, err, "the source parses")
			assert.Contains(t, string(got), stubComment, "golines wraps no comment")
		})

		t.Run("keeps the keys of each struct tag as written", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Format([]byte(wideSource), widthPolicy(t, golang.Width80))
			assert.NoError(t, err, "the source parses")
			assert.Contains(t, string(got), stubTags, "golines pads no key of a tag into a column")
		})

		t.Run("returns a format that a second call keeps", func(t *testing.T) {
			t.Parallel()

			current := []byte(wideSource)
			assert.Idempotent(t, func(p plugin.Policy) error {
				out, err := backend.Format(current, p)
				current = out
				return err
			}, widthPolicy(t, golang.Width80), func() string { return string(current) },
				"the format of a format is the same format")
		})

		t.Run("returns an error for a source that does not parse", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Format([]byte(brokenSource), plugin.Policy{})
			assert.HasError(t, err, "gofumpt formats a file that parses")
		})

		t.Run("returns an error for a width that is not a number of columns", func(t *testing.T) {
			t.Parallel()

			p, err := plugin.NewPolicy(golang.Target, []plugin.PolicySpec{{
				Key: golang.Width, Choices: []plugin.Choice{wordWidth}, Default: wordWidth,
				Doc: "a width of words", Plan: true,
			}}, nil)
			assert.NoError(t, err, "the policy of another spec resolves")
			_, err = backend.Format([]byte(wideSource), p)
			assert.HasError(t, err, "a width is a number of columns")
			assert.Contains(t, err.Error(), string(wordWidth), "the error contains the width")
		})
	})
}

// TestFormatEnv checks that the format does not depend on the environment.
// It sets the switch of gofumpt, so it does not run in parallel.
func TestFormatEnv(t *testing.T) {
	t.Setenv(splitVariable, splitOn)

	_, err := backend.Format([]byte(wideSource), plugin.Policy{})
	assert.HasError(t, err, "the environment does not change the format of a file")
	assert.Contains(t, err.Error(), splitVariable, "the error names the variable")
}

// Format allocates what gofumpt and golines allocate. The ordinary run does
// not run benchmarks, so it checks those ceilings here. gofumpt reads
// GOFUMPT_SPLIT_LONG_LINES once for each node that it visits. A read of the
// environment allocates twice on Windows, so the test does not run on
// Windows.
func TestFormatAllocs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("gofumpt reads the environment once for each node, and each read allocates on Windows")
	}
	checkAllocs(t, formatCalls())
}

// BenchmarkFormat measures the format of a file that gofumpt alone formats
// and of a file whose long lines golines wraps.
func BenchmarkFormat(b *testing.B) {
	benchCalls(b, formatCalls())
}

// formatCalls returns the calls of Format for a file without a long line
// and for wideSource.
func formatCalls() []allocCall {
	grouped := []byte(groupedSource)
	wide := []byte(wideSource)
	var got []byte
	return []allocCall{
		{
			name: "Format", caseName: "a file without a long line", allocs: formatAllocs,
			call: func() { got, _ = backend.Format(grouped, plugin.Policy{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, string(got), groupedFormat, "Format groups the parameters")
			},
		},
		{
			name: "Format", caseName: "a file with long lines", allocs: wrapAllocs,
			call: func() { got, _ = backend.Format(wide, plugin.Policy{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, string(got), wideAt100, "Format wraps the long lines")
			},
		},
	}
}

// widthPolicy returns the resolved policy of the Go target that selects
// width.
func widthPolicy(tb assert.TB, width plugin.Choice) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(golang.Target, golang.Policies(),
		map[plugin.PolicyKey]plugin.Choice{golang.Width: width})
	assert.NoError(tb, err, "the Go target declares the width")
	return p
}
