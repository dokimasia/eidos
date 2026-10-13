// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// These constants are the parts that the broken fixtures add. They are a
// file that no plan generates, the plan and the plugin of the files that
// a rewriting ledger records, the digest of no bytes that a rewriting
// ledger records for an added file, the namespace of the audit check,
// which a composition can claim first, and the name of a declaration
// that the tree does not declare.
const (
	missingFile                = "svc/store/missing.txt"
	otherPlan                  = "elsewhere"
	otherPlugin      plugin.ID = "elsewhere-plugin"
	emptyDigest                = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	suiteNamespace             = "workspacetest"
	otherDeclaration           = "Other"
)

// renamedSource is Row's source with Row renamed, which an edit writes to
// change the export of the plan that mirrors Row.
const renamedSource = "package svc/store\ntype Record int string\n"

// configFile is a config file at the root of a tree, which no frontend
// claims, and configBody is its content.
const (
	configFile = ".fixture.yaml"
	configBody = "version: 1\n"
)

// claimTemplate is the template of a family that claims its files, and
// claimText is its text, which writes one struct for each record.
const (
	claimTemplate = "stub.tmpl"
	claimText     = "{{range .Records}}type {{.Decl.Name}} struct{}\n{{end}}"
)

// stateDir is the directory of the sealed state inside the state
// directory of the brand, which the edit of one fixture removes. coldRun
// is the ordinal of the cold run among the runs over one fixture, because
// AssertWarmEdited runs twice in its warm directory before the cold run.
// noterID is the generator that reports the number of its run.
const (
	stateDir           = "state"
	coldRun            = 3
	noterID  plugin.ID = "noter"
)

// These constants are the contracts of the checks that more than one case
// pins.
const (
	builds    = "the fixture's composition builds"
	generated = "the files under the brand's frame are the fixture's wanted files, byte for byte"
)

// otherSource is the canonical identity of a declaration the tree does
// not declare, which a rewriting ledger records as every file's source.
var otherSource = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: rowPkg, Name: otherDeclaration}.String()

// errLocked is the error that the open of the locked ledger returns.
var errLocked = errors.New("workspacetest_test: the state directory is locked")

// errUnconfigured is the error of a composition whose config does not
// decode.
var errUnconfigured = errors.New("workspacetest_test: the config file does not decode")

// runCode is the code of the findings of the generators that count their
// runs.
var runCode = diag.MustRegister(diag.Prefix("WSWARM"), diag.CodeSpec{
	Number:  1,
	Meaning: "a warm-check fixture reports the run its generator runs in",
})

// importing is the scripted language in the importer role, whose units a
// parse memo does not restore. Its import of a file is the file's path.
type importing struct {
	*frontendtest.Scripted
}

// ImportOf returns the file's path.
func (*importing) ImportOf(_ plugin.ImportScope, file string) string { return file }

