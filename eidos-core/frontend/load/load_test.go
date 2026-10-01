// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
// about by constant.
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
// statement writes.
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

// The files the ownership proof's probe cases add: a file many
// times the probe's size, and a file whose last line names the
// trailer's key under another brand with no frame around it.
const (
	bulkFile    = "svc/bulk/bulk.zz"
	bulkTypes   = 256
	keyedPath   = "svc/keyed"
	keyedFile   = "svc/keyed/keyed.zz"
	keyedName   = "Keyed"
	keyedSource = "package svc/keyed\ntype Keyed string\n// " + string(foreignBrand) +
		":provenance sha256:short\n"
	// paddingLine repeated paddingLines times grows a generated body
	// past the probe, so its head contains no trailer.
	paddingLine  = "// a line that grows the generated body\n"
	paddingLines = 8
	// proofError opens the error the ownership proof returns for a
	// file it cannot read.
	proofError = "load: read "
)

// The package documentation the two units of one package state, so
// a case can check which one the splice keeps.
const (
	earlierDoc = "Package shared is documented in the earlier unit."
	laterDoc   = "Package shared is documented in the later unit."
)

// The brand every standard load runs under, and another tool's.
const (
	ownBrand     output.Brand = "own"
	foreignBrand output.Brand = "foreign"
)

