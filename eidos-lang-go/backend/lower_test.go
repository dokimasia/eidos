// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The cases pin the type an enum lowers over, and the return an
// announced failure lowers into with the names it takes.
const (
	underlyingType = "int"
	errorType      = "error"
	errorName      = "err"
	nextErrorName  = "err1"
)

// The cases lower an enum and its variants, a generic host and its
// methods, and the callables and failure types a throw states.
const (
	phaseName     = "phase"
	openName      = "open"
	closedName    = "closed"
	closedValue   = "9"
	boxName       = "box"
	keyParam      = "K"
	valueParam    = "V"
	getName       = "get"
	putName       = "put"
	fetchName     = "fetch"
	saveName      = "save"
	countName     = "count"
	countResult   = "n"
	loadName      = "load"
	receiverName  = "r"
	shapeName     = "shape"
	notFoundType  = "notFound"
	timeoutType   = "timeout"
	conflictType  = "conflict"
	overflowType  = "overflow"
	intResultType = "int"
)

// The constants an enum lowers to: the type's name, then each
// variant's, joined in the neutral camel form.
const (
	phaseOpen   = "phaseOpen"
	phaseClosed = "phaseClosed"
)

// The allocations of a lowering.
const (
	// enumLowerAllocs is an enum of three variants without values: the
	// list of outputs, the defined type and its target, and per variant
	// the constant, its type, and its joined name's two.
	enumLowerAllocs = 1 + 2 + 3*4
	// throwsAllocs is a function announcing one failure: the error
	// return, its type, and the grown list of returns.
	throwsAllocs = 1 + 1 + 1
	// receiveAllocs is a struct of two methods: each method's receiver.
	receiveAllocs = 2
	// allocRuns is the calls assert.MaxAllocs makes: one to warm up and
	// one hundred it counts.
	allocRuns = 101
)

