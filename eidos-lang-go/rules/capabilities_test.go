// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The capability cases read the sentinel a base spells, the struct tag
// the fixture's Tagged.Name declares, key by key, and the spellings a
// substitution binds.
const (
	// sentinelBase is the base the convention spells as sentinel.
	sentinelBase = "not found"
	sentinel     = "ErrNotFound"
	// jsonKey and dbKey are the keys the tag states, with their
	// values, and xmlKey a key it does not state.
	jsonKey = "json"
	jsonTag = "name,omitempty"
	dbKey   = "db"
	dbTag   = "n"
	xmlKey  = "xml"
	// witness is the type every admitting bound derives.
	witness = "int"
	// paramName is Box's one type parameter, and argSpelling the
	// argument a substitution binds to it.
	paramName   = "T"
	argSpelling = "string"
	// stampedHost is the struct whose embed the member walk cannot
	// read, stampedLabel its own field, and timeSpelling the embed.
	stampedHost  = "Stamped"
	stampedLabel = "Label"
	timeSpelling = "time.Time"
)

// The allocations of the capabilities.
const (
	// sentinelAllocs is a sentinel's name: the base in Pascal case, and
	// the prefix joined onto it.
	sentinelAllocs = 2
	// witnessAllocs is a derived witness: the reference to int.
	witnessAllocs = 1
	// substituteAllocs is a list of a bound parameter: the copy of the
	// argument, the copy of the list's element list, and the copy of
	// the list.
	substituteAllocs = 3
	// settableAllocs is the settable set of a struct that embeds one
	// type: the binding the member walk runs on, the walk's member list,
	// the path into the embed, and the list of settable fields.
	settableAllocs = 4
	// comparableAllocs is Row, whose slice, map, time.Time and
	// time.Duration fields break comparability: the problem list as it
	// grows to the four.
	comparableAllocs = 3
)