// The store paths the door cases read: the one directory at the
// dependency store's root, a store no load provides, and an entry of
// the library's directory that one case makes a file and another a
// directory.
const (
	extRoot     = "ext"
	absentStore = "absent"
	dualEntry   = "ext/lib/x"
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

// config is the standard load configuration over a tree, under the
// own brand, mutated per case.
func config(tree fs.FS, mutate ...func(*load.Config)) (load.Config, *diag.Sink) {
	sink := diag.NewSink()
	cfg := load.Config{
		FS:         tree,
		Frontends:  []plugin.Frontend{frontendtest.NewScripted()},
		Sink:       sink,
		PluginSet:  []byte("set-1"),
		Signatures: []string{depPath},
		Brand:      ownBrand,
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

// stamped returns source framed as one brand's output, so a case
// can put a generated file into a tree.
func stamped(tb assert.TB, brand output.Brand, source string) []byte {
	tb.Helper()

	contract, err := output.NewContract(brand, frontendtest.NewScripted().Syntax())
	assert.NoError(tb, err, "the contract builds over the fixture language's comment syntax")
	b, err := contract.Stamp(plugin.RenderedFile{
		Name: "gen.zz", Plugins: []plugin.ID{"gen"}, Body: []byte(source),
	})
	assert.NoError(tb, err, "the source stamps")
	return b
}

// refuse drives one load the case states must fail and returns the
// driver's error, so the case asserts on its text.
func refuse(tb assert.TB, tree fs.FS, mutate ...func(*load.Config)) error {
	tb.Helper()

	cfg, _ := config(tree, mutate...)
	_, _, err := load.Load(context.Background(), cfg)
	assert.HasError(tb, err, "the load fails and seals no graph")
	return err
}

// findingOf returns the first finding the sink contains under a
// code, for a case asserting on a finding's address and message.
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

		t.Run("returns a sealed graph of the tree's declarations", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, stdTree())
			assert.True(t, g.Frozen(), "the load ends at the seal")

			row, held := g.Lookup(rowID())
			assert.True(t, held, "a loaded declaration is indexed")
			assert.Equal(t, row.(*node.Struct).Name, rowName, "the declaration has its own kind")

			pkg, held := g.PackageOf(rowID())
			assert.True(t, held, "the declaration has its package")
			assert.Equal(t, pkg.ID, symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath, Kind: symbol.KindPackage,
			}, "the package identity is canonical")

			_, held = g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath, Name: storeFile, Kind: symbol.KindFile,
			})
			assert.True(t, held, "the file is a declaration of its package")
		})

		t.Run("returns a report listing every unit in splice order", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			first := make([]string, 0, len(report.Units))
			for _, u := range report.Units {
				assert.Equal(t, u.Frontend, frontendtest.ScriptedID, "each unit names its frontend")
				assert.NotEmpty(t, u.Key, "each unit records its key")
				first = append(first, u.Files[0])
			}
			assert.Equal(t, first, []string{apiFile, depFile, storeFile},
				"the units sort by their first member's path, which claims make unique")
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
			assert.True(t, held, "one Twin is indexed")
			assert.Equal(t, twin.(*node.Struct).Fields[0].Type.Spelling, "left", "the Twin is the first in unit order")
		})

		t.Run("replays unit findings under the frontend's origin", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"bad/oops.zz": {Data: []byte("type Lost string\n")},
			}
			_, _, sink := loadTree(t, tree)
			found, held := findingOf(sink, frontendtest.ScriptedBadFile)
			assert.True(t, held, "the unit's finding is replayed into the run sink")
			assert.Equal(t, found.Origin, frontendtest.ScriptedID, "the finding has the frontend's origin")
			assert.Equal(t, found.Pos.File, "bad/oops.zz", "the finding is positioned")
			coretest.AssertPositioned(t, sink)
		})

		t.Run("returns an error naming a config without a tree", func(t *testing.T) {
			t.Parallel()

			_, _, err := load.Load(context.Background(), load.Config{Sink: diag.NewSink(), Brand: ownBrand})
			assert.HasError(t, err, "there is nothing to read")
			assert.Contains(t, err.Error(), "no tree to read", "the error names the missing tree")
		})

		t.Run("returns an error naming a config without a sink", func(t *testing.T) {
			t.Parallel()

			_, _, err := load.Load(context.Background(), load.Config{FS: stdTree(), Brand: ownBrand})
			assert.HasError(t, err, "there is nowhere to report")
			assert.Contains(t, err.Error(), "no sink to report into", "the error names the missing sink")
		})

		t.Run("returns an error for a config without a brand", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), func(cfg *load.Config) { cfg.Brand = "" })
			assert.Contains(t, err.Error(), "not a brand", "the error names the missing brand")
		})

		t.Run("returns an error naming a brand outside the spelling", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), func(cfg *load.Config) { cfg.Brand = "Own" })
			assert.Contains(t, err.Error(), `"Own"`, "the error names the brand")
		})

		t.Run("returns an error for a frontend without a version", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(versionless{frontendtest.NewScripted()}))
			assert.Contains(t, err.Error(), "version", "every unit key folds the declared version")
		})

		t.Run("returns an error naming an options field the key cannot see", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(hiddenOptions{frontendtest.NewScripted()}))
			assert.Contains(t, err.Error(), "secret", "the error names the hidden field")
		})
	})

	t.Run("dropOutput", func(t *testing.T) {
		t.Parallel()

		outputFile := "svc/store/row_gen.zz"
		generated := "package svc/store\ntype Generated string\n"
		generatedID := symbol.Identity{
			Lang: frontendtest.ScriptedLang, Package: storePath,
			Name: "Generated", Kind: symbol.KindStruct,
		}

		t.Run("excludes the brand's own outputs before the partition", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[outputFile] = &fstest.MapFile{Data: stamped(t, ownBrand, generated)}
			g, report, sink := loadTree(t, tree)
			coretest.AssertCodes(t, sink)
			assert.Equal(t, report.Excluded, []string{outputFile}, "the report lists the excluded file")
			for _, u := range report.Units {
				assert.False(t, slices.Contains(u.Files, outputFile), "no unit contains the file")
			}
			_, held := g.Lookup(generatedID)
			assert.False(t, held, "the file's declarations never enter the graph")
		})

		t.Run("loads another brand's output as ordinary input", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[outputFile] = &fstest.MapFile{Data: stamped(t, foreignBrand, generated)}
			g, report, _ := loadTree(t, tree)
			assert.Empty(t, report.Excluded, "a foreign frame proves nothing to this load")
			_, held := g.Lookup(generatedID)
			assert.True(t, held, "the file's declarations load")
		})

		t.Run("lists the excluded files sorted by path", func(t *testing.T) {
			t.Parallel()

			// The walk visits a/x.zz before a-b.zz, and a byte sort
			// puts a-b.zz first, because '-' sorts before '/'.
			tree := fstest.MapFS{
				"a/x.zz": {Data: stamped(t, ownBrand, "package a\ntype X string\n")},
				"a-b.zz": {Data: stamped(t, ownBrand, "package ab\ntype Y string\n")},
			}
			_, report, _ := loadTree(t, tree)
			assert.Equal(t, report.Excluded, []string{"a-b.zz", "a/x.zz"}, "the files are sorted by path")
		})

		t.Run("returns a read's own error", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, failingFS{tree: stdTree(), fail: storeFile})
			assert.Contains(t, err.Error(), storeFile, "the error names the file the proof could not read")
		})

		t.Run("reads a file without a trailer once beside its tail", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			bulk := bulkSource()
			tree[bulkFile] = &fstest.MapFile{Data: bulk}
			counted := &countingFS{tree: tree, read: map[string]int{}}
			loadTree(t, counted)
			assert.Equal(t, counted.bytesOf(bulkFile), len(bulk)+output.TailSize,
				"the proof reads the tail, and the unit reads the file")
		})

		t.Run("excludes the brand's own output longer than the probe", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			long := generated + strings.Repeat(paddingLine, paddingLines)
			tree[outputFile] = &fstest.MapFile{Data: stamped(t, ownBrand, long)}
			_, report, _ := loadTree(t, tree)
			assert.Equal(t, report.Excluded, []string{outputFile}, "the probe reads the file's tail")
		})

		t.Run("excludes the brand's own output from files that cannot seek", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[outputFile] = &fstest.MapFile{Data: stamped(t, ownBrand, generated)}
			_, report, _ := loadTree(t, unseekableFS{tree: tree})
			assert.Equal(t, report.Excluded, []string{outputFile}, "a file that cannot seek is read whole")
		})

		t.Run("loads a file whose tail names the trailer key without a frame", func(t *testing.T) {
			t.Parallel()

			tree := stdTree()
			tree[keyedFile] = &fstest.MapFile{Data: []byte(keyedSource)}
			g, _, _ := loadTree(t, tree)
			_, held := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: keyedPath, Name: keyedName, Kind: symbol.KindStruct,
			})
			assert.True(t, held, "a key without a frame proves nothing, so the file loads")
		})

		faults := []struct {
			name  string
			fault fault
		}{
			{name: "returns a seek's own error", fault: faultSeek},
			{name: "returns a rewind's own error", fault: faultRewind},
			{name: "returns a tail read's own error", fault: faultRead},
			{name: "returns a whole read's own error", fault: faultReopen},
		}
		for _, tt := range faults {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree := stdTree()
				tree[outputFile] = &fstest.MapFile{Data: stamped(t, ownBrand, generated)}
				err := refuse(t, &faultyFS{tree: tree, path: outputFile, fault: tt.fault})
				assert.True(t, errors.Is(err, errFault), "the error wraps the filesystem's own")
				assert.HasPrefix(t, err.Error(), proofError+outputFile, "the proof returns it, naming the file")
			})
		}
	})

	t.Run("treeFiles", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error wrapping a walk's cause", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, failingFS{tree: stdTree(), fail: "."})
			assert.Contains(t, err.Error(), "walk the tree", "the error names the phase")
			assert.ErrorIs(t, err, fs.ErrPermission, "the error wraps the filesystem's cause")
		})
	})

	t.Run("claim", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming two claims on one file", func(t *testing.T) {
			t.Parallel()

			rival := frontendtest.NewScripted()
			rival.ID = "rival"
			err := refuse(t, stdTree(), with(frontendtest.NewScripted(), rival))
			assert.Contains(t, err.Error(), string(frontendtest.ScriptedID), "the error names the first claimant")
			assert.Contains(t, err.Error(), "rival", "the error names the second claimant")
		})
	})

	t.Run("partitionAll", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error wrapping the frontend's partition error", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(&refusing{frontendtest.NewScripted()}))
			assert.ErrorIs(t, err, errPartitionRefused, "the error wraps the frontend's cause")
			assert.Contains(t, err.Error(), "partition", "the error names the phase")
		})

		t.Run("never asks a frontend claiming nothing to partition", func(t *testing.T) {
			t.Parallel()

			quiet := &idle{frontendtest.NewScripted()}
			quiet.ID = "idle"
			quiet.Sel = []string{"**/*.nothing"}
			_, report, _ := loadTree(t, stdTree(), with(frontendtest.NewScripted(), quiet))
			for _, u := range report.Units {
				assert.Equal(t, u.Frontend, frontendtest.ScriptedID, "the claiming frontend produced every unit")
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

		t.Run("returns an error for a unit without members", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, one(), with(partitions(func([]plugin.SourceRef) [][]plugin.SourceRef {
				return [][]plugin.SourceRef{{}}
			})))
			assert.Contains(t, err.Error(), "no members", "an empty unit has nothing to parse")
		})

		t.Run("returns an error naming a member the selection never claimed", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, one(), with(partitions(func([]plugin.SourceRef) [][]plugin.SourceRef {
				return [][]plugin.SourceRef{{{Path: modFile}}}
			})))
			assert.Contains(t, err.Error(), modFile, "the error names the file outside the claim")
			assert.Contains(t, err.Error(), "never claimed", "the error names why it may not be a member")
		})

		t.Run("returns an error naming a claimed file in no unit", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, one(), with(partitions(func([]plugin.SourceRef) [][]plugin.SourceRef {
				return nil
			})))
			assert.Contains(t, err.Error(), oneFile, "the error names the dropped file")
			assert.Contains(t, err.Error(), "in no unit", "the error names the silent drop")
		})

		t.Run("returns an error naming a file in two units", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, one(), with(partitions(func(files []plugin.SourceRef) [][]plugin.SourceRef {
				return [][]plugin.SourceRef{{files[0]}, {files[0]}}
			})))
			assert.Contains(t, err.Error(), oneFile, "the error names the shared file")
			assert.Contains(t, err.Error(), "2 units", "the error names how many units contain it")
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
				assert.NotEqual(t, key, none[file], "a declared configuration folds where none folds nothing")
			}
		})

		t.Run("returns an error naming a frontend whose options cannot encode", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(&unencodable{frontendtest.NewScripted()}))
			assert.Contains(t, err.Error(), "encode", "the error names the phase")
			assert.Contains(t, err.Error(), string(frontendtest.ScriptedID), "the error names the frontend")
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
				assert.Equal(t, u.Depth, plugin.DepthFull, "an empty root names no directory")
			}
		})
	})

	t.Run("parseAll", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error wrapping a frontend's parse error", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), with(&unparsable{frontendtest.NewScripted()}))
			assert.ErrorIs(t, err, errParseRefused, "the error wraps the frontend's cause")
			assert.Contains(t, err.Error(), "parse", "the error names the phase")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			cfg, _ := config(stdTree())
			g, report, err := load.Load(ctx, cfg)
			assert.ErrorIs(t, err, context.Canceled, "a skipped unit has no source unit to key")
			assert.HasPrefix(t, err.Error(), "load: ", "the error has the package prefix")
			assert.Nil(t, g, "no half-loaded graph is returned")
			assert.Nil(t, report, "no report over units that never parsed is returned")
		})

		t.Run("orders the graph by unit whatever the scheduling", func(t *testing.T) {
			t.Parallel()

			one, first, _ := loadTree(t, stdTree())
			two, second, _ := loadTree(t, stdTree())
			assert.Equal(t, encoded(t, two), encoded(t, one), "two loads of one tree encode identically")
			assert.Equal(t, keysOf(second), keysOf(first), "two loads of one tree fold the same keys")
		})
	})

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a store file's bytes for a qualified path", func(t *testing.T) {
			t.Parallel()

			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), read: member(libFile)}
			loadTree(t, depTree(), with(probe), stores(depStore()))
			assert.NoError(t, probe.readErr, "the store provides the file")
			assert.Equal(t, probe.bytes, depStore()[libFile].Data, "the bytes are the store's")
		})
	})

	t.Run("ReadDir", func(t *testing.T) {
		t.Parallel()

		t.Run("lists a workspace directory", func(t *testing.T) {
			t.Parallel()

			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), list: appPath}
			loadTree(t, depTree(), with(probe))
			assert.NoError(t, probe.listErr, "the workspace provides the directory")
			assert.Equal(t, entryNames(probe.entries), []string{path.Base(appFile)}, "the entries are the tree's")
		})

		t.Run("lists a store's root", func(t *testing.T) {
			t.Parallel()

			probe := &probing{
				ScriptedDependent: frontendtest.NewScriptedDependent(),
				list:              plugin.StorePath(frontendtest.ScriptedStore, ""),
			}
			loadTree(t, depTree(), with(probe), stores(depStore()))
			assert.NoError(t, probe.listErr, "the store has a root")
			assert.Equal(t, entryNames(probe.entries), []string{extRoot}, "the root lists the store's one directory")
		})

		t.Run("returns ErrStoreAbsent naming a store the load does not provide", func(t *testing.T) {
			t.Parallel()

			absent := plugin.StorePath(absentStore, libPath)
			probe := &probing{ScriptedDependent: frontendtest.NewScriptedDependent(), list: absent}
			loadTree(t, depTree(), with(probe))
			assert.ErrorIs(t, probe.listErr, plugin.ErrStoreAbsent, "the load provides no such store")
			assert.Contains(t, probe.listErr.Error(), absent, "the error names the path")
		})

		t.Run("records a directory entry apart from a file of its name", func(t *testing.T) {
			t.Parallel()

			flat := depStore()
			flat[dualEntry] = &fstest.MapFile{Data: []byte("not source\n")}
			_, asFile, _ := loadTree(t, depTree(), with(recorded()), stores(flat))
			nested := depStore()
			nested[path.Join(dualEntry, path.Base(noteFile))] = &fstest.MapFile{Data: []byte("not source\n")}
			_, asDir, _ := loadTree(t, depTree(), with(recorded()), stores(nested))
			assert.NotEqual(t, keysOf(asDir)[member(libFile)], keysOf(asFile)[member(libFile)],
				"a listing records which of its entries are directories")
		})
	})

	t.Run("checkStores", func(t *testing.T) {
		t.Parallel()

		names := []struct {
			name string
			give string
		}{
			{name: "returns an error naming an empty store name", give: ""},
			{name: "returns an error naming a store name with a colon", give: "go:mod"},
			{name: "returns an error naming a store name with a slash", give: "go/mod"},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := refuse(t, stdTree(), func(cfg *load.Config) {
					cfg.Stores = map[string]fs.FS{tt.give: fstest.MapFS{}}
				})
				assert.Contains(t, err.Error(), strconv.Quote(tt.give), "the error names the store")
				assert.Contains(t, err.Error(), "cannot name a store", "a qualified path cannot spell the name")
			})
		}

		t.Run("returns an error naming a store without a tree", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, stdTree(), func(cfg *load.Config) {
				cfg.Stores = map[string]fs.FS{frontendtest.ScriptedStore: nil}
			})
			assert.Contains(t, err.Error(), frontendtest.ScriptedStore, "the error names the store")
			assert.Contains(t, err.Error(), "no tree", "the store has nothing to read")
		})
	})

	t.Run("splice", func(t *testing.T) {
		t.Parallel()

		t.Run("merges two units contributing one package path", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, twoDirTree(), with(names(sharedPath, sharedPath)))
			coretest.AssertCodes(t, sink)

			pkg := packageOf(t, g, sharedPath)
			assert.Length(t, pkg.Files, 2, "both units' files are in the one package")
			assert.Equal(t, pkg.Files[0].Path, oneFile, "the files are in unit order")
			assert.Equal(t, pkg.Files[1].Path, twoFile, "the splice fixed the order")
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
			assert.Length(t, raws, 1, "the directive on the merged-away node attaches to the kept identity")
			assert.Equal(t, raws[0].Name, tableName, "the instance is the one the statement wrote")
		})

		t.Run("reports one package path declared under two names", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, twoDirTree(), with(names("left", "right")))
			coretest.AssertReports(t, sink, load.DuplicateDeclaration)

			found, _ := findingOf(sink, load.DuplicateDeclaration)
			assert.Contains(t, found.Msg, "left", "the finding names the kept name")
			assert.Contains(t, found.Msg, "right", "the finding names the dropped name")
			assert.Equal(t, packageOf(t, g, sharedPath).Name, "left", "the first name is kept")
		})

		t.Run("keeps the documentation of a later unit where the earlier states none", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, twoDirTree(), with(documents("", laterDoc)))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, packageOf(t, g, sharedPath).Doc, []string{laterDoc}, "the later documentation is kept")
		})

		t.Run("keeps the first documentation a merged package states", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, twoDirTree(), with(documents(earlierDoc, laterDoc)))
			assert.Equal(t, packageOf(t, g, sharedPath).Doc, []string{earlierDoc}, "the earlier documentation is kept")
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
			assert.Length(t, raws, 1, "the statement's instance is attached")
			assert.Equal(t, raws[0].Name, tableName, "the instance has its spelling")
			assert.Equal(t, raws[0].Args[0].Key, "name", "the arguments are parsed")
		})

		t.Run("attaches a negated carrier as a negated instance", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				storeFile: {Data: []byte("package svc/store\n// -" + string(ownBrand) + ":table\ntype Row int\n")},
			}
			g, _, _ := loadTree(t, tree)
			raws := g.DirectivesOf(rowID())
			assert.Length(t, raws, 1, "the carrier's instance is attached")
			assert.True(t, raws[0].Negated, "the instance is negated")
		})

		t.Run("reads no carrier written under another brand", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				storeFile: {Data: []byte("package svc/store\n// +" + string(foreignBrand) + ":table\ntype Row int\n")},
			}
			g, _, _ := loadTree(t, tree)
			assert.Empty(t, g.DirectivesOf(rowID()), "the foreign carrier is documentation")
		})

		t.Run("panics on a subject the resolution step never identified", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				_, _, _ = load.Load(context.Background(), mustConfig(
					oneFileTree(), with(&looseAttachment{frontendtest.NewScripted()}),
				))
			}, "a directive on an undeclared subject is the frontend's defect")
			assert.Contains(t, got, "never identified", "the panic names the defect")
		})
	})

	t.Run("attachStamps", func(t *testing.T) {
		t.Parallel()

		t.Run("records classification stamps in the store", func(t *testing.T) {
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
			assert.Length(t, stamps, 1, "the classifier's stamp is recorded")
			assert.Equal(t, stamps[0].Key, frontendtest.ScriptedTestKey, "the stamp has its key")
			assert.Equal(t, stamps[0].Value.(string), "yes", "the stamp has its value")
			assert.Equal(t, stamps[0].Origin, frontendtest.ScriptedID, "the kernel sets the origin")
		})

		t.Run("panics on a subject the resolution step never identified", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				_, _, _ = load.Load(context.Background(), mustConfig(
					oneFileTree(), with(&looseStamp{frontendtest.NewScripted()}),
				))
			}, "a stamp on an undeclared subject is the frontend's defect")
			assert.Contains(t, got, "never identified", "the panic names the defect")
		})
	})

	t.Run("subjectIdentity", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches a dropped duplicate's directive to the kept twin", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				oneFile: {Data: []byte("package shared\ntype Twin left\n")},
				twoFile: {Data: []byte("package shared\ntype Twin right\n+gen:table name=twins\n")},
			}
			g, _, sink := loadTree(t, tree)
			coretest.AssertReports(t, sink, load.DuplicateDeclaration)

			raws := g.DirectivesOf(twinID())
			assert.Length(t, raws, 1, "the duplicate's directive attaches to the kept twin's identity")
			assert.Equal(t, raws[0].Name, tableName, "the instance has its spelling")
		})
	})
}

