// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pipelinetest_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/pipelinetest"
)

// What the broken fixtures add: a source file without a package line, a
// file no run generates, another tool's output in the tree under its
// brand, the plan a second composition or a rewriting ledger names, and
// the digest of no bytes, which a rewriting ledger records, with its hex
// digits apart because a diff splits the digest after its name.
const (
	brokenFile                = "svc/broken/bad.zz"
	brokenSource              = "type Lost string\n"
	missingFile               = "svc/store/missing.txt"
	foreignFile               = "svc/store/other.txt"
	foreignBody               = "type Other struct{}\n"
	otherBrand   output.Brand = "other"
	otherPlan                 = "other"
	emptyHex                  = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	emptyDigest               = "sha256:" + emptyHex
)

// errLocked is what the locked ledger's open returns.
var errLocked = errors.New("pipelinetest_test: the state directory is locked")

// The checks exist to catch broken plans, so the broken ones are
// composed and each check's own failure is asserted.
func TestChecks(t *testing.T) {
	t.Parallel()

	t.Run("AssertClean", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a run whose findings are positioned warnings", func(t *testing.T) {
			t.Parallel()

			pipelinetest.AssertClean(t, over(&warning{
				Scripted: frontendtest.NewScripted(), at: position.Pos{File: rowFile, Line: 1},
			}), t.TempDir())
		})

		t.Run("rejects a run that reports an Error", func(t *testing.T) {
			t.Parallel()

			broken := fixture()
			broken.Tree = fstest.MapFS{
				rowFile:    {Data: []byte(rowSource)},
				brokenFile: {Data: []byte(brokenSource)},
			}
			root := t.TempDir()
			msg := assert.Rejects(t, "a source file the frontend reports an Error for", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, broken, root)
			})
			assert.Contains(t, msg, frontendtest.ScriptedBadFile.String(), "the rejection names the finding's code")
		})

		t.Run("rejects a finding without a position", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a warning no reader can open", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, over(&warning{Scripted: frontendtest.NewScripted()}), root)
			})
			assert.Contains(t, msg, "has no position", "the rejection names what is missing")
		})

		t.Run("rejects a run that returns an error", func(t *testing.T) {
			t.Parallel()

			locked := fixture()
			locked.Compose = func(root string) (*workspace.Workspace, error) {
				return onDisk(root, frontendtest.NewScripted()).
					Ledger(func() (ledger.Ledger, error) { return nil, errLocked }).
					Plans(plan(planName, mirror())).Build()
			}
			root := t.TempDir()
			msg := assert.Rejects(t, "a ledger that does not open", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, locked, root)
			})
			assert.Contains(t, msg, errLocked.Error(), "the rejection names the run's error")
		})

		t.Run("rejects a composition of two plans", func(t *testing.T) {
			t.Parallel()

			twice := fixture()
			twice.Compose = func(root string) (*workspace.Workspace, error) {
				other := naming(otherPlan, func(m *eidos.StructMatch) string { return "Other" + m.Struct.Name })
				return onDisk(root, frontendtest.NewScripted()).
					Plans(plan(planName, mirror()), plan(otherPlan, other)).Build()
			}
			root := t.TempDir()
			msg := assert.Rejects(t, "a fixture of more than one plan", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, twice, root)
			})
			assert.Contains(t, msg, "one plan", "the rejection names the plan count the suite checks")
		})

		t.Run("rejects a fixture that states no tree", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			msg := assert.Rejects(t, "a fixture with nothing to copy", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, pipelinetest.Fixture{Compose: compose}, root)
			})
			assert.Contains(t, msg, "no tree", "the rejection names what the fixture owes")
		})

		t.Run("rejects a fixture that states no composition", func(t *testing.T) {
			t.Parallel()

			bare := fixture()
			bare.Compose = nil
			root := t.TempDir()
			msg := assert.Rejects(t, "a fixture with nothing to run", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, bare, root)
			})
			assert.Contains(t, msg, "no composition", "the rejection names what the fixture owes")
		})

		t.Run("rejects a composition that does not build", func(t *testing.T) {
			t.Parallel()

			unbranded := fixture()
			unbranded.Compose = func(string) (*workspace.Workspace, error) { return workspace.New().Build() }
			root := t.TempDir()
			msg := assert.Rejects(t, "a composition without a brand", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, unbranded, root)
			})
			assert.Contains(t, msg, "composes", "the rejection names the step that failed")
		})

		t.Run("rejects a directory that already contains the tree", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(rowFile))), 0o755),
				"the source's directory is made")
			assert.NoError(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(rowFile)), []byte(rowSource), 0o644),
				"the source is placed")
			msg := assert.Rejects(t, "a directory the tree cannot copy into", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, fixture(), root)
			})
			assert.Contains(t, msg, "copies", "the rejection names the step that failed")
		})
	})

	t.Run("AssertGenerated", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a tree that contains another brand's output", func(t *testing.T) {
			t.Parallel()

			shared := fixture()
			shared.Tree = fstest.MapFS{
				rowFile:     {Data: []byte(rowSource)},
				foreignFile: {Data: framedBy(t, otherBrand, foreignFile, foreignBody)},
			}
			pipelinetest.AssertGenerated(t, shared, t.TempDir())
		})

		t.Run("rejects a file whose bytes differ from the wanted bytes", func(t *testing.T) {
			t.Parallel()

			stale := fixture()
			stale.Want = map[string][]byte{genFile: []byte(userStamped)}
			root := t.TempDir()
			msg := assert.Rejects(t, "a golden file the run does not reproduce", func(tb assert.TB) {
				pipelinetest.AssertGenerated(tb, stale, root)
			})
			assert.Contains(t, msg, "ForRow", "the rejection shows the generated bytes")
		})

		t.Run("rejects a generated file the fixture does not list", func(t *testing.T) {
			t.Parallel()

			short := fixture()
			short.Want = map[string][]byte{}
			root := t.TempDir()
			msg := assert.Rejects(t, "a generated file no golden covers", func(tb assert.TB) {
				pipelinetest.AssertGenerated(tb, short, root)
			})
			assert.Contains(t, msg, genFile, "the rejection names the unlisted file")
		})

		t.Run("rejects a wanted file the run does not generate", func(t *testing.T) {
			t.Parallel()

			long := fixture()
			long.Want = map[string][]byte{genFile: []byte(genStamped), missingFile: []byte(genStamped)}
			root := t.TempDir()
			msg := assert.Rejects(t, "a golden file the run never writes", func(tb assert.TB) {
				pipelinetest.AssertGenerated(tb, long, root)
			})
			assert.Contains(t, msg, missingFile, "the rejection names the missing file")
		})
	})

	t.Run("AssertRecorded", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a composition that records outside the state directory", func(t *testing.T) {
			t.Parallel()

			elsewhere := fixture()
			elsewhere.Compose = func(root string) (*workspace.Workspace, error) {
				return onDisk(root, frontendtest.NewScripted()).
					Ledger(func() (ledger.Ledger, error) { return ledger.NewMem(), nil }).
					Plans(plan(planName, mirror())).Build()
			}
			root := t.TempDir()
			msg := assert.Rejects(t, "a ledger that leaves no record on disk", func(tb assert.TB) {
				pipelinetest.AssertRecorded(tb, elsewhere, root)
			})
			assert.Contains(t, msg, "record", "the rejection names the missing record")
		})

		edits := []struct {
			name string
			edit func(manifest.Manifest) manifest.Manifest
			want string
		}{
			{
				name: "rejects a record that lists a file under another plan",
				edit: func(m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Plan = otherPlan
					}
					return m
				},
				want: otherPlan,
			},
			{
				name: "rejects a record whose digest is not the file's",
				edit: func(m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Hash = emptyDigest
					}
					return m
				},
				want: emptyHex,
			},
			{
				name: "rejects a record that lists a file the fixture does not want",
				edit: func(m manifest.Manifest) manifest.Manifest {
					m.Files = append(m.Files, manifest.Entry{Path: missingFile, Plan: planName, Hash: emptyDigest})
					return m
				},
				want: missingFile,
			},
			{
				name: "rejects a record that omits a wanted file",
				edit: func(m manifest.Manifest) manifest.Manifest {
					m.Files = nil
					return m
				},
				want: genFile,
			},
		}
		for _, tt := range edits {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				msg := assert.Rejects(t, "a record other than the run's", func(tb assert.TB) {
					pipelinetest.AssertRecorded(tb, rewritten(tt.edit), root)
				})
				assert.Contains(t, msg, tt.want, "the rejection shows the recorded difference")
			})
		}
	})

	t.Run("AssertIdempotent", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a plan whose output changes on every run", func(t *testing.T) {
			t.Parallel()

			var runs atomic.Int64
			restless := fixture()
			restless.Compose = func(root string) (*workspace.Workspace, error) {
				counting := naming(mirrorID, func(m *eidos.StructMatch) string {
					return "For" + m.Struct.Name + strconv.FormatInt(runs.Add(1), 10)
				})
				return onDisk(root, frontendtest.NewScripted()).Plans(plan(planName, counting)).Build()
			}
			root := t.TempDir()
			msg := assert.Rejects(t, "a generator that counts its runs", func(tb assert.TB) {
				pipelinetest.AssertIdempotent(tb, restless, root)
			})
			assert.Contains(t, msg, "ForRow2", "the rejection shows the second run's bytes")
		})

		t.Run("rejects a sink that rewrites an unchanged file", func(t *testing.T) {
			t.Parallel()

			rewriter := fixture()
			rewriter.Compose = func(root string) (*workspace.Workspace, error) {
				return onDisk(root, frontendtest.NewScripted()).
					Output(func() (output.Sink, error) {
						disk, err := output.NewDisk(root, fixtureBrand)
						return &touching{Disk: disk, root: root}, err
					}).
					Plans(plan(planName, mirror())).Build()
			}
			root := t.TempDir()
			msg := assert.Rejects(t, "a sink that touches what it committed", func(tb assert.TB) {
				pipelinetest.AssertIdempotent(tb, rewriter, root)
			})
			assert.Contains(t, msg, genFile, "the rejection names the touched file")
		})
	})

	t.Run("AssertRelocated", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a plan whose output depends on its directory", func(t *testing.T) {
			t.Parallel()

			rooted := fixture()
			rooted.Compose = func(root string) (*workspace.Workspace, error) {
				placed := naming(mirrorID, func(m *eidos.StructMatch) string {
					return "For" + m.Struct.Name + "In" + filepath.Base(root)
				})
				return onDisk(root, frontendtest.NewScripted()).Plans(plan(planName, placed)).Build()
			}
			one, two := t.TempDir(), t.TempDir()
			msg := assert.Rejects(t, "a generator that spells its directory", func(tb assert.TB) {
				pipelinetest.AssertRelocated(tb, rooted, one, two)
			})
			assert.Contains(t, msg, "same bytes", "the rejection names the bytes that differ")
		})

		t.Run("rejects a composition whose record depends on its directory", func(t *testing.T) {
			t.Parallel()

			named := fixture()
			named.Compose = func(root string) (*workspace.Workspace, error) {
				return onDisk(root, frontendtest.NewScripted()).
					Plans(plan(planName+"-"+filepath.Base(root), mirror())).Build()
			}
			one, two := t.TempDir(), t.TempDir()
			msg := assert.Rejects(t, "a plan named after its directory", func(tb assert.TB) {
				pipelinetest.AssertRelocated(tb, named, one, two)
			})
			assert.Contains(t, msg, "records the same files", "the rejection names the record that differs")
		})
	})
}

