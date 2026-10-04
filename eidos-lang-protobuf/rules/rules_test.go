// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"os"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The fixture's namespaces.
const (
	svcPkg = "svc.store"
	depPkg = "dep"
)

// rulesAllocs is one projection pass over the schema tree: 75 at the
// default benchtime. A memory profile attributes about 28 to SamplesOf,
// 27 to TypeOf, 5 to CallableOf, 4 to MembersOf and 3 to the fresh view
// and its binding, and it samples the tiny allocations of the rest only
// in part. The 2 more are for the runtime's own allocations in a run of
// one iteration: 300 fresh processes counted 0 or 1.
const rulesAllocs = 75 + 2

// The allocations of a signature's classification, a derived name and
// a resolution.
const (
	// returnRolesAllocs is the list of the returns' roles.
	returnRolesAllocs = 1
	// typeNameAllocs is a derived name: the word in PascalCase, and the
	// word joined onto the base.
	typeNameAllocs = 2
	// probeAllocs is a type resolved from inside a top-level message of a
	// two-segment package: the scope's joined name, the list of tiers, and
	// one list per tier for the message, the package, its parent and the
	// root.
	probeAllocs = 1 + 1 + 4
)

// allocCall is one call that an allocation test and a benchmark share:
// its benchmark path, its allocation ceiling, the call, and the check
// of the result the call leaves.
type allocCall struct {
	name   string
	allocs uint64
	call   func()
	check  func(tb assert.TB)
}

// fixture is the loaded tree with a tracked view over it.
type fixture struct {
	*rulestest.Fixture
	view rules.View
}

// loaded returns the fixture with a tracked view.
func loaded(tb assert.TB) *fixture {
	tb.Helper()

	_, f := setup(tb)
	return viewed(tb, f)
}

// loadedFrom loads a tree of the case's own schemas, for a case that
// needs declarations the shared fixture does not state.
func loadedFrom(tb assert.TB, files map[string]string) *fixture {
	tb.Helper()

	tree := fstest.MapFS{}
	for path, body := range files {
		tree[path] = &fstest.MapFile{Data: []byte(body)}
	}
	return viewed(tb, rulestest.Loaded(tb, protofrontend.New(), tree, protobuf.Keys))
}

// viewed returns a loaded tree with a tracked view over it.
func viewed(tb assert.TB, f *rulestest.Fixture) *fixture {
	tb.Helper()

	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	return &fixture{
		Fixture: f,
		view:    rules.View{Decls: reader, Facts: f.Facts, Reads: reads, Kernel: f.Keys},
	}
}

// bound returns protobuf's rules bound over the fixture's view.
func (f *fixture) bound() rules.Bound { return rules.NewBound(protorules.New(), f.view, nil) }

// decl looks a declaration up by identity.
func (f *fixture) decl(tb assert.TB, want symbol.Identity) symbol.Symbol {
	tb.Helper()

	sym, held := f.Graph.Lookup(want)
	assert.True(tb, held, "the fixture declares "+want.String())
	return sym
}

// rpc returns one rpc of the fixture's Store, in declaration order.
func (f *fixture) rpc(tb assert.TB, index int) *node.Method {
	tb.Helper()

	service, is := f.decl(tb, id(svcPkg, "Store", symbol.KindInterface)).(*node.Interface)
	assert.True(tb, is, "Store is a service")
	return service.Methods[index]
}

// callable returns the kernel's projection of one rpc of the fixture's
// Store.
func (f *fixture) callable(tb assert.TB, index int) rules.Callable {
	tb.Helper()

	c, is := f.bound().CallableOf(f.rpc(tb, index))
	assert.True(tb, is, "an rpc is callable")
	return c
}

