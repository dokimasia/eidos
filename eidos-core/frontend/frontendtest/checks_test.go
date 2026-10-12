// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest_test

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// ghostPath and ghostName are the identity of the declaration that the
// dangling frontend attaches to, which no load declares.
const (
	ghostPath = "svc/ghost"
	ghostName = "Ghost"
)

// These constants are the contracts of the checks, as their records
// contain them, for the cases that two checks or two fixtures break in
// the same way.
const (
	shallowRoot  = "a stated root loads at least one unit shallow"
	linksNothing = "the fixture's in-graph spellings resolve: a Resolve that returns no candidate links nothing"
)

// errRegistration is the error of a registration of keys that fails.
var errRegistration = errors.New("frontendtest_test: the registration of the keys fails")

// keyedScripted is the scripted frontend that registers its
// classification keys through its own role.
type keyedScripted struct{ *frontendtest.Scripted }

// Keys registers the scripted classification keys.
func (keyedScripted) Keys(r *meta.Registry) error { return frontendtest.ScriptedKeys(r) }

// The checks exist to catch broken frontends, so each case simulates a
// broken frontend and asserts the failure of the check.
func TestChecks(t *testing.T) {
	t.Parallel()

	t.Run("AssertDeterministicParse", func(t *testing.T) {
		t.Parallel()

		for _, record := range []string{recordStamp, recordDirective, recordFinding} {
			t.Run("rejects a "+record+" that changes between loads", func(t *testing.T) {
				t.Parallel()

				got := assert.Rejects(t, "a frontend recording something new per load", func(tb assert.TB) {
					frontendtest.AssertDeterministicParse(tb, over(&restless{
						Scripted: frontendtest.NewScripted(), record: record,
					}, fixture()))
				})
				assert.Length(t, got, 1, "the check fails once")
				expect.Equal(t, got[0].Assertion, "deterministic", "on the determinism of the parse")
				expect.Equal(t, got[0].Contract, "every parse of one fixture encodes the same graph, attaches "+
					"the same directives and stamps, and reports the same findings in the same order",
					"the record has the contract of the check")
			})
		}
	})

	t.Run("AssertPositionedDiagnostics", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a finding with no position", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a finding no reader can open", func(tb assert.TB) {
				frontendtest.AssertPositionedDiagnostics(tb, over(&unpositioned{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.Contains(t, coretest.Contracts(got), "the finding has a position: "+
				frontendtest.ScriptedBadFile.String()+" a finding with nowhere to point",
				"the rejection names the finding without a position")
		})

		t.Run("rejects a finding with no origin", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a finding nobody signed", func(tb assert.TB) {
				frontendtest.AssertPositionedDiagnostics(tb, over(&anonymous{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.HasPrefix(t, got[0].Contract, "the finding has an origin: ",
				"the rejection names the finding without an origin")
		})
	})

	t.Run("AssertClassified", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a silently dropped file", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a parse that swallows a file", func(tb assert.TB) {
				frontendtest.AssertClassified(tb, over(&swallower{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.HasSuffix(t, got[0].Contract, " declares into the graph or a finding names it: "+
				"nothing drops in silence", "the rejection names the dropped file")
		})

		t.Run("rejects a stamp under a key that neither the frontend nor the fixture registers", func(t *testing.T) {
			t.Parallel()

			keyless := fixture()
			keyless.Keys = nil
			got := assert.Rejects(t, "stamps without keys", func(tb assert.TB) {
				frontendtest.AssertClassified(tb, setupOver(keyless))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.Equal(t, got[0].Contract, "a recorded stamp applies under the keys of the frontend and the fixture",
				"the rejection is about the stamp that nothing could apply")
		})

		t.Run("rejects keys declared over a load that stamps nothing", func(t *testing.T) {
			t.Parallel()

			unstamped := plainFixture()
			unstamped.Keys = frontendtest.ScriptedKeys
			got := assert.Rejects(t, "keys without stamps", func(tb assert.TB) {
				frontendtest.AssertClassified(tb, setupOver(unstamped))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"the load stamps where the fixture declares classification keys to apply the stamps under"},
				"the rejection is about the contract between the fixture and the load")
		})

		t.Run("applies the stamps of a frontend that registers its keys through its role", func(t *testing.T) {
			t.Parallel()

			keyless := fixture()
			keyless.Keys = nil
			frontendtest.AssertClassified(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return keyedScripted{frontendtest.NewScripted()}, keyless
			})
		})
	})

	t.Run("Registry", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the kernel's keys", func(t *testing.T) {
			t.Parallel()

			_, kernel := frontendtest.Registry(t, frontendtest.NewScripted())
			assert.False(t, kernel.IsZero(), "the handles of the kernel's keys are set")
		})

		t.Run("registers the keys of the frontend under the spelling of its language", func(t *testing.T) {
			t.Parallel()

			registry, _ := frontendtest.Registry(t, keyedScripted{frontendtest.NewScripted()})
			claimant, claimed := registry.Claimant(frontendtest.ScriptedTestKey.Namespace())
			assert.True(t, claimed, "the frontend claims its namespace")
			assert.Equal(t, claimant, string(frontendtest.ScriptedLang),
				"the spelling of the language claims the namespace")
		})

		t.Run("registers the keys of each function", func(t *testing.T) {
			t.Parallel()

			registry, _ := frontendtest.Registry(t, frontendtest.NewScripted(), frontendtest.ScriptedKeys)
			_, held := registry.Resolve(frontendtest.ScriptedTestKey)
			assert.True(t, held, "the key of the function is registered")
		})

		t.Run("skips a nil function", func(t *testing.T) {
			t.Parallel()

			registry, _ := frontendtest.Registry(t, frontendtest.NewScripted(), nil)
			_, held := registry.Resolve(frontendtest.ScriptedTestKey)
			assert.False(t, held, "the registry has no scripted key")
		})

		t.Run("stops the test where a function fails", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a registration of keys that fails", func(tb assert.TB) {
				frontendtest.Registry(tb, frontendtest.NewScripted(), func(*meta.Registry) error {
					return errRegistration
				})
			})
			assert.Equal(t, coretest.Contracts(got), []string{"the fixture's keys register"},
				"the rejection names the registration")
		})
	})

	t.Run("AssertOwnedExcluded", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a claim that excludes the stamped copy", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a selection the check cannot place a copy under", func(tb assert.TB) {
				frontendtest.AssertOwnedExcluded(tb, over(&carved{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"the stamped copy is beside its source, inside the claim"},
				"the rejection names the placement")
		})
	})

	t.Run("AssertFingerprinted", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a frontend that declares no options", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertFingerprinted(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return optionless{frontendtest.NewScripted()}, plainFixture()
			})
		})

		t.Run("passes a language that refuses the perturbed byte", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertFingerprinted(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return &strict{frontendtest.NewScripted()}, plainFixture()
			})
		})

		t.Run("passes a fixture that loads every unit signature-only", func(t *testing.T) {
			t.Parallel()

			shallow := plainFixture()
			shallow.Signatures = []string{serviceRoot}
			frontendtest.AssertFingerprinted(t, setupOver(shallow))
		})

		t.Run("rejects a fixture tree it cannot copy", func(t *testing.T) {
			t.Parallel()

			unreadable := plainFixture()
			unreadable.Sources = failingFS{
				tree: fstest.MapFS{
					apiFile:   {Data: []byte(apiSource)},
					storeFile: {Data: []byte(crossSource)},
					notesFile: {Data: []byte("not source\n")},
				},
				fail: notesFile,
			}
			got := assert.Rejects(t, "a tree the perturbation cannot copy", func(tb assert.TB) {
				frontendtest.AssertFingerprinted(tb, setupOver(unreadable))
			})
			assert.Equal(t, coretest.Contracts(got), []string{"the fixture tree copies"},
				"the rejection names the step that could not run")
		})

		t.Run("rejects a unit missing from a load it is compared with", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a partition that merges its units after two loads", func(tb assert.TB) {
				frontendtest.AssertFingerprinted(tb, over(&drifter{Scripted: frontendtest.NewScripted()},
					plainFixture()))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.HasPrefix(t, got[0].Contract, "the load compared with contains the unit of ",
				"a key compared with nothing proves nothing")
		})
	})

	t.Run("AssertJailedReads", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a frontend declaring what it never read through the unit", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a parse whose bytes come from outside the door", func(tb assert.TB) {
				frontendtest.AssertJailedReads(tb, over(&unread{frontendtest.NewScripted()}, plainFixture()))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.HasPrefix(t, got[0].Contract, "every unit whose members changed folds the new bytes, "+
				"because its parse read them through the unit: ", "the rejection names the door")
		})

		t.Run("passes a language that refuses the perturbed byte", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertJailedReads(t, over(&strict{frontendtest.NewScripted()}, plainFixture()))
		})

		t.Run("rejects a fixture tree it cannot walk", func(t *testing.T) {
			t.Parallel()

			unwalkable := plainFixture()
			unwalkable.Sources = failingFS{tree: fstest.MapFS{apiFile: {Data: []byte(apiSource)}}, fail: "."}
			got := assert.Rejects(t, "a tree the selection cannot be read over", func(tb assert.TB) {
				frontendtest.AssertJailedReads(tb, setupOver(unwalkable))
			})
			assert.Equal(t, coretest.Contracts(got), []string{"the fixture tree walks"},
				"the rejection names the step that could not run")
		})

		t.Run("passes a fixture whose selection claims one file", func(t *testing.T) {
			t.Parallel()

			lone := plainFixture()
			lone.Sources = fstest.MapFS{apiFile: {Data: []byte(apiSource)}}
			frontendtest.AssertJailedReads(t, setupOver(lone))
		})
	})

	t.Run("AssertSignatureDepth", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a fixture whose root covers every unit", func(t *testing.T) {
			t.Parallel()

			deep := plainFixture()
			deep.Signatures = []string{serviceRoot}
			frontendtest.AssertSignatureDepth(t, setupOver(deep))
		})

		t.Run("rejects a stated root that contains no unit", func(t *testing.T) {
			t.Parallel()

			astray := plainFixture()
			astray.Signatures = []string{absentRoot}
			got := assert.Rejects(t, "a signature root the load never applies", func(tb assert.TB) {
				frontendtest.AssertSignatureDepth(tb, setupOver(astray))
			})
			assert.Equal(t, coretest.Contracts(got), []string{shallowRoot},
				"the rejection reports the missing shallow unit")
		})

		t.Run("rejects a stated root that contains no unit beside a dependency unit", func(t *testing.T) {
			t.Parallel()

			astray := dependentFixture()
			astray.Signatures = []string{absentRoot}
			got := assert.Rejects(t, "a signature root the load never applies", func(tb assert.TB) {
				frontendtest.AssertSignatureDepth(tb, over(frontendtest.NewScriptedDependent(), astray))
			})
			assert.Equal(t, coretest.Contracts(got), []string{shallowRoot},
				"a dependency unit loads shallow under any root, so it proves nothing about the root")
		})

		t.Run("passes a fixture that lists what the signature-only load drops", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertSignatureDepth(t, setup)
		})

		t.Run("rejects a listed identity the signature-only load keeps", func(t *testing.T) {
			t.Parallel()

			kept := depIdentity(depType, symbol.KindStruct)
			fx := fixture()
			fx.Dropped = []symbol.Identity{kept}
			got := assert.Rejects(t, "a depth that drops nothing the fixture lists", func(tb assert.TB) {
				frontendtest.AssertSignatureDepth(tb, setupOver(fx))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"a signature-only load drops a listed identity: " + kept.String()},
				"the rejection names the identity kept")
		})

		t.Run("rejects a listed identity the full load does not declare", func(t *testing.T) {
			t.Parallel()

			ghost := depIdentity(ghostName, symbol.KindConstant)
			fx := fixture()
			fx.Dropped = []symbol.Identity{ghost}
			got := assert.Rejects(t, "a listing of a declaration no load makes", func(tb assert.TB) {
				frontendtest.AssertSignatureDepth(tb, setupOver(fx))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"a listed identity loads at full depth: " + ghost.String()},
				"the rejection names the missing identity")
		})
	})

	t.Run("AssertAttachedDirectives", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects an instance that fails validation", func(t *testing.T) {
			t.Parallel()

			mistyped := fixture()
			mistyped.Sources = fstest.MapFS{
				storeFile: {Data: []byte(
					"package svc/store\ntype Row string\n+gen:table wrong=x\n",
				)},
			}
			mistyped.Signatures = nil
			got := assert.Rejects(t, "a carrier writing a key no schema declares", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, setupOver(mistyped))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.HasPrefix(t, got[0].Contract, "an attached instance passes validation: ",
				"the rejection names the class")
		})

		t.Run("rejects a carrier line left in documentation", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a frontend that attaches without stripping", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, over(&undocumented{
					Scripted: frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.That(t, got[0].Contract).Contains(" keeps no carrier line in its documentation: ",
				"the rejection names the class").HasSuffix(strconv.Quote(carrierLine), "and the line left behind")
		})

		marks := []struct {
			name string
			line string
		}{
			{name: "rejects a carrier line left with its set mark", line: setCarrier},
			{name: "rejects a carrier line left with its negated mark", line: negatedCarrier},
		}
		for _, tt := range marks {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := assert.Rejects(t, "a documentation line the strip never saw", func(tb assert.TB) {
					frontendtest.AssertAttachedDirectives(tb, over(&undocumented{
						Scripted: frontendtest.NewScripted(), line: tt.line,
					}, fixture()))
				})
				assert.NotEmpty(t, got, "the check fails")
				assert.HasSuffix(t, got[0].Contract, strconv.Quote(tt.line), "the mark does not hide the leak")
			})
		}

		t.Run("rejects directives on a subject the graph does not contain", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "an attachment to a declaration nothing declares", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, over(&dangling{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			ghost := symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: ghostPath, Name: ghostName, Kind: symbol.KindStruct,
			}
			assert.Contains(t, coretest.Contracts(got), "the graph contains "+ghost.String()+", which directives name",
				"the rejection names the subject")
		})

		t.Run("passes a fixture that declares schemas without keys", func(t *testing.T) {
			t.Parallel()

			keyless := plainFixture()
			keyless.Sources = fstest.MapFS{
				apiFile:   {Data: []byte(apiSource)},
				storeFile: {Data: []byte(crossSource + carrierStatement)},
			}
			keyless.Schemas = frontendtest.ScriptedSchemas()
			frontendtest.AssertAttachedDirectives(t, setupOver(keyless))
		})

		t.Run("rejects a fixture whose carriers attach nothing", func(t *testing.T) {
			t.Parallel()

			barren := plainFixture()
			barren.Schemas = frontendtest.ScriptedSchemas()
			got := assert.Rejects(t, "schemas no carrier in the tree writes", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, setupOver(barren))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"the fixture declares schemas, so its carriers attach something"},
				"the rejection reports the missing attachment")
		})
	})

	t.Run("AssertLinked", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a Resolve that returns no candidate", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a mute resolver over a two-package fixture", func(tb assert.TB) {
				frontendtest.AssertLinked(tb, over(&mute{frontendtest.NewScripted()}, fixture()))
			})
			assert.Equal(t, coretest.Contracts(got), []string{linksNothing}, "the rejection names what never happened")
		})

		t.Run("rejects a mute Resolve over a single-package fixture", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a mute resolver with no second package to hide behind",
				func(tb assert.TB) {
					frontendtest.AssertLinked(tb, over(&mute{frontendtest.NewScripted()}, singleFixture()))
				})
			assert.Equal(t, coretest.Contracts(got), []string{linksNothing},
				"in-graph spellings resolve in one package too")
		})

		t.Run("rejects a Resolve that returns nothing across exactly two packages", func(t *testing.T) {
			t.Parallel()

			local := plainFixture()
			local.Sources = fstest.MapFS{
				apiFile:   {Data: []byte(apiSource)},
				storeFile: {Data: []byte(crossSource + localStatement)},
			}
			got := assert.Rejects(t, "a resolver returning candidates in its own package alone",
				func(tb assert.TB) {
					frontendtest.AssertLinked(tb, over(&insular{frontendtest.NewScripted()}, local))
				})
			assert.Equal(t, coretest.Contracts(got), []string{
				"a multi-package fixture resolves across its packages, or its Resolve never returns a candidate",
			}, "two packages are already across packages")
		})

		t.Run("rejects an unresolved reference that spells nothing", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a builtin that lost its spelling", func(tb assert.TB) {
				frontendtest.AssertLinked(tb, over(&blank{frontendtest.NewScripted()}, fixture()))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.That(t, got[0].Contract).
				HasPrefix("a reference in ", "the rejection names the package").
				HasSuffix(" that resolves to nothing keeps its spelling, as a builtin or an external does",
					"and the lost spelling")
		})

		t.Run("passes a single-package fixture without a cross-package reference", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertLinked(t, setupOver(singleFixture()))
		})
	})

	t.Run("AssertDependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the scripted language in the dependent role", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertDependencies(t, over(frontendtest.NewScriptedDependent(), dependentFixture()))
		})

		t.Run("passes a dependent whose units are unclaimed workspace files", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertDependencies(t, over(&vendored{frontendtest.NewScripted()}, vendoredFixture()))
		})

		t.Run("passes a language that refuses the perturbed byte", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertDependencies(t, over(
				strictDependent{&strict{frontendtest.NewScripted()}}, dependentFixture(),
			))
		})

		t.Run("rejects a frontend outside the dependent role", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a frontend that loads no dependency", func(tb assert.TB) {
				frontendtest.AssertDependencies(tb, setupOver(dependentFixture()))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"the frontend is in the dependent role, so a dependency round runs"},
				"the rejection names the missing role")
		})

		t.Run("rejects rounds that return no unit", func(t *testing.T) {
			t.Parallel()

			bare := dependentFixture()
			bare.Stores = map[string]fs.FS{frontendtest.ScriptedStore: fstest.MapFS{}}
			got := assert.Rejects(t, "a store without the imported package", func(tb assert.TB) {
				frontendtest.AssertDependencies(tb, over(frontendtest.NewScriptedDependent(), bare))
			})
			assert.Equal(t, coretest.Contracts(got), []string{"a dependency round returns a unit"},
				"the rejection names the empty rounds")
		})

		t.Run("rejects a dependency unit declared from bytes it never read through the unit", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a dependency parse that reads around the door", func(tb assert.TB) {
				frontendtest.AssertDependencies(tb, over(
					&unreadDependency{frontendtest.NewScriptedDependent()}, dependentFixture(),
				))
			})
			assert.NotEmpty(t, got, "the check fails")
			assert.HasPrefix(t, got[0].Contract, "a changed byte in a dependency member moves the door fold "+
				"of the unit that read it: ", "the rejection names the fold the byte never moved")
		})
	})

	t.Run("AssertReexports", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the scripted language in the exporter role", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertReexports(t, over(frontendtest.NewScriptedExporter(), exporterFixture()))
		})

		t.Run("rejects a frontend outside the exporter role", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a frontend that follows no re-export", func(tb assert.TB) {
				frontendtest.AssertReexports(tb, setupOver(exporterFixture()))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"the frontend is in the exporter role, so a re-export is followed"},
				"the rejection names the missing role")
		})

		t.Run("rejects a fixture that lists no re-exported declaration", func(t *testing.T) {
			t.Parallel()

			bare := exporterFixture()
			bare.Reexported = nil
			got := assert.Rejects(t, "a fixture with nothing to check", func(tb assert.TB) {
				frontendtest.AssertReexports(tb, over(frontendtest.NewScriptedExporter(), bare))
			})
			assert.Equal(t, coretest.Contracts(got), []string{
				"the fixture lists a declaration a re-export publishes, so the check has something to check",
			}, "the rejection reports the missing part of the fixture")
		})

		t.Run("rejects an exporter whose files publish nothing", func(t *testing.T) {
			t.Parallel()

			fx := exporterFixture()
			got := assert.Rejects(t, "a re-export the frontend never names", func(tb assert.TB) {
				frontendtest.AssertReexports(tb, over(&silentExporter{frontendtest.NewScriptedExporter()}, fx))
			})
			want := make([]string, len(fx.Reexported))
			for i, id := range fx.Reexported {
				want[i] = "a reference through a re-export targets " + id.String()
			}
			assert.Equal(t, coretest.Contracts(got), want,
				"the rejection names each declaration no reference targets")
		})
	})

	t.Run("AssertEmptyLoad", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a tree whose one file declares nothing", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertEmptyLoad(t, setup)
		})

		t.Run("rejects a fixture without an empty tree", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a fixture without an empty tree", func(tb assert.TB) {
				frontendtest.AssertEmptyLoad(tb, setupOver(plainFixture()))
			})
			assert.Equal(t, coretest.Contracts(got), []string{"the fixture has a tree whose files declare nothing"},
				"the rejection reports the missing part of the fixture")
		})

		t.Run("rejects an empty tree that the selection does not claim", func(t *testing.T) {
			t.Parallel()

			unclaimed := fixture()
			unclaimed.Empty = fstest.MapFS{notesFile: {Data: []byte("not source\n")}}
			got := assert.Rejects(t, "an empty tree whose load never calls the frontend", func(tb assert.TB) {
				frontendtest.AssertEmptyLoad(tb, setupOver(unclaimed))
			})
			assert.Equal(t, coretest.Contracts(got), []string{"the selection claims something to check"},
				"the load calls no frontend that claims no file")
		})

		t.Run("rejects a file of the empty tree that declares something", func(t *testing.T) {
			t.Parallel()

			declaring := fixture()
			declaring.Empty = fstest.MapFS{emptyFile: {Data: []byte(emptySource + "type Lost string\n")}}
			got := assert.Rejects(t, "an empty tree whose file declares a type", func(tb assert.TB) {
				frontendtest.AssertEmptyLoad(tb, setupOver(declaring))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{emptyFile + " declares nothing in the graph, as in the empty tree"},
				"the rejection names the file")
		})

		t.Run("rejects an Error over the empty tree", func(t *testing.T) {
			t.Parallel()

			broken := fixture()
			broken.Empty = fstest.MapFS{badFile: {Data: []byte("type Lost string\n")}}
			got := assert.Rejects(t, "an empty tree whose file reports an Error", func(tb assert.TB) {
				frontendtest.AssertEmptyLoad(tb, setupOver(broken))
			})
			assert.Equal(t, coretest.Contracts(got), []string{
				"the load of a tree whose files declare nothing reports no Error: " + badFile +
					` opens with "type", not a package line`,
				badFile + " enters the graph as a file",
			}, "the rejection names the Error and the file that the load leaves out")
		})
	})
}

