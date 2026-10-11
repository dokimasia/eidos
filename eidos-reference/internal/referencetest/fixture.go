// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package referencetest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/frontend/load"
	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/lang/typescript"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Brand is the brand of the trees of the ensemble, so a carrier sets a
// directive as //+reference:handler.
const Brand output.Brand = "reference"

// Tree is the workspace tree of the module, relative to the directory of a
// package one level below the root of the module, where the test of a
// plugin runs.
const Tree = "../testdata/tree"

// Spec describes a fixture: the tree, the packages of the fixture, the
// target of the plan, and the plugins that the fixture registers and runs.
type Spec struct {
	// Dir is the directory of the tree, slash-separated and relative to the
	// directory of the test.
	Dir string
	// Packages are the paths of the packages of the tree that the fixture
	// contains, such as example.com/shop/orders or web/store.
	Packages []string
	// Target is the target of the plan of a generator's phase call. It is
	// zero for an annotator.
	Target plugin.Target
	// Plugins are the plugins whose keys the fixture registers and whose
	// directives it validates, the plugin under test among them.
	Plugins []plugin.Plugin
	// Annotators stamp the fixture in order before the plugin under test
	// runs, as the annotators of earlier buckets stamp a run.
	Annotators []plugin.Annotator
	// Hosts generate in order over a fixture of their own after the
	// annotators, and the fixture seeds their units, as the generators of
	// earlier buckets place their units in a plan.
	Hosts []plugin.Generator
}

// Fixture is a plugintest fixture over the packages of a tree, with the
// declarations of those packages.
//
// # Concurrency
//
// A Fixture belongs to one test, as a plugintest fixture does.
type Fixture struct {
	*plugintest.Fixture
	// decls are the declarations of the packages of the fixture, in the
	// order of the walk of each package.
	decls []node.Declaration
}

// New returns a fixture over the packages of the tree that s describes. It
// loads the tree, and registers the rules, the keys and the classification
// stamps of Go and TypeScript, the keys of the plugins of s, and the
// validated directives of every declaration of the packages. Then the
// annotators of s stamp the fixture, and the units of the hosts of s are
// seeded into it. New stops the test where the tree does not load, where a
// key, a schema, a stamp or a phase call fails, and where the load, the
// validation or a phase call reports an Error.
//
// Each call loads the tree again, so a plugintest setup that calls New
// returns a fresh fixture on every call.
func New(ctx context.Context, tb assert.TB, s Spec) *Fixture {
	tb.Helper()

	f := build(ctx, tb, s)
	if len(s.Hosts) == 0 {
		return f
	}
	hosts := build(ctx, tb, s)
	var res plugintest.Result
	for _, h := range s.Hosts {
		res = hosts.Generate(tb, h)
		assert.NoError(tb, res.Err, "the host "+string(h.Name())+" generates")
		clean(tb, res.Sink, "the host "+string(h.Name()))
	}
	// The emit store of a fixture persists across its phase calls, so the
	// store of the last call contains the units of every host.
	f.Seed(tb, slices.Collect(res.Emit.Units())...)
	return f
}

// Find returns the declaration of a kind in the packages of the fixture,
// with an owner and a name. owner is the name of the declaration that the
// declaration is a member of, and empty for a declaration at file level.
// Find stops the test where no declaration or more than one matches.
func (f *Fixture) Find(tb assert.TB, kind symbol.Kind, owner, name string) node.Declaration {
	tb.Helper()

	var found []node.Declaration
	for _, d := range f.decls {
		if id := d.Identity(); id.Kind == kind && id.Owner == owner && id.Name == name {
			found = append(found, d)
		}
	}
	assert.Length(tb, found, 1, fmt.Sprintf("the fixture declares one %s %q in %q", kind, name, owner))
	return found[0]
}

