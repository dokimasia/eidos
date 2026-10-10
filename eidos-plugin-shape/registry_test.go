// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape_test

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
)

// The spellings of the key layout of the catalog, and the values that the
// directive of a case writes.
const (
	namespace     = "shape"
	partSep       = "."
	idPart        = "id"
	rolePart      = "role"
	stringValue   = "v"
	intValue      = 7
	readingID     = "i"
	subjectName   = "Subject"
	outsiderName  = "Outsider"
	referenceName = "Target"
)

// reading is the generated reader of one spec. reader is the name of the
// function, read returns its params struct as a value of any type, and has
// calls it without the conversion, so a measurement counts the reader
// alone.
type reading struct {
	name   string
	reader string
	read   func(sdk.Matcher) (any, bool)
	has    func(sdk.Matcher) bool
}

// readerOf returns the reading of the generated reader of the spec of a
// name. It takes the name of the reader from the runtime's record of the
// function.
func readerOf[N ~string, P any](name N, read func(sdk.Matcher) (P, bool)) reading {
	fn := runtime.FuncForPC(reflect.ValueOf(read).Pointer()).Name()
	return reading{
		name:   string(name),
		reader: fn[strings.LastIndex(fn, partSep)+1:],
		read:   func(m sdk.Matcher) (any, bool) { return read(m) },
		has: func(m sdk.Matcher) bool {
			_, held := read(m)
			return held
		},
	}
}

