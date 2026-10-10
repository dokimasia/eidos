// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The schemas of the presence cases, the packages they declare, and the
// message whose fields the cases read.
const (
	presenceFile    = "svc/p.proto"
	presencePkg     = "svc.p"
	requiredFile    = "svc/q.proto"
	requiredPkg     = "svc.q"
	presenceMessage = "M"
)

// presenceSource declares a field of each form that the presence rule
// tells apart, in proto3, whose singular fields have no presence that the
// load states.
const presenceSource = `syntax = "proto3";

package svc.p;

import "google/protobuf/timestamp.proto";
import "google/protobuf/wrappers.proto";
import "google/protobuf/struct.proto";

message M {
  N message = 1;
  string scalar = 2;
  repeated N list = 3;
  map<string, N> table = 4;
  google.protobuf.Timestamp when = 5;
  google.protobuf.Int32Value wrapped = 6;
  E state = 7;
  google.protobuf.NullValue nothing = 8;
  optional N stated = 9;
  oneof choice {
    N member = 10;
  }
  message N {}
}

enum E {
  E_ZERO = 0;
}
`

// requiredSource declares a required message field of proto2.
const requiredSource = `syntax = "proto2";

package svc.q;

message M {
  required N field = 1;
  message N {}
}
`

// The allocations of a presence decision: none, whatever the field.
const presenceAllocs = 0

// A proto message field has presence in every version, also where its
// type does not contain it, so each form the rule tells apart is pinned.
func TestPresence(t *testing.T) {
	t.Parallel()

	t.Run("Presence", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			field string
			want  bool
		}{
			{name: "reports true for a singular field of a message", field: "message", want: true},
			{name: "reports true for a singular field of a well-known message", field: "when", want: true},
			{name: "reports false for a scalar field", field: "scalar"},
			{name: "reports false for a repeated field", field: "list"},
			{name: "reports false for a map field", field: "table"},
			{name: "reports false for a wrapper, whose shape is an optional", field: "wrapped"},
			{name: "reports false for a field of an enum", field: "state"},
			{name: "reports false for a field of the well-known enum", field: "nothing"},
			{name: "reports false for a field that the load gave the optional form", field: "stated"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loadedFrom(t, map[string]string{presenceFile: presenceSource})
				field := messageField(t, f, presencePkg, tt.field)
				assert.Equal(t, presenceRules(t).Presence(field, f.view), tt.want,
					"the field has presence where protobuf gives its type presence")
			})
		}

		t.Run("reports false for a member of a oneof", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, map[string]string{presenceFile: presenceSource})
			m, _ := f.decl(t, id(presencePkg, presenceMessage, symbol.KindStruct)).(*node.Struct)
			var choice *node.Sum
			for _, ty := range m.Types {
				if sum, is := ty.(*node.Sum); is {
					choice = sum
				}
			}
			assert.NotNil(t, choice, "the message declares the oneof")
			assert.False(t, presenceRules(t).Presence(choice.Variants[0].Fields[0], f.view),
				"the oneof's sum contains the member's presence")
		})

		t.Run("reports false for a required field", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, map[string]string{requiredFile: requiredSource})
			assert.False(t, presenceRules(t).Presence(messageField(t, f, requiredPkg, "field"), f.view),
				"a required field is always set")
		})

		t.Run("reports false for a missing field", func(t *testing.T) {
			t.Parallel()

			assert.False(t, presenceRules(t).Presence(nil, rules.View{}), "nothing has no presence")
		})

		t.Run("passes the kernel's fold of a message field an optional", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, map[string]string{presenceFile: presenceSource})
			shape := f.bound().FieldTypeOf(messageField(t, f, presencePkg, "message"))
			assert.Equal(t, shape.Form, symbol.FormOptional, "a proto3 message field translates with its presence")
		})
	})
}

// A presence decision reads the view and allocates nothing. The ordinary
// run, which runs no benchmark, checks that ceiling here.
func TestPresenceAllocs(t *testing.T) {
	checkAllocs(t, presenceCalls(t))
}

// BenchmarkPresence measures the decision that the kernel makes for every
// field that a generator translates.
func BenchmarkPresence(b *testing.B) {
	benchCalls(b, presenceCalls(b))
}

// presenceCalls returns a call of Presence over a message field and over
// a scalar field.
func presenceCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loadedFrom(tb, map[string]string{presenceFile: presenceSource})
	r := presenceRules(tb)
	message, scalar := messageField(tb, f, presencePkg, "message"), messageField(tb, f, presencePkg, "scalar")
	var present bool
	return []allocCall{
		{
			name: "Presence", caseName: "a message field", allocs: presenceAllocs,
			call:  func() { present = r.Presence(message, f.view) },
			check: func(tb assert.TB) { assert.True(tb, present, "Presence reports the message field") },
		},
		{
			name: "Presence", caseName: "a scalar field", allocs: presenceAllocs,
			call:  func() { present = r.Presence(scalar, f.view) },
			check: func(tb assert.TB) { assert.False(tb, present, "Presence reports no scalar field") },
		},
	}
}

// presenceRules returns protobuf's rules as the presence capability.
func presenceRules(tb assert.TB) rules.PresenceRules {
	tb.Helper()

	r, is := protorules.New().(rules.PresenceRules)
	assert.True(tb, is, "protobuf's rules report the presence of a field")
	return r
}

// messageField returns one field of the message M that a package
// declares.
func messageField(tb assert.TB, f *fixture, pkg, name string) *node.Field {
	tb.Helper()

	m, is := f.decl(tb, id(pkg, presenceMessage, symbol.KindStruct)).(*node.Struct)
	assert.True(tb, is, "M is a message")
	for _, field := range m.Fields {
		if field.Name == name {
			return field
		}
	}
	tb.Fatalf("M declares no field %s", name)
	return nil
}