// The kernel asserts the Go rules against the optional capability
// interfaces. Each method is pinned through the interface that
// declares it.
func TestCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("SentinelName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the base in PascalCase behind Err", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capability[rules.ErrorValueRules](t).SentinelName(sentinelBase), sentinel,
				"the prefix and the base")
		})
	})

	t.Run("IsSentinelName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for the prefix followed by an upper-case rune", give: sentinel, want: true},
			{name: "reports false for the prefix alone", give: "Err"},
			{name: "reports false for a lower-case rune after the prefix", give: "Errors"},
			{name: "reports false for a name without the prefix", give: "NotFound"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, capability[rules.ErrorValueRules](t).IsSentinelName(tt.give), tt.want,
					"whether the name follows the convention")
			})
		}
	})

	t.Run("Tag", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			key        string
			want       string
			wantStated bool
		}{
			{name: "returns the value of the json key", key: jsonKey, want: jsonTag, wantStated: true},
			{name: "returns the value of the db key", key: dbKey, want: dbTag, wantStated: true},
			{name: "reports false for a key the tag does not state", key: xmlKey},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, stated := capability[rules.TagRules](t).Tag(f.field(t, "Tagged", "Name"), tt.key)
				assert.Equal(t, stated, tt.wantStated, "whether the tag states the key")
				assert.Equal(t, got, tt.want, "the key's value")
			})
		}

		t.Run("reports false for a missing field", func(t *testing.T) {
			t.Parallel()

			_, stated := capability[rules.TagRules](t).Tag(nil, jsonKey)
			assert.False(t, stated, "no field, no tag")
		})
	})

	t.Run("Derive", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			host  string
			param int
			want  string
		}{
			{name: "returns int for a parameter bound by any", host: "Box", want: witness},
			{name: "returns int for a parameter bound by comparable", host: "Pair", want: witness},
			{name: "returns int for a type set that names ~int", host: "Sized", want: witness},
			{name: "returns int for an empty interface the graph declares", host: "Sized", param: 2, want: witness},
			{name: "reports false for a bound that states methods", host: "Bound"},
			{name: "reports false for a type set that names no int", host: "Sized", param: 1},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				w, derived := capability[rules.GenericsRules](t).Derive(paramsOf(t, f, tt.host)[tt.param], f.view)
				assert.Equal(t, derived, tt.want != "", "whether int satisfies the bound")
				if derived {
					assert.Equal(t, w.Spelling, tt.want, "the witness")
				}
			})
		}

		t.Run("returns int for a parameter without a bound", func(t *testing.T) {
			t.Parallel()

			w, derived := capability[rules.GenericsRules](t).Derive(&node.TypeParam{Name: paramName}, rules.View{})
			assert.True(t, derived, "no bound admits every type")
			assert.Equal(t, w.Spelling, witness, "the witness")
		})

		t.Run("reports false for a type set without the facts", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, derived := capability[rules.GenericsRules](t).Derive(paramsOf(t, f, "Sized")[0],
				rules.View{Decls: f.view.Decls})
			assert.False(t, derived, "no type set is knowable")
		})

		t.Run("reports false for a missing parameter", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, derived := capability[rules.GenericsRules](t).Derive(nil, f.view)
			assert.False(t, derived, "no parameter, no witness")
		})

		t.Run("derives one witness per parameter through the bound rules", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			assert.Length(t, f.bound().Witnesses(paramsOf(t, f, "Pair")), 2, "Pair's key and value")
		})
	})

	t.Run("Reified", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false", func(t *testing.T) {
			t.Parallel()

			assert.False(t, capability[rules.GenericsRules](t).Reified(), "Go erases type arguments at run time")
		})
	})

	t.Run("Substitute", func(t *testing.T) {
		t.Parallel()

		t.Run("binds the argument inside a composite", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got := capability[rules.GenericsRules](t).Substitute(listOfParam(), paramsOf(t, f, "Box"),
				[]*node.TypeRef{builtin(argSpelling)})
			assert.Equal(t, got.Elems[0].Spelling, argSpelling, "the argument at the parameter's position")
		})

		t.Run("leaves the reference it restates unchanged", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			list := listOfParam()
			capability[rules.GenericsRules](t).Substitute(list, paramsOf(t, f, "Box"),
				[]*node.TypeRef{builtin(argSpelling)})
			assert.Equal(t, list.Elems[0].Spelling, paramName, "the restatement is a copy")
		})

		t.Run("returns a reference that names no parameter as it is", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			untouched := builtin(witness)
			got := capability[rules.GenericsRules](t).Substitute(untouched, paramsOf(t, f, "Box"),
				[]*node.TypeRef{builtin(argSpelling)})
			assert.Equal(t, got, untouched, "the same reference", assert.ByIdentity())
		})

		t.Run("returns the reference as it is for mismatched arguments", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			item := &node.TypeRef{Spelling: paramName}
			got := capability[rules.GenericsRules](t).Substitute(item, paramsOf(t, f, "Box"), nil)
			assert.Equal(t, got, item, "no argument binds", assert.ByIdentity())
		})
	})

	t.Run("Settable", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the promoted fields after the struct's own", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, settableNames(t, "Derived"), []string{"Name", "Kind"},
				"Derived's own field, then the one Base promotes")
		})

		t.Run("leaves out an unexported field", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, settableNames(t, "Row"),
				[]string{"ID", "Tags", "Next", "When", "Dur", "Dep", "Set", "Ratio"},
				"another package cannot set Row's name")
		})

		t.Run("returns the settable fields beside a gap", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, settableNames(t, stampedHost), []string{stampedLabel},
				"the struct's own exported field, whatever its embed yields")
		})

		t.Run("returns the gap of an embed the walk cannot read", func(t *testing.T) {
			t.Parallel()

			gaps := settable(t, stampedHost).Gaps
			assert.Length(t, gaps, 1, "one gap, so a builder over the set knows it is partial")
			assert.Equal(t, gaps[0].Reason, rules.GapUnresolved, "the embed resolves to nothing the view contains")
			assert.Equal(t, gaps[0].Contributor.Spelling, timeSpelling, "naming the embed")
		})

		t.Run("returns an empty set for a missing struct", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			set := capability[rules.PromotionRules](t).Settable(nil, f.view)
			assert.Empty(t, set.Members, "nothing has no members")
			assert.Empty(t, set.Gaps, "and no gaps")
		})
	})

	t.Run("Comparable", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give *node.TypeRef
			want bool
		}{
			{
				name: "reports true for a struct of strings",
				give: ref(fxPath, "Base", symbol.KindStruct), want: true,
			},
			{
				name: "reports true for a struct that embeds a comparable struct",
				give: ref(fxPath, "Derived", symbol.KindStruct), want: true,
			},
			{
				name: "reports true for an interface",
				give: ref(fxPath, "Reader", symbol.KindInterface), want: true,
			},
			{
				name: "reports true for an enumeration",
				give: ref(fxPath, "Color", symbol.KindEnum), want: true,
			},
			{
				name: "reports true for a defined type over a comparable type",
				give: ref(fxPath, "Weight", symbol.KindAlias), want: true,
			},
			{
				name: "reports true for an instantiation whose argument compares",
				give: boxOf(builtin(witness)), want: true,
			},
			{
				name: "reports true for a pointer",
				give: composite("*Row", symbol.FormOptional, builtin("Row")), want: true,
			},
			{
				name: "reports true for a channel",
				give: composite("chan int", symbol.FormStream, builtin(witness)), want: true,
			},
			{
				name: "reports true for an array of a comparable element",
				give: composite("[2]int", symbol.FormArray, builtin(witness)), want: true,
			},
			{
				name: "reports true for an empty inline interface",
				give: &node.TypeRef{Spelling: "interface{}", Form: symbol.FormInline}, want: true,
			},
			{
				name: "reports true for an inline interface that states methods",
				give: &node.TypeRef{Spelling: "interface{ Close() error }", Form: symbol.FormInline}, want: true,
			},
			{name: "reports true for any", give: builtin("any"), want: true},
			{name: "reports true for string", give: builtin(argSpelling), want: true},
			{name: "reports true for complex64", give: builtin("complex64"), want: true},
			{name: "reports true for unsafe.Pointer", give: builtin("unsafe.Pointer"), want: true},
			{
				name: "reports false for a struct with a slice field",
				give: ref(fxPath, "Uncomparable", symbol.KindStruct),
			},
			{
				name: "reports false for a struct with several uncomparable fields",
				give: ref(fxPath, "Row", symbol.KindStruct),
			},
			{
				name: "reports false for an instantiation whose argument does not compare",
				give: boxOf(composite("[]int", symbol.FormList, builtin(witness))),
			},
			{name: "reports false for a generic type without its arguments", give: boxOf(nil)},
			{name: "reports false for a function type", give: composite("func()", symbol.FormFunc)},
			{
				name: "reports false for a map",
				give: composite("map[int]int", symbol.FormMap, builtin(witness), builtin(witness)),
			},
			{
				name: "reports false for an inline struct",
				give: &node.TypeRef{Spelling: "struct{}", Form: symbol.FormInline},
			},
			{name: "reports false for a name outside the workspace", give: builtin("Outside")},
			{
				name: "reports false for a predeclared name another package qualifies",
				give: &node.TypeRef{Spelling: "p.int", Package: foreignPkg},
			},
			{
				name: "reports false for a target the graph does not declare",
				give: ref(fxPath, "Ghost", symbol.KindStruct),
			},
			{name: "reports false for a missing reference"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				ok, _ := capability[rules.EqualityRules](t).Comparable(tt.give, f.view)
				assert.Equal(t, ok, tt.want, "whether Go compares the type with ==")
			})
		}

		t.Run("returns no problem for a type that compares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, problems := capability[rules.EqualityRules](t).Comparable(ref(fxPath, "Base", symbol.KindStruct), f.view)
			assert.Empty(t, problems, "nothing breaks comparability")
		})

		t.Run("returns the slice field as the problem", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, problems := capability[rules.EqualityRules](t).Comparable(
				ref(fxPath, "Uncomparable", symbol.KindStruct), f.view)
			assert.Length(t, problems, 1, "one field breaks comparability")
			assert.Equal(t, problems[0].Form, symbol.FormList, "the slice")
		})

		t.Run("returns the argument that does not compare as the problem", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, problems := capability[rules.EqualityRules](t).Comparable(
				boxOf(composite("[]int", symbol.FormList, builtin(witness))), f.view)
			assert.Length(t, problems, 1, "one argument breaks comparability")
			assert.Equal(t, problems[0].Form, symbol.FormList, "the slice the argument binds")
		})
	})
}

