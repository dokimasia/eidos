// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest_test

import (
	"strconv"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
)

// The identities of the declarations of the scripted tree that the
// contracts of the kit include, in the form that [symbol.Identity.String]
// returns.
const (
	userField = "fake:svc/api.User#f0"
	boxField  = "fake:svc/api.Box#f0"
	boxParam  = "fake:svc/api.Box#T"
	rowUser   = "fake:svc/store.Row#f0"
	rowInt    = "fake:svc/store.Row#f1"
	rowString = "fake:svc/store.Row#f2"
)

// The contracts of the per-subject checks of the kit. The subject follows
// each contract in its record.
const (
	distinct   = "the two halves of a derived pair differ: "
	sampled    = "a sample without a value has a refusal: "
	alternated = "an alternate without a value has a refusal: "
	leaf       = "a named reference folds to a leaf and not to its name: "
	spelled    = "a derived witness has a spelling: "
	recorded   = "a sample of a named type reads the declaration through the view that it receives: "
)

// TestSuite checks that the scripted language passes the suite, and that
// each check rejects a language with the defect that the check covers.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunRulesSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the scripted language on every check", func(t *testing.T) {
			t.Parallel()
			rulestest.RunRulesSuite(t, setup)
		})
	})

	t.Run("AssertDeterministic", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fold whose leaves change between bounds", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a builtin projection that counts its calls", func(tb assert.TB) {
				rulestest.AssertDeterministic(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return &restless{SourceRules: rulestest.Scripted()}, f
				})
			})
			assert.Equal(t, coretest.Contracts(records), []string{"bounds over one view return equal values"},
				"the rejection names the property")
		})
	})

	t.Run("AssertDistinctSamples", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a language that derives one value twice", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a pair that cannot tell a subject apart", func(tb assert.TB) {
				rulestest.AssertDistinctSamples(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return same{rulestest.Scripted()}, f
				})
			})
			assert.Permutation(t, coretest.Contracts(records), []string{
				distinct + userField, distinct + rowInt, distinct + rowString,
			}, "the rejection names each field whose type has a pair")
		})

		t.Run("passes a pair that differs below its first level", func(t *testing.T) {
			t.Parallel()

			rulestest.AssertDistinctSamples(t, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
				_, f := setup(tb)
				return nested{rulestest.Scripted()}, f
			})
		})

		fresh := []struct {
			name string
			give func(t *emit.TypeRef) emit.Value
		}{
			{
				name: "passes a pair of two functions of one type",
				give: func(t *emit.TypeRef) emit.Value { return emit.Func(t) },
			},
			{name: "passes a pair of two makes of one type", give: emit.Make},
			{
				name: "passes a pair of two conversions of functions",
				give: func(t *emit.TypeRef) emit.Value { return emit.Conversion(t, emit.Func(t)) },
			},
		}
		for _, tt := range fresh {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				rulestest.AssertDistinctSamples(t, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return twice{SourceRules: rulestest.Scripted(), value: tt.give}, f
				})
			})
		}
	})

	t.Run("AssertRefusesWithReason", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a refusal without a reason", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a sample that refuses without a reason", func(tb assert.TB) {
				rulestest.AssertRefusesWithReason(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return silent{rulestest.Scripted()}, f
				})
			})
			fields := []string{userField, boxField, rowUser, rowInt, rowString}
			want := append(make([]string, 0, 1+2*len(fields)),
				"a sample of a spelling that the language cannot reason about has a refusal")
			for _, field := range fields {
				want = append(want, sampled+field, alternated+field)
			}
			assert.Permutation(t, coretest.Contracts(records), want,
				"the rejection names both halves of every field and the unknown spelling")
		})
	})

	t.Run("AssertTotal", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fold that returns a name", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a leaf that is not a leaf", func(tb assert.TB) {
				rulestest.AssertTotal(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return naming{rulestest.Scripted()}, f
				})
			})
			assert.Permutation(t, coretest.Contracts(records), []string{
				leaf + "string at svc/api/user.zz:2:0",
				leaf + "int at svc/api/user.zz:3:0",
				leaf + "int at svc/store/row.zz:3:0",
				leaf + "string at svc/store/row.zz:3:0",
				leaf + "int at svc/store/row.zz:4:0",
			}, "the rejection names every reference that folds through Builtin")
		})
	})

	t.Run("AssertWitnesses", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a derived witness without a spelling", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a witness that no backend can write", func(tb assert.TB) {
				rulestest.AssertWitnesses(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					s := rulestest.Scripted()
					return unspelled{SourceRules: s, GenericsRules: s.(rules.GenericsRules)}, f
				})
			})
			assert.Equal(t, coretest.Contracts(records), []string{spelled + boxParam},
				"the rejection names the parameter whose witness has no spelling")
		})

		t.Run("passes a language without the generics capability", func(t *testing.T) {
			t.Parallel()

			rulestest.AssertWitnesses(t, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
				_, f := setup(tb)
				return same{rulestest.Scripted()}, f
			})
		})

		t.Run("passes a fixture that declares no type parameter", func(t *testing.T) {
			t.Parallel()

			rulestest.AssertWitnesses(t, over(fstest.MapFS{storeFile: {Data: []byte(rowSource)}}))
		})

		t.Run("rejects a fixture whose type parameters have no reference", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a generic type that no field uses", func(tb assert.TB) {
				rulestest.AssertWitnesses(tb, over(fstest.MapFS{
					apiFile: {Data: []byte("package svc/api\ntype Box int\ntypeparam T\n")},
				}))
			})
			assert.Equal(t, coretest.Contracts(records),
				[]string{"the fixture references a type parameter whose witnesses derive"},
				"the rejection states why the fixture fails")
		})
	})

	t.Run("AssertRecorded", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fixture without a graph", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "nothing to read", func(tb assert.TB) {
				rulestest.AssertRecorded(tb, func(assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					return rulestest.Scripted(), &rulestest.Fixture{}
				})
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the fixture has a graph"},
				"the rejection states what the setup must return")
		})

		t.Run("rejects a language that reads through a view of its own", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "reads that the view never records", func(tb assert.TB) {
				rulestest.AssertRecorded(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return unrecorded{SourceRules: rulestest.Scripted(), graph: f.Graph}, f
				})
			})
			assert.Equal(t, coretest.Contracts(records), []string{recorded + rowUser},
				"the rejection names the field whose declaration the view did not record")
		})
	})
}