// The checks exist to catch compositions that break the workspace
// frame, so each case composes a broken composition and asserts the
// failure of the check.
func TestChecks(t *testing.T) {
	t.Parallel()

	t.Run("AssertGenerated", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fixture without a tree", func(t *testing.T) {
			t.Parallel()

			bare := fixture(t)
			bare.Tree = nil
			root := t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to copy", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, bare, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has a tree"},
				"the rejection reports the missing part of the fixture")
		})

		t.Run("rejects a directory that already contains the tree", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{rowFile: files.Text(rowSource)})
			records := assert.Rejects(t, "a directory the tree cannot copy into", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, fixture(t), root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture's tree copies into the run's directory"},
				"the rejection names the step that failed")
		})

		t.Run("rejects a fixture without plans", func(t *testing.T) {
			t.Parallel()

			planless := fixture(t)
			planless.Plans = nil
			root := t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to compose", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, planless, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has plans"},
				"the rejection reports the missing part of the fixture")
		})

		t.Run("rejects a fixture of one plan", func(t *testing.T) {
			t.Parallel()

			single := fixture(t)
			single.Plans = func() []workspace.Plan { return []workspace.Plan{mirrors()} }
			root := t.TempDir()
			records := assert.Rejects(t, "a fixture of fewer than two plans", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, single, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the suite checks at least two plans"},
				"the rejection names the plan count the suite checks")
			got, _ := records[0].Got()
			assert.Equal(t, got, any(1), "the record shows the fixture's one plan")
		})

		t.Run("rejects a fixture without a composition", func(t *testing.T) {
			t.Parallel()

			bare := fixture(t)
			bare.Compose = nil
			root := t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to run", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, bare, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has a composition"},
				"the rejection reports the missing part of the fixture")
		})

		t.Run("rejects a composition that does not build", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a composition without a brand", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, unbranded(t), root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{builds}, "the rejection names the step that failed")
		})

		t.Run("rejects a composition that returns an error", func(t *testing.T) {
			t.Parallel()

			unconfigured := fixture(t)
			unconfigured.Compose = func(string) (*workspace.Builder, error) { return nil, errUnconfigured }
			root := t.TempDir()
			records := assert.Rejects(t, "a composition whose config does not decode", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, unconfigured, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture's composition configures"},
				"the rejection names the step that failed")
			got, _ := records[0].Got()
			err, _ := got.(error)
			assert.ErrorIs(t, err, errUnconfigured, "the record contains the error of the composition")
		})

		t.Run("rejects plans that do not run clean", func(t *testing.T) {
			t.Parallel()

			locked := fixture(t)
			locked.Compose = func(root string) (*workspace.Builder, error) {
				return onDisk(root).Ledger(func() (ledger.Ledger, error) { return nil, errLocked }), nil
			}
			root := t.TempDir()
			records := assert.Rejects(t, "a ledger that does not open", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, locked, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture's plans run clean"},
				"the rejection names the run's error")
			got, _ := records[0].Got()
			err, _ := got.(error)
			assert.ErrorIs(t, err, errLocked, "the record contains the ledger's error")
		})

		wants := []struct {
			name string
			edit func(map[string][]byte)
		}{
			{
				name: "rejects a wanted file the plans do not generate",
				edit: func(want map[string][]byte) { want[missingFile] = want[genFile] },
			},
			{
				name: "rejects a file whose bytes differ from the wanted bytes",
				edit: func(want map[string][]byte) { want[genFile] = want[stubFile] },
			},
		}
		for _, tt := range wants {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				golden := fixture(t)
				tt.edit(golden.Want)
				root := t.TempDir()
				records := assert.Rejects(t, "a golden tree the plans do not reproduce", func(tb assert.TB) {
					workspacetest.AssertGenerated(tb, golden, root)
				})
				assert.Equal(t, coretest.Contracts(records), []string{generated}, "the rejection names the tree")
				got, _ := records[0].Got()
				f := fixture(t)
				assert.Equal(t, got, any(map[string]string{
					genFile: string(f.Want[genFile]), stubFile: string(f.Want[stubFile]),
				}), "the record shows the tree the plans generated")
			})
		}

		t.Run("rejects a record that lists a file under another plan", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a ledger that records the files under another plan", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, rewritten(t, underOther), root)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the record lists each file under the plan whose commit wrote it"},
				"the rejection names the record")
			got, _ := records[0].Got()
			assert.Equal(t, got, any(map[string]string{genFile: otherPlan, stubFile: otherPlan}),
				"the record shows the plan each file is recorded under")
		})
	})

	t.Run("AssertCollision", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a first plan that routes no file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a first plan whose copy collides with nothing", func(tb assert.TB) {
				workspacetest.AssertCollision(tb, idleFirst(t), root)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the run reports a PlanCollision for a copy of plan " + idlePlan},
				"the rejection names the missing finding")
		})
	})

	t.Run("AssertIsolated", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a ledger that drops the entries of a failed plan", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a ledger that forgets the failed plan", func(tb assert.TB) {
				workspacetest.AssertIsolated(tb, rewritten(t, forgetting(stubsPlan)), root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the failed plan's entries remain"},
				"the rejection names the entries the record lost")
		})
	})

	t.Run("AssertExported", func(t *testing.T) {
		t.Parallel()

		edits := []struct {
			name string
			edit coretest.Edit
			want []string
		}{
			{
				name: "rejects a record that lists the exported files under another plan",
				edit: underOther,
				want: []string{
					"plan " + mirrorsPlan + " records the file it exports ForRow in",
					"plan " + stubsPlan + " records the file it exports RowStub in",
				},
			},
			{
				name: "rejects a record that lists another plugin",
				edit: func(_ int, m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Plugins = []plugin.ID{otherPlugin}
					}
					return m
				},
				want: []string{
					"the record of " + genFile + " names the plugin that exports ForRow",
					"the record of " + stubFile + " names the plugin that exports RowStub",
				},
			},
			{
				name: "rejects a record that lists another source",
				edit: func(_ int, m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Sources = []string{otherSource}
					}
					return m
				},
				want: []string{
					"the record of " + genFile + " lists the source ForRow derives from",
					"the record of " + stubFile + " lists the source RowStub derives from",
				},
			},
			{
				name: "rejects a record that lists a file the export does not",
				edit: func(_ int, m manifest.Manifest) manifest.Manifest {
					if !slices.ContainsFunc(m.Files, func(e manifest.Entry) bool { return e.Path == missingFile }) {
						missing := manifest.Entry{Path: missingFile, Plan: mirrorsPlan, Hash: emptyDigest}
						m.Files = append(m.Files, missing)
						slices.SortFunc(m.Files, func(a, b manifest.Entry) int {
							return strings.Compare(a.Path, b.Path)
						})
					}
					return m
				},
				want: []string{
					"plan " + mirrorsPlan + " exports a declaration in " + missingFile + ", which it records",
				},
			},
		}
		for _, tt := range edits {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				records := assert.Rejects(t, "a record other than the plans' commits", func(tb assert.TB) {
					workspacetest.AssertExported(tb, rewritten(t, tt.edit), root)
				})
				assert.Equal(t, coretest.Contracts(records), tt.want,
					"the rejection names each declaration and file the record and the export disagree on")
			})
		}

		t.Run("passes a plan whose family claims its files", func(t *testing.T) {
			t.Parallel()

			claiming := fixture(t)
			claiming.Plans = func() []workspace.Plan { return []workspace.Plan{mirrors(), claimedStubs()} }
			workspacetest.AssertExported(t, claiming, t.TempDir())
		})
	})

	t.Run("AssertCycleRefused", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a composition that does not build", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a composition that fails before its cycle", func(tb assert.TB) {
				workspacetest.AssertCycleRefused(tb, unbranded(t), root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{builds},
				"the rejection names the composition's own error")
		})

		t.Run("passes a composition that configures from the config file of its tree", func(t *testing.T) {
			t.Parallel()

			configured := fixture(t)
			configured.Tree = fstest.MapFS{
				configFile: {Data: []byte(configBody)},
				rowFile:    {Data: []byte(rowSource)},
			}
			compose := configured.Compose
			configured.Compose = func(root string) (*workspace.Builder, error) {
				if _, err := os.Stat(filepath.Join(root, configFile)); err != nil {
					return nil, err
				}
				return compose(root)
			}
			workspacetest.AssertCycleRefused(t, configured, t.TempDir())
		})
	})

	t.Run("AssertSwept", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a last plan that routes no file", func(t *testing.T) {
			t.Parallel()

			idleLast := fixture(t)
			idleLast.Plans = func() []workspace.Plan { return []workspace.Plan{mirrors(), idle()} }
			root := t.TempDir()
			records := assert.Rejects(t, "a last plan with no file to remove", func(tb assert.TB) {
				workspacetest.AssertSwept(tb, idleLast, root)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the fixture's last plan " + idlePlan + " routes a file"},
				"the rejection reports the missing file of the last plan")
		})
	})

	t.Run("AssertAudited", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a first plan that emits nothing", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a first plan with no origin to promise a key on", func(tb assert.TB) {
				workspacetest.AssertAudited(tb, idleFirst(t), root)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the fixture's first plan emits a unit that derives from a source declaration"},
				"the rejection reports the missing unit of the first plan")
		})

		t.Run("rejects a composition that claims the suite's namespace", func(t *testing.T) {
			t.Parallel()

			claiming := fixture(t)
			claiming.Compose = func(root string) (*workspace.Builder, error) {
				return onDisk(root).Keys(func(r *meta.Registry) error { return r.ClaimNamespace(suiteNamespace) }), nil
			}
			root := t.TempDir()
			records := assert.Rejects(t, "a composition the suite's key cannot register in", func(tb assert.TB) {
				workspacetest.AssertAudited(tb, claiming, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{builds},
				"the rejection names the composition that does not build")
			got, _ := records[0].Got()
			err, _ := got.(error)
			assert.HasError(t, err, "the record contains Build's error")
			assert.Contains(t, err.Error(), suiteNamespace,
				"the error contains the namespace that two registrants claim")
		})
	})

	t.Run("AssertChecked", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a tree that contains no file", func(t *testing.T) {
			t.Parallel()

			empty := fixture(t)
			empty.Tree = fstest.MapFS{}
			root := t.TempDir()
			records := assert.Rejects(t, "a tree with no file to seed a failure at", func(tb assert.TB) {
				workspacetest.AssertChecked(tb, empty, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture's tree contains a file"},
				"the rejection reports the missing file of the tree")
		})

		t.Run("rejects a record that lists the read files under another plan", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a ledger that records the files under another plan", func(tb assert.TB) {
				workspacetest.AssertChecked(tb, rewritten(t, underOther), root)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the check reads the plan's files as its commit recorded them"},
				"the rejection names the record that differs")
		})
	})

	t.Run("AssertWarmEdited", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the fixture's warm run after its edit", func(t *testing.T) {
			t.Parallel()

			workspacetest.AssertWarmEdited(t, fixture(t), t.TempDir(), t.TempDir())
		})

		t.Run("rejects a fixture without an edit", func(t *testing.T) {
			t.Parallel()

			uneditable := fixture(t)
			uneditable.Edit = nil
			warm, cold := t.TempDir(), t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to change", func(tb assert.TB) {
				workspacetest.AssertWarmEdited(tb, uneditable, warm, cold)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has an edit"},
				"the rejection reports the missing part of the fixture")
		})

		t.Run("rejects an edit that leaves the first plan's files unchanged", func(t *testing.T) {
			t.Parallel()

			idempotent := fixture(t)
			idempotent.Edit = func(root string) error { return writeRow(root, rowSource) }
			warm, cold := t.TempDir(), t.TempDir()
			records := assert.Rejects(t, "an edit that writes the source as it was", func(tb assert.TB) {
				workspacetest.AssertWarmEdited(tb, idempotent, warm, cold)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the fixture's edit changes the files of plan " + mirrorsPlan},
				"the rejection names the edit that changes nothing")
		})

		t.Run("rejects a warm run that ignores the sealed state", func(t *testing.T) {
			t.Parallel()

			forgetful := fixture(t)
			forgetful.Edit = func(root string) error {
				if err := os.RemoveAll(filepath.Join(root, ledger.StateDir(fixtureBrand), stateDir)); err != nil {
					return err
				}
				return writeRow(root, rowEdited)
			}
			warm, cold := t.TempDir(), t.TempDir()
			records := assert.Rejects(t, "an edit that also removes the sealed state", func(tb assert.TB) {
				workspacetest.AssertWarmEdited(tb, forgetful, warm, cold)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the run after the edit reads the sealed state"},
				"the rejection names the run that ran cold")
		})

		const (
			files    = "the warm run writes the same files as the cold run outside the state directory"
			entries  = "the warm run records the same entries as the cold run"
			findings = "the warm run reports the same findings as the cold run"
			exports  = "the probe reads the same exports in the warm run as in the cold run"
		)
		rejections := []struct {
			name    string
			fixture func(*testing.T) workspacetest.Fixture
			want    []string
		}{
			{
				name: "rejects a warm run whose files differ from those of the cold run", fixture: renamedPerRun,
				want: []string{files, entries, exports},
			},
			{
				name: "rejects a warm run whose entries differ from those of the cold run", fixture: recordedOtherCold,
				want: []string{entries},
			},
			{
				name: "rejects a warm run whose findings differ from those of the cold run", fixture: notedPerRun,
				want: []string{findings},
			},
			{
				name: "rejects a warm run whose exports differ from those of the cold run", fixture: packagedPerRun,
				want: []string{exports},
			},
		}
		for _, tt := range rejections {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				warm, cold := t.TempDir(), t.TempDir()
				records := assert.Rejects(t, "a run whose warm result differs from its cold result",
					func(tb assert.TB) {
						workspacetest.AssertWarmEdited(tb, tt.fixture(t), warm, cold)
					})
				assert.Equal(t, coretest.Contracts(records), tt.want,
					"the rejection names what the two runs disagree on")
			})
		}
	})

	t.Run("AssertWarmUnchanged", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a composition whose runs forget the sealed state", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a ledger that each run opens empty", func(tb assert.TB) {
				workspacetest.AssertWarmUnchanged(tb, forgetful(t), root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the run over an unchanged tree reads the sealed state",
				"the run over an unchanged tree hashes no file",
				"the run over an unchanged tree parses no unit",
				"the run over an unchanged tree runs no invocation of " + string(loweringID),
				"the run over an unchanged tree runs no invocation of " + string(mirrorID),
				"the run over an unchanged tree runs no invocation of " + string(stubberID),
			}, "the rejection names the work that the cold run does")
		})
	})

	t.Run("AssertTouched", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a composition whose runs forget the sealed state", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a ledger that each run opens empty", func(tb assert.TB) {
				workspacetest.AssertTouched(tb, forgetful(t), root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the run after the touch reads the sealed state",
				"the run after the touch parses no unit",
			}, "the rejection names the parse that the cold run makes")
		})

		t.Run("passes a tree whose first file no frontend claims", func(t *testing.T) {
			t.Parallel()

			configured := fixture(t)
			configured.Tree = fstest.MapFS{
				configFile: {Data: []byte(configBody)},
				rowFile:    {Data: []byte(rowSource)},
			}
			workspacetest.AssertTouched(t, configured, t.TempDir())
		})

		t.Run("rejects a tree of which no frontend claims a file", func(t *testing.T) {
			t.Parallel()

			unclaimed := fixture(t)
			unclaimed.Tree = fstest.MapFS{configFile: {Data: []byte(configBody)}}
			root := t.TempDir()
			records := assert.Rejects(t, "a tree without a file to touch", func(tb assert.TB) {
				workspacetest.AssertTouched(tb, unclaimed, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"a frontend claims a file of the fixture's tree"},
				"the rejection reports the missing claim of the tree")
		})
	})

	t.Run("AssertDamaged", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a composition that keeps no sealed state", func(t *testing.T) {
			t.Parallel()

			stateless := fixture(t)
			stateless.Compose = func(root string) (*workspace.Builder, error) {
				return workspace.New().
					Brand(fixtureBrand).
					Frontends(frontendtest.NewScripted()).
					Targets(fixtureTarget).
					Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }), nil
			}
			root := t.TempDir()
			records := assert.Rejects(t, "a composition without a ledger", func(tb assert.TB) {
				workspacetest.AssertDamaged(tb, stateless, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the run writes the segments of the sealed state"},
				"the rejection names the state that is missing")
		})
	})

	t.Run("AssertRestored", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fixture without an edit", func(t *testing.T) {
			t.Parallel()

			uneditable := fixture(t)
			uneditable.Edit = nil
			root := t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to revert", func(tb assert.TB) {
				workspacetest.AssertRestored(tb, uneditable, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has an edit"},
				"the rejection reports the missing part of the fixture")
		})

		t.Run("rejects an edit that leaves the tree unchanged", func(t *testing.T) {
			t.Parallel()

			idempotent := fixture(t)
			idempotent.Edit = func(root string) error { return writeRow(root, rowSource) }
			root := t.TempDir()
			records := assert.Rejects(t, "an edit that writes the source as it was", func(tb assert.TB) {
				workspacetest.AssertRestored(tb, idempotent, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the run after the edit parses the edited units"},
				"the rejection names the edit that changes nothing")
		})

		t.Run("passes a fixture whose frontend imports by parsing its reverted units again", func(t *testing.T) {
			t.Parallel()

			importer := fixture(t)
			importer.Compose = func(root string) (*workspace.Builder, error) {
				return workspace.New().
					Brand(fixtureBrand).
					Frontends(&importing{frontendtest.NewScripted()}).
					Targets(fixtureTarget).
					Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
					Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) }), nil
			}
			workspacetest.AssertRestored(t, importer, t.TempDir())
		})
	})

	t.Run("AssertExportCutoff", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fixture without an edit", func(t *testing.T) {
			t.Parallel()

			uneditable := fixture(t)
			uneditable.Edit = nil
			root := t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to change", func(tb assert.TB) {
				workspacetest.AssertExportCutoff(tb, uneditable, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has an edit"},
				"the rejection reports the missing part of the fixture")
		})

		t.Run("rejects an edit that changes the first plan's export", func(t *testing.T) {
			t.Parallel()

			renaming := fixture(t)
			renaming.Edit = func(root string) error { return writeRow(root, renamedSource) }
			root := t.TempDir()
			records := assert.Rejects(t, "an edit that renames the exported struct", func(tb assert.TB) {
				workspacetest.AssertExportCutoff(tb, renaming, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the first plan's export is unchanged after the edit",
				"the cutoff plan runs no invocation after the edit",
			}, "the rejection names the changed export and the invocation that it runs again")
		})
	})

	t.Run("AssertWarmChecked", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fixture without an edit", func(t *testing.T) {
			t.Parallel()

			uneditable := fixture(t)
			uneditable.Edit = nil
			warm, cold := t.TempDir(), t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to change", func(tb assert.TB) {
				workspacetest.AssertWarmChecked(tb, uneditable, warm, cold)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has an edit"},
				"the rejection reports the missing part of the fixture")
		})

		rejections := []struct {
			name    string
			fixture func(*testing.T) workspacetest.Fixture
			want    []string
		}{
			{
				name: "rejects a warm call whose files differ from those of the cold call", fixture: renamedPerRun,
				want: []string{
					"the warm call reads the same files as the cold call",
					"the warm call reads the same export as the cold call",
				},
			},
			{
				name: "rejects a warm call whose export differs from that of the cold call", fixture: packagedPerRun,
				want: []string{"the warm call reads the same export as the cold call"},
			},
		}
		for _, tt := range rejections {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				warm, cold := t.TempDir(), t.TempDir()
				records := assert.Rejects(t, "a call whose warm reads differ from its cold reads", func(tb assert.TB) {
					workspacetest.AssertWarmChecked(tb, tt.fixture(t), warm, cold)
				})
				assert.Equal(t, coretest.Contracts(records), tt.want,
					"the rejection names what the two calls disagree on")
			})
		}
	})
}