// readings are the generated readers, in the order of the names of the
// specs.
var readings = []reading{
	readerOf(shape.Accumulates, shape.AccumulatesOf),
	readerOf(shape.Aggregator, shape.AggregatorOf),
	readerOf(shape.AnsweringWriter, shape.AnsweringWriterOf),
	readerOf(shape.Appender, shape.AppenderOf),
	readerOf(shape.Associative, shape.AssociativeOf),
	readerOf(shape.Atomic, shape.AtomicOf),
	readerOf(shape.BatchReader, shape.BatchReaderOf),
	readerOf(shape.BatchWriter, shape.BatchWriterOf),
	readerOf(shape.Bounded, shape.BoundedOf),
	readerOf(shape.Cache, shape.CacheOf),
	readerOf(shape.Cacheable, shape.CacheableOf),
	readerOf(shape.Cas, shape.CasOf),
	readerOf(shape.Causal, shape.CausalOf),
	readerOf(shape.Chain, shape.ChainOf),
	readerOf(shape.CircuitBreaker, shape.CircuitBreakerOf),
	readerOf(shape.Closer, shape.CloserOf),
	readerOf(shape.Codec, shape.CodecOf),
	readerOf(shape.Commutative, shape.CommutativeOf),
	readerOf(shape.CompositeWriter, shape.CompositeWriterOf),
	readerOf(shape.Computation, shape.ComputationOf),
	readerOf(shape.Concurrent, shape.ConcurrentOf),
	readerOf(shape.ConcurrentReaders, shape.ConcurrentReadersOf),
	readerOf(shape.Conservative, shape.ConservativeOf),
	readerOf(shape.CrdtMerge, shape.CrdtMergeOf),
	readerOf(shape.Cursor, shape.CursorOf),
	readerOf(shape.DefaultOnError, shape.DefaultOnErrorOf),
	readerOf(shape.DeleteRemoves, shape.DeleteRemovesOf),
	readerOf(shape.Deleter, shape.DeleterOf),
	readerOf(shape.Deprecated, shape.DeprecatedOf),
	readerOf(shape.Errors, shape.ErrorsOf),
	readerOf(shape.Eventually, shape.EventuallyOf),
	readerOf(shape.Hooks, shape.HooksOf),
	readerOf(shape.Idempotent, shape.IdempotentOf),
	readerOf(shape.IfAbsent, shape.IfAbsentOf),
	readerOf(shape.IfMatch, shape.IfMatchOf),
	readerOf(shape.Indexed, shape.IndexedOf),
	readerOf(shape.InjectionSafe, shape.InjectionSafeOf),
	readerOf(shape.IntegrationOnly, shape.IntegrationOnlyOf),
	readerOf(shape.LeaderElection, shape.LeaderElectionOf),
	readerOf(shape.LeakFree, shape.LeakFreeOf),
	readerOf(shape.Lease, shape.LeaseOf),
	readerOf(shape.Lifecycle, shape.LifecycleOf),
	readerOf(shape.LifecycleAfterClose, shape.LifecycleAfterCloseOf),
	readerOf(shape.Lookup, shape.LookupOf),
	readerOf(shape.Monotonic, shape.MonotonicOf),
	readerOf(shape.MonotonicReads, shape.MonotonicReadsOf),
	readerOf(shape.MonotonicWrites, shape.MonotonicWritesOf),
	readerOf(shape.MultiAggregator, shape.MultiAggregatorOf),
	readerOf(shape.MultiArgWriter, shape.MultiArgWriterOf),
	readerOf(shape.MultiReader, shape.MultiReaderOf),
	readerOf(shape.Mutator, shape.MutatorOf),
	readerOf(shape.NilSafe, shape.NilSafeOf),
	readerOf(shape.NoDuplicates, shape.NoDuplicatesOf),
	readerOf(shape.NotFound, shape.NotFoundOf),
	readerOf(shape.OrderAfter, shape.OrderAfterOf),
	readerOf(shape.Outbox, shape.OutboxOf),
	readerOf(shape.OverMatch, shape.OverMatchOf),
	readerOf(shape.Pagination, shape.PaginationOf),
	readerOf(shape.Partition, shape.PartitionOf),
	readerOf(shape.Permutation, shape.PermutationOf),
	readerOf(shape.Persister, shape.PersisterOf),
	readerOf(shape.PointInTime, shape.PointInTimeOf),
	readerOf(shape.PointerReader, shape.PointerReaderOf),
	readerOf(shape.PoisonAccessor, shape.PoisonAccessorOf),
	readerOf(shape.Poisonable, shape.PoisonableOf),
	readerOf(shape.Pool, shape.PoolOf),
	readerOf(shape.Predicate, shape.PredicateOf),
	readerOf(shape.Publisher, shape.PublisherOf),
	readerOf(shape.Pure, shape.PureOf),
	readerOf(shape.RateLimit, shape.RateLimitOf),
	readerOf(shape.ReadAfterWrite, shape.ReadAfterWriteOf),
	readerOf(shape.ReadYourWrites, shape.ReadYourWritesOf),
	readerOf(shape.Reader, shape.ReaderOf),
	readerOf(shape.ReaderNoError, shape.ReaderNoErrorOf),
	readerOf(shape.ReaderWithBool, shape.ReaderWithBoolOf),
	readerOf(shape.RetrySucceeds, shape.RetrySucceedsOf),
	readerOf(shape.Saga, shape.SagaOf),
	readerOf(shape.Sample, shape.SampleOf),
	readerOf(shape.Scheduled, shape.ScheduledOf),
	readerOf(shape.Scope, shape.ScopeOf),
	readerOf(shape.Serializable, shape.SerializableOf),
	readerOf(shape.SideEffect, shape.SideEffectOf),
	readerOf(shape.SingleFlight, shape.SingleFlightOf),
	readerOf(shape.SnapshotIsolation, shape.SnapshotIsolationOf),
	readerOf(shape.StableOrder, shape.StableOrderOf),
	readerOf(shape.Sticky, shape.StickyOf),
	readerOf(shape.StreamConsumer, shape.StreamConsumerOf),
	readerOf(shape.StreamReader, shape.StreamReaderOf),
	readerOf(shape.StreamReflectsMutations, shape.StreamReflectsMutationsOf),
	readerOf(shape.TamperEvident, shape.TamperEvidentOf),
	readerOf(shape.TimeAware, shape.TimeAwareOf),
	readerOf(shape.Timeout, shape.TimeoutOf),
	readerOf(shape.Total, shape.TotalOf),
	readerOf(shape.Transaction, shape.TransactionOf),
	readerOf(shape.TTL, shape.TTLOf),
	readerOf(shape.Tx, shape.TxOf),
	readerOf(shape.Updater, shape.UpdaterOf),
	readerOf(shape.Upserter, shape.UpserterOf),
	readerOf(shape.Validates, shape.ValidatesOf),
	readerOf(shape.VoidLifecycle, shape.VoidLifecycleOf),
	readerOf(shape.Watcher, shape.WatcherOf),
	readerOf(shape.Windowed, shape.WindowedOf),
	readerOf(shape.Workflow, shape.WorkflowOf),
	readerOf(shape.WrappedVia, shape.WrappedViaOf),
	readerOf(shape.Writer, shape.WriterOf),
	readerOf(shape.WritesFollowReads, shape.WritesFollowReadsOf),
	readerOf(shape.XSSSafe, shape.XSSSafeOf),
}

