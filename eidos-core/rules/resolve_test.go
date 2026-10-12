// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The package of the resolution fixture and the names that it declares
// beside the names that the other specs of the package share.
const (
	resPath     = "res"
	pageName    = "Page"
	openName    = "Open"
	pureName    = "Pure"
	helperName  = "helper"
	maxName     = "Max"
	putName     = "Put"
	nextName    = "Next"
	pageParam   = "page"
	sizeField   = "size"
	secretField = "secret"
	innerField  = "inner"
)

// valueFieldAllocs is the allocation count of the resolution of a value
// field on a type subject. The walk allocates the list of the one type that
// it searches and the members that the member walk returns.
const valueFieldAllocs = 2

// errNotInScope is the error that [probing] returns for a name that the
// package of the subject does not declare.
var errNotInScope = errors.New("rules_test: no declaration of the name is in scope")

// probing is the resolver of the cases. It resolves a name in scope to a
// declaration in the package of the subject that has the name and one of
// the kinds. It resolves int to a placeholder, and it takes the element of
// an optional as the type of its value. No subject can use the member
// named [secretField], and only the own type can use the member named
// [innerField].
type probing struct{}

// InScope returns the first declaration in the package of the subject that
// has the name and one of the kinds. It returns errNotInScope when the
// package declares none.
func (probing) InScope(scope rules.Scope, name string, kinds []symbol.Kind, v rules.View) (symbol.Symbol, error) {
	for _, k := range kinds {
		if sym, held := v.Lookup(coretest.ID(scope.Subject.Package, name, k)); held {
			return sym, nil
		}
	}
	return nil, errNotInScope
}

// Usable reports false for the secret member, and for the inner member
// where own is zero.
func (probing) Usable(m rules.Member, own symbol.Identity, _ rules.Scope) bool {
	decl, _ := m.Symbol.(node.Declaration)
	switch decl.Identity().Name {
	case secretField:
		return false
	case innerField:
		return !own.IsZero()
	default:
		return true
	}
}

// Placeholder returns an alias named int for int, and false for every
// other name.
func (probing) Placeholder(_ rules.Scope, name string, _ rules.View) (symbol.Symbol, bool) {
	if name != intSpelling {
		return nil, false
	}
	return &node.Alias{ID: symbol.Identity{Lang: coretest.Lang, Name: name, Kind: symbol.KindAlias}, Name: name}, true
}

// Value returns the element of an optional, and every other reference
// unchanged.
func (probing) Value(ref *node.TypeRef, _ rules.View) *node.TypeRef {
	if ref.Form == symbol.FormOptional && len(ref.Elems) == 1 {
		return ref.Elems[0]
	}
	return ref
}

