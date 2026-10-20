// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package lowering_test

import (
	"errors"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The language and host the member check names in its error.
const (
	rustLang  = "rust"
	shapeHost = "Shape"
)

// The callables, the failure type, the variant, the field and the type
// parameters that the cases of the sum and the callable steps name.
const (
	readName    = "read"
	writeName   = "write"
	ioError     = "IOError"
	circleName  = "circle"
	radiusName  = "radius"
	firstParam  = "T"
	secondParam = "U"
)

// errName is the value that the guards of the bodies test, and auditSlot
// is the slot that a body declares.
const (
	errName   = "err"
	auditSlot = "audit"
)

// stackMethods is the most methods the member check's set holds on the
// stack.
const stackMethods = 8

// The allocations of the copies and of the member check.
const (
	// copyParamsAllocs is the copy of one parameter with one bound of
	// one argument: the list, the parameter, the list of bounds, the
	// bound, its list of arguments and the argument.
	copyParamsAllocs = 6
	// copyRefsAllocs is the copy of two references without children: the
	// list and the two references.
	copyRefsAllocs = 3
	// copyRefAllocs is the copy of a reference without children.
	copyRefAllocs = 1
	// paramArgsAllocs is the list of two arguments and the two
	// references.
	paramArgsAllocs = 3
	// uniqueAllocs is the member check of more methods than the stack
	// holds: the set at its final size.
	uniqueAllocs = 3
)

// errFold is the refusal of the fold that a case passes to Callables.
var errFold = errors.New("lowering_test: the fold refuses the callable")

// TestLowering checks that the copies share no node with their input,
// that the steps of a sum and of a callable lower what their contracts
// state, and that each error has the prefix of its language.
func TestLowering(t *testing.T) {
	t.Parallel()

	t.Run("CopyTypeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a new parameter for each parameter", func(t *testing.T) {
			t.Parallel()

			in := boundParams()
			out := lowering.CopyTypeParams(in)
			assert.Length(t, out, 1, "one parameter is copied")
			assert.NotEqual(t, out[0], in[0], "the copy is a new parameter", assert.ByIdentity())
			assert.Equal(t, out[0].Name, in[0].Name, "the copy has the name of the parameter")
		})

		t.Run("returns new bounds down to the argument tree", func(t *testing.T) {
			t.Parallel()

			in := boundParams()
			out := lowering.CopyTypeParams(in)
			bound := in[0].Bounds[0]
			assert.NotEqual(t, out[0].Bounds[0], bound, "the bound is new", assert.ByIdentity())
			assert.NotEqual(t, out[0].Bounds[0].Args[0], bound.Args[0], "the argument of the bound is new",
				assert.ByIdentity())
			assert.Equal(t, out[0].Bounds[0].Args[0].Spelling, "K", "the argument has the same spelling")
		})

		t.Run("returns nil for no parameters", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, lowering.CopyTypeParams(nil), "the copy of no parameters is nil")
		})
	})

	t.Run("CopyTypeRefs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a new reference for each reference in order", func(t *testing.T) {
			t.Parallel()

			in := leaves()
			out := lowering.CopyTypeRefs(in)
			assert.Equal(t, out, in, "the copies have the spellings of the references in order")
			assert.NotEqual(t, out[0], in[0], "the copies share no node with the references", assert.ByIdentity())
		})

		t.Run("returns nil for no references", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, lowering.CopyTypeRefs(nil), "the copy of a nil list is nil")
		})
	})

	t.Run("CopyTypeRef", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a tree equal to its input", func(t *testing.T) {
			t.Parallel()

			in := mapOfUsers()
			assert.Equal(t, lowering.CopyTypeRef(in), in, "the copy has the whole tree of the input")
		})

		t.Run("returns a tree that shares no node with its input", func(t *testing.T) {
			t.Parallel()

			in := mapOfUsers()
			out := lowering.CopyTypeRef(in)
			expect.NotEqual(t, out.Elems[1], in.Elems[1], "a child is new", assert.ByIdentity())
			expect.NotEqual(t, out.Elems[1].Elems[0], in.Elems[1].Elems[0], "a child of a child is new",
				assert.ByIdentity())
		})

		t.Run("returns nil for a nil reference", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, lowering.CopyTypeRef(nil), "the copy of a nil reference is nil")
		})
	})

	t.Run("ParamArgs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one reference for each parameter in order", func(t *testing.T) {
			t.Parallel()

			args := lowering.ParamArgs([]*emit.TypeParam{{Name: firstParam}, {Name: secondParam}})
			assert.Equal(t, args, []*emit.TypeRef{{Spelling: firstParam}, {Spelling: secondParam}},
				"each argument has the name of its parameter")
		})

		t.Run("returns nil for no parameters", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, lowering.ParamArgs(nil), "no parameters have no arguments")
		})
	})

	t.Run("NamedPayload", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a payload whose fields have names", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, lowering.NamedPayload(rustLang, circle(radiusName)), "each field has a name")
		})

		t.Run("returns nil for a variant without a payload", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, lowering.NamedPayload(rustLang, &emit.SumVariant{Name: circleName}),
				"a variant without fields has nothing to check")
		})

		t.Run("returns an error for a payload entry without a name", func(t *testing.T) {
			t.Parallel()

			err := lowering.NamedPayload(rustLang, circle(""))
			assert.HasError(t, err, "a field without a name fails")
			assert.Equal(t, err.Error(),
				"rust: each field of a variant's declaration has a name, and a payload entry of circle "+
					"does not have one",
				"the error has the prefix of the language and the name of the variant")
		})
	})

	t.Run("Callables", func(t *testing.T) {
		t.Parallel()

		iface := &emit.Interface{Name: shapeHost}
		iface.Methods.Append(failing()...)
		folded := []struct {
			name string
			give symbol.Symbol
			want []string
		}{
			{
				name: "folds a function with failures",
				give: &emit.Function{Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}}},
				want: []string{readName},
			},
			{
				name: "folds a method with failures",
				give: &emit.Method{Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}}},
				want: []string{readName},
			},
			{
				name: "folds each method of a struct that has failures in order",
				give: failingStruct(),
				want: []string{readName, writeName},
			},
			{
				name: "folds each method of an interface that has failures in order",
				give: iface,
				want: []string{readName, writeName},
			},
			{
				name: "folds a function with failures whose body a template renders",
				give: &emit.Function{
					Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}},
					Body: emit.Body{Ref: &emit.TemplateRef{Name: readName}},
				},
				want: []string{readName},
			},
			{
				name: "folds a function with failures whose prologue has no return statement",
				give: &emit.Function{
					Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}},
					Body: emit.Body{Prologue: emit.SlotOf(emit.Stmt{Kind: emit.StmtGuard, Name: errName})},
				},
				want: []string{readName},
			},
		}
		for _, tt := range folded {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var names []string
				err := lowering.Callables(rustLang, tt.give,
					func(name string, _ *emit.Slot[*emit.Return], _ []*emit.TypeRef) error {
						names = append(names, name)
						return nil
					})
				assert.NoError(t, err, "the fold accepts every callable")
				assert.Equal(t, names, tt.want, "the fold receives each callable with failures by name")
			})
		}

		t.Run("passes the failures of the callable to the fold", func(t *testing.T) {
			t.Parallel()

			fn := &emit.Function{Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}}}
			want := fn.Throws
			var got []*emit.TypeRef
			err := lowering.Callables(rustLang, fn,
				func(_ string, _ *emit.Slot[*emit.Return], throws []*emit.TypeRef) error {
					got = throws
					return nil
				})
			assert.NoError(t, err, "the fold accepts the function")
			assert.Equal(t, got, want, "the fold receives the failures of the function", assert.ByIdentity())
		})

		t.Run("keeps the returns that the fold appends", func(t *testing.T) {
			t.Parallel()

			fn := &emit.Function{Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}}}
			err := lowering.Callables(rustLang, fn,
				func(_ string, returns *emit.Slot[*emit.Return], throws []*emit.TypeRef) error {
					returns.Append(&emit.Return{Type: throws[0]})
					return nil
				})
			assert.NoError(t, err, "the fold accepts the function")
			assert.Length(t, fn.Returns.Items(), 1, "the function has the return that the fold appends")
		})

		t.Run("clears the failures of a callable once the fold returns nil", func(t *testing.T) {
			t.Parallel()

			fn := &emit.Function{Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}}}
			err := lowering.Callables(rustLang, fn,
				func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error { return nil })
			assert.NoError(t, err, "the fold accepts the function")
			assert.Nil(t, fn.Throws, "the function has no failures after the fold")
		})

		t.Run("does not call the fold for a callable without failures", func(t *testing.T) {
			t.Parallel()

			called := false
			err := lowering.Callables(rustLang, &emit.Function{Name: readName},
				func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error {
					called = true
					return nil
				})
			assert.NoError(t, err, "a function without failures has nothing to fold")
			assert.False(t, called, "the fold does not receive a function without failures")
		})

		t.Run("returns the error of the fold", func(t *testing.T) {
			t.Parallel()

			err := lowering.Callables(rustLang, failingStruct(),
				func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error { return errFold })
			assert.ErrorIs(t, err, errFold, "the error of the fold is returned unchanged")
		})

		returning := emit.Stmt{Kind: emit.StmtReturn}
		bodies := []struct {
			name string
			give emit.Body
		}{
			{
				name: "returns an error for a callable with failures and verbatim text in its body",
				give: emit.Body{Verbatim: "return nil"},
			},
			{
				name: "returns an error for a callable with failures and statements in its body",
				give: emit.Body{Stmts: []emit.Stmt{returning}},
			},
			{
				name: "returns an error for a callable with failures and a guard in its prologue that returns",
				give: emit.Body{Prologue: emit.SlotOf(emit.Stmt{
					Kind: emit.StmtGuard, Name: errName, Then: []emit.Stmt{returning},
				})},
			},
			{
				name: "returns an error for a callable with failures and a return statement in its epilogue",
				give: emit.Body{Epilogue: emit.SlotOf(returning)},
			},
			{
				name: "returns an error for a callable with failures and a return statement in a declared slot",
				give: emit.Body{Slots: []*emit.NamedSlot{{Name: auditSlot, Slot: emit.SlotOf(returning)}}},
			},
		}
		for _, tt := range bodies {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				fn := &emit.Function{Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}}, Body: tt.give}
				called := false
				err := lowering.Callables(rustLang, fn,
					func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error {
						called = true
						return nil
					})
				assert.HasError(t, err, "the lowering refuses a body that it does not rewrite")
				expect.Equal(t, err.Error(), "rust: the failures of read move into its results, "+
					"and the lowering does not rewrite the statements or the text of its body",
					"the error has the prefix of the language and the name of the callable")
				expect.False(t, called, "the fold does not receive the callable")
				expect.Length(t, fn.Throws, 1, "the function keeps its failure")
			})
		}

		t.Run("keeps the failures of a callable whose fold returns an error", func(t *testing.T) {
			t.Parallel()

			fn := &emit.Function{Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}}}
			err := lowering.Callables(rustLang, fn,
				func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error { return errFold })
			assert.ErrorIs(t, err, errFold, "the fold refuses the function")
			assert.Length(t, fn.Throws, 1, "the function keeps its failure")
		})

		t.Run("returns an error for a host that declares one method name twice", func(t *testing.T) {
			t.Parallel()

			twice := &emit.Struct{Name: shapeHost}
			twice.Methods.Append(&emit.Method{Name: readName}, &emit.Method{Name: readName})
			err := lowering.Callables(rustLang, twice,
				func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error { return nil })
			assert.HasError(t, err, "a method name declared twice fails")
			assert.Contains(t, err.Error(), "rust: method read", "the error has the prefix of the language")
		})

		t.Run("does not call the fold for a declaration that has no callables", func(t *testing.T) {
			t.Parallel()

			called := false
			err := lowering.Callables(rustLang, &emit.Enum{Name: shapeHost},
				func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error {
					called = true
					return nil
				})
			assert.NoError(t, err, "an enum has nothing to fold")
			assert.False(t, called, "the fold does not receive an enum")
		})
	})

	t.Run("UniqueMethods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for distinct names", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, lowering.UniqueMethods(rustLang, shapeHost, methods(stackMethods+1)),
				"each name once passes")
		})

		t.Run("returns an error that names the method declared twice", func(t *testing.T) {
			t.Parallel()

			err := lowering.UniqueMethods(rustLang, shapeHost, []*emit.Method{{Name: "area"}, {Name: "area"}})
			assert.HasError(t, err, "a name declared twice fails")
			assert.Contains(t, err.Error(), "rust: method area", "the error has the prefix of the language")
		})
	})

	t.Run("DeleteAccessor", func(t *testing.T) {
		t.Parallel()

		deleter := func() *emit.Method { return &emit.Method{Name: "area", Accessor: symbol.AccessorDelete} }
		holding := func(m *emit.Method) *emit.Struct {
			s := &emit.Struct{Name: shapeHost}
			s.Methods.Append(m)
			return s
		}
		refused := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "returns an error for a method with the delete accessor", give: deleter()},
			{name: "returns an error for a struct with a deleter", give: holding(deleter())},
			{
				name: "returns an error for an interface with a deleter",
				give: func() symbol.Symbol {
					i := &emit.Interface{Name: shapeHost}
					i.Methods.Append(deleter())
					return i
				}(),
			},
			{
				name: "returns an error for an enum with a deleter",
				give: func() symbol.Symbol {
					e := &emit.Enum{Name: shapeHost}
					e.Methods.Append(deleter())
					return e
				}(),
			},
			{
				name: "returns an error for a sum with a deleter",
				give: func() symbol.Symbol {
					s := &emit.Sum{Name: shapeHost}
					s.Methods.Append(deleter())
					return s
				}(),
			},
			{
				name: "returns an error for a deleter of a nested type",
				give: func() symbol.Symbol {
					outer := &emit.Struct{Name: shapeHost}
					outer.Types.Append(holding(deleter()))
					return outer
				}(),
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := lowering.DeleteAccessor(rustLang, tt.give)
				assert.HasError(t, err, "the language deletes no property")
				assert.Contains(t, err.Error(), "rust: a property has no deleter", "the error names the language")
			})
		}

		passed := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "returns nil for a getter", give: &emit.Method{Name: "area", Accessor: symbol.AccessorGet}},
			{name: "returns nil for a struct without a deleter", give: holding(&emit.Method{Name: "area"})},
			{name: "returns nil for a function", give: &emit.Function{Name: "area"}},
		}
		for _, tt := range passed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, lowering.DeleteAccessor(rustLang, tt.give), "the declaration deletes no property")
			})
		}
	})
}