// Each capability allocates what it returns and the working state its
// walk needs. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestCapabilitiesAllocs(t *testing.T) {
	checkAllocs(t, capabilityCalls(t))
}

// BenchmarkCapabilities measures each capability the kernel calls
// through its optional interfaces.
func BenchmarkCapabilities(b *testing.B) {
	benchCalls(b, capabilityCalls(b))
}

// capabilityCalls returns a call of every capability over the fixture
// tree.
func capabilityCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	errs, tags := capability[rules.ErrorValueRules](tb), capability[rules.TagRules](tb)
	generics, promotion := capability[rules.GenericsRules](tb), capability[rules.PromotionRules](tb)
	equality := capability[rules.EqualityRules](tb)
	tagged, box := f.field(tb, "Tagged", "Name"), paramsOf(tb, f, "Box")
	derived, _ := f.decl(tb, id(fxPath, "Derived", symbol.KindStruct)).(*node.Struct)
	list, plain, args := listOfParam(), builtin(witness), []*node.TypeRef{builtin(argSpelling)}
	row, base := ref(fxPath, "Row", symbol.KindStruct), ref(fxPath, "Base", symbol.KindStruct)
	var (
		text     string
		reported bool
		got      *node.TypeRef
		set      rules.MemberSet
		problems []*node.TypeRef
	)
	return []allocCall{
		{
			name: "SentinelName", allocs: sentinelAllocs,
			call:  func() { text = errs.SentinelName(sentinelBase) },
			check: func(tb assert.TB) { assert.Equal(tb, text, sentinel, "SentinelName spells the sentinel") },
		},
		{
			name:  "IsSentinelName",
			call:  func() { reported = errs.IsSentinelName(sentinel) },
			check: func(tb assert.TB) { assert.True(tb, reported, "IsSentinelName reports the sentinel") },
		},
		{
			name:  "Tag",
			call:  func() { text, _ = tags.Tag(tagged, jsonKey) },
			check: func(tb assert.TB) { assert.Equal(tb, text, jsonTag, "Tag returns the json value") },
		},
		{
			name: "Derive", allocs: witnessAllocs,
			call:  func() { got, _ = generics.Derive(box[0], f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, got.Spelling, witness, "Derive returns int") },
		},
		{
			name: "Substitute", caseName: "a reference that names a parameter", allocs: substituteAllocs,
			call: func() { got = generics.Substitute(list, box, args) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got.Elems[0].Spelling, argSpelling, "Substitute binds the argument")
			},
		},
		{
			name: "Substitute", caseName: "a reference that names no parameter",
			call: func() { got = generics.Substitute(plain, box, args) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got, plain, "Substitute returns the reference as it is", assert.ByIdentity())
			},
		},
		{
			name:  "Reified",
			call:  func() { reported = generics.Reified() },
			check: func(tb assert.TB) { assert.False(tb, reported, "Reified reports erasure") },
		},
		{
			name: "Settable", allocs: settableAllocs,
			call:  func() { set = promotion.Settable(derived, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, set.Members, 2, "Settable returns Name and Kind") },
		},
		{
			name: "Comparable", caseName: "a struct that does not compare", allocs: comparableAllocs,
			call: func() { reported, problems = equality.Comparable(row, f.view) },
			check: func(tb assert.TB) {
				assert.False(tb, reported, "Comparable reports that Row does not compare")
				assert.Length(tb, problems, 4, "Comparable finds all four")
			},
		},
		{
			name: "Comparable", caseName: "a struct that compares",
			call: func() { reported, problems = equality.Comparable(base, f.view) },
			check: func(tb assert.TB) {
				assert.True(tb, reported, "Comparable reports Base")
				assert.Empty(tb, problems, "Comparable finds no problem in Base")
			},
		},
	}
}

