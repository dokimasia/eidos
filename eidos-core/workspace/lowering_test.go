// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The target of the lowering composition, its lowering entry, its name
// key, and the width policy that its backend declares with the policy's
// choices.
const (
	lowerTarget plugin.Target    = "tsx"
	lowerEntry  plugin.ID        = "tsx-lowering"
	lowerName   meta.KeyName     = "tsx.name"
	widthKey    plugin.PolicyKey = "tsx.width"
	narrow      plugin.Choice    = "narrow"
	wide        plugin.Choice    = "wide"
	full        plugin.Choice    = "full"
)

// The plan of the lowering composition, its generator and its backend.
const (
	clientPlan              = "client"
	clientMirror  plugin.ID = "client-mirror"
	clientBackend plugin.ID = "tsx-printer"
)

// The lines and files of the lowering trees, beside the lines of the warm
// tree. Each directive line attaches to the struct on the line before it,
// and the package note attaches to the package of its file.
const (
	cacheFile    = "svc/cache/cache.zz"
	strayFile    = "other/stray.zz"
	extraFile    = "svc/extra/extra.zz"
	getLine      = "method Get\n"
	xrayLine     = "type Xray int\n"
	skippedLine  = "+skip plugin=tsx-lowering\n"
	renamedLine  = "+tsx name=fetch\n"
	wideLine     = "+tsx width=wide\n"
	fullLine     = "+tsx width=full\n"
	hugeLine     = "+tsx width=huge\n"
	widePkgNote  = "pkgnote tsx width=wide\n"
	fullPkgNote  = "pkgnote tsx width=full\n"
	carrierLine  = 3
	refusedStart = "X"
)

// widthPolicy is the one lowering policy of the fixture target.
var widthPolicy = plugin.PolicySpec{
	Key: widthKey, Choices: []plugin.Choice{narrow, wide, full}, Default: narrow,
	Doc: "the width of a declaration in the fixture target",
}

