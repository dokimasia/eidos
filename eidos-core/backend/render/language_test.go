// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The hooks' fixture: a struct no unit contains, the groups a
// cluster names, and the member lines every group template spells.
const (
	// outsideName names a struct a cluster claims and no unit
	// contains.
	outsideName = "Outside"
	// firstGroup and secondGroup name two declared groups, and
	// ghostGroup a group no template spells.
	firstGroup  render.GroupName = "first"
	secondGroup render.GroupName = "second"
	ghostGroup  render.GroupName = "ghost"
	// memberLines spells a cluster's members one name per line.
	memberLines = "{{range .Decls}}\t{{.Name}}\n{{end}}"
)

// grouped returns the fixture language clustering through c, with
// the block, first and second group templates declared.
func grouped(c render.Cluster) render.Language {
	l := language()
	l.Cluster = c
	l.Groups = map[render.GroupName]string{
		blockGroup:  "types (\n" + memberLines + ")\n",
		firstGroup:  string(firstGroup) + "(\n" + memberLines + ")\n",
		secondGroup: string(secondGroup) + "(\n" + memberLines + ")\n",
	}
	return l
}

// structsOnly clusters a unit's structs under the block group and
// leaves every other declaration a singleton.
func structsOnly(decls []symbol.Symbol) []render.Clustered {
	c := render.Clustered{Group: blockGroup}
	for _, d := range decls {
		if _, isStruct := d.(*emit.Struct); isStruct {
			c.Decls = append(c.Decls, d)
		}
	}
	if len(c.Decls) == 0 {
		return nil
	}
	return []render.Clustered{c}
}

// vanishing is a split that returns no unit for any input.
func vanishing(plugin.Unit) []plugin.Unit { return nil }

// vanished renders one populated unit through a language whose split
// vanishes it, under the composition's identity.
func vanished(tb assert.TB) ([]plugin.RenderedFile, *diag.Sink) {
	tb.Helper()

	l := language()
	l.Split = vanishing
	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the language composes")
	sink := diag.NewSink()
	files, err := p.Render(&plugin.RenderContext{
		Emit: seeded(tb, unitOf(emitter, storeKey, alphaName)), Sink: sink, Plugin: composedPlugin,
	})
	assert.NoError(tb, err, "the pass runs whole")
	return files, sink
}

