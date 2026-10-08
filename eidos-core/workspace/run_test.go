// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"text/template"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/files"
	"go.dokimi.dev/assert/history"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// runCode is a code for the run fixtures' findings.
var runCode = diag.Code{Prefix: "tst", Number: 7}

// flagGroup is the fact group the fixture key registers into, so a
// case can drop the group and not the key.
const flagGroup meta.GroupName = "shape.all"

// The plugin whose directive the negation cases write, and that
// directive's canonical spelling.
const (
	negatingPlugin plugin.ID      = "mirror"
	optOutName     directive.Name = "mirror:stub"
)

// The sizes of the parallel run fixtures: structs in one package, and
// the instances of the weaver's repeatable directive on one struct.
const (
	parallelSubjects  = 64
	parallelInstances = 32
)

// The worker counts the runs hand their phase calls: sequential
// dispatch, four workers and eight workers.
const (
	oneWorker    = 1
	fourWorkers  = 4
	eightWorkers = 8
)

// mirroredCapability is what the providing mirror declares and the
// weaver requires, which puts the weaver in the later bucket.
const mirroredCapability plugin.Capability = "mirrored"

// auditSchema is the weaver's repeatable directive: each instance on a
// subject adds one field to the subject's mirrored struct.
var auditSchema = directive.Schema{
	Plugin: "weaver", Name: "audit", Repeatable: true,
	Doc: "adds one audited field to the mirrored struct",
}

// The pipeline corpus at the canonical scale: two hundred structs per
// package, of which ten in a hundred have the mark the annotator
// stamps, and three of those ten reference a struct in the next
// package, so the settle follows resolved references across packages.
// The names encode the roles, which keeps the corpus deterministic
// without a seed. The corpus has no raw directives.
const (
	// pipelinePackages is the benchmark's package count: 200,000
	// structs.
	pipelinePackages = 1_000
	// pipelineTestPackages is the share of the corpus the tests run.
	pipelineTestPackages = 50
	// pipelineRefs, pipelineMarked and pipelineUnmarked split one
	// package's structs by role: marked with a cross-package
	// reference, marked, and unmarked.
	pipelineRefs     = 6
	pipelineMarked   = 14
	pipelineUnmarked = 180
	// pipelinePathPrefix is the path of every corpus package, before
	// the package's index.
	pipelinePathPrefix = "example.com/e2e/pkg"
	// pipelineBrand is the brand the pipeline's composition declares
	// and stamps its files under.
	pipelineBrand output.Brand = "e2e"
	// pipelineMarkedPrefix opens the name of every struct the annotator
	// marks, pipelineRefPrefix the referencing ones among them, and
	// pipelineMarkPrefix the others. pipelinePlainPrefix opens the name of
	// every unmarked struct.
	pipelineMarkedPrefix = "m"
	pipelineRefPrefix    = "mr"
	pipelineMarkPrefix   = "mk"
	pipelinePlainPrefix  = "pl"
	// pipelineScript is the file that spells a corpus package in the
	// scripted language, in the package's own directory.
	pipelineScript = "unit.zz"
	// pipelineStructs is the number of structs of one corpus package.
	pipelineStructs = pipelineRefs + pipelineMarked + pipelineUnmarked
	// corpusFields is the number of fields of a struct of the corpus's
	// tree before an edit widens it.
	corpusFields = 2
)

// The documentation each reader of the edge corpus writes on its mirror:
// what it read of another package through its edge kind.
const (
	lookupDoc     = "%s of the next package has %d fields"
	factDoc       = "%s of the next package is %d fields wide"
	membershipDoc = "the scope declares %d structs"
	packageDoc    = "the package after the next declares %d fields"
)

// The package and the structs of the edge corpus, the pipeline corpus's
// tree that the warm checks edit. In each package, the first four
// referencing structs read another package through one edge kind each.
// The mirror of each of them documents what the struct read. Each edit
// changes the package edgeEdited. The struct that reads the edited
// declaration is in a file that no other edge kind of the edit changes.
const (
	// edgeEdited is the index of the package each edit changes.
	edgeEdited = 7
	// edgeLookup looks up edgeLookedUp of the next package.
	edgeLookup   = "mr0"
	edgeLookedUp = "mk0"
	// edgeFact reads the width the annotator stamps on edgeMeasured of
	// the next package.
	edgeFact     = "mr1"
	edgeMeasured = "mk1"
	// edgeMembership counts the structs of its scope, in the first
	// package alone.
	edgeMembership = "mr2"
	// edgePackage sums the fields of the package after the next one,
	// which it reads whole.
	edgePackage = "mr3"
	// edgeWidened is a plain struct only the package reader reads,
	// edgeLast the last plain struct of a package, and edgeAppended the
	// struct an edit declares after it. edgeRemoved is a plain struct the
	// stubs plan stubs, as it stubs every plain struct whose name ends in
	// edgeStubbed.
	edgeWidened  = "pl7"
	edgeLast     = "pl179"
	edgeAppended = "pl180"
	edgeRemoved  = "pl10"
	edgeStubbed  = "0"
)

// measurerID is the annotator of the edge corpus, which stamps the mark
// and the width of every marked struct.
const measurerID plugin.ID = "measurer"

// The ceilings of [BenchmarkRun]. Each cold ceiling covers one run whose
// pooled state the collector emptied before the run began.
const (
	// coldRunAllocs is one run over the canonical workspace of 200,000
	// declarations, 1,091,311 on average with a standard deviation of 114
	// over 24 fresh processes. The fact store allocates three times for
	// each subject's first claim, 600,000 in all, and sync.Map adds about
	// 75,600 trie nodes at random where the hashes of the subjects share
	// a prefix. The mirror allocates twice for each struct, and the phase
	// calls, their journals, the emit store and the seal allocate about
	// 15,000 times. The ceiling allows eight standard deviations above the
	// mean.
	coldRunAllocs = 1_091_311 + 8*114
	// oneRunAllocs is one run over one declaration. The frame's own
	// structures allocate 125 times, among them the seal, the fact store,
	// the emit store and the phase calls' state. The headroom of 12 is for
	// the runtime's own allocations, which a fresh process counts at one
	// iteration: the plan's goroutine, the sudogs whose central cache the
	// collector cleared, and the type-assertion caches. 200 fresh
	// processes counted 0 to 8 of them.
	oneRunAllocs = 125 + 12
	// warmRunAllocs is one run over one declaration in a process whose
	// pools already contain the phase calls' state. Most runs allocate 98
	// times. The seal allocates 24 of them, and the annotation, the plan,
	// the fact store, the index, the sink, the frame's goroutines and the
	// report allocate the rest. A collection between two runs can empty
	// the pool, and the next run then allocates the phase calls' state
	// again. The mean of 20 runs in one process measured up to 100.
	warmRunAllocs = 98 + 2
	// pipelineAllocs is one cold run of the pipeline over 200,000
	// declarations on one worker, 440,246 on average with a standard
	// deviation of 36 over 24 fresh processes. A memory profile
	// attributes about 211,000 to the render, where text/template's
	// reflection executes the struct template once for each of the
	// 20,000 mirrors. The annotation's stamps allocate about 67,000: two
	// for each of the 20,000 marked subjects, and the sync.Map trie
	// nodes. The generate phase allocates about 66,000, the settle's
	// respell about 27,000 and the write about 25,000. The rest are tiny
	// allocations, such as the mirrors' names, which the profiler
	// samples only in part. The ceiling allows eight standard deviations
	// above the mean.
	pipelineAllocs = 440_246 + 8*36
	// parallelPipelineAllocs is the same run on four workers, 441,280 on
	// average with a standard deviation of 45 over 24 fresh processes.
	// The parallel phase calls add about 1,000 allocations for their
	// goroutines and for their lanes' effect buffers and journals.
	parallelPipelineAllocs = 441_280 + 8*45
	// treeRunAllocs is one cold run of the edge corpus's tree of 200,000
	// declarations into a fresh memory ledger, its load and its record of
	// the state included, 2,604,412 on average with a standard deviation
	// of 41 over 24 fresh processes. A memory profile of one run
	// attributes about 1,690,000 to the scripted frontend's parse of the
	// tree, 199,000 to the phase calls' handlers, most of them the
	// annotator's stamps, and 181,000 to the render's templates. The
	// record of the phases allocates only to grow its buffers. The
	// profile counts 2,231,387 and misses about 373,000 allocations below
	// 16 bytes that share a block of the tiny allocator. The ceiling allows
	// eight standard deviations above the mean.
	treeRunAllocs = 2_604_412 + 8*41
	// warmTreeRunAllocs is one warm run over the edge corpus's tree of
	// 200,000 declarations after an edit that widens one struct,
	// 1,932,585 on average with a standard deviation of 31 over 24 fresh
	// processes. A memory profile of four runs attributes these shares to
	// one run:
	//
	//   - The load keeps 999 of the 1,000 units. The run lists the
	//     residents of every directory, which reads every package, and the
	//     plans read every declaration, so the run decodes every region:
	//     about 1,235,000.
	//   - The plans' generators allocate about 291,000. About 142,000 of
	//     them are the reads of the mark of every struct. The run restores
	//     the bags of the 20,000 marked structs for those reads, and reads
	//     the other structs absent through the recorded presence.
	//   - The render allocates about 181,000, the manifest's documents
	//     about 58,000, the commit of the phase record about 50,000 and the
	//     load about 31,000.
	//
	// The profile misses about 68,000 allocations below 16 bytes that share
	// a block of the tiny allocator. The ceiling allows eight standard
	// deviations above the mean.
	warmTreeRunAllocs = 1_932_585 + 8*31
)