// vendored is the scripted language in the dependent role that reads
// each need from the skip directory of the workspace, which the claim
// leaves out. It has the shape of a Go vendor tree, whose dependency
// members are workspace files.
type vendored struct {
	*frontendtest.Scripted
}

// Dependencies returns one unit per need the skip directory lists.
func (*vendored) Dependencies(
	_ context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	var out [][]plugin.SourceRef
	for _, need := range round.Needs {
		dir := path.Join(vendorRoot, need.Path)
		entries, err := r.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		unit := make([]plugin.SourceRef, len(entries))
		for i, e := range entries {
			unit[i] = plugin.SourceRef{Path: path.Join(dir, e.Name())}
		}
		out = append(out, unit)
	}
	return out, nil
}

// unreadDependency is the scripted language in the dependent role that
// declares the files of each dependency unit from their paths alone. It
// has the shape of a frontend that reads a dependency around the unit,
// which the dependency check must expose.
type unreadDependency struct {
	*frontendtest.ScriptedDependent
}

// Parse lowers a workspace unit, and declares a dependency unit's
// files without reading them.
func (f *unreadDependency) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if _, _, qualified := plugin.CutStorePath(u.Files()[0].Path); !qualified {
		return f.ScriptedDependent.Parse(ctx, u)
	}
	for _, ref := range u.Files() {
		_, inner, _ := plugin.CutStorePath(ref.Path)
		pkg := u.Graph().Package(path.Dir(inner))
		pkg.Files = append(pkg.Files, &node.File{Path: ref.Path})
	}
	return nil
}

