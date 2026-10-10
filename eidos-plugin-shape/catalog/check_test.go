// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
)

// The names of the declarations of the fixtures of the plugin shapecheck,
// the id of a second instance, and the parts of the messages of its
// findings.
const (
	stepName     = "Step"
	undoName     = "Undo"
	appendName   = "Append"
	replayName   = "Replay"
	dbName       = "DB"
	txName       = "Tx"
	secondID     = "b"
	txScope      = "the contract tx in the package " + shapetest.Path
	chainScope   = "the contract chain in the package " + shapetest.Path
	instanceText = "the instance b of the contract saga"
	tooMany      = " has 2 callables in the role commit, whose arity is one"
	noBegin      = " has no callable in the role begin, whose arity is one"
)

// The plugin shapecheck validates the arity of every role of every
// contract instance.
func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		t.Run("accepts an instance whose roles have the callables that their arities admit", func(t *testing.T) {
			t.Parallel()

			r, _ := transaction(t, shape.TxBegin, shape.TxCommit, shape.TxRollback)
			assert.Empty(t, r.Classify(t), "the transaction has its begin, its commit and its rollback")
		})

		t.Run("reports a role with too many callables at each callable of the role", func(t *testing.T) {
			t.Parallel()

			r, methods := transaction(t, shape.TxBegin, shape.TxCommit, shape.TxCommit, shape.TxRollback)
			findings := r.Classify(t)
			assert.Length(t, findings, 2, "each commit has a finding")
			for i, d := range findings {
				expect.Equal(t, d.Code, catalog.RoleArity, "the finding reports the arity of the role")
				expect.Equal(t, d.Pos, methods[i+1].Pos, "the finding is at a commit")
				expect.Equal(t, d.Msg, txScope+tooMany, "the finding counts the commits of the package")
			}
		})

		t.Run("reports a missing role once at the first member of the instance", func(t *testing.T) {
			t.Parallel()

			r, methods := transaction(t, shape.TxCommit, shape.TxRollback)
			findings := r.Classify(t)
			assert.Length(t, findings, 1, "the missing begin has one finding")
			expect.Equal(t, findings[0].Code, catalog.RoleArity, "the finding reports the arity of the role")
			expect.Equal(t, findings[0].Pos, methods[0].Pos, "the finding is at the first member")
			expect.Equal(t, findings[0].Msg, txScope+noBegin, "the finding names the missing role")
		})

		t.Run("accepts an instance whose methods belong to two types of the package", func(t *testing.T) {
			t.Parallel()

			begin := shapetest.Method(dbName, string(shape.TxBegin), nil, failure)
			commit := shapetest.Method(txName, string(shape.TxCommit), nil, failure)
			rollback := shapetest.Method(txName, string(shape.TxRollback), nil, failure)
			r := shapetest.New(t, shapetest.Package(begin, commit, rollback))
			r.Declare(t, begin.ID, shapetest.Instance{Contract: shape.Tx, Role: shape.TxBegin})
			r.Declare(t, commit.ID, shapetest.Instance{Contract: shape.Tx, Role: shape.TxCommit})
			r.Declare(t, rollback.ID, shapetest.Instance{Contract: shape.Tx, Role: shape.TxRollback})
			assert.Empty(t, r.Classify(t), "the begin of DB and the commit and the rollback of Tx form one transaction")
		})

		t.Run("validates the instances of one package apart by their ids", func(t *testing.T) {
			t.Parallel()

			stepA := shapetest.Method(storeName, stepName+instanceID, nil, failure)
			undoA := shapetest.Method(storeName, undoName+instanceID, nil, failure)
			stepB := shapetest.Method(storeName, stepName+secondID, nil, failure)
			r := shapetest.New(t, shapetest.Package(stepA, undoA, stepB))
			for _, d := range []struct {
				method *node.Method
				role   shape.Role
				id     string
			}{
				{stepA, shape.SagaStep, instanceID},
				{undoA, shape.SagaCompensate, instanceID},
				{stepB, shape.SagaStep, secondID},
			} {
				r.Declare(t, d.method.ID, shapetest.Instance{
					Contract: shape.Saga,
					Role:     d.role,
					Params: map[directive.ParamKey]directive.Value{
						catalog.IDParam: {Kind: directive.TypeString, Str: d.id},
					},
				})
			}
			findings := r.Classify(t)
			assert.Length(t, findings, 1, "the saga b lacks its compensation")
			expect.Equal(t, findings[0].Pos, stepB.Pos, "the finding is at the step of b")
			expect.HasPrefix(t, findings[0].Msg, instanceText, "the finding names the instance b")
		})

		t.Run("names the package of the functions of an instance", func(t *testing.T) {
			t.Parallel()

			appendFn := shapetest.Function(appendName, nil, failure)
			r := shapetest.New(t, shapetest.Package(appendFn))
			r.Declare(t, appendFn.ID, shapetest.Instance{Contract: shape.Chain, Role: shape.ChainAppend})
			findings := r.Classify(t)
			assert.Length(t, findings, 1, "the chain lacks its replay")
			expect.HasPrefix(t, findings[0].Msg, chainScope, "the finding names the package of the chain")
		})

		t.Run("accepts an optional role without a callable", func(t *testing.T) {
			t.Parallel()

			appendFn := shapetest.Function(appendName, nil, failure)
			replayFn := shapetest.Function(replayName, nil, failure)
			r := shapetest.New(t, shapetest.Package(appendFn, replayFn))
			r.Declare(t, appendFn.ID, shapetest.Instance{Contract: shape.Chain, Role: shape.ChainAppend})
			r.Declare(t, replayFn.ID, shapetest.Instance{Contract: shape.Chain, Role: shape.ChainReplay})
			assert.Empty(t, r.Classify(t), "the chain needs no verify")
		})
	})
}

// transaction returns a run over one method of the struct Store for each
// role, in order, and the methods. Each method declares its role in the
// one transaction of the package.
func transaction(t *testing.T, roles ...shape.Role) (*shapetest.Run, []*node.Method) {
	t.Helper()

	methods := make([]*node.Method, 0, len(roles))
	decls := make([]node.Declaration, 0, len(roles))
	for i, role := range roles {
		m := shapetest.Method(storeName, string(role)+strconv.Itoa(i), nil, failure)
		methods = append(methods, m)
		decls = append(decls, m)
	}
	r := shapetest.New(t, shapetest.Package(decls...))
	for i, role := range roles {
		r.Declare(t, methods[i].ID, shapetest.Instance{Contract: shape.Tx, Role: role})
	}
	return r, methods
}
