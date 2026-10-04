// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The fixture's packages and files.
const (
	fxPath  = "fx"
	depPath = "fx/dep"
	fxFile  = "fx/a.go"
)

// The imports the signature cases name: the standard library's
// context package, and a workspace package whose last element is
// context too.
const (
	contextPath    = "context"
	foreignContext = "example.test/context"
)

// rulesAllocs is one projection pass over the fixture tree: 160 at the
// default benchtime. A memory profile attributes 78 to TypeOf, 41 to
// SamplesOf, 15 to MembersOf, 11 to CallableOf and 3 to the fresh view
// and its binding, and it samples the tiny allocations of the rest only
// in part. The 2 more are for the runtime's own allocations in a run of
// one iteration: 300 fresh processes counted 0 or 1.
const rulesAllocs = 160 + 2

// The allocations of a signature's classification and a derived name.
const (
	// returnRolesAllocs is the list of the returns' roles.
	returnRolesAllocs = 1
	// typeNameAllocs is a derived name: the word in Pascal case, and the
	// word joined onto the base.
	typeNameAllocs = 2
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

// fixture loads the tree and hands out a view over it.
type fixture struct {
	*rulestest.Fixture
	view   rules.View
	reader *store.Reader
	reads  *store.ReadSet
	file   *node.File
}

// loaded returns the fixture with a tracked view over it.
func loaded(tb assert.TB) *fixture {
	tb.Helper()

	_, f := setup(tb)
	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	fx := &fixture{
		Fixture: f, reader: reader, reads: reads,
		view: rules.View{Decls: reader, Facts: f.Facts, Reads: reads, Kernel: f.Keys},
	}
	pkg, held := f.Graph.PackageOf(id(fxPath, "Row", symbol.KindStruct))
	assert.True(tb, held, "the fixture package loaded")
	for _, file := range pkg.Files {
		if file.Path == fxFile {
			fx.file = file
		}
	}
	assert.NotNil(tb, fx.file, "the fixture file loaded")
	return fx
}

// bound returns the Go rules bound over the fixture's view.
func (f *fixture) bound() rules.Bound { return rules.NewBound(gorules.New(), f.view, nil) }

// decl looks a declaration up by identity.
func (f *fixture) decl(tb assert.TB, want symbol.Identity) symbol.Symbol {
	tb.Helper()

	sym, held := f.Graph.Lookup(want)
	assert.True(tb, held, "the fixture declares "+want.String())
	return sym
}

// field returns a field of a struct in the fixture package.
func (f *fixture) field(tb assert.TB, host, name string) *node.Field {
	tb.Helper()

	s, is := f.decl(tb, id(fxPath, host, symbol.KindStruct)).(*node.Struct)
	assert.True(tb, is, host+" is a struct")
	for _, candidate := range s.Fields {
		if candidate.Name == name {
			return candidate
		}
	}
	tb.Fatalf("%s declares no field %s", host, name)
	return nil
}

// scope returns a resolution scope from a fixture subject.
func (f *fixture) scope(subject symbol.Identity) rules.Scope {
	return rules.Scope{Subject: subject, File: f.file}
}

// constKey returns the handle the fixture's constant values stamp
// under.
func (f *fixture) constKey() (meta.Key[string], bool) {
	return meta.Lookup[string](f.Facts.Registry(), golang.ConstValueKey)
}

// The rules value classifies a signature and a member walk for the
// kernel. Each classification is pinned.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns rules that meet the projection contract over the fixture tree", func(t *testing.T) {
			t.Parallel()

			rulestest.RunRulesSuite(t, setup)
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns Go", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, gorules.New().Lang(), golang.Lang, "the language")
		})
	})

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a policy that walks embeds under promotion", func(t *testing.T) {
			t.Parallel()

			policy := gorules.New().Members()
			assert.Equal(t, policy.Contributes, []rules.Contribution{rules.ContributesEmbeds}, "embeds contribute")
			assert.Equal(t, policy.Shadowing, rules.ShadowPromote, "a shallower member shadows a deeper one")
			assert.Equal(t, policy.Depth, 0, "to the kernel's default depth")
			assert.True(t, policy.EmbedsAreFields, "an embedded field is a member")
		})

		t.Run("returns a policy that records an embedded field beside the members it promotes", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			derived := f.decl(t, id(fxPath, "Derived", symbol.KindStruct))
			set, is := f.bound().MembersOf(derived)
			assert.True(t, is, "a struct walks")
			var names []string
			for _, m := range set.Members {
				switch d := m.Symbol.(type) {
				case *node.Field:
					names = append(names, d.Name)
				case *node.Embed:
					names = append(names, d.ID.Name)
				}
			}
			assert.Equal(t, names, []string{"Name", "Base", "Kind"},
				"Derived's field, its embedded field Base, then the Kind that Base promotes")
		})
	})

	t.Run("ParamRole", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies a context.Context parameter as the context", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			load, is := f.decl(t, id(fxPath, "Load", symbol.KindFunction)).(*node.Function)
			assert.True(t, is, "Load is a function")
			c, _ := f.bound().CallableOf(load)
			assert.Equal(t, c.Params[0].Role, rules.ParamContext, "the context is the context")
		})

		t.Run("classifies a parameter of any other type as input", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			load, _ := f.decl(t, id(fxPath, "Load", symbol.KindFunction)).(*node.Function)
			c, _ := f.bound().CallableOf(load)
			assert.Equal(t, c.Params[1].Role, rules.ParamInput, "the id is input")
		})

		t.Run("classifies a Context through an aliased import as the context", func(t *testing.T) {
			t.Parallel()

			aliased := param(&node.TypeRef{Spelling: "ctx.Context", Package: contextPath})
			assert.Equal(t, gorules.New().ParamRole(aliased, rules.View{}), rules.ParamContext,
				"the package the import names decides, not the qualifier")
		})

		t.Run("classifies a Context another package named context declares as input", func(t *testing.T) {
			t.Parallel()

			foreign := param(&node.TypeRef{Spelling: "context.Context", Package: foreignContext})
			assert.Equal(t, gorules.New().ParamRole(foreign, rules.View{}), rules.ParamInput,
				"the qualifier matches, and the import path does not")
		})

		t.Run("classifies a parameter without a type as input", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, gorules.New().ParamRole(&node.Param{}, rules.View{}), rules.ParamInput, "nothing to read")
		})
	})

	t.Run("ReturnRoles", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies a last error return as the error", func(t *testing.T) {
			t.Parallel()

			roles, model := gorules.New().ReturnRoles(returnsOf(builtin("int"), builtin("error")), rules.View{})
			assert.Equal(t, roles[1], rules.ReturnError, "the last return")
			assert.Equal(t, model, rules.ErrorsLastReturn, "under the last-return model")
		})

		t.Run("classifies the second of two bool returns as the ok flag", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			find, _ := f.decl(t, id(fxPath, "Find", symbol.KindFunction)).(*node.Function)
			c, _ := f.bound().CallableOf(find)
			assert.Equal(t, c.Returns[1].Role, rules.ReturnOkBool, "the second of two returns, a bool, is ok")
		})

		t.Run("reports no error model for a callable without an error return", func(t *testing.T) {
			t.Parallel()

			_, model := gorules.New().ReturnRoles(returnsOf(builtin("int"), builtin("bool")), rules.View{})
			assert.Equal(t, model, rules.ErrorsNone, "no error return")
		})

		t.Run("classifies an iter.Seq return as a stream", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			all, _ := f.decl(t, id(fxPath, "All", symbol.KindFunction)).(*node.Function)
			c, _ := f.bound().CallableOf(all)
			assert.Equal(t, c.Returns[0].Role, rules.ReturnStream, "the fixture's All returns iter.Seq")
		})

		t.Run("classifies an iter.Seq2 return through an aliased import as a stream", func(t *testing.T) {
			t.Parallel()

			aliased := &node.TypeRef{Spelling: "it.Seq2", Package: golang.IterPackage}
			roles, _ := gorules.New().ReturnRoles(returnsOf(aliased), rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnStream},
				"the package the import names decides, not the qualifier")
		})

		t.Run("classifies a return without a type as a value", func(t *testing.T) {
			t.Parallel()

			roles, _ := gorules.New().ReturnRoles([]*node.Return{nil, {Type: nil}}, rules.View{})
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
			{name: "keeps an exported base exported", word: "check", base: "Row", want: "CheckRow"},
			{name: "keeps an unexported base unexported", word: "check", base: "row", want: "checkRow"},
			{name: "keeps an initialism's shape", word: "mock", base: "HTTPClient", want: "MockHTTPClient"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, gorules.New().TypeName(tt.word, tt.base), tt.want, "the joined name")
			})
		}
	})
}