// TestResolve checks each resolution that the walk performs, the members
// that a subject can use, and each error of the walk.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		resolved := []struct {
			name    string
			subject symbol.Identity
			give    string
			kind    directive.ResolutionKind
			want    symbol.Identity
		}{
			{
				name:    "resolves a callable of a method subject among the methods of its type",
				subject: coretest.MemberID(resPath, rowName, getName, symbol.KindMethod),
				give:    putName,
				kind:    directive.ResolveCallableInScope,
				want:    coretest.MemberID(resPath, rowName, putName, symbol.KindMethod),
			},
			{
				name:    "resolves a callable of a method subject to a function in scope",
				subject: coretest.MemberID(resPath, rowName, getName, symbol.KindMethod),
				give:    helperName,
				kind:    directive.ResolveCallableInScope,
				want:    coretest.ID(resPath, helperName, symbol.KindFunction),
			},
			{
				name:    "resolves a callable of a function subject in scope",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    helperName,
				kind:    directive.ResolveCallableInScope,
				want:    coretest.ID(resPath, helperName, symbol.KindFunction),
			},
			{
				name:    "resolves a package variable to a constant in scope",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    maxName,
				kind:    directive.ResolvePackageVar,
				want:    coretest.ID(resPath, maxName, symbol.KindConstant),
			},
			{
				name:    "resolves a type in scope",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    pageName,
				kind:    directive.ResolveTypeInScope,
				want:    coretest.ID(resPath, pageName, symbol.KindStruct),
			},
			{
				name:    "resolves a type to the placeholder of the language",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    intSpelling,
				kind:    directive.ResolveTypeInScope,
				want:    symbol.Identity{Lang: coretest.Lang, Name: intSpelling, Kind: symbol.KindAlias},
			},
			{
				name:    "resolves a value field among the fields of a type subject",
				subject: coretest.ID(resPath, rowName, symbol.KindStruct),
				give:    nameField,
				kind:    directive.ResolveValueField,
				want:    coretest.MemberID(resPath, rowName, nameField, symbol.KindField),
			},
			{
				name:    "resolves a value field among the fields of the type of a field subject",
				subject: coretest.MemberID(resPath, rowName, countField, symbol.KindField),
				give:    nameField,
				kind:    directive.ResolveValueField,
				want:    coretest.MemberID(resPath, rowName, nameField, symbol.KindField),
			},
			{
				name:    "resolves a value field among the fields of the return type of a callable",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    countField,
				kind:    directive.ResolveValueField,
				want:    coretest.MemberID(resPath, rowName, countField, symbol.KindField),
			},
			{
				name:    "resolves a value field among the fields of the type of an input parameter",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    sizeField,
				kind:    directive.ResolveValueField,
				want:    coretest.MemberID(resPath, pageName, sizeField, symbol.KindField),
			},
			{
				name:    "resolves a value field through the type that the resolver returns for a value",
				subject: coretest.ID(resPath, openName, symbol.KindFunction),
				give:    nameField,
				kind:    directive.ResolveValueField,
				want:    coretest.MemberID(resPath, rowName, nameField, symbol.KindField),
			},
			{
				name:    "resolves a member on a handle among the methods of the return type",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    putName,
				kind:    directive.ResolveMemberOnHandle,
				want:    coretest.MemberID(resPath, rowName, putName, symbol.KindMethod),
			},
			{
				name:    "resolves a member that only the own type can use on the own type",
				subject: coretest.ID(resPath, rowName, symbol.KindStruct),
				give:    innerField,
				kind:    directive.ResolveValueField,
				want:    coretest.MemberID(resPath, rowName, innerField, symbol.KindField),
			},
		}
		for _, tt := range resolved {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				sym, err := resolveIn(t, tt.subject, tt.give, tt.kind)
				assert.NoError(t, err, "Resolve returns no error")
				decl, names := sym.(node.Declaration)
				assert.True(t, names, "the walk returns a declaration")
				assert.Equal(t, decl.Identity(), tt.want, "the walk returns the declaration of the name")
			})
		}

		t.Run("resolves a host parameter on the signature of the subject", func(t *testing.T) {
			t.Parallel()

			sym, err := resolveIn(t, coretest.ID(resPath, fetchName, symbol.KindFunction), pageParam,
				directive.ResolveHostParam)
			assert.NoError(t, err, "Resolve returns no error")
			param, is := sym.(*node.Param)
			assert.True(t, is, "the walk returns a parameter")
			assert.Equal(t, param.Name, pageParam, "the walk returns the parameter of the name")
		})

		refused := []struct {
			name    string
			subject symbol.Identity
			give    string
			kind    directive.ResolutionKind
			want    string
		}{
			{
				name:    "returns an error for an empty spelling",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    " ",
				kind:    directive.ResolvePackageVar,
				want:    "nothing to resolve",
			},
			{
				name:    "returns an error for a kind that the walk does not perform",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    maxName,
				kind:    directive.ResolveNone,
				want:    "is not a resolution that golang performs",
			},
			{
				name:    "returns an error for a callable that neither the type nor the scope declares",
				subject: coretest.MemberID(resPath, rowName, getName, symbol.KindMethod),
				give:    ghostName,
				kind:    directive.ResolveCallableInScope,
				want:    "has no method named Ghost",
			},
			{
				name:    "returns an error for a member that no searched type declares",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    ghostName,
				kind:    directive.ResolveValueField,
				want:    "has no Field member named Ghost",
			},
			{
				name:    "returns an error for a member that the subject cannot use",
				subject: coretest.ID(resPath, rowName, symbol.KindStruct),
				give:    secretField,
				kind:    directive.ResolveValueField,
				want:    "has no Field member named secret",
			},
			{
				name:    "returns an error for a member that only the own type can use on another type",
				subject: coretest.MemberID(resPath, pageName, nextName, symbol.KindMethod),
				give:    innerField,
				kind:    directive.ResolveValueField,
				want:    "has no Field member named inner",
			},
			{
				name:    "returns an error for a member on a handle among the input parameters",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    sizeField,
				kind:    directive.ResolveMemberOnHandle,
				want:    "has no Field or Method member named size",
			},
			{
				name:    "returns an error for a member of a subject outside the view",
				subject: coretest.ID(resPath, ghostName, symbol.KindFunction),
				give:    nameField,
				kind:    directive.ResolveValueField,
				want:    "does not belong to a type that the view declares",
			},
			{
				name:    "returns an error for a callable without a value of a declared type",
				subject: coretest.ID(resPath, pureName, symbol.KindFunction),
				give:    nameField,
				kind:    directive.ResolveValueField,
				want:    "has no value of a type that the view declares",
			},
			{
				name:    "returns an error for a host parameter of a subject that is not a callable",
				subject: coretest.ID(resPath, rowName, symbol.KindStruct),
				give:    pageParam,
				kind:    directive.ResolveHostParam,
				want:    "is not a callable",
			},
			{
				name:    "returns an error for a host parameter that the subject does not declare",
				subject: coretest.ID(resPath, fetchName, symbol.KindFunction),
				give:    ghostName,
				kind:    directive.ResolveHostParam,
				want:    "has no parameter named Ghost",
			},
			{
				name:    "returns an error for a host parameter of a subject outside the view",
				subject: coretest.ID(resPath, ghostName, symbol.KindFunction),
				give:    pageParam,
				kind:    directive.ResolveHostParam,
				want:    "is outside the view",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := resolveIn(t, tt.subject, tt.give, tt.kind)
				refusal := assert.ErrorAs[*rules.ResolveError](t, err, "the walk returns a ResolveError")
				assert.Equal(t, refusal.Lang, coretest.Lang, "the error has the language of the rules")
				assert.Contains(t, refusal.Msg, tt.want, "the error has the reason")
			})
		}

		t.Run("returns an error under the zero language for nil rules", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, resolving()))
			_, err := rules.Resolve(probing{}, nil, v, rules.Scope{}, "", directive.ResolvePackageVar)
			refusal := assert.ErrorAs[*rules.ResolveError](t, err, "the walk returns a ResolveError")
			assert.Equal(t, refusal.Lang, symbol.Lang(""), "the error has the zero language")
		})

		resolver := []struct {
			name string
			kind directive.ResolutionKind
		}{
			{name: "returns the error of the resolver for a package variable", kind: directive.ResolvePackageVar},
			{name: "returns the error of the resolver for a type", kind: directive.ResolveTypeInScope},
			{
				name: "returns the error of the resolver for a callable of a function subject",
				kind: directive.ResolveCallableInScope,
			},
		}
		for _, tt := range resolver {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := resolveIn(t, coretest.ID(resPath, fetchName, symbol.KindFunction), ghostName, tt.kind)
				assert.ErrorIs(t, err, errNotInScope, "the walk returns the error of the resolver unchanged")
			})
		}
	})

	t.Run("ResolveError", func(t *testing.T) {
		t.Parallel()

		t.Run("Error", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the language before the reason", func(t *testing.T) {
				t.Parallel()

				err := &rules.ResolveError{Lang: coretest.Lang, Msg: "nothing to resolve"}
				assert.Equal(t, err.Error(), "golang: nothing to resolve", "the text starts with the language")
			})
		})
	})
}

