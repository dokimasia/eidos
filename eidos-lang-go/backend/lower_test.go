// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"cmp"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

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

// The variant names in upper case and in mixed case that the join
// cases lower, and the constants that they join to.
const (
	upperOpen         = "OPEN"
	upperStatus       = "STATUS_ACTIVE"
	upperUserID       = "USER_ID"
	mixedName         = "OpenNow"
	phaseStatusActive = "phaseStatusActive"
	phaseUserID       = "phaseUserID"
	phaseOpenNow      = "phaseOpenNow"
)

// The cases lower a sum of a circle with a radius and an empty
// variant, and pin the marker method, the variant structs and their
// receivers that it lowers to.
const (
	circleName      = "circle"
	emptyName       = "empty"
	radiusName      = "radius"
	shapeMarker     = "isShape"
	shapeCircle     = "shapeCircle"
	shapeEmpty      = "shapeEmpty"
	circleReceiver  = "*shapeCircle"
	genericReceiver = "*shapeCircle[K, V]"
)

// The allocations of a lowering.
const (
	// enumLowerAllocs is an enum of three variants without values: the
	// list of outputs, the defined type and its target, and per variant
	// the constant, its type, and its joined name's two.
	enumLowerAllocs = 1 + 2 + 3*4
	// upperEnumLowerAllocs is an enum of two variant names in upper case.
	// Each variant adds its lower-cased name to what a variant of
	// enumLowerAllocs allocates.
	upperEnumLowerAllocs = 1 + 2 + 2*5
	// sumLowerAllocs is a sum of a variant with one field and a variant
	// without fields: the list of outputs, the interface, its marker
	// method, the method's list and the marker's name's two, and per
	// variant the struct, its joined name's two, the marker method and its
	// list, the receiver, its pointer type, the type's list of one element,
	// the element and the pointer's spelling. The variant with a field
	// adds its list of fields.
	sumLowerAllocs = 6 + 2*10 + 1
	// genericSumLowerAllocs is the same sum over two type parameters. Each
	// variant adds the copied list of parameters and each copy, the list
	// of the receiver's arguments and each argument, and the spelling of
	// the arguments.
	genericSumLowerAllocs = sumLowerAllocs + 2*(3+3+1)
	// throwsAllocs is a function announcing one failure: the error
	// return, its type, and the grown list of returns.
	throwsAllocs = 1 + 1 + 1
	// receiveAllocs is a struct of two methods: each method's receiver.
	receiveAllocs = 2
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

		names := []struct {
			name string
			give string
			want string
		}{
			{name: "joins a variant name in upper case as a word in title case", give: upperOpen, want: phaseOpen},
			{
				name: "joins each word of a variant name in upper case in title case",
				give: upperStatus, want: phaseStatusActive,
			},
			{name: "keeps an initialism of a variant name in upper case", give: upperUserID, want: phaseUserID},
			{name: "keeps a variant name in mixed case as written", give: mixedName, want: phaseOpenNow},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				e := &emit.Enum{Name: phaseName}
				e.Variants.Append(&emit.EnumVariant{Name: tt.give})
				out, err := backend.Lower(e)
				assert.NoError(t, err, "the enum lowers")
				assert.Length(t, out, 2, "one defined type and one constant")
				constant, isConstant := out[1].(*emit.Constant)
				assert.True(t, isConstant, "the variant becomes a constant")
				assert.Equal(t, constant.Name, tt.want, "the constant joins the type's name and the variant's words")
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
			assert.NotEqual(t, methods[0].Receives.Args[0], methods[1].Receives.Args[0],
				"a respell of one method's receiver leaves the other's", assert.ByIdentity())
		})

		t.Run("fills the host receiver of a struct method stating none", func(t *testing.T) {
			t.Parallel()

			origin := symbol.Identity{Package: storePkg, Name: rowName}
			s := &emit.Struct{Name: rowName, Origin: origin}
			s.Methods.Append(&emit.Method{Name: loadName})
			lowered(t, s)
			filled := s.Methods.Items()[0]
			assert.NotNil(t, filled.Receives, "an unstated receiver is filled")
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
			assert.Nil(t, s.Methods.Items()[0].Receives,
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
			assert.Empty(t, f.Throws, "a second settle finds nothing to lower")
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
			assert.Empty(t, m.Throws, "the member's failures are consumed")
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

		t.Run("passes a struct through unchanged", func(t *testing.T) {
			t.Parallel()

			lowered(t, &emit.Struct{Name: rowName})
		})

		t.Run("reshapes a sum into an interface followed by one struct per variant", func(t *testing.T) {
			t.Parallel()

			s := shapeSum()
			out, err := backend.Lower(s)
			assert.NoError(t, err, "the sum lowers")
			assert.Length(t, out, 3, "one interface and one struct per variant")

			iface, isInterface := out[0].(*emit.Interface)
			assert.True(t, isInterface, "the principal output is the interface")
			expect.Equal(t, iface.Name, shapeName, "the interface keeps the sum's name")
			expect.Equal(t, iface.Doc, s.Doc, "the interface keeps the sum's documentation")
			expect.Equal(t, iface.Origin, s.Origin, "the interface has the sum's origin")

			circle, isStruct := out[1].(*emit.Struct)
			assert.True(t, isStruct, "the first variant becomes a struct")
			expect.Equal(t, circle.Name, shapeCircle, "the struct joins the sum's name and the variant's")
			expect.Equal(t, circle.Doc, s.Variants.Items()[0].Doc, "the struct keeps the variant's documentation")
			expect.Equal(t, circle.Origin, s.Origin, "the struct has the sum's origin")

			empty, isStruct := out[2].(*emit.Struct)
			assert.True(t, isStruct, "the second variant becomes a struct")
			expect.Equal(t, empty.Name, shapeEmpty, "the second struct follows the first")
		})

		t.Run("declares the marker as the one method of the interface", func(t *testing.T) {
			t.Parallel()

			out, err := backend.Lower(shapeSum())
			assert.NoError(t, err, "the sum lowers")
			methods := out[0].(*emit.Interface).Methods.Items()
			assert.Length(t, methods, 1, "the interface declares the marker alone")
			expect.Equal(t, methods[0].Name, shapeMarker, "the marker joins is and the sum's name")
			expect.Equal(t, methods[0].Visibility, symbol.VisibilityPackage,
				"the marker is unexported, so a type of another package cannot declare it")
		})

		t.Run("implements the marker on each struct through a pointer receiver", func(t *testing.T) {
			t.Parallel()

			s := shapeSum()
			out, err := backend.Lower(s)
			assert.NoError(t, err, "the sum lowers")
			methods := out[1].(*emit.Struct).Methods.Items()
			assert.Length(t, methods, 1, "the struct declares the marker alone")
			m := methods[0]
			expect.Equal(t, m.Name, shapeMarker, "the struct's method is the interface's marker")
			expect.True(t, m.Body.IsZero(), "the marker's body is empty")
			assert.NotNil(t, m.Receiver, "the marker states its receiver")
			expect.Equal(t, m.Receiver.Name, "", "the receiver has no name, because the empty body reads none")
			expect.Equal(t, m.Receiver.Type, &emit.TypeRef{
				Spelling: circleReceiver, Form: symbol.FormOptional,
				Elems: []*emit.TypeRef{{Spelling: shapeCircle, Target: s.Origin}},
			}, "the receiver points at the struct through the sum's origin, so the settle respells both")
		})

		t.Run("gives each struct the fields of its variant", func(t *testing.T) {
			t.Parallel()

			s := shapeSum()
			out, err := backend.Lower(s)
			assert.NoError(t, err, "the sum lowers")
			expect.Equal(t, out[1].(*emit.Struct).Fields.Items(), s.Variants.Items()[0].Fields.Items(),
				"the circle's struct has the radius")
			expect.Empty(t, out[2].(*emit.Struct).Fields.Items(), "the empty variant's struct has no fields")
		})

		t.Run("restates a generic sum's parameters on each variant's receiver", func(t *testing.T) {
			t.Parallel()

			s := shapeSum()
			s.TypeParams = []*emit.TypeParam{{Name: keyParam}, {Name: valueParam}}
			out, err := backend.Lower(s)
			assert.NoError(t, err, "the generic sum lowers")
			expect.Equal(t, out[0].(*emit.Interface).TypeParams, s.TypeParams,
				"the interface keeps the sum's parameters")
			circle := out[1].(*emit.Struct)
			expect.Equal(t, circle.TypeParams, s.TypeParams, "the struct declares the sum's parameters")
			expect.NotEqual(t, circle.TypeParams[0], s.TypeParams[0],
				"the struct declares a copy, so a respell of one output leaves the others", assert.ByIdentity())
			expect.Equal(t, circle.Methods.Items()[0].Receiver.Type.Spelling, genericReceiver,
				"the receiver restates the parameters as arguments")
		})

		t.Run("returns an error for a sum with methods", func(t *testing.T) {
			t.Parallel()

			s := shapeSum()
			s.Methods.Append(&emit.Method{Name: getName})
			_, err := backend.Lower(s)
			assert.HasError(t, err, "no variant struct has a body for the sum's methods")
		})

		t.Run("returns an error for a variant field without a name", func(t *testing.T) {
			t.Parallel()

			s := shapeSum()
			s.Variants.Items()[0].Fields.Append(&emit.Field{Type: &emit.TypeRef{Spelling: intResultType}})
			_, err := backend.Lower(s)
			assert.HasError(t, err, "a Go struct field has a name")
		})
	})
}

