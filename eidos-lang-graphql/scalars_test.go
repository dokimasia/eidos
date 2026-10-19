// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The arguments of the annotation specifiedBy for the specifications of
// scalars.graphql.org that the table contains, and for one that it does
// not contain.
const (
	dateTimeURL     = `url: "https://scalars.graphql.org/andimarek/date-time.html"`
	durationURL     = `url: "https://scalars.graphql.org/chillicream/duration.html"`
	longURL         = `url: "https://scalars.graphql.org/chillicream/long.html"`
	unsignedIntURL  = `url: "https://scalars.graphql.org/chillicream/unsigned-int.html"`
	unsignedLongURL = `url: "https://scalars.graphql.org/chillicream/unsigned-long.html"`
	base64URL       = `url: "https://scalars.graphql.org/chillicream/base64-string.html"`
	anyURL          = `url: "https://scalars.graphql.org/chillicream/any.html"`
	uuidURL         = `url: "https://scalars.graphql.org/chillicream/uuid.html"`
)

// scalarAllocs is the declaration of a scalar of the table: the alias, its
// list of annotations and the list of arguments.
const scalarAllocs = 3

// TestScalars pins the built-in scalars and the table of the custom scalars
// of scalars.graphql.org.
func TestScalars(t *testing.T) {
	t.Parallel()

	t.Run("IsBuiltin", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for Int", give: "Int", want: true},
			{name: "reports true for Float", give: "Float", want: true},
			{name: "reports true for String", give: "String", want: true},
			{name: "reports true for Boolean", give: "Boolean", want: true},
			{name: "reports true for ID", give: "ID", want: true},
			{name: "reports false for a scalar of the table", give: string(graphql.Long)},
			{name: "reports false for a spelling in another case", give: "int"},
			{name: "reports false for the empty spelling", give: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, graphql.IsBuiltin(tt.give), tt.want, "IsBuiltin reports the five scalars of GraphQL")
			})
		}
	})

	t.Run("ScalarShape", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      symbol.Annotations
			wantShape rules.TypeShape
			wantFound bool
		}{
			{
				name:      "returns the timestamp for the specification of a date and a time",
				give:      symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{dateTimeURL}}},
				wantShape: rules.Reference(string(graphql.DateTime), rules.WellKnownTimestamp),
				wantFound: true,
			},
			{
				name:      "returns the duration for the specification of a duration",
				give:      symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{durationURL}}},
				wantShape: rules.Reference(string(graphql.Duration), rules.WellKnownDuration),
				wantFound: true,
			},
			{
				name:      "returns a signed integer of 64 bits for the specification of a long",
				give:      symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{longURL}}},
				wantShape: rules.Scalar(string(graphql.Long), rules.ScalarInt, 64),
				wantFound: true,
			},
			{
				name:      "returns an unsigned integer of 32 bits for the specification of an unsigned int",
				give:      symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{unsignedIntURL}}},
				wantShape: rules.Scalar(string(graphql.UnsignedInt), rules.ScalarUint, 32),
				wantFound: true,
			},
			{
				name:      "returns an unsigned integer of 64 bits for the specification of an unsigned long",
				give:      symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{unsignedLongURL}}},
				wantShape: rules.Scalar(string(graphql.UnsignedLong), rules.ScalarUint, 64),
				wantFound: true,
			},
			{
				name:      "returns bytes for the specification of a base64 string",
				give:      symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{base64URL}}},
				wantShape: rules.Leaf(symbol.FormBytes, string(graphql.Base64String)),
				wantFound: true,
			},
			{
				name:      "returns a value of any shape for the specification of any",
				give:      symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{anyURL}}},
				wantShape: rules.Leaf(symbol.FormDynamic, string(graphql.Any)),
				wantFound: true,
			},
			{
				name: "returns the shape of the annotation specifiedBy after another annotation",
				give: symbol.Annotations{
					{Name: graphql.DeprecatedDirective},
					{Name: graphql.SpecifiedByDirective, Args: []string{longURL}},
				},
				wantShape: rules.Scalar(string(graphql.Long), rules.ScalarInt, 64),
				wantFound: true,
			},
			{
				name: "reports false for a specification outside the table",
				give: symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{uuidURL}}},
			},
			{
				name: "reports false for an annotation specifiedBy without an argument",
				give: symbol.Annotations{{Name: graphql.SpecifiedByDirective}},
			},
			{
				name: "reports false for the URL of the table on another annotation",
				give: symbol.Annotations{{Name: graphql.DeprecatedDirective, Args: []string{longURL}}},
			},
			{name: "reports false for no annotation"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				shape, found := graphql.ScalarShape(tt.give)
				assert.Equal(t, found, tt.wantFound, "ScalarShape reports whether the table contains the URL")
				assert.Equal(t, shape, tt.wantShape, "the shape of the entry of the table")
			})
		}
	})

	t.Run("Scalar", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    plugin.Choice
			wantArg string
		}{
			{
				name: "returns DateTime with the specification of a date and a time", give: graphql.DateTime,
				wantArg: dateTimeURL,
			},
			{
				name: "returns Duration with the specification of a duration", give: graphql.Duration,
				wantArg: durationURL,
			},
			{name: "returns Long with the specification of a long", give: graphql.Long, wantArg: longURL},
			{
				name: "returns UnsignedInt with the specification of an unsigned int", give: graphql.UnsignedInt,
				wantArg: unsignedIntURL,
			},
			{
				name: "returns UnsignedLong with the specification of an unsigned long", give: graphql.UnsignedLong,
				wantArg: unsignedLongURL,
			},
			{
				name: "returns Base64String with the specification of a base64 string", give: graphql.Base64String,
				wantArg: base64URL,
			},
			{name: "returns Any with the specification of any", give: graphql.Any, wantArg: anyURL},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, found := graphql.Scalar(string(tt.give))
				assert.True(t, found, "the table contains the name")
				assert.Equal(t, got, &emit.Alias{
					Name: string(tt.give), Defined: true,
					Annotations: symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{tt.wantArg}}},
				}, "a defined alias without a target, with the annotation specifiedBy")
			})
		}

		misses := []struct {
			name string
			give string
		}{
			{name: "reports false for a scalar outside the table", give: "UUID"},
			{name: "reports false for a built-in scalar", give: graphql.IntScalar},
		}
		for _, tt := range misses {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, found := graphql.Scalar(tt.give)
				assert.False(t, found, "the table does not contain the name")
				assert.Nil(t, got, "a name outside the table has no declaration")
			})
		}

		t.Run("returns a declaration whose annotations the table projects", func(t *testing.T) {
			t.Parallel()

			alias, found := graphql.Scalar(string(graphql.UnsignedLong))
			assert.True(t, found, "the table contains UnsignedLong")
			shape, found := graphql.ScalarShape(alias.Annotations)
			assert.True(t, found, "the annotations of the declaration state a URL of the table")
			assert.Equal(t, shape, rules.Scalar(string(graphql.UnsignedLong), rules.ScalarUint, 64),
				"the declaration projects as the shape of its entry")
		})
	})
}

