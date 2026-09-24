// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// linked loads a tree through the frontend and the resolution
// phase, so a case reads the declaration a reference binds, not its
// candidates.
func linked(tb assert.TB, files map[string]string) *store.Graph {
	tb.Helper()

	tree := fstest.MapFS{}
	for path, body := range files {
		tree[path] = &fstest.MapFile{Data: []byte(body)}
	}
	return rulestest.Loaded(tb, protofrontend.New(), tree, protobuf.Keys).Graph
}

// targets returns each field's name against the identity its type
// resolved to, so a case reads the whole message at once.
func targets(tb assert.TB, g *store.Graph, pkg, message string) map[string]symbol.Identity {
	tb.Helper()

	sym, held := g.Lookup(symbol.Identity{
		Lang: protobuf.Lang, Package: pkg, Name: message, Kind: symbol.KindStruct,
	})
	assert.True(tb, held, "the fixture declares "+pkg+"."+message)
	out := map[string]symbol.Identity{}
	for _, f := range sym.(*node.Struct).Fields {
		out[f.Name] = f.Type.Target
	}
	return out
}

// nested returns the identity of a message declared inside another.
func nested(pkg, owner, name string) symbol.Identity {
	return symbol.Identity{
		Lang: protobuf.Lang, Package: pkg, Owner: owner, Name: name, Kind: symbol.KindStruct,
	}
}