// strictDependent is the strict language in the dependent role, so a
// perturbed dependency member fails the load.
type strictDependent struct {
	*strict
}

// Dependencies returns the scripted dependent's units.
func (f strictDependent) Dependencies(
	ctx context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	return (&frontendtest.ScriptedDependent{Scripted: f.Scripted}).Dependencies(ctx, round, r)
}

// silentExporter is the scripted language in the exporter role whose
// files publish nothing, which the re-export check must expose.
type silentExporter struct {
	*frontendtest.ScriptedExporter
}

// Exports returns no candidate for any name.
func (*silentExporter) Exports(plugin.ImportScope, string) plugin.Candidates { return nil }

// These constants are the drifts that the restless frontend can record.
const (
	recordStamp     = "stamp"
	recordDirective = "directive"
	recordFinding   = "finding"
)

// restless records one value that differs on every load, which the
// determinism check must expose. The case chooses the value: the stamp
// value, a directive argument or the text of a finding.
type restless struct {
	*frontendtest.Scripted
	record string
	loads  atomic.Int64
}

// Parse lowers the unit, then records the load's own count on its
// first file.
func (r *restless) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := r.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	pkgs := u.Graph().Packages()
	if len(pkgs) == 0 {
		return nil
	}
	n := strconv.FormatInt(r.loads.Add(1), 10)
	file := pkgs[0].Files[0]
	at := position.Pos{File: file.Path, Line: 1}
	switch r.record {
	case recordStamp:
		u.Graph().Stamp(file, meta.RawStamp{Key: frontendtest.ScriptedTestKey, Value: n, Pos: at})
	case recordDirective:
		raw, err := directive.Parse(carrierLine + n)
		if err != nil {
			return err
		}
		raw.Pos = at
		u.Graph().Attach(file, raw)
	default:
		u.Warnf(frontendtest.ScriptedBadFile, at, "load %s", n)
	}
	return nil
}