// encoded renders the identities of every package the graph
// contains, so two loads compare whole.
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

// packageOf returns the package the graph contains under a path.
func packageOf(tb assert.TB, g *store.Graph, path string) *node.Package {
	tb.Helper()

	found, held := g.Lookup(symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: path, Kind: symbol.KindPackage,
	})
	assert.True(tb, held, "the graph contains the package the case is about")
	return found.(*node.Package)
}

// mustConfig is [config] for a case that drives the load itself,
// because the driver panics and never returns.
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
// frontend builds its declarations without reading them.
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

// bulkSource returns a scripted file of bulkTypes declarations,
// many times the size of the ownership proof's probe.
func bulkSource() []byte {
	var b strings.Builder
	b.WriteString("package svc/bulk\n")
	for i := range bulkTypes {
		fmt.Fprintf(&b, "type Bulk%d string\n", i)
	}
	return []byte(b.String())
}

// countingFS counts the bytes each path's reads return, so a case
// can pin how much of a file a load reads. It implements Open alone,
// so every read of a file goes through a counted handle, and it
// passes a directory through untouched.
type countingFS struct {
	tree fstest.MapFS
	mu   sync.Mutex
	read map[string]int
}

// Open returns a directory as the tree has it, and a file behind a
// counting handle.
func (c *countingFS) Open(name string) (fs.File, error) {
	f, err := c.tree.Open(name)
	if err != nil {
		return nil, err
	}
	if _, isDir := f.(fs.ReadDirFile); isDir {
		return f, nil
	}
	return &countingFile{File: f, fs: c, name: name}, nil
}