// A declaration Go states as it is allocates nothing, and a reshaping
// allocates the declarations and the references it adds. A lowering in
// place consumes its fact, so each counted call lowers a fresh
// declaration, built outside the count. The ordinary run, which runs no
// benchmark, checks those ceilings here. Each count keeps the first
// error of its calls, which cmp.Or returns without allocating.
func TestLowerAllocs(t *testing.T) {
	checkAllocs(t, lowerCalls())

	var err error
	assert.MaxAllocsWithSetup(t, func() *emit.Function { return thrower(nil, overflowType) },
		func(f *emit.Function) {
			_, lerr := backend.Lower(f)
			err = cmp.Or(err, lerr)
		}, throwsAllocs, "Lower appends the error return within its ceiling")
	assert.NoError(t, err, "Lower lowers every function that throws")

	assert.MaxAllocsWithSetup(t, func() *emit.Struct { return hostOf(getName, putName) },
		func(s *emit.Struct) {
			_, lerr := backend.Lower(s)
			err = cmp.Or(err, lerr)
		}, receiveAllocs, "Lower fills the receivers within its ceiling")
	assert.NoError(t, err, "Lower lowers every struct")
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
// it is, over an enum, over an enum of variant names in upper case, over
// a sum and over a generic sum.
func lowerCalls() []allocCall {
	constant := &emit.Constant{Name: countName, Value: closedValue}
	phase := enumOf("", "", "")
	upper := &emit.Enum{Name: phaseName}
	upper.Variants.Append(&emit.EnumVariant{Name: upperOpen}, &emit.EnumVariant{Name: upperStatus})
	shape := shapeSum()
	generic := shapeSum()
	generic.TypeParams = []*emit.TypeParam{{Name: keyParam}, {Name: valueParam}}
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
				assert.Empty(tb, out, "Lower keeps the constant in place")
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
		{
			name: "Lower", caseName: "an enum of variant names in upper case", allocs: upperEnumLowerAllocs,
			call: func() { out, err = backend.Lower(upper) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower reshapes the enum")
				assert.Length(tb, out, 3, "Lower returns the type and two constants")
			},
		},
		{
			name: "Lower", caseName: "a sum", allocs: sumLowerAllocs,
			call: func() { out, err = backend.Lower(shape) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower reshapes the sum")
				assert.Length(tb, out, 3, "Lower returns the interface and two structs")
			},
		},
		{
			name: "Lower", caseName: "a generic sum", allocs: genericSumLowerAllocs,
			call: func() { out, err = backend.Lower(generic) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower reshapes the generic sum")
				assert.Length(tb, out, 3, "Lower returns the interface and two generic structs")
			},
		},
	}
}

// shapeSum returns a sum of a circle with a radius and an empty
// variant, each documented.
func shapeSum() *emit.Sum {
	s := &emit.Sum{
		Origin: symbol.Identity{Lang: protoLang, Package: svcPkg, Name: shapeName, Kind: symbol.KindSum},
		Doc:    []string{"shape is one closed figure."},
		Name:   shapeName,
	}
	circle := &emit.SumVariant{Doc: []string{"circle bounds by a radius."}, Name: circleName}
	circle.Fields.Append(&emit.Field{Name: radiusName, Type: &emit.TypeRef{Spelling: intResultType}})
	s.Variants.Append(circle, &emit.SumVariant{Name: emptyName})
	return s
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
	got := make([]string, 0, len(out)-1)
	for _, d := range out[1:] {
		c, isConstant := d.(*emit.Constant)
		assert.True(tb, isConstant, "a variant lowers to a constant")
		got = append(got, c.Value)
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
	assert.Empty(tb, out, "in place: a nil list keeps the declaration")
	return s
}
