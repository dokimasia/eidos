// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// structID returns the identity a resolved struct of that name
// carries, for origins the fixtures below derive from.
func structID(name string) symbol.Identity {
	return symbol.Identity{
		Lang:    coretest.Lang,
		Package: coretest.StorePath,
		Name:    name,
		Kind:    symbol.KindStruct,
	}
}

// unit returns a minimal valid unit for one plugin and key.
func unit(p, key string) plugin.Unit {
	return plugin.Unit{
		Plugin: plugin.ID(p),
		Per:    plugin.PerSource,
		Word:   "stub",
		Key:    key,
		Pkg:    coretest.PackageID(coretest.StorePath),
	}
}

// The emit store is one plan's accumulated output: its refusals keep
// a phase call's flush honest, its enumeration order is total so two
// runs agree, and its kind index is what makes an
// emit-triggered rule cost only its matches.
func TestEmit(t *testing.T) {
	t.Parallel()

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("records a unit", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(unit("stubgen", "svc/store/unit.go")),
				"a valid unit arrives")

			var got []plugin.Unit
			for u := range e.Units() {
				got = append(got, u)
			}
			assert.Length(t, got, 1, "the store holds what arrived")
			assert.Equal(t, got[0].Key, "svc/store/unit.go",
				"the unit comes back as it was added")
		})

		t.Run("refuses a second unit under one accumulator", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(unit("stubgen", "svc/store/unit.go")),
				"the first flush arrives")
			assert.HasError(t, e.Add(unit("stubgen", "svc/store/unit.go")),
				"one phase call flushes each accumulator once")

			var got int
			for range e.Units() {
				got++
			}
			assert.Equal(t, got, 1, "the refused unit did not arrive")
		})

		t.Run("admits one key under two plugins", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(unit("stubgen", "svc/store/unit.go")),
				"the first plugin's unit arrives")
			assert.NoError(t, e.Add(unit("audit", "svc/store/unit.go")),
				"two plugins may contribute to one cardinality key")
		})

		t.Run("refuses a zero cardinality", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "svc/store/unit.go")
			u.Per = 0
			assert.HasError(t, plugin.NewEmit().Add(u),
				"a unit without a cardinality addresses nothing")
		})

		t.Run("refuses an empty word", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "svc/store/unit.go")
			u.Word = ""
			assert.HasError(t, plugin.NewEmit().Add(u),
				"a family without a word cannot be routed")
		})

		t.Run("refuses a plan unit naming a key", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "svc/store/unit.go")
			u.Per = plugin.PerPlan
			assert.HasError(t, plugin.NewEmit().Add(u),
				"a plan has one output, so a keyed plan unit is a defect")
		})
	})

	t.Run("Units", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a total order", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			second := unit("stubgen", "a.go")
			second.Tag = "test"
			for _, u := range []plugin.Unit{
				{Plugin: "stubgen", Per: plugin.PerPlan, Word: "registry"},
				second,
				unit("stubgen", "a.go"),
				unit("audit", "z.go"),
			} {
				assert.NoError(t, e.Add(u), "every fixture unit arrives")
			}

			var got []string
			for u := range e.Units() {
				got = append(got, string(u.Plugin)+"/"+u.Key+"/"+u.Tag)
			}
			assert.Equal(t, got, []string{
				"audit/z.go/",
				"stubgen/a.go/",
				"stubgen/a.go/test",
				"stubgen//",
			}, "units order by plugin, then cardinality, then key, then tag")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(unit("stubgen", "a.go")), "a unit arrives")
			assert.NoError(t, e.Add(unit("audit", "b.go")), "a second arrives")

			var got int
			for range e.Units() {
				got++
				break
			}
			assert.Equal(t, got, 1, "the iteration stops when the range stops")
		})
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		// carrying returns a unit holding one origined struct with
		// one origined method in its slot.
		carrying := func(p, key, structName string) plugin.Unit {
			s := &emit.Struct{Origin: structID(structName), Name: structName}
			s.Methods.Append(&emit.Method{Origin: structID(structName), Name: "Get"})
			u := unit(p, key)
			u.Decls = []symbol.Symbol{s}
			u.Origins = []symbol.Identity{structID(structName)}
			return u
		}

		t.Run("walks each unit's tree in Units order", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(carrying("stubgen", "b.go", "Store")),
				"the later-ordered unit arrives first")
			assert.NoError(t, e.Add(carrying("audit", "a.go", "Cache")),
				"the earlier-ordered unit arrives second")

			var structs []string
			for s := range e.ByKind(symbol.KindStruct) {
				st, ok := s.(*emit.Struct)
				assert.True(t, ok, "a struct enumeration yields structs")
				structs = append(structs, st.Name)
			}
			assert.Equal(t, structs, []string{"Cache", "Store"},
				"the enumeration follows Units order, never insertion order")

			var methods int
			for range e.ByKind(symbol.KindMethod) {
				methods++
			}
			assert.Equal(t, methods, 2,
				"a slotted declaration returns under its own kind")
		})

		t.Run("skips a value without an origin", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "a.go")
			u.Decls = []symbol.Symbol{
				&emit.Struct{Name: "NoOrigin"},
				&emit.File{Path: "a.go"},
				&emit.Struct{Origin: structID("Store"), Name: "Store"},
			}
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(u), "the unit arrives whole")

			var structs []string
			for s := range e.ByKind(symbol.KindStruct) {
				st, ok := s.(*emit.Struct)
				assert.True(t, ok, "a struct enumeration yields structs")
				structs = append(structs, st.Name)
			}
			assert.Equal(t, structs, []string{"Store"},
				"a value without an origin is not a subject")

			var files int
			for range e.ByKind(symbol.KindFile) {
				files++
			}
			assert.Equal(t, files, 0,
				"a kind without origin storage never returns")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(carrying("stubgen", "a.go", "Store")),
				"a carrying unit arrives")

			var got int
			for range e.ByKind(symbol.KindMethod) {
				got++
				break
			}
			assert.Equal(t, got, 1, "the iteration stops when the range stops")
		})
	})
}

