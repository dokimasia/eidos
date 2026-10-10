// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/protobuf"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The service fixture's tree, whose directories are the versions of the
// schema, the directory of the files that every version writes, the
// version that the cases of one version read, the schema, the files that
// the plans write, and the module that the server's package path starts
// with.
const (
	serviceTree   = "testdata/service"
	serviceWant   = "testdata/service/want"
	baseVersion   = "proto3"
	schemaPath    = "svc/store.proto"
	serverPath    = "svc/store_server.go"
	clientPath    = "svc/store.ts"
	serviceModule = "example.com/acme"
)

// versions are the versions of the service schema, each a directory of
// the fixture's tree.
var versions = []string{"proto2", "proto3", "edition2023", "edition2024", "edition2026"}

// The lines of the schema that the cases edit, the lines that they write,
// and the line and the column of a field that a case adds to Summary.
const (
	summaryOpens   = "message Summary {\n"
	wideSize       = "int64 size = 1;"
	narrowSize     = "int32 size = 1;"
	nullField      = "  google.protobuf.NullValue nothing = 2;\n"
	nullLine       = 46
	nullColumn     = 3
	stateOpens     = "  enum State {\n"
	phaseCarrier   = "  //+acme:typescript name=SessionPhase\n"
	stateLine      = 13
	carriedLine    = 14
	carrierLine    = 13
	edition2024    = `edition = "2024";`
	edition2025    = `edition = "2025";`
	unreleasedTree = "edition2024"
)

// The members of the client that the policy and the name cases read.
const (
	lifetimeNumber = "  lifetime?: number | undefined;\n"
	phaseEnum      = "export enum SessionPhase {\n"
	phaseState     = "  state: SessionPhase;\n"
	stateEnum      = "export enum SessionState {\n"
)

// The key that the explain cases explain, the lowering entry of the
// TypeScript target, and the values of the claims on the key.
const (
	nameKey              = "typescript.name"
	tsLowering plugin.ID = "typescript-lowering"
	flatName             = "SessionState"
	phaseName            = "SessionPhase"
)

