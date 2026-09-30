// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
)

// registered returns the handle of one key registered under the
// shape namespace.
func registered(tb assert.TB) meta.Key[string] {
	tb.Helper()

	key, err := meta.Register[string](claimed(tb), meta.KeySpec{
		Name: "shape.role", Doc: "the classified role",
	})
	assert.NoError(tb, err, "the key registers")
	return key
}

func TestKey(t *testing.T) {
	t.Parallel()

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give meta.KeyName
			want string
		}{
			{name: "returns the first segment of a two-segment name", give: "shape.role", want: "shape"},
			{name: "returns the first segment of a deeper name", give: "java.annotation.retention", want: "java"},
			{name: "returns a name without a separator whole", give: "shape", want: "shape"},
			{name: "returns the empty string for the empty name", give: "", want: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.Namespace(), tt.want, "the namespace is the segment before the first dot")
			})
		}
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the registered spelling", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, registered(t).Name(), meta.KeyName("shape.role"), "the name is the spelling")
		})
	})

	t.Run("ID", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a nonzero id for a registered key", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, registered(t).ID(), meta.KeyID(0), "the id is nonzero")
		})

		t.Run("returns the zero id for a key never registered", func(t *testing.T) {
			t.Parallel()

			var unregistered meta.Key[string]
			assert.Equal(t, unregistered.ID(), meta.KeyID(0), "the id is zero")
		})
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false for a registered key", func(t *testing.T) {
			t.Parallel()

			assert.False(t, registered(t).IsZero(), "the key names something")
		})

		t.Run("reports true for a key never registered", func(t *testing.T) {
			t.Parallel()

			var unregistered meta.Key[string]
			assert.True(t, unregistered.IsZero(), "the key names nothing")
		})
	})
}

// FuzzKeyName drives the boundary spelling with bytes nothing in
// this repository wrote: directive parameters arrive as arbitrary
// text and resolve through these names.
func FuzzKeyName(f *testing.F) {
	for _, seed := range []string{"shape.role", "shape", "", ".", "a.b.c", "..", "shape."} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		name := meta.KeyName(in)
		ns := name.Namespace()

		if !strings.HasPrefix(in, ns) {
			t.Fatalf("Namespace(%q) = %q, which the name does not start with", in, ns)
		}
		if dot := strings.IndexByte(in, '.'); dot >= 0 && ns != in[:dot] {
			t.Fatalf("Namespace(%q) = %q, want the segment before the first dot %q",
				in, ns, in[:dot])
		} else if dot < 0 && ns != in {
			t.Fatalf("Namespace(%q) = %q, want the whole name without a separator", in, ns)
		}
	})
}
