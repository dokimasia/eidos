// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The cross-language fixture's tree, its goldens, and the TypeScript file
// that it generates beside the Go double.
const (
	hubTree    = "testdata/hub/tree"
	hubWant    = "testdata/hub/want/"
	clientPath = "svc/store.ts"
)

// The lines of the store that the cases edit, the carriers that they write
// above those lines, the numbers of the lines after the edit, and the
// mode of a file that a case writes.
const (
	expiresLine    = "\tExpires int64\n"
	putLine        = "\tPut(ctx context.Context, s Session) error\n"
	hubGetByKey    = "\tGet(ctx context.Context, key string) (Session, error)\n"
	hubGetByID     = "\tGet(ctx context.Context, id string) (Session, error)\n"
	asNumber       = "\t//+acme:typescript int64=number\n"
	asString       = "\t//+acme:typescript int64=string\n"
	asLong         = "\t//+acme:typescript int64=long\n"
	putAsSave      = "\t//+acme:typescript name=save\n"
	expiresCarrier = 8
	putCarrier     = 16
	putLineNumber  = 17
	caseFileMode   = 0o600
)

// The members of the client that the policy and explain cases read.
const (
	expiresBigInt = "  expires: bigint;\n"
	expiresString = "  expires: string;\n"
	expiresNumber = "  expires: number;\n"
	idString      = "  id: string;\n"
	saveMethod    = "  save(s: Session): Promise<void>;\n"
	putMethod     = "  put(s: Session): Promise<void>;\n"
)

// The declarations that the refusal and visibility cases append to the
// store, and the lines of their members.
const (
	channels = "\n// Feed streams the events of a store.\ntype Feed struct {\n\tEvents chan int\n}\n" +
		"\n// Sender sends the events of a store.\ntype Sender interface {\n\tSend(events chan int) error\n}\n" +
		"\n// Receiver receives the events of a store.\ntype Receiver interface {\n\tReceive() chan int\n}\n"
	unexported = "\n// Token is one credential of a session.\ntype Token struct {\n\tValue  string\n\tsecret string\n}\n" +
		"\n// Signer signs tokens.\ntype Signer interface {\n\tSign(t Token) string\n\trotate()\n}\n"
	eventsLine  = 21
	sendLine    = 26
	receiveLine = 31
)

// The members of the client of the visibility case.
const (
	tokenValue = "  value: string;\n"
	tokenSign  = "  sign(t: Token): Promise<string>;\n"
	secret     = "secret"
	rotate     = "rotate"
)

// The key that the explain case explains, the lowering entry of the
// TypeScript target, and the values of the two claims on the key.
const (
	nameKey                = "typescript.name"
	tsLowering   plugin.ID = "typescript-lowering"
	savedName              = "save"
	respelledPut           = "put"
)

// storeLine is the line of Store in the store of the cross-language
// fixture. The layout refuses the client of Store there when the plan
// does not emit Session.
const storeLine = 14

