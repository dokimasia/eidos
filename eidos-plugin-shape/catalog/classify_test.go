// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog_test

import (
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the declarations of the fixtures of the plugin shape, the
// values that their directives write, and the separator of a key.
const (
	deleteName  = "Delete"
	countName   = "Count"
	hashName    = "Hash"
	refName     = "Ref"
	saveName    = "Save"
	pairName    = "Pair"
	pingName    = "Ping"
	retryName   = "Retry"
	expiryName  = "Expiry"
	shardName   = "Shard"
	lookupName  = "Lookup"
	closedName  = "ErrClosed"
	ghostName   = "Ghost"
	firstParam  = "p0"
	secondParam = "p1"
	instanceID  = "a"
	duration    = "1s"
	keySep      = "."
	attempts    = 3
	fewAttempts = 1
)

// The scaled corpus of the benchmark has scaledHosts types, named by
// hostPrefix and a number, so it has 1,150 callables. classifyBudget is
// the allocation budget of one annotate phase over it, about 5% above the
// 24,967 allocations that one phase measured. Per callable, the phase
// allocates the projection of the callable, the bound rules of the
// invocation, the list of the detected shapes and its copy in the fact
// store, the boxed values, and the bag and the key states of the four to
// six keys that it stamps.
const (
	scaledHosts    = 50
	hostPrefix     = "T"
	classifyBudget = 26_000
)

// The references of the fixtures of the plugin shape.
var (
	str      = &node.TypeRef{Spelling: shapetest.String}
	failure  = &node.TypeRef{Spelling: shapetest.Error}
	ctxRef   = &node.TypeRef{Spelling: shapetest.Context}
	closedID = symbol.Identity{
		Lang:    shapetest.Lang,
		Package: shapetest.Path,
		Name:    closedName,
		Kind:    symbol.KindVariable,
	}
	ghostID = symbol.Identity{
		Lang:    shapetest.Lang,
		Package: shapetest.Path,
		Name:    ghostName,
		Kind:    symbol.KindFunction,
	}
)

// stamps are the facts that the plugin shape stamped on one callable. They
// are the detected shapes of the callable, and its classification as
// [shape.Of] reads it.
type stamps struct {
	detected []string
	of       shape.Classification
}

// The plugin shape declares the directives of the catalog, registers its
// keys, and stamps the classifications that the directives declare and
// the detectors report.
func TestClassify(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the summary keys and every key of each spec", func(t *testing.T) {
			t.Parallel()

			r := shapetest.New(t, shapetest.Package())
			var got []string
			for name := range r.Keys.Keys() {
				if strings.HasPrefix(string(name), shape.KeyShape.Namespace()+keySep) {
					got = append(got, string(name))
				}
			}
			want := []string{
				string(shape.KeyDetected), string(shape.KeyShape), string(shape.KeyMixed),
				string(shape.KeyMember), string(shape.KeyClassified),
			}
			for _, s := range shape.Specs() {
				want = append(want, string(s.Key))
				if s.ID != "" {
					want = append(want, string(s.ID))
				}
				for _, p := range s.Params {
					want = append(want, string(p.Fact))
				}
				for _, b := range s.Bindings {
					want = append(want, string(b.Fact))
				}
			}
			assert.Permutation(t, got, want, "the plugin registers each key of the catalog once")
		})

		t.Run("returns the error of a namespace that another registrant claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.ClaimNamespace(shape.KeyShape.Namespace()), "another registrant claims the namespace")
			annotator := catalog.Annotators()[0]
			keys, provides := annotator.(plugin.KeyProvider)
			assert.True(t, provides, "the plugin shape registers keys")
			assert.HasError(t, keys.Keys(r.For(string(annotator.Name()))),
				"the plugin shape cannot register into the namespace of another")
		})
	})

	t.Run("Directives", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a variant of each spec in the directive of its form", func(t *testing.T) {
			t.Parallel()

			provider, provides := catalog.Annotators()[0].(plugin.DirectiveProvider)
			assert.True(t, provides, "the plugin shape declares directives")
			schemas := provider.Directives()
			assert.Length(t, schemas, 3, "the plugin shape declares three directives")
			forms := map[shape.Form]int{}
			for _, s := range shape.Specs() {
				forms[s.Form]++
			}
			tests := []struct {
				schema directive.Schema
				name   directive.Name
				form   shape.Form
			}{
				{schemas[0], catalog.ShapeDirective, shape.FormShape},
				{schemas[1], catalog.MixinDirective, shape.FormMixin},
				{schemas[2], catalog.ContractDirective, shape.FormContract},
			}
			for _, tt := range tests {
				expect.Equal(t, tt.schema.Name, tt.name, "the directive has the name of its form")
				expect.Length(t, tt.schema.Variants, forms[tt.form], "the directive has a variant of each spec")
			}
		})

		t.Run("overrides the detectors with the shape directive and repeats the others", func(t *testing.T) {
			t.Parallel()

			provider, _ := catalog.Annotators()[0].(plugin.DirectiveProvider)
			schemas := provider.Directives()
			assert.Length(t, schemas, 3, "the plugin shape declares three directives")
			expect.True(t, schemas[0].Overrides, "an instance of the shape directive keeps the detectors off")
			expect.True(t, schemas[1].Repeatable, "a callable has any number of mixins")
			expect.True(t, schemas[2].Repeatable, "a callable has a role in any number of contracts")
			expect.Equal(t, schemas[2].Params, []directive.ParamSpec{{
				Key: catalog.IDParam, Type: directive.TypeString,
				Doc: "the id that separates two instances of the contract in one package",
			}}, "the contract directive has the param id")
		})
	})

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the detected shapes in the order of precedence", func(t *testing.T) {
			t.Parallel()

			del := shapetest.Method(storeName, deleteName, []*node.TypeRef{str}, failure)
			r := shapetest.New(t, shapetest.Package(del))
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[deleteName], stamps{
				detected: []string{string(shape.Deleter), string(shape.Writer)},
				of: shape.Classification{
					Shape:  shape.Deleter,
					Params: map[string]shape.Params{string(shape.Deleter): {shape.DeleterKey: del.Params[0].ID}},
				},
			}, "Delete is a deleter before a writer, with the key of its parameter")
		})

		callables := []struct {
			name string
			give func() (node.Declaration, *node.Param)
		}{
			{
				name: "stamps the detected shapes of a function",
				give: func() (node.Declaration, *node.Param) {
					del := shapetest.Function(deleteName, []*node.TypeRef{str}, failure)
					return del, del.Params[0]
				},
			},
			{
				name: "stamps the detected shapes of a method of an interface",
				give: func() (node.Declaration, *node.Param) {
					del := hostedDelete(symbol.KindInterface)
					return &node.Interface{ID: del.Host, Name: storeName, Methods: []*node.Method{del}}, del.Params[0]
				},
			},
			{
				name: "stamps the detected shapes of a method of an enum",
				give: func() (node.Declaration, *node.Param) {
					del := hostedDelete(symbol.KindEnum)
					return &node.Enum{ID: del.Host, Name: storeName, Methods: []*node.Method{del}}, del.Params[0]
				},
			},
			{
				name: "stamps the detected shapes of a method of a sum",
				give: func() (node.Declaration, *node.Param) {
					del := hostedDelete(symbol.KindSum)
					return &node.Sum{ID: del.Host, Name: storeName, Methods: []*node.Method{del}}, del.Params[0]
				},
			},
		}
		for _, tt := range callables {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				decl, key := tt.give()
				r := shapetest.New(t, shapetest.Package(decl))
				got, _ := stampsOf(t, r)
				assert.Equal(t, got[deleteName], stamps{
					detected: []string{string(shape.Deleter), string(shape.Writer)},
					of: shape.Classification{
						Shape:  shape.Deleter,
						Params: map[string]shape.Params{string(shape.Deleter): {shape.DeleterKey: key.ID}},
					},
				}, "Delete is a deleter before a writer, with the key of its parameter")
			})
		}

		ranks := []struct {
			name string
			give *node.Method
			want []string
		}{
			{
				name: "ranks a computation before an aggregator for a callable without a context",
				give: shapetest.Method(storeName, countName, nil, intRef),
				want: []string{string(shape.Computation), string(shape.Aggregator)},
			},
			{
				name: "ranks an aggregator first for a callable with a context",
				give: shapetest.Method(storeName, countName, []*node.TypeRef{ctxRef}, intRef),
				want: []string{string(shape.Aggregator)},
			},
			{
				name: "ranks a computation before a reader without an error for a callable without a context",
				give: shapetest.Method(storeName, hashName, []*node.TypeRef{str}, str),
				want: []string{string(shape.Computation), string(shape.ReaderNoError)},
			},
			{
				name: "ranks a reader without an error first for a callable with a context",
				give: shapetest.Method(storeName, hashName, []*node.TypeRef{ctxRef, str}, str),
				want: []string{string(shape.ReaderNoError)},
			},
			{
				name: "ranks a computation before a pointer reader for a callable without a context",
				give: shapetest.Method(storeName, refName, []*node.TypeRef{str}, optionalRef),
				want: []string{string(shape.Computation), string(shape.PointerReader), string(shape.ReaderNoError)},
			},
			{
				name: "ranks a pointer reader first for a callable with a context",
				give: shapetest.Method(storeName, refName, []*node.TypeRef{ctxRef, str}, optionalRef),
				want: []string{string(shape.PointerReader), string(shape.ReaderNoError)},
			},
		}
		for _, tt := range ranks {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := shapetest.New(t, shapetest.Package(tt.give))
				got, _ := stampsOf(t, r)
				assert.Equal(t, got[tt.give.Name].detected, tt.want, "the detected shapes are in precedence order")
			})
		}

		t.Run("stamps nothing in a package without declarations", func(t *testing.T) {
			t.Parallel()

			r := shapetest.New(t, shapetest.Package())
			got, findings := stampsOf(t, r)
			expect.Empty(t, findings, "the empty package has no finding")
			expect.Empty(t, got, "the empty package has no callable to stamp")
		})

		t.Run("counts the inputs of a binding without a context parameter", func(t *testing.T) {
			t.Parallel()

			save := shapetest.Method(storeName, saveName, []*node.TypeRef{ctxRef, str}, failure)
			r := shapetest.New(t, shapetest.Package(save))
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[saveName].of.Params[string(shape.Writer)][shape.WriterValue], any(save.Params[1].ID),
				"the value of the writer is the parameter after the context")
		})

		t.Run("counts the results of a binding without the error", func(t *testing.T) {
			t.Parallel()

			get := shapetest.Method(storeName, getName, []*node.TypeRef{str}, str, failure)
			r := shapetest.New(t, shapetest.Package(get))
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[getName].of.Params[string(shape.Reader)], shape.Params{
				shape.ReaderKey: get.Params[0].ID, shape.ReaderValue: get.Returns[0].ID,
			}, "the key of the reader is its parameter and the value its first return")
		})

		t.Run("stamps nothing on a callable that no detector reports", func(t *testing.T) {
			t.Parallel()

			pair := shapetest.Method(storeName, pairName, []*node.TypeRef{str, str}, str, str)
			r := shapetest.New(t, shapetest.Package(pair))
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[pairName], stamps{}, "Pair has no detected shape")
		})

		t.Run("stamps nothing on a callable of a language without rules", func(t *testing.T) {
			t.Parallel()

			put := shapetest.Method(storeName, putName, []*node.TypeRef{str}, failure)
			r := shapetest.New(t, shapetest.Package(put))
			r.Rules = nil
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[putName], stamps{}, "the absent rules project nothing to detect")
		})

		t.Run("stamps a declared shape and keeps the detectors off its callable", func(t *testing.T) {
			t.Parallel()

			put := shapetest.Method(storeName, putName, []*node.TypeRef{str}, failure)
			r := shapetest.New(t, shapetest.Package(put))
			r.Declare(t, put.ID, shapetest.Instance{Shape: shape.Reader})
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[putName], stamps{of: shape.Classification{
				Shape:  shape.Reader,
				Params: map[string]shape.Params{string(shape.Reader): {shape.ReaderKey: put.Params[0].ID}},
			}}, "Put is a reader by its directive, and no detector runs on it")
		})

		t.Run("stamps the declared shape and a mixin of a function", func(t *testing.T) {
			t.Parallel()

			lookup := shapetest.Function(lookupName, []*node.TypeRef{str}, failure)
			r := shapetest.New(t, shapetest.Package(lookup))
			r.Declare(t, lookup.ID, shapetest.Instance{Shape: shape.Reader}, shapetest.Instance{Mixin: shape.Pure})
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[lookupName], stamps{of: shape.Classification{
				Shape:  shape.Reader,
				Mixins: []shape.Mixin{shape.Pure},
				Params: map[string]shape.Params{string(shape.Reader): {shape.ReaderKey: lookup.Params[0].ID}},
			}}, "Lookup is a pure reader by its directives")
		})

		t.Run("stamps no binding at a position that the callable lacks", func(t *testing.T) {
			t.Parallel()

			ping := shapetest.Method(storeName, pingName, nil, failure)
			r := shapetest.New(t, shapetest.Package(ping))
			r.Declare(t, ping.ID, shapetest.Instance{Shape: shape.Writer})
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[pingName].of, shape.Classification{Shape: shape.Writer},
				"Ping has no input, so the writer has no value")
		})

		t.Run("stamps a mixin and its params", func(t *testing.T) {
			t.Parallel()

			put := shapetest.Method(storeName, putName, []*node.TypeRef{str, str}, str, str)
			get := shapetest.Method(storeName, getName, []*node.TypeRef{str, str}, str, str)
			r := shapetest.New(t, shapetest.Package(put, get))
			r.Declare(t, put.ID, shapetest.Instance{Mixin: shape.Atomic, Params: map[directive.ParamKey]directive.Value{
				shape.AtomicRead: {Kind: directive.TypeReference, Ref: getName, Target: get.ID},
			}})
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[putName].of, shape.Classification{
				Mixins: []shape.Mixin{shape.Atomic},
				Params: map[string]shape.Params{string(shape.Atomic): {shape.AtomicRead: get.ID}},
			}, "Put is atomic, and Get reads its state")
		})

		t.Run("stamps the role, the id and the params of a contract", func(t *testing.T) {
			t.Parallel()

			commit := shapetest.Method(storeName, commitName, []*node.TypeRef{str, str}, str, str)
			r := shapetest.New(t, shapetest.Package(commit))
			r.Declare(t, commit.ID, shapetest.Instance{
				Contract: shape.Tx, Role: shape.TxCommit,
				Params: map[directive.ParamKey]directive.Value{
					catalog.IDParam: {Kind: directive.TypeString, Str: instanceID},
					shape.TxClosed:  {Kind: directive.TypeReference, Ref: closedName, Target: closedID},
				},
			})
			got, _ := stampsOf(t, r)
			assert.Equal(t, got[commitName].of, shape.Classification{
				Contracts: []shape.Membership{{Contract: shape.Tx, Role: shape.TxCommit, ID: instanceID}},
				Params:    map[string]shape.Params{string(shape.Tx): {shape.TxClosed: closedID}},
			}, "Commit commits the transaction a, which reports ErrClosed")
		})

		t.Run("stamps no id for an empty id", func(t *testing.T) {
			t.Parallel()

			commit := shapetest.Method(storeName, commitName, []*node.TypeRef{str, str}, str, str)
			r := shapetest.New(t, shapetest.Package(commit))
			r.Declare(t, commit.ID, shapetest.Instance{
				Contract: shape.Tx, Role: shape.TxCommit,
				Params: map[directive.ParamKey]directive.Value{catalog.IDParam: {Kind: directive.TypeString}},
			})
			got, _ := stampsOf(t, r)
			want := []shape.Membership{{Contract: shape.Tx, Role: shape.TxCommit}}
			assert.Equal(t, got[commitName].of.Contracts, want, "an empty id is the one instance of the scope")
		})

		t.Run("stamps the value of an int param", func(t *testing.T) {
			t.Parallel()

			retry := shapetest.Method(storeName, retryName, []*node.TypeRef{str, str}, str, str)
			r := shapetest.New(t, shapetest.Package(retry))
			r.Declare(t, retry.ID, shapetest.Instance{
				Mixin: shape.RetrySucceeds,
				Params: map[directive.ParamKey]directive.Value{
					shape.RetrySucceedsAttempts: {Kind: directive.TypeInt, Int: attempts},
				},
			})
			got, findings := stampsOf(t, r)
			expect.Empty(t, findings, "the number of attempts is above the minimum")
			assert.Equal(t, got[retryName].of.Params[string(shape.RetrySucceeds)], shape.Params{
				shape.RetrySucceedsAttempts: int64(attempts),
			}, "Retry succeeds after the number of attempts that its directive writes")
		})

		t.Run("reports an int below its minimum at the directive", func(t *testing.T) {
			t.Parallel()

			retry := shapetest.Method(storeName, retryName, []*node.TypeRef{str, str}, str, str)
			r := shapetest.New(t, shapetest.Package(retry))
			ds := r.Declare(t, retry.ID, shapetest.Instance{
				Mixin: shape.RetrySucceeds,
				Params: map[directive.ParamKey]directive.Value{
					shape.RetrySucceedsAttempts: {Kind: directive.TypeInt, Int: fewAttempts},
				},
			})
			_, findings := stampsOf(t, r)
			assert.Length(t, findings, 1, "the one attempt has one finding")
			expect.Equal(t, findings[0].Code, catalog.ParamRange, "the finding reports the range of the param")
			expect.Equal(t, findings[0].Pos, ds[0].Pos, "the finding is at the directive")
		})

		t.Run("reports two params that exclude each other once at the directive", func(t *testing.T) {
			t.Parallel()

			expiry := shapetest.Method(storeName, expiryName, []*node.TypeRef{str, str}, str, str)
			r := shapetest.New(t, shapetest.Package(expiry))
			ds := r.Declare(t, expiry.ID, shapetest.Instance{
				Mixin: shape.TTL,
				Params: map[directive.ParamKey]directive.Value{
					shape.TTLDuration: {Kind: directive.TypeString, Str: duration},
					shape.TTLLifetime: {Kind: directive.TypeReference, Ref: ghostName, Target: ghostID},
				},
			})
			_, findings := stampsOf(t, r)
			assert.Length(t, findings, 1, "the pair of params has one finding")
			expect.Equal(t, findings[0].Code, catalog.ExclusiveParams, "the finding reports the exclusive params")
			expect.Equal(t, findings[0].Pos, ds[0].Pos, "the finding is at the directive")
		})

		tests := []struct {
			name     string
			callee   func() node.Declaration
			axis     string
			findings []diag.Code
		}{
			{
				name: "accepts a host parameter that the method of another param declares",
				callee: func() node.Declaration {
					return shapetest.Method(storeName, getName, []*node.TypeRef{str}, str, failure)
				},
				axis: firstParam,
			},
			{
				name: "reports a host parameter that the method of another param does not declare",
				callee: func() node.Declaration {
					return shapetest.Method(storeName, getName, []*node.TypeRef{str}, str, failure)
				},
				axis:     secondParam,
				findings: []diag.Code{catalog.UnsharedParam},
			},
			{
				name: "accepts a host parameter that the function of another param declares",
				callee: func() node.Declaration {
					return shapetest.Function(lookupName, []*node.TypeRef{str}, str, failure)
				},
				axis: firstParam,
			},
			{
				name: "reports a host parameter that the function of another param does not declare",
				callee: func() node.Declaration {
					return shapetest.Function(lookupName, []*node.TypeRef{str}, str, failure)
				},
				axis:     secondParam,
				findings: []diag.Code{catalog.UnsharedParam},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				shard := shapetest.Method(storeName, shardName, []*node.TypeRef{str, str}, str, str)
				callee := tt.callee()
				r := shapetest.New(t, shapetest.Package(shard, callee))
				r.Declare(t, shard.ID, shapetest.Instance{
					Mixin: shape.Partition,
					Params: map[directive.ParamKey]directive.Value{
						shape.PartitionRead: {
							Kind:   directive.TypeReference,
							Ref:    callee.Identity().Name,
							Target: callee.Identity(),
						},
						shape.PartitionAxis: {Kind: directive.TypeReference, Ref: tt.axis},
					},
				})
				_, findings := stampsOf(t, r)
				assert.Equal(t, codesOf(findings), tt.findings, "the directive has the findings of its parameters")
			})
		}

		t.Run("reports a host parameter for a callable that the view does not contain", func(t *testing.T) {
			t.Parallel()

			shard := shapetest.Method(storeName, shardName, []*node.TypeRef{str, str}, str, str)
			r := shapetest.New(t, shapetest.Package(shard))
			r.Declare(t, shard.ID, shapetest.Instance{
				Mixin: shape.Partition,
				Params: map[directive.ParamKey]directive.Value{
					shape.PartitionRead: {Kind: directive.TypeReference, Ref: ghostName, Target: ghostID},
					shape.PartitionAxis: {Kind: directive.TypeReference, Ref: firstParam},
				},
			})
			_, findings := stampsOf(t, r)
			assert.Equal(t, codesOf(findings), []diag.Code{catalog.UnsharedParam},
				"a callable outside the view declares no parameter")
		})
	})
}

