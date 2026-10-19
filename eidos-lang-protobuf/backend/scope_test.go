// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The features that the scope cases state, with the values that they set.
var (
	implicitPresence = symbol.Annotation{
		Name: protobuf.FeaturePresence,
		Args: []string{string(protobuf.PresenceImplicit)},
	}
	closedEnum = symbol.Annotation{Name: protobuf.FeatureEnumType, Args: []string{string(protobuf.EnumClosed)}}
)

// TestScope pins the version of a file that its header states, and the
// features that apply inside each declaration of the file.
func TestScope(t *testing.T) {
	t.Parallel()

	t.Run("FileScope", func(t *testing.T) {
		t.Parallel()

		versions := []struct {
			name string
			give *emit.File
			want string
		}{
			{name: "returns proto3 for a header without a syntax marker", give: &emit.File{}, want: protobuf.Proto3},
			{name: "returns proto2 for its syntax marker", give: proto2File, want: protobuf.Proto2},
			{name: "returns proto3 for its syntax marker", give: proto3File, want: protobuf.Proto3},
			{name: "returns edition 2023 for its syntax marker", give: edition2023File, want: protobuf.Edition2023},
			{name: "returns edition 2024 for its syntax marker", give: edition2024File, want: protobuf.Edition2024},
			{name: "returns edition 2026 for its syntax marker", give: edition2026File, want: protobuf.Edition2026},
			{
				name: "returns one version for a header that states it twice",
				give: &emit.File{Annotations: symbol.Annotations{
					{Name: syntaxName, Args: []string{protobuf.Edition2023}},
					{Name: syntaxName, Args: []string{protobuf.Edition2023}},
				}},
				want: protobuf.Edition2023,
			},
		}
		for _, tt := range versions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.FileScope(tt.give)
				assert.NoError(t, err, "the header states a version of the table")
				assert.Equal(t, got.Version().Name, tt.want, "the version of the file")
				want, _ := protobuf.EditionNamed(tt.want)
				assert.Equal(t, got.Features(), want.Defaults, "the defaults of the version")
				assert.False(t, got.Nested(), "the top level of a file is no body of a message")
			})
		}

		t.Run("returns the features of the header applied to the defaults of the edition", func(t *testing.T) {
			t.Parallel()

			got, err := backend.FileScope(&emit.File{Annotations: symbol.Annotations{
				{Name: syntaxName, Args: []string{protobuf.Edition2023}}, implicitPresence,
			}})
			assert.NoError(t, err, "an edition states features")
			assert.Equal(t, got.Features().Presence, protobuf.PresenceImplicit, "the feature of the header applies")
		})

		refusals := []struct {
			name string
			give *emit.File
			want string
		}{
			{
				name: "returns an error for a version outside the table",
				give: &emit.File{Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{"2025"}}}},
				want: "protobuf: the header states the version 2025, which protobuf does not have",
			},
			{
				name: "returns an error for two versions",
				give: &emit.File{Annotations: symbol.Annotations{
					{
						Name: syntaxName,
						Args: []string{protobuf.Proto3},
					},
					{Name: syntaxName, Args: []string{protobuf.Proto2}},
				}},
				want: "protobuf: the header states the versions proto3 and proto2, and a file has one version",
			},
			{
				name: "returns an error for a syntax marker without an argument",
				give: &emit.File{Annotations: symbol.Annotations{{Name: syntaxName}}},
				want: "protobuf: the marker protobuf:syntax takes one argument, and it states 0",
			},
			{
				name: "returns an error for a feature in a file of proto3",
				give: &emit.File{
					Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{protobuf.Proto3}}, closedEnum},
				},
				want: "protobuf: the header states the feature features.enum_type, and proto3 has no features",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.FileScope(tt.give)
				assert.HasError(t, err, "the header states no version of a file")
				assert.Equal(t, err.Error(), tt.want, "the error states the reason")
			})
		}
	})

	t.Run("Within", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the features that a declaration states applied to its scope", func(t *testing.T) {
			t.Parallel()

			file := fileScope(t, edition2023File)
			got, err := file.Within(symbol.Annotations{closedEnum})
			assert.NoError(t, err, "a declaration of an edition states features")
			assert.Equal(t, got.Features().EnumType, protobuf.EnumClosed, "the feature of the declaration applies")
			assert.Equal(t, got.Features().Presence, file.Features().Presence, "the other features stay")
			assert.True(t, got.Nested(), "the body of a declaration is nested")
		})

		t.Run("returns the features of a message literal", func(t *testing.T) {
			t.Parallel()

			file := fileScope(t, edition2024File)
			got, err := file.Within(symbol.Annotations{
				{Name: protobuf.FeaturesOption, Args: []string{"{ field_presence: IMPLICIT }"}},
			})
			assert.NoError(t, err, "a declaration of an edition states features")
			assert.Equal(t, got.Features().Presence, protobuf.PresenceImplicit, "the field of the literal applies")
		})

		t.Run("returns the scope for a declaration without features", func(t *testing.T) {
			t.Parallel()

			file := fileScope(t, proto2File)
			got, err := file.Within(symbol.Annotations{{Name: "deprecated", Args: []string{"true"}}})
			assert.NoError(t, err, "an option is no feature")
			assert.Equal(t, got.Features(), file.Features(), "the features of the file apply")
			assert.Equal(t, got.Version(), file.Version(), "the version of the file applies")
		})

		t.Run("returns an error for a feature in a file of proto2", func(t *testing.T) {
			t.Parallel()

			file := fileScope(t, proto2File)
			_, err := file.Within(symbol.Annotations{implicitPresence})
			assert.HasError(t, err, "proto2 has no features")
			assert.Equal(t, err.Error(),
				"protobuf: a declaration states the feature features.field_presence, and proto2 has no features",
				"the error states the reason")
		})

		t.Run("returns an error for a feature without an argument", func(t *testing.T) {
			t.Parallel()

			file := fileScope(t, edition2023File)
			_, err := file.Within(symbol.Annotations{{Name: protobuf.FeatureEnumType}})
			assert.HasError(t, err, "a feature has a value")
			assert.Equal(t, err.Error(),
				"protobuf: the feature features.enum_type takes one argument, and it states 0",
				"the error states the reason")
		})
	})
}

