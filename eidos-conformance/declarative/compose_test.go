// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/conformance/declarative"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	host "go.dokimi.dev/eidos/declarative"
	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The tree of the fixture, the goldens of the files that its plans write,
// and the plugin directory inside the tree.
const (
	treeDir   = "testdata/tree"
	wantDir   = "testdata/want"
	pluginDir = "plugins/acme-audit"
)

// The interface Ledger of the tree, its package and its file, and the
// instance of the plugin's directive on it.
const (
	svcPackage                 = "example.com/acme/svc"
	ledgerName                 = "Ledger"
	ledgerFile                 = "svc/ledger.go"
	brand                      = "acme"
	auditName   directive.Name = "acme-audit:audit"
	levelKey                   = "level"
	sinkKey                    = "sink"
	infoLevel                  = "info"
	recordSink                 = "record"
	carrierLine                = 5
)

// The params of the carrier of Ledger that the validation cases replace,
// and the mode of the file that an edit writes.
const (
	infoParam    = "level=info"
	traceParam   = "level=trace"
	recordParam  = "sink=record"
	missingParam = "sink=missing"
	caseFileMode = 0o600
)

// goSuffix is the suffix of the generated Go files that the toolchain
// cases check.
const goSuffix = ".go"

// written lists the files that a run of the fixture writes, in the order of
// their paths.
var written = []string{
	"ops/transfer.audit.ts",
	"ops/transfer_audit.go",
	"svc/ledger_audit.go",
	"svc/ledger_audit_test.go",
}

// The binary of the fixture opens its tree through the config file, which
// adds the declarative plugin to both plans, and each run writes the files
// that the templates of the plugin describe.
func TestCompose(t *testing.T) {
	t.Parallel()

	t.Run("Compose", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the wanted files on a cold run", func(t *testing.T) {
			t.Parallel()

			root := treeCopy(t)
			report := cleanRun(t, root)
			assert.True(t, report.Stats.Cold, "the first run is cold")
			assert.Equal(t, paths(report), written, "the plans write the files of the fixture")
			for _, p := range written {
				want := filepath.Join(wantDir, filepath.FromSlash(p))
				golden.MatchAt(t, want, read(t, root, p), golden.ShouldUpdate())
			}
		})

		t.Run("writes the files of the cold run on a warm run", func(t *testing.T) {
			t.Parallel()

			root := treeCopy(t)
			cleanRun(t, root)
			var report *workspace.Report
			assert.Pure(t, func() map[string]string { return contents(t, root) },
				func() { report = cleanRun(t, root) }, "the warm run writes the files of the cold run")
			assert.False(t, report.Stats.Cold, "the second run reads the sealed state")
		})

		t.Run("reports the findings of the cold run on a warm run", func(t *testing.T) {
			t.Parallel()

			root := treeCopy(t)
			cold := cleanRun(t, root)
			warm := cleanRun(t, root)
			assert.False(t, warm.Stats.Cold, "the second run reads the sealed state")
			assert.Equal(t, findings(warm), findings(cold), "the warm run reports the findings of the cold run")
		})

		t.Run("writes Go that go vet passes", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, gotesting.New()) {
				return
			}
			root := treeCopy(t)
			cleanRun(t, root)
			gotesting.AssertVets(t.Context(), t, workspacetest.Generated(t, root, brand, os.DirFS(treeDir), goSuffix))
		})

		t.Run("writes Go tests that pass", func(t *testing.T) {
			t.Parallel()

			adapter := gotesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			root := treeCopy(t)
			cleanRun(t, root)
			toolchain.AssertTestsPass(t.Context(), t, adapter,
				workspacetest.Generated(t, root, brand, os.DirFS(treeDir), goSuffix))
		})

		refusals := []struct {
			name string
			old  string
			new  string
			code diag.Code
		}{
			{
				name: "reports BadSpelling at the carrier for a level outside the choices",
				old:  infoParam,
				new:  traceParam,
				code: directive.BadSpelling,
			},
			{
				name: "reports UnresolvedReference at the carrier for a sink that refers to no function",
				old:  recordParam,
				new:  missingParam,
				code: directive.UnresolvedReference,
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := treeCopy(t)
				edit(t, root, tt.old, tt.new)
				report, err := run(t, root)
				assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused instance fails the run")
				var refused []diag.Diag
				for d := range report.Sink.All() {
					if d.Code == tt.code {
						refused = append(refused, d)
					}
				}
				assert.Length(t, refused, 1, "the run reports the one refused instance")
				assert.Equal(t, refused[0].Pos.File, ledgerFile, "the finding is in the file of Ledger")
				assert.Equal(t, refused[0].Pos.Line, carrierLine, "the finding is at the carrier")
			})
		}

		t.Run("adds a plugin that passes the plugin suite", func(t *testing.T) {
			t.Parallel()

			dir, err := filepath.Abs(filepath.Join(treeDir, filepath.FromSlash(pluginDir)))
			assert.NoError(t, err, "the plugin directory resolves")
			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				g, err := host.Load(dir)
				assert.NoError(tb, err, "the plugin directory loads")
				return g, svcFixture(ctx, tb)
			})
		})
	})
}