// TestHub checks that one run over a Go workspace writes a Go double and
// a TypeScript client from one graph.
func TestHub(t *testing.T) {
	t.Parallel()

	t.Run("ComposeHub", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the workspace suite over a Go double and a TypeScript client", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWorkspaceSuite(t, hubFixture(t))
		})

		t.Run("passes the warm suite over an edit that renames a parameter of Get", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWarmColdSuite(t, hubFixture(t))
		})

		t.Run("writes a client that tsc type-checks", func(t *testing.T) {
			t.Parallel()

			adapter := tstesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			root := hubRoot(t, "", "")
			client := cleanClient(t, root, workspace.Config{})
			toolchain.AssertTypeChecks(t.Context(), t, adapter, toolchain.Generated{
				Files: map[string][]byte{clientPath: []byte(client)},
			})
		})
	})

	t.Run("HubPlans", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a 64-bit integer as a bigint without a selection", func(t *testing.T) {
			t.Parallel()

			client := cleanClient(t, hubRoot(t, "", ""), workspace.Config{})
			assert.Contains(t, client, expiresBigInt, "the default of typescript.int64 is bigint")
		})

		t.Run("writes a 64-bit integer as a string under the config's selection", func(t *testing.T) {
			t.Parallel()

			client := cleanClient(t, hubRoot(t, "", ""), workspace.Config{
				Policies: map[plugin.PolicyKey]plugin.Choice{typescript.Int64: typescript.String},
			})
			assert.Contains(t, client, expiresString, "the selection applies")
		})

		t.Run("writes a 64-bit integer as a number under a directive on the field", func(t *testing.T) {
			t.Parallel()

			client := cleanClient(t, hubRoot(t, expiresLine, asNumber), workspace.Config{})
			expect.Contains(t, client, expiresNumber, "the directive overrides the policy of the field")
			expect.Contains(t, client, idString, "every other member keeps its spelling")
		})

		t.Run("writes after an edit of the directive what a cold run over the edited tree writes", func(t *testing.T) {
			t.Parallel()

			warm := hubRoot(t, expiresLine, asNumber)
			w := hubBuild(t, warm, workspace.Config{})
			_, err := hubRun(t, w, warm)
			assert.NoError(t, err, "the cold run is clean")
			editStore(t, warm, asNumber, asString)
			report, err := hubRun(t, w, warm)
			assert.NoError(t, err, "the warm run is clean")
			assert.False(t, report.Stats.Cold, "the run after the edit reads the sealed state")
			cold := cleanClient(t, hubRoot(t, expiresLine, asString), workspace.Config{})
			assert.Equal(t, readClient(t, warm), cold, "the warm run writes the client of the cold run")
		})

		t.Run("returns an error from Build for a selection outside the key's choices", func(t *testing.T) {
			t.Parallel()

			_, err := golang.ComposeHub(t.TempDir()).Plans(golang.HubPlans()...).Config(workspace.Config{
				Policies: map[plugin.PolicyKey]plugin.Choice{typescript.Int64: "long"},
			}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `policy typescript.int64 takes one of bigint, string, number, not "long"`,
				"the error lists the choices")
		})

		t.Run("reports BadSpelling at a directive with a choice outside the key's choices", func(t *testing.T) {
			t.Parallel()

			root := hubRoot(t, expiresLine, asLong)
			report, err := hubRun(t, hubBuild(t, root, workspace.Config{}), root)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused choice fails the run")
			refused := findingsOf(report, directive.BadSpelling)
			assert.Length(t, refused, 1, "the run reports the one refused choice")
			assert.Equal(t, refused[0].Pos, position.Pos{File: storeFile, Line: expiresCarrier, Col: 2},
				"the finding is at the carrier")
		})

		t.Run("leaves out the unexported members of a struct and an interface", func(t *testing.T) {
			t.Parallel()

			client := cleanClientWith(t, unexported)
			expect.Contains(t, client, tokenValue, "the exported field is a property")
			expect.Contains(t, client, tokenSign, "the exported method is an async method")
			expect.NotContains(t, client, secret, "the unexported field is left out")
			expect.NotContains(t, client, rotate, "the unexported method is left out")
		})

		refusals := []struct {
			name string
			line int
		}{
			{name: "reports RefusedType at a field of a channel", line: eventsLine},
			{name: "reports RefusedType at a method with a parameter of a channel", line: sendLine},
			{name: "reports RefusedType at a method with a result of a channel", line: receiveLine},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := hubRoot(t, "", "")
				appendStore(t, root, channels)
				report, err := hubRun(t, hubBuild(t, root, workspace.Config{}), root)
				assert.ErrorIs(t, err, workspace.ErrRunFailed, "a refused type fails the plan")
				var lines []int
				for _, d := range findingsOf(report, eidos.RefusedType) {
					expect.Equal(t, d.Pos.File, storeFile, "the refusal is in the store")
					lines = append(lines, d.Pos.Line)
				}
				assert.Contains(t, lines, tt.line, "the refusal is at the declaration with the type")
			})
		}

		t.Run("writes the name of a directive on Put", func(t *testing.T) {
			t.Parallel()

			client := cleanClient(t, hubRoot(t, putLine, putAsSave), workspace.Config{})
			expect.Contains(t, client, saveMethod, "the directive names the method save")
			expect.NotContains(t, client, putMethod, "the method is not named put")
		})

		t.Run("explains the name of Put with the directive's claim first", func(t *testing.T) {
			t.Parallel()

			root := hubRoot(t, putLine, putAsSave)
			w := hubBuild(t, root, workspace.Config{})
			_, err := hubRun(t, w, root)
			assert.NoError(t, err, "the run is clean")
			target, err := w.ParseTarget(fmt.Sprintf("%s@%s:%d", nameKey, storeFile, putLineNumber))
			assert.NoError(t, err, "the target parses")
			explained, err := w.Explain(t.Context(), target)
			assert.NoError(t, err, "the generation explains the name")
			assert.Length(t, explained.Claims, 2, "the name has the directive's claim and the lowering's claim")
			first, second := explained.Claims[0], explained.Claims[1]
			expect.Equal(t, first.Value, any(savedName), "the directive's name ranks first")
			expect.Equal(t, first.Claim.Authority, meta.AuthorityDirective, "the first claim has directive authority")
			expect.Equal(t, first.Claim.Pos.Line, putCarrier, "the first claim has the carrier's position")
			expect.True(t, first.Winner, "the directive's claim wins")
			expect.Equal(t, second.Value, any(respelledPut), "the lowering's claim is the respelled name")
			expect.Equal(t, second.Claim.Authority, meta.AuthorityPlugin, "the second claim has plugin authority")
			expect.Equal(t, second.Claim.Plugin, tsLowering, "the lowering entry makes the second claim")
		})
	})

	t.Run("EditHub", func(t *testing.T) {
		t.Parallel()

		t.Run("renames the parameter of Get in the store", func(t *testing.T) {
			t.Parallel()

			root := hubRoot(t, "", "")
			want := filesUnder(t, root)
			want[storeFile] = strings.Replace(want[storeFile], hubGetByKey, hubGetByID, 1)
			assert.NoError(t, golang.EditHub(root), "the edit applies")
			assert.Equal(t, filesUnder(t, root), want, "the edit changes the line of Get and no other line")
		})

		t.Run("returns an error for a store whose Get has no parameter key", func(t *testing.T) {
			t.Parallel()

			root := hubRoot(t, "", "")
			assert.NoError(t, golang.EditHub(root), "the first edit applies")
			err := golang.EditHub(root)
			assert.HasError(t, err, "a second edit finds no parameter key to rename")
			assert.Contains(t, err.Error(), storeFile, "the error contains the file")
		})

		t.Run("returns fs.ErrNotExist for a root without the store", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, golang.EditHub(t.TempDir()), fs.ErrNotExist, "an empty root has no store to read")
		})
	})

	t.Run("StoreClient", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UntranslatedReference at Store for a client of the interface alone", func(t *testing.T) {
			t.Parallel()

			root := hubRoot(t, "", "")
			plans := golang.HubPlans()
			plans[1].Generators = []plugin.Generator{golang.StoreClient()}
			w, err := golang.ComposeHub(root).Plans(plans...).Build()
			assert.NoError(t, err, "the composition builds")
			report, err := hubRun(t, w, root)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the untranslated reference fails the plan")
			refused := findingsOf(report, layout.UntranslatedReference)
			assert.Length(t, refused, 1, "the layout refuses the client of Store")
			assert.Equal(t, refused[0].Pos.Line, storeLine, "the finding is at the origin of the client")
		})
	})
}