// A language's Naming, Split and Cluster are the hooks it routes and
// groups its output through, so how the pass applies each is
// contract.
func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Naming", func(t *testing.T) {
		t.Parallel()

		t.Run("assembles one file per spelled name", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				unitOf(emitter, storeKey, alphaName),
				unitOf(emitter, userKey, betaName),
			))
			coretest.AssertCodes(t, sink)
			names := make([]string, 0, len(files))
			for _, f := range files {
				names = append(names, f.Name)
			}
			assert.Equal(t, names, []string{storeFile, userFile}, "one file per name, in name order")
		})

		t.Run("assembles the units that spell one name into one file", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, language(), seeded(t,
				unitOf(weaverPlugin, storeKey, omegaName),
				unitOf(emitter, storeKey, alphaName),
			))
			assert.Length(t, files, 1, "two plugins, one file")
			assert.ContainsInOrder(t, string(files[0].Body), []string{alphaName, omegaName},
				"contributions arrive in unit order, which is total")
		})
	})

	t.Run("Split", func(t *testing.T) {
		t.Parallel()

		t.Run("reshapes a unit before the naming spells its file", func(t *testing.T) {
			t.Parallel()

			keys := map[string]string{alphaName: alphaKey, betaName: betaKey}
			l := language()
			l.Split = func(u plugin.Unit) []plugin.Unit {
				out := make([]plugin.Unit, 0, len(u.Decls))
				for _, d := range u.Decls {
					su := u
					su.Decls = []symbol.Symbol{d}
					su.Key = keys[d.(*emit.Struct).Name]
					out = append(out, su)
				}
				return out
			}
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName, betaName)))
			coretest.AssertCodes(t, sink)
			names := make([]string, 0, len(files))
			for _, f := range files {
				names = append(names, f.Name)
			}
			assert.Equal(t, names, []string{alphaFile, betaFile},
				"one file per split unit, named from the rewritten key")
		})

		t.Run("reports RefusedTemplate for a split that returns nothing for a populated unit", func(t *testing.T) {
			t.Parallel()

			_, sink := vanished(t)
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "into nothing",
				"the vanishing reports instead of narrowing silently")
		})

		t.Run("renders no file for a unit its split vanishes", func(t *testing.T) {
			t.Parallel()

			files, _ := vanished(t)
			assert.Length(t, files, 0, "nothing routed, nothing rendered")
		})

		t.Run("positions the finding of a vanishing split at its unit's routing key", func(t *testing.T) {
			t.Parallel()

			_, sink := vanished(t)
			coretest.AssertReports(t, sink, render.RefusedTemplate)
			for d := range sink.All() {
				assert.Equal(t, d.Pos.File, storeKey, "the unit it could not route")
			}
		})

		t.Run("reports the finding of a vanishing split under the context's plugin", func(t *testing.T) {
			t.Parallel()

			_, sink := vanished(t)
			coretest.AssertReports(t, sink, render.RefusedTemplate)
			for d := range sink.All() {
				assert.Equal(t, d.Origin, composedPlugin, "the composition's identity")
			}
		})
	})

	t.Run("Cluster", func(t *testing.T) {
		t.Parallel()

		t.Run("renders a cluster at the position of its first member", func(t *testing.T) {
			t.Parallel()

			u := unitOf(emitter, storeKey, alphaName)
			u.Decls = append(u.Decls,
				fn(storeKey, handleName, emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}).Decls[0],
				unitOf(emitter, storeKey, betaName).Decls[0],
			)
			files, sink := runPass(t, grouped(structsOnly), seeded(t, u))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body),
				"types (\n\t"+alphaName+"\n\t"+betaName+"\n)\n"+
					"func "+handleName+"() {\n\treturn\n}\n",
				"the cluster at its first member's position, the singleton through its kind template")
		})

		t.Run("renders a file whose every declaration a cluster gathers", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, grouped(structsOnly), seeded(t, unitOf(emitter, storeKey, alphaName, betaName)))
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "a rendered cluster is content to stamp")
			assert.Equal(t, string(files[0].Body), "types (\n\t"+alphaName+"\n\t"+betaName+"\n)\n",
				"the cluster alone")
		})

		t.Run("reports UnknownGroup for a cluster naming a group without a template", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: ghostGroup, Decls: decls}}
			})
			_, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			coretest.AssertCodes(t, sink, render.UnknownGroup)
		})

		t.Run("renders the file without the declarations of a cluster naming an unknown group", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: ghostGroup, Decls: decls[1:]}}
			})
			files, _ := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName, betaName)))
			assert.Length(t, files, 1, "the file still renders")
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n",
				"without the skipped cluster's declarations")
		})

		t.Run("ignores a member the unit does not contain", func(t *testing.T) {
			t.Parallel()

			outside := unitOf(emitter, storeKey, outsideName).Decls[0]
			l := grouped(func([]symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: firstGroup, Decls: []symbol.Symbol{outside}}}
			})
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName, betaName)))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body),
				"type "+alphaName+" struct{}\ntype "+betaName+" struct{}\n",
				"every declaration renders as the singleton it remained")
		})

		t.Run("renders a declaration two clusters claim in the first", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{
					{Group: firstGroup, Decls: decls[:1]},
					{Group: secondGroup, Decls: decls[:1]},
				}
			})
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName, betaName)))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body),
				string(firstGroup)+"(\n\t"+alphaName+"\n)\ntype "+betaName+" struct{}\n",
				"the first cluster renders it, and the second renders nothing")
		})

		t.Run("reports RefusedTemplate for a cluster its group template fails on", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: firstGroup, Decls: decls}}
			})
			l.Groups[firstGroup] = failing
			_, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "group template",
				"the finding names the template that failed")
		})

		t.Run("renders none of the declarations of a cluster its group template fails on", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: firstGroup, Decls: decls[:1]}}
			})
			l.Groups[firstGroup] = failing
			files, _ := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName, betaName)))
			assert.Length(t, files, 1, "the file still renders")
			assert.Equal(t, string(files[0].Body), "type "+betaName+" struct{}\n",
				"the cluster's declarations render nowhere")
		})
	})
}