// Each copy allocates what it returns, the steps of a sum and of a
// callable allocate what their contracts state, and the member check
// allocates its set only past the room of the stack. The ordinary run,
// which runs no benchmark, checks those ceilings here.
func TestLoweringAllocs(t *testing.T) {
	params, refs, leaf := boundParams(), leaves(), &emit.TypeRef{Spelling: "string"}
	pair := []*emit.TypeParam{{Name: firstParam}, {Name: secondParam}}
	variant := circle(radiusName)
	few, many := methods(stackMethods), methods(stackMethods+1)
	var (
		copiedParams []*emit.TypeParam
		copiedRefs   []*emit.TypeRef
		copied       *emit.TypeRef
		args         []*emit.TypeRef
		err          error
	)
	assert.MaxAllocs(t, func() { copiedParams = lowering.CopyTypeParams(params) }, copyParamsAllocs,
		"CopyTypeParams allocates the copies down to the argument")
	assert.Length(t, copiedParams, 1, "CopyTypeParams copies the parameter")
	assert.MaxAllocs(t, func() { copiedRefs = lowering.CopyTypeRefs(refs) }, copyRefsAllocs,
		"CopyTypeRefs allocates the list and the copies")
	assert.Length(t, copiedRefs, 2, "CopyTypeRefs copies both references")
	assert.MaxAllocs(t, func() { copied = lowering.CopyTypeRef(leaf) }, copyRefAllocs,
		"CopyTypeRef allocates the copy")
	assert.Equal(t, copied.Spelling, leaf.Spelling, "CopyTypeRef copies the spelling")
	assert.MaxAllocs(t, func() { args = lowering.ParamArgs(pair) }, paramArgsAllocs,
		"ParamArgs allocates the list and each argument")
	assert.Length(t, args, 2, "ParamArgs returns one argument for each parameter")
	assert.MaxAllocs(t, func() { err = lowering.NamedPayload(rustLang, variant) }, 0,
		"NamedPayload allocates nothing for fields with names")
	assert.NoError(t, err, "NamedPayload passes fields with names")
	assert.MaxAllocsWithSetup(t, failingStruct,
		func(s *emit.Struct) {
			err = lowering.Callables(rustLang, s,
				func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error { return nil })
		}, 0, "Callables allocates nothing for a host whose fold allocates nothing")
	assert.NoError(t, err, "Callables folds the host")
	assert.MaxAllocs(t, func() { err = lowering.UniqueMethods(rustLang, shapeHost, few) }, 0,
		"UniqueMethods allocates nothing for methods the stack's set holds")
	assert.NoError(t, err, "UniqueMethods passes distinct names")
	assert.MaxAllocs(t, func() { err = lowering.UniqueMethods(rustLang, shapeHost, many) }, uniqueAllocs,
		"UniqueMethods allocates the set past the stack's room")
	assert.NoError(t, err, "UniqueMethods passes distinct names")
	host := &emit.Struct{Name: shapeHost}
	host.Methods.Append(few...)
	assert.MaxAllocs(t, func() { err = lowering.DeleteAccessor(rustLang, host) }, 0,
		"DeleteAccessor allocates nothing for a host without a deleter")
	assert.NoError(t, err, "DeleteAccessor passes a host without a deleter")
}

