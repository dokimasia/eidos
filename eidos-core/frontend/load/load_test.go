// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The standard tree's paths, so a case names the file it asserts
// about rather than repeating a literal.
const (
	modFile   = "mod.zz"
	apiPath   = "svc/api"
	apiFile   = "svc/api/user.zz"
	storePath = "svc/store"
	storeFile = "svc/store/row.zz"
	depPath   = "svc/dep"
	depFile   = "svc/dep/dep.zz"
)

// The standard tree's declarations, and the one directive its
// carrier writes.
const (
	rowName    = "Row"
	userName   = "User"
	rowMaxName = "rowmax"
	hiddenName = "hidden"
	tableName  = directive.Name("gen:table")
)

// The paths and names the cases that build their own tree use.
const (
	sharedPath = "shared"
	twinName   = "Twin"
	loosePath  = "loose"
	looseName  = "Undeclared"
	oneFile    = "a/one.zz"
	twoFile    = "b/two.zz"
)

// stdTree is the happy-path fixture: two packages, one cross-package
// reference, a directive, a constant, and a dependency package
// loaded signature-only.
func stdTree() fstest.MapFS {
	return fstest.MapFS{
		modFile: {Data: []byte("mod v1\n")},
		apiFile: {Data: []byte(
			"package svc/api\ntype User string\n",
		)},
		storeFile: {Data: []byte(
			"package svc/store\nimport api svc/api\ntype Row api.User int\n+gen:table name=users\nconst rowmax\n",
		)},
		depFile: {Data: []byte(
			"package svc/dep\ntype Dep string\nconst hidden\n",
		)},
	}
}

// config is the standard load configuration over a tree, mutated
// per case.
func config(tree fs.FS, mutate ...func(*load.Config)) (load.Config, *diag.Sink) {
	sink := diag.NewSink()
	cfg := load.Config{
		FS:         tree,
		Frontends:  []plugin.Frontend{frontendtest.NewScripted()},
		Sink:       sink,
		PluginSet:  []byte("set-1"),
		Signatures: []string{depPath},
	}
	for _, m := range mutate {
		m(&cfg)
	}
	return cfg, sink
}

// with composes exactly these frontends instead of the standard one.
func with(fronts ...plugin.Frontend) func(*load.Config) {
	return func(cfg *load.Config) { cfg.Frontends = fronts }
}

// loadTree drives one load over a tree with the fake frontend and
// the standard configuration, mutated per test.
func loadTree(
	tb assert.TB, tree fs.FS, mutate ...func(*load.Config),
) (*store.Graph, *load.Report, *diag.Sink) {
	tb.Helper()

	cfg, sink := config(tree, mutate...)
	g, report, err := load.Load(context.Background(), cfg)
	assert.NoError(tb, err, "the load finishes")
	return g, report, sink
}

// The brands the ownership cases stamp under.
const (
	ownBrand     output.Brand = "own"
	foreignBrand output.Brand = "foreign"
)

// stamped returns source framed as one brand's output, so a case
// can put a generated file into a tree.
func stamped(tb assert.TB, brand output.Brand, source string) []byte {
	tb.Helper()

	contract, err := output.NewContract(brand, frontendtest.NewScripted().Syntax())
	assert.NoError(tb, err, "the fixture language carries the frame")
	b, err := contract.Stamp(plugin.RenderedFile{
		Name: "gen.zz", Plugins: []plugin.ID{"gen"}, Body: []byte(source),
	})
	assert.NoError(tb, err, "the source stamps")
	return b
}

// refuse drives one load the case states must fail and returns the
// driver's error, so the case asserts on what it says.
func refuse(tb assert.TB, tree fs.FS, mutate ...func(*load.Config)) error {
	tb.Helper()

	cfg, _ := config(tree, mutate...)
	_, _, err := load.Load(context.Background(), cfg)
	assert.HasError(tb, err, "the load refuses rather than sealing a graph")
	return err
}

