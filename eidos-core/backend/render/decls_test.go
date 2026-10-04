// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The declarations' fixture: the nested declarations a host places,
// and the call a skipped declaration records.
const (
	// innerName and deepestName name the levels a rowName host nests.
	innerName   = "Inner"
	deepestName = "Deepest"
	// phaseName names a nested enum.
	phaseName = "Phase"
	// runCall is a qualified call whose head the scaffold records.
	runCall = "pkg.Run"
)

// Declarations render in the order the flush fixed, and a
// declaration the language cannot spell whole is skipped with the
// imports it recorded, so the order, the skip and the nested
// indentation are contract.
func TestDecls(t *testing.T) {
	t.Parallel()

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("renders declarations in the unit's canonical order", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, language(), seeded(t,
				unitOf(emitter, storeKey, betaName, alphaName),
			))
			assert.ContainsInOrder(t, string(files[0].Body), []string{betaName, alphaName},
				"the flush fixed the unit's order")
		})

		t.Run("reports UnspeltKind for a declaration kind without a template", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, language(), seeded(t, unspelt()))
			coretest.AssertCodes(t, sink, render.UnspeltKind)
		})

		t.Run("renders the remaining declarations past a kind without a template", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, language(), seeded(t, unspelt()))
			assert.Length(t, files, 1, "the file renders")
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n",
				"with the spelt declaration alone")
		})

		t.Run("reports RefusedKind for a declaration of a refused kind", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, refusing(language()), seeded(t, unspelt()))
			coretest.AssertCodes(t, sink, render.RefusedKind)
		})

		t.Run("reports the reason the language states for a refused kind", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, refusing(language()), seeded(t, unspelt()))
			assert.Contains(t, reported(t, sink, render.RefusedKind), refusalReason,
				"the finding explains the skip in the language's terms")
		})

		t.Run("renders the remaining declarations past a refused kind", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, refusing(language()), seeded(t, unspelt()))
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n",
				"with the spelt declaration alone")
		})

		t.Run("withdraws the imports a skipped declaration recorded", func(t *testing.T) {
			t.Parallel()

			load := fn(storeKey, handleName, emit.Body{Stmts: []emit.Stmt{call(runCall), refused()}})
			load.Decls = append([]symbol.Symbol{&emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, alphaName).ID,
				Name:   alphaName,
			}}, load.Decls...)
			files, sink := runPass(t, language(), seeded(t, load))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n",
				"the file renders without the import the skipped declaration recorded")
		})

		t.Run("withdraws the package a skipped declaration bound", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Kinds[symbol.KindStruct] = failingOn(brokenName,
				action(qualifyHelper, strconv.Quote(storePkg), strconv.Quote(rowName)),
				qualifiedStruct)
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, brokenName, alphaName)))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
			assert.Equal(t, string(files[0].Body),
				"use "+storePkg+" as "+storeLocal+"\n"+
					"type "+alphaName+" "+storeLocal+"."+rowName+"\n",
				"the next declaration binds the package again and records its import")
		})

		t.Run("frees the name a skipped declaration bound", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Kinds[symbol.KindStruct] = failingOn(brokenName,
				action(qualifyHelper, strconv.Quote(legacyPkg), strconv.Quote(rowName)),
				qualifiedStruct)
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, brokenName, alphaName)))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
			assert.Equal(t, string(files[0].Body),
				"use "+storePkg+" as "+storeLocal+"\n"+
					"type "+alphaName+" "+storeLocal+"."+rowName+"\n",
				"the next package binds the name the skipped declaration bound")
		})

		t.Run("frees the declaration a skipped declaration imported", func(t *testing.T) {
			t.Parallel()

			// item imports rowName from pkg and writes nothing.
			item := func(pkg string) string {
				return action("$_", ":=", itemHelper, strconv.Quote(pkg), strconv.Quote(rowName))
			}
			l := binding()
			l.Kinds[symbol.KindStruct] = failingOn(brokenName, item(storePkg),
				when(alphaName, item(legacyPkg))+when(betaName, item(storePkg))+structTpl)
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, brokenName, alphaName, betaName)))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
			assert.Equal(t, string(files[0].Body),
				"use "+legacyPkg+" as "+rowName+"\n"+
					"use "+storePkg+" as "+rowName2+"\n"+
					"type "+alphaName+" struct{}\n"+
					"type "+betaName+" struct{}\n",
				"the declaration the skipped one imported binds as if it never had")
		})

		t.Run("frees the name a skipped declaration claimed for the file's own package", func(t *testing.T) {
			t.Parallel()

			// claim claims rowName of pkg and writes nothing.
			claim := func(pkg string) string {
				return action("$_", ":=", claimHelper, strconv.Quote(pkg), strconv.Quote(rowName))
			}
			l := binding()
			l.Kinds[symbol.KindStruct] = failingOn(brokenName, claim(storePkg),
				when(alphaName, claim(legacyPkg))+structTpl)
			u := unitOf(emitter, storeKey, brokenName, alphaName)
			u.Pkg = coretest.PackageID(storePkg)
			files, sink := runPass(t, l, seeded(t, u))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
			assert.Equal(t, string(files[0].Body),
				"use "+legacyPkg+" as "+rowName+"\n"+
					"type "+alphaName+" struct{}\n",
				"the import claims the name the skipped declaration claimed")
		})

		t.Run("keeps an import an earlier declaration recorded that a skipped one bound", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Kinds[symbol.KindStruct] = when(alphaName, use(storePkg, storeLocal)) +
				failingOn(brokenName,
					action(qualifyHelper, strconv.Quote(storePkg), strconv.Quote(rowName)),
					structTpl)
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName, brokenName)))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
			assert.Equal(t, string(files[0].Body),
				"use "+storePkg+" as "+storeLocal+"\n"+
					"type "+alphaName+" struct{}\n",
				"the skipped declaration withdraws nothing it did not add")
		})

		t.Run("withdraws the imports of a cluster its group template fails on", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Cluster = func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: blockGroup, Decls: decls[:1]}}
			}
			l.Groups = map[render.GroupName]string{
				blockGroup: action(qualifyHelper, strconv.Quote(storePkg), strconv.Quote(rowName)) +
					failing,
			}
			u := fn(storeKey, handleName, emit.Body{})
			u.Decls = append([]symbol.Symbol{&emit.Struct{Name: alphaName}}, u.Decls...)
			files, sink := runPass(t, l, seeded(t, u))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
			assert.Equal(t, string(files[0].Body), "func "+handleName+"() {\n}\n",
				"the file renders without the cluster's import")
		})

		reservations := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "reserves the name a struct declares",
				give: &emit.Struct{Name: storeLocal}, want: storeLocal2,
			},
			{
				name: "reserves the name an interface declares",
				give: &emit.Interface{Name: storeLocal}, want: storeLocal2,
			},
			{
				name: "reserves the name an enum declares",
				give: &emit.Enum{Name: storeLocal}, want: storeLocal2,
			},
			{
				name: "reserves the name a sum declares",
				give: &emit.Sum{Name: storeLocal}, want: storeLocal2,
			},
			{
				name: "reserves the name a function declares",
				give: &emit.Function{Name: storeLocal}, want: storeLocal2,
			},
			{
				name: "reserves the name an alias declares",
				give: &emit.Alias{Name: storeLocal}, want: storeLocal2,
			},
			{
				name: "reserves the name a constant declares",
				give: &emit.Constant{Name: storeLocal}, want: storeLocal2,
			},
			{
				name: "reserves the name a variable declares",
				give: &emit.Variable{Name: storeLocal}, want: storeLocal2,
			},
			{
				name: "reserves no name for a method",
				give: &emit.Method{Name: storeLocal}, want: storeLocal,
			},
		}
		for _, tt := range reservations {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				l := binding()
				for _, k := range []symbol.Kind{
					symbol.KindInterface, symbol.KindEnum, symbol.KindSum, symbol.KindFunction,
					symbol.KindAlias, symbol.KindConstant, symbol.KindVariable, symbol.KindMethod,
				} {
					l.Kinds[k] = "{{.Name}}\n"
				}
				l.Kinds[symbol.KindStruct] = qualifiedStruct
				u := unitOf(emitter, storeKey, alphaName)
				u.Decls = append(u.Decls, tt.give)
				files, sink := runPass(t, l, seeded(t, u))
				coretest.AssertCodes(t, sink)
				assert.HasPrefix(t, string(files[0].Body), "use "+storePkg+" as "+tt.want+"\n",
					"the import binds around the declared name")
			})
		}

		t.Run("renders nested declarations through their kind templates", func(t *testing.T) {
			t.Parallel()

			inner := &emit.Struct{Name: innerName}
			inner.Types.Append(&emit.Struct{Name: deepestName})

			files, sink := runPass(t, hosting(), seeded(t, hostOf(inner)))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body),
				"type "+rowName+" {\n"+
					"\ttype "+innerName+" {\n"+
					"\t\ttype "+deepestName+" {\n"+
					"\t\t}\n"+
					"\t}\n"+
					"}\n",
				"each level indents once more")
		})

		t.Run("reports UnspeltKind for a nested kind without a template", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, hosting(), seeded(t, hostOf(&emit.Enum{Name: phaseName})))
			coretest.AssertCodes(t, sink, render.UnspeltKind)
		})

		t.Run("renders the host without a nested kind that has no template", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, hosting(), seeded(t, hostOf(&emit.Enum{Name: phaseName})))
			assert.Equal(t, string(files[0].Body), "type "+rowName+" {\n\n}\n",
				"the member spells nothing")
		})

		t.Run("reports RefusedKind for a nested declaration of a refused kind", func(t *testing.T) {
			t.Parallel()

			l := hosting()
			l.Refused = map[symbol.Kind]string{symbol.KindEnum: refusalReason}
			_, sink := runPass(t, l, seeded(t, hostOf(&emit.Enum{Name: phaseName})))
			assert.Contains(t, reported(t, sink, render.RefusedKind), refusalReason,
				"a nested declaration reports the way a file-level one does")
		})

		t.Run("reports RefusedTemplate for a host whose member's template fails", func(t *testing.T) {
			t.Parallel()

			l := hosting()
			l.Kinds[symbol.KindEnum] = failing
			_, sink := runPass(t, l, seeded(t, hostOf(&emit.Enum{Name: phaseName})))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), missingField,
				"a host never renders around a half-spelt member")
		})

		t.Run("renders a member that spells nothing without indentation", func(t *testing.T) {
			t.Parallel()

			l := hosting()
			l.Kinds[symbol.KindEnum] = ""
			files, sink := runPass(t, l, seeded(t, hostOf(&emit.Enum{Name: phaseName})))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body), "type "+rowName+" {\n\n}\n",
				"the block adds no indentation of its own")
		})
	})
}