// drifter partitions like the scripted language for two loads and
// into one unit from the third on, so a key comparison meets a unit
// the later load lacks.
type drifter struct {
	*frontendtest.Scripted
	loads atomic.Int64
}

// Partition drifts after the second call.
func (d *drifter) Partition(
	ctx context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	if d.loads.Add(1) <= 2 {
		return d.Scripted.Partition(ctx, files, r)
	}
	return [][]plugin.SourceRef{files}, nil
}

// unread declares the file of each member without a read through the
// unit. It has the shape of a frontend that reads its bytes in another
// way, which the door check must expose.
type unread struct {
	*frontendtest.Scripted
}

// Parse declares one file node per member, from its path alone.
func (*unread) Parse(_ context.Context, u *plugin.SourceUnit) error {
	for _, ref := range u.Files() {
		pkg := u.Graph().Package(path.Dir(ref.Path))
		pkg.Files = append(pkg.Files, &node.File{Path: ref.Path})
	}
	return nil
}

// insular resolves a bare spelling in the file's own package and
// no qualified one, which the cross-package check must expose.
type insular struct {
	*frontendtest.Scripted
}

// Resolve returns candidates for a spelling without a qualifier
// alone.
func (i *insular) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	if strings.Contains(spelling, ".") {
		return nil
	}
	return i.Scripted.Resolve(scope, spelling)
}