// The annotate phase of the plugin shape over the scaled corpus allocates
// inside classifyBudget. Each call classifies a run that the setup builds
// outside the count, because a run keeps its stamps. The check runs alone,
// because the count includes every goroutine's allocations.
func TestClassifyAllocs(t *testing.T) {
	var res plugintest.Result
	assert.MaxAllocsWithSetup(t, func() *shapetest.Run { return scaledRun(t) }, func(r *shapetest.Run) {
		res = r.Annotate(t, r.Annotators[0])
	}, classifyBudget, "the annotate phase over the scaled corpus allocates inside its budget")
	assert.NoError(t, res.Err, "the plugin shape classifies the scaled corpus")
}

// BenchmarkClassify measures the annotate phase of the plugin shape over a
// corpus of scaledHosts types, each with one method of each detected
// shape. The construction of the fixture is outside the measurement, and
// so is the seal of its index, which the run of the plugin shapecheck
// before it performs.
func BenchmarkClassify(b *testing.B) {
	b.Run("Annotate", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(classifyBudget)
		defer c.End()
		var res plugintest.Result
		for c.Loop() {
			var r *shapetest.Run
			c.Excluding(func() { r = scaledRun(b) })
			res = r.Annotate(b, r.Annotators[0])
		}
		assert.NoError(b, res.Err, "the plugin shape classifies the scaled corpus")
	})
}

