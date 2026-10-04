// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest_test

import (
	"context"
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

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// What the broken fixtures add: a file no plan generates, the plan and
// the plugin a rewriting ledger records files under, the digest of no
// bytes, which a rewriting ledger records for an added file, and the
// namespace the audit check claims, pinned so a composition can claim
// it first.
const (
	missingFile                = "svc/store/missing.txt"
	otherPlan                  = "elsewhere"
	otherPlugin      plugin.ID = "elsewhere-plugin"
	emptyDigest                = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	suiteNamespace             = "workspacetest"
	otherDeclaration           = "Other"
)

// The sealed state's directory inside the brand's state directory, which
// one fixture's edit removes, the ordinal of the cold run among the runs
// over one fixture, and the generator that reports the run it runs in.
// AssertWarmEdited runs twice in its warm directory before the cold run.
const (
	stateDir           = "state"
	coldRun            = 3
	noterID  plugin.ID = "noter"
)

// otherSource is the canonical identity of a declaration the tree does
// not declare, which a rewriting ledger records as every file's source.
var otherSource = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: rowPkg, Name: otherDeclaration}.String()

// errLocked is what the locked ledger's open returns.
var errLocked = errors.New("workspacetest_test: the state directory is locked")

// runCode is the code the run-counting generators report under.
var runCode = diag.MustRegister(diag.Prefix("WSWARM"), diag.CodeSpec{
	Number:  1,
	Meaning: "a warm-check fixture reports the run its generator runs in",
})

