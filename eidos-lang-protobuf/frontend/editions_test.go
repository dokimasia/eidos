// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The features of descriptor.proto whose defaults decide what the
// frontend loads.
const (
	presenceFeature   = "field_presence"
	enumTypeFeature   = "enum_type"
	encodingFeature   = "message_encoding"
	visibilityFeature = "default_symbol_visibility"
)

// The feature values that the cases compare against, as descriptor.proto
// names them.
const (
	explicitPresence     = "EXPLICIT"
	openEnum             = "OPEN"
	closedEnum           = "CLOSED"
	lengthPrefixed       = "LENGTH_PREFIXED"
	delimitedEncoding    = "DELIMITED"
	exportAllVisibility  = "EXPORT_ALL"
	exportTopVisibility  = "EXPORT_TOP_LEVEL"
	strictVisibility     = "STRICT"
	localMark            = "local"
	optionImportFile     = "acme/options.proto"
	optionImportDeclarer = "root/acme/options.proto"
	editionPath          = "svc/edition.proto"
)

// defaultsSource is a schema whose loads show a version's defaults: a
// singular scalar field, a message field, a nested message and an enum.
// The verbs are the version statement and the label of a singular field,
// which proto2 requires.
const defaultsSource = `%s
package svc.d;
message M {
  %sstring a = 1;
  %sN b = 2;
  message N {}
}
enum E {
  E_ZERO = 0;
}
`

// The version statements of the cases.
const (
	proto2Statement  = `syntax = "proto2";`
	proto3Statement  = `syntax = "proto3";`
	ed2023Statement  = `edition = "2023";`
	ed2024Statement  = `edition = "2024";`
	ed2026Statement  = `edition = "2026";`
	proto2FieldLabel = "optional "
)

