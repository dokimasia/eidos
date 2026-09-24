// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The syntax errors the cap case writes, and the cap the frontend
// reports up to.
const (
	badFields    = 15
	syntaxCap    = 10
	badFieldLine = "message M { string x = ; }\n"
)

// formsOf returns the form of each field's type, in field order.
func formsOf(s *node.Struct) []symbol.TypeForm {
	out := make([]symbol.TypeForm, 0, len(s.Fields))
	for _, f := range s.Fields {
		out = append(out, f.Type.Form)
	}
	return out
}

// The lowering turns protobuf's own shapes into the node model, so
// each mapping the corpus reads is pinned at the unit level.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a message as a struct under its proto package", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

// Row is a stored record.
message Row {
  string name = 1;
  int64 size = 2;
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			pkg := gb.Packages()[0]
			assert.Equal(t, pkg.ID.Package, fixturePkg, "the proto package is the namespace")
			assert.Equal(t, pkg.Path, []string{"svc", "store"}, "split on its dots into the path")
			assert.Equal(t, pkg.Name, "store", "and named by its last segment")
			file := onlyFile(t, gb)
			row, is := declOf(t, file, "Row").(*node.Struct)
			assert.True(t, is, "a message is a struct")
			assert.Equal(t, row.Doc, []string{"Row is a stored record."}, "with its documentation")
			assert.Length(t, row.Fields, 2, "and its fields")
			assert.Equal(t, row.Fields[0].Name, "name", "in source order")
			assert.Equal(t, row.Fields[0].Type.Spelling, "string", "each with its type")
			assert.Equal(t, stampsOn(gb, row.Fields[0], string(protobuf.FieldKey)), []string{"1"},
				"and its wire number, which decides compatibility")
			assert.Equal(t, stampsOn(gb, row.Fields[1], string(protobuf.FieldKey)), []string{"2"},
				"per field")
		})

		t.Run("states the label forms the projection has", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

message Row {
  repeated string tags = 1;
  optional int64 size = 2;
  map<string, int64> counts = 3 [json_name = "cnt"];
  string plain = 4;
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)

			tags := row.Fields[0].Type
			assert.Equal(t, tags.Form, symbol.FormList, "repeated is a list")
			assert.Equal(t, tags.Elems[0].Spelling, "string", "over its element")
			assert.Equal(t, tags.Spelling, "repeated string", "keeping what the schema wrote")

			size := row.Fields[1].Type
			assert.Equal(t, size.Form, symbol.FormOptional, "optional is presence, which the optional form projects")
			assert.Equal(t, size.Elems[0].Spelling, "int64", "over its type")

			counts := row.Fields[2]
			assert.Equal(t, counts.Type.Form, symbol.FormMap, "a map is the map form")
			assert.Equal(t, counts.Type.Spelling, "map<string, int64>", "spelled as written")
			assert.Length(t, counts.Type.Elems, 2, "key then value")
			assert.Equal(t, counts.Type.Elems[0].Spelling, "string", "the key")
			assert.Equal(t, counts.Type.Elems[1].Spelling, "int64", "and the value")
			assert.Equal(t, stampsOn(gb, counts, string(protobuf.MapEntryKey)), []string{"map"},
				"marked as the map the schema wrote")
			assert.Equal(t, stampsOn(gb, counts, string(protobuf.JSONNameKey)), []string{"cnt"},
				"and a map field's json_name is stamped the way any field's is")
			assert.Empty(t, stampsOn(gb, counts, string(protobuf.OptionsKey)),
				"and not restated among its options")

			assert.Equal(t, row.Fields[3].Type.Form, symbol.FormNamed,
				"a plain field is the type itself")
		})

		t.Run("resolves an edition's field presence down its messages", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `edition = "2023";

package svc.ed;

option features.field_presence = IMPLICIT;

message Plain {
  string a = 1;
}

message Explicit {
  option features.field_presence = EXPLICIT;
  string b = 1;
  int64 c = 2 [features.field_presence = LEGACY_REQUIRED];
  repeated string d = 3;
  message Inner {
    string e = 1;
  }
  oneof body {
    string f = 4;
  }
}
`)
			assert.False(t, sink.Failed(), "an edition file loads")
			file := onlyFile(t, gb)
			plain, _ := declOf(t, file, "Plain").(*node.Struct)
			assert.Equal(t, formsOf(plain), []symbol.TypeForm{symbol.FormNamed},
				"the file's implicit presence gives a singular field no presence")

			explicit, _ := declOf(t, file, "Explicit").(*node.Struct)
			assert.Equal(t, formsOf(explicit), []symbol.TypeForm{
				symbol.FormOptional, symbol.FormNamed, symbol.FormList,
			}, "a message's explicit presence is the optional form, and a field's own feature overrides it")
			assert.Equal(t, explicit.Fields[0].Type.Spelling, "string", "spelled as written")
			assert.Equal(t, stampsOn(gb, explicit.Fields[1], string(protobuf.LabelKey)), []string{"required"},
				"legacy required is stamped the way proto2's required is")
			inner, _ := explicit.Types[0].(*node.Struct)
			assert.Equal(t, formsOf(inner), []symbol.TypeForm{symbol.FormOptional},
				"a nested message inherits its message's presence")
			sum, _ := explicit.Types[1].(*node.Sum)
			assert.Equal(t, sum.Variants[0].Fields[0].Type.Form, symbol.FormNamed,
				"and a oneof member has presence through its oneof, so its form is the type")

			defaulted, sink := parsed(t, "edition = \"2024\";\npackage svc.ed;\nmessage M {\n  string a = 1;\n}\n")
			assert.False(t, sink.Failed(), "a file stating no presence loads")
			m, _ := declOf(t, onlyFile(t, defaulted), "M").(*node.Struct)
			assert.Equal(t, formsOf(m), []symbol.TypeForm{symbol.FormOptional},
				"and takes the edition default, which is explicit presence")
		})

		t.Run("loads a oneof as a sum whose variants are its members", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

message Row {
  // Body is what the row stores.
  oneof body {
    // Text is the row as text.
    //+gen:table name=text
    string text = 1;
    bytes blob = 2;
  }
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)
			assert.Length(t, row.Types, 1, "the oneof is a nested type")
			sum, is := row.Types[0].(*node.Sum)
			assert.True(t, is, "a oneof is the tagged sum shape, never an untagged union")
			assert.Equal(t, sum.Name, "body", "named as written")
			assert.Equal(t, sum.Doc, []string{"Body is what the row stores."}, "with its documentation")
			assert.Length(t, sum.Variants, 2, "one variant per member")
			text := sum.Variants[0]
			assert.Equal(t, text.Name, "text", "in source order")
			assert.Length(t, text.Fields, 1, "each with the member's own field")
			assert.Equal(t, text.Fields[0].Type.Spelling, "string", "and its type")
			assert.Equal(t, text.Doc, []string{"Text is the row as text."},
				"the variant repeats the member's documentation")
			assert.Length(t, gb.Attachments(), 1, "the member's carrier attaches once")
			assert.True(t, gb.Attachments()[0].Subject == symbol.Symbol(text.Fields[0]),
				"to the member's field, which is the member")
			assert.Equal(t, stampsOn(gb, sum, string(protobuf.OneofKey)), []string{"body"},
				"marked as the oneof it projected from")
			assert.Empty(t, row.Fields, "a oneof's members are the sum's, not the message's")
		})

		t.Run("loads an enum with its declared numbers", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

enum Colour {
  COLOUR_UNSPECIFIED = 0;
  COLOUR_RED = 1;
  COLOUR_BLUE = 7;
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			colour, is := declOf(t, onlyFile(t, gb), "Colour").(*node.Enum)
			assert.True(t, is, "an enum is the enum kind")
			assert.Length(t, colour.Variants, 3, "one variant per value")
			assert.Equal(t, colour.Variants[0].Value, "0", "with the number the schema declared")
			assert.Equal(t, colour.Variants[2].Value, "7", "however it skips")
		})

		t.Run("loads a service as an interface of rpc methods", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

message Req {}
message Res {}

// Store reads and writes rows.
service Store {
  // Get reads one row.
  rpc Get(Req) returns (Res);
  rpc Watch(Req) returns (stream Res);
  rpc Send(stream Req) returns (Res);
  rpc Chat(stream Req) returns (stream Res);
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			store, is := declOf(t, onlyFile(t, gb), "Store").(*node.Interface)
			assert.True(t, is, "a service is an interface")
			assert.Equal(t, store.Doc, []string{"Store reads and writes rows."}, "with its documentation")
			assert.Length(t, store.Methods, 4, "one method per rpc")

			get := store.Methods[0]
			assert.True(t, get.Abstract, "an rpc is a signature, never a body")
			assert.Equal(t, get.Doc, []string{"Get reads one row."}, "with its own documentation")
			assert.Length(t, get.Params, 1, "one parameter for the request")
			assert.Equal(t, get.Params[0].Type.Spelling, "Req", "which names the request message")
			assert.Length(t, get.Returns, 1, "and one return for the response")
			assert.Equal(t, get.Returns[0].Type.Spelling, "Res", "which names the response message")
			assert.Empty(t, stampsOn(gb, get, string(protobuf.StreamKey)),
				"an rpc that streams neither side is marked as nothing")

			assert.Equal(t, store.Methods[1].Returns[0].Type.Form, symbol.FormStream,
				"a streaming response is a stream over its message")
			assert.Equal(t, stampsOn(gb, store.Methods[1], string(protobuf.StreamKey)),
				[]string{"response"}, "naming which side streams")
			assert.Equal(t, stampsOn(gb, store.Methods[2], string(protobuf.StreamKey)),
				[]string{"request"}, "either side")
			assert.Equal(t, stampsOn(gb, store.Methods[3], string(protobuf.StreamKey)),
				[]string{"both"}, "or both")
		})

		t.Run("nests a message and an enum declared inside a message", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

message Row {
  message Key {
    string id = 1;
  }
  enum State {
    STATE_UNSPECIFIED = 0;
  }
  Key key = 1;
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			file := onlyFile(t, gb)
			assert.Length(t, file.Decls, 1, "a nested declaration is not a file-level one")
			row, _ := declOf(t, file, "Row").(*node.Struct)
			assert.Length(t, row.Types, 2, "both nest under the message that declares them")
			_, nestedMessage := row.Types[0].(*node.Struct)
			_, nestedEnum := row.Types[1].(*node.Enum)
			assert.True(t, nestedMessage && nestedEnum, "each as the kind it is")
		})

		t.Run("stamps the residue the projection has no form for", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

option go_package = "example.test/svc/store";
option java_package = "test.example.svc";

message Row {
  reserved 2, 15, 9 to 11;
  reserved "old_name";
  string name = 1;
}

enum Colour {
  reserved 3;
  COLOUR_UNSPECIFIED = 0;
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			file := onlyFile(t, gb)

			assert.Equal(t, stampsOn(gb, file, string(protobuf.SyntaxKey)), []string{"proto3"},
				"the syntax decides what an absent value means, so it is stamped")
			assert.Equal(t, stampsOn(gb, file, string(protobuf.PackageKey)), []string{fixturePkg},
				"and the proto package, which is not the path the file loaded under")
			options := stampsOn(gb, file, string(protobuf.OptionsKey))
			assert.Length(t, options, 1, "the file's options arrive as one entry")
			assert.Contains(t, options[0], "go_package=", "naming each option")
			assert.Contains(t, options[0], "java_package=", "for generators outside this workspace")

			row, _ := declOf(t, file, "Row").(*node.Struct)
			reserved := stampsOn(gb, row, string(protobuf.ReservedKey))
			assert.Length(t, reserved, 1,
				"every reserved declaration folds into one list, because the question is what may not be reused")
			assert.Contains(t, reserved[0], "9 to 11", "the ranges as written")
			assert.Contains(t, reserved[0], "old_name", "and the names beside them")
			colour, _ := declOf(t, file, "Colour").(*node.Enum)
			assert.Equal(t, stampsOn(gb, colour, string(protobuf.ReservedKey)), []string{"3"},
				"an enum reserves too")
		})

		t.Run("stamps an edition's reserved identifiers and extension range options", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `edition = "2023";

package svc.store;

message Row {
  reserved old_name, legacy;
  reserved 7;
  extensions 100 to 199 [verification = UNVERIFIED];
  string name = 1;
}
`)
			assert.False(t, sink.Failed(), "the edition schema loads clean")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)
			assert.Equal(t, stampsOn(gb, row, string(protobuf.ReservedKey)), []string{"old_name, legacy, 7"},
				"an edition writes reserved names as identifiers, and no entry stamps empty")
			assert.Equal(t, stampsOn(gb, row, string(protobuf.ExtensionsKey)),
				[]string{"100 to 199 [verification = UNVERIFIED]"},
				"and an extension range keeps the options it states")
		})

		t.Run("records the imports a schema states", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

import "svc/other.proto";
import "google/protobuf/timestamp.proto";

message Row {
  google.protobuf.Timestamp when = 1;
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			file := onlyFile(t, gb)
			assert.Length(t, file.Imports, 2, "both imports load")
			assert.Equal(t, file.Imports[0].Path, "svc/other.proto", "by the path the schema wrote")
			assert.Equal(t, file.Imports[1].Path, "google/protobuf/timestamp.proto", "in source order")
			row, _ := declOf(t, file, "Row").(*node.Struct)
			assert.Equal(t, row.Fields[0].Type.Spelling, "google.protobuf.Timestamp",
				"a well-known type keeps its qualified spelling")
		})

		t.Run("refuses an extend block, positioned", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto2";

package svc.store;

message Row {
  optional string name = 1;
}

extend Row {
  optional int64 size = 100;
}
`)
			assert.True(t, sink.Failed(), "an extension changes a declaration the file does not declare")
			assert.True(t, slices.Contains(codesOf(sink), protofrontend.RefusedExtension),
				"under the refusal's own code")
			var msg string
			for d := range sink.All() {
				if d.Code == protofrontend.RefusedExtension {
					msg = d.Msg
					assert.Equal(t, d.Pos.Line, 9, "at the block")
				}
			}
			assert.Contains(t, msg, "Row", "naming the message it would extend")
			file := onlyFile(t, gb)
			assert.Length(t, file.Decls, 1, "and the block loads as nothing")
		})

		t.Run("reports a syntax error and keeps what parsed", func(t *testing.T) {
			t.Parallel()

			_, sink := parsed(t, `syntax = "proto3";

package svc.store;

message Row {
  string name = ;
}
`)
			assert.True(t, sink.Failed(), "a bad token reports")
			assert.True(t, slices.Contains(codesOf(sink), protofrontend.UnparsedFile),
				"under the file's own code")
			for d := range sink.All() {
				if d.Code == protofrontend.UnparsedFile {
					assert.Equal(t, d.Pos.File, fixturePath, "positioned in the file")
					assert.True(t, d.Pos.Line > 0, "at a line")
				}
			}
		})

		t.Run("caps the syntax errors one file reports", func(t *testing.T) {
			t.Parallel()

			_, sink := parsed(t, "syntax = \"proto3\";\npackage svc.store;\n"+
				strings.Repeat(badFieldLine, badFields))
			var found []diag.Diag
			for d := range sink.All() {
				found = append(found, d)
			}
			assert.Length(t, found, syntaxCap+1, "the cap's worth of errors, then one count")
			assert.Contains(t, found[syntaxCap].Msg, "and 5 more syntax errors", "naming the rest")
		})

		t.Run("loads a file stating no package under the unnamed namespace", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, "syntax = \"proto3\";\n\nmessage Row {\n  string name = 1;\n}\n")
			assert.False(t, sink.Failed(), "proto admits a file with no package")
			assert.Equal(t, gb.Packages()[0].ID.Package, "", "which the graph keeps as the unnamed namespace")
			file := onlyFile(t, gb)
			assert.Empty(t, stampsOn(gb, file, string(protobuf.PackageKey)), "and nothing is stamped")
		})

		t.Run("attaches a directive carrier to its declaration", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

// Row is a stored record.
//+gen:table name=rows
message Row {
  string name = 1;
}
`)
			assert.False(t, sink.Failed(), "the carrier is well formed")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)
			assert.Equal(t, row.Doc, []string{"Row is a stored record."}, "the carrier is not documentation")
			assert.Length(t, gb.Attachments(), 1, "the carrier attached")
			assert.Equal(t, string(gb.Attachments()[0].Raw.Name), "gen:table", "under its spelling")
			assert.True(t, gb.Attachments()[0].Subject == symbol.Symbol(row), "to the declaration below it")
		})

		t.Run("stamps proto2's residue", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto2";

package svc.store;

message Row {
  required string name = 1;
  optional int64 size = 2 [default = 7];
  optional string alias = 3 [json_name = "aliasName"];
  extensions 100 to 200;
}
`)
			assert.False(t, sink.Failed(), "a proto2 file loads")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)

			assert.Equal(t, stampsOn(gb, row.Fields[0], string(protobuf.LabelKey)), []string{"required"},
				"required has no form in the projection, so it is stamped")
			assert.Equal(t, row.Fields[1].Value, "7",
				"a default is the value an absent field reads as, which the model stores on the field")
			assert.Equal(t, stampsOn(gb, row.Fields[2], string(protobuf.JSONNameKey)), []string{"aliasName"},
				"a json_name decides the JSON spelling and nothing about the type")
			assert.Empty(t, stampsOn(gb, row.Fields[1], string(protobuf.OptionsKey)),
				"an option the model stores on the field is not restated as a stamp")
			assert.Contains(t, stampsOn(gb, row, string(protobuf.ExtensionsKey))[0], "100 to 200",
				"an extension range reserves numbers, which a compatibility check reads")
		})

		t.Run("stamps how each import is written", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

import "plain.proto";
import public "shared.proto";
import weak "optional.proto";
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			file := onlyFile(t, gb)
			imports := stampsOn(gb, file, string(protobuf.ImportKey))
			assert.Length(t, imports, 1, "the imports are stamped as one list")
			assert.Contains(t, imports[0], "plain.proto", "a plain import by its path")
			assert.Contains(t, imports[0], "public shared.proto",
				"a public one marked, because it re-exports what it imports")
			assert.Contains(t, imports[0], "weak optional.proto",
				"and a weak one, because it may be absent at run time")
		})

		t.Run("stamps a oneof's options and refuses a group inside it", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto2";

package svc.store;

message Row {
  oneof body {
    option deprecated = true;
    optional string text = 1;
    optional group Inner = 2 {
      optional string v = 3;
    }
  }
}
`)
			assert.True(t, sink.Failed(), "a group inside a oneof is a group like any other")
			assert.True(t, slices.Contains(codesOf(sink), protofrontend.RefusedGroup),
				"under the refusal's own code, never silently")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)
			sum, _ := row.Types[0].(*node.Sum)
			assert.Length(t, sum.Variants, 1, "the field loads and the group loads as nothing")
			assert.Contains(t, stampsOn(gb, sum, string(protobuf.OptionsKey))[0], "deprecated=",
				"and a oneof's own options are stamped, the way every other level's are")
		})

		t.Run("refuses a group, positioned", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto2";

package svc.store;

message Row {
  optional group Inner = 1 {
    optional string v = 2;
  }
}
`)
			assert.True(t, sink.Failed(), "a group is one declaration the model represents as two")
			assert.True(t, slices.Contains(codesOf(sink), protofrontend.RefusedGroup),
				"under the refusal's own code")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)
			assert.Empty(t, row.Fields, "and the group loads as nothing")
		})

		t.Run("refuses a carrier no declaration takes", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

message Row {
  string name = 1;
}

//+gen:table name=orphan

//tool:mark on
`)
			assert.True(t, sink.Failed(), "an authored directive above nothing is reported")
			assert.True(t, slices.Contains(codesOf(sink), protofrontend.UnaddressedCarrier),
				"under the carrier's own code, never silently")
			var msg string
			for d := range sink.All() {
				if d.Code == protofrontend.UnaddressedCarrier {
					msg = d.Msg
					assert.Equal(t, d.Pos.Line, 9, "positioned at the carrier's own line")
				}
			}
			assert.Contains(t, msg, "gen:table", "naming the directive that attached nowhere")

			file := onlyFile(t, gb)
			assert.Length(t, gb.Attachments(), 0, "and nothing attached")
			var names []string
			for _, a := range file.Annotations {
				names = append(names, a.Name)
			}
			assert.Contains(t, strings.Join(names, " "), "tool:mark",
				"a floating tool directive becomes the file's annotation, the way it does elsewhere")
		})

		t.Run("loads every proto3 form the grammar admits", func(t *testing.T) {
			t.Parallel()

			gb, sink := grammar(t, "proto3.proto")
			assert.False(t, sink.Failed(), "the proto3 schema loads clean")
			file := onlyFile(t, gb)

			row, is := declOf(t, file, "Row").(*node.Struct)
			assert.True(t, is, "a message is a struct")
			assert.Equal(t, formsOf(row), []symbol.TypeForm{
				symbol.FormNamed, symbol.FormOptional, symbol.FormList, symbol.FormMap,
				symbol.FormNamed, symbol.FormNamed, symbol.FormNamed, symbol.FormNamed,
				symbol.FormNamed,
			}, "every proto3 field form lowers to the form the label states")

			assert.Equal(t, stampsOn(gb, file, string(protobuf.SyntaxKey)), []string{"proto3"},
				"the syntax is stamped")
			assert.Equal(t, stampsOn(gb, row.Fields[8], string(protobuf.JSONNameKey)),
				[]string{"renamedField"}, "a json_name is stamped")
			assert.Empty(t, stampsOn(gb, row.Fields[0], string(protobuf.LabelKey)),
				"proto3 states no required, so no field has the label")

			colour, _ := declOf(t, file, "Colour").(*node.Enum)
			assert.Length(t, colour.Variants, 3, "an aliased enum keeps both names")
			store, _ := declOf(t, file, "Store").(*node.Interface)
			assert.Length(t, store.Methods, 4, "every rpc streaming form loads")
		})

		t.Run("loads an edition and stamps its features per level", func(t *testing.T) {
			t.Parallel()

			for _, tt := range []struct {
				file    string
				edition string
			}{
				{"edition2023.proto", "2023"},
				{"edition2024.proto", "2024"},
			} {
				t.Run(tt.edition, func(t *testing.T) {
					t.Parallel()

					gb, sink := grammar(t, tt.file)
					assert.False(t, sink.Failed(), "the edition schema loads clean")
					file := onlyFile(t, gb)
					features := string(protobuf.FeaturesKey)

					assert.Equal(t, stampsOn(gb, file, string(protobuf.SyntaxKey)),
						[]string{tt.edition}, "the edition is stamped where a syntax would be")
					assert.NotEmpty(t, stampsOn(gb, file, features),
						"a file's features are stamped")
					for _, options := range stampsOn(gb, file, string(protobuf.OptionsKey)) {
						assert.NotContains(t, options, "features.",
							"and never mixed into the plain options")
					}

					row, _ := declOf(t, file, "Row").(*node.Struct)
					var stated int
					for _, f := range row.Fields {
						stated += len(stampsOn(gb, f, features))
					}
					assert.True(t, stated > 0,
						"a field states its own features, which is where presence is decided")
				})
			}
		})

		t.Run("admits features on every level an edition states them", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{"svc/ed.proto": `edition = "2023";
package svc.ed;
enum Mode {
  option features.enum_type = CLOSED;
  MODE_OFF = 0 [features.legacy_closed_enum = true];
}
service Store {
  option features.json_format = ALLOW;
  rpc Get(Req) returns (Req) {
    option features.json_format = ALLOW;
  }
}
message Req {}
`})
			_, held := g.Lookup(symbol.Identity{
				Lang: protobuf.Lang, Package: "svc.ed", Name: "Store", Kind: symbol.KindInterface,
			})
			assert.True(t, held, "the load stamps an enum value's, a service's and an rpc's features and seals")
		})

		t.Run("loads every element the grammar admits", func(t *testing.T) {
			t.Parallel()

			gb, sink := grammar(t, "proto2.proto")
			file := onlyFile(t, gb)

			t.Run("projects every declaration the model has a kind for", func(t *testing.T) {
				row, is := declOf(t, file, "Row").(*node.Struct)
				assert.True(t, is, "a message is a struct")

				var fields []string
				for _, f := range row.Fields {
					fields = append(fields, f.Name)
				}
				assert.Equal(t, fields,
					[]string{"required_field", "defaulted", "repeated_field", "map_field"},
					"every field form loads, in source order")

				var nested []string
				for _, ty := range row.Types {
					switch d := ty.(type) {
					case *node.Sum:
						nested = append(nested, "sum:"+d.Name)
					case *node.Struct:
						nested = append(nested, "message:"+d.Name)
					case *node.Enum:
						nested = append(nested, "enum:"+d.Name)
					}
				}
				assert.Equal(t, nested, []string{"sum:body", "message:Nested", "enum:NestedEnum"},
					"a oneof, a nested message and a nested enum each load as their own kind")

				_, isEnum := declOf(t, file, "Colour").(*node.Enum)
				assert.True(t, isEnum, "a file-level enum loads")
				_, isService := declOf(t, file, "Store").(*node.Interface)
				assert.True(t, isService, "and a service does")
				assert.Length(t, file.Imports, 3, "every import loads")
			})

			t.Run("stamps every residue the projection has no kind for", func(t *testing.T) {
				row, _ := declOf(t, file, "Row").(*node.Struct)
				colour, _ := declOf(t, file, "Colour").(*node.Enum)
				store, _ := declOf(t, file, "Store").(*node.Interface)
				sum, _ := row.Types[0].(*node.Sum)
				nestedEnum, _ := row.Types[2].(*node.Enum)

				for _, tt := range []struct {
					what    string
					subject symbol.Symbol
					key     string
				}{
					{"a file's syntax", file, string(protobuf.SyntaxKey)},
					{"a file's package", file, string(protobuf.PackageKey)},
					{"a file's options", file, string(protobuf.OptionsKey)},
					{"a file's imports", file, string(protobuf.ImportKey)},
					{"a message's options", row, string(protobuf.OptionsKey)},
					{"a message's reserved ranges", row, string(protobuf.ReservedKey)},
					{"a message's extension ranges", row, string(protobuf.ExtensionsKey)},
					{"a field's wire number", row.Fields[0], string(protobuf.FieldKey)},
					{"a field's required label", row.Fields[0], string(protobuf.LabelKey)},
					{"a map field's mark", row.Fields[3], string(protobuf.MapEntryKey)},
					{"a oneof's mark", sum, string(protobuf.OneofKey)},
					{"a oneof's options", sum, string(protobuf.OptionsKey)},
					{"a nested enum's options", nestedEnum, string(protobuf.OptionsKey)},
					{"a nested enum's reserved ranges", nestedEnum, string(protobuf.ReservedKey)},
					{"an enum's options", colour, string(protobuf.OptionsKey)},
					{"an enum's reserved ranges", colour, string(protobuf.ReservedKey)},
					{"an enum value's options", colour.Variants[0], string(protobuf.OptionsKey)},
					{"a service's options", store, string(protobuf.OptionsKey)},
					{"an rpc's options", store.Methods[0], string(protobuf.OptionsKey)},
				} {
					assert.NotEmpty(t, stampsOn(gb, tt.subject, tt.key), tt.what+" is stamped")
				}
				assert.Equal(t, row.Fields[1].Value, "7",
					"and a default is on the field the model gives it to")
			})

			t.Run("refuses every construct the model has no shape for", func(t *testing.T) {
				var refusals []string
				for d := range sink.All() {
					refusals = append(refusals, d.Msg)
					assert.True(t, d.Pos.Line > 0, "every refusal is positioned")
				}
				assert.Length(t, refusals, 4,
					"two extend blocks and two groups, each refused and never dropped")

				var groups, extends int
				for _, msg := range refusals {
					switch {
					case strings.Contains(msg, "group"):
						groups++
					case strings.Contains(msg, "extends"):
						extends++
					}
				}
				assert.Equal(t, groups, 2, "the group in the message and the one in the oneof")
				assert.Equal(t, extends, 2, "the extend inside the message and the one beside it")
			})
		})
	})
}
