// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The ceilings of the resolutions. A type of the package of the subject
// allocates nothing. A type of the package of the schema for a subject of
// the documents allocates the path of the package of the documents of the
// subject, which it probes before the package of the schema.
const (
	ownTypeAllocs    = 0
	schemaTypeAllocs = 1
)

// The subjects of the resolutions beside Book: a field of Book, the root of
// queries, a field with arguments of it, and an operation.
var (
	titleID = symbol.Identity{
		Lang:    graphql.Lang,
		Package: shopPkg,
		Owner:   "Book",
		Name:    "title",
		Kind:    symbol.KindField,
	}
	queryID  = symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Query", Kind: symbol.KindStruct}
	lookupID = symbol.Identity{
		Lang:    graphql.Lang,
		Package: shopPkg,
		Owner:   "Query",
		Name:    "book",
		Kind:    symbol.KindMethod,
	}
	getBookID = symbol.Identity{Lang: graphql.Lang, Package: docsPkg, Name: "GetBook", Kind: symbol.KindFunction}
)

// TestResolve pins the resolution of the spelling of a directive from each
// kind of subject.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			subject symbol.Identity
			give    string
			kind    directive.ResolutionKind
			want    symbol.Identity
		}{
			{
				name:    "returns an object type of the package of the subject",
				subject: bookID, give: "Author", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Author", Kind: symbol.KindStruct},
			},
			{
				name:    "returns an enum of the package of the subject",
				subject: bookID, give: "Status", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Status", Kind: symbol.KindEnum},
			},
			{
				name:    "returns a union of the package of the subject",
				subject: bookID,
				give:    "SearchResult",
				kind:    directive.ResolveTypeInScope,
				want: symbol.Identity{
					Lang:    graphql.Lang,
					Package: shopPkg,
					Name:    "SearchResult",
					Kind:    symbol.KindAlias,
				},
			},
			{
				name:    "returns an interface of the package of the subject",
				subject: bookID, give: "Node", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Node", Kind: symbol.KindInterface},
			},
			{
				name:    "returns a type of the documents for a subject of the schema",
				subject: bookID,
				give:    "GetBookQuery",
				kind:    directive.ResolveTypeInScope,
				want: symbol.Identity{
					Lang:    graphql.Lang,
					Package: docsPkg,
					Name:    "GetBookQuery",
					Kind:    symbol.KindStruct,
				},
			},
			{
				name:    "returns a type of the schema for a subject of the documents",
				subject: getBookID, give: "Book", kind: directive.ResolveTypeInScope, want: bookID,
			},
			{
				name:    "returns a nested type of a selection by the chain of its owners",
				subject: getBookID, give: "GetBookQuery.book", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{
					Lang: graphql.Lang, Package: docsPkg, Owner: "GetBookQuery", Name: "book", Kind: symbol.KindStruct,
				},
			},
			{
				name:    "returns the stand-in of a built-in scalar",
				subject: bookID, give: graphql.StringScalar, kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: graphql.Lang, Name: graphql.StringScalar, Kind: symbol.KindAlias},
			},
			{
				name:    "returns an operation as a callable for a subject of the schema",
				subject: bookID, give: "GetBook", kind: directive.ResolveCallableInScope, want: getBookID,
			},
			{
				name:    "returns a field with arguments of the type of a method",
				subject: lookupID, give: "search", kind: directive.ResolveCallableInScope,
				want: symbol.Identity{
					Lang: graphql.Lang, Package: shopPkg, Owner: "Query", Name: "search", Kind: symbol.KindMethod,
				},
			},
			{
				name:    "returns a field of the subject",
				subject: bookID, give: "title", kind: directive.ResolveValueField, want: titleID,
			},
			{
				name:    "returns a field of the type of a field",
				subject: titleID, give: "pages", kind: directive.ResolveValueField,
				want: symbol.Identity{
					Lang: graphql.Lang, Package: shopPkg, Owner: "Book", Name: "pages", Kind: symbol.KindField,
				},
			},
			{
				name:    "returns a field of the result of an operation",
				subject: getBookID, give: "book", kind: directive.ResolveValueField,
				want: symbol.Identity{
					Lang: graphql.Lang, Package: docsPkg, Owner: "GetBookQuery", Name: "book", Kind: symbol.KindField,
				},
			},
			{
				name:    "returns a field of the type of a nullable result",
				subject: lookupID, give: "title", kind: directive.ResolveValueField, want: titleID,
			},
			{
				name:    "returns a field with arguments on the handle of a type",
				subject: queryID, give: "book", kind: directive.ResolveMemberOnHandle, want: lookupID,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				scope := rules.Scope{Subject: tt.subject}
				got, err := graphqlrules.New().Resolve(scope, tt.give, tt.kind, loaded(t).view)
				assert.NoError(t, err, "the spelling resolves")
				decl, declares := got.(node.Declaration)
				assert.True(t, declares, "the spelling resolves to a declaration")
				assert.Equal(t, decl.Identity(), tt.want, "the declaration of the spelling")
			})
		}

		params := []struct {
			name    string
			subject symbol.Identity
		}{
			{name: "returns an argument of a field with arguments", subject: lookupID},
			{name: "returns a variable of an operation", subject: getBookID},
		}
		for _, tt := range params {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := graphqlrules.New().Resolve(rules.Scope{Subject: tt.subject}, "id",
					directive.ResolveHostParam, loaded(t).view)
				assert.NoError(t, err, "the parameter resolves")
				p, is := got.(*node.Param)
				assert.True(t, is, "the spelling resolves to a parameter")
				assert.Equal(t, p.Name, "id", "the parameter of the name")
			})
		}

		failures := []struct {
			name    string
			subject symbol.Identity
			give    string
			kind    directive.ResolutionKind
			want    string
		}{
			{
				name:    "returns an error for a spelling that is not a GraphQL name",
				subject: bookID, give: "a-b", kind: directive.ResolveTypeInScope, want: "is not a GraphQL name",
			},
			{
				name:    "returns an error for a chain with an empty name",
				subject: getBookID, give: "GetBookQuery..book", kind: directive.ResolveTypeInScope,
				want: "is not a GraphQL name",
			},
			{
				name:    "returns an error for a name that no declaration has",
				subject: bookID, give: "Ghost", kind: directive.ResolveTypeInScope,
				want: "named Ghost is in the packages of the schema of",
			},
			{
				name:    "returns an error for an empty spelling",
				subject: bookID, give: "  ", kind: directive.ResolveTypeInScope, want: "nothing to resolve",
			},
			{
				name:    "returns an error for a resolution that GraphQL does not perform",
				subject: bookID, give: "title", kind: directive.ResolveMetadataKey, want: "is not a resolution",
			},
			{
				name:    "returns an error for a package variable, which GraphQL does not declare",
				subject: bookID, give: "limit", kind: directive.ResolvePackageVar, want: "no Constant or Variable",
			},
			{
				name:    "returns an error for a field that the type does not declare",
				subject: bookID, give: "ghost", kind: directive.ResolveValueField, want: "named ghost",
			},
			{
				name:    "returns an error for a parameter that the field does not declare",
				subject: lookupID,
				give:    "ghost",
				kind:    directive.ResolveHostParam,
				want:    "has no parameter named ghost",
			},
		}
		for _, tt := range failures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := graphqlrules.New().Resolve(rules.Scope{Subject: tt.subject}, tt.give, tt.kind, loaded(t).view)
				assert.HasError(t, err, "the spelling refers to nothing")
				assert.Contains(t, err.Error(), tt.want, "the error states why")
				assert.HasPrefix(t, err.Error(), string(graphql.Lang)+":", "the error is of the GraphQL rules")
			})
		}
	})
}

