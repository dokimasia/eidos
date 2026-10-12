// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/toolchain"
)

// rowDoc is the doc comment of the documented declarations of the cases.
const rowDoc = "Row is one record."

// A consumer reads the doc comment of each public declaration of
// generated output, so the check requires one at file level and nowhere
// else.
func TestDocumented(t *testing.T) {
	t.Parallel()

	t.Run("AssertDocumented", func(t *testing.T) {
		t.Parallel()

		t.Run("passes public declarations with doc comments", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertDocumented(t, literal{Scripted: frontendtest.NewScripted(), decls: documented(true)},
				toolchain.Generated{Files: map[string][]byte{apiFile: nil}})
		})

		t.Run("fails each public declaration without a doc comment", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "public declarations without doc comments", func(tb assert.TB) {
				toolchain.AssertDocumented(tb, literal{Scripted: frontendtest.NewScripted(), decls: documented(false)},
					toolchain.Generated{Files: map[string][]byte{apiFile: nil}})
			})
			lang := string(frontendtest.ScriptedLang)
			assert.Equal(t, coretest.Contracts(got), []string{
				lang + ": the public Struct Row in " + apiFile + " has a doc comment",
				lang + ": the public Function Load in " + apiFile + " has a doc comment",
				lang + ": the public Method Row.Touch in " + apiFile + " has a doc comment",
			}, "the rejection reports each declaration with its kind, its name and its file")
		})

		t.Run("stops the test where the output declares nothing public", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "output without a public declaration", func(tb assert.TB) {
				toolchain.AssertDocumented(tb, literal{Scripted: frontendtest.NewScripted(), decls: hidden},
					toolchain.Generated{Files: map[string][]byte{apiFile: nil}})
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{string(frontendtest.ScriptedLang) + ": the generated output declares something public"},
				"the rejection reports the output without a public declaration")
		})

		t.Run("stops the test where the load reports an Error", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "output that does not load", func(tb assert.TB) {
				toolchain.AssertDocumented(tb, frontendtest.NewScripted(),
					toolchain.Generated{Files: map[string][]byte{badFile: []byte("type Lost string\n")}})
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{string(frontendtest.ScriptedLang) + ": the generated output loads through its frontend"},
				"the rejection reports the load")
		})

		t.Run("stops the test without a frontend", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a check without a frontend", func(tb assert.TB) {
				toolchain.AssertDocumented(tb, nil, output())
			})
			assert.Equal(t, coretest.Contracts(got), []string{"toolchain: a frontend loads the generated output"},
				"the rejection reports the missing frontend")
		})
	})
}

// documented returns the declarations of a file: a public struct with an
// undocumented field and an undocumented method, a public function, a
// public method at file level, and an undocumented struct that is not
// public. Where docs is set, the public declarations at file level have
// doc comments.
func documented(docs bool) func(path, content string) []symbol.Symbol {
	return func(path, _ string) []symbol.Symbol {
		at := position.Pos{File: path, Line: 1}
		var doc []string
		if docs {
			doc = []string{rowDoc}
		}
		public := symbol.VisibilityPublic
		return []symbol.Symbol{
			&node.Struct{
				Name: "Row", Pos: at, Visibility: public, Doc: doc,
				Fields:  []*node.Field{{Name: "ID", Visibility: public, Type: &node.TypeRef{Spelling: "string"}}},
				Methods: []*node.Method{{Name: "Get", Visibility: public}},
			},
			&node.Function{Name: "Load", Pos: at, Visibility: public, Doc: doc},
			&node.Method{
				Name:       "Touch",
				Pos:        at,
				Visibility: public,
				Doc:        doc,
				Receives:   &node.TypeRef{Spelling: "Row"},
			},
			&node.Struct{Name: "inner", Pos: at, Visibility: symbol.VisibilityPrivate},
		}
	}
}
