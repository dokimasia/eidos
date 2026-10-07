// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"crypto/sha256"
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// fingerprintAllocs is the copy of the fingerprint a call returns.
const fingerprintAllocs = 1

// The inputs a fingerprint case changes: another workspace name,
// frontend version and option, the directory a centralised layout writes
// under, the file a template tree contains, another brand, the two
// frontends' names, and the ignored spellings.
const (
	otherWorkspace                = "other"
	otherVersion                  = "2"
	otherTag                      = "other"
	genDir                        = "gen"
	templateFile                  = "struct.tmpl"
	otherBrand     output.Brand   = "other"
	firstFrontend  plugin.ID      = "first"
	secondFrontend plugin.ID      = "second"
	otherIgnored   directive.Name = "legacy:"
	ignoredA       directive.Name = "alpha:"
	ignoredB       directive.Name = "beta:"
)

// A run reads the sealed state only where the live generation records
// the composition's fingerprint, so the fingerprint's determinism and
// its sensitivity to each input of the composition are pinned here.
func TestFingerprint(t *testing.T) {
	t.Parallel()

	t.Run("Fingerprint", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the same bytes for the same composition", func(t *testing.T) {
			t.Parallel()

			b1, _ := flagged()
			w1, err := b1.Build()
			assert.NoError(t, err, "the composition composes")
			b2, _ := flagged()
			w2, err := b2.Build()
			assert.NoError(t, err, "its twin composes")
			assert.Equal(t, w2.Fingerprint(), w1.Fingerprint(), "one composition, one fingerprint")
			assert.Deterministic(t, func(w *workspace.Workspace) ([]byte, error) { return w.Fingerprint(), nil }, w1,
				"and the derivation repeats")
		})

		t.Run("returns a SHA-256 digest", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition composes")
			assert.Length(t, w.Fingerprint(), sha256.Size, "the fingerprint is one digest")
		})

		t.Run("returns a copy the caller can change", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition composes")
			first := w.Fingerprint()
			first[0]++
			assert.NotEqual(t, w.Fingerprint(), first, "a change to a copy leaves the workspace's own")
		})

		t.Run("returns other bytes for another scheduled roster", func(t *testing.T) {
			t.Parallel()

			b1, _ := flagged()
			w1, err := b1.Build()
			assert.NoError(t, err, "the flagged composition composes")
			extra, _ := eidos.NewPlugin("extra").
				Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Stamper) error {
					return nil
				})).Build().(plugin.Annotator)
			b2, _ := flagged()
			w2, err := b2.Annotators(extra).Build()
			assert.NoError(t, err, "the widened composition composes")
			assert.NotEqual(t, w2.Fingerprint(), w1.Fingerprint(), "two compositions never read one generation")
		})

		// edited returns the fingerprint of the valid composition with a
		// second plan the edit changes, and the checks.
		edited := func(t *testing.T, edit func(*workspace.Plan), checks ...plugin.WorkspaceCheck) []byte {
			t.Helper()

			bindings := planTo("bindings", "fixture", mirror("bindings-mirror"))
			edit(&bindings)
			w, err := valid().Plans(bindings).Checks(checks...).Build()
			assert.NoError(t, err, "the composition composes")
			return w.Fingerprint()
		}
		unchanged := func(*workspace.Plan) {}

		t.Run("returns other bytes for another scope of a plan", func(t *testing.T) {
			t.Parallel()

			scoped := edited(t, func(p *workspace.Plan) {
				p.Sources = workspace.Sources{Packages: []string{"./svc/..."}}
			})
			assert.NotEqual(t, scoped, edited(t, unchanged), "a change of scope runs cold")
		})

		t.Run("returns the same bytes for one scope's patterns in any order", func(t *testing.T) {
			t.Parallel()

			forward := edited(t, func(p *workspace.Plan) {
				p.Sources = workspace.Sources{Packages: []string{"a", "b"}}
			})
			backward := edited(t, func(p *workspace.Plan) {
				p.Sources = workspace.Sources{Packages: []string{"b", "a"}}
			})
			assert.Equal(t, backward, forward, "the patterns fold sorted")
		})

		t.Run("returns other bytes for another dependency of a plan", func(t *testing.T) {
			t.Parallel()

			depending := edited(t, func(p *workspace.Plan) { p.DependsOn = []string{"plan"} })
			assert.NotEqual(t, depending, edited(t, unchanged), "a dependency runs cold")
		})

		t.Run("returns other bytes for an added check", func(t *testing.T) {
			t.Parallel()

			checked := edited(t, unchanged, &recordingCheck{name: "stubbed"})
			assert.NotEqual(t, checked, edited(t, unchanged), "a check runs cold")
		})

		t.Run("returns other bytes for other plans a check reads", func(t *testing.T) {
			t.Parallel()

			one := edited(t, unchanged, &recordingCheck{name: "stubbed", reads: []string{"plan"}})
			other := edited(t, unchanged, &recordingCheck{name: "stubbed", reads: []string{"bindings"}})
			assert.NotEqual(t, other, one, "the plans a check reads run cold")
		})

		t.Run("returns other bytes for another layout of a plan", func(t *testing.T) {
			t.Parallel()

			centralised := edited(t, func(p *workspace.Plan) {
				p.Layout = layout.Config{Policy: layout.PolicyCentralised, Dir: genDir}
			})
			assert.NotEqual(t, centralised, edited(t, unchanged), "a change of layout runs cold")
		})

		changes := []struct {
			name    string
			compose func() *workspace.Builder
		}{
			{
				name:    "returns other bytes for another brand",
				compose: func() *workspace.Builder { return loading().Brand(otherBrand) },
			},
			{
				name:    "returns other bytes for another workspace name",
				compose: func() *workspace.Builder { return loading().Workspace(otherWorkspace) },
			},
			{
				name: "returns other bytes for another version of a frontend",
				compose: func() *workspace.Builder {
					f := scriptedAs(firstFrontend)
					f.Ver = otherVersion
					return valid().Frontends(f, scriptedAs(secondFrontend))
				},
			},
			{
				name: "returns other bytes for other options of a frontend",
				compose: func() *workspace.Builder {
					f := scriptedAs(firstFrontend)
					f.Opts = &frontendtest.ScriptedOptions{Tag: otherTag}
					return valid().Frontends(f, scriptedAs(secondFrontend))
				},
			},
			{
				name: "returns other bytes for another selection of a frontend",
				compose: func() *workspace.Builder {
					f := scriptedAs(firstFrontend)
					f.Sel = f.Sel[:1]
					return valid().Frontends(f, scriptedAs(secondFrontend))
				},
			},
			{
				name: "returns other bytes for the frontends in another order",
				compose: func() *workspace.Builder {
					return valid().Frontends(scriptedAs(secondFrontend), scriptedAs(firstFrontend))
				},
			},
			{
				name:    "returns other bytes for another ignored spelling",
				compose: func() *workspace.Builder { return loading().Ignore(otherIgnored) },
			},
		}
		for _, tt := range changes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NotEqual(
					t,
					fingerprinted(t, tt.compose()),
					fingerprinted(t, loading()),
					"the changed input runs cold",
				)
			})
		}

		t.Run("returns other bytes for one ignored spelling in place of another", func(t *testing.T) {
			t.Parallel()

			one := fingerprinted(t, valid().Ignore(ignoredA))
			other := fingerprinted(t, valid().Ignore(ignoredB))
			assert.NotEqual(t, other, one, "the spellings fold, not their count")
		})

		t.Run("returns the same bytes for the ignored spellings in any order", func(t *testing.T) {
			t.Parallel()

			forward := fingerprinted(t, valid().Ignore(ignoredA, ignoredB))
			backward := fingerprinted(t, valid().Ignore(ignoredB, ignoredA))
			assert.Equal(t, backward, forward, "the spellings fold sorted")
		})

		t.Run("returns the same bytes for another template tree, which each run folds", func(t *testing.T) {
			t.Parallel()

			one := fingerprinted(t, templatedBuilder(fstest.MapFS{templateFile: {Data: []byte("one")}}))
			other := fingerprinted(t, templatedBuilder(fstest.MapFS{templateFile: {Data: []byte("other")}}))
			assert.Equal(t, other, one, "the fingerprint leaves the trees to the run")
		})
	})
}