// The lowering is Go's declared idiom for the constructs it states
// in other declarations, so each reshaping is pinned.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("reshapes an enum into a defined type followed by one constant per variant", func(t *testing.T) {
			t.Parallel()

			origin := symbol.Identity{Lang: protoLang, Package: svcPkg, Name: phaseName, Kind: symbol.KindEnum}
			e := &emit.Enum{
				Origin: origin,
				Doc:    []string{"phase names a lifecycle step."},
				Name:   phaseName,
			}
			e.Variants.Append(
				&emit.EnumVariant{Doc: []string{"open admits writes."}, Name: openName},
				&emit.EnumVariant{Name: closedName, Value: closedValue},
			)
			out, err := backend.Lower(e)
			assert.NoError(t, err, "the enum lowers")
			assert.Length(t, out, 3, "one defined type and one constant per variant")

			alias, isAlias := out[0].(*emit.Alias)
			assert.True(t, isAlias, "the principal output is the defined type")
			assert.True(t, alias.Defined, "defined, not transparent")
			assert.Equal(t, alias.Name, phaseName, "keeping the enum's name")
			assert.Equal(t, alias.Target.Spelling, underlyingType, "over the int underlying")
			assert.Equal(t, alias.Origin, origin, "under the enum's origin")
			assert.Equal(t, alias.Doc, e.Doc, "with its documentation")

			first, isConstant := out[1].(*emit.Constant)
			assert.True(t, isConstant, "each variant becomes a constant")
			assert.Equal(t, first.Name, phaseOpen, "named type-then-variant in the neutral form")
			assert.Equal(t, first.Type.Spelling, phaseName,
				"typed by the defined type, so the reference follows a respell")
			assert.Equal(t, first.Value, "0", "an unstated value counts by ordinal")
			assert.Equal(t, first.Origin, origin, "under the enum's origin")

			second, isConstant := out[2].(*emit.Constant)
			assert.True(t, isConstant, "the second variant becomes a constant")
			assert.Equal(t, second.Name, phaseClosed, "beside the first")
			assert.Equal(t, second.Value, closedValue, "a stated value spells verbatim")
		})

		values := []struct {
			name string
			give []string
			want []string
		}{
			{
				name: "repeats the last stated value with iota at each position",
				give: []string{"1 << iota", "", ""},
				want: []string{"1 << 0", "1 << 1", "1 << 2"},
			},
			{
				name: "repeats a stated value without iota as written",
				give: []string{"5", ""},
				want: []string{"5", "5"},
			},
			{
				name: "counts a variant before any stated value by its ordinal",
				give: []string{"", "", closedValue, ""},
				want: []string{"0", "1", closedValue, closedValue},
			},
			{
				name: "keeps an iota inside a string literal",
				give: []string{`"iota"`},
				want: []string{`"iota"`},
			},
			{
				name: "keeps an iota inside a longer identifier",
				give: []string{"iotaSuffix"},
				want: []string{"iotaSuffix"},
			},
		}
		for _, tt := range values {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, constantValues(t, enumOf(tt.give...)), tt.want,
					"the value a constant group evaluates to")
			})
		}

		t.Run("returns an error for an enum with members", func(t *testing.T) {
			t.Parallel()

			e := enumOf("")
			e.Methods.Append(&emit.Method{Name: getName})
			_, err := backend.Lower(e)
			assert.HasError(t, err, "a constant group has no members")
		})

		overloads := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "returns an error for a struct declaring one method twice", give: hostOf(getName, getName)},
			{name: "returns an error for an interface declaring one method twice", give: interfaceOf(getName, getName)},
		}
		for _, tt := range overloads {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(tt.give)
				assert.HasError(t, err, "Go overloads nothing")
			})
		}

		t.Run("restates a generic struct's parameters on each method's receiver", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: boxName, TypeParams: []*emit.TypeParam{{Name: keyParam}, {Name: valueParam}}}
			s.Methods.Append(&emit.Method{Name: getName}, &emit.Method{Name: putName})
			lowered(t, s)
			for _, m := range s.Methods.Items() {
				assert.Equal(t, m.Receives.Args,
					[]*emit.TypeRef{{Spelling: keyParam}, {Spelling: valueParam}},
					m.Name+" receives the host with its parameters as arguments")
			}
		})

		t.Run("gives each method of a generic struct receiver arguments of its own", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: boxName, TypeParams: []*emit.TypeParam{{Name: keyParam}}}
			s.Methods.Append(&emit.Method{Name: getName}, &emit.Method{Name: putName})
			lowered(t, s)
			methods := s.Methods.Items()
			assert.True(t, methods[0].Receives.Args[0] != methods[1].Receives.Args[0],
				"a respell of one method's receiver leaves the other's")
		})

		t.Run("fills the host receiver of a struct method stating none", func(t *testing.T) {
			t.Parallel()

			origin := symbol.Identity{Package: storePkg, Name: rowName}
			s := &emit.Struct{Name: rowName, Origin: origin}
			s.Methods.Append(&emit.Method{Name: loadName})
			lowered(t, s)
			filled := s.Methods.Items()[0]
			assert.True(t, filled.Receives != nil, "an unstated receiver is filled")
			assert.Equal(t, filled.Receives.Spelling, rowName, "with the host's name")
			assert.Equal(t, filled.Receives.Target, origin,
				"bound to the host's origin, so the settle respells both together")
		})

		t.Run("keeps the receiver a struct method states", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName}
			s.Methods.Append(&emit.Method{
				Name:     saveName,
				Receiver: &emit.Param{Name: receiverName, Type: &emit.TypeRef{Spelling: "*" + rowName}},
			})
			lowered(t, s)
			assert.True(t, s.Methods.Items()[0].Receives == nil,
				"a stated receiver is how a generator asks for a pointer or a name")
		})

		t.Run("appends one error return for the failures a function announces", func(t *testing.T) {
			t.Parallel()

			f := thrower([]*emit.Return{{Type: &emit.TypeRef{Spelling: rowName}}}, notFoundType, timeoutType)
			lowered(t, f)
			assert.Length(t, f.Returns, 2, "the error return appends")
			assert.Equal(t, f.Returns[1].Type.Spelling, errorType,
				"one error whatever the announced count, because the "+
					"concrete types arrive through errors.As")
		})

		t.Run("consumes the failures a function announces", func(t *testing.T) {
			t.Parallel()

			f := thrower(nil, notFoundType)
			lowered(t, f)
			assert.Length(t, f.Throws, 0, "a second settle finds nothing to lower")
		})

		t.Run("lowers the failures a struct's member method announces", func(t *testing.T) {
			t.Parallel()

			host := &emit.Struct{Name: rowName}
			host.Methods.Append(&emit.Method{
				Name:   saveName,
				Throws: []*emit.TypeRef{{Spelling: conflictType}},
			})
			lowered(t, host)
			m := host.Methods.Items()[0]
			assert.Length(t, m.Throws, 0, "the member's failures are consumed")
			assert.Equal(t, m.Returns, []*emit.Return{{Type: &emit.TypeRef{Spelling: errorType}}},
				"a bare thrower returns the unnamed error alone")
		})

		t.Run("names the error return beside named results", func(t *testing.T) {
			t.Parallel()

			f := thrower([]*emit.Return{{Name: countResult, Type: &emit.TypeRef{Spelling: intResultType}}},
				overflowType)
			f.Name = countName
			lowered(t, f)
			assert.Equal(t, f.Returns[1].Name, errorName,
				"Go refuses a list mixing named and unnamed results")
		})

		t.Run("numbers the error return's name past a result named err", func(t *testing.T) {
			t.Parallel()

			f := thrower([]*emit.Return{{Name: errorName, Type: &emit.TypeRef{Spelling: intResultType}}},
				overflowType)
			lowered(t, f)
			assert.Equal(t, f.Returns[1].Name, nextErrorName, "the next free name")
		})

		passes := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "passes a struct through unchanged", give: &emit.Struct{Name: rowName}},
			{name: "passes a sum through unchanged", give: &emit.Sum{Name: shapeName}},
		}
		for _, tt := range passes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				lowered(t, tt.give)
			})
		}
	})
}

