// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the declarations of the fixtures, and the values of their
// directives.
const (
	storeName  = "Store"
	putName    = "Put"
	getName    = "Get"
	findName   = "Find"
	commitName = "Commit"
	retryName  = "Retry"
	staleName  = "Stale"
	closedName = "ErrClosed"
	instanceID = "a"
	sample     = "x"
	attempts   = 3
)

// The allocations of Of on two callables of the fixture. A map of params
// costs two allocations, and the value of a binding or a reference costs
// one, because Of boxes it. Put has the map of params, the map of the
// writer and the identity of its value. Commit also has the list of two
// mixins, which grows twice, the list of one membership, and a second map
// with a reference.
const (
	writerOfAllocs = 5
	commitOfAllocs = 11
)

// closedID is the identity of the error that a reference of the fixtures
// resolves to.
var closedID = symbol.Identity{
	Lang:    shapetest.Lang,
	Package: shapetest.Path,
	Name:    closedName,
	Kind:    symbol.KindVariable,
}

// classification is the fixture of the cases of Of and of the predicates.
// It has one method of each kind of classification, the run that
// classified them, and the classification of each method by its name.
type classification struct {
	put, get, find, commit, retry, stale *node.Method
	run                                  *shapetest.Run
	got                                  map[string]shape.Classification
}

// classify classifies the methods of the fixture, and returns the run and
// the classification of each method. Stale has the summary keys of a
// shape, a mixin and a contract without a family key, as a meta drop
// leaves a callable.
func classify(tb testing.TB) classification {
	tb.Helper()

	str := &node.TypeRef{Spelling: shapetest.String}
	failure := &node.TypeRef{Spelling: shapetest.Error}
	c := classification{
		put:    shapetest.Method(storeName, putName, []*node.TypeRef{str}, failure),
		get:    shapetest.Method(storeName, getName, []*node.TypeRef{str, str}, str, str),
		find:   shapetest.Method(storeName, findName, []*node.TypeRef{str}, str),
		commit: shapetest.Method(storeName, commitName, nil, failure),
		retry:  shapetest.Method(storeName, retryName, []*node.TypeRef{str, str}, str, str),
		stale:  shapetest.Method(storeName, staleName, []*node.TypeRef{str, str}, str, str),
		got:    map[string]shape.Classification{},
	}
	r := shapetest.New(tb, shapetest.Package(c.put, c.get, c.find, c.commit, c.retry, c.stale))
	c.run = r
	get := directive.Value{Kind: directive.TypeReference, Ref: getName, Target: c.get.ID}
	r.Declare(tb, c.find.ID, shapetest.Instance{Shape: shape.Writer, Params: map[directive.ParamKey]directive.Value{
		shape.WriterReads:  get,
		shape.WriterSample: {Kind: directive.TypeString, Str: sample},
	}})
	r.Declare(tb, c.commit.ID,
		shapetest.Instance{Mixin: shape.Atomic, Params: map[directive.ParamKey]directive.Value{shape.AtomicRead: get}},
		shapetest.Instance{Mixin: shape.Idempotent},
		shapetest.Instance{Contract: shape.Tx, Role: shape.TxCommit, Params: map[directive.ParamKey]directive.Value{
			catalog.IDParam: {Kind: directive.TypeString, Str: instanceID},
			shape.TxClosed:  {Kind: directive.TypeReference, Ref: closedName, Target: closedID},
		}})
	r.Declare(tb, c.retry.ID, shapetest.Instance{
		Mixin: shape.RetrySucceeds,
		Params: map[directive.ParamKey]directive.Value{
			shape.RetrySucceedsAttempts: {Kind: directive.TypeInt, Int: attempts},
		},
	})
	shapeKey, _ := meta.Lookup[string](r.Keys, shape.KeyShape)
	plugintest.Stamp(tb, r.Fixture, shapeKey, c.stale.ID, string(shape.Writer))
	for _, name := range []meta.KeyName{shape.KeyMixed, shape.KeyMember} {
		mark, _ := meta.Lookup[bool](r.Keys, name)
		plugintest.Stamp(tb, r.Fixture, mark, c.stale.ID, true)
	}
	r.Classify(tb)
	r.Consume(tb, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
		c.got[m.Method.Name] = shape.Of(m)
		return nil
	}))
	expect.Length(tb, c.got, 6, "the consumer reads every method")
	return c
}

