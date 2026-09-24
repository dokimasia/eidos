// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest_test

import (
	"context"
	"errors"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The identity the dangling frontend attaches to, which no load
// ever declares.
const (
	ghostPath = "svc/ghost"
	ghostName = "Ghost"
)

// The checks exist to catch broken frontends, so the broken ones
// are simulated and each check's own failure is asserted.
func TestChecks(t *testing.T) {
	t.Parallel()

	t.Run("AssertDeterministicParse", func(t *testing.T) {
		t.Parallel()

		for _, record := range []string{recordStamp, recordDirective, recordFinding} {
			t.Run("rejects a "+record+" that changes between loads", func(t *testing.T) {
				t.Parallel()

				msg := assert.Rejects(t, "a frontend recording something new per load", func(tb assert.TB) {
					frontendtest.AssertDeterministicParse(tb, over(&restless{
						Scripted: frontendtest.NewScripted(), record: record,
					}, fixture()))
				})
				assert.Contains(t, msg, "the same", "the rejection names what drifted")
			})
		}
	})

	t.Run("AssertPositionedDiagnostics", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a finding with no position", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a finding no reader can open", func(tb assert.TB) {
				frontendtest.AssertPositionedDiagnostics(tb, over(&unpositioned{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.Contains(t, msg, "no position", "the rejection names what is missing")
		})

		t.Run("rejects a finding with no origin", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a finding nobody signed", func(tb assert.TB) {
				frontendtest.AssertPositionedDiagnostics(tb, over(&anonymous{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.Contains(t, msg, "no origin", "the rejection names what is missing")
		})
	})

	t.Run("AssertClassified", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a silently dropped file", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a parse that swallows a file", func(tb assert.TB) {
				frontendtest.AssertClassified(tb, over(&swallower{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.Contains(t, msg, "silently dropped", "the rejection names the class")
		})

		t.Run("rejects a fixture that stamps and declares no keys", func(t *testing.T) {
			t.Parallel()

			keyless := fixture()
			keyless.Keys = nil
			msg := assert.Rejects(t, "a stamp no registry can apply", func(tb assert.TB) {
				frontendtest.AssertClassified(tb, setupOver(keyless))
			})
			assert.Contains(t, msg, "no keys", "the rejection names what the fixture owes")
		})

		t.Run("rejects keys declared over a load that stamps nothing", func(t *testing.T) {
			t.Parallel()

			unstamped := plainFixture()
			unstamped.Keys = frontendtest.ScriptedKeys
			msg := assert.Rejects(t, "a classifier that never stamps", func(tb assert.TB) {
				frontendtest.AssertClassified(tb, setupOver(unstamped))
			})
			assert.Contains(t, msg, "stamps nothing", "the rejection names the dead classifier")
		})
	})

	t.Run("AssertOwnedExcluded", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a claim that never reaches the stamped copy", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a selection the check cannot place a copy under", func(tb assert.TB) {
				frontendtest.AssertOwnedExcluded(tb, over(&carved{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.Contains(t, msg, "claim reaches it", "the rejection names the placement")
		})
	})

	t.Run("AssertFingerprinted", func(t *testing.T) {
		t.Parallel()

		t.Run("folds a key for a frontend that declares no options", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertFingerprinted(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return optionless{frontendtest.NewScripted()}, plainFixture()
			})
		})

		t.Run("holds the fold where a language refuses the perturbed byte", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertFingerprinted(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return &strict{frontendtest.NewScripted()}, plainFixture()
			})
		})

		t.Run("holds the fold where every unit loads shallow", func(t *testing.T) {
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
			msg := assert.Rejects(t, "a tree the perturbation cannot copy", func(tb assert.TB) {
				frontendtest.AssertFingerprinted(tb, setupOver(unreadable))
			})
			assert.Contains(t, msg, "copies", "the rejection names the step that could not run")
		})

		t.Run("rejects a unit missing from a load it is compared with", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a partition that merges its units after two loads", func(tb assert.TB) {
				frontendtest.AssertFingerprinted(tb, over(&drifter{Scripted: frontendtest.NewScripted()},
					plainFixture()))
			})
			assert.Contains(t, msg, "missing", "a key compared with nothing proves nothing")
		})
	})

	t.Run("AssertJailedReads", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a frontend declaring what it never read through the unit", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a parse whose bytes come from outside the door", func(tb assert.TB) {
				frontendtest.AssertJailedReads(tb, over(&unread{frontendtest.NewScripted()}, plainFixture()))
			})
			assert.Contains(t, msg, "through the unit", "the rejection names the door")
		})

		t.Run("accepts a language that refuses the perturbed byte", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertJailedReads(t, over(&strict{frontendtest.NewScripted()}, plainFixture()))
		})

		t.Run("rejects a fixture tree it cannot walk", func(t *testing.T) {
			t.Parallel()

			unwalkable := plainFixture()
			unwalkable.Sources = failingFS{tree: fstest.MapFS{apiFile: {Data: []byte(apiSource)}}, fail: "."}
			msg := assert.Rejects(t, "a tree the selection cannot be read over", func(tb assert.TB) {
				frontendtest.AssertJailedReads(tb, setupOver(unwalkable))
			})
			assert.Contains(t, msg, "walks", "the rejection names the step that could not run")
		})

		t.Run("holds a fixture whose selection claims one file", func(t *testing.T) {
			t.Parallel()

			lone := plainFixture()
			lone.Sources = fstest.MapFS{apiFile: {Data: []byte(apiSource)}}
			frontendtest.AssertJailedReads(t, setupOver(lone))
		})
	})

	t.Run("AssertSignatureDepth", func(t *testing.T) {
		t.Parallel()

		t.Run("holds a fixture whose root covers every unit", func(t *testing.T) {
			t.Parallel()

			deep := plainFixture()
			deep.Signatures = []string{serviceRoot}
			frontendtest.AssertSignatureDepth(t, setupOver(deep))
		})

		t.Run("rejects a stated root no unit sits under", func(t *testing.T) {
			t.Parallel()

			astray := plainFixture()
			astray.Signatures = []string{absentRoot}
			msg := assert.Rejects(t, "a signature root the load never applies", func(tb assert.TB) {
				frontendtest.AssertSignatureDepth(tb, setupOver(astray))
			})
			assert.Contains(t, msg, "at least one unit shallow",
				"the rejection names what the stated root owes")
		})
	})

	t.Run("AssertAttachedDirectives", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects an instance the schema refuses", func(t *testing.T) {
			t.Parallel()

			mistyped := fixture()
			mistyped.Sources = fstest.MapFS{
				storeFile: {Data: []byte(
					"package svc/store\ntype Row string\n+gen:table wrong=x\n",
				)},
			}
			mistyped.Signatures = nil
			msg := assert.Rejects(t, "a carrier writing a key no schema declares", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, setupOver(mistyped))
			})
			assert.Contains(t, msg, "fails validation", "the rejection names the class")
		})

		t.Run("rejects a carrier line left in documentation", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a frontend that attaches without stripping", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, over(&undocumented{
					Scripted: frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.Contains(t, msg, "carrier line", "the rejection names the class")
		})

		t.Run("rejects a carrier line left with its mark", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a documentation line the strip never saw", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, over(&undocumented{
					Scripted: frontendtest.NewScripted(), line: carrierStatement[:len(carrierStatement)-1],
				}, fixture()))
			})
			assert.Contains(t, msg, "carrier line", "the mark does not hide the leak")
		})

		t.Run("rejects directives on a subject the graph does not hold", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "an attachment to a declaration nothing declares", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, over(&dangling{
					frontendtest.NewScripted(),
				}, fixture()))
			})
			assert.Contains(t, msg, "does not hold", "the rejection names the class")
		})

		t.Run("holds a fixture that declares schemas and no keys", func(t *testing.T) {
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
			msg := assert.Rejects(t, "schemas no carrier in the tree writes", func(tb assert.TB) {
				frontendtest.AssertAttachedDirectives(tb, setupOver(barren))
			})
			assert.Contains(t, msg, "must attach something",
				"the rejection names what the declared schemas owe")
		})
	})

	t.Run("AssertLinked", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a Resolve that never answers", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a mute resolver over a two-package fixture", func(tb assert.TB) {
				frontendtest.AssertLinked(tb, over(&mute{frontendtest.NewScripted()}, fixture()))
			})
			assert.Contains(t, msg, "returns no candidate", "the rejection names what never happened")
		})

		t.Run("rejects a mute Resolve over a single-package fixture", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a mute resolver with no second package to hide behind",
				func(tb assert.TB) {
					frontendtest.AssertLinked(tb, over(&mute{frontendtest.NewScripted()}, singleFixture()))
				})
			assert.Contains(t, msg, "returns no candidate",
				"in-graph spellings resolve in one package too")
		})

		t.Run("rejects a Resolve that returns nothing across exactly two packages", func(t *testing.T) {
			t.Parallel()

			local := plainFixture()
			local.Sources = fstest.MapFS{
				apiFile:   {Data: []byte(apiSource)},
				storeFile: {Data: []byte(crossSource + localStatement)},
			}
			msg := assert.Rejects(t, "a resolver returning candidates in its own package alone",
				func(tb assert.TB) {
					frontendtest.AssertLinked(tb, over(&insular{frontendtest.NewScripted()}, local))
				})
			assert.Contains(t, msg, "resolving nothing across packages",
				"two packages are already across packages")
		})

		t.Run("rejects an unresolved reference that spells nothing", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a builtin that lost its spelling", func(tb assert.TB) {
				frontendtest.AssertLinked(tb, over(&blank{frontendtest.NewScripted()}, fixture()))
			})
			assert.Contains(t, msg, "spells nothing", "the rejection names the lost spelling")
		})

		t.Run("asks nothing across packages of a single-package fixture", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertLinked(t, setupOver(singleFixture()))
		})
	})
}

