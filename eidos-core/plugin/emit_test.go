// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The shape of the units the benchmarks add: how many a store holds
// before the measured add, and the structs and methods of each.
const (
	benchUnits   = 100
	benchStructs = 10
	benchMethods = 5
)

// The allocations of a store, of an add and of a contribution.
const (
	// newEmitAllocs is an empty store: the store, its set of accumulator
	// keys and its map of kinds.
	newEmitAllocs = 3
	// firstContributeAllocs is the first contribution to a store of one
	// hosting unit: the map from each indexed declaration to its unit,
	// with its first group, and the unit's list of contributors.
	firstContributeAllocs = 3
	// firstAddAllocs is the first unit of an empty store: the first
	// group of the set of accumulator keys and the key, which the set
	// stores apart because a reference is larger than 128 bytes, the
	// list of units, the first group of the map of kinds, and for each of
	// the unit's two kinds the kind's map of units with its first group
	// and the unit's declarations of the kind.
	firstAddAllocs = 2 + 1 + 1 + 2*3
	// laterAddAllocs is a unit of a store that has its kinds: the
	// accumulator's key, and the unit's declarations of each of its two
	// kinds. The containers grow by doubling, which the mean rounds
	// down.
	laterAddAllocs = 1 + 2
)

// The emit store is one plan's accumulated output: its refusals keep
// a phase call's flush honest, its enumeration order is total so two
// runs agree, and its kind index is what makes an
// emit-triggered rule cost only its matches.
func TestEmit(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Cardinality
			want string
		}{
			{name: "returns per-source for PerSource", give: plugin.PerSource, want: "per-source"},
			{name: "returns per-package for PerPackage", give: plugin.PerPackage, want: "per-package"},
			{name: "returns per-plan for PerPlan", give: plugin.PerPlan, want: "per-plan"},
			{name: "returns the number of the zero cardinality", give: 0, want: "Cardinality(0)"},
			{
				name: "returns every digit of an undeclared cardinality's number",
				give: plugin.Cardinality(12), want: "Cardinality(12)",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("FileKey", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Unit
			want string
		}{
			{
				name: "returns the source path of a per-source unit",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go"},
				want: "svc/store.go",
			},
			{
				name: "returns the empty string for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: coretest.StorePath},
				want: "",
			},
			{
				name: "returns the empty string for a per-plan unit",
				give: plugin.Unit{Per: plugin.PerPlan},
				want: "",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.FileKey(), tt.want, "the filename's stem source")
			})
		}
	})

	t.Run("Unit.Ref", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the key the unit flushes under", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "svc/store/a.go")
			u.Tag = "test"
			assert.Equal(t, u.Ref(), plugin.UnitRef{
				Plugin: "stubgen", Tag: "test", Pkg: coretest.PackageID(coretest.StorePath), Key: "svc/store/a.go",
			}, "the reference names the plugin, the tag, the package and the key")
		})
	})

	t.Run("NewEmit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a store without a unit", func(t *testing.T) {
			t.Parallel()

			var got int
			for range plugin.NewEmit().Units() {
				got++
			}
			assert.Equal(t, got, 0, "nothing has arrived")
		})
	})

	t.Run("Settled", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false before the settle", func(t *testing.T) {
			t.Parallel()

			assert.False(t, plugin.NewEmit().Settled(), "a new store has not settled")
		})

		t.Run("reports true after the settle", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, plugin.Settle(e, nil, nil, nil), "a store without a backend settles")
			assert.True(t, e.Settled(), "the settle marks the store")
		})
	})

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
			assert.Length(t, got, 1, "the store contains what arrived")
			assert.Equal(t, got[0].Key, "svc/store/unit.go",
				"the unit comes back as it was added")
		})

		t.Run("returns an error for a second unit under one accumulator", func(t *testing.T) {
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

		t.Run("records one key under two plugins", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(unit("stubgen", "svc/store/unit.go")),
				"the first plugin's unit arrives")
			assert.NoError(t, e.Add(unit("audit", "svc/store/unit.go")),
				"two plugins may contribute to one cardinality key")
		})

		t.Run("records one key under two packages", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			native := unit("stubgen", coretest.StorePath)
			native.Per = plugin.PerPackage
			foreign := native
			foreign.Pkg.Lang = "other"
			assert.NoError(t, e.Add(native), "the first language's package unit arrives")
			assert.NoError(t, e.Add(foreign),
				"a second language spelling the one package path is a second namespace")
		})

		t.Run("returns an error for a zero cardinality", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "svc/store/unit.go")
			u.Per = 0
			assert.HasError(t, plugin.NewEmit().Add(u),
				"a unit without a cardinality addresses nothing")
		})

		t.Run("returns an error for an empty word", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "svc/store/unit.go")
			u.Word = ""
			assert.HasError(t, plugin.NewEmit().Add(u),
				"a family without a word cannot be routed")
		})

		t.Run("returns an error for a plan unit naming a key", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "svc/store/unit.go")
			u.Per = plugin.PerPlan
			assert.HasError(t, plugin.NewEmit().Add(u),
				"a plan has one output, so a keyed plan unit is a defect")
		})

		t.Run("returns an error for a nil declaration", func(t *testing.T) {
			t.Parallel()

			u := unit("stubgen", "svc/store/unit.go")
			u.Decls = append(u.Decls, nil)
			assert.HasError(t, plugin.NewEmit().Add(u),
				"a nil declaration would abort a render worker, so the door refuses it")
		})
	})

	t.Run("Contribute", func(t *testing.T) {
		t.Parallel()

		t.Run("records the plugin on the unit that contains the host", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			u := hosting("stubgen", "a.go", "Store")
			assert.NoError(t, e.Add(u), "the hosting unit arrives")
			assert.True(t, e.Contribute(methodOf(u), "audit"), "the store contains the host")
			assert.Equal(t, contributorsOf(e), [][]plugin.ID{{"audit"}}, "the unit names the weaver")
		})

		t.Run("records a plugin once", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			u := hosting("stubgen", "a.go", "Store")
			assert.NoError(t, e.Add(u), "the hosting unit arrives")
			e.Contribute(methodOf(u), "audit")
			e.Contribute(u.Decls[0], "audit")
			assert.Equal(t, contributorsOf(e), [][]plugin.ID{{"audit"}}, "a second append names no one twice")
		})

		t.Run("records the contributors in sorted order", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			u := hosting("stubgen", "a.go", "Store")
			assert.NoError(t, e.Add(u), "the hosting unit arrives")
			e.Contribute(methodOf(u), "zeta")
			e.Contribute(methodOf(u), "audit")
			assert.Equal(t, contributorsOf(e), [][]plugin.ID{{"audit", "zeta"}}, "the order is the names'")
		})

		t.Run("records nothing for the unit's own plugin", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			u := hosting("stubgen", "a.go", "Store")
			assert.NoError(t, e.Add(u), "the hosting unit arrives")
			assert.True(t, e.Contribute(methodOf(u), "stubgen"), "the store contains the host")
			assert.Equal(t, contributorsOf(e), [][]plugin.ID{nil}, "a unit's emitter is not its contributor")
		})

		t.Run("returns false for a host the store does not contain", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(hosting("stubgen", "a.go", "Store")), "the hosting unit arrives")
			assert.False(t, e.Contribute(&emit.Struct{Origin: structID("Store"), Name: "Elsewhere"}, "audit"),
				"a value no unit contains has no unit to name the weaver in")
			assert.Equal(t, contributorsOf(e), [][]plugin.ID{nil}, "and nothing is recorded")
		})

		t.Run("records the plugin on a unit that arrived after an earlier contribution", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			first := hosting("stubgen", "a.go", "Store")
			assert.NoError(t, e.Add(first), "the first unit arrives")
			e.Contribute(methodOf(first), "audit")
			second := hosting("stubgen", "b.go", "Cache")
			assert.NoError(t, e.Add(second), "the second unit arrives")
			assert.True(t, e.Contribute(methodOf(second), "audit"), "the store contains the later host")
			assert.Equal(t, contributorsOf(e), [][]plugin.ID{{"audit"}, {"audit"}}, "both units name the weaver")
		})
	})

	t.Run("Ref", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a root's place in its unit", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			u := hosting("stubgen", "a.go", "Store")
			assert.NoError(t, e.Add(u), "the hosting unit arrives")
			ref, held := e.Ref(u.Decls[0])
			assert.True(t, held, "the store contains the root")
			assert.Equal(t, ref, plugin.EmitRef{Unit: u.Ref(), Index: 0}, "a root's place is its walk position")
		})

		t.Run("returns a slotted declaration's place in its unit's walk", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			u := hosting("stubgen", "a.go", "Store")
			assert.NoError(t, e.Add(u), "the hosting unit arrives")
			ref, _ := e.Ref(methodOf(u))
			assert.Equal(t, ref, plugin.EmitRef{Unit: u.Ref(), Index: 1}, "the method follows its struct")
		})

		t.Run("counts the declarations of the roots before a later root", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			u := hosting("stubgen", "a.go", "Store")
			later := &emit.Struct{Origin: structID("Cache"), Name: "Cache"}
			u.Decls = append(u.Decls, later)
			assert.NoError(t, e.Add(u), "the two-root unit arrives")
			ref, _ := e.Ref(later)
			assert.Equal(t, ref.Index, 2, "the second root follows the first root's struct and method")
		})

		t.Run("returns false for a declaration no unit contains", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(hosting("stubgen", "a.go", "Store")), "the hosting unit arrives")
			_, held := e.Ref(&emit.Struct{Origin: structID("Store"), Name: "Elsewhere"})
			assert.False(t, held, "a value no unit contains has no reference")
		})

		t.Run("returns the place of a declaration in a unit that arrived after an earlier call", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			first := hosting("stubgen", "a.go", "Store")
			assert.NoError(t, e.Add(first), "the first unit arrives")
			e.Ref(first.Decls[0])
			second := hosting("stubgen", "b.go", "Cache")
			assert.NoError(t, e.Add(second), "the second unit arrives")
			ref, held := e.Ref(methodOf(second))
			assert.True(t, held, "the later unit's tree is walked")
			assert.Equal(t, ref, plugin.EmitRef{Unit: second.Ref(), Index: 1}, "the place is in the later unit")
		})

		t.Run("returns the first place of a declaration two units contain", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			first := hosting("stubgen", "b.go", "Store")
			second := unit("stubgen", "a.go")
			second.Decls = first.Decls
			assert.NoError(t, e.Add(first), "the first unit arrives")
			assert.NoError(t, e.Add(second), "the second unit shares the first unit's root")
			ref, _ := e.Ref(first.Decls[0])
			assert.Equal(t, ref.Unit, first.Ref(), "the unit that arrived first names the shared root")
		})

		t.Run("returns the first place of a declaration one tree contains twice", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Origin: structID("Store"), Name: "Store"}
			get := &emit.Method{Origin: structID("Store"), Name: "Get"}
			s.Methods.Append(get, get)
			u := unit("stubgen", "a.go")
			u.Decls = []symbol.Symbol{s}
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(u), "the unit arrives")
			ref, _ := e.Ref(get)
			assert.Equal(t, ref.Index, 1, "the walk's first visit names the method")
		})
	})

	t.Run("Units", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a total order", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			second := unit("stubgen", "a.go")
			second.Tag = "test"
			cached := unit("stubgen", "a.go")
			cached.Pkg = coretest.PackageID(coretest.CachePath)
			for _, u := range []plugin.Unit{
				{Plugin: "stubgen", Per: plugin.PerPlan, Word: "registry"},
				second,
				unit("stubgen", "a.go"),
				cached,
				unit("audit", "z.go"),
			} {
				assert.NoError(t, e.Add(u), "every fixture unit arrives")
			}

			var got []string
			for u := range e.Units() {
				got = append(got, string(u.Plugin)+"/"+u.Key+"/"+u.Pkg.Package+"/"+u.Tag)
			}
			assert.Equal(t, got, []string{
				"audit/z.go/svc/store/",
				"stubgen/a.go/svc/cache/",
				"stubgen/a.go/svc/store/",
				"stubgen/a.go/svc/store/test",
				"stubgen///",
			}, "units order by plugin, then cardinality, then key, then package, then tag")
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

		t.Run("walks each unit's tree in Units order", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(hosting("stubgen", "b.go", "Store")),
				"the later-ordered unit arrives first")
			assert.NoError(t, e.Add(hosting("audit", "a.go", "Cache")),
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
			assert.NoError(t, e.Add(hosting("stubgen", "a.go", "Store")),
				"a hosting unit arrives")

			var got int
			for range e.ByKind(symbol.KindMethod) {
				got++
				break
			}
			assert.Equal(t, got, 1, "the iteration stops when the range stops")
		})
	})
}