// scaledRun returns a run over the corpus of scaledHosts types, after the
// plugin shapecheck sealed the index of its fixture. The annotate phase of
// the plugin shape is the next step of the run.
func scaledRun(tb assert.TB) *shapetest.Run {
	tb.Helper()

	hosts := make([]string, scaledHosts)
	for i := range hosts {
		hosts[i] = hostPrefix + strconv.Itoa(i)
	}
	pkg, _ := corpusPackage(hosts...)
	r := shapetest.New(tb, pkg)
	r.Annotate(tb, r.Annotators[1])
	return r
}

// hostedDelete returns the method Delete of the declaration Store of a
// kind. Delete takes a key and returns only its error.
func hostedDelete(kind symbol.Kind) *node.Method {
	del := shapetest.Method(storeName, deleteName, []*node.TypeRef{str}, failure)
	del.Host.Kind = kind
	return del
}

// stampsOf classifies the run, and returns the stamps of each callable by
// its name and the findings of the classification.
func stampsOf(t *testing.T, r *shapetest.Run) (map[string]stamps, []diag.Diag) {
	t.Helper()

	findings := r.Classify(t)
	got := map[string]stamps{}
	r.Consume(t,
		sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
			detected, _ := sdk.Fact(m, meta.Named[[]string](shape.KeyDetected))
			got[m.Method.Name] = stamps{detected: detected, of: shape.Of(m)}
			return nil
		}),
		sdk.OnFunction(func(m *sdk.FunctionMatch, _ *sdk.Emitter) error {
			detected, _ := sdk.Fact(m, meta.Named[[]string](shape.KeyDetected))
			got[m.Function.Name] = stamps{detected: detected, of: shape.Of(m)}
			return nil
		}),
	)
	return got, findings
}

// codesOf returns the codes of the findings, in order, and nil for no
// finding.
func codesOf(findings []diag.Diag) []diag.Code {
	if len(findings) == 0 {
		return nil
	}
	out := make([]diag.Code, 0, len(findings))
	for _, d := range findings {
		out = append(out, d.Code)
	}
	return out
}
