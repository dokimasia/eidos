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
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The import fixture: the file of another package's message, the
// proto root a workspace can place the import paths under, and the
// imports that name a file other than the declaring one.
const (
	depFile       = "dep/target.proto"
	protoRoot     = "protos/"
	segmentFile   = "mydep/target.proto"
	targetImport  = "target.proto"
	unknownImport = "other.proto"
)

// importSource declares a message whose fields name a message of
// depFile, directly and as a list's element, and a message of its
// own file.
const importSource = `syntax = "proto3";
package svc.store;
import "dep/target.proto";
message Row {
  dep.Target one = 1;
  repeated dep.Target many = 2;
  Local local = 3;
}
message Local { string v = 1; }
`

// depSource declares the message importSource imports.
const depSource = `syntax = "proto3";
package dep;
message Target { int64 id = 1; }
`

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

// typesOf returns each field's type reference by the field's name,
// so a case reads the whole message at once.
func typesOf(tb assert.TB, g *store.Graph, pkg, message string) map[string]*node.TypeRef {
	tb.Helper()

	sym, found := g.Lookup(symbol.Identity{
		Lang: protobuf.Lang, Package: pkg, Name: message, Kind: symbol.KindStruct,
	})
	assert.True(tb, found, "the fixture declares "+pkg+"."+message)
	out := map[string]*node.TypeRef{}
	for _, f := range sym.(*node.Struct).Fields {
		out[f.Name] = f.Type
	}
	return out
}

// targets returns each field's name against the identity its type
// resolved to.
func targets(tb assert.TB, g *store.Graph, pkg, message string) map[string]symbol.Identity {
	tb.Helper()

	out := map[string]symbol.Identity{}
	for name, ref := range typesOf(tb, g, pkg, message) {
		out[name] = ref.Target
	}
	return out
}

// nested returns the identity of a message declared inside another.
func nested(pkg, owner, name string) symbol.Identity {
	return symbol.Identity{
		Lang: protobuf.Lang, Package: pkg, Owner: owner, Name: name, Kind: symbol.KindStruct,
	}
}

// importer returns the frontend in its importer role.
func importer(tb assert.TB) plugin.Importer {
	tb.Helper()

	imp, is := protofrontend.New().(plugin.Importer)
	assert.True(tb, is, "the frontend names the imports of a file")
	return imp
}

// importing returns the scope of a file in fixturePkg that states
// the given imports, in source order.
func importing(imports ...string) plugin.ImportScope {
	return plugin.ImportScope{Bindings: protofrontend.BindingsFor(fixturePkg, imports...)}
}