// readFixture is a run that declares one classification on the method
// Subject, with every param of its spec written, and nothing on the
// method Outsider, which no detector reports. Target is the method that
// every reference of the directive resolves to.
type readFixture struct {
	subject, outsider, target *node.Method
	run                       *shapetest.Run
}

// readFixtureOf returns the fixture of one spec, classified. The
// directive writes the first role and an id for a contract, and a value
// of each param by its type.
func readFixtureOf(tb testing.TB, s shape.Spec) readFixture {
	tb.Helper()

	str := &node.TypeRef{Spelling: shapetest.String}
	failure := &node.TypeRef{Spelling: shapetest.Error}
	f := readFixture{
		subject:  shapetest.Method(storeName, subjectName, []*node.TypeRef{str, str, str}, str, str, str, failure),
		outsider: shapetest.Method(storeName, outsiderName, []*node.TypeRef{str, str}, str, str),
		target:   shapetest.Method(storeName, referenceName, nil, str, str),
	}
	f.run = shapetest.New(tb, shapetest.Package(f.subject, f.outsider, f.target))
	in := shapetest.Instance{Params: map[directive.ParamKey]directive.Value{}}
	switch s.Form {
	case shape.FormShape:
		in.Shape = shape.Shape(s.Name)
	case shape.FormMixin:
		in.Mixin = shape.Mixin(s.Name)
	default:
		in.Contract, in.Role = shape.Contract(s.Name), s.Roles[0].Name
		in.Params[catalog.IDParam] = directive.Value{Kind: directive.TypeString, Str: readingID}
	}
	for _, p := range s.Params {
		switch p.Type {
		case directive.TypeInt:
			in.Params[p.Key] = directive.Value{Kind: directive.TypeInt, Int: intValue}
		case directive.TypeReference:
			in.Params[p.Key] = directive.Value{Kind: directive.TypeReference, Ref: referenceName, Target: f.target.ID}
		default:
			in.Params[p.Key] = directive.Value{Kind: directive.TypeString, Str: stringValue}
		}
	}
	f.run.Declare(tb, f.subject.ID, in)
	f.run.Classify(tb)
	return f
}

// want returns the values that the params struct of a spec has on the
// subject, in the order of its fields.
func (f readFixture) want(s shape.Spec) []any {
	var out []any
	if s.Form == shape.FormContract {
		out = append(out, s.Roles[0].Name, readingID)
	}
	for _, b := range s.Bindings {
		if b.From == shape.SourceInput {
			out = append(out, f.subject.Params[b.Index].ID)
		} else {
			out = append(out, f.subject.Returns[b.Index].ID)
		}
	}
	for _, p := range s.Params {
		switch p.Type {
		case directive.TypeInt:
			out = append(out, int64(intValue))
		case directive.TypeReference:
			out = append(out, f.target.ID)
		default:
			out = append(out, stringValue)
		}
	}
	return out
}