// treeCopy copies the tree of the fixture into a new directory of the test,
// and returns the directory.
func treeCopy(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	assert.NoError(t, os.CopyFS(root, os.DirFS(treeDir)), "the tree copies")
	return root
}

// run opens the workspace of the tree at root through cli.Open, as the
// binary of the fixture opens it, and returns the report and the error of
// one run over the tree.
func run(t *testing.T, root string) (*workspace.Report, error) {
	t.Helper()

	stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: root, Getenv: os.Getenv}
	members, err := cli.Open(stdio, cli.Flags{}, declarative.Compose)
	assert.NoError(t, err, "the workspace opens")
	assert.Length(t, members, 1, "the config file configures one workspace")
	return members[0].Workspace.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
}

// cleanRun runs the workspace of the tree at root, and fails the test where
// the run reports an Error or returns an error. Each Error fails the test on
// a line of its own, before the error of the run does.
func cleanRun(t *testing.T, root string) *workspace.Report {
	t.Helper()

	report, err := run(t, root)
	for d := range report.Sink.All() {
		expect.NotEqual(t, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s at %s is not an Error of the run: %s", d.Code, d.Pos, d.Msg))
	}
	assert.NoError(t, err, "the run is clean")
	return report
}

// paths returns the path of each file that the record of report lists, in
// the order of the record.
func paths(report *workspace.Report) []string {
	out := make([]string, 0, len(report.Manifest.Files))
	for _, e := range report.Manifest.Files {
		out = append(out, e.Path)
	}
	return out
}

// read returns the bytes of the file at the slash-separated path p below
// root.
func read(t *testing.T, root, p string) []byte {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	assert.NoError(t, err, "the file "+p+" reads")
	return b
}

// contents returns the text of each file of written below root, by its
// path.
func contents(t *testing.T, root string) map[string]string {
	t.Helper()

	out := make(map[string]string, len(written))
	for _, p := range written {
		out[p] = string(read(t, root, p))
	}
	return out
}

// findings returns each finding of report with its code, its position and
// its message, in the order of the sink.
func findings(report *workspace.Report) []string {
	var out []string
	for d := range report.Sink.All() {
		out = append(out, fmt.Sprintf("%s %s %s", d.Code, d.Pos, d.Msg))
	}
	return out
}

// edit replaces from with to in the file of Ledger below root.
func edit(t *testing.T, root, from, to string) {
	t.Helper()

	file := filepath.Join(root, filepath.FromSlash(ledgerFile))
	src := string(read(t, root, ledgerFile))
	assert.Contains(t, src, from, "the file of Ledger contains the text that the case replaces")
	assert.NoError(t, os.WriteFile(file, []byte(strings.Replace(src, from, to, 1)), caseFileMode),
		"the file of Ledger writes")
}

// svcFixture returns a fixture of the package svc of the tree, as the Go
// frontend loads it under ctx, with the Go rules, the target golang, and
// the validated instance of the plugin's directive on Ledger.
func svcFixture(ctx context.Context, tb assert.TB) *plugintest.Fixture {
	tb.Helper()

	g, _, err := load.Load(ctx, load.Config{
		FS:        os.DirFS(treeDir),
		Frontends: []plugin.Frontend{gofrontend.New(nil)},
		Sink:      diag.NewSink(),
		Brand:     brand,
	})
	assert.NoError(tb, err, "the tree loads")
	f := plugintest.New(tb)
	f.Rules = rules.NewRegistry()
	assert.NoError(tb, f.Rules.Register(gorules.New()), "the Go rules register")
	f.Target = golang.Target
	var ledger symbol.Identity
	for pkg := range g.Packages() {
		if pkg.ID.Package != svcPackage {
			continue
		}
		f.Load(tb, pkg)
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				if i, is := decl.(*node.Interface); is && i.Name == ledgerName {
					ledger = i.ID
				}
			}
		}
	}
	assert.NotEqual(tb, ledger, symbol.Identity{}, "the package svc declares Ledger")
	f.Validated(tb, ledger, directive.Directive{
		Name: auditName,
		Params: map[directive.ParamKey]directive.Value{
			levelKey: {Kind: directive.TypeString, Str: infoLevel},
			sinkKey:  {Kind: directive.TypeReference, Ref: recordSink},
		},
		Pos: position.Pos{File: ledgerFile, Line: carrierLine, Col: 1},
	})
	return f
}