// dropping is a backend whose lowering hook returns a declaration
// without an origin: the defect the settle refuses, and the one way
// a plan fails after its schedule ran whole.
type dropping struct{ fakeBackend }

// Lower returns one declaration without an origin.
func (dropping) Lower(symbol.Symbol) ([]symbol.Symbol, error) {
	return []symbol.Symbol{&emit.Struct{Name: "made"}}, nil
}

// annotateSeen is a hand-rolled annotator that records the worker
// count its phase call hands it.
type annotateSeen struct{ seen *atomic.Int64 }

// Name returns the annotator's name.
func (annotateSeen) Name() plugin.ID { return "annotate-seen" }

// Annotate records the call's worker count.
func (a annotateSeen) Annotate(ctx *plugin.AnnotatorContext) error {
	a.seen.Store(int64(ctx.Workers))
	return nil
}

// generateSeen is a hand-rolled generator that records the worker
// count its phase call hands it.
type generateSeen struct{ seen *atomic.Int64 }

// Name returns the generator's name.
func (generateSeen) Name() plugin.ID { return "generate-seen" }

// Generate records the call's worker count.
func (g generateSeen) Generate(ctx *plugin.GeneratorContext) error {
	g.seen.Store(int64(ctx.Workers))
	return nil
}

// native is the scripted rules registered under the fixture's
// language, resolving a name as a struct in the fixture package and
// recording the scope it was asked from.
type native struct {
	scope *rules.Scope
}

// Lang returns the fixture's language.
func (native) Lang() symbol.Lang { return coretest.Lang }

// Members returns the scripted rules' member policy.
func (native) Members() rules.MemberPolicy { return rulestest.Scripted().Members() }

// ParamRole returns the scripted rules' role for a parameter.
func (native) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	return rulestest.Scripted().ParamRole(p, v)
}

// ReturnRoles returns the scripted rules' roles for the returns.
func (native) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return rulestest.Scripted().ReturnRoles(rs, v)
}

// Builtin returns the scripted rules' shape for a builtin.
func (native) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	return rulestest.Scripted().Builtin(ref, v)
}

// Resolve returns the fixture package's struct named name, and
// records the scope it was asked from.
func (n native) Resolve(
	scope rules.Scope, name string, _ directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	if n.scope != nil {
		*n.scope = scope
	}
	if sym, held := v.Lookup(coretest.ID(coretest.StorePath, name, symbol.KindStruct)); held {
		return sym, nil
	}
	return nil, fmt.Errorf("nothing in %s is named %s", coretest.StorePath, name)
}

// SamplesOf returns the scripted rules' samples.
func (native) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return rulestest.Scripted().SamplesOf(ref, hint, v)
}

// ZeroValue returns the scripted rules' zero value.
func (native) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().ZeroValue(ref, v)
}

// LiteralFor returns the scripted rules' literal for text.
func (native) LiteralFor(f *node.File, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().LiteralFor(f, ref, text, v)
}

// TypeName returns the scripted rules' type name.
func (native) TypeName(word, base string) string { return rulestest.Scripted().TypeName(word, base) }

// corpusKeys are the handles of the two keys the measuring annotator
// registers. The mark selects a struct for mirroring, and the width
// counts the struct's fields. Each Build sets both handles, and the
// readers plan's generator reads them through the pointer after the
// Build of its run.
type corpusKeys struct {
	mark  meta.Key[bool]
	width meta.Key[int64]
}