// findingOf returns the first finding the sink holds under a code,
// for a case asserting on a finding's address rather than on its
// presence.
func findingOf(sink *diag.Sink, c diag.Code) (diag.Diag, bool) {
	for d := range sink.All() {
		if d.Code == c {
			return d, true
		}
	}
	return diag.Diag{}, false
}

// rowID is the standard tree's one struct in svc/store.
func rowID() symbol.Identity {
	return symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: storePath, Name: rowName, Kind: symbol.KindStruct,
	}
}

// twinID is the identity two declarations of the shared tree spell.
func twinID() symbol.Identity {
	return symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: sharedPath, Name: twinName, Kind: symbol.KindStruct,
	}
}

// The driver is the read side's one pipeline, so its phases are
// pinned end to end over the scripted language.
func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("seals a graph the store serves", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, stdTree())
			assert.True(t, g.Frozen(), "the load ends at the seal")

			row, held := g.Lookup(rowID())
			assert.True(t, held, "a loaded declaration is indexed")
			assert.Equal(t, row.(*node.Struct).Name, rowName, "as its own kind")

			pkg, held := g.PackageOf(rowID())
			assert.True(t, held, "under its package")
			assert.Equal(t, pkg.ID, symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath, Kind: symbol.KindPackage,
			}, "whose identity is canonical")

			_, held = g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath, Name: storeFile, Kind: symbol.KindFile,
			})
			assert.True(t, held, "the file is a declaration of its package")
		})

		t.Run("reports every unit in splice order", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			first := make([]string, 0, len(report.Units))
			for _, u := range report.Units {
				assert.Equal(t, u.Frontend, frontendtest.ScriptedID, "each unit names its frontend")
				assert.NotEmpty(t, u.Key, "and carries the key the fold produced")
				first = append(first, u.Files[0])
			}
			assert.Equal(t, first, []string{apiFile, depFile, storeFile},
				"units sort by their first member's path, which claims make unique")
		})

		t.Run("keeps the first of two declarations spelling one identity", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				oneFile: {Data: []byte("package shared\ntype Twin left\n")},
				twoFile: {Data: []byte("package shared\ntype Twin right\n")},
			}
			g, _, sink := loadTree(t, tree)
			coretest.AssertReports(t, sink, load.DuplicateDeclaration)

			twin, held := g.Lookup(twinID())
			assert.True(t, held, "one Twin stands")
			assert.Equal(t, twin.(*node.Struct).Fields[0].Type.Spelling, "left",
				"the first in unit order")
		})

		t.Run("replays unit findings under the frontend's origin", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"bad/oops.zz": {Data: []byte("type Lost string\n")},
			}
			_, _, sink := loadTree(t, tree)
			found, held := findingOf(sink, frontendtest.ScriptedBadFile)
			assert.True(t, held, "a unit's finding reaches the run sink")
			assert.Equal(t, found.Origin, frontendtest.ScriptedID, "origin-bound")
			assert.Equal(t, found.Pos.File, "bad/oops.zz", "positioned")
			coretest.AssertPositioned(t, sink)
		})

		t.Run("refuses a config with no tree", func(t *testing.T) {
			t.Parallel()

			_, _, err := load.Load(context.Background(), load.Config{Sink: diag.NewSink()})
			assert.HasError(t, err, "there is nothing to read")
			assert.Contains(t, err.Error(), "no tree to read", "saying so")
		})

		t.Run("refuses a config with no sink", func(t *testing.T) {
			t.Parallel()

			_, _, err := load.Load(context.Background(), load.Config{FS: stdTree()})
			assert.HasError(t, err, "there is nowhere to report")
			assert.Contains(t, err.Error(), "no sink to report into", "saying so")
		})

		t.Run("refuses a versionless frontend", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(versionless{frontendtest.NewScripted()}))
			assert.Contains(t, err.Error(), "version",
				"every unit key folds the declared version")
		})

		t.Run("refuses options hiding a field from the key", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(hiddenOptions{frontendtest.NewScripted()}))
			assert.Contains(t, err.Error(), "secret",
				"naming the knob the encoding cannot see")
		})
	})

	t.Run("disown", func(t *testing.T) {
		t.Parallel()

		ownedFile := "svc/store/row_gen.zz"
		generated := "package svc/store\ntype Generated string\n"

		t.Run("refuses the workspace's own outputs before anything partitions", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[ownedFile] = &fstest.MapFile{Data: stamped(t, ownBrand, generated)}
			g, report, sink := loadTree(t, tree, func(cfg *load.Config) { cfg.Brand = ownBrand })
			coretest.AssertCodes(t, sink)
			assert.Equal(t, report.Excluded, []string{ownedFile}, "the report lists the refusal")
			for _, u := range report.Units {
				assert.False(t, slices.Contains(u.Files, ownedFile),
					"and no unit holds the file")
			}
			_, held := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath,
				Name: "Generated", Kind: symbol.KindStruct,
			})
			assert.False(t, held, "so its declarations never enter the graph")
		})

		t.Run("loads another brand's output as ordinary input", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[ownedFile] = &fstest.MapFile{Data: stamped(t, foreignBrand, generated)}
			g, report, _ := loadTree(t, tree, func(cfg *load.Config) { cfg.Brand = ownBrand })
			assert.Empty(t, report.Excluded, "a foreign frame proves nothing to this load")
			_, held := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath,
				Name: "Generated", Kind: symbol.KindStruct,
			})
			assert.True(t, held, "and the file's declarations load")
		})

		t.Run("excludes nothing under the zero brand", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[ownedFile] = &fstest.MapFile{Data: stamped(t, ownBrand, generated)}
			_, report, _ := loadTree(t, tree)
			assert.Empty(t, report.Excluded, "a composition declaring no output owns nothing")
		})

		t.Run("returns a read's own error", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, failingFS{tree: stdTree(), fail: storeFile}, func(cfg *load.Config) {
				cfg.Brand = ownBrand
			})
			assert.Contains(t, err.Error(), storeFile, "naming the file the proof could not read")
		})
	})

	t.Run("treeFiles", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a tree it cannot walk", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, failingFS{tree: stdTree(), fail: "."})
			assert.Contains(t, err.Error(), "walk the tree", "naming the phase")
			assert.ErrorIs(t, err, fs.ErrPermission, "and carrying the filesystem's own cause")
		})
	})

	t.Run("claim", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses two claims on one file", func(t *testing.T) {
			t.Parallel()

			rival := frontendtest.NewScripted()
			rival.ID = "rival"
			err := refuse(t, stdTree(), with(frontendtest.NewScripted(), rival))
			assert.Contains(t, err.Error(), string(frontendtest.ScriptedID), "naming the first claimant")
			assert.Contains(t, err.Error(), "rival", "and the second")
		})
	})

	t.Run("partitionAll", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the frontend's own partition error", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(&refusing{frontendtest.NewScripted()}))
			assert.ErrorIs(t, err, errPartitionRefused, "the frontend's cause carries up")
			assert.Contains(t, err.Error(), "partition", "naming the phase")
		})

		t.Run("never asks a frontend claiming nothing to partition", func(t *testing.T) {
			t.Parallel()

			quiet := &idle{frontendtest.NewScripted()}
			quiet.ID = "idle"
			quiet.Sel = []string{"**/*.nothing"}
			_, report, _ := loadTree(t, stdTree(), with(frontendtest.NewScripted(), quiet))
			for _, u := range report.Units {
				assert.Equal(t, u.Frontend, frontendtest.ScriptedID,
					"the one claiming frontend produced every unit")
			}
		})
	})

	t.Run("checkPartition", func(t *testing.T) {
		t.Parallel()

		one := func() fstest.MapFS {
			return fstest.MapFS{
				modFile: {Data: []byte("mod v1\n")},
				oneFile: {Data: []byte("package shared\ntype Twin left\n")},
			}
		}

		t.Run("refuses a unit with no members", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, one(), with(partitions(func([]plugin.SourceRef) [][]plugin.SourceRef {
				return [][]plugin.SourceRef{{}}
			})))
			assert.Contains(t, err.Error(), "no members", "an empty unit has nothing to parse")
		})

		t.Run("refuses a member the selection never claimed", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, one(), with(partitions(func([]plugin.SourceRef) [][]plugin.SourceRef {
				return [][]plugin.SourceRef{{{Path: modFile}}}
			})))
			assert.Contains(t, err.Error(), modFile, "naming the file outside the claim")
			assert.Contains(t, err.Error(), "never claimed", "and why it may not be a member")
		})

		t.Run("refuses a claimed file in no unit", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, one(), with(partitions(func([]plugin.SourceRef) [][]plugin.SourceRef {
				return nil
			})))
			assert.Contains(t, err.Error(), oneFile, "naming the dropped file")
			assert.Contains(t, err.Error(), "in no unit", "which is a silent drop")
		})

		t.Run("refuses a file in two units", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, one(), with(partitions(func(files []plugin.SourceRef) [][]plugin.SourceRef {
				return [][]plugin.SourceRef{{files[0]}, {files[0]}}
			})))
			assert.Contains(t, err.Error(), oneFile, "naming the shared file")
			assert.Contains(t, err.Error(), "2 units", "and how many units hold it")
		})
	})

	t.Run("encodeOptions", func(t *testing.T) {
		t.Parallel()

		t.Run("keys a frontend declaring no options apart from one that does", func(t *testing.T) {
			t.Parallel()

			_, declared, _ := loadTree(t, stdTree())
			_, bare, _ := loadTree(t, stdTree(), with(optionless{frontendtest.NewScripted()}))

			configured, none := keysOf(declared), keysOf(bare)
			assert.Equal(t, len(none), len(configured), "both compositions load the same units")
			for file, key := range configured {
				assert.NotEqual(t, key, none[file],
					"a declared configuration folds where none folds nothing")
			}
		})

		t.Run("refuses options that cannot encode", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(&unencodable{frontendtest.NewScripted()}))
			assert.Contains(t, err.Error(), "encode", "naming the phase")
			assert.Contains(t, err.Error(), string(frontendtest.ScriptedID), "and the frontend")
		})
	})

	t.Run("depthOf", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a unit under a stated root signature-only", func(t *testing.T) {
			t.Parallel()

			g, report, _ := loadTree(t, stdTree())
			_, held := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath,
				Name: rowMaxName, Kind: symbol.KindConstant,
			})
			assert.True(t, held, "a full unit keeps its constants")

			_, held = g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: depPath,
				Name: hiddenName, Kind: symbol.KindConstant,
			})
			assert.False(t, held, "a signature unit drops what its parse skipped")

			for _, u := range report.Units {
				want := plugin.DepthFull
				if u.Files[0] == depFile {
					want = plugin.DepthSignatures
				}
				assert.Equal(t, u.Depth, want, "the report records each unit's depth")
			}
		})

		t.Run("claims nothing for an empty root", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree(), func(cfg *load.Config) {
				cfg.Signatures = []string{""}
			})
			for _, u := range report.Units {
				assert.Equal(t, u.Depth, plugin.DepthFull,
					"an empty root names no directory, so every unit loads full")
			}
		})
	})

	t.Run("parseAll", func(t *testing.T) {
		t.Parallel()

		t.Run("stops the load at a frontend's parse error", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(&unparsable{frontendtest.NewScripted()}))
			assert.ErrorIs(t, err, errParseRefused, "the frontend's cause carries up")
			assert.Contains(t, err.Error(), "parse", "naming the phase")
		})

		t.Run("stops the load at a cancelled context", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			cfg, _ := config(stdTree())
			g, report, err := load.Load(ctx, cfg)
			assert.ErrorIs(t, err, context.Canceled,
				"a skipped unit has no source unit to key, so the load cannot finish")
			assert.Nil(t, g, "and nothing half-loaded is handed back")
			assert.Nil(t, report, "nor a report over units that never parsed")
		})

		t.Run("orders the graph by unit rather than by scheduling", func(t *testing.T) {
			t.Parallel()

			one, first, _ := loadTree(t, stdTree())
			two, second, _ := loadTree(t, stdTree())
			assert.Equal(t, encoded(t, two), encoded(t, one),
				"two loads of one tree encode identically")
			assert.Equal(t, keysOf(second), keysOf(first),
				"and fold the same key per unit")
		})
	})

	t.Run("splice", func(t *testing.T) {
		t.Parallel()

		t.Run("merges two units contributing one package path", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, twoDirTree(), with(names(sharedPath, sharedPath)))
			coretest.AssertCodes(t, sink)

			pkg := packageOf(t, g, sharedPath)
			assert.Length(t, pkg.Files, 2, "both units' files land in the one package")
			assert.Equal(t, pkg.Files[0].Path, oneFile, "in unit order")
			assert.Equal(t, pkg.Files[1].Path, twoFile, "which the splice fixed")
		})

		t.Run("resolves a record on any unit's package node", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				oneFile: {Data: []byte("package shared\ntype Twin left\n")},
				twoFile: {Data: []byte("package shared\npkgnote gen:table\n")},
			}
			g, _, sink := loadTree(t, tree)
			coretest.AssertCodes(t, sink)

			pkgID := symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: sharedPath, Kind: symbol.KindPackage,
			}
			raws := g.DirectivesOf(pkgID)
			assert.Length(t, raws, 1,
				"a directive through the merged-away node reaches the identity that stands")
			assert.Equal(t, string(raws[0].Name), "gen:table", "carrying the instance")
		})

		t.Run("reports one package path declared under two names", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, twoDirTree(), with(names("left", "right")))
			coretest.AssertReports(t, sink, load.DuplicateDeclaration)

			found, _ := findingOf(sink, load.DuplicateDeclaration)
			assert.Contains(t, found.Msg, "left", "naming the name that stands")
			assert.Contains(t, found.Msg, "right", "and the one that does not")
			assert.Equal(t, packageOf(t, g, sharedPath).Name, "left", "the first stands")
		})

		t.Run("fills an unnamed package from a later unit", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, twoDirTree(), with(names("", "right")))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, packageOf(t, g, sharedPath).Name, "right",
				"a unit that named nothing is not a disagreement")
		})
	})

	t.Run("attach", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches directives on assigned identities", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, stdTree())
			raws := g.DirectivesOf(rowID())
			assert.Length(t, raws, 1, "the carrier's instance is attached")
			assert.Equal(t, raws[0].Name, tableName, "under its spelling")
			assert.Equal(t, raws[0].Args[0].Key, "name", "arguments parsed")
		})

		t.Run("panics on a subject the resolution step never identified", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				_, _, _ = load.Load(context.Background(), mustConfig(
					oneFileTree(), with(&looseAttachment{frontendtest.NewScripted()}),
				))
			}, "a directive on an undeclared subject is the frontend's defect")
			assert.Contains(t, got, "never identified", "and says so")
		})
	})

	t.Run("attachStamps", func(t *testing.T) {
		t.Parallel()

		t.Run("carries classification stamps to the store", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"svc/api/user_test.zz": {Data: []byte(
					"package svc/api\nstamp fake.testFile yes\ntype UserTest string\n",
				)},
			}
			g, _, _ := loadTree(t, tree)
			file := symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: apiPath, Name: "svc/api/user_test.zz",
				Kind: symbol.KindFile,
			}
			stamps := g.StampsOf(file)
			assert.Length(t, stamps, 1, "the classifier's stamp is carried")
			assert.Equal(t, stamps[0].Key, frontendtest.ScriptedTestKey, "under its key")
			assert.Equal(t, stamps[0].Value.(string), "yes", "with its value")
			assert.Equal(t, stamps[0].Origin, frontendtest.ScriptedID,
				"the origin is the kernel's fill, not the frontend's word")
		})

		t.Run("panics on a subject the resolution step never identified", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				_, _, _ = load.Load(context.Background(), mustConfig(
					oneFileTree(), with(&looseStamp{frontendtest.NewScripted()}),
				))
			}, "a stamp on an undeclared subject is the frontend's defect")
			assert.Contains(t, got, "never identified", "and says so")
		})
	})

	t.Run("subjectIdentity", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches a dropped duplicate's directive to the standing twin", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				oneFile: {Data: []byte("package shared\ntype Twin left\n")},
				twoFile: {Data: []byte("package shared\ntype Twin right\n+gen:table name=twins\n")},
			}
			g, _, sink := loadTree(t, tree)
			coretest.AssertReports(t, sink, load.DuplicateDeclaration)

			raws := g.DirectivesOf(twinID())
			assert.Length(t, raws, 1,
				"the survivor's identity stands for what the duplicate carried")
			assert.Equal(t, raws[0].Name, tableName, "under its spelling")
		})
	})
}