// bytesOf returns how many bytes the reads of a path returned.
func (c *countingFS) bytesOf(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.read[name]
}

// countingFile counts what its reads return into its filesystem.
type countingFile struct {
	fs.File
	fs   *countingFS
	name string
}

// Read reads through the file and counts the bytes it returns.
func (f *countingFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	f.fs.mu.Lock()
	f.fs.read[f.name] += n
	f.fs.mu.Unlock()
	return n, err
}

// Seek seeks through the file, which every map file supports.
func (f *countingFile) Seek(offset int64, whence int) (int64, error) {
	s, seeks := f.File.(io.Seeker)
	if !seeks {
		return 0, errors.ErrUnsupported
	}
	return s.Seek(offset, whence)
}

// unseekableFS hands out files that cannot seek, the shape of a
// filesystem whose files stream. It passes a directory through.
type unseekableFS struct {
	tree fstest.MapFS
}

// Open returns a directory as the tree has it, and a file behind a
// handle without Seek.
func (u unseekableFS) Open(name string) (fs.File, error) {
	f, err := u.tree.Open(name)
	if err != nil {
		return nil, err
	}
	if _, isDir := f.(fs.ReadDirFile); isDir {
		return f, nil
	}
	return streamFile{f: f}, nil
}

// streamFile is a file with Stat, Read and Close alone.
type streamFile struct {
	f fs.File
}