// Run is the frame: it loads or takes a graph, seals it, validates the
// directives, applies the drops, annotates, and runs each plan through
// its generators, its settle, its render and its commit. The cases pin
// which inputs stop the frame, which findings fail the run, and what
// the report records. A warm run after an edit that a generator reads
// through one edge kind leaves what a cold run over the edited tree
// leaves, for each edge kind.
func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a report with one emit store per plan", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the fixture run is clean")
			assert.NotNil(t, report.Sink, "the report has the findings")
			assert.NotNil(t, report.Facts, "the report has the arbitrated facts")
			assert.Length(t, report.Emits, 1, "the report has one store per plan")
			assert.Length(t, units(report.Emits["plan"]), 1, "the store has the mirrored unit")
		})

		t.Run("returns a store of its own to each concurrent run", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			graphs := make([]*store.Graph, 2)
			for i := range graphs {
				graphs[i], _ = alpha(t)
			}
			outcomes := history.Concurrently(len(graphs), time.Minute, func(run int) (any, error) {
				return w.Run(t.Context(), workspace.Input{Graph: graphs[run]})
			})
			stores := make([]*plugin.Emit, len(outcomes))
			for i, o := range outcomes {
				assert.True(t, o.Finished, "each run finishes")
				assert.NoError(t, o.Error, "each run is clean")
				report, _ := o.Output.(*workspace.Report)
				assert.NotNil(t, report, "each run returns its report")
				stores[i] = report.Emits["plan"]
				assert.Length(t, units(stores[i]), 1, "each run has its own unit")
			}
			assert.NotEqual(t, stores[1], stores[0], "the runs share no store", assert.ByIdentity())
		})

		t.Run("runs a graph the load already sealed", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			g, _ := alpha(t)
			g.Freeze()
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run takes the sealed graph")
			assert.NotNil(t, report, "the frame runs whole")
		})

		t.Run("writes the file a loaded tree's plan renders", func(t *testing.T) {
			t.Parallel()

			var o opener
			w, err := workspace.New().
				Brand(fixtureBrand).
				Frontends(frontendtest.NewScripted()).
				Annotators(stamper("noter", quiet)).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{mirror("mirror")},
					Backend:    printer(t, "fixture"),
				}).
				Output(o.open).
				Build()
			assert.NoError(t, err, "the loading composition composes")
			tree := fstest.MapFS{
				"svc/store/row.zz": {Data: []byte("package svc/store\ntype Row int string\n")},
			}
			run, err := w.Run(t.Context(), workspace.Input{Tree: tree})
			assert.NoError(t, err, "the run loads the tree and writes")
			assert.Length(t, run.Load.Units, 1, "one unit is parsed")
			changes := run.Plans[0].Changes
			assert.Length(t, changes, 1, "the run writes the rendered file")
			assert.Equal(t, changes[0].Path, "svc/store/gen.txt",
				"the path is the package's directory and the target's filename")
			assert.Equal(t, changes[0].Action, output.ActionCreated, "the file is new")

			stamped := o.opened()[0].Files()[changes[0].Path]
			p, framed := output.Read(stamped)
			assert.True(t, framed, "the file has a frame")
			assert.Equal(t, p.Brand, w.Brand(), "the frame claims the composition's brand")
		})

		t.Run("returns an error for the zero Input", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			report, err := w.Run(t.Context(), workspace.Input{})
			assert.HasError(t, err, "there is nothing to run over")
			assert.Nil(t, report, "nothing ran")
		})

		t.Run("returns an error for an Input that sets both of its sources", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Tree: fstest.MapFS{}, Graph: g})
			assert.HasError(t, err, "one input is read")
			assert.Nil(t, report, "nothing ran")
		})

		t.Run("stamps the key a plugin registers", func(t *testing.T) {
			t.Parallel()

			report, s, flag, err := keyedRun(t, nil)
			assert.NoError(t, err, "the run is clean")
			assert.False(t, report.Sink.Failed(), "nothing is reported")
			v, held := meta.Get(report.Facts, s, flag)
			assert.True(t, held, "the plugin's key is stamped")
			assert.True(t, v, "the stamp is the plugin's value")
		})

		t.Run("mirrors the stamped subject in the plan", func(t *testing.T) {
			t.Parallel()

			report, s, _, err := keyedRun(t, nil)
			assert.NoError(t, err, "the run is clean")
			got := units(report.Emits["plan"])
			assert.Length(t, got, 1, "the flagged subject is mirrored")
			assert.Equal(t, got[0].Plugin, plugin.ID("mirror"), "the unit names its plugin")
			assert.Equal(t, got[0].Origins, []symbol.Identity{s}, "the unit has its provenance")
		})

		t.Run("settles the plan's store before the report", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, nil)
			assert.NoError(t, err, "the run is clean")
			assert.True(t, report.Emits["plan"].Settled(), "the store is settled")
		})

		t.Run("drops a fact the meta directive names", func(t *testing.T) {
			t.Parallel()

			report, s, flag, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				assert.NoError(t, g.AttachDirectives(s, []directive.Raw{rawMeta("shape.flag", 4)}),
					"the drop attaches before the seal")
			})
			assert.NoError(t, err, "a drop is authored intent, not a finding")
			assert.False(t, report.Sink.Failed(), "nothing is reported")
			_, held := meta.Get(report.Facts, s, flag)
			assert.False(t, held, "the drop outranks the stamp whenever it arrives")
			assert.Empty(t, units(report.Emits["plan"]), "the flag gates nothing")
		})

		t.Run("drops every member fact of a group the meta directive names", func(t *testing.T) {
			t.Parallel()

			report, s, flag, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				assert.NoError(t, g.AttachDirectives(s, []directive.Raw{rawMeta(string(flagGroup), 4)}),
					"the group drop attaches before the seal")
			})
			assert.NoError(t, err, "a drop is authored intent, not a finding")
			coretest.AssertCodes(t, report.Sink)
			_, held := meta.Get(report.Facts, s, flag)
			assert.False(t, held, "the tombstone covers the group")
			assert.Empty(t, units(report.Emits["plan"]), "the flag gates nothing")
		})

		t.Run("keeps the facts under directives that drop nothing", func(t *testing.T) {
			t.Parallel()

			report, s, flag, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				assert.NoError(t, g.AttachDirectives(s, []directive.Raw{
					rawDiag("tst-0007", 4),
					rawBareMeta(5),
				}), "the instances attach before the seal")
			})
			assert.NoError(t, err, "neither instance is a fault")
			coretest.AssertCodes(t, report.Sink)
			v, held := meta.Get(report.Facts, s, flag)
			assert.True(t, held, "the diag instance and the bare meta instance remove nothing")
			assert.True(t, v, "the fact keeps the plugin's value")
			assert.Length(t, units(report.Emits["plan"]), 1, "the flag still gates")
		})

		t.Run("applies a load stamp at plugin authority", func(t *testing.T) {
			t.Parallel()

			var pkg symbol.Identity
			report, _, flag, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				pkg = symbol.Identity{Lang: s.Lang, Package: s.Package, Kind: symbol.KindPackage}
				assert.NoError(t, g.AttachStamps(pkg, []meta.RawStamp{{
					Key: "shape.flag", Value: true,
					Pos:    position.Pos{File: "alpha.go", Line: 1},
					Origin: "fakefront",
				}}), "the raw stamp attaches before the seal")
			})
			assert.NoError(t, err, "the run is clean")
			v, held := meta.Get(report.Facts, pkg, flag)
			assert.True(t, held, "the load's classification reads back through the typed handle")
			assert.True(t, v, "the fact is the load's value")
		})

		strayStamp := func(g *store.Graph, s symbol.Identity) {
			assert.NoError(t, g.AttachStamps(s, []meta.RawStamp{{
				Key: "shape.ghost", Value: true,
				Pos:    position.Pos{File: "alpha.go", Line: 2},
				Origin: "fakefront",
			}}), "the stray stamp attaches")
		}

		t.Run("reports RefusedStamp for a stamp under an unregistered key", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, strayStamp)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the Error finding fails the run")
			coretest.AssertReports(t, report.Sink, meta.RefusedStamp)
		})

		t.Run("runs the plans after a refused stamp", func(t *testing.T) {
			t.Parallel()

			report, _, _, _ := keyedRun(t, strayStamp)
			assert.Length(t, units(report.Emits["plan"]), 1, "the frame runs whole")
		})

		unclaimed := func(g *store.Graph, s symbol.Identity) {
			assert.NoError(t, g.AttachDirectives(s, []directive.Raw{{
				Name: "ghost",
				Pos:  position.Pos{File: "alpha.go", Line: 5, Col: 1},
			}}), "the stray instance attaches")
		}

		t.Run("reports UnclaimedName for a directive nothing registered", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, unclaimed)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the Error finding fails the run")
			coretest.AssertReports(t, report.Sink, directive.UnclaimedName)
		})

		t.Run("runs the plans after an unclaimed directive", func(t *testing.T) {
			t.Parallel()

			report, _, _, _ := keyedRun(t, unclaimed)
			assert.Length(t, units(report.Emits["plan"]), 1, "the frame runs whole")
		})

		t.Run("reports nothing for an ignored foreign directive", func(t *testing.T) {
			t.Parallel()

			b, _ := flagged()
			w, err := b.Ignore("k8s:").Build()
			assert.NoError(t, err, "the composition opts out of a foreign tool's prefix")
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{{
				Name: "k8s:deepcopy-gen",
				Pos:  position.Pos{File: "alpha.go", Line: 5, Col: 1},
			}}), "the foreign directive attaches like any other")
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run passes")
			coretest.AssertCodes(t, report.Sink)
			assert.Length(t, units(report.Emits["plan"]), 1, "the frame runs whole")
		})

		t.Run("reports DanglingSubject for a directive on a subject the graph does not contain", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, func(g *store.Graph, _ symbol.Identity) {
				ghost := coretest.Struct("example.com/elsewhere", "Ghost")
				assert.NoError(t, g.AttachDirectives(ghost.Identity(), []directive.Raw{rawMeta("shape.flag", 9)}),
					"the dangling attachment arrives before the seal")
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "a dangling subject is an Error")
			coretest.AssertReports(t, report.Sink, directive.DanglingSubject)
		})

		t.Run("reports validation findings in position order", func(t *testing.T) {
			t.Parallel()

			const subjects = 64
			report, _, _, err := keyedRun(t, func(g *store.Graph, _ symbol.Identity) {
				for i := range subjects {
					ghost := coretest.Struct("example.com/elsewhere", fmt.Sprintf("Ghost%02d", i))
					// Identity order runs against position order, so a
					// report in subject or completion order is out of
					// position order.
					assert.NoError(t,
						g.AttachDirectives(ghost.Identity(), []directive.Raw{rawMeta("shape.flag", subjects-i)}),
						"the dangling attachment arrives before the seal")
				}
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "a dangling subject is an Error")
			var lines []int
			for d := range report.Sink.All() {
				if d.Code == directive.DanglingSubject {
					lines = append(lines, d.Pos.Line)
				}
			}
			assert.Length(t, lines, subjects, "each subject is one finding")
			assert.Pairwise(t, lines, func(earlier, later int) bool { return earlier < later },
				"the findings are in position order")
		})

		t.Run("validates a directive on a package", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, func(g *store.Graph, _ symbol.Identity) {
				pkg := coretest.PackageID(coretest.StorePath)
				assert.NoError(t, g.AttachDirectives(pkg, []directive.Raw{rawMeta("shape.flag", 9)}),
					"a package-subject directive attaches before the seal")
			})
			assert.NoError(t, err, "a package is a subject the graph contains")
			coretest.AssertCodes(t, report.Sink)
		})

		t.Run("reports DanglingSubject for a stamp on a subject the graph does not contain", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, func(g *store.Graph, _ symbol.Identity) {
				ghost := coretest.Struct("example.com/elsewhere", "Ghost")
				assert.NoError(t, g.AttachStamps(ghost.Identity(), []meta.RawStamp{{
					Key: "shape.ghostly", Value: true,
				}}), "the dangling stamp arrives before the seal")
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "a dangling stamp is an Error")
			coretest.AssertReports(t, report.Sink, directive.DanglingSubject)
		})

		t.Run("skips a subject that negates a plugin's directive in that plugin's bare rule", func(t *testing.T) {
			t.Parallel()

			w, err := optingOut(true).Build()
			assert.NoError(t, err, "the composition composes")
			g, first, _ := pair(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the negation is authored intent, not a finding")
			got := units(report.Emits["plan"])
			assert.Length(t, got, 1, "one unit is emitted")
			assert.Equal(t, got[0].Origins, []symbol.Identity{first.Identity()},
				"only the subject that did not negate is mirrored")
		})

		t.Run("reports NegationRefused for a negated directive whose schema is not negatable", func(t *testing.T) {
			t.Parallel()

			w, err := optingOut(false).Build()
			assert.NoError(t, err, "the composition composes")
			g, _, _ := pair(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused negation fails the run")
			coretest.AssertReports(t, report.Sink, directive.NegationRefused)
		})

		t.Run("mirrors a subject whose negation was refused", func(t *testing.T) {
			t.Parallel()

			w, err := optingOut(false).Build()
			assert.NoError(t, err, "the composition composes")
			g, first, second := pair(t)
			report, _ := w.Run(t.Context(), workspace.Input{Graph: g})
			got := units(report.Emits["plan"])
			assert.Length(t, got, 1, "one unit is emitted")
			assert.Equal(t, got[0].Origins, []symbol.Identity{first.Identity(), second.Identity()},
				"a refused instance opts nothing out")
		})

		t.Run("returns an annotator's error without running the plans", func(t *testing.T) {
			t.Parallel()

			angry := stamper("angry", func(*eidos.StructMatch, *eidos.Stamper) error {
				return errors.New("boom")
			})
			w, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(angry).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the failing composition composes")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.HasError(t, err, "the returned error stops the frame")
			assert.Contains(t, err.Error(), "angry", "the error names the role")
			assert.Contains(t, err.Error(), "boom", "the error has the cause")
			assert.ErrorIsNot(t, err, workspace.ErrRunFailed, "a handler error is a defect, not a finding")
			assert.Empty(t, report.Emits, "no plan ran")
		})

		t.Run("runs a plan whose sibling fails", func(t *testing.T) {
			t.Parallel()

			bad := generator("bad", func(*eidos.StructMatch, *eidos.Emitter) error {
				return errors.New("boom")
			})
			w, err := workspace.New().
				Brand(fixtureBrand).
				Targets("fixture").
				Plans(
					planTo("crashing", "fixture", bad),
					planTo("steady", "fixture", mirror("mirror")),
				).
				Build()
			assert.NoError(t, err, "the two-plan composition composes")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.HasError(t, err, "the failing plan is reported")
			assert.Contains(t, err.Error(), `"crashing"`, "the error names the plan")
			assert.Length(t, units(report.Emits["steady"]), 1, "the sibling ran whole")
		})

		t.Run("returns a settle error for a lowering that drops the origin", func(t *testing.T) {
			t.Parallel()

			w, err := workspace.New().
				Brand(fixtureBrand).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{mirror("mirror")},
					Backend:    dropping{fakeBackend{name: "printer", target: "fixture"}},
				}).
				Build()
			assert.NoError(t, err, "a backend declaring a lowering seam composes")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.HasError(t, err, "the lowering fails the plan")
			assert.That(t, err.Error()).
				Contains("settle", "the error names the stage").
				Contains("origin", "the error names the rule the backend broke")
			assert.ErrorIsNot(t, err, workspace.ErrRunFailed, "a backend defect is not a finding")

			got := units(report.Emits["plan"])
			assert.Length(t, got, 1, "the plan's store is in the report")
			emitted, held := got[0].Decls[0].(*emit.Struct)
			assert.True(t, held, "the unit's declaration is the generator's struct")
			assert.Equal(t, emitted.Name, "ForAlpha",
				"the store has what the generator emitted, because a refused lowering replaces nothing")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			g, _ := alpha(t)
			var report *workspace.Report
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				var err error
				report, err = w.Run(ctx, workspace.Input{Graph: g})
				return err
			}, "the caller's cancellation is returned")
			assert.Equal(t, report.Plans, []workspace.PlanReport{{Name: "plan", Status: workspace.PlanCancelled}},
				"the report states the plan's outcome")
		})

		t.Run("reports UnreadableRecord for a record that does not read", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(
				t,
				files.Tree{ledger.ManifestPath(fixtureBrand) + "/ea.json": files.Text("not a record")},
			)
			report := cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{}))),
				routedIn(t, coretest.StorePath))
			unreadable := findings(report.Sink, workspace.UnreadableRecord)
			assert.Length(t, unreadable, 1, "one finding for the record")
			assert.Equal(t, unreadable[0].Severity, diag.SeverityInfo, "as information")
			assert.Equal(t, unreadable[0].Pos, position.Pos{File: ledger.ManifestPath(fixtureBrand)},
				"at the record's directory")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "the run proceeds")
			assert.Equal(t, paths(recorded(t, root)), []string{storeGen}, "and its commit replaces the broken record")
		})

		t.Run("returns an error for a ledger that fails to open", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return nil, errNoDevice }))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errNoDevice, "the open's error fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "no plan runs")
		})

		t.Run("returns an error for a ledger open function that returns two nils", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return nil, nil }))
			_, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.HasError(t, err, "the run fails")
			assert.Contains(t, err.Error(), "returned (nil, nil)", "the error names the fault")
		})

		t.Run("returns an error for a tree without a frontend to load it", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			_, err = w.Run(t.Context(), workspace.Input{Tree: fstest.MapFS{}})
			assert.HasError(t, err, "the tree is not loaded")
			assert.Contains(t, err.Error(), "no frontend", "the error names the fault")
		})

		t.Run("returns the load's error for a store it cannot load", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Frontends(frontendtest.NewScripted()).Build()
			assert.NoError(t, err, "the loading composition is valid")
			report, err := w.Run(t.Context(), workspace.Input{
				Tree: fstest.MapFS{}, Stores: map[string]fs.FS{"go:mod": fstest.MapFS{}},
			})
			assert.HasError(t, err, "the load fails")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "and no plan runs")
		})

		t.Run("stops before the next annotator after a cancellation", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			later := false
			w, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(
					stamperAt("first", 1, nil, nil,
						func(*eidos.StructMatch, *eidos.Stamper) error {
							cancel()
							return nil
						}),
					stamperAt("second", 2, nil, nil,
						func(*eidos.StructMatch, *eidos.Stamper) error {
							later = true
							return nil
						}),
				).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the two-annotator composition composes")
			g, _ := alpha(t)
			_, err = w.Run(ctx, workspace.Input{Graph: g})
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
			assert.False(t, later, "the schedule stops at the next role")
		})

		t.Run("stops before the next generator after a cancellation", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			later := false
			w, err := workspace.New().
				Brand(fixtureBrand).
				Targets("fixture").
				Plans(planTo("plan", "fixture",
					generatorAt("first", 1, func(*eidos.StructMatch, *eidos.Emitter) error {
						cancel()
						return nil
					}),
					generatorAt("second", 2, func(*eidos.StructMatch, *eidos.Emitter) error {
						later = true
						return nil
					}),
				)).
				Build()
			assert.NoError(t, err, "the two-generator plan composes")
			g, _ := alpha(t)
			report, err := w.Run(ctx, workspace.Input{Graph: g})
			assert.ErrorIs(t, err, context.Canceled, "the plan returns the cancellation")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCancelled, "the report states the plan's outcome")
			assert.False(t, later, "the later bucket never ran")
		})

		grumpyRun := func(t *testing.T) (*workspace.Report, error) {
			t.Helper()

			grump := stamper("grump", func(m *eidos.StructMatch, st *eidos.Stamper) error {
				m.Errorf(runCode, "structs are refused here")
				return nil
			})
			w, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(grump).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the grumpy composition composes")
			g, _ := alpha(t)
			return w.Run(t.Context(), workspace.Input{Graph: g})
		}

		t.Run("returns ErrRunFailed for an Error finding", func(t *testing.T) {
			t.Parallel()

			_, err := grumpyRun(t)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the finding classifies the run")
		})

		t.Run("runs the plans after an Error finding", func(t *testing.T) {
			t.Parallel()

			report, _ := grumpyRun(t)
			assert.Length(t, units(report.Emits["plan"]), 1, "the frame runs to the end")
		})

		refSchema := directive.Schema{
			Plugin: "refy", Name: "ref",
			Params: []directive.ParamSpec{{
				Key: "to", Type: directive.TypeReference, Resolution: directive.ResolveCallableInScope,
				Doc: "the struct the directive points at",
			}},
			Doc: "points at a sibling",
		}
		// pointing returns a composition whose one generator records
		// what the ref directive's param bound to.
		pointing := func(bound *symbol.Identity) *workspace.Builder {
			gen, _ := eidos.NewPlugin("refy").
				Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
				Handle(eidos.Directive(refSchema, eidos.OnStruct(
					func(m *eidos.StructMatch, e *eidos.Emitter) error {
						v, _ := m.Directive().Param("to")
						*bound = v.Target
						return nil
					},
				))).Build().(plugin.Generator)
			return workspace.New().
				Brand(fixtureBrand).
				Targets("fixture").
				Plans(workspace.Plan{
					Name: "plan", Generators: []plugin.Generator{gen},
					Backend: fakeBackend{name: "printer", target: "fixture"},
				})
		}

		t.Run("binds a reference param through the registered rules", func(t *testing.T) {
			t.Parallel()

			var bound symbol.Identity
			var seen rules.Scope
			w, err := pointing(&bound).Rules(native{scope: &seen}).Build()
			assert.NoError(t, err, "the composition composes")
			s := coretest.Struct(coretest.StorePath, "Alpha")
			s.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
			pkg := coretest.Package(coretest.StorePath, s)
			pkg.Files[0].Pos = position.Pos{File: "alpha.go", Line: 1, Col: 1}
			g := store.New()
			assert.NoError(t, g.AddPackage(pkg), "the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{rawRef("Alpha", 2)}),
				"the directive attaches")
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run is clean")
			assert.False(t, report.Sink.Failed(), "nothing is reported")
			assert.Equal(t, bound, s.Identity(), "the handler receives the bound identity")
			assert.Equal(t, seen.Subject, s.Identity(), "the rules are asked from the subject")
			assert.NotNil(t, seen.File, "the rules are asked in the subject's file")
		})

		t.Run("returns ErrRunFailed for a reference the rules bind to nothing", func(t *testing.T) {
			t.Parallel()

			var bound symbol.Identity
			w, err := pointing(&bound).Rules(native{}).Build()
			assert.NoError(t, err, "the composition composes")
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{rawRef("Ghost", 2)}),
				"the directive attaches")
			_, err = w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.Equal(t, bound, symbol.Identity{}, "the rejected instance never gated the rule")
		})

		t.Run("reports UnresolvedReference for a language without rules", func(t *testing.T) {
			t.Parallel()

			var bound symbol.Identity
			w, err := pointing(&bound).Build()
			assert.NoError(t, err, "the composition composes without rules")
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{rawRef("Alpha", 2)}),
				"the directive attaches")
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			coretest.AssertReports(t, report.Sink, directive.UnresolvedReference)
		})

		t.Run("binds the registered rules to every match", func(t *testing.T) {
			t.Parallel()

			var form symbol.TypeForm
			noter := stamper("noter", func(m *eidos.StructMatch, st *eidos.Stamper) error {
				form = m.Rules().TypeOf(&node.TypeRef{Spelling: "int"}).Form
				return nil
			})
			w, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(noter).
				Rules(native{}).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the composition composes")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, form, symbol.FormScalar, "the annotator's match folds through the registered rules")
			assert.NotContains(t, coretest.Codes(report.Sink), rules.AbsentRules, "no language is unregistered")
		})

		t.Run("hands the composition's worker count to every phase call", func(t *testing.T) {
			t.Parallel()

			annotated, generated := &atomic.Int64{}, &atomic.Int64{}
			w := built(t, workspace.New().
				Brand(fixtureBrand).
				Annotators(annotateSeen{seen: annotated}).
				Targets("fixture").
				Parallel(eightWorkers).
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{generateSeen{seen: generated}},
					Backend:    fakeBackend{name: "printer", target: "fixture"},
				}))
			g, _ := alpha(t)
			cleanRun(t, w, g)
			assert.Equal(t, annotated.Load(), int64(eightWorkers), "the annotator's call runs on the workers")
			assert.Equal(t, generated.Load(), int64(eightWorkers), "and so does the generator's")
		})

		t.Run("writes on eight workers the bytes one worker writes for a package's accumulator", func(t *testing.T) {
			t.Parallel()

			registry := func() plugin.Generator { return generator("registry", mirrored) }
			serial := parallelFiles(t, oneWorker, crowded(t, parallelSubjects, 0), registry())
			parallel := parallelFiles(t, eightWorkers, crowded(t, parallelSubjects, 0), registry())
			assert.Length(t, serial, 1, "the package's matches assemble one file")
			assert.Equal(t, parallel, serial, "the file does not depend on the worker count")
		})

		t.Run("writes on eight workers the bytes one worker writes for one slot many invocations append into",
			func(t *testing.T) {
				t.Parallel()

				serial := parallelFiles(t, oneWorker, crowded(t, 1, parallelInstances), providing(), weaverOf())
				parallel := parallelFiles(t, eightWorkers, crowded(t, 1, parallelInstances), providing(), weaverOf())
				assert.Length(t, serial, 1, "the woven struct renders in one file")
				for _, body := range serial {
					assert.Contains(t, body, "audit00 audit01", "the fields follow the instances' order")
				}
				assert.Equal(t, parallel, serial, "the file does not depend on the worker count")
			})

		t.Run("names a weaver in the frame of the file it appends into", func(t *testing.T) {
			t.Parallel()

			mirror, weaver := providing(), weaverOf()
			files := parallelFiles(t, oneWorker, crowded(t, 1, 1), mirror, weaver)
			assert.Length(t, files, 1, "the woven struct renders in one file")
			for _, body := range files {
				record, framed := output.Read([]byte(body))
				assert.True(t, framed, "the file is stamped")
				assert.Equal(t, record.Plugins, []plugin.ID{mirror.Name(), weaver.Name()},
					"the frame names the emitter and the weaver")
			}
		})

		t.Run("writes the bytes of its first run on every later run over the same corpus", func(t *testing.T) {
			t.Parallel()

			w := pipelineWorkspace(t, pipelineTestPackages, oneWorker)
			assert.Length(t, pipelineHashes(t, w), pipelineTestPackages, "one file per package commits")
			assert.Deterministic(t, func(w *workspace.Workspace) (map[string]string, error) {
				return pipelineHashes(t, w), nil
			}, w, "nothing a run leaves behind changes the next")
		})

		t.Run("writes on four workers the bytes one worker writes for the pipeline corpus", func(t *testing.T) {
			t.Parallel()

			parallel := pipelineHashes(t, pipelineWorkspace(t, pipelineTestPackages, fourWorkers))
			serial := pipelineHashes(t, pipelineWorkspace(t, pipelineTestPackages, oneWorker))
			assert.Equal(t, parallel, serial, "the hashes do not depend on the worker count")
		})

		edits := []struct {
			name   string
			edit   func(root string) error
			reader int
			want   string
		}{
			{
				name:   "matches a cold run after an edit to a declaration a generator looks up",
				edit:   corpusEdit(edgeEdited, corpusLine(edgeLookedUp), widenedLine(edgeLookedUp)),
				reader: edgeEdited - 1,
				want:   fmt.Sprintf(lookupDoc, edgeLookedUp, corpusFields+1),
			},
			{
				name:   "matches a cold run after an edit to a fact a generator reads of another declaration",
				edit:   corpusEdit(edgeEdited, corpusLine(edgeMeasured), widenedLine(edgeMeasured)),
				reader: edgeEdited - 1,
				want:   fmt.Sprintf(factDoc, edgeMeasured, corpusFields+1),
			},
			{
				name:   "matches a cold run after a struct appears in the scope a generator enumerates",
				edit:   corpusEdit(edgeEdited, corpusLine(edgeLast), corpusLine(edgeLast)+corpusLine(edgeAppended)),
				reader: 0,
				want:   fmt.Sprintf(membershipDoc, pipelineTestPackages*pipelineStructs+1),
			},
			{
				name:   "matches a cold run after an edit to a member of a package a generator reads whole",
				edit:   corpusEdit(edgeEdited, corpusLine(edgeWidened), widenedLine(edgeWidened)),
				reader: edgeEdited - 2,
				want:   fmt.Sprintf(packageDoc, pipelineStructs*corpusFields+1),
			},
			{
				name:   "matches a cold run after a struct another plan stubs disappears",
				edit:   corpusEdit(edgeEdited, corpusLine(edgeRemoved), ""),
				reader: edgeEdited - 2,
				want:   fmt.Sprintf(packageDoc, (pipelineStructs-1)*corpusFields),
			},
		}
		for _, tt := range edits {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				warm := t.TempDir()
				workspacetest.AssertWarmEdited(t, edgeFixture(tt.edit), warm, t.TempDir())
				assert.Contains(t, files.Read(t, filepath.Join(warm, pipelinePath(tt.reader), "gen.txt")), tt.want,
					"the struct that reads the edited declaration documents the edit")
			})
		}
	})
}