// encoded renders every package the graph holds, so two loads
// compare whole rather than declaration by declaration.
func encoded(tb assert.TB, g *store.Graph) []string {
	tb.Helper()

	var out []string
	for pkg := range g.ByKind(symbol.KindPackage) {
		for decl := range node.Declarations(pkg) {
			out = append(out, decl.Identity().String())
		}
	}
	return out
}

// packageOf returns the package the graph holds under a path.
func packageOf(tb assert.TB, g *store.Graph, path string) *node.Package {
	tb.Helper()

	held, found := g.Lookup(symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: path, Kind: symbol.KindPackage,
	})
	assert.True(tb, found, "the graph holds the package the case is about")
	return held.(*node.Package)
}

// mustConfig is [config] for a case that drives the load itself,
// because the driver panics rather than returning.
func mustConfig(tree fs.FS, mutate ...func(*load.Config)) load.Config {
	cfg, _ := config(tree, mutate...)
	return cfg
}

// twoDirTree is two directories the scripted partition groups into
// two units, so a case can make two units contribute one package.
func twoDirTree() fstest.MapFS {
	return fstest.MapFS{
		oneFile: {Data: []byte("package shared\n")},
		twoFile: {Data: []byte("package shared\n")},
	}
}

// oneFileTree is a single claimed file, for the cases whose
// frontend builds its declarations rather than reading them.
func oneFileTree() fstest.MapFS {
	return fstest.MapFS{oneFile: {Data: []byte("package loose\n")}}
}