// TestResolveAllocs checks the ceilings of Resolve in the ordinary run,
// which does not run benchmarks.
func TestResolveAllocs(t *testing.T) {
	checkAllocs(t, resolveCalls(t))
}

// BenchmarkResolve measures a resolution in the package of the subject, and
// one that probes the packages of the schema in their order.
func BenchmarkResolve(b *testing.B) {
	benchCalls(b, resolveCalls(b))
}

// resolveCalls returns the calls of Resolve that TestResolveAllocs and
// BenchmarkResolve measure.
func resolveCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := graphqlrules.New()
	var (
		got symbol.Symbol
		err error
	)
	return []allocCall{
		{
			name: "Resolve", caseName: "a type of the package of the subject", allocs: ownTypeAllocs,
			call: func() {
				got, err = r.Resolve(rules.Scope{Subject: bookID}, "Author", directive.ResolveTypeInScope, f.view)
			},
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Author resolves")
				assert.Equal(tb, got.Kind(), symbol.KindStruct, "Resolve returns the object type")
			},
		},
		{
			name: "Resolve", caseName: "a type of the schema for a subject of the documents", allocs: schemaTypeAllocs,
			call: func() {
				got, err = r.Resolve(rules.Scope{Subject: getBookID}, "Book", directive.ResolveTypeInScope, f.view)
			},
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Book resolves")
				assert.Equal(tb, got.Kind(), symbol.KindStruct, "Resolve returns the object type")
			},
		},
	}
}