// A fingerprint allocates its copy in the ordinary run, which runs no
// benchmark. The check runs alone, because the count includes every
// goroutine's allocations.
func TestFingerprintAllocs(t *testing.T) {
	w, err := valid().Build()
	assert.NoError(t, err, "the composition composes")
	var got []byte
	assert.MaxAllocs(t, func() { got = w.Fingerprint() }, fingerprintAllocs,
		"Fingerprint allocates the copy it returns")
	assert.Length(t, got, sha256.Size, "Fingerprint returns the digest")
}

// BenchmarkFingerprint measures the copy of a workspace's fingerprint,
// which a run takes once to key the sealed state.
func BenchmarkFingerprint(b *testing.B) {
	b.Run("Fingerprint", func(b *testing.B) {
		w, err := valid().Build()
		assert.NoError(b, err, "the composition composes")
		c := bench.Start(b).MaxAllocs(fingerprintAllocs)
		defer c.End()
		var got []byte
		for c.Loop() {
			got = w.Fingerprint()
		}
		assert.Length(b, got, sha256.Size, "Fingerprint returns the digest")
	})
}

// fingerprinted returns the fingerprint of the composition a builder
// builds, and fails the test where it does not compose.
func fingerprinted(t *testing.T, b *workspace.Builder) []byte {
	t.Helper()

	return built(t, b).Fingerprint()
}

// loading returns the valid composition with two scripted frontends, the
// first and then the second.
func loading() *workspace.Builder {
	return valid().Frontends(scriptedAs(firstFrontend), scriptedAs(secondFrontend))
}

// scriptedAs returns the scripted frontend under a name of its own.
func scriptedAs(name plugin.ID) *frontendtest.Scripted {
	f := frontendtest.NewScripted()
	f.ID = name
	return f
}

// templated returns a generator that mirrors every struct in the gen
// family and declares tree as its template tree for every target.
func templated(name plugin.ID, tree fs.FS) plugin.Generator {
	p, held := eidos.NewPlugin(name).
		Templates(tree).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(mirrored)).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// templatedBuilder returns a composition of one plan whose generator
// declares tree as its template tree.
func templatedBuilder(tree fs.FS) *workspace.Builder {
	return workspace.New().
		Brand(fixtureBrand).
		Targets("fixture").
		Plans(planTo("plan", "fixture", templated("templated", tree)))
}