// build returns a fixture over the packages of the tree that s describes,
// stamped by the annotators of s. It is what [New] returns for a spec
// without hosts.
func build(ctx context.Context, tb assert.TB, s Spec) *Fixture {
	tb.Helper()

	g := loaded(ctx, tb, s.Dir)
	f := &Fixture{Fixture: plugintest.New(tb)}
	f.Target = s.Target
	f.Rules = rules.NewRegistry()
	assert.NoError(tb, f.Rules.Register(gorules.New()), "the Go rules register")
	assert.NoError(tb, f.Rules.Register(tsrules.New()), "the TypeScript rules register")
	assert.NoError(tb, golang.Keys(f.Keys.For(string(golang.Lang))), "the Go keys register")
	assert.NoError(tb, typescript.Keys(f.Keys.For(string(typescript.Lang))), "the TypeScript keys register")
	schemas := directive.NewRegistry()
	assert.Total(tb, schemas.Register, directive.Kernel(), "the schemas of the kernel register")
	for _, p := range slices.Concat(s.Plugins, plugins(s.Annotators), plugins(s.Hosts)) {
		if kp, provides := p.(plugin.KeyProvider); provides {
			assert.NoError(tb, kp.Keys(f.Keys.For(string(p.Name()))), "the keys of "+string(p.Name())+" register")
		}
		dp, declares := p.(plugin.DirectiveProvider)
		if !declares {
			continue
		}
		for _, d := range dp.Directives() {
			if _, registered := schemas.ResolveName(d.Canonical()); !registered {
				assert.NoError(tb, schemas.Register(d), "the schema "+string(d.Canonical())+" registers")
			}
		}
	}
	assert.Empty(tb, schemas.Seal(), "the schemas of the fixture seal")
	f.Keys.Seal()
	f.Facts = meta.NewFacts(f.Keys)

	sink := diag.NewSink()
	reader, err := g.Reader(store.NewReadSet(), nil)
	assert.NoError(tb, err, "the graph of the tree opens a reader")
	view := rules.View{Decls: reader, Facts: f.Facts, Kernel: f.Kernel}
	contained := 0
	for pkg := range g.Packages() {
		if !slices.Contains(s.Packages, pkg.ID.Package) {
			continue
		}
		contained++
		f.Load(tb, pkg)
		for d := range node.Declarations(pkg) {
			id := d.Identity()
			f.decls = append(f.decls, d)
			for i, st := range g.StampsOf(id) {
				claim := meta.Claim{
					Subject: id,
					Plugin:  st.Origin,
					Order:   meta.Order{Subject: id, Instance: i},
					Pos:     st.Pos,
				}
				assert.NoError(
					tb,
					f.Facts.StampRaw(st, claim),
					"the stamp "+string(st.Key)+" on "+id.String()+" applies",
				)
			}
			var raws []directive.Raw
			for _, r := range g.DirectivesOf(id) {
				if _, claimed := schemas.ResolveName(r.Name); claimed {
					raws = append(raws, r)
				}
			}
			resolve := resolver(tb, f.Rules, view, pkg, d)
			if typed := directive.Validate(id, raws, schemas, f.Keys, resolve, sink); len(typed) > 0 {
				f.Validated(tb, id, typed...)
			}
		}
	}
	clean(tb, sink, "the validation of the directives")
	assert.Equal(tb, contained, len(s.Packages), "the tree declares each package of the spec")
	for _, a := range s.Annotators {
		res := f.Annotate(tb, a)
		assert.NoError(tb, res.Err, "the annotator "+string(a.Name())+" annotates")
		clean(tb, res.Sink, "the annotator "+string(a.Name()))
	}
	return f
}

// loaded returns the graph of the tree at dir, loaded with the Go and the
// TypeScript frontends under [Brand]. It stops the test where the load
// returns an error or reports an Error.
func loaded(ctx context.Context, tb assert.TB, dir string) *store.Graph {
	tb.Helper()

	sink := diag.NewSink()
	g, _, err := load.Load(ctx, load.Config{
		FS:        os.DirFS(filepath.FromSlash(dir)),
		Frontends: []plugin.Frontend{gofrontend.New(nil), tsfrontend.New()},
		Sink:      sink,
		Brand:     Brand,
	})
	assert.NoError(tb, err, "the tree "+dir+" loads")
	clean(tb, sink, "the load of "+dir)
	return g
}

// resolver returns the resolver through which validation binds the
// reference params of decl, a declaration of pkg. The resolver binds a
// name through the rules of the language of the subject, in the scope of
// the file of decl and over view, as a run binds it. Validation calls it
// with decl as the subject. It returns the error of a name that the rules
// do not resolve. It stops the test where pkg has no file at the position
// of decl, and where a name resolves to a symbol without an identity.
func resolver(
	tb assert.TB, rs *rules.Registry, view rules.View, pkg *node.Package, decl node.Declaration,
) directive.Resolver {
	return func(subject symbol.Identity, name string, kind directive.ResolutionKind) (symbol.Identity, error) {
		src, _ := rs.For(subject.Lang)
		at := slices.IndexFunc(pkg.Files, func(file *node.File) bool { return file.Pos.File == decl.Position().File })
		assert.NotEqual(tb, at, -1, "the package of "+subject.String()+" has the file of the subject")
		sym, err := src.Resolve(rules.Scope{Subject: subject, File: pkg.Files[at]}, name, kind, view)
		if err != nil {
			return symbol.Identity{}, err
		}
		resolved, is := sym.(node.Declaration)
		assert.True(tb, is, fmt.Sprintf("%q resolves to a declaration", name))
		return resolved.Identity(), nil
	}
}

// clean fails the test once for each Error of sink, with the code, the
// position and the message of the finding.
func clean(tb assert.TB, sink *diag.Sink, what string) {
	tb.Helper()

	for d := range sink.All() {
		expect.NotEqual(tb, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s reports no Error, and reports %s at %s: %s", what, d.Code, d.Pos, d.Msg))
	}
}

// plugins returns the annotators or the generators ps as plugins, in order.
func plugins[P plugin.Plugin](ps []P) []plugin.Plugin {
	out := make([]plugin.Plugin, 0, len(ps))
	for _, p := range ps {
		out = append(out, p)
	}
	return out
}
