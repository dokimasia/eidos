// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pipelinetest_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
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
// brand, the plan a second composition or a rewriting ledger names, the
// digest of no bytes, which a rewriting ledger records, and the warning
// a frontend reports without a position.
const (
	brokenFile                = "svc/broken/bad.zz"
	brokenSource              = "type Lost string\n"
	missingFile               = "svc/store/missing.txt"
	foreignFile               = "svc/store/other.txt"
	foreignBody               = "type Other struct{}\n"
	otherBrand   output.Brand = "other"
	otherPlan                 = "other"
	emptyDigest               = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	warned                    = "a warning the fixture reports"
)

// The contracts of the checks, each before or after the path it names.
const (
	lists           = "the record lists "
	wanted          = ", which the fixture wants"
	changes         = "a second run changes no byte of "
	moves           = "a second run moves the mtime of no file: "
	relocatedBytes  = "a run in another directory generates the same bytes"
	relocatedRecord = "a run in another directory records the same files"
)

// manifestFile is the record's document that lists genFile: the
// document of the path's bucket in the state directory, named with the
// extension the ledger stores every document under.
var manifestFile = ledger.ManifestPath(fixtureBrand) + "/" + manifest.BucketOf(genFile) + ".json"

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
			records := assert.Rejects(t, "a source file the frontend reports an Error for", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, broken, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the run reports no Error: " + frontendtest.ScriptedBadFile.String() + " " + brokenFile +
					` opens with "type", not a package line at ` + brokenFile + ":1:0",
				"the run returns no error",
			}, "the rejection names the finding's code and position, then the run's error")
		})

		t.Run("rejects a finding without a position", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a warning no reader can open", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, over(&warning{Scripted: frontendtest.NewScripted()}), root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				"the finding states a file: " + frontendtest.ScriptedBadFile.String() + " " + warned,
			}, "the rejection names the finding without a file")
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
			records := assert.Rejects(t, "a ledger that does not open", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, locked, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the run returns no error"},
				"the rejection names the run's error")
			got, _ := records[0].Got()
			err, _ := got.(error)
			assert.ErrorIs(t, err, errLocked, "the record contains the ledger's error")
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
			records := assert.Rejects(t, "a fixture of more than one plan", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, twice, root)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the suite checks one plan, and the composition runs one"},
				"the rejection names the plan count the suite checks")
		})

		t.Run("rejects a fixture that states no tree", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to copy", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, pipelinetest.Fixture{Compose: compose}, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture states a tree"},
				"the rejection names what the fixture owes")
		})

		t.Run("rejects a fixture that states no composition", func(t *testing.T) {
			t.Parallel()

			bare := fixture()
			bare.Compose = nil
			root := t.TempDir()
			records := assert.Rejects(t, "a fixture with nothing to run", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, bare, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture states a composition"},
				"the rejection names what the fixture owes")
		})

		t.Run("rejects a composition that does not build", func(t *testing.T) {
			t.Parallel()

			unbranded := fixture()
			unbranded.Compose = func(string) (*workspace.Workspace, error) { return workspace.New().Build() }
			root := t.TempDir()
			records := assert.Rejects(t, "a composition without a brand", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, unbranded, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture's workspace composes"},
				"the rejection names the step that failed")
		})

		t.Run("rejects a directory that already contains the tree", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{rowFile: files.Text(rowSource)})
			records := assert.Rejects(t, "a directory the tree cannot copy into", func(tb assert.TB) {
				pipelinetest.AssertClean(tb, fixture(), root)
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the fixture's tree copies into the run's directory"},
				"the rejection names the step that failed")
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

		wants := []struct {
			name string
			want map[string][]byte
		}{
			{
				name: "rejects a file whose bytes differ from the wanted bytes",
				want: map[string][]byte{genFile: []byte(userStamped)},
			},
			{
				name: "rejects a generated file the fixture does not list",
				want: map[string][]byte{},
			},
			{
				name: "rejects a wanted file the run does not generate",
				want: map[string][]byte{genFile: []byte(genStamped), missingFile: []byte(genStamped)},
			},
		}
		for _, tt := range wants {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				golden := fixture()
				golden.Want = tt.want
				root := t.TempDir()
				records := assert.Rejects(t, "a golden tree the run does not reproduce", func(tb assert.TB) {
					pipelinetest.AssertGenerated(tb, golden, root)
				})
				assert.Equal(t, coretest.Contracts(records), []string{
					"the files under the brand's frame are the fixture's wanted files, byte for byte",
				}, "the rejection names the generated tree")
				got, _ := records[0].Got()
				assert.Equal(t, got, any(map[string]string{genFile: genStamped}),
					"the record shows the tree the run generated")
			})
		}
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
			records := assert.Rejects(t, "a ledger that leaves no record on disk", func(tb assert.TB) {
				pipelinetest.AssertRecorded(tb, elsewhere, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{lists + genFile + wanted},
				"the rejection names the wanted file the record omits")
		})

		edits := []struct {
			name     string
			edit     func(manifest.Manifest) manifest.Manifest
			contract string
			// got is the value the record shows, and nil for a membership
			// check, whose record states none.
			got any
		}{
			{
				name: "rejects a record that lists a file under another plan",
				edit: func(m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Plan = otherPlan
					}
					return m
				},
				contract: lists + genFile + " under the composition's plan",
				got:      otherPlan,
			},
			{
				name: "rejects a record whose digest is not the file's",
				edit: func(m manifest.Manifest) manifest.Manifest {
					for i := range m.Files {
						m.Files[i].Hash = emptyDigest
					}
					return m
				},
				contract: "the record states the digest of " + genFile,
				got:      emptyDigest,
			},
			{
				name: "rejects a record that lists a file the fixture does not want",
				edit: func(m manifest.Manifest) manifest.Manifest {
					if !slices.ContainsFunc(m.Files, func(e manifest.Entry) bool { return e.Path == missingFile }) {
						m.Files = append(m.Files, manifest.Entry{Path: missingFile, Plan: planName, Hash: emptyDigest})
						slices.SortFunc(m.Files, byPath)
					}
					return m
				},
				contract: "the fixture wants " + missingFile + ", which the record lists",
			},
			{
				name: "rejects a record that omits a wanted file",
				edit: func(m manifest.Manifest) manifest.Manifest {
					m.Files = nil
					return m
				},
				contract: lists + genFile + wanted,
			},
		}
		for _, tt := range edits {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				records := assert.Rejects(t, "a record other than the run's", func(tb assert.TB) {
					pipelinetest.AssertRecorded(tb, rewritten(t, tt.edit), root)
				})
				assert.Equal(t, coretest.Contracts(records), []string{tt.contract},
					"the rejection names the entry that differs")
				got, _ := records[0].Got()
				assert.Equal(t, got, tt.got, "the record shows the recorded value")
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
			records := assert.Rejects(t, "a generator that counts its runs", func(tb assert.TB) {
				pipelinetest.AssertIdempotent(tb, restless, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{
				changes + manifestFile, moves + manifestFile, changes + genFile, moves + genFile,
			}, "the rejection names the record and the generated file that the second run rewrote, in path order")
			got, _ := records[2].Got()
			body, _ := got.(string)
			assert.Contains(t, body, "ForRow2", "the record shows the second run's bytes")
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
			records := assert.Rejects(t, "a sink that touches what it committed", func(tb assert.TB) {
				pipelinetest.AssertIdempotent(tb, rewriter, root)
			})
			assert.Equal(t, coretest.Contracts(records), []string{moves + genFile},
				"the rejection names the touched file")
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
			records := assert.Rejects(t, "a generator that spells its directory", func(tb assert.TB) {
				pipelinetest.AssertRelocated(tb, rooted, one, two)
			})
			assert.Equal(t, coretest.Contracts(records), []string{relocatedBytes, relocatedRecord},
				"the rejection names the bytes and the digests that differ")
			got, _ := records[0].Got()
			framed, _ := got.(map[string]string)
			assert.Contains(t, framed[genFile], "ForRowIn"+filepath.Base(two),
				"the record shows the second directory's bytes")
		})

		t.Run("rejects a composition whose record depends on its directory", func(t *testing.T) {
			t.Parallel()

			named := fixture()
			named.Compose = func(root string) (*workspace.Workspace, error) {
				return onDisk(root, frontendtest.NewScripted()).
					Plans(plan(planName+"-"+filepath.Base(root), mirror())).Build()
			}
			one, two := t.TempDir(), t.TempDir()
			records := assert.Rejects(t, "a plan named after its directory", func(tb assert.TB) {
				pipelinetest.AssertRelocated(tb, named, one, two)
			})
			assert.Equal(t, coretest.Contracts(records), []string{relocatedRecord},
				"the rejection names the record that differs")
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
// copy of every record the run commits, opened under the test's context.
// The edit is applied after every document the commit writes, so it is
// idempotent.
func rewritten(tb testing.TB, edit func(manifest.Manifest) manifest.Manifest) pipelinetest.Fixture {
	tb.Helper()

	f := fixture()
	var runs atomic.Int64
	f.Compose = func(root string) (*workspace.Workspace, error) {
		return onDisk(root, frontendtest.NewScripted()).
			Ledger(func() (ledger.Ledger, error) {
				return coretest.NewRewriting(tb.Context(), root, fixtureBrand, int(runs.Add(1)),
					func(_ int, m manifest.Manifest) manifest.Manifest { return edit(m) })
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
	u.Warnf(frontendtest.ScriptedBadFile, f.at, warned)
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

// byPath orders two entries by path, the order a record keeps.
func byPath(a, b manifest.Entry) int {
	return strings.Compare(a.Path, b.Path)
}