// A target's lowering entry stamps the target's name on each declaration
// of another language that a plan of the target admits, and the values of
// the target's directive on its subjects. A warm run stamps again the
// packages and the instances that an edit changed, and leaves the facts
// that a cold run over the edited tree leaves.
func TestLowering(t *testing.T) {
	t.Parallel()

	t.Run("lowerings", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error listing two backends of a target whose backend respells names", func(t *testing.T) {
			t.Parallel()

			_, err := lowerComposition(t, workspace.Sources{}).Plans(workspace.Plan{
				Name: "plain", Generators: []plugin.Generator{mirror("plain-mirror")},
				Backend: printerAs(t, "plain-printer", lowerTarget, ""),
			}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `target "tsx" renders through tsx-printer and plain-printer`,
				"the error lists the target and both backends")
		})

		t.Run("returns no error for two backends of a target without a respell or a policy", func(t *testing.T) {
			t.Parallel()

			w := built(t, valid().Plans(planTo("second", "fixture", mirror("second-mirror"))))
			assert.Equal(t, w.Describe().Annotate, []workspace.Component{{Name: "noter", Bucket: 1}},
				"the composition schedules no lowering entry")
		})

		t.Run("returns an error for a plugin named after the lowering entry", func(t *testing.T) {
			t.Parallel()

			_, err := lowerComposition(t, workspace.Sources{}).Annotators(stamper(lowerEntry, quiet)).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `two plugins return the name "tsx-lowering"`,
				"the error contains the contended name")
		})

		t.Run("returns an entry for a backend that declares policies and does not respell names", func(t *testing.T) {
			t.Parallel()

			w := built(t, policyComposition(t))
			assert.Equal(t, w.Describe().Annotate, []workspace.Component{{Name: lowerEntry, Bucket: 1}},
				"the composition schedules the lowering entry")
		})

		t.Run("returns the entries in the order of their names before every annotator", func(t *testing.T) {
			t.Parallel()

			w := built(t, lowerComposition(t, workspace.Sources{}).
				Annotators(stamper("early", quiet)).
				Targets("ts").
				Plans(workspace.Plan{
					Name: "short", Generators: []plugin.Generator{mirror("short-mirror")},
					Backend: lowerBackend(t, "ts-printer", "ts", camel),
				}))
			want := []workspace.Component{
				{Name: "ts-lowering", Bucket: 1}, {Name: lowerEntry, Bucket: 2}, {Name: "early", Bucket: 3},
			}
			assert.Equal(t, w.Describe().Annotate, want, "the entries run first, in name order")
		})
	})

	t.Run("register", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the name key and the policy key under the target's spelling", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, lowerComposition(t, workspace.Sources{}), lowerTree(rowLine))
			r := report.Facts.Registry()
			claimant, claimed := r.Claimant(string(lowerTarget))
			assert.True(t, claimed, "the target's namespace is claimed")
			expect.Equal(t, claimant, string(lowerTarget), "the target's spelling claims the namespace")
			_, named := meta.Lookup[string](r, lowerName)
			expect.True(t, named, "the name key is registered")
			_, chosen := meta.Lookup[string](r, meta.KeyName(widthKey))
			expect.True(t, chosen, "the policy key is registered")
		})

		t.Run("does not register a name key for a backend that does not respell names", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, policyComposition(t), lowerTree(rowLine))
			_, named := meta.Lookup[string](report.Facts.Registry(), lowerName)
			assert.False(t, named, "the name key is not registered")
		})

		t.Run("registers the target's keys beside the keys of its language's frontend", func(t *testing.T) {
			t.Parallel()

			w := built(t, lowerComposition(t, workspace.Sources{}).Frontends(keyedFrontend{
				Scripted: lowerFrontend(), lang: symbol.Lang(lowerTarget),
				register: func(r *meta.Registry) error {
					if err := r.ClaimNamespace(string(lowerTarget)); err != nil {
						return err
					}
					spec := meta.KeySpec{Name: "tsx.optional", Doc: "marks an optional member"}
					_, err := meta.Register[bool](r, spec)
					return err
				},
			}))
			for _, k := range []meta.KeyName{"tsx.optional", lowerName, meta.KeyName(widthKey)} {
				_, err := w.ParseTarget(string(k) + "@svc/store/row.zz:1")
				expect.NoError(t, err, string(k)+" is registered")
			}
		})

		t.Run("returns an error with the difference of a key of the language's frontend", func(t *testing.T) {
			t.Parallel()

			_, err := lowerComposition(t, workspace.Sources{}).Frontends(keyedFrontend{
				Scripted: lowerFrontend(), lang: symbol.Lang(lowerTarget),
				register: func(r *meta.Registry) error {
					if err := r.ClaimNamespace(string(lowerTarget)); err != nil {
						return err
					}
					for _, name := range []meta.KeyName{lowerName, meta.KeyName(widthKey)} {
						spec := meta.KeySpec{Name: name, Doc: "a key of the frontend"}
						if _, err := meta.Register[string](r, spec); err != nil {
							return err
						}
					}
					return nil
				},
			}).Build()
			assert.HasError(t, err, "the composition fails")
			expect.Contains(t, err.Error(), `key "tsx.name" is registered twice with the docs`,
				"the error contains the difference of the name key")
			expect.Contains(t, err.Error(), `key "tsx.width" is registered twice with the docs`,
				"the error contains the difference of the policy key")
		})

		t.Run("returns an error for a target whose namespace another registrant claims", func(t *testing.T) {
			t.Parallel()

			_, err := workspace.New().
				Brand(fixtureBrand).
				Frontends(frontendtest.NewScripted()).
				Targets(meta.KernelNamespace).
				Plans(workspace.Plan{
					Name: clientPlan, Generators: []plugin.Generator{mirror(clientMirror)},
					Backend: lowerBackend(t, clientBackend, meta.KernelNamespace, camel),
				}).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `namespace "gen" is claimed twice: by "eidos-core" and by "gen"`,
				"the error contains the kernel and the target")
		})

		t.Run("returns an error for the composition's claim of the target's namespace", func(t *testing.T) {
			t.Parallel()

			_, err := lowerComposition(t, workspace.Sources{}).
				Keys(func(r *meta.Registry) error { return r.ClaimNamespace(string(lowerTarget)) }).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `namespace "tsx" is claimed twice: by "tsx" and by the composition`,
				"the error contains both claimants")
		})
	})

	t.Run("schema", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a directive that refuses a value outside the policy's choices", func(t *testing.T) {
			t.Parallel()

			report, err := built(t, lowerComposition(t, workspace.Sources{})).
				Run(t.Context(), workspace.Input{Tree: lowerTree(rowLine, hugeLine)})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused value fails the run")
			refused := findings(report.Sink, directive.BadSpelling)
			assert.Length(t, refused, 1, "the run reports the refused value")
			assert.Contains(t, refused[0].Msg, "narrow, wide, full", "the finding lists the choices")
		})

		t.Run("returns a directive without a name param for a backend that does not respell names", func(t *testing.T) {
			t.Parallel()

			report, err := built(t, policyComposition(t)).
				Run(t.Context(), workspace.Input{Tree: lowerTree(rowLine, renamedLine)})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the unknown param fails the run")
			assert.Length(t, findings(report.Sink, directive.UnknownKey), 1, "the run reports the name param")
		})

		t.Run("returns an error for a target named after a directive of the kernel", func(t *testing.T) {
			t.Parallel()

			skip := plugin.Target(directive.KernelSkip)
			_, err := workspace.New().
				Brand(fixtureBrand).
				Targets(skip).
				Plans(workspace.Plan{
					Name: clientPlan, Generators: []plugin.Generator{mirror(clientMirror)},
					Backend: lowerBackend(t, clientBackend, skip, camel),
				}).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "target skip is a kernel name, which no target takes",
				"the error contains the target")
		})

		t.Run("returns a directive that no composition ignores", func(t *testing.T) {
			t.Parallel()

			_, err := lowerComposition(t, workspace.Sources{}).Ignore(directive.Name(lowerTarget)).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "tsx is a kernel name", "the error is about the target's directive")
		})

		t.Run("returns a directive whose name no plugin's schema takes", func(t *testing.T) {
			t.Parallel()

			_, err := lowerComposition(t, workspace.Sources{}).Plans(planTo("picky", "fixture",
				schemad("picky", directive.Schema{Plugin: "picky", Name: "tsx", Doc: "claims the target's name"}))).
				Targets("fixture").
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "tsx is a kernel name", "the error is about the plugin's schema")
		})
	})

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the respelled name of each struct, field and method of an admitted package", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, lowerComposition(t, workspace.Sources{}), lowerTree(rowLine, getLine))
			want := map[string]string{
				"Row": "row", "Row.f0": "f0", "Row.f1": "f1", "Row.Get": "get", "Cache": "cache", "Cache.f0": "f0",
			}
			expect.Equal(t, targetFacts(t, report.Facts, lowerName), want, "each declaration has its target name")
			expect.Equal(t, invoked(report, lowerEntry), 2, "the entry runs once for each package")
		})

		t.Run("does not stamp a name on a declaration whose name the respell refuses", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, lowerComposition(t, workspace.Sources{}), lowerTree(xrayLine))
			names := targetFacts(t, report.Facts, lowerName)
			expect.NotContains(t, slices.Collect(maps.Keys(names)), "Xray", "the refused struct has no name")
			expect.Equal(t, names["Xray.f0"], "f0", "the field of the refused struct has its name")
		})

		t.Run("does not stamp a name on a declaration that a skip excludes from the entry", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, lowerComposition(t, workspace.Sources{}), lowerTree(rowLine, skippedLine))
			names := targetFacts(t, report.Facts, lowerName)
			expect.NotContains(t, slices.Collect(maps.Keys(names)), "Row", "the skipped struct has no name")
			expect.Equal(t, names["Row.f0"], "f0", "the field of the skipped struct has its name")
		})

		t.Run("does not stamp a name on a package of the target's language", func(t *testing.T) {
			t.Parallel()

			own := plugin.Target(frontendtest.ScriptedLang)
			report := lowerRun(t, workspace.New().
				Brand(fixtureBrand).
				Frontends(frontendtest.NewScripted()).
				Targets(own).
				Plans(workspace.Plan{
					Name: clientPlan, Generators: []plugin.Generator{mirror(clientMirror)},
					Backend: lowerBackend(t, clientBackend, own, camel),
				}), lowerTree(rowLine))
			names := targetFacts(t, report.Facts, own.NameKey())
			assert.Empty(t, names, "a package of the target's language has no name")
		})

		t.Run("does not stamp a name on a package that the plan's sources leave out", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, lowerComposition(t, workspace.Sources{Packages: []string{"svc/cache"}}),
				lowerTree(rowLine))
			want := map[string]string{"Cache": "cache", "Cache.f0": "f0"}
			assert.Equal(t, targetFacts(t, report.Facts, lowerName), want, "the store package has no name")
		})

		t.Run("does not stamp a name on a package that a store provides", func(t *testing.T) {
			t.Parallel()

			w := built(t, workspace.New().
				Brand(fixtureBrand).
				Frontends(frontendtest.NewScriptedDependent()).
				Targets(lowerTarget).
				Plans(workspace.Plan{
					Name: clientPlan, Generators: []plugin.Generator{mirror(clientMirror)},
					Backend: lowerBackend(t, clientBackend, lowerTarget, camel, widthPolicy),
				}))
			report, err := w.Run(t.Context(), workspace.Input{
				Tree: fstest.MapFS{"svc/a/a.zz": {Data: []byte("package svc/a\nimport dep dep/b\ntype A dep.B\n")}},
				Stores: map[string]fs.FS{
					frontendtest.ScriptedStore: fstest.MapFS{"dep/b/b.zz": {Data: []byte("package dep/b\ntype B\n")}},
				},
			})
			assert.NoError(t, err, "the run is clean")
			want := map[string]string{"A": "a", "A.f0": "f0"}
			assert.Equal(t, targetFacts(t, report.Facts, lowerName), want, "the dependency's struct has no name")
		})

		t.Run("ranks the name of the target's directive above the entry's stamp", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, lowerComposition(t, workspace.Sources{}), lowerTree(rowLine, renamedLine))
			key, _ := meta.Lookup[string](report.Facts.Registry(), lowerName)
			claims := slices.Collect(report.Facts.Claims(rowID, key.ID()))
			assert.Length(t, claims, 2, "the row has the directive's claim and the entry's claim")
			expect.Equal(t, claims[0].Value, any("fetch"), "the directive's name ranks first")
			expect.Equal(t, claims[0].Claim.Authority, meta.AuthorityDirective,
				"the first claim has directive authority")
			expect.Equal(t, claims[0].Claim.Pos.Line, carrierLine, "the first claim has the carrier's position")
			expect.Equal(t, claims[1].Value, any("row"), "the entry's claim has the respell as its value")
			expect.Equal(t, claims[1].Claim.Authority, meta.AuthorityPlugin, "the entry's claim has plugin authority")
			expect.Equal(t, claims[1].Claim.Plugin, lowerEntry, "the entry makes the claim")
			expect.Equal(t, claims[1].Claim.Order, meta.Order{Subject: storePackage}, "the package orders the claim")
			expect.Equal(t, claims[1].Claim.Derived, []meta.Read{{Subject: rowID}}, "the claim derives from the row")
		})

		t.Run("stamps the policy value of the target's directive on its subject", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, lowerComposition(t, workspace.Sources{}), lowerTree(rowLine, wideLine))
			key, _ := meta.Lookup[string](report.Facts.Registry(), meta.KeyName(widthKey))
			got, held := meta.GetAtLeast(report.Facts, rowID, key, meta.AuthorityDirective)
			assert.True(t, held, "the row has the directive's width")
			assert.Equal(t, got, string(wide), "the width is the directive's value")
		})

		t.Run("stamps the value of the target's directive on a subject that a skip excludes", func(t *testing.T) {
			t.Parallel()

			tree := lowerTree(rowLine, skippedLine, wideLine)
			report := lowerRun(t, lowerComposition(t, workspace.Sources{}), tree)
			want := map[string]string{"Row": string(wide)}
			assert.Equal(t, targetFacts(t, report.Facts, meta.KeyName(widthKey)), want,
				"the directive applies beside the skip")
		})

		t.Run("stamps the policy value of the target's directive on a package", func(t *testing.T) {
			t.Parallel()

			report := lowerRun(t, lowerComposition(t, workspace.Sources{}), lowerTree(widePkgNote, rowLine))
			key, _ := meta.Lookup[string](report.Facts.Registry(), meta.KeyName(widthKey))
			got, held := meta.Get(report.Facts, storePackage, key)
			assert.True(t, held, "the package has the directive's width")
			assert.Equal(t, got, string(wide), "the width is the directive's value")
		})

		t.Run("stamps the names of every kind that rule 0 covers", func(t *testing.T) {
			t.Parallel()

			w := built(t, workspace.New().
				Brand(fixtureBrand).
				Targets(lowerTarget).
				Plans(workspace.Plan{
					Name: clientPlan, Generators: []plugin.Generator{mirror(clientMirror)},
					Backend: lowerBackend(t, clientBackend, lowerTarget, camel, widthPolicy),
				}))
			report := cleanRun(t, w, coretest.Frozen(t, everyNamedKind()))
			want := map[string]string{
				"Row": "row", "Row.Column": "column", "Row.Scan": "scan", "Row.Close": "close",
				"Row.Cell": "rowCell", "Row.Cell.Width": "width", "Reader": "reader", "Reader.Label": "label",
				"Reader.Scan": "scan", "Reader.Shade": "readerShade", "Reader.Shade.Tone": "tone",
				"Status": "status", "Status.StatusOpen": "statusOpen", "Status.Code": "code",
				"Status.Describe": "describe", "Result": "result", "Result.ResultOk": "resultOk",
				"ResultOk.Value": "value", "Result.Unwrap": "unwrap", "RowID": "rowID", "Load": "load",
				"Registry": "registry", "Version": "version",
			}
			assert.Equal(t, targetFacts(t, report.Facts, lowerName), want,
				"each declaration, field, method and variant has its name, and a nested type has its flat name")
		})
	})

	t.Run("selected", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps again the names of the package that an edit changed alone", func(t *testing.T) {
			t.Parallel()

			report, err := warmAfter(t, built(t, sealedLowering(t, workspace.Sources{})),
				lowerTree(rowLine), lowerTree(widerRow))
			assert.NoError(t, err, "the run is clean")
			expect.Equal(t, invoked(report, lowerEntry), 1, "the entry runs on the store package alone")
			expect.Equal(t, targetFacts(t, report.Facts, lowerName)["Row.f2"], "f2", "the new field has its name")
		})

		t.Run("stamps the names of a package that the edit added", func(t *testing.T) {
			t.Parallel()

			after := lowerTree(rowLine)
			after[extraFile] = &fstest.MapFile{Data: []byte("package svc/extra\ntype Extra int\n")}
			report, err := warmAfter(t, built(t, sealedLowering(t, workspace.Sources{})), lowerTree(rowLine), after)
			assert.NoError(t, err, "the run is clean")
			expect.Equal(t, invoked(report, lowerEntry), 1, "the entry runs on the new package alone")
			expect.Equal(t, targetFacts(t, report.Facts, lowerName)["Extra"], "extra", "the new struct has its name")
		})

		t.Run("withdraws the names of a package that the edit removed", func(t *testing.T) {
			t.Parallel()

			after := lowerTree(rowLine)
			delete(after, cacheFile)
			report, err := warmAfter(t, built(t, sealedLowering(t, workspace.Sources{})), lowerTree(rowLine), after)
			assert.NoError(t, err, "the run is clean")
			want := map[string]string{"Row": "row", "Row.f0": "f0", "Row.f1": "f1"}
			assert.Equal(t, targetFacts(t, report.Facts, lowerName), want, "the cache package has no name")
		})

		t.Run("stamps the names of a package that a removed file let into the plan's sources", func(t *testing.T) {
			t.Parallel()

			before := lowerTree(rowLine)
			before[strayFile] = &fstest.MapFile{Data: []byte("package svc/store\ntype Stray int\n")}
			report, err := warmAfter(t, built(t, sealedLowering(t, workspace.Sources{Packages: []string{"svc/..."}})),
				before, lowerTree(rowLine))
			assert.NoError(t, err, "the run is clean")
			expect.Equal(t, targetFacts(t, report.Facts, lowerName)["Row"], "row", "the admitted package has its names")
		})

		t.Run("withdraws the names of a package that a new file moves out of the plan's sources", func(t *testing.T) {
			t.Parallel()

			after := lowerTree(rowLine)
			after[strayFile] = &fstest.MapFile{Data: []byte("package svc/store\ntype Stray int\n")}
			w := built(t, sealedLowering(t, workspace.Sources{Packages: []string{"svc/..."}}))
			report, err := warmAfter(t, w, lowerTree(rowLine), after)
			assert.NoError(t, err, "the run is clean")
			want := map[string]string{"Cache": "cache", "Cache.f0": "f0"}
			assert.Equal(t, targetFacts(t, report.Facts, lowerName), want, "the store package has no name")
		})

		t.Run("stamps again the value of an edited directive instance", func(t *testing.T) {
			t.Parallel()

			report, err := warmAfter(t, built(t, sealedLowering(t, workspace.Sources{})),
				lowerTree(rowLine, wideLine), lowerTree(rowLine, fullLine))
			assert.NoError(t, err, "the run is clean")
			want := map[string]string{"Row": string(full)}
			assert.Equal(t, targetFacts(t, report.Facts, meta.KeyName(widthKey)), want, "the row has the edited width")
		})

		t.Run("withdraws the value of a removed directive instance", func(t *testing.T) {
			t.Parallel()

			report, err := warmAfter(t, built(t, sealedLowering(t, workspace.Sources{})),
				lowerTree(rowLine, wideLine), lowerTree(rowLine))
			assert.NoError(t, err, "the run is clean")
			assert.Empty(t, targetFacts(t, report.Facts, meta.KeyName(widthKey)), "no subject has a width")
		})

		t.Run("stamps again the value of an edited directive on a package", func(t *testing.T) {
			t.Parallel()

			report, err := warmAfter(t, built(t, sealedLowering(t, workspace.Sources{})),
				lowerTree(widePkgNote, rowLine), lowerTree(fullPkgNote, rowLine))
			assert.NoError(t, err, "the run is clean")
			want := map[string]string{"": string(full)}
			assert.Equal(t, targetFacts(t, report.Facts, meta.KeyName(widthKey)), want,
				"the package has the edited width")
		})

		t.Run("leaves the names and the values that a cold run over the edited tree leaves", func(t *testing.T) {
			t.Parallel()

			edited := lowerTree(widerRow, fullLine, getLine)
			warm, err := warmAfter(t, built(t, sealedLowering(t, workspace.Sources{})),
				lowerTree(rowLine, wideLine), edited)
			assert.NoError(t, err, "the warm run is clean")
			cold := sealedRun(t, built(t, sealedLowering(t, workspace.Sources{})), workspace.Input{Tree: edited})
			for _, k := range []meta.KeyName{lowerName, meta.KeyName(widthKey)} {
				expect.Equal(t, targetFacts(t, warm.Facts, k), targetFacts(t, cold.Facts, k),
					"the warm run leaves the same "+string(k)+" facts as the cold run")
			}
		})
	})
}

