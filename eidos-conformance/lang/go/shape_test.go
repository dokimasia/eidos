// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"io/fs"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	langgo "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The shape fixture's trees, and the package that its source declares.
const (
	shapeTree    = "testdata/shape/tree"
	shapeGoRoot  = "testdata/shape/goroot"
	shapePackage = "example.com/acme/store"
)

// The types of the shape fixture whose methods the cases read.
const (
	shapesType    = "Shapes"
	txType        = "Tx"
	overridesType = "Overrides"
)

// The plan of the consumers of the cases and its directory, the names of
// the consumers and the word of their family, and the separators that the
// consumer of the whole vocabulary writes between classifications.
const (
	consumerPlan                = "consumers"
	consumerDir                 = "gen"
	writersID         plugin.ID = "writers"
	vocabularyID      plugin.ID = "vocabulary"
	consumerWord                = "consumer"
	classificationSep           = " "
	roleSep                     = "="
)

// The module file of a tree that a case builds, the path of its one
// source file, and the paths of the two source files of a case that
// edits one of them between two runs.
const (
	caseModule   = "module example.com/acme\n\ngo 1.27\n"
	caseFile     = "store/store.go"
	txFile       = "store/tx.go"
	rollbackFile = "store/rollback.go"
)

// shapeEdit is the modification time of the file that a warm case edits.
// Every other file of the case keeps the zero time.
var shapeEdit = time.Date(2026, time.October, 9, 12, 0, 0, 0, time.UTC)