// A run over one declaration allocates within its ceilings in the
// ordinary run, which runs no benchmark: a warm run over pools that
// earlier runs filled, and a cold run over pools the collector emptied.
// Each run takes a graph loaded outside the count, because a run seals
// the graph it is given, and the cold run's setup empties the pools
// after the load. The runs over 200,000 declarations take too long to
// repeat 101 times, so only [BenchmarkRun] checks their ceilings. Each
// count keeps the first error of its runs, which cmp.Or returns without
// allocating.
func TestRunAllocs(t *testing.T) {
	builder, _ := flagged()
	w, err := builder.Build()
	assert.NoError(t, err, "the flagged composition composes")
	one := []*node.Package{coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Alpha"))}
	run := func(g *store.Graph) {
		_, rerr := w.Run(t.Context(), workspace.Input{Graph: g})
		err = cmp.Or(err, rerr)
	}
	assert.MaxAllocsWithSetup(t, func() *store.Graph { return loaded(t, one) }, run, warmRunAllocs,
		"a warm run over one declaration allocates the frame's own structures")
	assert.NoError(t, err, "every warm run succeeds")
	assert.MaxAllocsWithSetup(t, func() *store.Graph {
		g := loaded(t, one)
		emptyPools()
		return g
	}, run, oneRunAllocs, "a cold run over one declaration allocates the phase calls' state besides")
	assert.NoError(t, err, "every cold run succeeds")
}

// BenchmarkRun takes the frame over graphs it seals. Each iteration
// loads a fresh graph, because the seal is Run's own, and the load is
// outside the measurement. The cold cases also empty the pools that
// hold the phase calls' state, so each iteration counts what the first
// run of a process allocates.
//
// The flagged composition has one annotator stamping every struct
// through its own key provider, and one plan mirroring the flagged
// subjects toward a backend that renders nothing. The pipeline
// composition runs every stage the kernel implements: annotate,
// generate, settle, layout, render, stamp and commit into a memory
// sink. Its cases reset the process's peak resident set before they
// start and report it beside the counts, so the metric covers the case
// alone. The tree case runs the edge corpus's composition over a tree,
// its load inside the measurement, and records the state into a fresh
// memory ledger on each run. It reports the bytes of the state for each
// declaration. The warm case runs the same composition over one ledger,
// so each run reads the generation of the run before it, and reports the
// invocations of the annotator.
func BenchmarkRun(b *testing.B) {
	builder, _ := flagged()
	w, err := builder.Build()
	assert.NoError(b, err, "the flagged composition composes")
	one := []*node.Package{coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Alpha"))}

	b.Run("Run", func(b *testing.B) {
		b.Run("a cold run over 200,000 declarations", func(b *testing.B) {
			benchRun(b, w, coretest.Workspace(1_000, 10, 20), emptyPools, coldRunAllocs)
		})

		b.Run("a cold run over one declaration", func(b *testing.B) {
			benchRun(b, w, one, emptyPools, oneRunAllocs)
		})

		b.Run("a warm run over one declaration", func(b *testing.B) {
			benchRun(b, w, one, func() {}, warmRunAllocs)
		})

		b.Run("a cold run of the pipeline over 200,000 declarations on one worker", func(b *testing.B) {
			resetPeakRSS()
			report := benchRun(b, pipelineWorkspace(b, pipelinePackages, oneWorker),
				pipelineCorpus(pipelinePackages), emptyPools, pipelineAllocs)
			assert.Length(b, report.Plans[0].Changes, pipelinePackages, "the run commits one file per package")
			reportPeakRSS(b)
		})

		b.Run("a cold run of the pipeline over 200,000 declarations on four workers", func(b *testing.B) {
			resetPeakRSS()
			report := benchRun(b, pipelineWorkspace(b, pipelinePackages, fourWorkers),
				pipelineCorpus(pipelinePackages), emptyPools, parallelPipelineAllocs)
			assert.Length(b, report.Plans[0].Changes, pipelinePackages, "the run commits one file per package")
			reportPeakRSS(b)
		})

		b.Run("a cold run over the tree of 200,000 declarations into a ledger", func(b *testing.B) {
			resetPeakRSS()
			report := benchTree(b, recordedEdges(b, pipelinePackages), pipelineTree(pipelinePackages), treeRunAllocs)
			assert.True(b, report.Stats.Generation, "the run writes a generation")
			b.ReportMetric(float64(report.Stats.Size)/float64(pipelinePackages*pipelineStructs), "state-B/decl")
			reportPeakRSS(b)
		})

		b.Run("a warm run over the tree of 200,000 declarations after one edit", func(b *testing.B) {
			resetPeakRSS()
			report := benchWarm(b, pipelinePackages, warmTreeRunAllocs)
			assert.False(b, report.Stats.Cold, "the run reads the sealed state")
			b.ReportMetric(float64(invoked(report, measurerID)), "annotator-invocations")
			reportPeakRSS(b)
		})
	})
}

// rawMeta returns a positioned raw instance of the kernel meta
// directive dropping ref.
func rawMeta(ref string, line int) directive.Raw {
	return directive.Raw{
		Name: "meta",
		Args: []directive.RawArg{{Key: "drop", Value: directive.RawValue{Text: ref}}},
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// rawBareMeta returns a positioned meta instance without a drop:
// the schema admits one, and the drop pass has to pass over it.
func rawBareMeta(line int) directive.Raw {
	return directive.Raw{
		Name: "meta",
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// rawDiag returns a positioned kernel diag instance, which is a
// validated directive the drop pass is not about.
func rawDiag(code string, line int) directive.Raw {
	return directive.Raw{
		Name: "diag",
		Args: []directive.RawArg{{Key: "off", Value: directive.RawValue{Text: code}}},
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// rawRef returns a positioned raw instance of the fixture ref
// directive pointing at name.
func rawRef(name string, line int) directive.Raw {
	return directive.Raw{
		Name: "refy:ref",
		Args: []directive.RawArg{{Key: "to", Value: directive.RawValue{Text: name}}},
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// generatorAt returns a facade-built generator placed by priority,
// running h once per struct in scope: two of them in one plan run
// in the order their priorities fix.
func generatorAt(
	name plugin.ID, pri int, h func(*eidos.StructMatch, *eidos.Emitter) error,
) plugin.Generator {
	p, held := eidos.NewPlugin(name).
		Priority(plugin.RoleGenerator, pri).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(h)).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// flagged returns a composition whose annotator registers and
// stamps shape.flag through its own key provider, and whose
// generator mirrors the subjects the flag reads present on. The
// key handle arrives when Build runs the steps. The handlers read
// it through their closures, which is the seam under test.
func flagged() (*workspace.Builder, *meta.Key[bool]) {
	var flag meta.Key[bool]
	shape, _ := eidos.NewPlugin("shape").
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("shape"); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{
				Name: "shape.flag", Group: flagGroup, Doc: "marks a fixture subject",
			})
			flag = k
			return err
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			eidos.Stamp(st, flag, true)
			return nil
		})).Build().(plugin.Annotator)
	flagMirror, _ := eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if _, held := eidos.Fact(m, flag); !held {
				return nil
			}
			return mirrored(m, e)
		})).Build().(plugin.Generator)
	b := workspace.New().
		Brand(fixtureBrand).
		Annotators(shape).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{flagMirror},
			Backend:    fakeBackend{name: "printer", target: "fixture"},
		})
	return b, &flag
}

// keyedRun builds the flagged composition and runs it over the
// one-struct fixture after attach adds its directives.
func keyedRun(
	t *testing.T, attach func(*store.Graph, symbol.Identity),
) (*workspace.Report, symbol.Identity, meta.Key[bool], error) {
	t.Helper()

	b, flag := flagged()
	w, err := b.Build()
	assert.NoError(t, err, "the keyed composition composes")
	g, s := alpha(t)
	if attach != nil {
		attach(g, s.Identity())
	}
	report, err := w.Run(t.Context(), workspace.Input{Graph: g})
	return report, s.Identity(), *flag, err
}

// optingOut returns a composition whose mirror generator declares
// the stub directive, negatable or not, and mirrors every struct.
func optingOut(negatable bool) *workspace.Builder {
	mirrorer, _ := eidos.NewPlugin(negatingPlugin).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(
			eidos.OnStruct(mirrored),
			eidos.Directive(directive.Schema{
				Plugin: string(negatingPlugin), Name: "stub", Negatable: negatable,
				Doc: "a fixture directive a subject negates to opt out",
			}, eidos.OnEmit(symbol.KindStruct,
				func(*eidos.EmitMatch, *eidos.Emitter) error { return nil })),
		).Build().(plugin.Generator)
	return workspace.New().
		Brand(fixtureBrand).
		Targets("fixture").
		Plans(planTo("plan", "fixture", mirrorer))
}

// pair returns an unfrozen one-package graph with two positioned
// structs, the second negating the mirror's stub directive.
func pair(tb assert.TB) (*store.Graph, *node.Struct, *node.Struct) {
	tb.Helper()

	first := coretest.Struct(coretest.StorePath, "Alpha")
	first.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
	second := coretest.Struct(coretest.StorePath, "Beta")
	second.Pos = position.Pos{File: "beta.go", Line: 7, Col: 1}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, first, second)),
		"the fixture package is admitted")
	assert.NoError(tb, g.AttachDirectives(second.Identity(), []directive.Raw{{
		Name: optOutName, Negated: true, Pos: position.Pos{File: "beta.go", Line: 6, Col: 1},
	}}), "the negated instance attaches before the seal")
	return g, first, second
}