// forgetful returns the fixture whose composition opens an empty memory
// ledger for each run, so every run runs cold.
func forgetful(t *testing.T) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	f.Compose = func(root string) (*workspace.Builder, error) {
		return onDisk(root).Ledger(func() (ledger.Ledger, error) { return ledger.NewMem(), nil }), nil
	}
	return f
}

// idleFirst returns the fixture with the idle plan first, so its first
// plan routes no file and emits nothing.
func idleFirst(t *testing.T) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	f.Plans = func() []workspace.Plan { return []workspace.Plan{idle(), mirrors()} }
	return f
}

// unbranded returns the fixture whose composition declares nothing, so
// it does not build.
func unbranded(t *testing.T) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	f.Compose = func(string) (*workspace.Builder, error) { return workspace.New(), nil }
	return f
}

// rewritten returns the fixture whose ledger records the edited copy of
// every record a run commits. The edit is applied after every document
// a commit writes, so it is idempotent, and it reads the run's ordinal,
// counted per fixture.
func rewritten(t *testing.T, edit coretest.Edit) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	var runs atomic.Int64
	f.Compose = func(root string) (*workspace.Builder, error) {
		return onDisk(root).Ledger(func() (ledger.Ledger, error) {
			return coretest.NewRewriting(t.Context(), root, fixtureBrand, int(runs.Add(1)), edit)
		}), nil
	}
	return f
}

