// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package sentinel_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	goannotate "go.dokimi.dev/eidos/lang/go/annotate"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/sentinel"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with the errors of the catalog, its sentinels,
// its error type with its cause, and the test function of the package.
const (
	catalogPackage = "example.com/shop/catalog"
	errNotFound    = "ErrNotFound"
	errConflict    = "ErrConflict"
	notFoundError  = "NotFoundError"
	causeField     = "Err"
	catalogTest    = "TestCatalogErrors"
)

// The tree of the errors of each shape, its package, its sentinel, its
// variable that is no sentinel, its error types, and the test function of
// the package.
const (
	errsTree    = "testdata/errs"
	errsPackage = "example.com/errs"
	errClosed   = "ErrClosed"
	timeoutName = "Timeout"
	valueError  = "ValueError"
	wrapError   = "WrapError"
	listError   = "ListError"
	boxError    = "BoxError"
	errsTest    = "TestErrsErrors"
)

// The tree of a TypeScript variable with the name of a sentinel, and its
// module.
const (
	webTree    = "testdata/script"
	webPackage = "web/errors"
)

// The prefix that a config sets, the template of the family, and the
// registrant that claims the namespace of sentinel first.
const (
	shopPrefix      = "shop"
	fileTemplate    = "sentinel.test.tmpl"
	otherRegistrant = "other"
)

// sentinel stamps the sentinels of each package, records them with the
// error types of the package into its test file, and passes the plugin
// suite.
func TestSentinel(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := sentinel.New()
				return p, fixture(tb, referencetest.Tree, catalogPackage, p).Fixture
			})
		})

		t.Run("stamps the fact on a variable whose name the rules of Go accept", func(t *testing.T) {
			t.Parallel()

			assert.True(t, stamped(t, errsTree, errsPackage, golang.Target, errClosed), "ErrClosed is a sentinel")
		})

		t.Run("stamps no fact on a variable whose name the rules of Go reject", func(t *testing.T) {
			t.Parallel()

			assert.False(t, stamped(t, errsTree, errsPackage, golang.Target, timeoutName), "Timeout is no sentinel")
		})

		t.Run("stamps no fact on a variable of a language whose rules name no error values", func(t *testing.T) {
			t.Parallel()

			assert.False(t, stamped(t, webTree, webPackage, "", errClosed), "TypeScript has no sentinels")
		})

		t.Run("records each sentinel of the package", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage, nil)
			expect.That(t, got).
				Contains(errNotFound, "the file checks ErrNotFound").
				Contains(errConflict, "the file checks ErrConflict")
			expect.False(t, got[errNotFound].Type, "ErrNotFound is a sentinel")
		})

		t.Run("records no variable that is no sentinel", func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, record(t, errsTree, errsPackage, nil), timeoutName, "the file does not check Timeout")
		})

		t.Run("records an error type with the field that its method Unwrap returns", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage, nil)[notFoundError]
			expect.True(t, got.Type, "NotFoundError is an error type")
			expect.Equal(t, got.Cause, causeField, "the cause of NotFoundError is Err")
		})

		t.Run("records an error type without a method Unwrap of one error without a cause", func(t *testing.T) {
			t.Parallel()

			got := record(t, errsTree, errsPackage, nil)
			for _, name := range []string{valueError, wrapError, listError} {
				assert.Contains(t, got, name, "the file checks "+name)
				expect.Equal(t, got[name].Cause, "", name+" has no cause")
			}
		})

		t.Run("records no generic error type", func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, record(t, errsTree, errsPackage, nil), boxError, "the file does not check BoxError")
		})

		t.Run("records the name of the test function of the package", func(t *testing.T) {
			t.Parallel()

			expect.Equal(t, record(t, referencetest.Tree, catalogPackage, nil)[errNotFound].Test, catalogTest,
				"the test function joins the name of the package catalog")
			expect.Equal(t, record(t, errsTree, errsPackage, nil)[errClosed].Test, errsTest,
				"the test function joins the name of the package errs")
		})

		t.Run("records the check of the prefix without a config", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage, nil)[errNotFound]
			expect.Equal(t, got.Prefix, "", "the prefix is the name of the package")
			expect.True(t, got.Checked, "the file checks the prefix")
		})

		t.Run("records the prefix of the option prefix", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage, &sentinel.Options{Prefix: shopPrefix})[errNotFound]
			expect.Equal(t, got.Prefix, shopPrefix, "the prefix is the option")
			expect.True(t, got.Checked, "the file checks the prefix")
		})

		t.Run("records no check of the prefix where the option prefix is off", func(t *testing.T) {
			t.Parallel()

			got := record(t, referencetest.Tree, catalogPackage, &sentinel.Options{Prefix: sentinel.PrefixOff})[errNotFound]
			assert.False(t, got.Checked, "the file does not check the prefix")
		})

		t.Run("returns from Keys the error of a namespace that another registrant claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(otherRegistrant).ClaimNamespace(sentinel.KeyValue.Namespace()),
				"another registrant claims the namespace of sentinel")
			keys, provides := sentinel.New().(plugin.KeyProvider)
			assert.True(t, provides, "sentinel registers keys")
			assert.HasError(t, keys.Keys(r.For(string(sentinel.ID))), "sentinel cannot claim its namespace")
		})
	})
}

// fixture returns a fixture over the Go package pkg of the tree dir, loaded
// under the context of tb and stamped by the annotator of the Go
// satellite, with the target golang and the plugin sentinel p.
func fixture(tb assert.TB, dir, pkg string, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(tb, referencetest.Spec{
		Dir:        dir,
		Packages:   []string{pkg},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{p},
		Annotators: []plugin.Annotator{goannotate.New()},
	})
}

// stamped runs the annotator of sentinel over the package pkg of the tree
// dir, and reports whether it stamps the variable named name.
func stamped(t *testing.T, dir, pkg string, target plugin.Target, name string) bool {
	t.Helper()

	p := sentinel.New()
	f := referencetest.New(t, referencetest.Spec{
		Dir: dir, Packages: []string{pkg}, Target: target, Plugins: []plugin.Plugin{p},
	})
	assert.NoError(t, f.Annotate(t, p).Err, "the annotator runs whole")
	key, registered := meta.Lookup[bool](f.Keys, sentinel.KeyValue)
	assert.True(t, registered, "sentinel registers its key")
	value, _ := meta.Get(f.Facts, f.Find(t, symbol.KindVariable, "", name).Identity(), key)
	return value
}

// record runs sentinel over the package pkg of the tree dir, with the
// options o where o is not nil, and returns the data of each record by the
// name of its declaration. It checks that the family claims its file
// through the template, one file for the package.
func record(t *testing.T, dir, pkg string, o *sentinel.Options) map[string]sentinel.Subject {
	t.Helper()

	p := sentinel.New()
	if o != nil {
		*p.(plugin.OptionsProvider).Options().(*sentinel.Options) = *o
	}
	f := fixture(t, dir, pkg, p)
	assert.NoError(t, f.Annotate(t, p).Err, "the annotator runs whole")
	res := f.Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	got := map[string]sentinel.Subject{}
	for u := range res.Emit.Units() {
		expect.That(t, u.Template).Equal(fileTemplate, "the family claims its file through the template")
		expect.That(t, u.Per).Equal(plugin.PerPackage, "the package has one file")
		for _, d := range u.Decls {
			origin, _ := emit.OriginOf(d)
			got[origin.Name] = u.Data[origin].(sentinel.Subject)
		}
	}
	return got
}