// The drifts the restless frontend can record.
const (
	recordStamp     = "stamp"
	recordDirective = "directive"
	recordFinding   = "finding"
)

// restless records one thing that differs on every load, the stamp
// value, a directive argument or a finding's wording, as the case
// chooses, which the determinism check must expose.
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

// unread declares each member's file without reading it through
// the unit, the shape of a frontend reading its bytes some other
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
func (i *insular) Resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
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
func (b *blank) Resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
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

// over builds the stated frontend over the stated fixture, for a
// case driving a check against an implementation it must reject.
func over(f plugin.Frontend, fx *frontendtest.Fixture) frontendtest.Setup {
	return func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
		return f, fx
	}
}

// mute resolves nothing, which a two-package fixture must expose.
type mute struct {
	*frontendtest.Scripted
}

// Resolve answers no candidate for any spelling.
func (*mute) Resolve(plugin.ImportScope, string) []symbol.Identity { return nil }

// swallower parses every unit's first member alone and says
// nothing about the rest, which the no-drop check must expose.
type swallower struct {
	*frontendtest.Scripted
}

// Parse hands only the first member to the real lowering, leaving
// the rest undeclared and unreported.
func (s *swallower) Parse(_ context.Context, u *plugin.SourceUnit) error {
	return s.ParseFile(u, u.Files()[0].Path)
}

