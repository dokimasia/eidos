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

// The names of the declarations of the instance fixtures, the package of a
// callable outside the view, and the ids of the instances in the package.
const (
	cacheName    = "Cache"
	beginName    = "Begin"
	rollbackName = "Rollback"
	stepName     = "Step"
	undoName     = "Undo"
	appendName   = "Append"
	replayName   = "Replay"
	ghostHost    = "Ghost"
	ghostPath    = "acme/ghost"
	firstID      = "a"
	secondID     = "b"
	cacheID      = "cache"
)

// instanceAllocs is the number of allocations of a read of an instance of
// three members. The list of members grows to a capacity of one, two and
// four.
const instanceAllocs = 3

// instances is the fixture of the cases of InstanceOf. The struct Store
// declares a transaction and two sagas, and the struct Cache declares the
// begin of a transaction of its own id. A method of Cache and two
// functions of the package form a chain.
type instances struct {
	begin, get, commit, rollback, cacheBegin *node.Method
	stepA, undoA, stepB, undoB, chainMethod  *node.Method
	appendFn, replayFn                       *node.Function
	run                                      *shapetest.Run
}

// instanceFixture declares and classifies the fixture of the cases of
// InstanceOf.
func instanceFixture(tb testing.TB) instances {
	tb.Helper()

	f := instances{
		begin:       shapetest.Method(storeName, beginName, nil),
		get:         shapetest.Method(storeName, getName, nil),
		commit:      shapetest.Method(storeName, commitName, nil),
		rollback:    shapetest.Method(storeName, rollbackName, nil),
		stepA:       shapetest.Method(storeName, stepName+firstID, nil),
		undoA:       shapetest.Method(storeName, undoName+firstID, nil),
		stepB:       shapetest.Method(storeName, stepName+secondID, nil),
		undoB:       shapetest.Method(storeName, undoName+secondID, nil),
		cacheBegin:  shapetest.Method(cacheName, beginName, nil),
		chainMethod: shapetest.Method(cacheName, appendName, nil),
		appendFn:    shapetest.Function(appendName, nil),
		replayFn:    shapetest.Function(replayName, nil),
	}
	f.run = shapetest.New(tb, shapetest.Package(
		f.begin, f.get, f.commit, f.rollback, f.stepA, f.undoA, f.stepB, f.undoB,
		f.cacheBegin, f.chainMethod, f.appendFn, f.replayFn,
	))
	roles := []struct {
		subject  symbol.Identity
		contract shape.Contract
		role     shape.Role
		id       string
	}{
		{f.begin.ID, shape.Tx, shape.TxBegin, ""},
		{f.commit.ID, shape.Tx, shape.TxCommit, ""},
		{f.rollback.ID, shape.Tx, shape.TxRollback, ""},
		{f.stepA.ID, shape.Saga, shape.SagaStep, firstID},
		{f.undoA.ID, shape.Saga, shape.SagaCompensate, firstID},
		{f.stepB.ID, shape.Saga, shape.SagaStep, secondID},
		{f.undoB.ID, shape.Saga, shape.SagaCompensate, secondID},
		{f.cacheBegin.ID, shape.Tx, shape.TxBegin, cacheID},
		{f.chainMethod.ID, shape.Chain, shape.ChainAppend, ""},
		{f.appendFn.ID, shape.Chain, shape.ChainAppend, ""},
		{f.replayFn.ID, shape.Chain, shape.ChainReplay, ""},
	}
	for _, r := range roles {
		in := shapetest.Instance{Contract: r.contract, Role: r.role}
		if r.id != "" {
			in.Params = map[directive.ParamKey]directive.Value{catalog.IDParam: {Kind: directive.TypeString, Str: r.id}}
		}
		f.run.Declare(tb, r.subject, in)
	}
	f.run.Classify(tb)
	return f
}

