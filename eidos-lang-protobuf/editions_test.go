// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"go.dokimi.dev/eidos/lang/protobuf"
)

// The features of descriptor.proto whose defaults the table states.
const (
	presenceFeature   = "field_presence"
	enumTypeFeature   = "enum_type"
	encodingFeature   = "message_encoding"
	visibilityFeature = "default_symbol_visibility"
)

// TestEditions pins the table of protobuf versions, the defaults of their
// features, and the resolution of a feature from the options of a
// declaration.
func TestEditions(t *testing.T) {
	t.Parallel()

	t.Run("EditionNamed", func(t *testing.T) {
		t.Parallel()

		versions := []struct {
			name       string
			give       string
			wantSyntax bool
			want       protobuf.Features
		}{
			{
				name: "returns proto2 under the defaults of descriptor.proto", give: protobuf.Proto2, wantSyntax: true,
				want: descriptorDefaults(t, descriptorpb.Edition_EDITION_PROTO2),
			},
			{
				name: "returns proto3 under the defaults of descriptor.proto", give: protobuf.Proto3, wantSyntax: true,
				want: descriptorDefaults(t, descriptorpb.Edition_EDITION_PROTO3),
			},
			{
				name: "returns edition 2023 under the defaults of descriptor.proto", give: protobuf.Edition2023,
				want: descriptorDefaults(t, descriptorpb.Edition_EDITION_2023),
			},
			{
				name: "returns edition 2024 under the defaults of descriptor.proto", give: protobuf.Edition2024,
				want: descriptorDefaults(t, descriptorpb.Edition_EDITION_2024),
			},
			{
				// descriptorpb at v1.36.12 predates Edition 2026, so its
				// defaults follow descriptor.proto of protobuf v36.0.
				name: "returns edition 2026 under the defaults of protobuf v36.0", give: protobuf.Edition2026,
				want: protobuf.Features{
					Presence: protobuf.PresenceExplicit, EnumType: protobuf.EnumOpen,
					Encoding: protobuf.EncodingLengthPrefixed, Visibility: protobuf.VisibilityStrict,
				},
			},
		}
		for _, tt := range versions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, known := protobuf.EditionNamed(tt.give)
				assert.True(t, known, "the table contains the version")
				assert.Equal(t, got, protobuf.Edition{Name: tt.give, Syntax: tt.wantSyntax, Defaults: tt.want},
					"the version with its statement and its defaults")
			})
		}

		unknown := []struct {
			name string
			give string
		}{
			{name: "reports false for an edition that protobuf did not release", give: "2025"},
			{name: "reports false for the edition in development", give: "UNSTABLE"},
			{name: "reports false for a syntax outside the table", give: "proto4"},
			{name: "reports false for an empty name", give: ""},
		}
		for _, tt := range unknown {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, known := protobuf.EditionNamed(tt.give)
				assert.False(t, known, "the table does not contain the version")
			})
		}
	})

	t.Run("Features", func(t *testing.T) {
		t.Parallel()

		base := protobuf.Features{
			Presence: protobuf.PresenceExplicit, EnumType: protobuf.EnumOpen,
			Encoding: protobuf.EncodingLengthPrefixed, Visibility: protobuf.VisibilityExportAll,
		}

		t.Run("Apply", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name      string
				giveName  string
				giveValue string
				want      protobuf.Features
			}{
				{
					name:      "sets field_presence from its own option",
					giveName:  "features.field_presence",
					giveValue: "IMPLICIT",
					want: protobuf.Features{
						Presence: protobuf.PresenceImplicit, EnumType: base.EnumType,
						Encoding: base.Encoding, Visibility: base.Visibility,
					},
				},
				{
					name: "sets enum_type from its own option", giveName: "features.enum_type", giveValue: "CLOSED",
					want: protobuf.Features{
						Presence: base.Presence, EnumType: protobuf.EnumClosed,
						Encoding: base.Encoding, Visibility: base.Visibility,
					},
				},
				{
					name: "sets message_encoding from its own option", giveName: "features.message_encoding",
					giveValue: "DELIMITED",
					want: protobuf.Features{
						Presence: base.Presence, EnumType: base.EnumType,
						Encoding: protobuf.EncodingDelimited, Visibility: base.Visibility,
					},
				},
				{
					name:      "sets default_symbol_visibility from its own option",
					giveName:  "features.default_symbol_visibility",
					giveValue: "LOCAL_ALL",
					want: protobuf.Features{
						Presence: base.Presence, EnumType: base.EnumType,
						Encoding: base.Encoding, Visibility: protobuf.VisibilityLocalAll,
					},
				},
				{
					name: "sets each feature of a message literal", giveName: "features",
					giveValue: "{ field_presence: IMPLICIT, enum_type: CLOSED; message_encoding: DELIMITED " +
						"default_symbol_visibility: STRICT }",
					want: protobuf.Features{
						Presence: protobuf.PresenceImplicit, EnumType: protobuf.EnumClosed,
						Encoding: protobuf.EncodingDelimited, Visibility: protobuf.VisibilityStrict,
					},
				},
				{
					name: "skips a field of a message literal whose value is a message", giveName: "features",
					giveValue: "{ [pb.cpp] { legacy_closed_enum: true enum_type: CLOSED } field_presence: IMPLICIT }",
					want: protobuf.Features{
						Presence: protobuf.PresenceImplicit, EnumType: base.EnumType,
						Encoding: base.Encoding, Visibility: base.Visibility,
					},
				},
				{
					name:      "skips a field of a message literal without a key",
					giveName:  "features",
					giveValue: "{ IMPLICIT }",
					want:      base,
				},
				{
					name: "skips a field of a message literal whose value is a string", giveName: "features",
					giveValue: `{ field_presence: "IMPLICIT" }`, want: base,
				},
				{
					name: "skips a string of a message literal past its escaped quote", giveName: "features",
					giveValue: `{ name: 'a\'} enum_type: CLOSED', field_presence: IMPLICIT }`,
					want: protobuf.Features{
						Presence: protobuf.PresenceImplicit, EnumType: base.EnumType,
						Encoding: base.Encoding, Visibility: base.Visibility,
					},
				},
				{
					name: "skips a string of a message literal that does not close", giveName: "features",
					giveValue: `{ name: "open field_presence: IMPLICIT }`, want: base,
				},
				{
					name: "skips an extension name of a message literal that does not close", giveName: "features",
					giveValue: "{ [pb.cpp field_presence: IMPLICIT }", want: base,
				},
				{
					name: "skips a features option whose value is no message literal", giveName: "features",
					giveValue: "IMPLICIT", want: base,
				},
				{name: "leaves the features of another option", giveName: "deprecated", giveValue: "true", want: base},
				{
					name: "leaves the features of another feature", giveName: "features.utf8_validation",
					giveValue: "NONE", want: base,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, base.Apply(tt.giveName, tt.giveValue), tt.want,
						"Apply returns the features after the option")
				})
			}
		})

		t.Run("LocalByDefault", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name       string
				give       protobuf.DefaultVisibility
				giveNested bool
				want       bool
			}{
				{
					name: "reports false for a top-level declaration under EXPORT_ALL",
					give: protobuf.VisibilityExportAll,
				},
				{
					name: "reports false for a nested declaration under EXPORT_ALL", give: protobuf.VisibilityExportAll,
					giveNested: true,
				},
				{
					name: "reports false for a top-level declaration under EXPORT_TOP_LEVEL",
					give: protobuf.VisibilityExportTopLevel,
				},
				{
					name: "reports true for a nested declaration under EXPORT_TOP_LEVEL",
					give: protobuf.VisibilityExportTopLevel, giveNested: true, want: true,
				},
				{
					name: "reports true for a top-level declaration under LOCAL_ALL", give: protobuf.VisibilityLocalAll,
					want: true,
				},
				{
					name: "reports true for a top-level declaration under STRICT",
					give: protobuf.VisibilityStrict,
					want: true,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					f := protobuf.Features{Visibility: tt.give}
					assert.Equal(t, f.LocalByDefault(tt.giveNested), tt.want,
						"LocalByDefault reports whether the default visibility makes the declaration local")
				})
			}
		})
	})

	t.Run("Edition", func(t *testing.T) {
		t.Parallel()

		names := []struct {
			name string
			give string
			want string
		}{
			{name: "spells proto2", give: protobuf.Proto2, want: "proto2"},
			{name: "spells proto3", give: protobuf.Proto3, want: "proto3"},
			{name: "spells edition 2023", give: protobuf.Edition2023, want: "2023"},
			{name: "spells edition 2024", give: protobuf.Edition2024, want: "2024"},
			{name: "spells edition 2026", give: protobuf.Edition2026, want: "2026"},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give, tt.want, "the value of the version statement")
			})
		}
	})
}