// A reference lookup over a walked store allocates nothing, because a
// journaled phase call resolves one for every emit-phase invocation, and
// an add allocates what the store keeps. The checks run alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestEmitAllocs(t *testing.T) {
	e := plugin.NewEmit()
	u := hosting("stubgen", "a.go", "Store")
	assert.NoError(t, e.Add(u), "the hosting unit arrives")
	host := methodOf(u)
	e.Ref(host)
	assert.MaxAllocs(t, func() {
		if _, held := e.Ref(host); !held {
			t.Fatal("Ref misses the walked method")
		}
	}, 0, "Ref allocates nothing once the store is walked")
	assert.MaxAllocs(t, func() {
		if u.Ref().Key != "a.go" {
			t.Fatal("Unit.Ref returns another key")
		}
	}, 0, "Unit.Ref allocates nothing")
	assert.MaxAllocs(t, func() {
		if !e.Contribute(host, "audit") {
			t.Fatal("Contribute misses the walked method")
		}
	}, 0, "Contribute allocates nothing for a contributor the unit names")

	added := benchUnits
	later := addedUnits(t, benchUnits)
	units := benchUnitsOf(benchUnits + allocRuns)
	assert.MaxAllocs(t, func() {
		if err := later.Add(units[added]); err != nil {
			t.Fatalf("Add: unexpected error: %v", err)
		}
		added++
	}, laterAddAllocs, "Add allocates the key and the declarations of each kind of a later unit")

	var built *plugin.Emit
	assert.MaxAllocs(t, func() { built = plugin.NewEmit() }, newEmitAllocs,
		"NewEmit allocates the store and its two maps")
	assert.False(t, built.Settled(), "NewEmit returns an unsettled store")

	empty := make([]*plugin.Emit, allocRuns)
	for i := range empty {
		empty[i] = plugin.NewEmit()
	}
	at := 0
	assert.MaxAllocs(t, func() {
		if err := empty[at].Add(units[0]); err != nil {
			t.Fatalf("Add: unexpected error: %v", err)
		}
		at++
	}, firstAddAllocs, "Add allocates the entries of the first unit of a store")

	hosted := make([]*plugin.Emit, allocRuns)
	for i := range hosted {
		hosted[i] = plugin.NewEmit()
		assert.NoError(t, hosted[i].Add(u), "the hosting unit arrives")
	}
	at = 0
	assert.MaxAllocs(t, func() {
		if !hosted[at].Contribute(host, "audit") {
			t.Fatal("Contribute misses the hosted method")
		}
		at++
	}, firstContributeAllocs, "Contribute allocates the map of hosts and the contributor of a first contribution")

	settled := false
	assert.MaxAllocs(t, func() { settled = e.Settled() }, 0, "Settled allocates nothing")
	assert.False(t, settled, "Settled reports false before the settle")
	assert.MaxAllocs(t, func() {
		if u.FileKey() != "a.go" || plugin.PerSource.String() != "per-source" {
			t.Fatal("the unit's key or the cardinality's spelling read back wrong")
		}
	}, 0, "FileKey and String allocate nothing")
}

