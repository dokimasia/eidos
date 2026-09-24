// Copyright ThesmOS B.V. 2026
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
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The suite is the projections' conformance check, so the scripted
// language must pass it, and each check must reject a language
// broken in the way that check covers.
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

			msg := assert.Rejects(t, "a builtin projection that counts its calls", func(tb assert.TB) {
				rulestest.AssertDeterministic(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return &restless{SourceRules: rulestest.Scripted()}, f
				})
			})
			assert.Contains(t, msg, "equal values", "the rejection names the property")
		})
	})

	t.Run("AssertDistinctSamples", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a language deriving one value twice", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a pair that cannot tell a subject apart", func(tb assert.TB) {
				rulestest.AssertDistinctSamples(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return same{rulestest.Scripted()}, f
				})
			})
			assert.Contains(t, msg, "differ", "the rejection names the property")
		})

		t.Run("accepts a pair differing below its first level", func(t *testing.T) {
			t.Parallel()

			rulestest.AssertDistinctSamples(t, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
				_, f := setup(tb)
				return nested{rulestest.Scripted()}, f
			})
		})
	})

	t.Run("AssertRefusesWithReason", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a refusal without a reason", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a sample that refuses in silence", func(tb assert.TB) {
				rulestest.AssertRefusesWithReason(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return silent{rulestest.Scripted()}, f
				})
			})
			assert.Contains(t, msg, "refusal", "the rejection names what is missing")
		})
	})

	t.Run("AssertTotal", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fold that returns a name", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a leaf that is not a leaf", func(tb assert.TB) {
				rulestest.AssertTotal(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return naming{rulestest.Scripted()}, f
				})
			})
			assert.Contains(t, msg, "leaf", "the rejection names the rule")
		})
	})

	t.Run("AssertWitnesses", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a derived witness without a spelling", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a witness no backend can write", func(tb assert.TB) {
				rulestest.AssertWitnesses(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return generic(rulestest.Scripted(), unspelled), f
				})
			})
			assert.Contains(t, msg, "spelling", "the rejection names what is missing")
		})

		t.Run("rejects a Substitute that rewrites its input", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a substitution in place", func(tb assert.TB) {
				rulestest.AssertWitnesses(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return generic(rulestest.Scripted(), inPlace), f
				})
			})
			assert.Contains(t, msg, "unchanged", "the rejection names the copy it owes")
		})

		t.Run("rejects a Substitute that leaves a parameter in place", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a substitution that rewrites nothing", func(tb assert.TB) {
				rulestest.AssertWitnesses(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return generic(rulestest.Scripted(), inert), f
				})
			})
			assert.Contains(t, msg, "becomes the parameter's witness", "the rejection names the rewrite")
		})

		t.Run("accepts a language without the generics capability", func(t *testing.T) {
			t.Parallel()

			rulestest.AssertWitnesses(t, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
				_, f := setup(tb)
				return same{rulestest.Scripted()}, f
			})
		})

		t.Run("accepts a fixture declaring no type parameter", func(t *testing.T) {
			t.Parallel()

			rulestest.AssertWitnesses(t, over(fstest.MapFS{storeFile: {Data: []byte(rowSource)}}))
		})

		t.Run("rejects a fixture whose type parameters no reference names", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "a generic type no field uses", func(tb assert.TB) {
				rulestest.AssertWitnesses(tb, over(fstest.MapFS{
					apiFile: {Data: []byte("package svc/api\ntype Box int\ntypeparam T\n")},
				}))
			})
			assert.Contains(t, msg, "proves nothing", "the rejection names why the fixture fails")
		})
	})

	t.Run("AssertRecorded", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects a fixture with no graph", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "nothing to read", func(tb assert.TB) {
				rulestest.AssertRecorded(tb, func(assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					return rulestest.Scripted(), &rulestest.Fixture{}
				})
			})
			assert.Contains(t, msg, "no graph", "the rejection names what the setup owes")
		})

		t.Run("rejects a language reading through a view of its own", func(t *testing.T) {
			t.Parallel()

			msg := assert.Rejects(t, "reads the handed view never records", func(tb assert.TB) {
				rulestest.AssertRecorded(tb, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
					_, f := setup(tb)
					return unrecorded{SourceRules: rulestest.Scripted(), graph: f.Graph}, f
				})
			})
			assert.Contains(t, msg, "the view handed to it", "the rejection names the view")
		})
	})
}