// Stat returns the file's information.
func (s streamFile) Stat() (fs.FileInfo, error) { return s.f.Stat() }

// Read reads the file's next bytes.
func (s streamFile) Read(p []byte) (int, error) { return s.f.Read(p) }

// Close closes the file.
func (s streamFile) Close() error { return s.f.Close() }

// fault names the one operation a faulty file fails.
type fault uint8

// The operations a faulty file can fail: the Seek to its end, the
// Seek back to the tail's start, every Read, or its path's second
// Open, the whole read after the probe.
const (
	faultSeek fault = iota + 1
	faultRewind
	faultRead
	faultReopen
)

// errFault is the error every fault returns.
var errFault = errors.New("load_test: the fixture's fault")

// faultyFS fails one operation on one path and opens every other
// path as the tree has it.
type faultyFS struct {
	tree  fstest.MapFS
	path  string
	fault fault
	opens atomic.Int64
}

// Open returns the path's file behind a faulty handle, and fails the
// path's second Open under faultReopen.
func (f *faultyFS) Open(name string) (fs.File, error) {
	file, err := f.tree.Open(name)
	if err != nil || name != f.path {
		return file, err
	}
	if f.opens.Add(1) > 1 && f.fault == faultReopen {
		return nil, errFault
	}
	return &faultyFile{File: file, fault: f.fault}, nil
}