// camel spells a name with its first letter in lower case, as the fixture
// target writes names. It refuses a name that opens with refusedStart.
func camel(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	if strings.HasPrefix(name, refusedStart) {
		return "", errors.New("the fixture target does not spell a name that starts with " + refusedStart)
	}
	return strings.ToLower(name[:1]) + name[1:], nil
}

// lowerBackend returns a printer of target under name, which declares
// the policies and respells names through respell, where respell is not
// nil.
func lowerBackend(
	tb assert.TB, name plugin.ID, target plugin.Target, respell plugin.Respell, policies ...plugin.PolicySpec,
) plugin.Backend {
	tb.Helper()

	b := backend.New(name, target, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the fixture does not spell statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: totalCoverage()}).
		Policies(policies...)
	if respell != nil {
		b = b.Respell(respell)
	}
	return b.Build()
}

// lowerComposition returns the builder of the lowering composition: the
// scripted frontend, and the client plan, which mirrors each struct of
// sources through a printer of the fixture target that respells names
// and declares the width policy.
func lowerComposition(tb assert.TB, sources workspace.Sources) *workspace.Builder {
	tb.Helper()

	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Targets(lowerTarget).
		Plans(workspace.Plan{
			Name: clientPlan, Sources: sources, Generators: []plugin.Generator{mirror(clientMirror)},
			Backend: lowerBackend(tb, clientBackend, lowerTarget, camel, widthPolicy),
		})
}

