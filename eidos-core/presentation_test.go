// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
)

// The presentation cases declare a plugin, the targets its
// declarations name, its helpers, the template its trees contain and
// the text each tree's template reads.
const (
	styledPlugin plugin.ID     = "styled"
	stubTarget   plugin.Target = "stub"
	otherTarget  plugin.Target = "other"
	toneHelper                 = "tone"
	markHelper                 = "mark"
	stubFile                   = "body.tmpl"
	toneInput                  = "Ab"
	pluginText                 = "plugin"
	targetText                 = "target"
)

// templatesOfAllocs is the ceiling of TemplatesOf over the files of
// embedded: the joined path, the four allocations of the Stat and the Open
// of fstest.MapFS for a directory that it synthesizes, and the tree.
const templatesOfAllocs = 6

// shout and whisper are two helpers of one shape, told apart by
// what they return for toneInput.
var (
	shout   = strings.ToUpper
	whisper = strings.ToLower
)

// errRefusedSub is the error of the Sub of [refusingSub].
var errRefusedSub = errors.New("eidos_test: the files refuse a subtree")

// refusingSub is a tree of files whose Sub refuses every directory, as an
// [fs.SubFS] may.
type refusingSub struct{ fstest.MapFS }

// Sub returns errRefusedSub for every directory.
func (refusingSub) Sub(string) (fs.FS, error) { return nil, errRefusedSub }

// A target's presentation layers over the plugin-level one, which is
// what lets one plugin serve every target and specialise a few.
func TestPresentation(t *testing.T) {
	t.Parallel()

	t.Run("Templates", func(t *testing.T) {
		t.Parallel()

		layered := func(tb assert.TB) plugin.TemplateProvider {
			tb.Helper()

			return provider(tb, eidos.NewPlugin(styledPlugin).
				Templates(stubTreeOf(pluginText)).
				For(stubTarget, eidos.Templates(stubTreeOf(targetText))))
		}

		t.Run("serves its target in place of the plugin-level tree", func(t *testing.T) {
			t.Parallel()

			tree, held := layered(t).Templates(stubTarget)
			assert.True(t, held, "the target has a tree")
			assert.Equal(t, readTree(t, tree), targetText, "the target's own tree serves it")
		})

		t.Run("leaves another target on the plugin-level tree", func(t *testing.T) {
			t.Parallel()

			tree, held := layered(t).Templates(otherTarget)
			assert.True(t, held, "the other target has a tree")
			assert.Equal(t, readTree(t, tree), pluginText, "the plugin-level tree serves it")
		})
	})

	t.Run("TemplatesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("serves the directory of its target", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).
				For(stubTarget, eidos.TemplatesOf(embedded(), stubTarget)).
				For(otherTarget, eidos.TemplatesOf(embedded(), otherTarget)))
			tree, held := tp.Templates(stubTarget)
			assert.True(t, held, "the target has a tree")
			assert.Equal(t, readTree(t, tree), targetText, "the tree is the directory of the target")
		})

		tests := []struct {
			name  string
			files fs.FS
			give  plugin.Target
			want  string
		}{
			{
				name:  "panics at the typed build where the files have no directory of its target",
				files: embedded(),
				give:  "absent",
				want:  "templates/absent",
			},
			{
				name: "panics at the typed build where the path of its target is a file",
				files: fstest.MapFS{
					path.Join(eidos.TemplatesDir, string(stubTarget)): &fstest.MapFile{Data: []byte(targetText)},
				},
				give: stubTarget,
				want: "is no directory",
			},
			{
				name:  "panics at the typed build for a target that is no name of a directory",
				files: embedded(),
				give:  "../" + stubTarget,
				want:  "no name of a directory",
			},
			{
				name:  "panics at the typed build where the files refuse the directory of its target",
				files: refusingSub{embedded()},
				give:  stubTarget,
				want:  errRefusedSub.Error(),
			},
			{
				name:  "panics at the typed build for nil files",
				files: nil,
				give:  stubTarget,
				want:  "a nil template tree",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := assert.Panics(t, func() {
					provider(t, eidos.NewPlugin(styledPlugin).For(stubTarget, eidos.TemplatesOf(tt.files, tt.give)))
				}, "the typed build refuses the declaration")
				assert.Contains(t, fmt.Sprint(got), tt.want, "the panic states the defect of the declaration")
			})
		}
	})

	t.Run("Funcs", func(t *testing.T) {
		t.Parallel()

		replaced := func(tb assert.TB) plugin.TemplateProvider {
			tb.Helper()

			return provider(tb, eidos.NewPlugin(styledPlugin).
				Funcs(template.FuncMap{toneHelper: shout}).
				For(stubTarget, eidos.Funcs(template.FuncMap{toneHelper: whisper})))
		}

		t.Run("replaces a plugin-level helper of its name in its target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, toneOf(t, replaced(t).TemplateFuncs(stubTarget)), whisper(toneInput),
				"the target's helper serves it")
		})

		t.Run("leaves another target on the plugin-level helper", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, toneOf(t, replaced(t).TemplateFuncs(otherTarget)), shout(toneInput),
				"the plugin-level helper serves it")
		})

		t.Run("adds its helpers to the plugin-level helpers", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).
				Funcs(template.FuncMap{toneHelper: shout}).
				For(stubTarget, eidos.Funcs(template.FuncMap{markHelper: whisper})))
			assert.Length(t, tp.TemplateFuncs(stubTarget), 2, "both levels' helpers are returned")
		})
	})

	t.Run("Overrides", func(t *testing.T) {
		t.Parallel()

		t.Run("names the replaced helpers of its target in order", func(t *testing.T) {
			t.Parallel()

			names := []string{"h", "g", "f", "e", "d", "c", "b", "a"}
			overrides := template.FuncMap{}
			for _, name := range names {
				overrides[name] = shout
			}
			// A map's iteration starts at a random slot, so twenty
			// builds over eight names expose an unsorted read of them
			// with near certainty.
			for range 20 {
				tp := provider(t, eidos.NewPlugin(styledPlugin).For(stubTarget, eidos.Overrides(overrides)))
				assert.Equal(t, tp.Overrides(stubTarget), slices.Sorted(slices.Values(names)),
					"the replaced names are sorted")
			}
		})

		t.Run("adds its replacements to its target's helpers", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).
				Funcs(template.FuncMap{toneHelper: whisper}).
				For(stubTarget, eidos.Overrides(template.FuncMap{toneHelper: shout})))
			assert.Equal(t, toneOf(t, tp.TemplateFuncs(stubTarget)), shout(toneInput),
				"the replacement serves the target")
		})

		t.Run("names nothing for another target", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).
				For(stubTarget, eidos.Overrides(template.FuncMap{toneHelper: shout})))
			assert.Empty(t, tp.Overrides(otherTarget), "the other target replaces nothing")
		})
	})
}