// TestScalarsAllocs checks the ceilings of the scalars in the ordinary run,
// which does not run benchmarks.
func TestScalarsAllocs(t *testing.T) {
	t.Run("IsBuiltin", func(t *testing.T) {
		spelling := graphql.IDScalar
		builtin := false
		assert.MaxAllocs(t, func() { builtin = graphql.IsBuiltin(spelling) }, 0, "IsBuiltin does not allocate")
		assert.True(t, builtin, "ID is built in")
	})

	t.Run("ScalarShape", func(t *testing.T) {
		annotations := symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{anyURL}}}
		found := false
		assert.MaxAllocs(t, func() { _, found = graphql.ScalarShape(annotations) }, 0,
			"ScalarShape does not allocate")
		assert.True(t, found, "the table contains the URL of the last entry")
	})

	t.Run("Scalar", func(t *testing.T) {
		var got *emit.Alias
		assert.MaxAllocs(t, func() { got, _ = graphql.Scalar(string(graphql.Any)) }, scalarAllocs,
			"Scalar allocates the alias, its annotations and their arguments")
		assert.NotNil(t, got, "the table contains Any")
	})
}

// BenchmarkScalars measures the check of a built-in scalar, which the
// frontend runs for each reference, the projection of a custom scalar and
// the declaration of a scalar of the table.
func BenchmarkScalars(b *testing.B) {
	b.Run("IsBuiltin", func(b *testing.B) {
		spelling := graphql.IDScalar
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		builtin := false
		for c.Loop() {
			builtin = graphql.IsBuiltin(spelling)
		}
		assert.True(b, builtin, "ID is built in")
	})

	b.Run("ScalarShape", func(b *testing.B) {
		annotations := symbol.Annotations{{Name: graphql.SpecifiedByDirective, Args: []string{anyURL}}}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		found := false
		for c.Loop() {
			_, found = graphql.ScalarShape(annotations)
		}
		assert.True(b, found, "the table contains the URL of the last entry")
	})

	b.Run("Scalar", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(scalarAllocs)
		defer c.End()
		var got *emit.Alias
		for c.Loop() {
			got, _ = graphql.Scalar(string(graphql.Any))
		}
		assert.NotNil(b, got, "the table contains Any")
	})
}
