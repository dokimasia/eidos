// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// oldenName is the annotator that handles the directive old.
const oldenName plugin.ID = "olden"

// unusedSource is the source of the store with a diag directive whose code
// the mirror does not report for the store.
var unusedSource = source + suppressing

// deprecatedSource is the source of the store with the directive old.
var deprecatedSource = source + "+old\n"

// oldSchema is the schema of the directive old. Its Deprecated field has the
// rewrite of the directive.
var oldSchema = directive.Schema{
	Plugin:     string(oldenName),
	Name:       "old",
	Doc:        "marks a struct in the old way",
	Deprecated: "write +new in place of +old",
}

// Doctor checks the config, the sealed state and the directives of each
// workspace with a dry run, and writes nothing but the lock file.
func TestDoctor(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("returns StatusOK for a workspace without findings", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := invoke(t, compose, cmdDoctor, stored(t, nil))
			expect.Equal(t, status, cli.StatusOK, "doctor finds nothing")
			expect.Empty(t, stdout, "doctor writes no result")
			expect.Empty(t, stderr, "doctor reports nothing")
		})

		t.Run("writes an UnusedSuppression Info without --verbose", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{storePath: files.Text(unusedSource)})
			status, _, stderr := invoke(t, compose, cmdDoctor, root)
			expect.Equal(t, status, cli.StatusOK, "an Info fails nothing")
			expect.HasPrefix(t, stderr, storePath+":3: info "+workspace.UnusedSuppression.String()+": ",
				"doctor writes the Info at the directive")
		})

		t.Run("writes a DeprecatedDirective Warning at a deprecated directive", func(t *testing.T) {
			t.Parallel()

			deprecating := func() *workspace.Builder {
				olden := eidos.NewPlugin(oldenName).
					Handle(eidos.Directive(oldSchema, eidos.OnStruct(func(*eidos.StructMatch, *eidos.Stamper) error {
						return nil
					}))).
					Build().(plugin.Annotator)
				return compose().Annotators(olden)
			}
			root := stored(t, files.Tree{storePath: files.Text(deprecatedSource)})
			status, _, stderr := invoke(t, deprecating, cmdDoctor, root)
			expect.Equal(t, status, cli.StatusOK, "a Warning fails nothing")
			expect.HasPrefix(t, stderr, storePath+":3: warning "+directive.DeprecatedDirective.String()+": ",
				"doctor writes the Warning at the directive")
		})

		t.Run("writes a ColdState Info without --verbose", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			files.Write(t, root, files.Tree{confName: files.Text(refinedConfig)})
			_, _, stderr := invoke(t, compose, cmdDoctor, root)
			assert.Equal(t, stderr,
				".acme/state/CURRENT: info "+workspace.ColdState.String()+
					": the composition changed since the sealed state was recorded (load)\n",
				"doctor writes that the next run runs cold")
		})

		t.Run("writes no Info of another code without --verbose", func(t *testing.T) {
			t.Parallel()

			_, _, stderr := invoke(t, compose, cmdDoctor, stored(t, files.Tree{storePath: files.Text(chattySource)}))
			assert.Empty(t, stderr, "doctor leaves out the Info of the mirror")
		})

		t.Run("writes an Info of another code under --verbose", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{storePath: files.Text(chattySource)})
			_, _, stderr := invoke(t, compose, cmdDoctor, root, verboseFlag)
			assert.Equal(t, stderr, storePath+":2: info "+noted.String()+": Chatty is chatty (mirror)\n",
				"doctor writes the Info of the mirror")
		})

		t.Run("returns StatusFailed for an Error finding", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdDoctor, stored(t, files.Tree{badPath: files.Text(badSource)}))
			expect.Equal(t, status, cli.StatusFailed, "the Error fails doctor")
			expect.HasPrefix(t, stderr, badPath+":1: error ", "doctor writes the finding")
		})

		t.Run("returns StatusFailed for a config error", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			status, _, stderr := invoke(t, compose, cmdDoctor, root)
			expect.Equal(t, status, cli.StatusFailed, "doctor reports the config error as a failure")
			expect.Contains(t, stderr, "field workerz not found", "doctor writes the fault")
		})

		t.Run("checks the members of a list that build beside a member that does not", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:            files.Text(listConfig),
				"svc/store.zz":      files.Text("package store\ntype Store string\n" + suppressing),
				"tools/" + confName: files.Text("version: 2\n"),
				"tools/tools.zz":    files.Text("package tools\ntype Tool string\n"),
			})
			status, _, stderr := invoke(t, compose, cmdDoctor, root)
			expect.Equal(t, status, cli.StatusFailed, "the config error of tools fails doctor")
			expect.That(t, stderr).
				Contains("the file has version 2", "doctor writes the fault of tools").
				Contains("svc/store.zz:3: info "+workspace.UnusedSuppression.String(), "doctor checks svc")
		})

		t.Run("returns StatusFailed for a locator that returns an error", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, failing, cmdDoctor, stored(t, nil))
			expect.Equal(t, status, cli.StatusFailed, "the locator fails doctor")
			expect.Equal(t, stderr, locateError, "doctor writes the error of the locator")
		})

		t.Run("writes the error of a run that no finding reports", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{storePath: files.Text(brokenSource)})
			status, _, stderr := invoke(t, compose, cmdDoctor, root)
			expect.Equal(t, status, cli.StatusFailed, "the error fails doctor")
			expect.Equal(t, stderr, brokenError, "doctor writes the error of the run")
		})

		t.Run("writes no generated file", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			_, _, _ = invoke(t, compose, cmdDoctor, root)
			files.Absent(t, filepath.Join(root, "svc", "mirror.txt"), "doctor makes a dry run")
		})

		t.Run("writes the name of the member into each finding of a list", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:       files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
				"svc/store.zz": files.Text("package store\ntype Store string\n" + suppressing),
			})
			_, stdout, _ := invoke(t, compose, cmdDoctor, root, jsonFlag)
			got := decoded[findingEvent](t, stdout, eventDiag)
			assert.Length(t, got, 1, "the member has one finding")
			expect.Equal(t, got[0].Workspace, memberSvc, "the event names the member")
			expect.Equal(t, got[0].Code, workspace.UnusedSuppression.String(), "the finding is the Info")
		})
	})
}
