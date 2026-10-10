// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
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

// A language's Naming, Split and Cluster are the hooks a plan's files
// are named, split and grouped through, so how the pass applies each
// is contract.
func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Naming", func(t *testing.T) {
		t.Parallel()

		t.Run("spells each unit's filename", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			assert.Equal(t, p.FileName(unitOf(emitter, storeKey, alphaName)), storeFile,
				"the naming's spelling")
		})

		t.Run("renders one file per spelled name", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				unitOf(emitter, storeKey, alphaName),
				unitOf(emitter, userKey, betaName),
			))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, paths(files), []string{storeFile, userFile}, "one file per name, in name order")
		})

		t.Run("renders the units of one file in unit order", func(t *testing.T) {
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
			assert.Equal(t, paths(files), []string{alphaFile, betaFile},
				"one file per split unit, named from the rewritten key")
		})

		t.Run("returns the unit whole for a language without a split", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			u := unitOf(emitter, storeKey, alphaName, betaName)
			parts := p.SplitUnit(u)
			assert.Length(t, parts, 1, "one part")
			assert.Equal(t, parts[0].Decls, u.Decls, "the part is the unit")
		})

		t.Run("returns no part for a unit its split vanishes", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Split = vanishing
			p, err := render.New(passName, l)
			assert.NoError(t, err, "the language composes")
			assert.Empty(t, p.SplitUnit(unitOf(emitter, storeKey, alphaName)), "the split's result as returned")
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
					"\n"+
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
				"type "+alphaName+" struct{}\n\ntype "+betaName+" struct{}\n",
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
				string(firstGroup)+"(\n\t"+alphaName+"\n)\n\ntype "+betaName+" struct{}\n",
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

// paths returns the routed paths of rendered files, in order.
func paths(files []plugin.RenderedFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}