// crowded returns an unfrozen graph declaring n structs in one file of
// the store package's own directory, and that many raw instances of the
// weaver's audit directive on the first struct.
func crowded(tb assert.TB, n, instances int) *store.Graph {
	tb.Helper()

	file := coretest.StorePath + "/crowd.go"
	decls := make([]symbol.Symbol, n)
	structs := make([]*node.Struct, n)
	for i := range structs {
		s := coretest.Struct(coretest.StorePath, fmt.Sprintf("S%02d", i))
		s.Pos = position.Pos{File: file, Line: i + 1, Col: 1}
		structs[i], decls[i] = s, s
	}
	p := coretest.Package(coretest.StorePath, decls...)
	p.Files[0].Path = file
	g := store.New()
	assert.NoError(tb, g.AddPackage(p), "the fixture package is admitted")
	raws := make([]directive.Raw, instances)
	for i := range raws {
		raws[i] = directive.Raw{Name: auditSchema.Canonical(), Pos: position.Pos{File: file, Line: i + 1, Col: 1}}
	}
	if instances > 0 {
		assert.NoError(tb, g.AttachDirectives(structs[0].Identity(), raws), "the instances attach before the seal")
	}
	return g
}

// providing returns the mirror generator under the capability the
// weaver requires.
func providing() plugin.Generator {
	p, held := eidos.NewPlugin("mirror").
		Provides(mirroredCapability).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(mirrored)).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// weaverOf returns the weaver: after the mirror, each instance of its