// Resolution is half of what a frontend does, and protobuf's rules
// are protoc's: innermost scope outward, a leading dot rooted, and a
// dotted name split wherever the graph declares it.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a nested message every way a schema addresses it", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{
				"svc/store.proto": `syntax = "proto3";
package svc.store;
import "dep/target.proto";
message Row {
  message Key { string id = 1; }
  Key bare = 1;
  Row.Key relative = 2;
  .svc.store.Row.Key rooted = 3;
  dep.Target sibling = 4;
  dep.Target.Inner nested_sibling = 5;
  .dep.Target.Inner rooted_sibling = 6;
}
`,
				"dep/target.proto": `syntax = "proto3";
package dep;
message Target {
  message Inner { string v = 1; }
  int64 id = 1;
}
`,
			})
			got := targets(t, g, "svc.store", "Row")
			key := nested("svc.store", "Row", "Key")
			inner := nested("dep", "Target", "Inner")
			assert.Equal(t, got["bare"], key,
				"a bare name resolves in the message it is written in, which is protoc's innermost scope")
			assert.Equal(t, got["relative"], key, "a relative name resolves to the same declaration")
			assert.Equal(t, got["rooted"], key, "and so does the fully-qualified form")
			assert.Equal(t, got["sibling"],
				symbol.Identity{Lang: protobuf.Lang, Package: "dep", Name: "Target", Kind: symbol.KindStruct},
				"a cross-package reference resolves")
			assert.Equal(t, got["nested_sibling"], inner,
				"a nested one does too, though the name states no boundary between namespace and owner")
			assert.Equal(t, got["rooted_sibling"], inner, "rooted or not")
		})

		t.Run("resolves a sub-package of the file's own namespace", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{
				"acme/row.proto": `syntax = "proto3";
package acme;
import "acme/v1/user.proto";
message Row { v1.User owner = 1; }
`,
				"acme/v1/user.proto": `syntax = "proto3";
package acme.v1;
message User { string id = 1; }
`,
			})
			assert.Equal(t, targets(t, g, "acme", "Row")["owner"],
				symbol.Identity{Lang: protobuf.Lang, Package: "acme.v1", Name: "User", Kind: symbol.KindStruct},
				"v1.User from acme splits into the package acme.v1 and the message User")
		})

		t.Run("prefers the innermost scope where two scopes declare one name", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{
				"svc/store.proto": `syntax = "proto3";
package svc.store;
message Key { string outer = 1; }
message Row {
  message Key { string inner = 1; }
  Key which = 1;
}
message Other {
  Key which = 1;
}
`,
			})
			row := targets(t, g, "svc.store", "Row")
			assert.Equal(t, row["which"], nested("svc.store", "Row", "Key"),
				"inside Row, Key is Row's own")
			other := targets(t, g, "svc.store", "Other")
			assert.Equal(t, other["which"],
				symbol.Identity{Lang: protobuf.Lang, Package: "svc.store", Name: "Key", Kind: symbol.KindStruct},
				"outside it, the same spelling is the file's own")
		})

		t.Run("resolves a oneof member in its message, because a oneof is no scope", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{
				"svc/store.proto": `syntax = "proto3";
package svc.store;
message Text { string v = 1; }
message Row {
  oneof body {
    Text note = 1;
    string Text = 2;
  }
}
`,
			})
			row, _ := g.Lookup(nested("svc.store", "", "Row"))
			member := row.(*node.Struct).Types[0].(*node.Sum).Variants[0].Fields[0]
			assert.Equal(t, member.Type.Target,
				symbol.Identity{Lang: protobuf.Lang, Package: "svc.store", Name: "Text", Kind: symbol.KindStruct},
				"a type spelled like a sibling member's variant resolves to the message, not the variant")
		})

		t.Run("resolves outward through an enclosing namespace", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{
				"a.proto": `syntax = "proto3";
package svc;
message Shared { string v = 1; }
`,
				"b.proto": `syntax = "proto3";
package svc.store;
import "a.proto";
message Row { Shared up = 1; }
`,
			})
			got := targets(t, g, "svc.store", "Row")
			assert.Equal(t, got["up"],
				symbol.Identity{Lang: protobuf.Lang, Package: "svc", Name: "Shared", Kind: symbol.KindStruct},
				"a name the file's own namespace lacks resolves in the namespace above it")
		})

		t.Run("leaves a scalar, a well-known type and an undeclared message unresolved", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{
				"a.proto": `syntax = "proto3";
package svc;
import "google/protobuf/timestamp.proto";
message Row {
  string scalar = 1;
  google.protobuf.Timestamp when = 2;
  .google.protobuf.Timestamp rooted_when = 3;
  some.Ghost missing = 4;
}
`,
				"google/protobuf/timestamp.proto": `syntax = "proto3";
package google.protobuf;
message Timestamp { int64 seconds = 1; int32 nanos = 2; }
`,
			})
			got := targets(t, g, "svc", "Row")
			assert.True(t, got["scalar"].IsZero(),
				"a scalar names no declaration and keeps its spelling")
			assert.True(t, got["when"].IsZero(),
				"a well-known type keeps its spelling even where the workspace declares it, so the rules map it")
			assert.True(t, got["rooted_when"].IsZero(), "rooted or not")
			assert.True(t, got["missing"].IsZero(),
				"and a message nothing declares degrades visibly and never fails")
		})

		t.Run("names the candidates in protoc's probe order, one tier per scope", func(t *testing.T) {
			t.Parallel()

			f := protofrontend.New()
			assert.Empty(t, f.Resolve(scopeOf("svc.store", ""), "int64"),
				"a scalar names no declaration, so no candidate returns")
			assert.Empty(t, f.Resolve(scopeOf("svc.store", ""), "   "),
				"and an empty spelling returns none either")

			rooted := f.Resolve(scopeOf("svc.store", ""), ".dep.Target.Inner")
			assert.Length(t, rooted, 1, "a rooted name probes one tier")
			assert.Equal(t, rooted[0][0], symbol.Identity{Lang: protobuf.Lang, Package: "dep.Target", Name: "Inner"},
				"split longest-namespace first, so a package takes precedence over a nested type")

			inner := f.Resolve(scopeOf("svc.store", "Row"), "Key")
			assert.Contains(t, inner[0],
				symbol.Identity{Lang: protobuf.Lang, Package: "svc.store", Owner: "Row", Name: "Key"},
				"the message a reference is written in is the first tier")
			assert.Contains(t, inner[1],
				symbol.Identity{Lang: protobuf.Lang, Package: "svc.store", Name: "Key"},
				"and the package a later one, so an outer Key never makes the inner one ambiguous")

			sum := scopeOf("svc.store", "")
			sum.Owner = symbol.Identity{
				Lang: protobuf.Lang, Package: "svc.store", Owner: "Row", Name: "body", Kind: symbol.KindSum,
			}
			assert.Contains(t, f.Resolve(sum, "Key")[0],
				symbol.Identity{Lang: protobuf.Lang, Package: "svc.store", Owner: "Row", Name: "Key"},
				"a reference in a oneof starts at the message that declares the oneof")
		})
	})
}