// TestEditionsAllocs checks that a lookup and a resolution allocate
// nothing, in the ordinary run, which runs no benchmark.
func TestEditionsAllocs(t *testing.T) {
	var (
		e     protobuf.Edition
		known bool
	)
	assert.MaxAllocs(t, func() { e, known = protobuf.EditionNamed(protobuf.Edition2026) }, 0,
		"EditionNamed allocates nothing")
	assert.True(t, known, "the table contains edition 2026")

	var f protobuf.Features
	literal := "{ field_presence: IMPLICIT enum_type: CLOSED }"
	assert.MaxAllocs(t, func() { f = e.Defaults.Apply(protobuf.FeaturesOption, literal) }, 0,
		"Apply allocates nothing")
	assert.Equal(t, f.EnumType, protobuf.EnumClosed, "Apply sets the enum type of the literal")
}

// BenchmarkEditions measures the lookup of the last version and the
// resolution of a message literal of two features.
func BenchmarkEditions(b *testing.B) {
	b.Run("EditionNamed", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var known bool
		for c.Loop() {
			_, known = protobuf.EditionNamed(protobuf.Edition2026)
		}
		assert.True(b, known, "the table contains edition 2026")
	})

	b.Run("Features", func(b *testing.B) {
		b.Run("Apply", func(b *testing.B) {
			base, _ := protobuf.EditionNamed(protobuf.Edition2023)
			literal := "{ field_presence: IMPLICIT enum_type: CLOSED }"
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var f protobuf.Features
			for c.Loop() {
				f = base.Defaults.Apply(protobuf.FeaturesOption, literal)
			}
			assert.Equal(b, f.Presence, protobuf.PresenceImplicit, "Apply sets the presence of the literal")
		})
	})
}

// descriptorDefaults returns the defaults of the four features that the
// table states, at one edition. It reads them from the edition defaults
// that protobuf-go's descriptorpb declares on the fields of FeatureSet, and
// takes the last default at or before the edition.
func descriptorDefaults(tb assert.TB, edition descriptorpb.Edition) protobuf.Features {
	tb.Helper()

	fields := (&descriptorpb.FeatureSet{}).ProtoReflect().Descriptor().Fields()
	values := map[string]string{}
	for _, feature := range []string{presenceFeature, enumTypeFeature, encodingFeature, visibilityFeature} {
		field := fields.ByName(protoreflect.Name(feature))
		assert.NotNil(tb, field, "descriptorpb declares the feature "+feature)
		options, _ := field.Options().(*descriptorpb.FieldOptions)
		for _, d := range options.GetEditionDefaults() {
			if d.GetEdition() <= edition {
				values[feature] = d.GetValue()
			}
		}
	}
	return protobuf.Features{
		Presence:   protobuf.Presence(values[presenceFeature]),
		EnumType:   protobuf.EnumType(values[enumTypeFeature]),
		Encoding:   protobuf.Encoding(values[encodingFeature]),
		Visibility: protobuf.DefaultVisibility(values[visibilityFeature]),
	}
}