// BenchmarkEmit measures the emit store: the adds a phase call's flush
// makes, and the enumerations an emit-triggered rule and the settle
// make over a plan's output.
func BenchmarkEmit(b *testing.B) {
	units := benchUnitsOf(benchUnits * 10)

	b.Run("Add", func(b *testing.B) {
		b.Run("the first unit of a store", func(b *testing.B) {
			var e *plugin.Emit
			empty := func() { e = plugin.NewEmit() }
			c := bench.Start(b).MaxAllocs(firstAddAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(empty)
				err = e.Add(units[0])
			}
			assert.NoError(b, err, "the unit is added")
		})

		b.Run("a later unit", func(b *testing.B) {
			var (
				e    *plugin.Emit
				next int
			)
			fill := func() {
				e, next = addedUnits(b, benchUnits), benchUnits
			}
			fill()
			c := bench.Start(b).MaxAllocs(laterAddAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				if next == len(units) {
					c.Excluding(fill)
				}
				err = e.Add(units[next])
				next++
			}
			assert.NoError(b, err, "the unit is added")
		})
	})

	b.Run("ByKind", func(b *testing.B) {
		e := plugin.NewEmit()
		const units, methods = 100, 20
		for i := range units {
			assert.NoError(b, e.Add(benchUnit("stubgen", "unit"+strconv.Itoa(i)+".go", 1, methods)),
				"every unit is added")
		}
		for range e.ByKind(symbol.KindMethod) { // the first enumeration orders the store
		}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		n := 0
		for c.Loop() {
			n = 0
			for range e.ByKind(symbol.KindMethod) {
				n++
			}
		}
		assert.Equal(b, n, units*methods, "ByKind yields every method")
	})

	b.Run("Units", func(b *testing.B) {
		e := plugin.NewEmit()
		const units = 1_000
		for i := range units {
			assert.NoError(b, e.Add(unit("stubgen", "unit"+strconv.Itoa(i)+".go")), "every unit is added")
		}
		for range e.Units() { // the first enumeration orders the store
		}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		n := 0
		for c.Loop() {
			n = 0
			for range e.Units() {
				n++
			}
		}
		assert.Equal(b, n, units, "Units yields every unit")
	})

	b.Run("Ref", func(b *testing.B) {
		e := plugin.NewEmit()
		const units, methods = 100, 20
		var host symbol.Symbol
		for i := range units {
			u := benchUnit("stubgen", "unit"+strconv.Itoa(i)+".go", 1, methods)
			if err := e.Add(u); err != nil {
				b.Fatalf("Add: unexpected error: %v", err)
			}
			host = methodOf(u)
		}
		e.Ref(host)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got plugin.EmitRef
		for c.Loop() {
			got, _ = e.Ref(host)
		}
		if got.Index != 1 {
			b.Fatalf("Ref places the last unit's first method at %d", got.Index)
		}
	})

	b.Run("Unit.Ref", func(b *testing.B) {
		u := unit("stubgen", "a.go")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got plugin.UnitRef
		for c.Loop() {
			got = u.Ref()
		}
		if got.Key != "a.go" {
			b.Fatalf("Unit.Ref returns key %q", got.Key)
		}
	})

	b.Run("String", func(b *testing.B) {
		per := plugin.PerPackage
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = per.String()
		}
		assert.Equal(b, got, "per-package", "String spells PerPackage")
	})

	b.Run("FileKey", func(b *testing.B) {
		u := unit("stubgen", "a.go")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = u.FileKey()
		}
		assert.Equal(b, got, "a.go", "FileKey returns a per-source unit's path")
	})

	b.Run("NewEmit", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newEmitAllocs)
		defer c.End()
		var e *plugin.Emit
		for c.Loop() {
			e = plugin.NewEmit()
		}
		assert.False(b, e.Settled(), "NewEmit returns an unsettled store")
	})

	b.Run("Settled", func(b *testing.B) {
		e := plugin.NewEmit()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		settled := true
		for c.Loop() {
			settled = e.Settled()
		}
		assert.False(b, settled, "Settled reports false before the settle")
	})

	b.Run("Contribute", func(b *testing.B) {
		b.Run("a contributor the unit names", func(b *testing.B) {
			e := plugin.NewEmit()
			u := hosting("stubgen", "a.go", "Store")
			assert.NoError(b, e.Add(u), "the hosting unit arrives")
			host := methodOf(u)
			e.Contribute(host, "audit")
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			held := false
			for c.Loop() {
				held = e.Contribute(host, "audit")
			}
			assert.True(b, held, "the store contains the host")
		})

		b.Run("the first contribution to a store", func(b *testing.B) {
			u := hosting("stubgen", "a.go", "Store")
			host := methodOf(u)
			var e *plugin.Emit
			fresh := func() {
				e = plugin.NewEmit()
				assert.NoError(b, e.Add(u), "the hosting unit arrives")
			}
			c := bench.Start(b).MaxAllocs(firstContributeAllocs)
			defer c.End()
			held := false
			for c.Loop() {
				c.Excluding(fresh)
				held = e.Contribute(host, "audit")
			}
			assert.True(b, held, "the store contains the host")
		})
	})
}

