// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Type expressions carry their verbatim spellings with arguments
// split for instantiations, and the reference shapes the corpus
// leans on are pinned here through parsed fields.
func TestTypeRef(t *testing.T) {
	t.Parallel()

	fieldsOf := func(tb assert.TB, src string) []*node.Field {
		tb.Helper()
		file := onlyFile(tb, parsedFile(tb, nil, plugin.DepthFull, src))
		return file.Decls[0].(*node.Struct).Fields
	}

	t.Run("splits instantiation arguments off the bare name", func(t *testing.T) {
		t.Parallel()

		fields := fieldsOf(t,
			"package p\n\ntype H struct {\n\ta List[int]\n\tb Pair[int, string]\n}\n")
		assert.Equal(t, fields[0].Type.Spelling, "List", "the bare name alone")
		assert.Length(t, fields[0].Type.Args, 1, "one argument split out")
		assert.Equal(t, fields[0].Type.Args[0].Spelling, "int", "as its own reference")
		assert.Length(t, fields[1].Type.Args, 2, "and two for a pair")
	})

	t.Run("keeps composites verbatim, decoration and all", func(t *testing.T) {
		t.Parallel()

		fields := fieldsOf(t,
			"package p\n\ntype H struct {\n\ta map[string]int\n\tb []*User\n\tc (chan int)\n}\n")
		assert.Equal(t, fields[0].Type.Spelling, "map[string]int", "a map is one spelling")
		assert.Equal(t, fields[1].Type.Spelling, "[]*User", "decoration stays in it")
		assert.Equal(t, fields[2].Type.Spelling, "chan int",
			"parentheses unwrap before the spelling is taken")
	})

	t.Run("positions a reference at its expression", func(t *testing.T) {
		t.Parallel()

		fields := fieldsOf(t, "package p\n\ntype H struct {\n\ta int\n}\n")
		assert.Equal(t, fields[0].Type.Pos.Line, 4, "at the type's own line")
	})
}