// blank erases the spelling of every builtin reference, which the
// linked check must expose.
type blank struct {
	*frontendtest.Scripted
}

// Resolve returns nothing for the empty spelling it erased, and the
// scripted candidates otherwise.
func (b *blank) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	if spelling == "" {
		return nil
	}
	return b.Scripted.Resolve(scope, spelling)
}

// Parse lowers the unit, then clears the spelling of every builtin
// reference.
func (b *blank) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := b.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	for _, pkg := range u.Graph().Packages() {
		node.Walk(pkg, func(s symbol.Symbol) bool {
			if ref, is := s.(*node.TypeRef); is && !strings.Contains(ref.Spelling, ".") &&
				ref.Spelling != "" && (ref.Spelling[0] < 'A' || ref.Spelling[0] > 'Z') {

				ref.Spelling = ""
			}
			return true
		})
	}
	return nil
}

// over returns a setup of the frontend f over the fixture fx, for a case
// that runs a check against an implementation that the check must
// reject.
func over(f plugin.Frontend, fx *frontendtest.Fixture) frontendtest.Setup {
	return func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
		return f, fx
	}
}

// mute resolves nothing, which a two-package fixture must expose.
type mute struct {
	*frontendtest.Scripted
}

// Resolve returns no candidate for any spelling.
func (*mute) Resolve(plugin.ImportScope, string) plugin.Candidates { return nil }

