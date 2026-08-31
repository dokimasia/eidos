// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package naming_test

import (
	"testing"
	"unicode"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/naming"
)

// The sanitiser stands between arbitrary source text and a file
// that has to compile, so its contract is total: there is no
// input it may refuse, and every output has to be a spelling a
// compiler accepts.
func TestIdentifier(t *testing.T) {
	t.Parallel()

	t.Run("sanitises what a compiler would refuse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			in   string
			want string
		}{
			{"empty input spells one underscore", "", "_"},
			{"an identifier passes through", "userID", "userID"},
			{"underscores survive", "user_id", "user_id"},
			{"anything else becomes an underscore", "user-id.v2", "user_id_v2"},
			{"a leading digit gains a prefix", "2things", "_2things"},
			{"a digit elsewhere stands", "thing2things", "thing2things"},
			{"letters outside ASCII survive", "héllo_wörld", "héllo_wörld"},
			{"symbols alone spell underscores", "!!!", "___"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, naming.Identifier(tt.in), tt.want, tt.name)
			})
		}
	})

	t.Run("refuses no input", func(t *testing.T) {
		t.Parallel()

		for _, in := range []string{
			"", " ", "123", "\xbe", "a\xbeb", "!@#$%^&*()", "\t\n", "ゼロ", "_",
		} {
			got := naming.Identifier(in)
			assert.True(t, got != "", "every input spells something: "+in)
			for i, r := range got {
				valid := r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r))
				assert.True(t, valid,
					"and every rune of it is one an identifier may carry: "+got)
			}
		}
	})
}