// A declaration Go states as it is allocates nothing, and a reshaping
// allocates the declarations and the references it adds. A lowering in
// place consumes its fact, so each counted call lowers a fresh
// declaration. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestLowerAllocs(t *testing.T) {
	checkAllocs(t, lowerCalls())

	fns, next := make([]*emit.Function, allocRuns), 0
	for i := range fns {
		fns[i] = thrower(nil, overflowType)
	}
	assert.MaxAllocs(t, func() {
		_, _ = backend.Lower(fns[next])
		next++
	}, throwsAllocs, "Lower appends the error return within its ceiling")

	hosts, next := make([]*emit.Struct, allocRuns), 0
	for i := range hosts {
		hosts[i] = hostOf(getName, putName)
	}
	assert.MaxAllocs(t, func() {
		_, _ = backend.Lower(hosts[next])
		next++
	}, receiveAllocs, "Lower fills the receivers within its ceiling")
}

// BenchmarkLower measures the lowering the settle runs over every
// declaration, a fresh declaration built outside the count for each
// lowering in place.
func BenchmarkLower(b *testing.B) {
	benchCalls(b, append(lowerCalls(),
		allocCall{name: "Lower", caseName: "a function that throws", bench: func(b *testing.B) {
			b.Helper()
			f := thrower(nil, overflowType)
			c := bench.Start(b).MaxAllocs(throwsAllocs)
			defer c.End()
			for c.Loop() {
				_, _ = backend.Lower(f)
				c.Excluding(func() { f = thrower(nil, overflowType) })
			}
			lowered(b, f)
			assert.Length(b, f.Returns, 1, "Lower appends the error return")
		}},
		allocCall{name: "Lower", caseName: "a struct", bench: func(b *testing.B) {
			b.Helper()
			host := hostOf(getName, putName)
			c := bench.Start(b).MaxAllocs(receiveAllocs)
			defer c.End()
			for c.Loop() {
				_, _ = backend.Lower(host)
				c.Excluding(func() { host = hostOf(getName, putName) })
			}
			lowered(b, host)
			assert.NotNil(b, host.Methods.Items()[0].Receives, "Lower fills the receiver")
		}},
	))
}

// lowerCalls returns a call of Lower over a declaration Go states as
// it is, and over an enum.
func lowerCalls() []allocCall {
	constant := &emit.Constant{Name: countName, Value: closedValue}
	phase := enumOf("", "", "")
	var (
		out []symbol.Symbol
		err error
	)
	return []allocCall{
		{
			name: "Lower", caseName: "a constant",
			call: func() { out, err = backend.Lower(constant) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower passes the constant")
				assert.Length(tb, out, 0, "Lower keeps the constant in place")
			},
		},
		{
			name: "Lower", caseName: "an enum", allocs: enumLowerAllocs,
			call: func() { out, err = backend.Lower(phase) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower reshapes the enum")
				assert.Length(tb, out, 4, "Lower returns the type and three constants")
			},
		},
	}
}

// enumOf returns an enum whose variants state the given values, in
// order.
func enumOf(values ...string) *emit.Enum {
	e := &emit.Enum{Name: phaseName}
	for i, v := range values {
		e.Variants.Append(&emit.EnumVariant{Name: openName + strconv.Itoa(i), Value: v})
	}
	return e
}

// hostOf returns a struct of svcPkg whose methods state no receiver.
func hostOf(methods ...string) *emit.Struct {
	host := structOf(boxName)
	for _, name := range methods {
		host.Methods.Append(&emit.Method{Name: name})
	}
	return host
}

// interfaceOf returns an interface declaring the named methods.
func interfaceOf(methods ...string) *emit.Interface {
	i := &emit.Interface{Name: boxName}
	for _, name := range methods {
		i.Methods.Append(&emit.Method{Name: name})
	}
	return i
}

// constantValues lowers an enum and returns its constants' values in
// variant order.
func constantValues(tb assert.TB, e *emit.Enum) []string {
	tb.Helper()

	out, err := backend.Lower(e)
	assert.NoError(tb, err, "the enum lowers")
	var got []string
	for _, d := range out[1:] {
		c, isConstant := d.(*emit.Constant)
		assert.True(tb, isConstant, "a variant lowers to a constant")
		if isConstant {
			got = append(got, c.Value)
		}
	}
	return got
}

// thrower returns a function announcing the given failures beside
// the given results.
func thrower(returns []*emit.Return, failures ...string) *emit.Function {
	f := &emit.Function{Name: fetchName, Returns: returns}
	for _, name := range failures {
		f.Throws = append(f.Throws, &emit.TypeRef{Spelling: name})
	}
	return f
}

// lowered lowers a declaration the lowering rewrites in place.
func lowered(tb assert.TB, s symbol.Symbol) symbol.Symbol {
	tb.Helper()

	out, err := backend.Lower(s)
	assert.NoError(tb, err, "the declaration lowers")
	assert.Length(tb, out, 0, "in place: a nil list keeps the declaration")
	return s
}