// lowerFrontend returns the scripted frontend under the name of a
// frontend of the fixture target's language, beside the composition's
// scripted frontend.
func lowerFrontend() *frontendtest.Scripted {
	f := frontendtest.NewScripted()
	f.ID = "tsxfront"
	return f
}

// policyComposition returns the lowering composition with a printer that
// declares the width policy and does not respell names.
func policyComposition(tb assert.TB) *workspace.Builder {
	tb.Helper()

	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Targets(lowerTarget).
		Plans(workspace.Plan{
			Name: clientPlan, Generators: []plugin.Generator{mirror(clientMirror)},
			Backend: lowerBackend(tb, clientBackend, lowerTarget, nil, widthPolicy),
		})
}

// sealedLowering returns the lowering composition over sources, which
// writes its output and records its runs into a ledger of its own.
func sealedLowering(tb assert.TB, sources workspace.Sources) *workspace.Builder {
	tb.Helper()

	l := ledger.NewMem()
	return lowerComposition(tb, sources).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// lowerTree returns a tree of the store package, whose file contains the
// lines, and of the cache package, which declares Cache.
func lowerTree(lines ...string) fstest.MapFS {
	return fstest.MapFS{
		warmSource: {Data: []byte("package svc/store\n" + strings.Join(lines, ""))},
		cacheFile:  {Data: []byte("package svc/cache\ntype Cache int\n")},
	}
}

// lowerRun builds a composition, runs it over a tree, and fails the test
// where the run is not clean.
func lowerRun(t *testing.T, b *workspace.Builder, tree fstest.MapFS) *workspace.Report {
	t.Helper()

	report, err := built(t, b).Run(t.Context(), workspace.Input{Tree: tree})
	assert.NoError(t, err, "the run is clean")
	return report
}

// targetFacts returns the value of a string key on each subject that has
// it in a run's facts. The subject's name keys each value, after the name
// of its owner and a dot for a member.
func targetFacts(tb assert.TB, facts *meta.Facts, name meta.KeyName) map[string]string {
	tb.Helper()

	key, registered := meta.Lookup[string](facts.Registry(), name)
	assert.True(tb, registered, "the composition registers "+string(name))
	out := map[string]string{}
	for id := range facts.ByKey(key.ID()) {
		v, _ := meta.Get(facts, id, key)
		if id.Owner != "" {
			out[id.Owner+"."+id.Name] = v
			continue
		}
		out[id.Name] = v
	}
	return out
}

// everyNamedKind returns the every-kind package with a member of each
// list that rule 0 of the lowering entry covers: a nested type with a
// field in the struct and in the interface, a field of the interface, a
// field and a method of the enum, a field of the sum's variant, a method
// of the sum, and a method at file level.
func everyNamedKind() *node.Package {
	const path = coretest.StorePath
	pkg := coretest.EveryKind(path)
	decls := pkg.Files[0].Decls
	for _, d := range decls {
		switch x := d.(type) {
		case *node.Struct:
			cell := coretest.Struct(path, "Cell")
			cell.ID = coretest.MemberID(path, coretest.StructName, "Cell", symbol.KindStruct)
			cell.Fields = []*node.Field{coretest.Field(path, coretest.StructName+symbol.OwnerSep+"Cell", "Width")}
			x.Types = node.Symbols{cell}
		case *node.Interface:
			shade := coretest.Struct(path, "Shade")
			shade.ID = coretest.MemberID(path, coretest.InterfaceName, "Shade", symbol.KindStruct)
			shade.Fields = []*node.Field{coretest.Field(path, coretest.InterfaceName+symbol.OwnerSep+"Shade", "Tone")}
			x.Types = node.Symbols{shade}
			x.Fields = []*node.Field{coretest.Field(path, coretest.InterfaceName, "Label")}
		case *node.Enum:
			x.Fields = []*node.Field{coretest.Field(path, coretest.EnumName, "Code")}
			x.Methods = []*node.Method{coretest.Method(path, coretest.EnumName, "Describe")}
		case *node.Sum:
			x.Variants[0].Fields = []*node.Field{coretest.Field(path, coretest.SumVariantName, "Value")}
			x.Methods = []*node.Method{coretest.Method(path, coretest.SumName, "Unwrap")}
		}
	}
	pkg.Files[0].Decls = append(pkg.Files[0].Decls, coretest.Method(path, coretest.StructName, "Close"))
	return pkg
}