// failingFS is a tree whose named path refuses to open, so a walk
// or a read that meets it fails for a filesystem's own reason. It
// implements the one method, leaving Stat, ReadDir and ReadFile to
// the fallbacks in io/fs, which is what puts every access through
// the refusal.
type failingFS struct {
	tree fstest.MapFS
	fail string
}

// Open returns the tree's file, or a refusal for the named path.
func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.fail {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return f.tree.Open(name)
}

// versionless hides the fake's version, which the driver must
// refuse.
// hiddenOptions declares an options struct with an unexported
// field, which the canonical encoding cannot see and the load must
// refuse.
type hiddenOptions struct {
	*frontendtest.Scripted
}

func (hiddenOptions) Options() any {
	return &struct {
		Tag    string
		secret string
	}{}
}

type versionless struct {
	f *frontendtest.Scripted
}

func (v versionless) Name() plugin.ID              { return v.f.Name() }
func (v versionless) Lang() symbol.Lang            { return v.f.Lang() }
func (v versionless) Syntax() plugin.CommentSyntax { return v.f.Syntax() }
func (v versionless) Selection() []string          { return v.f.Selection() }
func (v versionless) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	return v.f.Parse(ctx, u)
}

func (v versionless) Partition(
	ctx context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return v.f.Partition(ctx, files, r)
}

