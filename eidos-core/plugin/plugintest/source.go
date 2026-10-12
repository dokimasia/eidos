// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest

import (
	"fmt"
	"io/fs"
	"slices"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Source is a source tree that [Loaded] loads into a fixture.
type Source struct {
	// Tree is the workspace tree, and Stores are the trees that its
	// dependency units read, by name.
	Tree   fs.FS
	Stores map[string]fs.FS
	// Brand is the brand whose carriers the tree writes.
	Brand output.Brand
	// Frontends load the tree, and Rules are the rules of their languages,
	// which validation and the phase calls bind.
	Frontends []plugin.Frontend
	Rules     []rules.SourceRules
	// Plugins are the plugins whose keys the fixture registers and whose
	// directives it validates.
	Plugins []plugin.Plugin
	// Packages are the paths of the packages that the fixture contains, and
	// nil contains every package of the load.
	Packages []string
}

// Loaded loads the source into a fixture, as a workspace prepares its first
// phase call. It loads the tree under the context of tb, and adds the
// packages of the source to the graph of the fixture. It registers the
// kernel's keys, the keys of each frontend under its language and the keys
// of each plugin under its name, and seals the registry. It registers the
// kernel's schemas and the schemas of each plugin. It files the
// classification stamps of each declaration of the packages under
// [meta.LoadClaim]. It validates the directives of each declaration whose
// schema it registered, and binds each reference through the rules of the
// language of the subject in the scope of the file of the subject.
//
// The registry is sealed, so a test that needs the keys of another plugin
// lists the plugin in Plugins instead of calling [Key] afterwards.
//
// Loaded stops the test where the load returns an error, where a
// registration, a stamp or a schema fails, and where the load does not
// declare a package that Packages names. It fails the test once for each
// Error of the load or the validation.
func Loaded(tb assert.TB, s Source) *Fixture {
	tb.Helper()

	sink := diag.NewSink()
	g, _, err := load.Load(assert.Context(tb), load.Config{
		FS: s.Tree, Stores: s.Stores, Frontends: s.Frontends, Sink: sink, Brand: s.Brand,
	})
	assert.NoError(tb, err, "the source tree loads")
	reportedClean(tb, sink, "the load of the source tree")

	f := New(tb)
	f.Rules = rules.NewRegistry()
	assert.Total(tb, f.Rules.Register, s.Rules, "the rules of each language of the source register")
	for _, front := range s.Frontends {
		if kp, provides := front.(plugin.KeyProvider); provides {
			assert.NoError(tb, kp.Keys(f.Keys.For(string(front.Lang()))),
				"the keys of the frontend "+string(front.Name())+" register")
		}
	}
	schemas := directive.NewRegistry()
	assert.Total(tb, schemas.Register, directive.Kernel(), "the schemas of the kernel register")
	for _, p := range s.Plugins {
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

	reader, err := g.Reader(store.NewReadSet(), nil)
	assert.NoError(tb, err, "the graph of the source tree opens a reader")
	view := rules.NewView(reader, f.Facts, nil, f.Kernel)
	found := map[string]bool{}
	for pkg := range g.Packages() {
		if s.Packages != nil && !slices.Contains(s.Packages, pkg.ID.Package) {
			continue
		}
		found[pkg.ID.Package] = true
		f.Load(tb, pkg)
		for d := range node.Declarations(pkg) {
			id := d.Identity()
			for i, st := range g.StampsOf(id) {
				assert.NoError(tb, f.Facts.StampRaw(st, meta.LoadClaim(id, i, st)),
					"the stamp "+string(st.Key)+" on "+id.String()+" applies")
			}
			var raws []directive.Raw
			for _, r := range g.DirectivesOf(id) {
				if _, claimed := schemas.ResolveName(r.Name); claimed {
					raws = append(raws, r)
				}
			}
			typed := directive.Validate(id, raws, schemas, f.Keys, scopedResolver(tb, f.Rules, view, pkg, d), sink)
			if len(typed) > 0 {
				f.Validated(tb, id, typed...)
			}
		}
	}
	reportedClean(tb, sink, "the validation of the directives")
	for _, path := range s.Packages {
		assert.Contains(tb, found, path, "the load declares the package "+path)
	}
	return f
}

// scopedResolver returns the resolver through which validation binds the
// reference params of decl, a declaration of pkg. The resolver binds a
// name through the rules of the language of the subject, in the scope of
// the file of decl and over view, as a run binds it. It returns an error
// for a language without rules and for a name that the rules do not
// resolve. It stops the test where pkg has no file at the position of
// decl, and where a name resolves to a symbol without an identity.
func scopedResolver(
	tb assert.TB, rs *rules.Registry, view rules.View, pkg *node.Package, decl node.Declaration,
) directive.Resolver {
	return func(subject symbol.Identity, name string, kind directive.ResolutionKind) (symbol.Identity, error) {
		src, held := rs.For(subject.Lang)
		if !held {
			return symbol.Identity{}, fmt.Errorf("plugintest: no rules are registered for %s", subject.Lang)
		}
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

// reportedClean fails the test once for each Error of sink, with the code,
// the position and the message of the finding. what names the step that
// reported into sink.
func reportedClean(tb assert.TB, sink *diag.Sink, what string) {
	tb.Helper()

	for d := range sink.All() {
		expect.NotEqual(tb, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s reports no Error, and reports %s at %s: %s", what, d.Code, d.Pos, d.Msg))
	}
}
