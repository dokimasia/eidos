// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the restated schemas.
const (
	storePkg   = "svc.store"
	editionPkg = "svc.edition"
	barePkg    = "svc.bare"
	orderPkg   = "svc.order"
)

// restatedTree is a proto2 file with every residue of a declaration and of
// a file, a file of edition 2024 with a local message and a closed enum,
// and a file without a syntax statement, beside the files that the first
// one imports.
var restatedTree = fstest.MapFS{
	"svc/store.proto": {Data: []byte(`// The store of the rows.
syntax = "proto2";

// Package store stores rows.
package svc.store;

import "svc/common.proto";
import public "svc/types.proto";
import weak "svc/legacy.proto";

option go_package = "example.com/svc/storepb";

message Row {
  reserved 9 to 11, 40 to max;
  reserved "old", "older";
  extensions 100 to 199 [verification = UNVERIFIED];
  extensions 300;

  required string name = 1 [json_name = "rowName", deprecated = true];
  optional group Extra = 2 {
    optional string note = 1;
  }
}

enum Colour {
  COLOUR_UNSET = 0;
}
`)},
	"svc/common.proto": {Data: []byte("syntax = \"proto2\";\npackage svc.store;\n")},
	"svc/types.proto":  {Data: []byte("syntax = \"proto2\";\npackage svc.store;\n")},
	"svc/legacy.proto": {Data: []byte("syntax = \"proto2\";\npackage svc.store;\n")},
	"svc/edition.proto": {Data: []byte(`edition = "2024";

package svc.edition;

option features.enum_type = CLOSED;

local message Hidden {}

enum Mode {
  MODE_UNSET = 0;
}
`)},
	"svc/bare.proto": {Data: []byte("package svc.bare;\n\nmessage Bare {}\n")},
	"svc/order.proto": {Data: []byte(`syntax = "proto3";

package svc.order;

message Order {
  enum Kind {
    KIND_UNSET = 0;
  }
  string id = 1;
  oneof pick {
    string a = 2;
  }
  string b = 3;
}

service Orders {
  rpc Get(Order) returns (Order) {}
}
`)},
}

// restated returns what Stated returns for each identity and what Header
// returns for each declaration, in one generate phase over restatedTree.
func restated(
	tb assert.TB, stated, headers []symbol.Identity,
) ([]protobuf.Statement, []*emit.File) {
	tb.Helper()

	f := plugintest.Loaded(tb, plugintest.Source{
		Tree: restatedTree, Brand: frontendtest.Brand, Frontends: []plugin.Frontend{protofrontend.New()},
	})
	var (
		statements []protobuf.Statement
		files      []*emit.File
	)
	restater := eidos.NewPlugin("restater").
		Output(plugin.Output{Per: plugin.PerPlan, Word: "restated"}).
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, _ *eidos.Emitter) error {
			for _, id := range stated {
				statements = append(statements, protobuf.Stated(m, id))
			}
			for _, id := range headers {
				d, held := m.Reader().Lookup(id)
				assert.True(tb, held, "the graph declares "+id.String())
				decl, is := d.(node.Declaration)
				assert.True(tb, is, id.String()+" is a declaration")
				files = append(files, protobuf.Header(m, decl))
			}
			return nil
		})).Generator()
	r := f.Generate(tb, restater)
	assert.NoError(tb, r.Err, "the generate phase completes")
	return statements, files
}

// protoID returns the identity of a declaration of the restated tree.
func protoID(pkg, owner, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: protobuf.Lang, Package: pkg, Owner: owner, Name: name, Kind: kind}
}

// marker returns the annotation of a marker with its arguments.
func marker(m protobuf.Marker, args ...string) symbol.Annotation {
	return symbol.Annotation{Name: string(m), Args: args}
}