// underOther records every entry under the other plan.
func underOther(_ int, m manifest.Manifest) manifest.Manifest {
	for i := range m.Files {
		m.Files[i].Plan = otherPlan
	}
	return m
}

// forgetting returns an edit that keeps a fixture's first record and
// drops the entries of one plan from the record of every later run.
func forgetting(plan string) coretest.Edit {
	return func(run int, m manifest.Manifest) manifest.Manifest {
		if run > 1 {
			m.Files = slices.DeleteFunc(m.Files, func(e manifest.Entry) bool { return e.Plan == plan })
		}
		return m
	}
}

// writeRow writes Row's source file under root.
func writeRow(root, source string) error {
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(rowFile)), []byte(source), 0o644)
}

// renamedPerRun returns the fixture whose first plan names its struct
// after the number of runs its generator ran in, so no two runs generate
// the same file or export the same name.
func renamedPerRun(t *testing.T) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	var runs atomic.Int64
	f.Plans = func() []workspace.Plan {
		counting := planOf(mirrorsPlan, naming(mirrorID, genWord, func(m *eidos.StructMatch) *emit.Struct {
			return &emit.Struct{Origin: m.Struct.Identity(), Name: fmt.Sprintf("For%s%d", m.Struct.Name, runs.Add(1))}
		}))
		return []workspace.Plan{counting, stubs()}
	}
	return f
}

