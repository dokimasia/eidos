// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
)

func TestKey(t *testing.T) {
	t.Parallel()

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			key  meta.KeyName
			want string
		}{
			{name: "a two-segment name", key: "shape.role", want: "shape"},
			{name: "a deeper name keeps only the first segment", key: "java.annotation.retention", want: "java"},
			{name: "a dotless name is all namespace", key: "shape", want: "shape"},
			{name: "the empty name owns nothing", key: "", want: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.key.Namespace(), tt.want,
					"the namespace is the segment before the first dot")
			})
		}
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		var unregistered meta.Key[string]
		assert.True(t, unregistered.IsZero(),
			"a key that was never registered names nothing")
		assert.Equal(t, unregistered.ID(), meta.KeyID(0),
			"and its id is the zero id")
	})

	t.Run("handle", func(t *testing.T) {
		t.Parallel()

		r := meta.NewRegistry()
		assert.NoError(t, r.ClaimNamespace("shape", "eidos-plugin-shape"),
			"the namespace claims")
		key, err := meta.Register[string](r, meta.KeySpec{
			Name: "shape.role", Doc: "the classified role",
		})
		assert.NoError(t, err, "the key registers")

		assert.Equal(t, key.Name(), meta.KeyName("shape.role"),
			"the handle returns its boundary spelling")
		assert.NotEqual(t, key.ID(), meta.KeyID(0),
			"and a dense id the gate tuples hold")
		assert.False(t, key.IsZero(), "a registered handle names its key")
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
			t.Fatalf("Namespace(%q) = %q, want the whole dotless name", in, ns)
		}
	})
}