// benchUnit returns one unit holding structs of methods, the tree
// Add walks and indexes.
func benchUnit(p, key string, structs, methods int) plugin.Unit {
	decls := make([]symbol.Symbol, 0, structs)
	for i := range structs {
		name := "Struct" + strconv.Itoa(i)
		s := &emit.Struct{Origin: structID(name), Name: name}
		for m := range methods {
			s.Methods.Append(&emit.Method{
				Origin: structID(name), Name: "Method" + strconv.Itoa(m),
			})
		}
		decls = append(decls, s)
	}
	u := unit(p, key)
	u.Decls = decls
	return u
}

func BenchmarkEmit(b *testing.B) {
	b.Run("Add", func(b *testing.B) {
		b.ReportAllocs()
		u := benchUnit("stubgen", "a.go", 100, 5)
		for b.Loop() {
			if err := plugin.NewEmit().Add(u); err != nil {
				b.Fatalf("Add: unexpected error: %v", err)
			}
		}
	})

	b.Run("ByKind", func(b *testing.B) {
		b.ReportAllocs()
		e := plugin.NewEmit()
		const units, methods = 100, 20
		for i := range units {
			if err := e.Add(benchUnit("stubgen", "unit"+strconv.Itoa(i)+".go", 1, methods)); err != nil {
				b.Fatalf("Add: unexpected error: %v", err)
			}
		}
		for b.Loop() {
			var n int
			for range e.ByKind(symbol.KindMethod) {
				n++
			}
			if n != units*methods {
				b.Fatalf("ByKind yielded %d methods", n)
			}
		}
	})

	b.Run("Units", func(b *testing.B) {
		b.ReportAllocs()
		e := plugin.NewEmit()
		const units = 1_000
		for i := range units {
			if err := e.Add(unit("stubgen", "unit"+strconv.Itoa(i)+".go")); err != nil {
				b.Fatalf("Add: unexpected error: %v", err)
			}
		}
		for b.Loop() {
			var n int
			for range e.Units() {
				n++
			}
			if n != units {
				b.Fatalf("Units yielded %d units", n)
			}
		}
	})
}