// The rules value, its classifications and a derived name allocate
// only the list of roles and the name. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestRulesAllocs(t *testing.T) {
	checkAllocs(t, rulesCalls())
}

// BenchmarkRules measures each classification the kernel asks the Go
// rules for, and drives the four hot projections over the fixture tree
// under their ceiling.
func BenchmarkRules(b *testing.B) {
	benchCalls(b, rulesCalls())

	b.Run("New/the projections of the fixture tree", func(b *testing.B) {
		rulestest.BenchRules(b, setup, rulestest.Budget{MaxAllocs: rulesAllocs})
	})
}

// rulesCalls returns a call of the constructor and of every
// classification of rules.go.
func rulesCalls() []allocCall {
	r := gorules.New()
	ctx := param(&node.TypeRef{Spelling: "context.Context", Package: contextPath})
	rets := returnsOf(builtin("int"), builtin("error"))
	var (
		built  rules.SourceRules
		lang   symbol.Lang
		policy rules.MemberPolicy
		role   rules.ParamRole
		roles  []rules.ReturnRole
		name   string
	)
	return []allocCall{
		{
			name:  "New",
			call:  func() { built = gorules.New() },
			check: func(tb assert.TB) { assert.Equal(tb, built.Lang(), golang.Lang, "New returns the Go rules") },
		},
		{
			name:  "Lang",
			call:  func() { lang = r.Lang() },
			check: func(tb assert.TB) { assert.Equal(tb, lang, golang.Lang, "Lang returns Go") },
		},
		{
			name: "Members",
			call: func() { policy = r.Members() },
			check: func(tb assert.TB) {
				assert.Equal(tb, policy.Shadowing, rules.ShadowPromote, "Members returns the promotion policy")
			},
		},
		{
			name: "ParamRole",
			call: func() { role = r.ParamRole(ctx, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, role, rules.ParamContext, "ParamRole classifies the context")
			},
		},
		{
			name:   "ReturnRoles",
			allocs: returnRolesAllocs,
			call:   func() { roles, _ = r.ReturnRoles(rets, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnError},
					"ReturnRoles classifies the value and the error")
			},
		},
		{
			name:   "TypeName",
			allocs: typeNameAllocs,
			call:   func() { name = r.TypeName("check", "Row") },
			check:  func(tb assert.TB) { assert.Equal(tb, name, "CheckRow", "TypeName joins the word onto the base") },
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

// tree is the Go source the cases project: one package exercising
// every rule, and a sibling it imports.
func tree() fstest.MapFS {
	return fstest.MapFS{
		"fx/a.go": {Data: []byte(`package fx

import (
	"context"
	"iter"
	"time"

	"fx/dep"
)

// Row is a record with every builtin the table derives.
type Row struct {
	ID    int
	name  string
	Tags  []string
	Next  *Row
	When  time.Time
	Dur   time.Duration
	Dep   dep.Target
	Set   map[string]struct{}
	Ratio float64
}

// Bare has nothing a constructor could set.
type Bare struct{ hidden int }

// Base and Derived exercise promotion.
type Base struct{ Kind string }

type Derived struct {
	Base
	Name string
}

// Stamped embeds a type outside the workspace, which the member walk
// cannot read.
type Stamped struct {
	time.Time
	Label string
}

// Reader is an interface.
type Reader interface {
	Read(p []byte) (int, error)
}

// Color is an enumeration.
type Color int

const (
	Red Color = iota
	Green
	Blue
)

// Mode is an enumeration over a string.
type Mode string

const (
	ModeRead  Mode = "read"
	ModeWrite Mode = "write"
)

// Weight is a defined type over a builtin.
type Weight float64

// Plain is a transparent alias.
type Plain = int

// Box and Pair are generic.
type Box[T any] struct{ Item T }

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

type Bound[T Reader] struct{ R T }

// Number, Text and Anything are constraint interfaces, and Sized is
// generic over the three.
type Number interface{ ~int | ~float64 }

type Text interface{ ~string }

type Anything interface{}

type Sized[T Number, U Text, V Anything] struct {
	A T
	B U
	C V
}

// Tagged has a struct tag.
type Tagged struct {
	Name string ` + "`json:\"name,omitempty\" db:\"n\"`" + `
}

// Uncomparable has a slice field.
type Uncomparable struct{ Items []int }

// The callables.
func Load(ctx context.Context, id int) (*Row, error) { return nil, nil }

func Find(id int) (Row, bool) { return Row{}, false }

func All() iter.Seq[Row] { return nil }

func (r Row) Rename(name string) Row { return r }

var Registry map[string]Row

const Limit = 16

var ErrMissing = context.Canceled
`)},
		"fx/dep/d.go": {Data: []byte(`package dep

// Target is what fx refers to.
type Target struct{ V int }
`)},
	}
}

// setup is the suite's entry: the Go rules over the loaded tree.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()
	return gorules.New(), rulestest.Loaded(tb, gofrontend.New(nil), tree(), golang.Keys)
}

// id returns a top-level identity in one fixture package.
func id(path, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: path, Name: name, Kind: kind}
}