// hubFixture returns the workspace suite's fixture of the cross-language
// composition: the tree, the standard library's context package, the
// composition, its two plans, the two files that they generate, and the
// edit that renames a parameter of Get.
func hubFixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	want := map[string][]byte{}
	for _, path := range []string{stubFile, clientPath} {
		b, err := os.ReadFile(filepath.FromSlash(hubWant + path))
		assert.NoError(t, err, "the golden of "+path+" reads")
		want[path] = b
	}
	return workspacetest.Fixture{
		Tree:    os.DirFS(hubTree),
		Stores:  map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(pipelineRoot)},
		Compose: golang.ComposeHub,
		Plans:   golang.HubPlans,
		Want:    want,
		Edit:    golang.EditHub,
	}
}

// hubRoot copies the cross-language fixture's tree into a new directory,
// writes carrier above the first line of the store that equals line, and
// returns the directory. An empty line leaves the store as it is.
func hubRoot(t *testing.T, line, carrier string) string {
	t.Helper()

	root := t.TempDir()
	copyTree(t, root, hubTree)
	if line != "" {
		editStore(t, root, line, carrier+line)
	}
	return root
}

// hubBuild builds the cross-language composition over root, with its two
// plans and cfg.
func hubBuild(t *testing.T, root string, cfg workspace.Config) *workspace.Workspace {
	t.Helper()

	w, err := golang.ComposeHub(root).Plans(golang.HubPlans()...).Config(cfg).Build()
	assert.NoError(t, err, "the composition builds")
	return w
}