// faultyFile fails the operation its fault names.
type faultyFile struct {
	fs.File
	fault fault
}

// Seek fails a seek from the file's end under faultSeek, and a seek
// from its start under faultRewind.
func (f *faultyFile) Seek(offset int64, whence int) (int64, error) {
	s, seeks := f.File.(io.Seeker)
	switch {
	case !seeks,
		f.fault == faultSeek && whence == io.SeekEnd,
		f.fault == faultRewind && whence == io.SeekStart:
		return 0, errFault
	default:
		return s.Seek(offset, whence)
	}
}

// Read fails under faultRead.
func (f *faultyFile) Read(p []byte) (int, error) {
	if f.fault == faultRead {
		return 0, errFault
	}
	return f.File.Read(p)
}

// hiddenOptions declares an options struct with an unexported
// field, which the canonical encoding cannot see and the load must
// refuse.
type hiddenOptions struct {
	*frontendtest.Scripted
}

// Options returns a struct with a field the encoding cannot see.
func (hiddenOptions) Options() any {
	return &struct {
		Tag    string
		secret string
	}{}
}

// versionless hides the fake's version, which the driver must
// refuse.
type versionless struct {
	f *frontendtest.Scripted
}

// Name returns the wrapped frontend's name.
func (v versionless) Name() plugin.ID { return v.f.Name() }