// swallower parses every unit's first member alone and reports
// nothing about the rest, which the no-drop check must expose.
type swallower struct {
	*frontendtest.Scripted
}

// Parse hands only the first member to the real lowering, leaving
// the rest undeclared and unreported.
func (s *swallower) Parse(_ context.Context, u *plugin.SourceUnit) error {
	return s.ParseFile(u, u.Files()[0].Path)
}

// unpositioned reports a finding without a position, which the
// positioning check must expose.
type unpositioned struct {
	*frontendtest.Scripted
}

// Parse reports at the zero position before lowering the unit.
func (f *unpositioned) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	u.Errorf(frontendtest.ScriptedBadFile, position.Pos{}, "a finding with nowhere to point")
	return f.Scripted.Parse(ctx, u)
}

// anonymous declares no name, so its findings have no origin,
// which the positioning check must expose.
type anonymous struct {
	*frontendtest.Scripted
}

// Name declares nothing.
func (*anonymous) Name() plugin.ID { return "" }

// undocumented attaches its carriers and leaves a carrier line in the
// documentation of the declaration, which the attachment check must
// expose. The line is the bare payload where the case sets no line.
type undocumented struct {
	*frontendtest.Scripted
	line string
}

// Parse lowers the unit and then writes the carrier line back into
// every declaration's documentation.
func (f *undocumented) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := f.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	line := f.line
	if line == "" {
		line = carrierLine
	}
	for _, pkg := range u.Graph().Packages() {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				if s, is := decl.(*node.Struct); is {
					s.Doc = []string{line}
				}
			}
		}
	}
	return nil
}