// hosting returns the fixture language with a struct spelling that
// places its nested types one tab deep.
func hosting() render.Language {
	l := language()
	l.Kinds[symbol.KindStruct] = "type {{.Name}} {\n" +
		"{{- range .Types.Items}}\n" + action(render.BuiltinNested, strconv.Quote("\t"), ".") +
		"{{- end}}\n}\n"
	return l
}

// hostOf returns a unit of one struct that nests inner.
func hostOf(inner symbol.Symbol) plugin.Unit {
	host := &emit.Struct{Name: rowName}
	host.Types.Append(inner)
	u := unitOf(emitter, storeKey)
	u.Decls = append(u.Decls, host)
	return u
}

// when returns a template that runs body for the declaration named
// name alone.
func when(name, body string) string {
	return "{{if eq .Name " + strconv.Quote(name) + "}}" + body + "{{end}}"
}

// failingOn returns a template that fails on the declaration named
// name, after the given prefix ran, and spells the rest.
func failingOn(name, prefix, rest string) string {
	return when(name, prefix+failing) + rest
}

// refusing returns l refusing the method kind, which [unspelt]'s
// second declaration takes.
func refusing(l render.Language) render.Language {
	l.Refused = map[symbol.Kind]string{symbol.KindMethod: refusalReason}
	return l
}
