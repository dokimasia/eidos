// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing/fstest"

	"go.dokimi.dev/assert"

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

// fixture loads the tree and hands out a view over it.
type fixture struct {
	*rulestest.Fixture
	view   rules.View
	reader *store.Reader
	reads  *store.ReadSet
	file   *node.File
}

// setup is the suite's entry: the Go rules over the loaded tree.
func setup(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
	tb.Helper()
	return gorules.New(), rulestest.Loaded(tb, gofrontend.New(nil), tree(), golang.Keys)
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
	assert.True(tb, held, "the fixture holds "+want.String())
	return sym
}

// field returns a field of a struct in the fixture package.
func (f *fixture) field(tb assert.TB, host, name string) *node.Field {
	tb.Helper()

	s, is := f.decl(tb, id(fxPath, host, symbol.KindStruct)).(*node.Struct)
	assert.True(tb, is, host+" is a struct")
	for _, held := range s.Fields {
		if held.Name == name {
			return held
		}
	}
	tb.Fatalf("%s declares no field %s", host, name)
	return nil
}

// scope returns a resolution scope from a fixture subject.
func (f *fixture) scope(subject symbol.Identity) rules.Scope {
	return rules.Scope{Subject: subject, File: f.file}
}

// id returns a top-level identity in one fixture package.
func id(path, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: path, Name: name, Kind: kind}
}

// ref returns a resolved reference to a fixture type.
func ref(path, name string, kind symbol.Kind) *node.TypeRef {
	return &node.TypeRef{Spelling: name, Target: id(path, name, kind)}
}

// builtin returns an unresolved named reference.
func builtin(spelling string) *node.TypeRef { return &node.TypeRef{Spelling: spelling} }

// composite returns a structural reference over children.
func composite(spelling string, form symbol.TypeForm, children ...*node.TypeRef) *node.TypeRef {
	return &node.TypeRef{Spelling: spelling, Form: form, Elems: children}
}

// constKey returns the handle the fixture's constant values stamp
// under.
func (f *fixture) constKey() (meta.Key[string], bool) {
	return meta.Lookup[string](f.Facts.Registry(), golang.ConstValueKey)
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