func (v versionless) Resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
	return v.f.Resolve(scope, spelling)
}

// optionless hides the fake's options, which is the shape of a
// frontend declaring no configuration at all.
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

// errPartitionRefused is what the refusing frontend's partition
// returns, so a case can follow the cause up through the driver.
var errPartitionRefused = errors.New("load_test: the partition refuses")

// refusing fails to partition, which is fatal to the whole load.
type refusing struct {
	*frontendtest.Scripted
}

// Partition refuses.
func (*refusing) Partition(
	context.Context, []plugin.SourceRef, plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return nil, errPartitionRefused
}

// idle claims nothing and refuses to partition, so a driver that
// asks it anyway fails rather than passing quietly.
type idle struct {
	*frontendtest.Scripted
}

// Partition refuses, because nothing should ever call it.
func (*idle) Partition(
	context.Context, []plugin.SourceRef, plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return nil, errors.New("load_test: a frontend claiming nothing was asked to partition")
}

// partitioning returns the partition a case states, so the contract
// check meets a real violation rather than a stubbed one.
type partitioning struct {
	*frontendtest.Scripted
	parts func([]plugin.SourceRef) [][]plugin.SourceRef
}

// partitions returns a frontend whose partition is the given one.
func partitions(parts func([]plugin.SourceRef) [][]plugin.SourceRef) *partitioning {
	return &partitioning{Scripted: frontendtest.NewScripted(), parts: parts}
}

