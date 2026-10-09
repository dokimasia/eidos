// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The texts of the sections of the fixture documents.
const (
	observationText    = "an observation"
	falsifiabilityText = "a falsifiability"
	edgeText           = "an edge"
)

// A document decodes every section of a spec of its form under the key
// that the spec files write.
func TestDocument(t *testing.T) {
	t.Parallel()

	t.Run("Shape", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes every section of a shape", func(t *testing.T) {
			t.Parallel()

			got := decodeAs[specfront.Shape](t, shapeBase+
				"detected: true\n"+
				"params:\n  - {key: sample, type: string, doc: a param}\n"+
				"bindings:\n  value: {from: input, doc: a binding}\n"+
				"precedence:\n  yields_to: [writer]\n")
			assert.Equal(t, got, specfront.Shape{
				Name:           probeName,
				Form:           specfront.FormShape,
				Detected:       true,
				Claim:          claimText,
				Observation:    observationText,
				Falsifiability: falsifiabilityText,
				Params:         []specfront.Param{{Key: sampleKey, Type: specfront.TypeString, Doc: paramDoc}},
				Bindings: map[specfront.Name]specfront.Binding{
					valueKey: {From: specfront.SourceInput, Doc: bindingDoc},
				},
				Counterexamples: specfront.Counterexamples{Edge: edgeText},
				Precedence:      &specfront.Precedence{YieldsTo: []specfront.Name{writerName}},
			}, "the document has every section of the file")
		})
	})

	t.Run("Mixin", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes every section of a mixin", func(t *testing.T) {
			t.Parallel()

			got := decodeAs[specfront.Mixin](t, mixinBase+
				"documentary: true\n"+
				"params:\n  - {key: sample, type: string, doc: a param}\n")
			assert.Equal(t, got, specfront.Mixin{
				Name: probeName, Form: specfront.FormMixin, Documentary: true, Claim: claimText,
				Observation: observationText, Falsifiability: falsifiabilityText,
				Params:          []specfront.Param{{Key: sampleKey, Type: specfront.TypeString, Doc: paramDoc}},
				Counterexamples: specfront.Counterexamples{Edge: edgeText},
			}, "the document has every section of the file")
		})
	})

	t.Run("Contract", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes every section of a contract", func(t *testing.T) {
			t.Parallel()

			got := decodeAs[specfront.Contract](t, contractBase+
				"documentary: true\n"+
				"params:\n  - {key: sample, type: string, doc: a param}\n")
			assert.Equal(t, got, specfront.Contract{
				Name: probeName, Form: specfront.FormContract, Documentary: true, Claim: claimText,
				Observation: observationText, Falsifiability: falsifiabilityText,
				Params: []specfront.Param{{Key: sampleKey, Type: specfront.TypeString, Doc: paramDoc}},
				Roles: map[specfront.Name]specfront.RoleArity{
					beginRole: {Arity: specfront.ArityOne}, endRole: {Arity: specfront.ArityOptional},
				},
				Counterexamples: specfront.Counterexamples{Edge: edgeText},
			}, "the document has every section of the file")
		})
	})
}

// decodeAs decodes a YAML text into a value of T, and fails the test on
// an error of the decoder.
func decodeAs[T any](tb testing.TB, text string) T {
	tb.Helper()

	var v T
	assert.NoError(tb, yaml.Unmarshal([]byte(text), &v), "the text decodes")
	return v
}