// The catalog classifies the Go source of the fixture: each detector
// claims its method, a directive of each form stamps its classification,
// a shape directive keeps the detectors off its method, a meta drop
// removes a detected shape, and the catalog's checks refuse what the
// directives' validation does not.
func TestComposeShape(t *testing.T) {
	t.Parallel()

	t.Run("ComposeShape", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the detected shape that ranks first on each method", func(t *testing.T) {
			t.Parallel()

			facts := classified(t)
			tests := []struct {
				method string
				want   shape.Shape
			}{
				{"Count", shape.Aggregator},
				{"Store", shape.AnsweringWriter},
				{"GetAll", shape.BatchReader},
				{"Close", shape.Closer},
				{"Set", shape.CompositeWriter},
				{"Add", shape.Computation},
				{"Delete", shape.Deleter},
				{"Start", shape.Lifecycle},
				{"Lookup", shape.Lookup},
				{"Pair", shape.MultiAggregator},
				{"Record", shape.MultiArgWriter},
				{"GetWithMeta", shape.MultiReader},
				{"Mutate", shape.Mutator},
				{"Ref", shape.PointerReader},
				{"Err", shape.PoisonAccessor},
				{"Ready", shape.Predicate},
				{"Get", shape.Reader},
				{"Peek", shape.ReaderNoError},
				{"Find", shape.ReaderWithBool},
				{"Consume", shape.StreamConsumer},
				{"All", shape.StreamReader},
				{"Reset", shape.VoidLifecycle},
				{"Save", shape.Writer},
			}
			for _, tt := range tests {
				got, _ := meta.Get(facts, method(shapesType, tt.method), meta.Named[string](shape.KeyShape))
				expect.Equal(t, shape.Shape(got), tt.want, "the shape of "+tt.method)
			}
		})

		t.Run("lists the shape that loses by precedence after the one that ranks first", func(t *testing.T) {
			t.Parallel()

			got, _ := meta.Get(classified(t), method(shapesType, "Delete"), meta.Named[[]string](shape.KeyDetected))
			assert.Equal(t, got, []string{string(shape.Deleter), string(shape.Writer)},
				"the deleter ranks before the writer that it also is")
		})

		t.Run("explains the detected shapes of a method with the shape that loses by precedence", func(t *testing.T) {
			t.Parallel()

			state := ledger.NewMem()
			ws, err := golang.ComposeShape().
				Plans(workspace.Plan{
					Name:       consumerPlan,
					Generators: []plugin.Generator{writers()},
					Backend:    gobackend.New(),
					Layout:     layout.Config{Dir: consumerDir},
				}).
				Output(func() (output.Sink, error) { return output.NewMem(), nil }).
				Ledger(func() (ledger.Ledger, error) { return state, nil }).
				Build()
			assert.NoError(t, err, "the shape composition builds with a plan, an output and a ledger")
			_, err = ws.Run(t.Context(), workspace.Input{
				Tree:   os.DirFS(shapeTree),
				Stores: map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(shapeGoRoot)},
			})
			assert.NoError(t, err, "the run succeeds")
			got, err := ws.Explain(t.Context(), workspace.Target{Identity: method(shapesType, "Delete")})
			assert.NoError(t, err, "the generation explains Delete")
			at := slices.IndexFunc(
				got.Claims,
				func(c workspace.ExplainedClaim) bool { return c.Key == shape.KeyDetected },
			)
			assert.NotEqual(t, at, -1, "the explanation has the claim on the detected shapes")
			expect.Equal(t, got.Claims[at].Value, any([]string{string(shape.Deleter), string(shape.Writer)}),
				"the claim lists the deleter before the writer that loses")
			expect.True(t, got.Claims[at].Winner, "the claim of the plugin shape ranks first")
		})

		t.Run("stamps the parameter of a binding", func(t *testing.T) {
			t.Parallel()

			s, _ := shape.SpecOf(shape.Writer)
			got, _ := meta.Get(
				classified(t),
				method(shapesType, "Save"),
				meta.Named[symbol.Identity](s.Bindings[0].Fact),
			)
			assert.Equal(t, got.Name, "v", "the value of Save is its parameter v")
		})

		t.Run("stamps the declared shape of a method and keeps the detectors off it", func(t *testing.T) {
			t.Parallel()

			facts := classified(t)
			remove := method(overridesType, "Remove")
			got, _ := meta.Get(facts, remove, meta.Named[string](shape.KeyShape))
			expect.Equal(t, shape.Shape(got), shape.Writer, "the directive declares the writer")
			_, detected := meta.Get(facts, remove, meta.Named[[]string](shape.KeyDetected))
			expect.False(t, detected, "no detector ran on the method")
		})

		t.Run("removes the facts of a dropped shape", func(t *testing.T) {
			t.Parallel()

			s, _ := shape.SpecOf(shape.Writer)
			_, held := meta.Get(classified(t), method(overridesType, "Put"), meta.Named[bool](s.Key))
			assert.False(t, held, "the meta drop removed the writer")
		})

		t.Run("stamps every form of classification on one method", func(t *testing.T) {
			t.Parallel()

			facts := classified(t)
			commit := method(txType, "Commit")
			writer, _ := shape.SpecOf(shape.Writer)
			atomic, _ := shape.SpecOf(shape.Atomic)
			tx, _ := shape.SpecOf(shape.Tx)
			_, isWriter := meta.Get(facts, commit, meta.Named[bool](writer.Key))
			expect.True(t, isWriter, "Commit is a writer")
			_, isAtomic := meta.Get(facts, commit, meta.Named[bool](atomic.Key))
			expect.True(t, isAtomic, "Commit is atomic")
			role, _ := meta.Get(facts, commit, meta.Named[string](tx.Key))
			expect.Equal(t, shape.Role(role), shape.TxCommit, "Commit has the role commit of tx")
		})

		t.Run("binds a callable reference to a sibling method", func(t *testing.T) {
			t.Parallel()

			s, _ := shape.SpecOf(shape.Writer)
			got, _ := meta.Get(classified(t), method(txType, "Commit"), meta.Named[symbol.Identity](s.Params[0].Fact))
			assert.Equal(t, got, method(txType, "Get"), "reads=Get refers to the method Get of Tx")
		})

		t.Run("runs a generator behind a shape's predicate on the callables of the shape alone", func(t *testing.T) {
			t.Parallel()

			got := consumed(t, writers())
			assert.Permutation(t, slices.Collect(maps.Keys(got)), []string{"ShapesSave", "TxCommit", "OverridesRemove"},
				"the writers of the fixture, the detected, the declared and the overriding one")
			expect.Equal(t, got["ShapesSave"], "v", "Save reads its value binding through WriterOf")
		})

		t.Run("reads every classification of a callable through Of", func(t *testing.T) {
			t.Parallel()

			got := consumed(t, vocabulary())
			want := strings.Join([]string{
				string(shape.Writer), string(shape.Atomic), string(shape.Tx) + roleSep + string(shape.TxCommit),
			}, classificationSep)
			assert.Equal(t, got["TxCommit"], want, "Of reads the shape, the mixin and the contract role of Commit")
		})

		t.Run("reports UnknownVariant with the variants for a misspelled shape", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
//+acme:shape wrtier
func Save(v string) error { return nil }
`)
			assert.Length(t, got, 1, "the misspelled variant is one Error")
			expect.Equal(t, got[0].Code, directive.UnknownVariant, "under UnknownVariant")
			expect.Contains(t, got[0].Msg, string(shape.Writer), "the message lists the variants")
		})

		t.Run("reports RoleArity at each callable of a role with too many callables", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
type Tx struct{}
//+acme:contract tx role=begin
func (t *Tx) Begin() error { return nil }
//+acme:contract tx role=commit
func (t *Tx) Commit() error { return nil }
//+acme:contract tx role=commit
func (t *Tx) Flush() error { return nil }
//+acme:contract tx role=rollback
func (t *Tx) Rollback() error { return nil }
`)
			assert.Length(t, got, 2, "each of the two commits reports")
			for _, d := range got {
				expect.Equal(t, d.Code, catalog.RoleArity, "under RoleArity")
			}
		})

		t.Run("reports RoleArity once for a role without its callable", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
type Tx struct{}
//+acme:contract tx role=begin
func (t *Tx) Begin() error { return nil }
//+acme:contract tx role=commit
func (t *Tx) Commit() error { return nil }
`)
			assert.Length(t, got, 1, "the instance without a rollback reports once")
			expect.Equal(t, got[0].Code, catalog.RoleArity, "under RoleArity")
			expect.Contains(t, got[0].Msg, string(shape.TxRollback), "the message names the role")
		})

		const (
			txSource = `package store
type Tx struct{}
//+acme:contract tx role=begin
func (t *Tx) Begin() error { return nil }
//+acme:contract tx role=commit
func (t *Tx) Commit() error { return nil }
`
			rollbackMember = `package store
//+acme:contract tx role=rollback
func (t *Tx) Rollback() error { return nil }
`
			rollbackPlain = `package store
func (t *Tx) Rollback() error { return nil }
`
		)
		edits := []struct {
			name                  string
			before, after         string
			wantBefore, wantAfter int
		}{
			{
				name:       "reports RoleArity on a warm run after a member in another file loses its directive",
				before:     rollbackMember,
				after:      rollbackPlain,
				wantBefore: 0,
				wantAfter:  1,
			},
			{
				name:       "withdraws RoleArity on a warm run after the missing member in another file gains its directive",
				before:     rollbackPlain,
				after:      rollbackMember,
				wantBefore: 1,
				wantAfter:  0,
			},
		}
		for _, tt := range edits {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				state := ledger.NewMem()
				ws, err := golang.ComposeShape().
					Output(func() (output.Sink, error) { return output.NewMem(), nil }).
					Ledger(func() (ledger.Ledger, error) { return state, nil }).
					Build()
				assert.NoError(t, err, "the shape composition builds with an output and a ledger")
				stores := map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(shapeGoRoot)}
				first, _ := ws.Run(t.Context(), workspace.Input{Tree: fstest.MapFS{
					"go.mod":     {Data: []byte(caseModule)},
					txFile:       {Data: []byte(txSource)},
					rollbackFile: {Data: []byte(tt.before)},
				}, Stores: stores})
				assert.NotNil(t, first, "the first run returns a report")
				assert.Length(t, errorsOf(first), tt.wantBefore, "the first run reports each role without its callable")
				edited := fstest.MapFS{
					"go.mod":     {Data: []byte(caseModule)},
					txFile:       {Data: []byte(txSource)},
					rollbackFile: {Data: []byte(tt.after), ModTime: shapeEdit},
				}
				warm, _ := ws.Run(t.Context(), workspace.Input{Tree: edited, Stores: stores})
				assert.NotNil(t, warm, "the warm run returns a report")
				assert.False(t, warm.Stats.Cold, "the warm run reads the sealed state of the first run")
				fresh, err := golang.ComposeShape().Build()
				assert.NoError(t, err, "the shape composition builds")
				cold, _ := fresh.Run(t.Context(), workspace.Input{Tree: edited, Stores: stores})
				assert.NotNil(t, cold, "the cold run returns a report")
				got := errorsOf(warm)
				assert.Permutation(t, got, errorsOf(cold), "the warm run reports the same Errors as a cold run")
				assert.Length(t, got, tt.wantAfter, "the warm run reports each role without its callable")
				for _, d := range got {
					expect.Equal(t, d.Code, catalog.RoleArity, "under RoleArity")
				}
			})
		}

		t.Run("joins the methods of two types of one package in one instance", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
type DB struct{}
type Tx struct{}
//+acme:contract tx role=begin
func (d *DB) Begin() (*Tx, error) { return nil, nil }
//+acme:contract tx role=commit
func (t *Tx) Commit() error { return nil }
//+acme:contract tx role=rollback
func (t *Tx) Rollback() error { return nil }
`)
			assert.Empty(t, got, "the begin of DB and the commit and the rollback of Tx form one transaction")
		})

		t.Run("separates two instances of a contract in one package by their ids", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
//+acme:contract saga role=step id=charge
func Charge() error { return nil }
//+acme:contract saga role=compensate id=charge
func Refund() error { return nil }
//+acme:contract saga role=step id=ship
func Ship() error { return nil }
//+acme:contract saga role=compensate id=ship
func Unship() error { return nil }
`)
			assert.Empty(t, got, "each pair is an instance of its own")
		})

		t.Run("reports ParamRange for an int below the minimum of its spec", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
//+acme:mixin retry-succeeds attempts=1
func Send() error { return nil }
`)
			assert.Length(t, got, 1, "one attempt is no retry")
			expect.Equal(t, got[0].Code, catalog.ParamRange, "under ParamRange")
		})

		t.Run("reports ExclusiveParams for two params that exclude each other", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
type Entry struct{ Lifetime int }
//+acme:mixin ttl duration=5m lifetime=Lifetime
func Get(id string) (Entry, error) { return Entry{}, nil }
`)
			assert.Length(t, got, 1, "a ttl states one lifetime")
			expect.Equal(t, got[0].Code, catalog.ExclusiveParams, "under ExclusiveParams")
		})

		t.Run("reports UnsharedParam for an axis that the reader does not take", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
//+acme:mixin partition read=Read axis=tenant
func Write(tenant string, v string) error { return nil }
func Read(id string) (string, error) { return "", nil }
`)
			assert.Length(t, got, 1, "the reader takes no tenant")
			expect.Equal(t, got[0].Code, catalog.UnsharedParam, "under UnsharedParam")
		})

		t.Run("reports a mixin without its required param", func(t *testing.T) {
			t.Parallel()

			got := findings(t, `package store
//+acme:mixin not-found
func Get(id string) (string, error) { return "", nil }
`)
			assert.Length(t, got, 1, "not-found requires its sentinel")
			expect.Equal(t, got[0].Code, directive.MissingParam, "under MissingParam")
		})
	})
}