// InstanceOf returns the members of the instance of a contract that a
// callable has a role in.
func TestInstance(t *testing.T) {
	t.Parallel()

	t.Run("InstanceOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the members of the instance of a method in declaration order", func(t *testing.T) {
			t.Parallel()

			f := instanceFixture(t)
			got, found := instanceOf(t, f, f.commit, shape.Tx)
			assert.True(t, found, "Commit has a role in a transaction")
			assert.Equal(t, got, shape.Instance{
				Contract: shape.Tx,
				Package:  f.commit.ID.PackageIdentity(),
				Members: []shape.Member{
					{Callable: f.begin.ID, Role: shape.TxBegin},
					{Callable: f.commit.ID, Role: shape.TxCommit},
					{Callable: f.rollback.ID, Role: shape.TxRollback},
				},
			}, "the instance has the three methods of Store, and not the begin of Cache, whose id differs")
		})

		t.Run("returns the members with the id of the instance", func(t *testing.T) {
			t.Parallel()

			f := instanceFixture(t)
			got, found := instanceOf(t, f, f.undoB, shape.Saga)
			assert.True(t, found, "UndoB has a role in a saga")
			assert.Equal(t, got, shape.Instance{
				Contract: shape.Saga,
				Package:  f.undoB.ID.PackageIdentity(),
				ID:       secondID,
				Members: []shape.Member{
					{Callable: f.stepB.ID, Role: shape.SagaStep},
					{Callable: f.undoB.ID, Role: shape.SagaCompensate},
				},
			}, "the instance b has the step and the compensation of b alone")
		})

		t.Run("returns the methods and the functions of a package as one instance", func(t *testing.T) {
			t.Parallel()

			f := instanceFixture(t)
			got, found := instanceOf(t, f, f.replayFn, shape.Chain)
			assert.True(t, found, "Replay has a role in a chain")
			assert.Equal(t, got, shape.Instance{
				Contract: shape.Chain,
				Package:  symbol.Identity{Lang: shapetest.Lang, Package: shapetest.Path, Kind: symbol.KindPackage},
				Members: []shape.Member{
					{Callable: f.chainMethod.ID, Role: shape.ChainAppend},
					{Callable: f.appendFn.ID, Role: shape.ChainAppend},
					{Callable: f.replayFn.ID, Role: shape.ChainReplay},
				},
			}, "the instance has the method of Cache and the two functions")
		})

		t.Run("reports false for a callable without a role in the contract", func(t *testing.T) {
			t.Parallel()

			f := instanceFixture(t)
			_, found := instanceOf(t, f, f.get, shape.Tx)
			assert.False(t, found, "Get has no role in a transaction")
		})

		t.Run("reports false for a name that no contract spec has", func(t *testing.T) {
			t.Parallel()

			f := instanceFixture(t)
			_, found := instanceOf(t, f, f.commit, ghostName)
			assert.False(t, found, "no contract has the name ghost")
		})

		t.Run("returns an instance without members for a callable outside the view", func(t *testing.T) {
			t.Parallel()

			f := instanceFixture(t)
			host := symbol.Identity{Lang: shapetest.Lang, Package: ghostPath, Name: ghostHost, Kind: symbol.KindStruct}
			id := symbol.Identity{
				Lang: shapetest.Lang, Package: ghostPath, Owner: ghostHost, Name: commitName, Kind: symbol.KindMethod,
			}
			ghost := &node.Method{ID: id, Name: commitName, Host: host}
			tx, _ := shape.SpecOf(shape.Tx)
			role, _ := meta.Lookup[string](f.run.Keys, tx.Key)
			plugintest.Stamp(t, f.run.Fixture, role, ghost.ID, string(shape.TxCommit))
			var got shape.Instance
			var found bool
			f.run.Consume(t, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
				if m.Method.ID == f.commit.ID {
					got, found = shape.InstanceOf(m, ghost, shape.Tx)
				}
				return nil
			}))
			assert.True(t, found, "the ghost method has a role in a transaction")
			assert.Equal(t, got, shape.Instance{Contract: shape.Tx, Package: ghost.ID.PackageIdentity()},
				"the instance of a method outside the view has its package and no members")
		})
	})
}

// InstanceOf allocates the list of members of the instance alone, once the
// read set of the invocation has grown.
func TestInstanceAllocs(t *testing.T) {
	f := instanceFixture(t)
	f.run.Consume(t, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
		if m.Method.ID != f.commit.ID {
			return nil
		}
		var got shape.Instance
		expect.MaxAllocs(t, func() { got, _ = shape.InstanceOf(m, m.Method, shape.Tx) }, instanceAllocs,
			"InstanceOf allocates the growth of the list of three members")
		expect.Length(t, got.Members, 3, "the instance has three members")
		return nil
	}))
}

// BenchmarkInstance measures the read of an instance of three members.
func BenchmarkInstance(b *testing.B) {
	b.Run("InstanceOf", func(b *testing.B) {
		f := instanceFixture(b)
		f.run.Consume(b, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
			if m.Method.ID != f.commit.ID {
				return nil
			}
			var got shape.Instance
			c := bench.Start(b).MaxAllocs(instanceAllocs).Warmup(1)
			defer c.End()
			for c.Loop() {
				got, _ = shape.InstanceOf(m, m.Method, shape.Tx)
			}
			assert.Length(b, got.Members, 3, "the instance has three members")
			return nil
		}))
	})
}

// instanceOf returns the instance of the contract c that a callable of the
// fixture has a role in, as a consumer reads it in the invocation of the
// callable.
func instanceOf(t *testing.T, f instances, callable node.Declaration, c shape.Contract) (shape.Instance, bool) {
	t.Helper()

	var got shape.Instance
	var found bool
	read := func(m shape.Scoped, subject node.Declaration) {
		if subject.Identity() == callable.Identity() {
			got, found = shape.InstanceOf(m, subject, c)
		}
	}
	f.run.Consume(t,
		sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
			read(m, m.Method)
			return nil
		}),
		sdk.OnFunction(func(m *sdk.FunctionMatch, _ *sdk.Emitter) error {
			read(m, m.Function)
			return nil
		}),
	)
	return got, found
}