// over returns the plain fixture loaded by another frontend.
func over(front plugin.Frontend) pipelinetest.Fixture {
	f := fixture()
	f.Compose = func(root string) (*workspace.Workspace, error) {
		return onDisk(root, front).Plans(plan(planName, mirror())).Build()
	}
	return f
}

// rewritten returns the plain fixture whose ledger records the edited
// copy of every manifest the run commits.
func rewritten(edit func(manifest.Manifest) manifest.Manifest) pipelinetest.Fixture {
	f := fixture()
	f.Compose = func(root string) (*workspace.Workspace, error) {
		return onDisk(root, frontendtest.NewScripted()).
			Ledger(func() (ledger.Ledger, error) {
				dir, err := ledger.OpenDir(root, fixtureBrand)
				return rewriting{Dir: dir, edit: edit}, err
			}).
			Plans(plan(planName, mirror())).Build()
	}
	return f
}

// framedBy returns a body framed under a brand through a line comment,
// the way another tool built on the kernel writes its output.
func framedBy(tb assert.TB, brand output.Brand, path, body string) []byte {
	tb.Helper()

	c, err := output.NewContract(brand, plugin.CommentSyntax{Line: []string{"//"}})
	assert.NoError(tb, err, "the brand frames")
	b, err := c.Stamp(plugin.RenderedFile{Path: path, Plugins: []plugin.ID{mirrorID}, Body: []byte(body)})
	assert.NoError(tb, err, "the body stamps")
	return b
}

