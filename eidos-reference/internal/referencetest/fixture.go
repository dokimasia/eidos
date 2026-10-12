// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package referencetest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/rules"
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
	// runs, as the annotators of earlier turns stamp a run.
	Annotators []plugin.Annotator
	// Hosts generate in order over a fixture of their own after the
	// annotators, and the fixture seeds their units, as the generators of
	// earlier turns place their units in a plan.
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
// loads the tree through [plugintest.Loaded] with the Go and the
// TypeScript frontends and rules under [Brand], and registers the plugins
// of s, its annotators and its hosts. Then the annotators of s stamp the
// fixture, and the units of the hosts of s are seeded into it. New stops
// the test where [plugintest.Loaded] stops it, where a phase call returns
// an error, and where a phase call reports an Error.
//
// Each call loads the tree again, so a plugintest setup that calls New
// returns a fresh fixture on every call.
func New(tb assert.TB, s Spec) *Fixture {
	tb.Helper()

	f := build(tb, s)
	if len(s.Hosts) == 0 {
		return f
	}
	hosts := build(tb, s)
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
// without hosts. It seals the graph of the fixture to list the
// declarations of the packages, as the first phase call seals it.
func build(tb assert.TB, s Spec) *Fixture {
	tb.Helper()

	f := &Fixture{Fixture: plugintest.Loaded(tb, plugintest.Source{
		Tree:      os.DirFS(filepath.FromSlash(s.Dir)),
		Brand:     Brand,
		Frontends: []plugin.Frontend{gofrontend.New(nil), tsfrontend.New()},
		Rules:     []rules.SourceRules{gorules.New(), tsrules.New()},
		Plugins:   slices.Concat(s.Plugins, plugins(s.Annotators), plugins(s.Hosts)),
		Packages:  s.Packages,
	})}
	f.Target = s.Target
	f.Graph.Freeze()
	for pkg := range f.Graph.Packages() {
		f.decls = slices.AppendSeq(f.decls, node.Declarations(pkg))
	}
	for _, a := range s.Annotators {
		res := f.Annotate(tb, a)
		assert.NoError(tb, res.Err, "the annotator "+string(a.Name())+" annotates")
		clean(tb, res.Sink, "the annotator "+string(a.Name()))
	}
	return f
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