// classified runs the shape composition over the fixture's tree, and
// returns the facts of the run. A run that reports an Error fails the
// test with each Error.
func classified(tb testing.TB) *meta.Facts {
	tb.Helper()

	return run(tb, golang.ComposeShape(), os.DirFS(shapeTree)).Facts
}

// consumed runs the shape composition with one consuming generator over
// the fixture's tree, and returns the constants that the generator
// emits, their values keyed by their names.
func consumed(tb testing.TB, consumer plugin.Generator) map[string]string {
	tb.Helper()

	report := run(tb, golang.ComposeShape().Plans(workspace.Plan{
		Name:       consumerPlan,
		Generators: []plugin.Generator{consumer},
		Backend:    gobackend.New(),
		Layout:     layout.Config{Dir: consumerDir},
	}), os.DirFS(shapeTree))
	out := map[string]string{}
	for sym := range report.Emits[consumerPlan].ByKind(symbol.KindConstant) {
		c, _ := sym.(*emit.Constant)
		value, err := strconv.Unquote(c.Value)
		assert.NoError(tb, err, "the consumer writes a quoted value")
		out[c.Name] = value
	}
	return out
}

// run builds a composition and runs it over a tree with the fixture's
// standard library, and returns the report. A run that reports an Error
// fails the test with each Error.
func run(tb testing.TB, b *workspace.Builder, tree fs.FS) *workspace.Report {
	tb.Helper()

	ws, err := b.Build()
	assert.NoError(tb, err, "the shape composition builds")
	report, err := ws.Run(tb.Context(), workspace.Input{
		Tree:   tree,
		Stores: map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(shapeGoRoot)},
	})
	assert.NotNil(tb, report, "the run returns a report")
	for d := range report.Sink.All() {
		expect.NotEqual(tb, d.Severity, diag.SeverityError,
			"the run reports no Error, and reports "+d.Code.String()+" at "+d.Pos.String()+": "+d.Msg)
	}
	assert.NoError(tb, err, "the run succeeds")
	return report
}