// Lang returns the wrapped frontend's language.
func (v versionless) Lang() symbol.Lang { return v.f.Lang() }

// Syntax returns the wrapped frontend's comment syntax.
func (v versionless) Syntax() plugin.CommentSyntax { return v.f.Syntax() }

// Overloads reports whether the wrapped frontend's language
// overloads.
func (v versionless) Overloads() bool { return v.f.Overloads() }

// Selection returns the wrapped frontend's claim.
func (v versionless) Selection() []string { return v.f.Selection() }

// Parse lowers the unit through the wrapped frontend.
func (v versionless) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	return v.f.Parse(ctx, u)
}

// Partition groups the files through the wrapped frontend.
func (v versionless) Partition(
	ctx context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return v.f.Partition(ctx, files, r)
}

// Resolve probes a spelling through the wrapped frontend.
func (v versionless) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	return v.f.Resolve(scope, spelling)
}

// optionless hides the fake's options, which is the shape of a
// frontend declaring no configuration at all.
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

// errPartitionRefused is what the refusing frontend's partition
// returns, so a case can follow the cause up through the driver.
var errPartitionRefused = errors.New("load_test: the partition refuses")

// refusing fails to partition, which is fatal to the whole load.
type refusing struct {
	*frontendtest.Scripted
}

// Partition returns errPartitionRefused.
func (*refusing) Partition(
	context.Context, []plugin.SourceRef, plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return nil, errPartitionRefused
}