// Each option constructor returns its option by value without
// allocating, in the ordinary run, which runs no benchmark.
func TestPresentationAllocs(t *testing.T) {
	tree, fm := stubTree(), template.FuncMap{toneHelper: shout}
	var opt eidos.TargetOption
	assert.MaxAllocs(t, func() { opt = eidos.Templates(tree) }, 0, "Templates allocates nothing")
	assert.MaxAllocs(t, func() { opt = eidos.Funcs(fm) }, 0, "Funcs allocates nothing")
	assert.MaxAllocs(t, func() { opt = eidos.Overrides(fm) }, 0, "Overrides allocates nothing")
	tp := provider(t, eidos.NewPlugin(styledPlugin).For(stubTarget, opt))
	assert.Equal(t, tp.Overrides(stubTarget), []string{toneHelper}, "Overrides returns the option")
	files := embedded()
	assert.MaxAllocs(t, func() { opt = eidos.TemplatesOf(files, stubTarget) }, templatesOfAllocs,
		"TemplatesOf allocates the path, the information of the directory and the tree")
	served, held := provider(t, eidos.NewPlugin(styledPlugin).For(stubTarget, opt)).Templates(stubTarget)
	assert.True(t, held, "TemplatesOf declares the tree of the target")
	assert.Equal(t, readTree(t, served), targetText, "TemplatesOf returns the directory of the target")
}

// BenchmarkPresentation measures each option constructor, which a
// plugin's constructor calls once per declaration.
func BenchmarkPresentation(b *testing.B) {
	tree, fm, files := stubTree(), template.FuncMap{toneHelper: shout}, embedded()
	options := []struct {
		name   string
		build  func() eidos.TargetOption
		allocs uint64
	}{
		{name: "Templates", build: func() eidos.TargetOption { return eidos.Templates(tree) }},
		{name: "TemplatesOf", build: func() eidos.TargetOption {
			return eidos.TemplatesOf(files, stubTarget)
		}, allocs: templatesOfAllocs},
		{name: "Funcs", build: func() eidos.TargetOption { return eidos.Funcs(fm) }},
		{name: "Overrides", build: func() eidos.TargetOption { return eidos.Overrides(fm) }},
	}
	for _, tt := range options {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			var opt eidos.TargetOption
			for c.Loop() {
				opt = tt.build()
			}
			assert.NotNil(b, provider(b, eidos.NewPlugin(styledPlugin).For(stubTarget, opt)),
				"the option declares a presentation the plugin builds with")
		})
	}
}

// stubTreeOf returns a one-template tree whose template reads text.
func stubTreeOf(text string) fstest.MapFS {
	return fstest.MapFS{stubFile: &fstest.MapFile{Data: []byte(text)}}
}

// embedded returns the files of a plugin in the layout that TemplatesOf
// reads: a directory below the templates directory for each target, whose
// template reads the text of its tree.
func embedded() fstest.MapFS {
	return fstest.MapFS{
		path.Join(eidos.TemplatesDir, string(stubTarget), stubFile):  &fstest.MapFile{Data: []byte(targetText)},
		path.Join(eidos.TemplatesDir, string(otherTarget), stubFile): &fstest.MapFile{Data: []byte(pluginText)},
	}
}

// stubTree returns a one-template tree for template declarations.
func stubTree() fstest.MapFS { return stubTreeOf(pluginText) }

// provider builds the plugin a declaration describes and returns its
// template surface.
func provider(tb assert.TB, b *eidos.Builder) plugin.TemplateProvider {
	tb.Helper()

	tp, held := b.Handle(emitNothing()).Generator().(plugin.TemplateProvider)
	assert.True(tb, held, "the built value returns the template surface")
	return tp
}

// readTree returns the fixture template's text from a tree.
func readTree(tb assert.TB, tree fs.FS) string {
	tb.Helper()

	src, err := fs.ReadFile(tree, stubFile)
	assert.NoError(tb, err, "the tree is readable")
	return string(src)
}

// toneOf calls the tone helper of fm on toneInput.
func toneOf(tb assert.TB, fm template.FuncMap) string {
	tb.Helper()

	tone, held := fm[toneHelper].(func(string) string)
	assert.True(tb, held, "the tone helper is a string function")
	return tone(toneInput)
}