// audit directive on a subject appends one field into the subject's
// mirrored struct through the Emitter.
func weaverOf() plugin.Generator {
	p, held := eidos.NewPlugin("weaver").
		Requires(mirroredCapability).
		Handle(eidos.Directive(auditSchema,
			eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
				s, held := m.Value.(*emit.Struct)
				if !held {
					return nil
				}
				e.Slot(&s.Fields).Append(&emit.Field{
					Name: fmt.Sprintf("audit%02d", m.Directive().Instance),
					Type: &emit.TypeRef{Spelling: "int"},
				})
				return nil
			}))).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// listing is a kit backend whose struct template spells the name of
// each field, so the order of a struct's field slot shows in the
// rendered bytes.
func listing(tb assert.TB) plugin.Backend {
	tb.Helper()

	return backend.New("lister", "fixture", plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "type {{.Name}} struct { {{range .Fields.Items}}{{.Name}} {{end}}}\n",
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the fixture spells no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: totalCoverage()}).
		Build()
}

// parallelFiles runs one plan of the given generators toward the
// listing backend on the given workers over the graph, and returns the
// files the run committed, by path.
func parallelFiles(t *testing.T, workers int, g *store.Graph, gens ...plugin.Generator) map[string]string {
	t.Helper()

	var o opener
	w := built(t, workspace.New().
		Brand(fixtureBrand).
		Targets("fixture").
		Parallel(workers).
		Plans(workspace.Plan{Name: "plan", Generators: gens, Backend: listing(t)}).
		Output(o.open))
	cleanRun(t, w, g)
	out := map[string]string{}
	for _, sink := range o.opened() {
		for path, body := range sink.Files() {
			out[path] = string(body)
		}
	}
	return out
}