// ref returns a resolved reference to a fixture type.
func ref(path, name string, kind symbol.Kind) *node.TypeRef {
	return &node.TypeRef{Spelling: name, Target: id(path, name, kind)}
}

// builtin returns an unresolved named reference: a predeclared
// spelling bare, and a qualified one importing the package its
// qualifier names, which is the import path of every standard
// library package the cases name.
func builtin(spelling string) *node.TypeRef {
	ref := &node.TypeRef{Spelling: spelling}
	if qualifier, _, qualified := strings.Cut(spelling, "."); qualified {
		ref.Package = qualifier
	}
	return ref
}

// composite returns a structural reference over children.
func composite(spelling string, form symbol.TypeForm, children ...*node.TypeRef) *node.TypeRef {
	return &node.TypeRef{Spelling: spelling, Form: form, Elems: children}
}

// param returns a parameter typed by a reference.
func param(ref *node.TypeRef) *node.Param { return &node.Param{Type: ref} }

// returnsOf returns one return per reference, in order.
func returnsOf(refs ...*node.TypeRef) []*node.Return {
	out := make([]*node.Return, 0, len(refs))
	for _, ref := range refs {
		out = append(out, &node.Return{Type: ref})
	}
	return out
}

// stamp writes one fact at plugin authority.
func stamp(
	tb assert.TB,
	facts *meta.Facts,
	subject symbol.Identity,
	key meta.Key[string],
	value string,
) {
	tb.Helper()

	err := meta.Stamp(facts, key, value, meta.Claim{
		Subject: subject, Authority: meta.AuthorityPlugin, Plugin: "fixture",
	})
	assert.NoError(tb, err, "the fixture stamp applies")
}