// over returns a setup loading the scripted rules over a tree of the
// case's own.
func over(tree fstest.MapFS) rulestest.Setup {
	return func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
		return rulestest.Scripted(), rulestest.Loaded(tb, frontendtest.NewScripted(), tree, frontendtest.ScriptedKeys)
	}
}

// same derives one value for both halves.
type same struct {
	rules.SourceRules
}

// SamplesOf returns the first half twice.
func (s same) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	sample, _ := s.SourceRules.SamplesOf(ref, hint, v)
	return sample, sample
}

// nested derives two composites that differ only in a field of a
// field, where a comparison one level down reads them as equal.
type nested struct {
	rules.SourceRules
}

// The texts the nested pair differs in, two levels down.
const (
	nestedSample    = "1"
	nestedAlternate = "2"
)

// SamplesOf returns two composites of one composite each.
func (nested) SamplesOf(ref *node.TypeRef, _ string, _ rules.View) (rules.Sample, rules.Sample) {
	outer := func(text string) rules.Sample {
		inner := emit.Composite(rules.EmitRef(ref), emit.NamedField("b", emit.Literal(emit.LiteralInt, text)))
		return rules.Of(emit.Composite(rules.EmitRef(ref), emit.NamedField("a", inner)))
	}
	return outer(nestedSample), outer(nestedAlternate)
}

// silent refuses without a reason.
type silent struct {
	rules.SourceRules
}

// SamplesOf returns two empty samples with no refusal.
func (silent) SamplesOf(*node.TypeRef, string, rules.View) (rules.Sample, rules.Sample) {
	return rules.Sample{}, rules.Sample{}
}

// naming folds a builtin to the named form, which no leaf is.
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

// Builtin returns an opaque leaf spelled with the call count.
func (r *restless) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	return rules.Opaque(&node.TypeRef{Spelling: ref.Spelling + strconv.FormatInt(r.calls.Add(1), 10)})
}

// unrecorded samples through a view it mints over its own read set,
// so no read records on the view it is handed.
type unrecorded struct {
	rules.SourceRules
	graph *store.Graph
}

// SamplesOf derives through a view of its own.
func (u unrecorded) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	reader, err := u.graph.Reader(store.NewReadSet(), nil)
	if err != nil {
		return rules.Refused(rules.RefusedNoView), rules.Refused(rules.RefusedNoView)
	}
	v.Decls = reader
	return u.SourceRules.SamplesOf(ref, hint, v)
}

// The generics defects a case can give the scripted language.
const (
	unspelled = iota + 1 // Derive reports a witness with no spelling
	inPlace              // Substitute rewrites the reference it is handed
	inert                // Substitute returns every reference as it is
)

// broken is the scripted language with one generics defect.
type broken struct {
	rules.SourceRules
	rules.GenericsRules
	defect int
}

// generic returns the scripted rules with one generics defect.
func generic(s rules.SourceRules, defect int) broken {
	return broken{SourceRules: s, GenericsRules: s.(rules.GenericsRules), defect: defect}
}

// Derive returns a reference with no spelling under the unspelled
// defect, and the scripted witness otherwise.
func (b broken) Derive(p *node.TypeParam, v rules.View) (*node.TypeRef, bool) {
	if b.defect == unspelled {
		return &node.TypeRef{}, true
	}
	return b.GenericsRules.Derive(p, v)
}

// Substitute rewrites in place under the inPlace defect, rewrites
// nothing under the inert one, and substitutes as the scripted
// language does otherwise.
func (b broken) Substitute(ref *node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) *node.TypeRef {
	switch b.defect {
	case inPlace:
		for i, p := range params {
			if ref != nil && p != nil && ref.Spelling == p.Name {
				ref.Spelling = args[i].Spelling
				ref.Target = symbol.Identity{}
			}
		}
		return ref
	case inert:
		return ref
	default:
		return b.GenericsRules.Substitute(ref, params, args)
	}
}