// pipelinePath returns the path of the corpus package with index p.
func pipelinePath(p int) string { return pipelinePathPrefix + strconv.Itoa(p) }

// pipelineCorpus returns the first n packages of the pipeline corpus.
// Each package declares its structs in one file inside the package's
// own directory, so the package's generated file routes beside it.
func pipelineCorpus(n int) []*node.Package {
	pkgs := make([]*node.Package, n)
	for p := range pkgs {
		path := pipelinePath(p)
		decls := make([]symbol.Symbol, 0, pipelineStructs)
		for i := range pipelineRefs {
			decls = append(decls, coretest.Struct(path, pipelineRefPrefix+strconv.Itoa(i)))
		}
		for i := range pipelineMarked {
			decls = append(decls, coretest.Struct(path, pipelineMarkPrefix+strconv.Itoa(i)))
		}
		for i := range pipelineUnmarked {
			decls = append(decls, coretest.Struct(path, pipelinePlainPrefix+strconv.Itoa(i)))
		}
		pkg := coretest.Package(path, decls...)
		pkg.Files[0].Path = path + "/" + coretest.UnitFile
		pkgs[p] = pkg
	}
	return pkgs
}

// pipelineWorkspace composes the pipeline over a corpus of n packages
// on the given workers: an annotator that stamps the mark, a generator
// that mirrors the marked structs with the corpus's share of
// cross-package references, the pipeline backend, and an output that
// opens a fresh memory sink for each run.
func pipelineWorkspace(tb assert.TB, n, workers int) *workspace.Workspace {
	tb.Helper()

	var mark meta.Key[bool]
	marker, held := eidos.NewPlugin("marker").
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("e2e"); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{
				Name: "e2e.mark", Doc: "marks a corpus subject for mirroring",
			})
			mark = k
			return err
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			if strings.HasPrefix(m.Struct.Name, pipelineMarkedPrefix) {
				eidos.Stamp(st, mark, true)
			}
			return nil
		})).Build().(plugin.Annotator)
	assert.True(tb, held, "the annotator half composes")

	mirrorer, held := eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if _, marked := eidos.Fact(m, mark); !marked {
				return nil
			}
			out := &emit.Struct{
				Origin: m.Struct.Identity(),
				Name:   "for" + m.Struct.Name,
			}
			if strings.HasPrefix(m.Struct.Name, pipelineRefPrefix) {
				// The next package's first plain mark, referenced
				// resolved, so the settle follows the name across
				// packages once the respell moves it.
				p := packageIndex(tb, m.Struct.Identity().Package)
				target := coretest.Struct(pipelinePath((p+1)%n), "mk0")
				out.Fields.Append(&emit.Field{
					Name: "peer",
					Type: &emit.TypeRef{Target: target.ID, Spelling: "formk0"},
				})
			}
			e.PackageFile().Append(out)
			return nil
		})).Build().(plugin.Generator)
	assert.True(tb, held, "the generator half composes")

	w, err := workspace.New().
		Brand(pipelineBrand).
		Annotators(marker).
		Targets("fixture").
		Parallel(workers).
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirrorer},
			Backend:    pipelineBackend(),
		}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Build()
	assert.NoError(tb, err, "the composition validates")
	return w
}

// packageIndex returns the corpus index at the end of a package path.
func packageIndex(tb assert.TB, path string) int {
	tb.Helper()

	p, err := strconv.Atoi(strings.TrimPrefix(path, pipelinePathPrefix))
	assert.NoError(tb, err, "a corpus path ends in its index")
	return p
}

// pipelineBackend builds the pipeline's backend: one struct spelling,
// an upper-first respell so the settle rewrites every name and follows
// every reference, and a coverage that renders every fact so the guard
// walks each declaration.
func pipelineBackend() plugin.Backend {
	return backend.New("printer", "fixture",
		plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "type {{.Name}} {\n" +
				"{{- range .Fields.Items}}\n\t{{.Name}} {{spell .Type}}\n{{- end}}\n}\n",
		}).
		Funcs(func(*render.ImportSet) template.FuncMap {
			return template.FuncMap{
				"spell": func(t *emit.TypeRef) string {
					if t == nil {
						return ""
					}
					return t.Spelling
				},
			}
		}).
		Coverage(render.Coverage{Facts: totalCoverage()}).
		Respell(func(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
			return strings.ToUpper(name[:1]) + name[1:], nil
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return []byte("\tnoop()\n"), nil
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Build()
}

// pipelineHashes runs w over a fresh graph of the tests' share of the
// corpus, and returns the hashes of the files the run committed, by
// path.
func pipelineHashes(t *testing.T, w *workspace.Workspace) map[string]string {
	t.Helper()

	report := cleanRun(t, w, loaded(t, pipelineCorpus(pipelineTestPackages)))
	out := map[string]string{}
	for _, c := range report.Plans[0].Changes {
		out[c.Path] = c.Hash
	}
	return out
}

// pipelineTree returns the first n packages of the pipeline corpus
// spelled in the scripted language: per package, one file in the
// package's own directory that declares each struct with two fields.
func pipelineTree(n int) fstest.MapFS {
	tree := fstest.MapFS{}
	for p, pkg := range pipelineCorpus(n) {
		var b strings.Builder
		b.WriteString("package " + pipelinePath(p) + "\n")
		for _, d := range pkg.Files[0].Decls {
			b.WriteString(corpusLine(d.(*node.Struct).Name))
		}
		tree[pipelinePath(p)+"/"+pipelineScript] = &fstest.MapFile{Data: []byte(b.String())}
	}
	return tree
}

// corpusLine returns the scripted line that declares a corpus struct
// with its two fields.
func corpusLine(name string) string { return "type " + name + " int string\n" }

// widenedLine returns the scripted line that declares a corpus struct
// with a third field.
func widenedLine(name string) string { return "type " + name + " int string bool\n" }

// corpusEdit returns an edit that replaces the line from of the corpus
// package p's file under root with to, the way a person edits a source
// file. The edit returns the error of a file that does not read or
// write, and an error for a file without the line.
func corpusEdit(p int, from, to string) func(root string) error {
	return func(root string) error {
		path := filepath.Join(root, filepath.FromSlash(pipelinePath(p)+"/"+pipelineScript))
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(b), from) {
			return fmt.Errorf("workspace_test: %s declares no line %q", path, from)
		}
		return os.WriteFile(path, []byte(strings.Replace(string(b), from, to, 1)), 0o644)
	}
}

// edgeFixture returns the warm check's fixture over the tests' share of
// the pipeline corpus's tree: the measuring annotator, a readers plan
// that mirrors every marked struct and documents what the referencing
// structs read through each edge kind, a stubs plan that stubs every
// tenth plain struct, and the edit. Every plan and every composition
// reads the keys the fixture's one corpusKeys records.
func edgeFixture(edit func(root string) error) workspacetest.Fixture {
	keys := &corpusKeys{}
	return workspacetest.Fixture{
		Tree: pipelineTree(pipelineTestPackages),
		Compose: func(root string) *workspace.Builder {
			return edgeComposition(keys).
				Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
				Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) })
		},
		Plans: func() []workspace.Plan { return edgePlans(keys, pipelineTestPackages) },
		Edit:  edit,
	}
}

// edgeComposition returns the edge corpus's composition without its plans,
// its output and its ledger: the scripted frontend and the measuring
// annotator, which registers the corpus's keys into keys.
func edgeComposition(keys *corpusKeys) *workspace.Builder {
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Annotators(measuring(keys)).
		Targets("fixture")
}

// edgePlans returns the edge corpus's plans over a corpus of n packages:
// the readers plan, which mirrors every marked struct and documents what
// the referencing structs read, and the stubs plan.
func edgePlans(keys *corpusKeys, n int) []workspace.Plan {
	return []workspace.Plan{
		{
			Name:       "readers",
			Generators: []plugin.Generator{reading(keys, n)},
			Backend:    documenting("readers-printer"),
		},
		{
			Name:       "stubs",
			Generators: []plugin.Generator{stubbing()},
			Backend:    documenting("stubs-printer"),
		},
	}
}

// recordedEdges returns the edge corpus's composition over n packages,
// which writes into memory and records each run into a fresh memory
// ledger, so every run is cold and records its whole state.
func recordedEdges(tb assert.TB, n int) *workspace.Workspace {
	tb.Helper()

	keys := &corpusKeys{}
	return built(tb, edgeComposition(keys).
		Plans(edgePlans(keys, n)...).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return ledger.NewMem(), nil }))
}

