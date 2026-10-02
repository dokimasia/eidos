// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
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

// otherSource is the canonical identity of a declaration the tree does
// not declare, which a rewriting ledger records as every file's source.
var otherSource = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: rowPkg, Name: otherDeclaration}.String()

// errLocked is what the locked ledger's open returns.
var errLocked = errors.New("workspacetest_test: the state directory is locked")

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
			edit func(manifest.Manifest) manifest.Manifest
			want string
		}{
			{
				name: "rejects a record that lists the exported files under another plan",
				edit: underOther,
				want: "which its record does not list",
			},
			{
				name: "rejects a record that names another plugin",
				edit: func(m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Plugins = []plugin.ID{otherPlugin}
					}
					return m
				},
				want: "does not name",
			},
			{
				name: "rejects a record that lists another source",
				edit: func(m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Sources = []string{otherSource}
					}
					return m
				},
				want: "does not list as a source",
			},
			{
				name: "rejects a record that lists a file the export does not",
				edit: func(m manifest.Manifest) manifest.Manifest {
					m.Files = append(m.Files, manifest.Entry{Path: missingFile, Plan: mirrorsPlan, Hash: emptyDigest})
					slices.SortFunc(m.Files, func(a, b manifest.Entry) int { return strings.Compare(a.Path, b.Path) })
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
// every manifest a run commits.
func rewritten(t *testing.T, edit func(manifest.Manifest) manifest.Manifest) workspacetest.Fixture {
	t.Helper()

	f := fixture(t)
	f.Compose = func(root string) *workspace.Builder {
		return onDisk(root).Ledger(func() (ledger.Ledger, error) {
			dir, err := ledger.OpenDir(root, fixtureBrand)
			return rewriting{Dir: dir, edit: edit}, err
		})
	}
	return f
}

// underOther records every entry under the other plan.
func underOther(m manifest.Manifest) manifest.Manifest {
	for i := range m.Files {
		m.Files[i].Plan = otherPlan
	}
	return m
}

// forgetting returns an edit that keeps a run's first manifest and drops
// the entries of one plan from every later one. The edit counts the
// manifests it sees, so each fixture takes an edit of its own.
func forgetting(plan string) func(manifest.Manifest) manifest.Manifest {
	var commits atomic.Int64
	return func(m manifest.Manifest) manifest.Manifest {
		if commits.Add(1) > 1 {
			m.Files = slices.DeleteFunc(m.Files, func(e manifest.Entry) bool { return e.Plan == plan })
		}
		return m
	}
}

// rewriting is the state directory's ledger recording an edited copy of
// each manifest a run commits: the shape of a ledger that records
// something other than what the plans wrote, which the checks must
// expose.
type rewriting struct {
	*ledger.Dir
	edit func(manifest.Manifest) manifest.Manifest
}

// CommitRun records the edited copy of a clone of the manifest's
// entries, so the edit leaves the run's own list unchanged.
func (l rewriting) CommitRun(ctx context.Context, m manifest.Manifest) error {
	m.Files = slices.Clone(m.Files)
	return l.Dir.CommitRun(ctx, l.edit(m))
}