// over returns a setup that loads the scripted rules over a tree of the
// case.
func over(tree fstest.MapFS) rulestest.Setup {
	return func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
		return rulestest.Scripted(), rulestest.Loaded(tb, frontendtest.NewScripted(), tree, frontendtest.ScriptedKeys)
	}
}

// same derives one value for both halves.
type same struct {
	rules.SourceRules
}

// DeriveSamples returns the first half twice.
func (s same) DeriveSamples(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	sample, _ := s.SourceRules.DeriveSamples(ref, hint, v)
	return sample, sample
}

// nested derives two composites that differ only in a field of a field. A
// comparison of the first level reports them as equal.
type nested struct {
	rules.SourceRules
}

// The texts of the nested pair. They differ two levels down.
const (
	nestedSample    = "1"
	nestedAlternate = "2"
)

// DeriveSamples returns two composites of one composite each.
func (nested) DeriveSamples(ref *node.TypeRef, _ string, _ rules.View) (rules.Sample, rules.Sample) {
	outer := func(text string) rules.Sample {
		inner := emit.Composite(rules.EmitRef(ref), emit.NamedField("b", emit.Literal(emit.LiteralInt, text)))
		return rules.Of(emit.Composite(rules.EmitRef(ref), emit.NamedField("a", inner)))
	}
	return outer(nestedSample), outer(nestedAlternate)
}

// twice derives one tree for both halves through value. A function and a
// make are new values wherever a body writes them, so the halves of such a
// pair differ although their trees are equal.
type twice struct {
	rules.SourceRules
	value func(t *emit.TypeRef) emit.Value
}

// DeriveSamples returns the tree of value for the reference, twice.
func (w twice) DeriveSamples(ref *node.TypeRef, _ string, _ rules.View) (rules.Sample, rules.Sample) {
	return rules.Of(w.value(rules.EmitRef(ref))), rules.Of(w.value(rules.EmitRef(ref)))
}

// silent refuses without a reason.
type silent struct {
	rules.SourceRules
}

// DeriveSamples returns two empty samples without a refusal.
func (silent) DeriveSamples(*node.TypeRef, string, rules.View) (rules.Sample, rules.Sample) {
	return rules.Sample{}, rules.Sample{}
}

// naming folds a builtin to the named form. No leaf has the named form.
type naming struct {
	rules.SourceRules
}

// Builtin returns the name itself.
func (naming) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	return rules.TypeShape{Spelling: ref.Spelling}
}

// restless folds every builtin to a spelling that counts its calls,
// so no two bounds fold one reference alike.
type restless struct {
	rules.SourceRules
	calls atomic.Int64
}

// Builtin returns an opaque leaf whose spelling contains the call count.
func (r *restless) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	return rules.Opaque(&node.TypeRef{Spelling: ref.Spelling + strconv.FormatInt(r.calls.Add(1), 10)})
}

// unrecorded derives samples through a view over a read set of its own,
// so no read records on the view that it receives.
type unrecorded struct {
	rules.SourceRules
	graph *store.Graph
}

// DeriveSamples derives through a view of its own, which reads no fact.
func (u unrecorded) DeriveSamples(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	reader, err := u.graph.Reader(store.NewReadSet(), nil)
	if err != nil {
		return rules.Refused(rules.RefusedNoView), rules.Refused(rules.RefusedNoView)
	}
	return u.SourceRules.DeriveSamples(ref, hint, rules.NewView(reader, nil, nil, v.Kernel()))
}

// unspelled is the scripted language with a generics defect. Its Derive
// returns a witness without a spelling.
type unspelled struct {
	rules.SourceRules
	rules.GenericsRules
}

// Derive returns a reference without a spelling.
func (unspelled) Derive(*node.TypeParam, rules.View) (*node.TypeRef, bool) {
	return &node.TypeRef{}, true
}