// FileScope and Within read the annotations of a header without
// allocating. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestScopeAllocs(t *testing.T) {
	h := &emit.File{
		Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{protobuf.Edition2024}}, implicitPresence},
	}
	as := symbol.Annotations{closedEnum}
	var (
		file, inner backend.Scope
		err         error
	)
	assert.MaxAllocs(t, func() { file, err = backend.FileScope(h) }, 0, "FileScope allocates nothing")
	assert.NoError(t, err, "FileScope reads the header")
	assert.MaxAllocs(t, func() { inner, err = file.Within(as) }, 0, "Within allocates nothing")
	assert.NoError(t, err, "Within reads the annotations")
	assert.Equal(t, inner.Features().EnumType, protobuf.EnumClosed, "Within applies the feature")
}

// BenchmarkScope measures the scope that every template of a file reads.
func BenchmarkScope(b *testing.B) {
	h := &emit.File{
		Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{protobuf.Edition2024}}, implicitPresence},
	}
	b.Run("FileScope", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got backend.Scope
			err error
		)
		for c.Loop() {
			got, err = backend.FileScope(h)
		}
		assert.NoError(b, err, "FileScope reads the header")
		assert.Equal(b, got.Features().Presence, protobuf.PresenceImplicit, "FileScope applies the feature")
	})

	b.Run("Within", func(b *testing.B) {
		file := fileScope(b, h)
		as := symbol.Annotations{closedEnum}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got backend.Scope
			err error
		)
		for c.Loop() {
			got, err = file.Within(as)
		}
		assert.NoError(b, err, "Within reads the annotations")
		assert.Equal(b, got.Features().EnumType, protobuf.EnumClosed, "Within applies the feature")
	})
}