// warning is the scripted language reporting one warning per unit at
// the position the case states, which the clean check passes where the
// position names a file and rejects where it names none.
type warning struct {
	*frontendtest.Scripted
	at position.Pos
}

// Parse warns, then lowers the unit.
func (f *warning) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	u.Warnf(frontendtest.ScriptedBadFile, f.at, "a warning the fixture reports")
	return f.Scripted.Parse(ctx, u)
}

// touching is a disk sink that sets the times of every file it staged
// to the present after its commit: the shape of a sink that rewrites
// unchanged bytes, which the idempotence check must expose.
type touching struct {
	*output.Disk
	root   string
	staged []string
}

// Write records the path, then stages the file.
func (s *touching) Write(path string, body []byte) error {
	s.staged = append(s.staged, path)
	return s.Disk.Write(path, body)
}

// Commit commits, then sets the times of every staged file to the
// present.
func (s *touching) Commit() ([]output.Written, error) {
	written, err := s.Disk.Commit()
	now := time.Now()
	for _, path := range s.staged {
		err = errors.Join(err, os.Chtimes(filepath.Join(s.root, filepath.FromSlash(path)), now, now))
	}
	return written, err
}

// rewriting is the state directory's ledger recording an edited copy of
// each manifest the run commits: the shape of a ledger that records
// something other than what the run wrote, which the record check must
// expose.
type rewriting struct {
	*ledger.Dir
	edit func(manifest.Manifest) manifest.Manifest
}

// CommitRun records the edited copy.
func (l rewriting) CommitRun(ctx context.Context, m manifest.Manifest) error {
	m.Files = slices.Clone(m.Files)
	return l.Dir.CommitRun(ctx, l.edit(m))
}