// Partition returns the stated partition.
func (p *partitioning) Partition(
	_ context.Context, files []plugin.SourceRef, _ plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return p.parts(files), nil
}

// unencodableOptions carries a channel, which no canonical encoding
// renders.
type unencodableOptions struct {
	Ticks chan int
}

// unencodable declares options the driver cannot fold into a key.
type unencodable struct {
	*frontendtest.Scripted
}

// Options returns the unencodable declaration.
func (*unencodable) Options() any { return &unencodableOptions{} }

// errParseRefused is what the unparsable frontend returns, so a
// case can follow the cause up through the driver.
var errParseRefused = errors.New("load_test: the parse refuses")

// unparsable fails to parse, which is fatal to the whole load.
type unparsable struct {
	*frontendtest.Scripted
}

// Parse refuses.
func (*unparsable) Parse(context.Context, *plugin.SourceUnit) error {
	return errParseRefused
}

// naming declares one package path under a name chosen per unit, so
// two units can agree or disagree about what the package is called.
type naming struct {
	*frontendtest.Scripted
	byFirstMember map[string]string
}

// names returns a frontend naming the shared package first for the
// earlier unit and second for the later one.
func names(first, second string) *naming {
	return &naming{
		Scripted:      frontendtest.NewScripted(),
		byFirstMember: map[string]string{oneFile: first, twoFile: second},
	}
}