// The registry describes every spec under the key layout of the catalog,
// and each generated reader reads back what a directive declares.
func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Specs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the specs in name order", func(t *testing.T) {
			t.Parallel()

			specs := shape.Specs()
			names := make([]string, 0, len(specs))
			for _, s := range specs {
				names = append(names, s.Name)
			}
			assert.Pairwise(
				t,
				names,
				func(a, b string) bool { return a < b },
				"each spec has a name after the name before it",
			)
		})

		t.Run("returns new slices on each call", func(t *testing.T) {
			t.Parallel()

			first := shape.Specs()
			first[0].Name = ghostName
			first[0].Params[0].Doc = ghostName
			second := shape.Specs()
			expect.NotEqual(
				t,
				second[0].Name,
				ghostName,
				"a change of the list of one call leaves another call unchanged",
			)
			expect.NotEqual(t, second[0].Params[0].Doc, ghostName,
				"a change of the params of one call leaves another call unchanged")
		})

		t.Run("gives each spec the keys of the key layout", func(t *testing.T) {
			t.Parallel()

			for _, s := range shape.Specs() {
				group := namespace + partSep + s.Name
				expect.Equal(t, string(s.Group), group, "the group of "+s.Name+" is the namespace and its name")
				for _, p := range s.Params {
					expect.Equal(t, string(p.Fact), group+partSep+string(p.Key),
						"the key of a param of "+s.Name+" is the group and the key of the param")
				}
				for _, b := range s.Bindings {
					expect.Equal(t, string(b.Fact), group+partSep+string(b.Key),
						"the key of a binding of "+s.Name+" is the group and the name of the binding")
				}
				wantKey, wantID := group+partSep+s.Form.String(), ""
				if s.Form == shape.FormContract {
					wantKey, wantID = group+partSep+rolePart, group+partSep+idPart
				}
				expect.Equal(t, string(s.Key), wantKey, "the family key of "+s.Name+" has the part of its form")
				expect.Equal(t, string(s.ID), wantID, "the id key of "+s.Name+" exists for a contract alone")
			}
		})

		t.Run("gives the detected shapes alone a detector and a precedence", func(t *testing.T) {
			t.Parallel()

			for _, s := range shape.Specs() {
				if s.Detected {
					expect.Equal(t, s.Form, shape.FormShape, "the detected spec "+s.Name+" is a shape")
				}
				if len(s.YieldsTo) > 0 {
					expect.True(t, s.Detected, "the spec "+s.Name+" yields to other shapes as a detected shape")
				}
				for _, y := range s.YieldsTo {
					other, _ := shape.SpecOf(y)
					expect.True(t, other.Detected, "the spec "+s.Name+" yields to the detected shape "+string(y))
				}
			}
		})

		t.Run("gives roles to the contracts alone", func(t *testing.T) {
			t.Parallel()

			for _, s := range shape.Specs() {
				expect.Equal(t, len(s.Roles) > 0, s.Form == shape.FormContract,
					"the spec "+s.Name+" has roles exactly where it is a contract")
			}
		})

		t.Run("describes the spec of each generated reader", func(t *testing.T) {
			t.Parallel()

			names := make([]string, 0, len(readings))
			for _, r := range readings {
				names = append(names, r.name)
			}
			specs := make([]string, 0, len(names))
			for _, s := range shape.Specs() {
				specs = append(specs, s.Name)
			}
			assert.Equal(t, names, specs, "the readers of the table are the readers of every spec")
		})
	})

	for _, r := range readings {
		t.Run(r.reader, func(t *testing.T) {
			t.Parallel()

			s, found := shape.SpecOf(r.name)
			assert.True(t, found, "the catalog has the spec "+r.name)
			f := readFixtureOf(t, s)
			var got any
			var held, outsiderHeld bool
			f.run.Consume(t, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
				switch m.Method.ID {
				case f.subject.ID:
					got, held = r.read(m)
				case f.outsider.ID:
					_, outsiderHeld = r.read(m)
				}
				return nil
			}))

			t.Run("returns the values that the directive writes", func(t *testing.T) {
				t.Parallel()

				assert.True(t, held, "the subject has the classification")
				v := reflect.ValueOf(got)
				fields := make([]any, 0, v.NumField())
				for _, field := range v.Fields() {
					fields = append(fields, field.Interface())
				}
				assert.Equal(
					t,
					fields,
					f.want(s),
					"the params struct has the role, the id, the bindings and the params in order",
					assert.EquateEmpty(),
				)
			})

			t.Run("reports false for a callable without the classification", func(t *testing.T) {
				t.Parallel()

				assert.False(t, outsiderHeld, "the outsider has no classification")
			})
		})
	}
}

// A generated reader allocates nothing once the read set of the
// invocation has grown.
func TestRegistryAllocs(t *testing.T) {
	for _, r := range readings {
		t.Run(r.reader, func(t *testing.T) {
			s, _ := shape.SpecOf(r.name)
			f := readFixtureOf(t, s)
			f.run.Consume(t, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
				if m.Method.ID != f.subject.ID {
					return nil
				}
				var held bool
				expect.MaxAllocs(t, func() { held = r.has(m) }, 0, "the reader allocates nothing")
				expect.True(t, held, "the subject has the classification")
				return nil
			}))
		})
	}
}

// BenchmarkRegistry measures each generated reader on a callable with
// every param of its spec.
func BenchmarkRegistry(b *testing.B) {
	for _, r := range readings {
		b.Run(r.reader, func(b *testing.B) {
			s, _ := shape.SpecOf(r.name)
			f := readFixtureOf(b, s)
			f.run.Consume(b, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
				if m.Method.ID != f.subject.ID {
					return nil
				}
				var held bool
				c := bench.Start(b).MaxAllocs(0).Warmup(1)
				defer c.End()
				for c.Loop() {
					held = r.has(m)
				}
				assert.True(b, held, "the subject has the classification")
				return nil
			}))
		})
	}
}