// The rules are the projections a generator reads, so the member
// policy, the roles, the type-name join and the directive resolution
// are each pinned.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns rules that meet the projection contract over the schema tree", func(t *testing.T) {
			t.Parallel()

			rulestest.RunRulesSuite(t, setup)
		})

		t.Run("returns rules that project enums", func(t *testing.T) {
			t.Parallel()

			_, is := protorules.New().(rules.EnumRules)
			assert.True(t, is, "the enum capability")
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns protobuf", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Lang(), protobuf.Lang, "the language")
		})
	})

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a policy under which nothing contributes", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, protorules.New().Members().Contributes,
				"protobuf has no embedding and no supertypes, so a message's members are its own")
		})

		t.Run("returns override as the shadowing rule", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Members().Shadowing, rules.ShadowOverride, "the policy is total")
		})
	})

	t.Run("ParamRole", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies an rpc's request as input", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			assert.Equal(t, f.callable(t, 0).Params[0].Role, rules.ParamInput, "Get's one parameter")
		})
	})

	t.Run("ReturnRoles", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies a unary response as a value", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			assert.Equal(t, f.callable(t, 0).Returns[0].Role, rules.ReturnValue, "Get's one return")
		})

		t.Run("classifies a streaming response as a stream", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			assert.Equal(t, f.callable(t, 1).Returns[0].Role, rules.ReturnStream, "Watch's one return")
		})

		t.Run("returns ErrorsNone for an rpc", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			assert.Equal(t, f.callable(t, 0).Errors, rules.ErrorsNone, "a schema states no failure in its signature")
		})

		t.Run("classifies a return without a type as a value", func(t *testing.T) {
			t.Parallel()

			roles, _ := protorules.New().ReturnRoles([]*node.Return{nil, {}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue}, "nothing to read")
		})
	})

	t.Run("TypeName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			word string
			base string
			want string
		}{
			{name: "joins a word onto a Pascal base", word: "check", base: "Row", want: "CheckRow"},
			{
				name: "joins a word onto a snake-cased base in PascalCase",
				word: "check", base: "row_key", want: "CheckRowKey",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, protorules.New().TypeName(tt.word, tt.base), tt.want, "the joined name")
			})
		}
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		rowID := id(svcPkg, "Row", symbol.KindStruct)
		keyID := member(svcPkg, "Row", "Key", symbol.KindStruct)
		textID := member(svcPkg, "Row.body.text", "text", symbol.KindField)
		nameID := member(svcPkg, "Row", "name", symbol.KindField)
		resolves := []struct {
			name    string
			subject symbol.Identity
			give    string
			kind    directive.ResolutionKind
			want    symbol.Identity
		}{
			{
				name:    "returns a message nested in the subject",
				subject: rowID, give: "Key", kind: directive.ResolveTypeInScope, want: keyID,
			},
			{
				name:    "returns a sibling enum from the subject's namespace",
				subject: rowID, give: "Colour", kind: directive.ResolveTypeInScope,
				want: id(svcPkg, "Colour", symbol.KindEnum),
			},
			{
				name:    "returns a sibling enum from inside a nested message",
				subject: keyID, give: "Colour", kind: directive.ResolveTypeInScope,
				want: id(svcPkg, "Colour", symbol.KindEnum),
			},
			{
				name:    "returns a oneof by its dotted name",
				subject: rowID, give: "Row.body", kind: directive.ResolveTypeInScope,
				want: member(svcPkg, "Row", "body", symbol.KindSum),
			},
			{
				name:    "returns a nested message by its dotted name from outside its message",
				subject: id(svcPkg, "Colour", symbol.KindEnum), give: "Row.Key", kind: directive.ResolveTypeInScope,
				want: keyID,
			},
			{
				name:    "returns the message a fully-qualified name states",
				subject: rowID, give: ".dep.Target", kind: directive.ResolveTypeInScope,
				want: id(depPkg, "Target", symbol.KindStruct),
			},
			{
				name:    "returns a service as a callable scope",
				subject: rowID, give: "Store", kind: directive.ResolveCallableInScope,
				want: id(svcPkg, "Store", symbol.KindInterface),
			},
			{
				name:    "returns a declared field of the subject",
				subject: rowID, give: "name", kind: directive.ResolveValueField, want: nameID,
			},
			{
				name:    "returns a field of the subject as a member on a handle",
				subject: rowID, give: "name", kind: directive.ResolveMemberOnHandle, want: nameID,
			},
			{
				name:    "returns a oneof member as a field of the message",
				subject: rowID, give: "blob", kind: directive.ResolveValueField,
				want: member(svcPkg, "Row.body.blob", "blob", symbol.KindField),
			},
			{
				name:    "returns the message's field from a oneof member's subject",
				subject: textID, give: "name", kind: directive.ResolveValueField, want: nameID,
			},
			{
				name:    "returns a type from inside the message for a oneof member's subject",
				subject: textID, give: "Key", kind: directive.ResolveTypeInScope, want: keyID,
			},
		}
		for _, tt := range resolves {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := protorules.New().Resolve(rules.Scope{Subject: tt.subject}, tt.give, tt.kind, loaded(t).view)
				assert.NoError(t, err, "the spelling resolves")
				assert.Equal(t, identityOf(t, got), tt.want, "to the declaration")
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
				name:    "returns an error naming the outward search for a name nothing declares",
				subject: rowID, give: "Ghost", kind: directive.ResolveTypeInScope, want: "outward",
			},
			{
				name:    "returns an error for an empty spelling",
				subject: rowID, give: "  ", kind: directive.ResolveTypeInScope, want: "nothing to resolve",
			},
			{
				name:    "returns an error for a resolution kind a schema does not perform",
				subject: rowID, give: "name", kind: directive.ResolveMetadataKey, want: "not a resolution",
			},
			{
				name:    "returns an error for an rpc the service does not declare",
				subject: rowID, give: "Store.Ghost", kind: directive.ResolveCallableInScope, want: "outward",
			},
			{
				name:    "returns an error for an rpc named without its service",
				subject: rowID, give: "Get", kind: directive.ResolveCallableInScope, want: "outward",
			},
			{
				name:    "returns an error for a field the message does not declare",
				subject: rowID, give: "ghost", kind: directive.ResolveValueField, want: "no field",
			},
			{
				name:    "returns an error for a field of a file-level enum",
				subject: id(svcPkg, "Colour", symbol.KindEnum), give: "name", kind: directive.ResolveValueField,
				want: "belongs to no message",
			},
			{
				name:    "returns an error for a field of an enum variant",
				subject: member(svcPkg, "Colour", "COLOUR_RED", symbol.KindEnumVariant),
				give:    "name", kind: directive.ResolveValueField, want: "belongs to no message",
			},
			{
				name:    "returns an error for a subject the view does not contain",
				subject: id(svcPkg, "Ghost", symbol.KindStruct), give: "name", kind: directive.ResolveValueField,
				want: "does not contain",
			},
			{
				name:    "returns an error for a field whose message the view does not contain",
				subject: member(svcPkg, "Ghost", "name", symbol.KindField), give: "name",
				kind: directive.ResolveValueField, want: "does not contain",
			},
			{
				name:    "returns an error for a parameter of a message",
				subject: rowID, give: "request", kind: directive.ResolveHostParam, want: "is no rpc",
			},
			{
				name:    "returns an error for a parameter of an rpc the view does not contain",
				subject: id(svcPkg, "Ghost", symbol.KindMethod), give: "request",
				kind: directive.ResolveHostParam, want: "does not contain",
			},
		}
		for _, tt := range failures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := protorules.New().Resolve(rules.Scope{Subject: tt.subject}, tt.give, tt.kind, loaded(t).view)
				assert.HasError(t, err, "the spelling names nothing")
				assert.Contains(t, err.Error(), tt.want, "the error states why")
			})
		}

		t.Run("returns an rpc by its service's name", func(t *testing.T) {
			t.Parallel()

			got, err := protorules.New().Resolve(rules.Scope{Subject: rowID}, "Store.Get",
				directive.ResolveCallableInScope, loaded(t).view)
			assert.NoError(t, err, "an rpc resolves through its service")
			assert.Equal(t, identityOf(t, got).Name, "Get", "the rpc named")
		})

		t.Run("returns a message from an rpc's subject", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			scope := rules.Scope{Subject: f.rpc(t, 0).ID}
			got, err := protorules.New().Resolve(scope, "Row", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "an rpc resolves from its namespace")
			assert.Equal(t, identityOf(t, got), rowID, "the message")
		})

		t.Run("returns a parameter of the subject's rpc", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			scope := rules.Scope{Subject: f.rpc(t, 0).ID}
			got, err := protorules.New().Resolve(scope, "request", directive.ResolveHostParam, f.view)
			assert.NoError(t, err, "an rpc's one parameter resolves")
			assert.Equal(t, got.Kind(), symbol.KindParam, "as a parameter")
		})

		t.Run("returns an error for a parameter the rpc does not declare", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			scope := rules.Scope{Subject: f.rpc(t, 0).ID}
			_, err := protorules.New().Resolve(scope, "ghost", directive.ResolveHostParam, f.view)
			assert.HasError(t, err, "the rpc declares one parameter")
			assert.Contains(t, err.Error(), "declares no parameter", "the error states why")
		})
	})
}