// Parse declares the shared package under this unit's name and adds
// the unit's one file to it.
func (n *naming) Parse(_ context.Context, u *plugin.SourceUnit) error {
	path := u.Files()[0].Path
	pkg := u.Graph().Package(sharedPath)
	pkg.Name = n.byFirstMember[path]
	pkg.Files = append(pkg.Files, &node.File{Path: path})
	return nil
}

// looseAttachment attaches a directive to a declaration it never
// puts in a file, so the assignment step never identifies it.
type looseAttachment struct {
	*frontendtest.Scripted
}

// Parse declares a package and attaches to a subject outside it.
func (*looseAttachment) Parse(_ context.Context, u *plugin.SourceUnit) error {
	gb := u.Graph()
	pkg := gb.Package(loosePath)
	pkg.Files = append(pkg.Files, &node.File{Path: u.Files()[0].Path})
	gb.Attach(&node.Struct{Name: looseName}, directive.Raw{Name: tableName})
	return nil
}

// looseStamp stamps a declaration it never puts in a file, so the
// assignment step never identifies it.
type looseStamp struct {
	*frontendtest.Scripted
}

// Parse declares a package and stamps a subject outside it.
func (*looseStamp) Parse(_ context.Context, u *plugin.SourceUnit) error {
	gb := u.Graph()
	pkg := gb.Package(loosePath)
	pkg.Files = append(pkg.Files, &node.File{Path: u.Files()[0].Path})
	gb.Stamp(&node.Struct{Name: looseName}, meta.RawStamp{
		Key: frontendtest.ScriptedTestKey, Value: "yes",
	})
	return nil
}