// structID returns the identity a resolved struct of that name has,
// for origins the fixtures below derive from.
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

// hosting returns a unit that contains one origined struct with one
// origined method in its slot.
func hosting(p, key, structName string) plugin.Unit {
	s := &emit.Struct{Origin: structID(structName), Name: structName}
	s.Methods.Append(&emit.Method{Origin: structID(structName), Name: "Get"})
	u := unit(p, key)
	u.Decls = []symbol.Symbol{s}
	u.Origins = []symbol.Identity{structID(structName)}
	return u
}

// methodOf returns the method in the slot of a hosting unit's struct.
func methodOf(u plugin.Unit) symbol.Symbol {
	s, _ := u.Decls[0].(*emit.Struct)
	return s.Methods.Items()[0]
}

// contributorsOf returns the contributors of every unit of a store, in
// Units order.
func contributorsOf(e *plugin.Emit) [][]plugin.ID {
	var out [][]plugin.ID
	for u := range e.Units() {
		out = append(out, u.Contributors)
	}
	return out
}

// benchUnit returns one unit containing structs of methods, the tree
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

// benchUnitsOf returns n units of the stubgen plugin, each under a key
// of its own and with the tree of benchStructs structs of benchMethods
// methods.
func benchUnitsOf(n int) []plugin.Unit {
	out := make([]plugin.Unit, 0, n)
	for i := range n {
		out = append(out, benchUnit("stubgen", "unit"+strconv.Itoa(i)+".go", benchStructs, benchMethods))
	}
	return out
}

// addedUnits returns a store with the first n units of [benchUnitsOf]
// added.
func addedUnits(tb assert.TB, n int) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	for _, u := range benchUnitsOf(n) {
		assert.NoError(tb, e.Add(u), "every unit is added")
	}
	return e
}
