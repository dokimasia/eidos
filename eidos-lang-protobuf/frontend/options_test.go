// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/node"
)

// Options are the residue every declaration level can state, and
// edition features are the subset that decides what an absent value
// means, so the split between them is a contract.
func TestOptions(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the options a declaration states, at every level", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `syntax = "proto3";

package svc.store;

option go_package = "example.test/svc";

message Row {
  option deprecated = true;
  string name = 1 [deprecated = true];
}

enum Colour {
  option allow_alias = true;
  UNSET = 0 [deprecated = true];
}

service Store {
  option deprecated = true;
  rpc Get(Row) returns (Row) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}
`)
			assert.False(t, sink.Failed(), "the schema loads clean")
			file := onlyFile(t, gb)
			key := string(protobuf.OptionsKey)

			assert.Contains(t, stampsOn(gb, file, key)[0], "go_package=", "a file's options are stamped")
			row, _ := declOf(t, file, "Row").(*node.Struct)
			assert.Contains(t, stampsOn(gb, row, key)[0], "deprecated=", "a message's")
			assert.Contains(t, stampsOn(gb, row.Fields[0], key)[0], "deprecated=", "a field's")
			colour, _ := declOf(t, file, "Colour").(*node.Enum)
			assert.Contains(t, stampsOn(gb, colour, key)[0], "allow_alias=", "an enum's")
			assert.Contains(t, stampsOn(gb, colour.Variants[0], key)[0], "deprecated=", "a value's")
			store, _ := declOf(t, file, "Store").(*node.Interface)
			assert.Contains(t, stampsOn(gb, store, key)[0], "deprecated=", "a service's")
			assert.Contains(t, stampsOn(gb, store.Methods[0], key)[0], "idempotency_level=",
				"and an rpc's, which is where a transport reads its own")
		})

		t.Run("stamps edition features apart from the other options", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsed(t, `edition = "2024";

package svc.store;

option features.field_presence = IMPLICIT;
option go_package = "example.test/svc";

message Row {
  option features.field_presence = EXPLICIT;
  string name = 1 [features.field_presence = LEGACY_REQUIRED];
}
`)
			assert.False(t, sink.Failed(), "an edition file loads")
			file := onlyFile(t, gb)
			assert.Equal(t, stampsOn(gb, file, string(protobuf.SyntaxKey)), []string{"2024"},
				"the edition is stamped where the syntax keyword would be")

			features := string(protobuf.FeaturesKey)
			assert.Contains(t, stampsOn(gb, file, features)[0], "features.field_presence=IMPLICIT",
				"a file's features are stamped")
			assert.NotContains(t, stampsOn(gb, file, string(protobuf.OptionsKey))[0], "features.",
				"apart from its other options, because editions give features a meaning the others lack")
			row, _ := declOf(t, file, "Row").(*node.Struct)
			assert.Contains(t, stampsOn(gb, row, features)[0], "EXPLICIT", "a message's features are stamped")
			assert.Contains(t, stampsOn(gb, row.Fields[0], features)[0], "LEGACY_REQUIRED",
				"and a field's, which is where presence is finally decided")
		})
	})
}