// dangling attaches a directive to a declaration it never puts in a
// file, under an identity of its own, which the attachment check
// must expose as a subject the graph does not contain.
type dangling struct {
	*frontendtest.Scripted
}

// Parse lowers the unit and attaches to a declaration outside it.
func (f *dangling) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := f.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	ghost := &node.Struct{
		ID: symbol.Identity{
			Lang: frontendtest.ScriptedLang, Package: ghostPath,
			Name: ghostName, Kind: symbol.KindStruct,
		},
		Name: ghostName,
	}
	raw, err := directive.Parse(carrierLine)
	if err != nil {
		return err
	}
	raw.Pos = position.Pos{File: u.Files()[0].Path, Line: 1}
	u.Graph().Attach(ghost, raw)
	return nil
}

// errBlankTail is the error of the strict language for a member that
// ends in a blank line. The key check then meets a language whose
// grammar excludes the perturbed byte.
var errBlankTail = errors.New("frontendtest_test: a source file ends at one newline")

// strict refuses a member that ends in a blank line. It has the shape of
// a language whose grammar excludes the perturbation of the fingerprint
// check.
type strict struct {
	*frontendtest.Scripted
}

// Parse refuses a perturbed member and lowers the rest.
func (f *strict) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	for _, ref := range u.Files() {
		b, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(string(b), "\n\n") {
			return errBlankTail
		}
	}
	return f.Scripted.Parse(ctx, u)
}