// TestService checks that one workspace over a proto service writes Go
// server scaffolding and TypeScript client types, the same files from the
// schema in each protobuf version.
func TestService(t *testing.T) {
	t.Parallel()

	t.Run("ComposeService", func(t *testing.T) {
		t.Parallel()

		for _, version := range versions {
			t.Run("passes the workspace suite over the "+version+" schema", func(t *testing.T) {
				t.Parallel()

				workspacetest.RunWorkspaceSuite(t, serviceFixture(t, version))
			})

			t.Run("passes the warm suite over the "+version+" schema with a field of Summary narrowed",
				func(t *testing.T) {
					t.Parallel()

					workspacetest.RunWarmColdSuite(t, serviceFixture(t, version))
				})
		}

		t.Run("writes a client that tsc type-checks", func(t *testing.T) {
			t.Parallel()

			adapter := tstesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			root := serviceRoot(t, baseVersion)
			cleanRun(t, root, workspace.Config{})
			toolchain.AssertTypeChecks(t.Context(), t, adapter, toolchain.Generated{
				Files: map[string][]byte{
					clientPath: []byte(files.Read(t, filepath.Join(root, filepath.FromSlash(clientPath)))),
				},
			})
		})

		t.Run("writes a server that the Go toolchain type-checks", func(t *testing.T) {
			t.Parallel()

			adapter := gotesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			root := serviceRoot(t, baseVersion)
			cleanRun(t, root, workspace.Config{})
			toolchain.AssertTypeChecks(t.Context(), t, adapter, toolchain.Generated{
				Files: map[string][]byte{
					serverPath: []byte(files.Read(t, filepath.Join(root, filepath.FromSlash(serverPath)))),
				},
				Module: serviceModule,
			})
		})
	})

	t.Run("ServicePlans", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a duration as a number under the config's selection", func(t *testing.T) {
			t.Parallel()

			root := serviceRoot(t, baseVersion)
			cleanRun(t, root, workspace.Config{
				Policies: map[plugin.PolicyKey]plugin.Choice{typescript.Duration: typescript.Number},
			})
			client := files.Read(t, filepath.Join(root, filepath.FromSlash(clientPath)))
			assert.Contains(t, client, lifetimeNumber, "the selection applies to the well-known duration")
		})

		t.Run("reports RefusedType at a field of NullValue in each plan", func(t *testing.T) {
			t.Parallel()

			root := serviceRoot(t, baseVersion)
			editSchema(t, root, summaryOpens, summaryOpens+nullField)
			report, err := serviceRun(t, serviceBuild(t, root, workspace.Config{}), root)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "a refused type fails the plans")
			refused := findingsOf(report, eidos.RefusedType)
			assert.Length(t, refused, 2, "the server and the client each refuse the field")
			for _, d := range refused {
				expect.Equal(t, d.Pos, position.Pos{File: schemaPath, Line: nullLine, Col: nullColumn},
					"the refusal is at the field")
			}
		})

		t.Run("explains the name of Session.State with the lowering's claim", func(t *testing.T) {
			t.Parallel()

			root := serviceRoot(t, baseVersion)
			explained := explainState(t, root, stateLine)
			assert.Length(t, explained.Claims, 1, "the name has the lowering's claim alone")
			claim := explained.Claims[0]
			expect.Equal(t, claim.Value, any(flatName), "the lowering's name is the flat name")
			expect.Equal(t, claim.Claim.Authority, meta.AuthorityPlugin, "the claim has plugin authority")
			expect.Equal(t, claim.Claim.Plugin, tsLowering, "the lowering entry makes the claim")
		})

		t.Run("writes the name of a directive on Session.State", func(t *testing.T) {
			t.Parallel()

			root := serviceRoot(t, baseVersion)
			editSchema(t, root, stateOpens, phaseCarrier+stateOpens)
			cleanRun(t, root, workspace.Config{})
			client := files.Read(t, filepath.Join(root, filepath.FromSlash(clientPath)))
			expect.Contains(t, client, phaseEnum, "the directive names the enum")
			expect.Contains(t, client, phaseState, "a reference to the enum follows the name")
			expect.NotContains(t, client, stateEnum, "the enum is not named after its flat name")
		})

		t.Run("explains the name of Session.State with the directive's claim first", func(t *testing.T) {
			t.Parallel()

			root := serviceRoot(t, baseVersion)
			editSchema(t, root, stateOpens, phaseCarrier+stateOpens)
			explained := explainState(t, root, carriedLine)
			assert.Length(t, explained.Claims, 2, "the name has the directive's claim and the lowering's claim")
			first, second := explained.Claims[0], explained.Claims[1]
			expect.Equal(t, first.Value, any(phaseName), "the directive's name ranks first")
			expect.Equal(t, first.Claim.Authority, meta.AuthorityDirective, "the first claim has directive authority")
			expect.Equal(t, first.Claim.Pos.Line, carrierLine, "the first claim has the carrier's position")
			expect.True(t, first.Winner, "the directive's claim wins")
			expect.Equal(t, second.Value, any(flatName), "the lowering's claim is the flat name")
			expect.Equal(t, second.Claim.Plugin, tsLowering, "the lowering entry makes the second claim")
		})

		t.Run("reports UnknownEdition for an edition outside the table", func(t *testing.T) {
			t.Parallel()

			root := serviceRoot(t, unreleasedTree)
			editSchema(t, root, edition2024, edition2025)
			report, err := serviceRun(t, serviceBuild(t, root, workspace.Config{}), root)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the unknown edition fails the run")
			assert.Length(t, findingsOf(report, protofrontend.UnknownEdition), 1, "the statement reports once")
		})

		t.Run("writes no declaration of a file of an edition outside the table", func(t *testing.T) {
			t.Parallel()

			root := serviceRoot(t, unreleasedTree)
			editSchema(t, root, edition2024, edition2025)
			_, err := serviceRun(t, serviceBuild(t, root, workspace.Config{}), root)
			assert.HasError(t, err, "the unknown edition fails the run")
			files.Absent(t, filepath.Join(root, filepath.FromSlash(serverPath)), "the server plan writes no file")
			files.Absent(t, filepath.Join(root, filepath.FromSlash(clientPath)), "the client plan writes no file")
		})
	})

	t.Run("EditService", func(t *testing.T) {
		t.Parallel()

		for _, version := range versions {
			t.Run("narrows the field size of Summary in the "+version+" schema", func(t *testing.T) {
				t.Parallel()

				root := serviceRoot(t, version)
				src := files.Read(t, filepath.Join(root, filepath.FromSlash(schemaPath)))
				want := strings.Replace(src, wideSize, narrowSize, 1)
				assert.NoError(t, protobuf.EditService(root), "the edit applies")
				files.Equal(t, os.DirFS(root), files.Tree{schemaPath: files.Text(want)},
					"the edit narrows the field and changes no other line")
			})
		}

		t.Run("returns an error for a schema without the field size", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{schemaPath: files.Text("syntax = \"proto3\";\n")})
			err := protobuf.EditService(root)
			assert.HasError(t, err, "a schema without the field has nothing to narrow")
			assert.Contains(t, err.Error(), schemaPath, "the error contains the schema")
		})

		t.Run("returns fs.ErrNotExist for a root without the schema", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, protobuf.EditService(t.TempDir()), fs.ErrNotExist, "an empty root has no schema to read")
		})
	})
}