// capability returns the Go rules as the optional interface C, and
// stops the test where they do not implement it.
func capability[C any](tb assert.TB) C {
	tb.Helper()

	c, is := gorules.New().(C)
	assert.True(tb, is, "the Go rules implement the capability")
	return c
}

// paramsOf returns the type parameters of a generic struct the
// fixture declares.
func paramsOf(tb assert.TB, f *fixture, host string) []*node.TypeParam {
	tb.Helper()

	s, is := f.decl(tb, id(fxPath, host, symbol.KindStruct)).(*node.Struct)
	assert.True(tb, is, host+" is a struct")
	return s.TypeParams
}

// settable returns the set Settable returns for a struct the fixture
// declares.
func settable(tb assert.TB, host string) rules.MemberSet {
	tb.Helper()

	f := loaded(tb)
	s, is := f.decl(tb, id(fxPath, host, symbol.KindStruct)).(*node.Struct)
	assert.True(tb, is, host+" is a struct")
	return capability[rules.PromotionRules](tb).Settable(s, f.view)
}

// settableNames returns the names of the fields Settable returns for
// a struct the fixture declares, in the order it returns them.
func settableNames(tb assert.TB, host string) []string {
	tb.Helper()

	members := settable(tb, host).Members
	names := make([]string, 0, len(members))
	for _, m := range members {
		field, isField := m.Symbol.(*node.Field)
		assert.True(tb, isField, "a settable member is a field")
		names = append(names, field.Name)
	}
	return names
}

// boxOf returns a reference to the fixture's Box instantiated with
// arg, and to Box without arguments for a nil arg.
func boxOf(arg *node.TypeRef) *node.TypeRef {
	box := ref(fxPath, "Box", symbol.KindStruct)
	if arg != nil {
		box.Args = []*node.TypeRef{arg}
	}
	return box
}

// listOfParam returns a list of Box's type parameter.
func listOfParam() *node.TypeRef {
	return composite("[]"+paramName, symbol.FormList, &node.TypeRef{Spelling: paramName})
}