// A file loads under the defaults of its version, and resolves each
// feature from the version's default through its declarations, so the
// table of versions, the defaults that descriptor.proto declares, and
// the resolution are pinned.
func TestEditions(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		versions := []struct {
			name      string
			statement string
			label     string
			defaults  map[string]string
		}{
			{
				name:      "proto2",
				statement: proto2Statement,
				label:     proto2FieldLabel,
				defaults:  descriptorDefaults(t, descriptorpb.Edition_EDITION_PROTO2),
			},
			{
				name:      "proto3",
				statement: proto3Statement,
				defaults:  descriptorDefaults(t, descriptorpb.Edition_EDITION_PROTO3),
			},
			{
				name:      "edition 2023",
				statement: ed2023Statement,
				defaults:  descriptorDefaults(t, descriptorpb.Edition_EDITION_2023),
			},
			{
				name:      "edition 2024",
				statement: ed2024Statement,
				defaults:  descriptorDefaults(t, descriptorpb.Edition_EDITION_2024),
			},
			{
				// descriptorpb at v1.36.12 predates Edition 2026, so its
				// defaults follow descriptor.proto of protobuf v36.0.
				name:      "edition 2026",
				statement: ed2026Statement,
				defaults: map[string]string{
					presenceFeature:   explicitPresence,
					enumTypeFeature:   openEnum,
					encodingFeature:   lengthPrefixed,
					visibilityFeature: strictVisibility,
				},
			},
		}
		for _, v := range versions {
			source := fmt.Sprintf(defaultsSource, v.statement, v.label, v.label)

			t.Run(fmt.Sprintf("loads a field of %s under the default %s", v.name, presenceFeature), func(t *testing.T) {
				t.Parallel()

				gb, sink := parsed(t, source)
				assert.False(t, sink.Failed(), "the schema loads clean")
				m, _ := declOf(t, onlyFile(t, gb), "M").(*node.Struct)
				want := symbol.FormNamed
				if v.defaults[presenceFeature] == explicitPresence {
					want = symbol.FormOptional
				}
				assert.Equal(t, m.Fields[0].Type.Form, want, "a singular scalar field has the presence of the default")
			})

			t.Run(fmt.Sprintf("loads an enum of %s under the default %s", v.name, enumTypeFeature), func(t *testing.T) {
				t.Parallel()

				gb, _ := parsed(t, source)
				e, _ := declOf(t, onlyFile(t, gb), "E").(*node.Enum)
				var want []string
				if v.defaults[enumTypeFeature] == closedEnum {
					want = []string{closedEnum}
				}
				assert.Equal(t, stampsOn(gb, e, string(protobuf.ClosedKey)), want,
					"a closed enum has the closed mark, and an open one has none")
			})

			encoding := fmt.Sprintf("loads a message field of %s under the default %s", v.name, encodingFeature)
			t.Run(encoding, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsed(t, source)
				m, _ := declOf(t, onlyFile(t, gb), "M").(*node.Struct)
				var want []string
				if v.defaults[encodingFeature] == delimitedEncoding {
					want = []string{delimitedEncoding}
				}
				assert.Equal(t, stampsOn(gb, m.Fields[1], string(protobuf.DelimitedKey)), want,
					"a delimited message field has the delimited mark, and a length-prefixed one has none")
			})

			visibility := fmt.Sprintf("loads a message of %s under the default %s", v.name, visibilityFeature)
			t.Run(visibility, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsed(t, source)
				m, _ := declOf(t, onlyFile(t, gb), "M").(*node.Struct)
				nested, _ := m.Types[0].(*node.Struct)
				var wantTop, wantNested []string
				switch v.defaults[visibilityFeature] {
				case exportTopVisibility:
					wantNested = []string{localMark}
				case exportAllVisibility:
				default:
					wantTop, wantNested = []string{localMark}, []string{localMark}
				}
				expect.Equal(t, stampsOn(gb, m, string(protobuf.LocalKey)), wantTop,
					"a top-level message is local by the default visibility")
				expect.Equal(t, stampsOn(gb, nested, string(protobuf.LocalKey)), wantNested,
					"a nested message is local by the default visibility")
			})
		}

		t.Run("loads edition 2026 through the grammar of edition 2024", func(t *testing.T) {
			t.Parallel()

			gb, sink := grammar(t, "edition2026.proto")
			assert.False(t, sink.Failed(), "the parser's verdict on the edition value does not report")
			file := onlyFile(t, gb)
			expect.Equal(t, stampsOn(gb, file, string(protobuf.SyntaxKey)), []string{"2026"},
				"the edition is stamped as the file states it")
			row, _ := declOf(t, file, "Row").(*node.Struct)
			expect.Equal(t, row.Fields[0].Pos.Line, 9, "a position in the second parse is the position in the file")
		})

		t.Run("loads edition 2026 into the declarations of the same file in edition 2024", func(t *testing.T) {
			t.Parallel()

			declsOf := func(name string) []symbol.Symbol {
				src, err := os.ReadFile(filepath.Join("testdata", "grammar", name))
				assert.NoError(t, err, name+" is on disk")
				gb, sink := parsed(t, string(src), editionPath)
				assert.False(t, sink.Failed(), name+" loads clean")
				return onlyFile(t, gb).Decls
			}
			assert.Equal(t, declsOf("edition2026.proto"), declsOf("edition2024.proto"),
				"a parser that knows Edition 2026 can replace the second parse without a change of output")
		})

		unknown := []struct {
			name string
			give string
		}{
			{name: "reports UnknownEdition for an edition protobuf did not release", give: `edition = "2025";`},
			{name: "reports UnknownEdition for the edition in development", give: `edition = "UNSTABLE";`},
			{name: "reports UnknownEdition for a syntax outside the table", give: `syntax = "proto4";`},
			{name: "reports UnknownEdition for an edition under the syntax keyword", give: `syntax = "2024";`},
			{name: "reports UnknownEdition for a syntax under the edition keyword", give: `edition = "proto3";`},
			{name: "reports UnknownEdition for an edition that is no string", give: `edition = 2024;`},
		}
		for _, tt := range unknown {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, sink := parsed(t, tt.give+"\npackage svc.u;\nmessage M {\n  string a = 1;\n}\n")
				found := codesOf(sink)
				assert.Equal(t, found, []diag.Code{protofrontend.UnknownEdition},
					"the file reports the version it states and nothing the parser reports")
				expect.Empty(t, gb.Packages(), "the file loads nothing")
			})
		}

		t.Run("reports UnknownEdition at the statement", func(t *testing.T) {
			t.Parallel()

			_, sink := parsed(t, "\n"+`edition = "2025";`+"\n")
			found := slices.Collect(sink.All())
			assert.Length(t, found, 1, "the statement reports once")
			assert.Equal(t, found[0].Pos.Line, 2, "the finding is at the statement's line")
		})

		resolution := []struct {
			name    string
			give    string
			subject func(tb assert.TB, file *node.File) symbol.Symbol
			key     meta.KeyName
			want    []string
		}{
			{
				name:    "resolves enum_type from an enum's own options",
				give:    "edition = \"2023\";\npackage svc.r;\nenum E {\n  option features.enum_type = CLOSED;\n  E_ZERO = 0;\n}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol { return declOf(tb, file, "E") },
				key:     protobuf.ClosedKey,
				want:    []string{closedEnum},
			},
			{
				name: "resolves enum_type through the message that declares the enum",
				give: "edition = \"2023\";\npackage svc.r;\nmessage M {\n  option features.enum_type = CLOSED;\n" +
					"  enum E {\n    E_ZERO = 0;\n  }\n}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol {
					return declOf(tb, file, "M").(*node.Struct).Types[0]
				},
				key:  protobuf.ClosedKey,
				want: []string{closedEnum},
			},
			{
				name: "resolves enum_type from the file's options",
				give: "edition = \"2024\";\npackage svc.r;\noption features.enum_type = CLOSED;\n" +
					"enum E {\n  E_ZERO = 0;\n}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol { return declOf(tb, file, "E") },
				key:     protobuf.ClosedKey,
				want:    []string{closedEnum},
			},
			{
				name: "resolves message_encoding from a field's own options",
				give: "edition = \"2023\";\npackage svc.r;\nmessage M {\n" +
					"  N b = 1 [features.message_encoding = DELIMITED];\n  message N {}\n}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol {
					return declOf(tb, file, "M").(*node.Struct).Fields[0]
				},
				key:  protobuf.DelimitedKey,
				want: []string{delimitedEncoding},
			},
			{
				name: "marks no scalar field as delimited",
				give: "edition = \"2023\";\npackage svc.r;\noption features.message_encoding = DELIMITED;\n" +
					"message M {\n  string a = 1;\n}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol {
					return declOf(tb, file, "M").(*node.Struct).Fields[0]
				},
				key: protobuf.DelimitedKey,
			},
			{
				name: "resolves default_symbol_visibility from the file's options",
				give: "edition = \"2024\";\npackage svc.r;\noption features.default_symbol_visibility = LOCAL_ALL;\n" +
					"message M {}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol { return declOf(tb, file, "M") },
				key:     protobuf.LocalKey,
				want:    []string{localMark},
			},
			{
				name:    "marks a message that states local as local",
				give:    "edition = \"2024\";\npackage svc.r;\nlocal message M {}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol { return declOf(tb, file, "M") },
				key:     protobuf.LocalKey,
				want:    []string{localMark},
			},
			{
				name:    "marks no message that states export as local",
				give:    "edition = \"2026\";\npackage svc.r;\nexport message M {}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol { return declOf(tb, file, "M") },
				key:     protobuf.LocalKey,
			},
			{
				name:    "marks an enum that states local as local",
				give:    "edition = \"2024\";\npackage svc.r;\nlocal enum E {\n  E_ZERO = 0;\n}\n",
				subject: func(tb assert.TB, file *node.File) symbol.Symbol { return declOf(tb, file, "E") },
				key:     protobuf.LocalKey,
				want:    []string{localMark},
			},
		}
		for _, tt := range resolution {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, sink := parsed(t, tt.give)
				assert.False(t, sink.Failed(), "the schema loads clean")
				assert.Equal(t, stampsOn(gb, tt.subject(t, onlyFile(t, gb)), string(tt.key)), tt.want,
					"the resolved feature decides the mark")
			})
		}

		t.Run("stamps an option import with its mark", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, "edition = \"2024\";\npackage svc.o;\nimport option \""+optionImportFile+"\";\n")
			assert.False(t, sink.Failed(), "an option import loads")
			assert.Equal(t, stampsOn(gb, onlyFile(t, gb), string(protobuf.ImportKey)),
				[]string{"option " + optionImportFile}, "the import stamp marks an option import")
		})

		t.Run("names no option import as the import of a declaring file", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsed(t, "edition = \"2024\";\npackage svc.o;\nimport option \""+optionImportFile+"\";\n")
			assert.Length(t, gb.Scopes(), 1, "the file records its bindings")
			importer, _ := protofrontend.New().(plugin.Importer)
			got := importer.ImportOf(plugin.ImportScope{Bindings: gb.Scopes()[0].Bindings}, optionImportDeclarer)
			assert.Equal(t, got, optionImportDeclarer,
				"an option import imports no types, so the declaring file keeps its own path")
		})
	})
}

// descriptorDefaults returns the defaults of the four features that the
// frontend applies, at one edition. It reads them from the edition
// defaults that protobuf-go's descriptorpb declares on the fields of
// FeatureSet, and takes the last default at or before the edition.
func descriptorDefaults(tb assert.TB, edition descriptorpb.Edition) map[string]string {
	tb.Helper()

	fields := (&descriptorpb.FeatureSet{}).ProtoReflect().Descriptor().Fields()
	out := map[string]string{}
	for _, feature := range []string{presenceFeature, enumTypeFeature, encodingFeature, visibilityFeature} {
		field := fields.ByName(protoreflect.Name(feature))
		assert.NotNil(tb, field, "descriptorpb declares the feature "+feature)
		options, _ := field.Options().(*descriptorpb.FieldOptions)
		for _, d := range options.GetEditionDefaults() {
			if d.GetEdition() <= edition {
				out[feature] = d.GetValue()
			}
		}
	}
	return out
}