// hubRun runs w over the tree at root, with the standard library's
// context package, and returns the run's report and error.
func hubRun(t *testing.T, w *workspace.Workspace, root string) (*workspace.Report, error) {
	t.Helper()

	return w.Run(t.Context(), workspace.Input{
		Tree:   os.DirFS(root),
		Stores: map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(pipelineRoot)},
	})
}

// cleanClient runs the cross-language composition over root under cfg,
// fails the test where the run is not clean, and returns the client that
// the run wrote. Each Error of the run fails the test on a line of its
// own, before the run's error does.
func cleanClient(t *testing.T, root string, cfg workspace.Config) string {
	t.Helper()

	report, err := hubRun(t, hubBuild(t, root, cfg), root)
	for d := range report.Sink.All() {
		expect.NotEqual(t, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s at %s is not an Error of the run: %s", d.Code, d.Pos, d.Msg))
	}
	assert.NoError(t, err, "the run is clean")
	return readClient(t, root)
}

// cleanClientWith runs the cross-language composition over the fixture's
// tree with decls appended to the store, and returns the client that the
// clean run wrote.
func cleanClientWith(t *testing.T, decls string) string {
	t.Helper()

	root := hubRoot(t, "", "")
	appendStore(t, root, decls)
	return cleanClient(t, root, workspace.Config{})
}

// readClient returns the client that a run wrote under root.
func readClient(t *testing.T, root string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(clientPath)))
	assert.NoError(t, err, "the client reads")
	return string(b)
}

// editStore replaces the first occurrence of from with to in the store
// under root.
func editStore(t *testing.T, root, from, to string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(storeFile))
	b, err := os.ReadFile(path)
	assert.NoError(t, err, "the store reads")
	assert.Contains(t, string(b), from, "the store contains the line that the case edits")
	assert.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(b), from, to, 1)), caseFileMode),
		"the store writes")
}

// appendStore appends decls to the store under root.
func appendStore(t *testing.T, root, decls string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(storeFile))
	b, err := os.ReadFile(path)
	assert.NoError(t, err, "the store reads")
	assert.NoError(t, os.WriteFile(path, append(b, decls...), caseFileMode), "the store writes")
}

// findingsOf returns the findings of a run under one code, in the sink's
// order.
func findingsOf(report *workspace.Report, code diag.Code) []diag.Diag {
	var out []diag.Diag
	for d := range report.Sink.All() {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}