// A classification, a field and a parameter allocate nothing, and the
// list of roles, a derived name and a type's resolution allocate their
// results. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestRulesAllocs(t *testing.T) {
	checkAllocs(t, rulesCalls(t))
}

// BenchmarkRules measures each classification and resolution the
// kernel asks protobuf's rules for, and drives the four hot projections
// over the schema tree under their ceiling.
func BenchmarkRules(b *testing.B) {
	benchCalls(b, rulesCalls(b))

	b.Run("New/the projections of the schema tree", func(b *testing.B) {
		rulestest.BenchRules(b, setup, rulestest.Budget{MaxAllocs: rulesAllocs})
	})
}

// rulesCalls returns a call of the constructor and of every method of
// rules.go.
func rulesCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := protorules.New()
	request := f.rpc(tb, 0).Params[0]
	rets := []*node.Return{
		{Type: ref(svcPkg, "Row", symbol.KindStruct)},
		{Type: composite("stream Row", symbol.FormStream)},
	}
	row := rules.Scope{Subject: id(svcPkg, "Row", symbol.KindStruct)}
	var (
		built  rules.SourceRules
		lang   symbol.Lang
		policy rules.MemberPolicy
		role   rules.ParamRole
		roles  []rules.ReturnRole
		name   string
		got    symbol.Symbol
		err    error
	)
	return []allocCall{
		{
			name:  "New",
			call:  func() { built = protorules.New() },
			check: func(tb assert.TB) { assert.Equal(tb, built.Lang(), protobuf.Lang, "New returns protobuf's rules") },
		},
		{
			name:  "Lang",
			call:  func() { lang = r.Lang() },
			check: func(tb assert.TB) { assert.Equal(tb, lang, protobuf.Lang, "Lang returns protobuf") },
		},
		{
			name: "Members",
			call: func() { policy = r.Members() },
			check: func(tb assert.TB) {
				assert.Equal(tb, policy.Shadowing, rules.ShadowOverride, "Members returns the override policy")
			},
		},
		{
			name:  "ParamRole",
			call:  func() { role = r.ParamRole(request, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, role, rules.ParamInput, "ParamRole classifies the request") },
		},
		{
			name: "ReturnRoles", allocs: returnRolesAllocs,
			call: func() { roles, _ = r.ReturnRoles(rets, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnStream},
					"ReturnRoles classifies the value and the stream")
			},
		},
		{
			name: "TypeName", allocs: typeNameAllocs,
			call:  func() { name = r.TypeName("check", "Row") },
			check: func(tb assert.TB) { assert.Equal(tb, name, "CheckRow", "TypeName joins the word onto the base") },
		},
		{
			name: "Resolve",
			call: func() { got, err = r.Resolve(row, "name", directive.ResolveValueField, f.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds name")
				assert.Equal(tb, got.Kind(), symbol.KindField, "Resolve returns the field")
			},
		},
		{
			name: "Resolve/a type in scope", allocs: probeAllocs,
			call: func() { got, err = r.Resolve(row, "Key", directive.ResolveTypeInScope, f.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds Key")
				assert.Equal(tb, got.Kind(), symbol.KindStruct, "Resolve returns the message")
			},
		},
	}
}