// notedPerRun returns the fixture whose first plan also reports an Info
// that names the number of runs its generator ran in, so no two runs
// report the same findings and every run generates the same files.
func notedPerRun(t *testing.T) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	var runs atomic.Int64
	f.Plans = func() []workspace.Plan {
		first := mirrors()
		first.Generators = append(slices.Clip(first.Generators), counting(noterID, func(m *eidos.StructMatch) {
			m.Infof(runCode, "run %d", runs.Add(1))
		}))
		return []workspace.Plan{first, stubs()}
	}
	return f
}

// recordedOtherCold returns the fixture whose ledger records the cold
// run's entries under another plan, so the two runs leave the same files
// and record different entries.
func recordedOtherCold(t *testing.T) workspacetest.Fixture {
	t.Helper()

	return rewritten(t, func(run int, m manifest.Manifest) manifest.Manifest {
		if run == coldRun {
			return underOther(run, m)
		}
		return m
	})
}

// packagedPerRun returns the fixture whose backend names the package of
// each file that it routes after the number of files that it routed, so
// the two runs leave the same files and export different packages. Both
// plans render through the backend, as a target renders through one
// backend.
func packagedPerRun(t *testing.T) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	var routed atomic.Int64
	f.Plans = func() []workspace.Plan {
		packaged := printing().
			Packages(func(at plugin.Placement) (symbol.Identity, error) {
				pkg := at.Origin
				pkg.Name = fmt.Sprintf("routed%d", routed.Add(1))
				return pkg, nil
			}).
			Build()
		first, second := mirrors(), stubs()
		first.Backend, second.Backend = packaged, packaged
		return []workspace.Plan{first, second}
	}
	return f
}

// claimedStubs returns the plan that records RowStub for Row into a
// per-package family of stubber that claims its files through
// [claimTemplate], and that depends on mirrors.
func claimedStubs() workspace.Plan {
	p := eidos.NewPlugin(stubberID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: stubWord, Template: claimTemplate}).
		Templates(fstest.MapFS{claimTemplate: {Data: []byte(claimText)}}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			stub := &emit.Struct{Origin: m.Struct.Identity(), Name: m.Struct.Name + "Stub"}
			e.PackageFile().Record(m.Struct.Name, stub)
			return nil
		})).Generator()
	return planOf(stubsPlan, p, mirrorsPlan)
}

// counting returns a generator named id that calls report once per struct
// in scope and emits nothing.
func counting(id plugin.ID, report func(*eidos.StructMatch)) plugin.Generator {
	return eidos.NewPlugin(id).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
			report(m)
			return nil
		})).Generator()
}