// TestStated pins the channels through which Stated and Header restate
// the stamps of a protobuf source.
func TestStated(t *testing.T) {
	t.Parallel()

	t.Run("Stated", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want protobuf.Statement
		}{
			{
				name: "returns the number, the label, the json_name and the options of a field",
				give: protoID(storePkg, "Row", "name", symbol.KindField),
				want: protobuf.Statement{
					Annotations: symbol.Annotations{
						marker(protobuf.RequiredMarker),
						{Name: "json_name", Args: []string{`"rowName"`}},
						{Name: "deprecated", Args: []string{"true"}},
					},
					Number: &emit.TagEntry{Key: protobuf.NumberTag, Value: "1"},
				},
			},
			{
				name: "returns the reserved entries and each extension range of a message",
				give: protoID(storePkg, "", "Row", symbol.KindStruct),
				want: protobuf.Statement{Annotations: symbol.Annotations{
					marker(protobuf.ReservedMarker, "9 to 11", "40 to max", "old", "older"),
					marker(protobuf.ExtensionsMarker, "100 to 199", "[verification = UNVERIFIED]"),
					marker(protobuf.ExtensionsMarker, "300"),
				}},
			},
			{
				name: "returns the delimited marker of the field of a group",
				give: protoID(storePkg, "Row", "extra", symbol.KindField),
				want: protobuf.Statement{
					Annotations: symbol.Annotations{marker(protobuf.DelimitedMarker)},
					Number:      &emit.TagEntry{Key: protobuf.NumberTag, Value: "2"},
				},
			},
			{
				name: "returns the closed marker of an enum of proto2",
				give: protoID(storePkg, "", "Colour", symbol.KindEnum),
				want: protobuf.Statement{Annotations: symbol.Annotations{marker(protobuf.ClosedMarker)}},
			},
			{
				name: "returns the local marker of a local message",
				give: protoID(editionPkg, "", "Hidden", symbol.KindStruct),
				want: protobuf.Statement{Annotations: symbol.Annotations{marker(protobuf.LocalMarker)}},
			},
			{
				name: "returns the closed marker of an enum that the features of its file close",
				give: protoID(editionPkg, "", "Mode", symbol.KindEnum),
				want: protobuf.Statement{Annotations: symbol.Annotations{marker(protobuf.ClosedMarker)}},
			},
			{
				name: "returns the follows marker without a name for a nested declaration before every member",
				give: protoID(orderPkg, "Order", "Kind", symbol.KindEnum),
				want: protobuf.Statement{Annotations: symbol.Annotations{marker(protobuf.FollowsMarker, "")}},
			},
			{
				name: "returns the follows marker with the field before a oneof",
				give: protoID(orderPkg, "Order", "pick", symbol.KindSum),
				want: protobuf.Statement{Annotations: symbol.Annotations{marker(protobuf.FollowsMarker, "id")}},
			},
			{
				name: "returns the body marker of an rpc with a body",
				give: protoID(orderPkg, "Orders", "Get", symbol.KindMethod),
				want: protobuf.Statement{Annotations: symbol.Annotations{marker(protobuf.BodyMarker)}},
			},
			{
				name: "returns nothing for a declaration without residue",
				give: protoID(storePkg, "Colour", "COLOUR_UNSET", symbol.KindEnumVariant),
			},
		}
		ids := make([]symbol.Identity, 0, len(tests))
		for _, tt := range tests {
			ids = append(ids, tt.give)
		}
		got, _ := restated(t, ids, nil)
		assert.Length(t, got, len(tests), "Stated returns one statement per identity")
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[i], tt.want, "the statement of the residue")
			})
		}
	})

	t.Run("Header", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want *emit.File
		}{
			{
				name: "returns the syntax, the package, the imports and the options of a file",
				give: protoID(storePkg, "", "Row", symbol.KindStruct),
				want: &emit.File{
					Doc: []string{"The store of the rows.", "Package store stores rows."},
					Annotations: symbol.Annotations{
						marker(protobuf.SyntaxMarker, protobuf.Proto2),
						marker(protobuf.PackageMarker, storePkg),
						marker(protobuf.ImportMarker, "svc/common.proto"),
						marker(protobuf.ImportMarker, "public svc/types.proto"),
						marker(protobuf.ImportMarker, "weak svc/legacy.proto"),
						{Name: "go_package", Args: []string{`"example.com/svc/storepb"`}},
					},
				},
			},
			{
				name: "returns the features of a file of an edition",
				give: protoID(editionPkg, "", "Hidden", symbol.KindStruct),
				want: &emit.File{Annotations: symbol.Annotations{
					marker(protobuf.SyntaxMarker, protobuf.Edition2024),
					marker(protobuf.PackageMarker, editionPkg),
					{Name: protobuf.FeatureEnumType, Args: []string{"CLOSED"}},
				}},
			},
			{
				name: "returns proto2 for a file without a syntax statement",
				give: protoID(barePkg, "", "Bare", symbol.KindStruct),
				want: &emit.File{Annotations: symbol.Annotations{
					marker(protobuf.SyntaxMarker, protobuf.Proto2),
					marker(protobuf.PackageMarker, barePkg),
				}},
			},
		}
		ids := make([]symbol.Identity, 0, len(tests))
		for _, tt := range tests {
			ids = append(ids, tt.give)
		}
		_, got := restated(t, nil, ids)
		assert.Length(t, got, len(tests), "Header returns one header per declaration")
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[i], tt.want, "the header of the file of the declaration")
			})
		}
	})
}