// BenchmarkLowering measures each copy a backend's lowering makes of a
// shape, the steps of the lowering of a sum and of a callable, and the
// member check over a host's methods.
func BenchmarkLowering(b *testing.B) {
	b.Run("CopyTypeParams", func(b *testing.B) {
		params := boundParams()
		c := bench.Start(b).MaxAllocs(copyParamsAllocs)
		defer c.End()
		var got []*emit.TypeParam
		for c.Loop() {
			got = lowering.CopyTypeParams(params)
		}
		assert.Length(b, got, 1, "CopyTypeParams copies the parameter")
	})

	b.Run("CopyTypeRefs", func(b *testing.B) {
		refs := leaves()
		c := bench.Start(b).MaxAllocs(copyRefsAllocs)
		defer c.End()
		var got []*emit.TypeRef
		for c.Loop() {
			got = lowering.CopyTypeRefs(refs)
		}
		assert.Length(b, got, 2, "CopyTypeRefs copies both references")
	})

	b.Run("CopyTypeRef", func(b *testing.B) {
		leaf := &emit.TypeRef{Spelling: "string"}
		c := bench.Start(b).MaxAllocs(copyRefAllocs)
		defer c.End()
		var got *emit.TypeRef
		for c.Loop() {
			got = lowering.CopyTypeRef(leaf)
		}
		assert.Equal(b, got.Spelling, leaf.Spelling, "CopyTypeRef copies the spelling")
	})

	b.Run("ParamArgs", func(b *testing.B) {
		pair := []*emit.TypeParam{{Name: firstParam}, {Name: secondParam}}
		c := bench.Start(b).MaxAllocs(paramArgsAllocs)
		defer c.End()
		var got []*emit.TypeRef
		for c.Loop() {
			got = lowering.ParamArgs(pair)
		}
		assert.Length(b, got, 2, "ParamArgs returns one argument for each parameter")
	})

	b.Run("NamedPayload", func(b *testing.B) {
		variant := circle(radiusName)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = lowering.NamedPayload(rustLang, variant)
		}
		assert.NoError(b, err, "NamedPayload passes fields with names")
	})

	b.Run("Callables", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			var s *emit.Struct
			c.Excluding(func() { s = failingStruct() })
			err = lowering.Callables(rustLang, s,
				func(string, *emit.Slot[*emit.Return], []*emit.TypeRef) error { return nil })
		}
		assert.NoError(b, err, "Callables folds the host")
	})

	tests := []struct {
		name    string
		methods []*emit.Method
		allocs  uint64
	}{
		{name: "methods that fit on the stack", methods: methods(stackMethods)},
		{
			name:    "more methods than fit on the stack",
			methods: methods(stackMethods + 1),
			allocs:  uniqueAllocs,
		},
	}
	b.Run("UniqueMethods", func(b *testing.B) {
		for _, tt := range tests {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var err error
				for c.Loop() {
					err = lowering.UniqueMethods(rustLang, shapeHost, tt.methods)
				}
				assert.NoError(b, err, "UniqueMethods passes distinct names")
			})
		}
	})

	b.Run("DeleteAccessor", func(b *testing.B) {
		host := &emit.Struct{Name: shapeHost}
		host.Methods.Append(methods(stackMethods)...)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = lowering.DeleteAccessor(rustLang, host)
		}
		assert.NoError(b, err, "DeleteAccessor passes a host without a deleter")
	})
}