// TestResolveAllocs checks the ceilings of BenchmarkResolve. The ordinary
// run runs no benchmark, so this test checks them. A resolution in scope
// allocates what the resolver allocates. A member resolution allocates the
// list of the types that it searches and the members of the walk.
func TestResolveAllocs(t *testing.T) {
	checkCalls(t, resolveCalls(t))
}

// BenchmarkResolve measures a resolution in scope and a member
// resolution, which a directive validation makes once for each reference
// parameter.
func BenchmarkResolve(b *testing.B) {
	benchCalls(b, resolveCalls(b))
}

// resolveCalls returns a resolution of a package variable and of a value
// field on a type subject over the resolution fixture.
func resolveCalls(tb assert.TB) []allocCall {
	tb.Helper()

	v, _, _ := viewOver(tb, coretest.Frozen(tb, resolving()))
	source := scripted()
	fetch := rules.Scope{Subject: coretest.ID(resPath, fetchName, symbol.KindFunction)}
	row := rules.Scope{Subject: coretest.ID(resPath, rowName, symbol.KindStruct)}
	var (
		sym symbol.Symbol
		err error
	)
	return []allocCall{
		{
			name:     "Resolve",
			caseName: "a package variable",
			call:     func() { sym, err = rules.Resolve(probing{}, source, v, fetch, maxName, directive.ResolvePackageVar) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve resolves the constant")
				assert.Equal(tb, sym.Kind(), symbol.KindConstant, "Resolve returns the constant")
			},
		},
		{
			name:     "Resolve",
			caseName: "a value field on a type subject",
			allocs:   valueFieldAllocs,
			call:     func() { sym, err = rules.Resolve(probing{}, source, v, row, nameField, directive.ResolveValueField) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve resolves the field")
				assert.Equal(tb, sym.Kind(), symbol.KindField, "Resolve returns the field")
			},
		},
	}
}