// serviceFixture returns the workspace suite's fixture of the service
// composition over the schema of one version: the version's tree, the
// composition, its two plans, the two files that every version writes,
// and the edit that narrows a field of Summary.
func serviceFixture(t *testing.T, version string) workspacetest.Fixture {
	t.Helper()

	want := map[string][]byte{}
	for _, path := range []string{serverPath, clientPath} {
		want[path] = []byte(files.Read(t, filepath.Join(filepath.FromSlash(serviceWant), filepath.FromSlash(path))))
	}
	return workspacetest.Fixture{
		Tree:    os.DirFS(filepath.Join(filepath.FromSlash(serviceTree), version)),
		Compose: protobuf.ComposeService,
		Plans:   protobuf.ServicePlans,
		Want:    want,
		Edit:    protobuf.EditService,
	}
}

// serviceRoot copies the schema of one version into a new directory, and
// returns the directory.
func serviceRoot(t *testing.T, version string) string {
	t.Helper()

	root := t.TempDir()
	assert.NoError(t, os.CopyFS(root, os.DirFS(filepath.Join(filepath.FromSlash(serviceTree), version))),
		"the "+version+" tree copies")
	return root
}

// serviceBuild builds the service composition over root, with its two
// plans and cfg.
func serviceBuild(t *testing.T, root string, cfg workspace.Config) *workspace.Workspace {
	t.Helper()

	w, err := protobuf.ComposeService(root).Plans(protobuf.ServicePlans()...).Config(cfg).Build()
	assert.NoError(t, err, "the composition builds")
	return w
}

// serviceRun runs w over the tree at root, and returns the run's report
// and error.
func serviceRun(t *testing.T, w *workspace.Workspace, root string) (*workspace.Report, error) {
	t.Helper()

	return w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
}

// cleanRun runs the service composition over root under cfg, and fails
// the test where the run is not clean. Each Error of the run fails the
// test on a line of its own, before the run's error does.
func cleanRun(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	report, err := serviceRun(t, serviceBuild(t, root, cfg), root)
	for d := range report.Sink.All() {
		expect.NotEqual(t, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s at %s is not an Error of the run: %s", d.Code, d.Pos, d.Msg))
	}
	assert.NoError(t, err, "the run is clean")
}

// explainState runs the service composition over root, and returns the
// explanation of the TypeScript name of the enum State at line.
func explainState(t *testing.T, root string, line int) *workspace.Explanation {
	t.Helper()

	w := serviceBuild(t, root, workspace.Config{})
	_, err := serviceRun(t, w, root)
	assert.NoError(t, err, "the run is clean")
	target, err := w.ParseTarget(fmt.Sprintf("%s@%s:%d", nameKey, schemaPath, line))
	assert.NoError(t, err, "the target parses")
	explained, err := w.Explain(t.Context(), target)
	assert.NoError(t, err, "the generation explains the name")
	return explained
}

// editSchema replaces the first occurrence of from with to in the schema
// under root.
func editSchema(t *testing.T, root, from, to string) {
	t.Helper()

	src := files.Read(t, filepath.Join(root, filepath.FromSlash(schemaPath)))
	assert.Contains(t, src, from, "the schema contains the text that the case edits")
	files.Write(t, root, files.Tree{schemaPath: files.Text(strings.Replace(src, from, to, 1))})
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