// The checks exist to catch compositions that break the workspace
// frame, so the broken ones are composed and each check's own failure is
// asserted.
func TestChecks(t *testing.T) {
	t.Parallel()

	t.Run("AssertGenerated", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fixture that states no tree", func(t *testing.T) {
			t.Parallel()

			bare := fixture(t)
			bare.Tree = nil
			root := t.TempDir()
			msg := assert.Rejects(t, "a fixture with nothing to copy", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, bare, root)
			})
			assert.Contains(t, msg, "no tree", "the rejection names what the fixture owes")
		})

		t.Run("rejects a directory that already contains the tree", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			placed := filepath.Join(root, filepath.FromSlash(rowFile))
			assert.NoError(t, os.MkdirAll(filepath.Dir(placed), 0o755), "the source's directory is made")
			assert.NoError(t, os.WriteFile(placed, []byte(rowSource), 0o644), "the source is placed")
			msg := assert.Rejects(t, "a directory the tree cannot copy into", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, fixture(t), root)
			})
			assert.Contains(t, msg, "copies into the run's directory", "the rejection names the step that failed")
		})

		t.Run("rejects a fixture that states no plans", func(t *testing.T) {
			t.Parallel()

			planless := fixture(t)
			planless.Plans = nil
			root := t.TempDir()
			msg := assert.Rejects(t, "a fixture with nothing to compose", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, planless, root)
			})
			assert.Contains(t, msg, "no plans", "the rejection names what the fixture owes")
		})

		t.Run("rejects a fixture of one plan", func(t *testing.T) {
			t.Parallel()

			single := fixture(t)
			single.Plans = func() []workspace.Plan { return []workspace.Plan{mirrors()} }
			root := t.TempDir()
			msg := assert.Rejects(t, "a fixture of fewer than two plans", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, single, root)
			})
			assert.Contains(t, msg, "at least two plans", "the rejection names the plan count the suite checks")
		})

		t.Run("rejects a fixture that states no composition", func(t *testing.T) {
			t.Parallel()

			bare := fixture(t)
			bare.Compose = nil
			root := t.TempDir()
			msg := assert.Rejects(t, "a fixture with nothing to run", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, bare, root)
			})
			assert.Contains(t, msg, "no composition", "the rejection names what the fixture owes")
		})

		t.Run("rejects a composition that does not build", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a composition without a brand", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, unbranded(t), root)
			})
			assert.Contains(t, msg, "composition builds", "the rejection names the step that failed")
		})

		t.Run("rejects plans that do not run clean", func(t *testing.T) {
			t.Parallel()

			locked := fixture(t)
			locked.Compose = func(root string) *workspace.Builder {
				return onDisk(root).Ledger(func() (ledger.Ledger, error) { return nil, errLocked })
			}
			root := t.TempDir()
			msg := assert.Rejects(t, "a ledger that does not open", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, locked, root)
			})
			assert.Contains(t, msg, errLocked.Error(), "the rejection names the run's error")
		})

		t.Run("rejects a wanted file the plans do not generate", func(t *testing.T) {
			t.Parallel()

			long := fixture(t)
			long.Want[missingFile] = long.Want[genFile]
			root := t.TempDir()
			msg := assert.Rejects(t, "a golden file no plan writes", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, long, root)
			})
			assert.Contains(t, msg, missingFile, "the rejection names the missing file")
		})

		t.Run("rejects a file whose bytes differ from the wanted bytes", func(t *testing.T) {
			t.Parallel()

			stale := fixture(t)
			stale.Want[genFile] = stale.Want[stubFile]
			root := t.TempDir()
			msg := assert.Rejects(t, "a golden file the plans do not reproduce", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, stale, root)
			})
			assert.Contains(t, msg, "ForRow", "the rejection shows the generated bytes")
		})

		t.Run("rejects a record that lists a file under another plan", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a ledger that records the files under another plan", func(tb assert.TB) {
				workspacetest.AssertGenerated(tb, rewritten(t, underOther), root)
			})
			assert.Contains(t, msg, otherPlan, "the rejection shows the recorded plan")
		})
	})

	t.Run("AssertCollision", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a first plan that routes no file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a first plan whose copy collides with nothing", func(tb assert.TB) {
				workspacetest.AssertCollision(tb, idleFirst(t), root)
			})
			assert.Contains(t, msg, "no PlanCollision", "the rejection names the missing finding")
		})
	})

	t.Run("AssertIsolated", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a ledger that drops the entries of a failed plan", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a ledger that forgets the failed plan", func(tb assert.TB) {
				workspacetest.AssertIsolated(tb, rewritten(t, forgetting(stubsPlan)), root)
			})
			assert.Contains(t, msg, "entries remain", "the rejection names the entries the record lost")
		})
	})

	t.Run("AssertExported", func(t *testing.T) {
		t.Parallel()

		edits := []struct {
			name string
			edit coretest.Edit
			want string
		}{
			{
				name: "rejects a record that lists the exported files under another plan",
				edit: underOther,
				want: "which its record does not list",
			},
			{
				name: "rejects a record that names another plugin",
				edit: func(_ int, m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Plugins = []plugin.ID{otherPlugin}
					}
					return m
				},
				want: "does not name",
			},
			{
				name: "rejects a record that lists another source",
				edit: func(_ int, m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Sources = []string{otherSource}
					}
					return m
				},
				want: "does not list as a source",
			},
			{
				name: "rejects a record that lists a file the export does not",
				edit: func(_ int, m manifest.Manifest) manifest.Manifest {
					if !slices.ContainsFunc(m.Files, func(e manifest.Entry) bool { return e.Path == missingFile }) {
						missing := manifest.Entry{Path: missingFile, Plan: mirrorsPlan, Hash: emptyDigest}
						m.Files = append(m.Files, missing)
						slices.SortFunc(m.Files, byPath)
					}
					return m
				},
				want: missingFile,
			},
		}
		for _, tt := range edits {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				msg := assert.Rejects(t, "a record other than the plans' commits", func(tb assert.TB) {
					workspacetest.AssertExported(tb, rewritten(t, tt.edit), root)
				})
				assert.Contains(t, msg, tt.want, "the rejection names what the record and the export disagree on")
			})
		}
	})

	t.Run("AssertCycleRefused", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a composition that does not build", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a composition that fails before its cycle", func(tb assert.TB) {
				workspacetest.AssertCycleRefused(tb, unbranded(t), root)
			})
			assert.Contains(t, msg, "composition builds", "the rejection names the composition's own error")
		})
	})

	t.Run("AssertSwept", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a last plan that routes no file", func(t *testing.T) {
			t.Parallel()

			idleLast := fixture(t)
			idleLast.Plans = func() []workspace.Plan { return []workspace.Plan{mirrors(), idle()} }
			root := t.TempDir()
			msg := assert.Rejects(t, "a last plan with no file to remove", func(tb assert.TB) {
				workspacetest.AssertSwept(tb, idleLast, root)
			})
			assert.Contains(t, msg, "routes no file", "the rejection names what the last plan owes")
		})
	})

	t.Run("AssertAudited", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a first plan that emits nothing", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a first plan with no origin to promise a key on", func(tb assert.TB) {
				workspacetest.AssertAudited(tb, idleFirst(t), root)
			})
			assert.Contains(t, msg, "emits nothing", "the rejection names what the first plan owes")
		})

		t.Run("rejects a composition that claims the suite's namespace", func(t *testing.T) {
			t.Parallel()

			claiming := fixture(t)
			claiming.Compose = func(root string) *workspace.Builder {
				return onDisk(root).Keys(func(r *meta.Registry) error { return r.ClaimNamespace(suiteNamespace) })
			}
			root := t.TempDir()
			msg := assert.Rejects(t, "a composition the suite's key cannot register in", func(tb assert.TB) {
				workspacetest.AssertAudited(tb, claiming, root)
			})
			assert.Contains(t, msg, suiteNamespace, "the rejection names the namespace claimed twice")
		})
	})

	t.Run("AssertChecked", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a tree that contains no file", func(t *testing.T) {
			t.Parallel()

			empty := fixture(t)
			empty.Tree = fstest.MapFS{}
			root := t.TempDir()
			msg := assert.Rejects(t, "a tree with no file to seed a failure at", func(tb assert.TB) {
				workspacetest.AssertChecked(tb, empty, root)
			})
			assert.Contains(t, msg, "contains no file", "the rejection names what the tree owes")
		})

		t.Run("rejects a record that lists the read files under another plan", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a ledger that records the files under another plan", func(tb assert.TB) {
				workspacetest.AssertChecked(tb, rewritten(t, underOther), root)
			})
			assert.Contains(t, msg, "as its commit recorded them", "the rejection names the record that differs")
		})
	})

	t.Run("AssertWarmEdited", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the fixture's warm run after its edit", func(t *testing.T) {
			t.Parallel()

			workspacetest.AssertWarmEdited(t, fixture(t), t.TempDir(), t.TempDir())
		})

		t.Run("rejects a fixture that states no edit", func(t *testing.T) {
			t.Parallel()

			uneditable := fixture(t)
			uneditable.Edit = nil
			warm, cold := t.TempDir(), t.TempDir()
			msg := assert.Rejects(t, "a fixture with nothing to change", func(tb assert.TB) {
				workspacetest.AssertWarmEdited(tb, uneditable, warm, cold)
			})
			assert.Contains(t, msg, "no edit", "the rejection names what the fixture owes")
		})

		t.Run("rejects an edit that leaves the first plan's files unchanged", func(t *testing.T) {
			t.Parallel()

			idempotent := fixture(t)
			idempotent.Edit = func(root string) error { return writeRow(root, rowSource) }
			warm, cold := t.TempDir(), t.TempDir()
			msg := assert.Rejects(t, "an edit that writes the source as it was", func(tb assert.TB) {
				workspacetest.AssertWarmEdited(tb, idempotent, warm, cold)
			})
			assert.Contains(t, msg, "unchanged", "the rejection names the edit that changes nothing")
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
			msg := assert.Rejects(t, "an edit that also removes the sealed state", func(tb assert.TB) {
				workspacetest.AssertWarmEdited(tb, forgetful, warm, cold)
			})
			assert.Contains(t, msg, "reads the sealed state", "the rejection names the run that ran cold")
		})

		rejections := []struct {
			name    string
			fixture func(*testing.T) workspacetest.Fixture
			want    string
		}{
			{
				name: "rejects a warm run whose files differ from the cold run's", fixture: renamedPerRun,
				want: "the cold run's files",
			},
			{
				name: "rejects a warm run whose entries differ from the cold run's", fixture: recordedOtherCold,
				want: "the cold run's entries",
			},
			{
				name: "rejects a warm run whose findings differ from the cold run's", fixture: notedPerRun,
				want: "the cold run's findings",
			},
			{
				name: "rejects a warm run whose exports differ from the cold run's", fixture: packagedPerRun,
				want: "the cold run's exports",
			},
		}
		for _, tt := range rejections {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				warm, cold := t.TempDir(), t.TempDir()
				msg := assert.Rejects(t, "a run that tells the warm run from the cold one", func(tb assert.TB) {
					workspacetest.AssertWarmEdited(tb, tt.fixture(t), warm, cold)
				})
				assert.Contains(t, msg, tt.want, "the rejection names what the two runs disagree on")
			})
		}
	})
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
	f.Compose = func(string) *workspace.Builder { return workspace.New() }
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
	f.Compose = func(root string) *workspace.Builder {
		return onDisk(root).Ledger(func() (ledger.Ledger, error) {
			return coretest.NewRewriting(context.Background(), root, fixtureBrand, int(runs.Add(1)), edit)
		})
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

// byPath orders two entries by path, the order a record keeps.
func byPath(a, b manifest.Entry) int {
	return strings.Compare(a.Path, b.Path)
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

// packagedPerRun returns the fixture whose first plan's backend names
// the package of each file it routes after the number of files it routed,
// so the two runs leave the same files and export different packages.
func packagedPerRun(t *testing.T) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	var routed atomic.Int64
	f.Plans = func() []workspace.Plan {
		first := mirrors()
		first.Backend = printing(plugin.ID(mirrorsPlan + "-printer")).
			Packages(func(at plugin.Placement) (symbol.Identity, error) {
				pkg := at.Origin
				pkg.Name = fmt.Sprintf("routed%d", routed.Add(1))
				return pkg, nil
			}).
			Build()
		return []workspace.Plan{first, stubs()}
	}
	return f
}

// counting returns a generator named id that calls report once per struct
// in scope and emits nothing.
func counting(id plugin.ID, report func(*eidos.StructMatch)) plugin.Generator {
	p, held := eidos.NewPlugin(id).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
			report(m)
			return nil
		})).Build().(plugin.Generator)
	if !held {
		panic("workspacetest_test: an emitter rule lowers to the generator role")
	}
	return p
}