// optionless hides the scripted frontend's options, which is the
// shape of a frontend declaring no configuration at all.
type optionless struct {
	f *frontendtest.Scripted
}

// Name returns the wrapped frontend's name.
func (o optionless) Name() plugin.ID { return o.f.Name() }

// Lang returns the wrapped frontend's language.
func (o optionless) Lang() symbol.Lang { return o.f.Lang() }

// Syntax returns the wrapped frontend's comment syntax.
func (o optionless) Syntax() plugin.CommentSyntax { return o.f.Syntax() }

// Overloads reports whether the wrapped frontend's language
// overloads.
func (o optionless) Overloads() bool { return o.f.Overloads() }

// Version returns the wrapped frontend's version.
func (o optionless) Version() string { return o.f.Version() }

// Selection returns the wrapped frontend's claim.
func (o optionless) Selection() []string { return o.f.Selection() }

// Parse lowers the unit through the wrapped frontend.
func (o optionless) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	return o.f.Parse(ctx, u)
}

// Partition groups the files through the wrapped frontend.
func (o optionless) Partition(
	ctx context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return o.f.Partition(ctx, files, r)
}

// Resolve probes a spelling through the wrapped frontend.
func (o optionless) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	return o.f.Resolve(scope, spelling)
}

// carved carves the ownership check's copies out of the claim, so
// the stamped file is neither refused nor loaded.
type carved struct {
	plugin.Frontend
}

// Selection returns the inner claim with the copies negated.
func (c *carved) Selection() []string {
	return append(c.Frontend.Selection(), "!**/*_owned.zz")
}