// checkAllocs checks the ceiling of every call in the ordinary run, and
// the result each call leaves.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its
// ceiling, one sub-benchmark each. Each call runs once before the
// contract starts, so what the first call initialises stays out of the
// count.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	for _, tt := range calls {
		b.Run(tt.name, func(b *testing.B) {
			tt.call()
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			for c.Loop() {
				tt.call()
			}
			tt.check(b)
		})
	}
}

// setup is the suite's entry: protobuf's rules over the loaded
// schema tree.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()
	return protorules.New(),
		rulestest.Loaded(tb, protofrontend.New(), os.DirFS("testdata/schema"), protobuf.Keys)
}

// identityOf returns the identity of a resolved declaration.
func identityOf(tb assert.TB, sym symbol.Symbol) symbol.Identity {
	tb.Helper()

	decl, is := sym.(node.Declaration)
	assert.True(tb, is, "the resolved symbol is a declaration")
	return decl.Identity()
}

// id returns a top-level identity in one namespace.
func id(pkg, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: protobuf.Lang, Package: pkg, Name: name, Kind: kind}
}

// ref returns a resolved reference to a fixture declaration.
func ref(pkg, name string, kind symbol.Kind) *node.TypeRef {
	return &node.TypeRef{Spelling: name, Target: id(pkg, name, kind)}
}

// member returns the identity of a declaration nested in another.
func member(pkg, owner, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: protobuf.Lang, Package: pkg, Owner: owner, Name: name, Kind: kind}
}

// builtin returns an unresolved named reference: a scalar, or a
// message the workspace does not declare.
func builtin(spelling string) *node.TypeRef { return &node.TypeRef{Spelling: spelling} }

// composite returns a structural reference over children.
func composite(spelling string, form symbol.TypeForm, children ...*node.TypeRef) *node.TypeRef {
	return &node.TypeRef{Spelling: spelling, Form: form, Elems: children}
}
