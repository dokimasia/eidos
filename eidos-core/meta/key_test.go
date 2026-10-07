// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"encoding/binary"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/prop"

	"go.dokimi.dev/eidos/core/meta"
)

// namespaceIsFirstSegment is the property [FuzzKeyName] and its ForAll
// twin in [TestKey] state.
const namespaceIsFirstSegment = "Namespace must return the segment before a name's first dot, " +
	"and a name without a dot whole"

// A key's spelling resolves at the boundary and its handle fixes the
// value type, so the namespace a name spells and what a handle reports
// are contract.
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

		t.Run("returns the segment before the first dot of any name", func(t *testing.T) {
			t.Parallel()

			prop.ForAll(t, namespaceIsFirstSegment, namespacesFirstSegment)
		})
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

// A name and a handle report what they spell without allocating in the
// ordinary run, which runs no benchmark.
func TestKeyZeroAlloc(t *testing.T) {
	key := registered(t)
	name := key.Name()
	var namespace string
	assert.MaxAllocs(t, func() { namespace = name.Namespace() }, 0, "Namespace allocates nothing")
	assert.Equal(t, namespace, "shape", "Namespace returns the first segment")

	var spelled meta.KeyName
	assert.MaxAllocs(t, func() { spelled = key.Name() }, 0, "Name allocates nothing")
	assert.Equal(t, spelled, meta.KeyName("shape.role"), "Name returns the registered spelling")

	var id meta.KeyID
	assert.MaxAllocs(t, func() { id = key.ID() }, 0, "ID allocates nothing")
	assert.NotEqual(t, id, meta.KeyID(0), "ID returns the registered id")

	zero := true
	assert.MaxAllocs(t, func() { zero = key.IsZero() }, 0, "IsZero allocates nothing")
	assert.False(t, zero, "IsZero reports false for a registered key")
}

// BenchmarkKey measures what a name and a handle report: every directive
// parameter that names a key is checked against its namespace, and every
// read and write goes through a handle.
func BenchmarkKey(b *testing.B) {
	key := registered(b)

	b.Run("Namespace", func(b *testing.B) {
		name := key.Name()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = name.Namespace()
		}
		assert.Equal(b, got, "shape", "Namespace returns the first segment")
	})

	b.Run("Name", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got meta.KeyName
		for c.Loop() {
			got = key.Name()
		}
		assert.Equal(b, got, meta.KeyName("shape.role"), "Name returns the registered spelling")
	})

	b.Run("ID", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got meta.KeyID
		for c.Loop() {
			got = key.ID()
		}
		assert.NotEqual(b, got, meta.KeyID(0), "ID returns the registered id")
	})

	b.Run("IsZero", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := true
		for c.Loop() {
			got = key.IsZero()
		}
		assert.False(b, got, "IsZero reports false for a registered key")
	})
}

// FuzzKeyName checks [namespaceIsFirstSegment] on text nothing in this
// repository wrote: directive parameters arrive as arbitrary text and
// resolve through these names. Each seed is the choices of one case: a
// two-byte little-endian length, then the bytes of the name.
func FuzzKeyName(f *testing.F) {
	for _, seed := range []string{"shape.role", "shape", "", ".", "a.b.c", "..", "shape."} {
		f.Add(append(binary.LittleEndian.AppendUint16(nil, uint16(len(seed))), seed...))
	}

	prop.Fuzz(f, namespaceIsFirstSegment, namespacesFirstSegment)
}

// namespacesFirstSegment checks [namespaceIsFirstSegment] on a name
// that the case draws.
func namespacesFirstSegment(c *prop.Case) {
	in := string(c.Draw(prop.Bytes(), "name"))
	ns := meta.KeyName(in).Namespace()
	assert.HasPrefix(c, in, ns, "the name must start with its namespace")
	if dot := strings.IndexByte(in, '.'); dot >= 0 {
		assert.Equal(c, ns, in[:dot], "the namespace must be the segment before the first dot")
		return
	}
	assert.Equal(c, ns, in, "a name without a separator must be its own namespace")
}

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