// Of reads back what the plugin shape stamped on a callable.
func TestClassification(t *testing.T) {
	t.Parallel()

	t.Run("Of", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the zero classification for a callable without one", func(t *testing.T) {
			t.Parallel()

			c := classify(t)
			assert.Equal(t, c.got[getName], shape.Classification{}, "Get has no classification")
		})

		t.Run("returns the detected shape and its bindings", func(t *testing.T) {
			t.Parallel()

			c := classify(t)
			assert.Equal(t, c.got[putName], shape.Classification{
				Shape:  shape.Writer,
				Params: map[string]shape.Params{string(shape.Writer): {shape.WriterValue: c.put.Params[0].ID}},
			}, "Put is a writer, and its parameter fills the value")
		})

		t.Run("returns the declared shape and the values of its params", func(t *testing.T) {
			t.Parallel()

			c := classify(t)
			assert.Equal(t, c.got[findName], shape.Classification{
				Shape: shape.Writer,
				Params: map[string]shape.Params{string(shape.Writer): {
					shape.WriterValue: c.find.Params[0].ID, shape.WriterReads: c.get.ID, shape.WriterSample: sample,
				}},
			}, "Find is a writer by its directive, with the values that the directive writes")
		})

		t.Run("returns the mixins in name order and the role of each contract instance", func(t *testing.T) {
			t.Parallel()

			c := classify(t)
			assert.Equal(t, c.got[commitName], shape.Classification{
				Shape:     shape.PoisonAccessor,
				Mixins:    []shape.Mixin{shape.Atomic, shape.Idempotent},
				Contracts: []shape.Membership{{Contract: shape.Tx, Role: shape.TxCommit, ID: instanceID}},
				Params: map[string]shape.Params{
					string(shape.Atomic): {shape.AtomicRead: c.get.ID},
					string(shape.Tx):     {shape.TxClosed: closedID},
				},
			}, "Commit has its detected shape, its two mixins and its role in the instance a")
		})

		t.Run("returns the value of an int param", func(t *testing.T) {
			t.Parallel()

			c := classify(t)
			assert.Equal(t, c.got[retryName].Params, map[string]shape.Params{
				string(shape.RetrySucceeds): {shape.RetrySucceedsAttempts: int64(attempts)},
			}, "Retry has the number of attempts that its directive writes")
		})

		t.Run("leaves out a classification whose family key the subject lacks", func(t *testing.T) {
			t.Parallel()

			c := classify(t)
			assert.Equal(t, c.got[staleName], shape.Classification{},
				"the summary keys of Stale point at no classification, so Of returns none")
		})
	})
}

// Of allocates the classification that it returns, and nothing for its
// reads once the read set of the invocation has grown.
func TestClassificationAllocs(t *testing.T) {
	c := classify(t)
	tests := []struct {
		name    string
		subject symbol.Identity
		allocs  uint64
	}{
		{"a detected writer", c.put.ID, writerOfAllocs},
		{"a callable with a shape, two mixins and a contract role", c.commit.ID, commitOfAllocs},
	}
	for _, tt := range tests {
		c.run.Consume(t, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
			if m.Method.ID != tt.subject {
				return nil
			}
			var got shape.Classification
			expect.MaxAllocs(t, func() { got = shape.Of(m) }, tt.allocs,
				"Of allocates the classification of "+tt.name+" alone")
			expect.NotEqual(t, got.Shape, "", "the classification has the shape of "+tt.name)
			return nil
		}))
	}
}

// BenchmarkClassification measures the read of every classification of
// Put, a detected writer, and of Commit, which has a shape, two mixins and
// a role in a contract instance.
func BenchmarkClassification(b *testing.B) {
	b.Run("Of", func(b *testing.B) {
		b.Run("a detected writer", func(b *testing.B) {
			c := classify(b)
			c.run.Consume(b, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
				if m.Method.ID != c.put.ID {
					return nil
				}
				var got shape.Classification
				bc := bench.Start(b).MaxAllocs(writerOfAllocs).Warmup(1)
				defer bc.End()
				for bc.Loop() {
					got = shape.Of(m)
				}
				assert.Equal(b, got.Shape, shape.Writer, "Put is a writer")
				return nil
			}))
		})

		b.Run("a callable with a shape, two mixins and a contract role", func(b *testing.B) {
			c := classify(b)
			c.run.Consume(b, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
				if m.Method.ID != c.commit.ID {
					return nil
				}
				var got shape.Classification
				bc := bench.Start(b).MaxAllocs(commitOfAllocs).Warmup(1)
				defer bc.End()
				for bc.Loop() {
					got = shape.Of(m)
				}
				assert.Length(b, got.Mixins, 2, "Commit has two mixins")
				return nil
			}))
		})
	})
}