// resolveIn resolves name at kind from subject through [probing] over a
// new view of the resolution fixture.
func resolveIn(
	tb assert.TB,
	subject symbol.Identity,
	name string,
	kind directive.ResolutionKind,
) (symbol.Symbol, error) {
	tb.Helper()

	v, _, _ := viewOver(tb, coretest.Frozen(tb, resolving()))
	return rules.Resolve(probing{}, scripted(), v, rules.Scope{Subject: subject}, name, kind)
}

// resolving builds the resolution fixture. It declares these:
//   - Row, with the fields name, count, secret and inner, and the methods
//     Get and Put
//   - Page, with the field size and the method Next, which returns a Row
//   - Fetch, which takes a Page and returns a Row
//   - Open, which returns an optional Row
//   - Pure, which takes an int and returns an int
//   - helper, a function, and Max, a constant
func resolving() *node.Package {
	rowRef := named(resPath, rowName, symbol.KindStruct)
	row := coretest.Struct(resPath, rowName)
	row.Fields = []*node.Field{
		field(resPath, rowName, nameField, builtin(strSpelling)),
		field(resPath, rowName, countField, builtin(intSpelling)),
		field(resPath, rowName, secretField, builtin(intSpelling)),
		field(resPath, rowName, innerField, builtin(intSpelling)),
	}
	row.Methods = []*node.Method{method(resPath, rowName, getName), method(resPath, rowName, putName)}
	page := coretest.Struct(resPath, pageName)
	page.Fields = []*node.Field{field(resPath, pageName, sizeField, builtin(intSpelling))}
	next := coretest.Method(resPath, pageName, nextName)
	next.Returns = []*node.Return{{Type: rowRef}}
	page.Methods = []*node.Method{next}
	fetch := coretest.Function(resPath, fetchName)
	fetch.Params = []*node.Param{coretest.Param(resPath, fetchName, pageParam)}
	fetch.Params[0].Type = named(resPath, pageName, symbol.KindStruct)
	fetch.Returns[0].Type = rowRef
	open := coretest.Function(resPath, openName)
	open.Params = nil
	open.Returns[0].Type = &node.TypeRef{
		Spelling: "*" + rowName, Form: symbol.FormOptional, Elems: []*node.TypeRef{rowRef},
	}
	pure := coretest.Function(resPath, pureName)
	pure.Params[0].Type = builtin(intSpelling)
	pure.Returns[0].Type = builtin(intSpelling)
	return coretest.Package(resPath, row, page, fetch, open, pure,
		coretest.Function(resPath, helperName), coretest.Constant(resPath, maxName))
}