// idle claims nothing and refuses to partition, so a driver that
// asks it anyway fails the load.
type idle struct {
	*frontendtest.Scripted
}

// Partition returns an error, because nothing should ever call it.
func (*idle) Partition(
	context.Context, []plugin.SourceRef, plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return nil, errors.New("load_test: the driver asks a frontend claiming nothing to partition")
}

// partitioning returns the partition a case states, so the contract
// check meets a violation the partition itself commits.
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

// unencodableOptions has a channel field, which no canonical
// encoding renders.
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

// Parse returns errParseRefused.
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

// documenting declares one package path with documentation chosen
// per unit, so each unit can document the package or leave it
// undocumented.
type documenting struct {
	*frontendtest.Scripted
	byFirstMember map[string]string
}

// documents returns a frontend documenting the shared package with
// first in the earlier unit and second in the later one, and
// leaving it undocumented where the text is empty.
func documents(first, second string) *documenting {
	return &documenting{
		Scripted:      frontendtest.NewScripted(),
		byFirstMember: map[string]string{oneFile: first, twoFile: second},
	}
}

// Parse declares the shared package with this unit's documentation
// and adds the unit's one file to it.
func (d *documenting) Parse(_ context.Context, u *plugin.SourceUnit) error {
	path := u.Files()[0].Path
	pkg := u.Graph().Package(sharedPath)
	pkg.Name = sharedPath
	if doc := d.byFirstMember[path]; doc != "" {
		pkg.Doc = []string{doc}
	}
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

// probing is the scripted language in the dependent role whose rounds
// read one path and list another through the round's door, and record
// what each returned. It returns no unit, so its first round is its
// last.
type probing struct {
	*frontendtest.ScriptedDependent
	read    string
	list    string
	bytes   []byte
	readErr error
	entries []fs.DirEntry
	listErr error
}

// Dependencies reads and lists the stated paths and returns no unit.
func (p *probing) Dependencies(
	_ context.Context, _ *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	if p.read != "" {
		p.bytes, p.readErr = r.Read(p.read)
	}
	if p.list != "" {
		p.entries, p.listErr = r.ReadDir(p.list)
	}
	return nil, nil
}

// entryNames returns the names of a listing's entries, in listing
// order.
func entryNames(entries []fs.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
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