// boundParams returns one parameter T with one bound Ord of one
// argument K.
func boundParams() []*emit.TypeParam {
	bound := &emit.TypeRef{Spelling: "Ord", Args: []*emit.TypeRef{{Spelling: "K"}}}
	return []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{bound}}}
}

// leaves returns two references without children.
func leaves() []*emit.TypeRef {
	return []*emit.TypeRef{{Spelling: "string"}, {Spelling: "int"}}
}

// mapOfUsers returns a map from strings to optional users: a reference
// whose children have children.
func mapOfUsers() *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: "map[string]*User", Form: symbol.FormMap,
		Elems: []*emit.TypeRef{
			{Spelling: "string"},
			{Spelling: "*User", Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: "User"}}},
		},
	}
}

// methods returns n methods of distinct names.
func methods(n int) []*emit.Method {
	out := make([]*emit.Method, 0, n)
	for i := range n {
		out = append(out, &emit.Method{Name: "m" + strconv.Itoa(i)})
	}
	return out
}

// circle returns the variant circle with one field of the given name.
func circle(field string) *emit.SumVariant {
	v := &emit.SumVariant{Name: circleName}
	v.Fields.Append(&emit.Field{Name: field, Type: &emit.TypeRef{Spelling: "f64"}})
	return v
}

// failing returns the methods read and write, each with the failure
// IOError, and between them the method close, which has no failures.
func failing() []*emit.Method {
	return []*emit.Method{
		{Name: readName, Throws: []*emit.TypeRef{{Spelling: ioError}}},
		{Name: "close"},
		{Name: writeName, Throws: []*emit.TypeRef{{Spelling: ioError}}},
	}
}

// failingStruct returns the struct Shape with the methods that failing
// returns.
func failingStruct() *emit.Struct {
	s := &emit.Struct{Name: shapeHost}
	s.Methods.Append(failing()...)
	return s
}