// findings runs the shape composition over a tree of one source file,
// and returns the Errors that the run reports.
func findings(tb testing.TB, src string) []diag.Diag {
	tb.Helper()

	ws, err := golang.ComposeShape().Build()
	assert.NoError(tb, err, "the shape composition builds")
	report, _ := ws.Run(tb.Context(), workspace.Input{
		Tree: fstest.MapFS{
			"go.mod": {Data: []byte(caseModule)},
			caseFile: {Data: []byte(src)},
		},
		Stores: map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(shapeGoRoot)},
	})
	assert.NotNil(tb, report, "the run returns a report")
	return errorsOf(report)
}

// errorsOf returns the Errors of a run's report, in the order of its
// sink.
func errorsOf(report *workspace.Report) []diag.Diag {
	var out []diag.Diag
	for d := range report.Sink.All() {
		if d.Severity == diag.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// writers returns a generator behind the predicate of the shape writer.
// For each writer, it emits one constant named after the method's type
// and the method, whose value is the name of the writer's value binding.
func writers() plugin.Generator {
	g, _ := eidos.NewPlugin(writersID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: consumerWord}).
		Handle(eidos.Where(shape.Is(shape.Writer),
			eidos.OnMethod(func(m *eidos.MethodMatch, e *eidos.Emitter) error {
				w, _ := shape.WriterOf(m)
				e.PlanFile().Append(&emit.Constant{
					Origin: m.Method.ID, Name: m.Method.Host.Name + m.Method.Name, Value: strconv.Quote(w.Value.Name),
				})
				return nil
			}))).
		Build().(plugin.Generator)
	return g
}

// vocabulary returns a generator behind the predicate of every
// classification. For each classified method, it emits one constant named
// after the method's type and the method, whose value lists the method's
// shape, mixins and contract roles, which it reads through Of without
// naming a spec.
func vocabulary() plugin.Generator {
	g, _ := eidos.NewPlugin(vocabularyID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: consumerWord}).
		Handle(eidos.Where(shape.Any(),
			eidos.OnMethod(func(m *eidos.MethodMatch, e *eidos.Emitter) error {
				c := shape.Of(m)
				var parts []string
				if c.Shape != "" {
					parts = append(parts, string(c.Shape))
				}
				for _, mixin := range c.Mixins {
					parts = append(parts, string(mixin))
				}
				for _, r := range c.Contracts {
					parts = append(parts, string(r.Contract)+roleSep+string(r.Role))
				}
				e.PlanFile().Append(&emit.Constant{
					Origin: m.Method.ID, Name: m.Method.Host.Name + m.Method.Name,
					Value: strconv.Quote(strings.Join(parts, classificationSep)),
				})
				return nil
			}))).
		Build().(plugin.Generator)
	return g
}

// method returns the identity of a method of a type of the fixture's
// package.
func method(host, name string) symbol.Identity {
	return symbol.Identity{Lang: langgo.Lang, Package: shapePackage, Owner: host, Name: name, Kind: symbol.KindMethod}
}