// unpositioned reports a finding carrying no address, which the
// positioning check must expose.
type unpositioned struct {
	*frontendtest.Scripted
}

// Parse reports at the zero position before lowering the unit.
func (f *unpositioned) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	u.Errorf(frontendtest.ScriptedBadFile, position.Pos{}, "a finding with nowhere to point")
	return f.Scripted.Parse(ctx, u)
}

// anonymous declares no name, so its findings carry no origin,
// which the positioning check must expose.
type anonymous struct {
	*frontendtest.Scripted
}

// Name declares nothing.
func (*anonymous) Name() plugin.ID { return "" }

// undocumented attaches its carriers and leaves a carrier line in
// the declaration's documentation, the bare payload unless the case
// states the line, which the attachment check must expose.
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
// must expose as a subject the graph does not hold.
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

// errBlankTail is what the strict language refuses, so the key
// check meets a language the perturbed byte puts outside the
// grammar.
var errBlankTail = errors.New("frontendtest_test: a source file ends at one newline")

// strict refuses a member ending in a blank line, which is the
// shape of a language the fingerprint check's perturbation puts
// outside the grammar.
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

func (o optionless) Name() plugin.ID              { return o.f.Name() }
func (o optionless) Lang() symbol.Lang            { return o.f.Lang() }
func (o optionless) Syntax() plugin.CommentSyntax { return o.f.Syntax() }
func (o optionless) Version() string              { return o.f.Version() }
func (o optionless) Selection() []string          { return o.f.Selection() }
func (o optionless) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	return o.f.Parse(ctx, u)
}

func (o optionless) Partition(
	ctx context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return o.f.Partition(ctx, files, r)
}

func (o optionless) Resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
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
