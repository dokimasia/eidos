// Copyright Dokimasia B.V. 2026
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
	int64Row    = "| Scalar int, 64 bits | `int64` | `bigint` | `long` | `i64` | `int64` | `String` | `int` |\n"
	argumentRow = "| Reference with a type argument | `Session[string]` | `Session<string>` | " +
		"`Session<String>` | `Session<String>` | refused | refused | `Session[str]` |\n"
	inlineRow    = "| Inline type | refused | refused | refused | refused | refused | refused | refused |\n"
	traitRow     = "| Reference to an interface | `Session` | `Session` | `Session` | `Box<dyn Session>` |"
	int64Choices = "| typescript.int64: bigint | none | `bigint` | none | none | none | none | none |\n" +
		"| typescript.int64: string | none | `string` | none | none | none | none | none |\n" +
		"| typescript.int64: number | none | `number` | none | none | none | none | none |\n"
	unspelledRow = "| Bool | refused |\n"
)

// The rows of the top type, the empty value, the error value, the
// asynchronous stream and the choices of the duration's policy and of the
// encoding of a 32-bit integer of protobuf.
const (
	dynamicRow = "| Dynamic | `any` | `unknown` | `Object` | refused | `google.protobuf.Value` | `String` | " +
		"`object` |\n"
	emptyRow = "| Empty | `struct{}` | `Record<string, never>` | refused | `()` | `google.protobuf.Empty` | " +
		"refused | `None` |\n"
	errorRow = "| Error | `error` | `Error` | `Exception` | `Box<dyn std::error::Error>` | refused | refused | " +
		"`Exception` |\n"
	asyncRow = "| Asynchronous stream of text | `iter.Seq2[string, error]` | `AsyncIterable<string>` | " +
		"refused | refused | `stream string` | `String!` | `AsyncIterator[str]` |\n"
	durationChoices = "| typescript.duration: string | none | `string` | none | none | none | none | none |\n" +
		"| typescript.duration: number | none | `number` | none | none | none | none | none |\n"
	int32Choices = "| protobuf.int32: int32 | none | none | none | none | `int32` | none | none |\n" +
		"| protobuf.int32: sint32 | none | none | none | none | `sint32` | none | none |\n" +
		"| protobuf.int32: sfixed32 | none | none | none | none | `sfixed32` | none | none |\n"
)

// The rows of the integers of 8 and 16 bits and of the float of 32 bits.
const (
	int8Row    = "| Scalar int, 8 bits | `int8` | `number` | `byte` | `i8` | `int32` | `Int` | `int` |\n"
	int16Row   = "| Scalar int, 16 bits | `int16` | `number` | `short` | `i16` | `int32` | `Int` | `int` |\n"
	uint8Row   = "| Scalar uint, 8 bits | `uint8` | `number` | `byte` | `u8` | `uint32` | `Int` | `int` |\n"
	uint16Row  = "| Scalar uint, 16 bits | `uint16` | `number` | `short` | `u16` | `uint32` | `Int` | `int` |\n"
	float32Row = "| Scalar float, 32 bits | `float32` | `number` | `float` | `f32` | `float` | `Float` | `float` |\n"
)

// The rows of the choices of the GraphQL type of a 64-bit integer, and the
// rows of the map, whose list of entries the lowering of the GraphQL backend
// names after its field. The spoke leaves the map of entries without a
// spelling, so the cell is the stand-in of the hub.
const (
	graphqlInt64Choices = "| graphql.int64: String | none | none | none | none | none | `String` | none |\n" +
		"| graphql.int64: Float | none | none | none | none | none | `Float` | none |\n" +
		"| graphql.int64: Long | none | none | none | none | none | `Long` | none |\n"
	mapRow = "| Map of text to text | `map[string]string` | `Record<string, string>` | `Map<String, String>` | " +
		"`HashMap<String, String>` | `map<string, string>` | `?` | `dict[str, str]` |\n"
	graphqlMapChoices = "| graphql.map: entries | none | none | none | none | none | `?` | none |\n" +
		"| graphql.map: Any | none | none | none | none | none | `Any` | none |\n"
)

// The rows of the choices of the type of a dynamic value of Python, and of
// the first version of Python, whose syntax decides no spelling of a type.
const (
	pythonDynamicChoices = "| python.dynamic: object | none | none | none | none | none | none | `object` |\n" +
		"| python.dynamic: Any | none | none | none | none | none | none | `Any` |\n"
	pythonVersionRow = "| python.version: 3.11 | none | none | none | none | none | none | `str \\| None` |\n"
)

// The rows of a type parameter and of a list of a type parameter. Protobuf
// and GraphQL have no type parameters.
const (
	paramRow     = "| Type parameter | `T` | `T` | `T` | `T` | refused | refused | `T` |\n"
	paramListRow = "| List of a type parameter | `[]T` | `T[]` | `List<T>` | `Vec<T>` | refused | refused | " +
		"`list[T]` |\n"
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
		t.Run("writes the owned trait object of an interface for Rust", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown(matrix.Entries())
			assert.NoError(t, err, "the matrix of the satellites renders")
			assert.Contains(t, doc, traitRow, "Rust spells a value of an interface as a boxed trait object")
		})
		rows := []struct {
			name string
			want string
		}{
			{name: "writes the top type of each target", want: dynamicRow},
			{name: "writes the empty value of each target", want: emptyRow},
			{name: "writes the error value of each target", want: errorRow},
			{name: "writes the asynchronous stream of each target", want: asyncRow},
			{name: "writes the spelling of each choice of the duration's policy", want: durationChoices},
			{name: "writes the spelling of each encoding of a 32-bit integer of protobuf", want: int32Choices},
			{name: "writes the integer of 8 bits of each target", want: int8Row},
			{name: "writes the integer of 16 bits of each target", want: int16Row},
			{name: "writes the unsigned integer of 8 bits of each target", want: uint8Row},
			{name: "writes the unsigned integer of 16 bits of each target", want: uint16Row},
			{name: "writes the float of 32 bits of each target", want: float32Row},
			{name: "writes the spelling of each GraphQL type of a 64-bit integer", want: graphqlInt64Choices},
			{name: "writes the map of each target", want: mapRow},
			{name: "writes the spelling of each GraphQL type of a map", want: graphqlMapChoices},
			{name: "writes the spelling of each type of a dynamic value of Python", want: pythonDynamicChoices},
			{name: "writes the optional of text for a version of Python", want: pythonVersionRow},
			{name: "writes the name of a type parameter in each target with type parameters", want: paramRow},
			{name: "writes a list of a type parameter in each target with type parameters", want: paramListRow},
		}
		for _, tt := range rows {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				doc, err := matrix.Markdown(matrix.Entries())
				assert.NoError(t, err, "the matrix of the satellites renders")
				assert.Contains(t, doc, tt.want, "the hub has the row of the spokes' spellings")
			})
		}
		t.Run("writes refused in the column of a backend that spells no type of another language", func(t *testing.T) {
			t.Parallel()

			doc, err := matrix.Markdown([]matrix.Entry{{Corpus: gocorpus.Corpus(nil), Backend: bareBackend{}}})
			assert.NoError(t, err, "the matrix of a backend whose spoke refuses every shape renders")
			assert.Contains(t, doc, unspelledRow, "the spoke refuses every form")
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
