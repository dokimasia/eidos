// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Comments attribute the way protoc's source info attributes them,
// so the documentation, the trailing comment and the carriers of
// every kind are pinned.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("takes each kind's trailing comment where protoc does", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

import "dep/t.proto"; // the import

message Row { // the message
  string name = 1; // the field
  map<string, int64> counts = 2; // the map
  oneof body { // the oneof
    string text = 3; // the variant
  }
} // after the message

enum Colour { // the enum
  UNSET = 0; // the value
}

service Store { // the service
  rpc Get(Row) returns (Row); // the rpc without a body
  rpc Put(Row) returns (Row) { // the rpc body
    option idempotency_level = IDEMPOTENT;
  }
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			file := onlyFile(t, gb)
			assert.Equal(t, file.Imports[0].Comment, "the import", "an import takes its own")

			row, _ := declOf(t, file, "Row").(*node.Struct)
			assert.Equal(t, row.Comment, "the message",
				"a block declaration takes the comment after its opening brace, as protoc does")
			assert.Equal(t, row.Fields[0].Comment, "the field", "a field takes its own")
			assert.Equal(t, row.Fields[1].Comment, "the map", "a map field too")
			sum, _ := row.Types[0].(*node.Sum)
			assert.Equal(t, sum.Comment, "the oneof", "a oneof takes its own")
			assert.Equal(t, sum.Variants[0].Comment, "the variant", "and a member's variant repeats its field's")

			colour, _ := declOf(t, file, "Colour").(*node.Enum)
			assert.Equal(t, colour.Comment, "the enum", "an enum takes its own")
			assert.Empty(t, colour.Doc, "and the comment after the message's closing brace documents nothing")
			assert.Equal(t, colour.Variants[0].Comment, "the value", "and its values theirs")

			store, _ := declOf(t, file, "Store").(*node.Interface)
			assert.Equal(t, store.Comment, "the service", "a service takes its own")
			assert.Equal(t, store.Methods[0].Comment, "the rpc without a body", "an rpc its own")
			assert.Equal(t, store.Methods[1].Comment, "the rpc body", "and an rpc with a body the brace's")
		})

		t.Run("documents a declaration with the group directly above it", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `// Licence header, which a blank line detaches.

// The file's syntax.
syntax = "proto3";

// The package documentation.
package svc.store;

// Detached from Row by a blank line.

// Row is documented.
message Row {
  string a = 1;
  // Trails a, because a blank line follows it.

  // Documents b.
  string b = 2;
}

// Detached alone.

message Lone {}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			file := onlyFile(t, gb)
			assert.Equal(t, file.Doc, []string{"The file's syntax."},
				"the file takes the syntax statement's group, and the detached licence documents nothing")
			assert.Equal(t, gb.Packages()[0].Doc, []string{"The package documentation."},
				"the package takes the package statement's")
			row, _ := declOf(t, file, "Row").(*node.Struct)
			assert.Equal(t, row.Doc, []string{"Row is documented."},
				"a group a blank line separates from the declaration is detached")
			assert.Equal(t, row.Fields[0].Comment, "Trails a, because a blank line follows it.",
				"a group below a field and above a blank line trails the field")
			assert.Equal(t, row.Fields[1].Doc, []string{"Documents b."}, "and the next group documents b")
			lone, _ := declOf(t, file, "Lone").(*node.Struct)
			assert.Empty(t, lone.Doc, "and a lone group a blank line separates documents nothing")
		})

		t.Run("attributes a comment between two tokens on one line to neither", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

message Row {
  string a = 1; /* between */ string b = 2;
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)
			assert.Equal(t, row.Fields[0].Comment, "", "the comment trails neither the field before it")
			assert.Empty(t, row.Fields[1].Doc, "nor documents the field after it, as protoc drops it")
		})

		t.Run("strips a block comment's delimiters and gutter", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

/**
 * Row is a block comment,
 * its gutter stripped.
 */
message Row {}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			row, _ := declOf(t, onlyFile(t, gb), "Row").(*node.Struct)
			assert.Equal(t, row.Doc, []string{"Row is a block comment,", "its gutter stripped."},
				"one line per documentation line, the star at each line's start removed")
		})

		t.Run("attaches a carrier in a trailing comment and above the package", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

//+gen:table name=pkg
package svc.store;

message Row { //+gen:table name=rows
  string name = 1; //+gen:table name=col
}
`)
			assert.False(t, sink.Failed(), "every carrier has a subject")
			file := onlyFile(t, gb)
			row, _ := declOf(t, file, "Row").(*node.Struct)
			var subjects []symbol.Symbol
			for _, a := range gb.Attachments() {
				subjects = append(subjects, a.Subject)
			}
			assert.Length(t, subjects, 3, "three carriers attach")
			assert.True(t, slices.Contains(subjects, symbol.Symbol(gb.Packages()[0])),
				"a carrier above the package statement to the package")
			assert.True(t, slices.Contains(subjects, symbol.Symbol(row)),
				"one after a message's opening brace to the message")
			assert.True(t, slices.Contains(subjects, symbol.Symbol(row.Fields[0])),
				"and one after a field to the field")
			assert.Equal(t, row.Comment, "", "and a carrier is no comment text")
		})

		t.Run("reports a carrier on an import, which has no identity", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

//+gen:table name=imp
import "dep/t.proto";
`)
			assert.True(t, slices.Contains(codesOf(sink), protofrontend.UnaddressedCarrier),
				"the carrier reports, never dropped without a finding")
			assert.Empty(t, gb.Attachments(), "and attaches nothing")
		})

		t.Run("spells an enum number in every form proto admits", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto2";

package svc.store;

enum Wide {
  WIDE_ZERO = 0;
  WIDE_NEGATIVE = -1;
  WIDE_HEX = 0x1F;
  WIDE_HUGE = 9223372036854775807;
}
`)
			assert.False(t, sink.Failed(), "every number the grammar admits loads")
			wide, _ := declOf(t, onlyFile(t, gb), "Wide").(*node.Enum)
			var values []string
			for _, v := range wide.Variants {
				values = append(values, v.Value)
			}
			assert.Equal(t, values, []string{"0", "-1", "0x1F", "9223372036854775807"},
				"each value keeps the spelling the schema wrote")
		})
	})
}
