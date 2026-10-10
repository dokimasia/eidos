// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package matrix_test

import (
	"testing"

	"go.dokimi.dev/assert"

	gocorpus "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/conformance/matrix"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The rows of the hub that the cases expect from the satellites. Each
// spelling is the one that the spoke of its target declares for the form.
const (
	int64Row    = "| Scalar int, 64 bits | `int64` | `bigint` | `long` | `i64` |\n"
	argumentRow = "| Reference with a type argument | `Session[string]` | `Session<string>` | " +
		"`Session<String>` | `Session<String>` |\n"
	inlineRow    = "| Inline type | refused | refused | refused | refused |\n"
	int64Choices = "| typescript.int64: bigint | none | `bigint` | none | none |\n" +
		"| typescript.int64: string | none | `string` | none | none |\n" +
		"| typescript.int64: number | none | `number` | none | none |\n"
	unspelledRow = "| Bool | none |\n"
)

// The rows of the top type, the empty value, the asynchronous stream and
// the choices of the duration's policy.
const (
	dynamicRow = "| Dynamic | `any` | `unknown` | `Object` | refused |\n"
	emptyRow   = "| Empty | `struct{}` | `Record<string, never>` | refused | `()` |\n"
	asyncRow   = "| Asynchronous stream of text | `iter.Seq2[string, error]` | `AsyncIterable<string>` | " +
		"refused | refused |\n"
	durationChoices = "| typescript.duration: string | none | `string` | none | none |\n" +
		"| typescript.duration: number | none | `number` | none | none |\n"
)

// The policy of the stub target that the cases declare, and the error of
// a policy without a probe.
const (
	widthKey    plugin.PolicyKey = "stub.width"
	narrow      plugin.Choice    = "narrow"
	wide        plugin.Choice    = "wide"
	widthDoc                     = "the width of a number"
	unprobedErr                  = "matrix: the policy stub.width of target stub has no probe"
)

// policyBackend is a backend of the target stub that declares the
// policies specs.
type policyBackend struct {
	bareBackend
	specs []plugin.PolicySpec
}

// Policies returns the backend's policies.
func (b policyBackend) Policies() []plugin.PolicySpec {
	return b.specs
}

func TestHub(t *testing.T) {
	t.Parallel()

	t.Run("Markdown", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the spelling of each probe under the default policy of each target", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Contains(t, doc, int64Row, "TypeScript spells a 64-bit integer under its default policy")
		})
		t.Run("writes the spelling of each choice of a policy in the column of its target", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Contains(t, doc, int64Choices, "the rows of typescript.int64 contain the spellings of its choices")
		})
		t.Run("writes the type arguments of a reference in the brackets of each target", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Contains(t, doc, argumentRow, "Go writes square brackets, and the other targets angle brackets")
		})
		t.Run("writes refused for a form that a spoke refuses", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Contains(t, doc, inlineRow, "no target spells an inline type")
		})
		rows := []struct {
			name string
			want string
		}{
			{name: "writes the top type of each target", want: dynamicRow},
			{name: "writes the empty value of each target", want: emptyRow},
			{name: "writes the asynchronous stream of each target", want: asyncRow},
			{name: "writes the spelling of each choice of the duration's policy", want: durationChoices},
		}
		for _, tt := range rows {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				doc, err := matrix.Markdown(matrix.Entries())
				assert.NoError(t, err, "the matrix of the satellites renders")
				assert.Contains(t, doc, tt.want, "the hub has the row of the spokes' spellings")
			})
		}
		t.Run("writes none in the column of a backend without a spoke", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Backend: bareBackend{}}})
			assert.NoError(t, err, "the matrix of a backend without a spoke renders")
			assert.Contains(t, doc, unspelledRow, "a backend without a spoke does not spell a form")
		})
		t.Run("returns an error for a policy without a probe", func(t *testing.T) {
			t.Parallel()

			backend := policyBackend{specs: []plugin.PolicySpec{
				{Key: widthKey, Choices: []plugin.Choice{narrow, wide}, Default: narrow, Doc: widthDoc},
			}}
			_, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Backend: backend}})
			assert.HasError(t, err, "a policy without a probe fails the matrix")
			assert.Equal(t, err.Error(), unprobedErr, "the error contains the policy and its target")
		})
		t.Run("returns the error of policies that do not resolve", func(t *testing.T) {
			t.Parallel()

			specs := []plugin.PolicySpec{{Key: widthKey, Default: narrow, Doc: widthDoc}}
			_, want := plugin.NewPolicy(bareBackend{}.Target(), specs, nil)
			assert.HasError(t, want, "a policy without choices does not resolve")
			backend := policyBackend{specs: specs}
			_, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Backend: backend}})
			assert.Equal(t, err, want, "the matrix returns the error that NewPolicy returns for the specs")
		})
	})
}