// Resolution is half of what a frontend does, and protobuf's rules
// are protoc's: innermost scope outward, a leading dot rooted, and a
// dotted name split wherever the graph declares it. A resolved
// reference to another file's declaration records the import that
// names the file.
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
			assert.Equal(t, got["rooted"], key, "the fully-qualified form resolves to it too")
			assert.Equal(t, got["sibling"],
				symbol.Identity{Lang: protobuf.Lang, Package: "dep", Name: "Target", Kind: symbol.KindStruct},
				"a cross-package reference resolves")
			assert.Equal(t, got["nested_sibling"], inner,
				"a nested one resolves though the name states no boundary between namespace and owner")
			assert.Equal(t, got["rooted_sibling"], inner, "a rooted nested one resolves")
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

		t.Run("resolves a oneof member in the message that declares the oneof", func(t *testing.T) {
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
				"a type spelled like a sibling member's variant resolves to the message, because a oneof is no scope")
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

		unresolved := linked(t, map[string]string{
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
		tests := []struct {
			name  string
			field string
		}{
			{name: "leaves a scalar unresolved", field: "scalar"},
			{name: "leaves a well-known type unresolved where the workspace declares it", field: "when"},
			{name: "leaves a rooted well-known type unresolved", field: "rooted_when"},
			{name: "leaves a message nothing declares unresolved", field: "missing"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.True(t, targets(t, unresolved, "svc", "Row")[tt.field].IsZero(),
					"the reference keeps its spelling for the rules to classify")
			})
		}

		t.Run("returns no candidate for a scalar", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, protofrontend.New().Resolve(scopeOf("svc.store", ""), "int64"),
				"a scalar names no declaration")
		})

		t.Run("returns no candidate for an empty spelling", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, protofrontend.New().Resolve(scopeOf("svc.store", ""), "   "), "nothing to probe")
		})

		t.Run("returns one tier for a rooted name", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, protofrontend.New().Resolve(scopeOf("svc.store", ""), ".dep.Target.Inner"), 1,
				"a leading dot probes nothing outward")
		})

		t.Run("splits a rooted name at the longest package first", func(t *testing.T) {
			t.Parallel()

			rooted := protofrontend.New().Resolve(scopeOf("svc.store", ""), ".dep.Target.Inner")
			assert.Equal(t, rooted[0][0], symbol.Identity{Lang: protobuf.Lang, Package: "dep.Target", Name: "Inner"},
				"a package takes precedence over a nested type")
		})

		t.Run("returns the enclosing message as the first tier", func(t *testing.T) {
			t.Parallel()

			inner := protofrontend.New().Resolve(scopeOf("svc.store", "Row"), "Key")
			assert.Contains(t, inner[0],
				symbol.Identity{Lang: protobuf.Lang, Package: "svc.store", Owner: "Row", Name: "Key"},
				"the message a reference is written in")
		})

		t.Run("returns the package in a tier after the enclosing message", func(t *testing.T) {
			t.Parallel()

			inner := protofrontend.New().Resolve(scopeOf("svc.store", "Row"), "Key")
			assert.Contains(t, inner[1],
				symbol.Identity{Lang: protobuf.Lang, Package: "svc.store", Name: "Key"},
				"an outer Key never makes the inner one ambiguous")
		})

		t.Run("starts a reference in a oneof at the message that declares the oneof", func(t *testing.T) {
			t.Parallel()

			sum := scopeOf("svc.store", "")
			sum.Owner = symbol.Identity{
				Lang: protobuf.Lang, Package: "svc.store", Owner: "Row", Name: "body", Kind: symbol.KindSum,
			}
			assert.Contains(t, protofrontend.New().Resolve(sum, "Key")[0],
				symbol.Identity{Lang: protobuf.Lang, Package: "svc.store", Owner: "Row", Name: "Key"},
				"a oneof is no scope")
		})
	})

	t.Run("ImportOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the import naming the declaring file's workspace path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importer(t).ImportOf(importing(unknownImport, depFile), depFile), depFile,
				"the import is the path")
		})

		t.Run("returns the import naming the declaring file below a proto root", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importer(t).ImportOf(importing(depFile), protoRoot+depFile), depFile,
				"the import is the path's suffix after a directory boundary")
		})

		t.Run("returns the first import in source order that names the declaring file", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importer(t).ImportOf(importing(depFile, targetImport), depFile), depFile,
				"an exact import before a suffix import")
		})

		t.Run("returns the declaring file's path where no import names it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importer(t).ImportOf(importing(unknownImport), depFile), depFile,
				"the workspace path, which protoc rejects as a missing import")
		})

		t.Run("returns the declaring file's path where an import matches inside a path segment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importer(t).ImportOf(importing(depFile), segmentFile), segmentFile,
				"dep/target.proto names no file mydep/target.proto")
		})

		t.Run("returns the declaring file's path for a scope without bindings", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importer(t).ImportOf(plugin.ImportScope{}, depFile), depFile,
				"no recorded imports to match")
		})

		t.Run("sets the package of a loaded reference to the import naming its declaring file", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{fixturePath: importSource, depFile: depSource})
			assert.Equal(t, typesOf(t, g, fixturePkg, "Row")["one"].Package, depFile,
				"the load asks the frontend for the import")
		})

		t.Run("sets the package of a structural reference's named child", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{fixturePath: importSource, depFile: depSource})
			many := typesOf(t, g, fixturePkg, "Row")["many"]
			assert.Empty(t, many.Package, "the list names no package of its own")
			assert.Equal(t, many.Elems[0].Package, depFile, "its element names the import")
		})

		t.Run("sets the package of a loaded reference below a proto root to the import", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{protoRoot + fixturePath: importSource, protoRoot + depFile: depSource})
			assert.Equal(t, typesOf(t, g, fixturePkg, "Row")["one"].Package, depFile,
				"the import as the file writes it, not the workspace path")
		})

		t.Run("leaves the package of a reference to its own file's declaration empty", func(t *testing.T) {
			t.Parallel()

			g := linked(t, map[string]string{fixturePath: importSource, depFile: depSource})
			local := typesOf(t, g, fixturePkg, "Row")["local"]
			assert.Equal(t, local.Target, nested(fixturePkg, "", "Local"), "the reference resolves")
			assert.Empty(t, local.Package, "a file imports no declaration of its own")
		})
	})
}