// measuring returns the annotator that registers the corpus's keys into
// keys, and stamps the mark and the width on every struct whose name
// opens with the marked prefix.
func measuring(keys *corpusKeys) plugin.Annotator {
	p, held := eidos.NewPlugin(measurerID).
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("e2e"); err != nil {
				return err
			}
			mark, err := meta.Register[bool](r, meta.KeySpec{
				Name: "e2e.mark", Doc: "marks a corpus subject for mirroring",
			})
			if err != nil {
				return err
			}
			width, err := meta.Register[int64](r, meta.KeySpec{
				Name: "e2e.width", Doc: "counts the fields of a marked corpus subject",
			})
			keys.mark, keys.width = mark, width
			return err
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			if strings.HasPrefix(m.Struct.Name, pipelineMarkedPrefix) {
				eidos.Stamp(st, keys.mark, true)
				eidos.Stamp(st, keys.width, int64(len(m.Struct.Fields)))
			}
			return nil
		})).Build().(plugin.Annotator)
	if !held {
		panic("workspace_test: a stamper rule lowers to the annotator role")
	}
	return p
}

// reading returns the readers plan's generator over a corpus of n
// packages: per marked struct, a mirror in the gen family, documented
// with what the struct reads of another package through its edge kind.
func reading(keys *corpusKeys, n int) plugin.Generator {
	return generator("reader", func(m *eidos.StructMatch, e *eidos.Emitter) error {
		if _, marked := eidos.Fact(m, keys.mark); !marked {
			return nil
		}
		p, err := strconv.Atoi(strings.TrimPrefix(m.Struct.Identity().Package, pipelinePathPrefix))
		if err != nil {
			return err
		}
		e.PackageFile().Append(&emit.Struct{
			Origin: m.Struct.Identity(),
			Name:   "for" + m.Struct.Name,
			Doc:    edgeRead(m, keys, p, n),
		})
		return nil
	})
}

// edgeRead returns what a referencing struct of the package with index
// p reads of another package of a corpus of n packages through its edge
// kind, as a line of documentation, and nothing for any other struct.
func edgeRead(m *eidos.StructMatch, keys *corpusKeys, p, n int) []string {
	other := func(q int, name string) symbol.Identity {
		id := m.Struct.Identity()
		id.Package, id.Name = pipelinePath(q%n), name
		return id
	}
	switch {
	case m.Struct.Name == edgeLookup:
		s, _ := m.Reader().Lookup(other(p+1, edgeLookedUp))
		return []string{fmt.Sprintf(lookupDoc, edgeLookedUp, fieldCount(s))}
	case m.Struct.Name == edgeFact:
		w, _ := eidos.FactOf(m, other(p+1, edgeMeasured), keys.width)
		return []string{fmt.Sprintf(factDoc, edgeMeasured, w)}
	case m.Struct.Name == edgeMembership && p == 0:
		count := 0
		for range m.Reader().ByKind(symbol.KindStruct) {
			count++
		}
		return []string{fmt.Sprintf(membershipDoc, count)}
	case m.Struct.Name == edgePackage:
		sum := 0
		if pkg, found := m.Reader().PackageOf(other(p+2, edgeLookedUp)); found {
			for _, f := range pkg.Files {
				for _, d := range f.Decls {
					sum += fieldCount(d)
				}
			}
		}
		return []string{fmt.Sprintf(packageDoc, sum)}
	}
	return nil
}

// fieldCount returns the number of a struct's fields, and zero for any
// other declaration and for none.
func fieldCount(s symbol.Symbol) int {
	if st, is := s.(*node.Struct); is {
		return len(st.Fields)
	}
	return 0
}

// stubbing returns the stubs plan's generator: per plain struct whose
// name ends in edgeStubbed, a stub in the stub family.
func stubbing() plugin.Generator {
	p, held := eidos.NewPlugin("stubber").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "stub"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			name := m.Struct.Name
			if strings.HasPrefix(name, pipelinePlainPrefix) && strings.HasSuffix(name, edgeStubbed) {
				e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: name + "Stub"})
			}
			return nil
		})).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// documenting is a kit backend under a name of its own that spells a
// struct as one line behind its documentation, so what a generator
// documents shows in the rendered bytes. It names every file after its
// family word.
func documenting(name plugin.ID) plugin.Backend {
	return backend.New(name, "fixture", plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "{{range .Doc}}// {{.}}\n{{end}}type {{.Name}} struct{}\n",
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the fixture spells no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: totalCoverage()}).
		Build()
}

// benchRun measures w's runs over a fresh graph of pkgs against a
// ceiling of allocs per run, and returns the last run's report. The
// warm-up run builds what a process builds once. Each iteration loads
// its graph and calls reset outside the measurement.
func benchRun(
	b *testing.B, w *workspace.Workspace, pkgs []*node.Package, reset func(), allocs uint64,
) *workspace.Report {
	b.Helper()

	c := bench.Start(b).Warmup(1).MaxAllocs(allocs)
	defer c.End()
	var (
		report *workspace.Report
		err    error
	)
	for c.Loop() {
		var g *store.Graph
		c.Excluding(func() {
			g = loaded(b, pkgs)
			reset()
		})
		report, err = w.Run(b.Context(), workspace.Input{Graph: g})
	}
	assert.NoError(b, err, "the run is clean")
	assert.Length(b, report.Emits, 1, "the plan's store is in the report")
	return report
}

// benchTree measures w's cold runs over a tree against a ceiling of
// allocs per run, and returns the last run's report. The warm-up run
// builds what a process builds once, and each run records into the fresh
// ledger w's composition opens. Each iteration empties the process's
// pools outside the measurement.
func benchTree(b *testing.B, w *workspace.Workspace, tree fs.FS, allocs uint64) *workspace.Report {
	b.Helper()

	c := bench.Start(b).Warmup(1).MaxAllocs(allocs)
	defer c.End()
	var (
		report *workspace.Report
		err    error
	)
	for c.Loop() {
		c.Excluding(emptyPools)
		report, err = w.Run(b.Context(), workspace.Input{Tree: tree})
	}
	assert.NoError(b, err, "the run is clean")
	return report
}

// benchWarm measures the warm runs of the edge corpus's composition over
// the tree of n packages against a ceiling of allocs per run, and returns
// the report of the last run. A cold run before the measurement writes
// the first generation into the memory ledger that every run records
// into. The runs alternate between the tree with one widened struct and
// the tree as it was, so each run reads the generation of the run before
// it and finds one changed file. Each iteration empties the process's
// pools outside the measurement, as a new process starts with them empty.
func benchWarm(b *testing.B, n int, allocs uint64) *workspace.Report {
	b.Helper()

	mem := ledger.NewMem()
	keys := &corpusKeys{}
	w := built(b, edgeComposition(keys).
		Plans(edgePlans(keys, n)...).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return mem, nil }))
	path := pipelinePath(edgeEdited) + "/" + pipelineScript
	trees := [2]fstest.MapFS{pipelineTree(n), pipelineTree(n)}
	widened := strings.Replace(string(trees[1][path].Data), corpusLine(edgeMeasured), widenedLine(edgeMeasured), 1)
	trees[1][path] = &fstest.MapFile{Data: []byte(widened), ModTime: warmEdit}
	_, err := w.Run(b.Context(), workspace.Input{Tree: trees[0]})
	assert.NoError(b, err, "the cold run is clean")
	c := bench.Start(b).Warmup(1).MaxAllocs(allocs)
	defer c.End()
	var report *workspace.Report
	for i := 1; c.Loop(); i++ {
		c.Excluding(emptyPools)
		report, err = w.Run(b.Context(), workspace.Input{Tree: trees[i%2]})
	}
	assert.NoError(b, err, "the warm runs are clean")
	return report
}

// loaded returns an unsealed graph that admits every package of pkgs.
func loaded(tb assert.TB, pkgs []*node.Package) *store.Graph {
	tb.Helper()

	g := store.New()
	assert.Total(tb, g.AddPackage, pkgs, "the fixture package is admitted")
	return g
}

// emptyPools empties every sync.Pool of the process. The first
// collection moves each pool's values to the pool's victim cache, and
// the second drops the victims.
func emptyPools() {
	runtime.GC()
	runtime.GC()
}

// resetPeakRSS sets the process's peak resident set to its current
// resident set, which Linux does when 5 is written to
// /proc/self/clear_refs. Where the file does not exist, it does
// nothing.
func resetPeakRSS() {
	f, err := os.OpenFile("/proc/self/clear_refs", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString("5")
}

// reportPeakRSS reports the process's peak resident set in megabytes as
// the peak-RSS-MB metric, and nothing where the platform does not
// expose it.
func reportPeakRSS(b *testing.B) {
	b.Helper()

	if rss := peakRSS(); rss > 0 {
		b.ReportMetric(float64(rss)/(1<<20), "peak-RSS-MB")
	}
}

// peakRSS returns the process's peak resident set in bytes, which Linux
// reports as VmHWM in /proc/self/status. It returns zero where the file
// does not exist or does not contain the value.
func peakRSS() uint64 {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		if !strings.HasPrefix(line, "VmHWM:") {
			continue
		}
		kb, err := strconv.ParseUint(strings.TrimSuffix(
			strings.TrimSpace(strings.TrimPrefix(line, "VmHWM:")), " kB",
		), 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}
	return 0
}
