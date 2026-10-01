// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"io/fs"
	"strconv"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The body's fixture: the calls a slot places, the slots a body
// declares, and the second plugin and template a reference names.
const (
	// proCall, namedCall, epiCall, midCall, earlyCall, lateCall,
	// bareCall and refCall are the calls the fixture's statements and
	// templates place.
	proCall   = "pro"
	namedCall = "named"
	epiCall   = "epi"
	midCall   = "mid"
	earlyCall = "early"
	lateCall  = "late"
	bareCall  = "bare"
	refCall   = "ref"
	// checksSlot, firstSlot and secondSlot are named slots a body
	// declares, and ghostSlot one it never declares.
	checksSlot = "checks"
	firstSlot  = "first"
	secondSlot = "second"
	ghostSlot  = "ghost"
	// otherPlugin emits nothing, and its tree names the template.
	otherPlugin plugin.ID = "other"
	// otherTpl is a template the emitter's tree contains instead of
	// [refName].
	otherTpl = "other.tpl"
	// firstName and secondName name the functions two plugins emit.
	firstName  = "First"
	secondName = "Second"
	// verbatim is a verbatim body's text.
	verbatim = "\treturn nil\n"
)

// stmt is the fixture scaffold's spelling of call(name).
func stmt(name string) string { return "\t" + name + "()\n" }

// slot spells the slot marker for one named slot.
func slot(name string) string { return action(render.BuiltinSlot, strconv.Quote(name)) }

// A declaration renders with its whole body or not at all, so the
// fixed composition order, reference resolution in the emitter's
// tree and the marker placement of pending slots are contract.
func TestBody(t *testing.T) {
	t.Parallel()

	slots := action(render.BuiltinSlots)
	// withPrologue is a reference body with one prologue statement.
	withPrologue := func() emit.Body {
		b := refBody()
		b.Prologue.Append(call(proCall))
		return b
	}

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("renders a zero body as the kind template's shape alone", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t, fn(storeKey, handleName, emit.Body{})))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body), "func "+handleName+"() {\n}\n",
				"the kind template's own shape, no content")
		})

		t.Run("renders the prologue then the content then the named slots then the epilogue", func(t *testing.T) {
			t.Parallel()

			var body emit.Body
			body.Prologue.Append(call(proCall))
			body.Declare(checksSlot).Append(call(namedCall))
			body.Epilogue.Append(call(epiCall))
			body.Stmts = []emit.Stmt{call(contentCall)}
			files, sink := runPass(t, language(), seeded(t, fn(storeKey, handleName, body)))
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{proCall, contentCall, namedCall, epiCall}, "the fixed composition")
		})

		t.Run("renders a verbatim body byte for byte", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				fn(storeKey, handleName, emit.Body{Verbatim: verbatim})))
			coretest.AssertCodes(t, sink)
			assert.Contains(t, string(files[0].Body), verbatim, "the text arrives unchanged")
		})

		twoForms := func() emit.Body {
			var body emit.Body
			body.Prologue.Append(call(proCall))
			body.Stmts = []emit.Stmt{call(contentCall)}
			body.Verbatim = verbatim
			return body
		}

		t.Run("reports BodyConflict for a body of two forms", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, language(), seeded(t, fn(storeKey, handleName, twoForms())))
			coretest.AssertCodes(t, sink, render.BodyConflict)
		})

		t.Run("renders the slots of a body of two forms without its content", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, language(), seeded(t, fn(storeKey, handleName, twoForms())))
			assert.Equal(t, string(files[0].Body), "func "+handleName+"() {\n"+stmt(proCall)+"}\n",
				"the standard slots render and neither contested form does")
		})

		t.Run("reports RefusedTemplate for a statement the scaffold cannot spell", func(t *testing.T) {
			t.Parallel()

			body := emit.Body{Stmts: []emit.Stmt{refused()}}
			_, sink := runPass(t, language(), seeded(t, fn(storeKey, handleName, body)))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
		})

		t.Run("renders a reference template over the declaration and its data", func(t *testing.T) {
			t.Parallel()

			b := withPrologue()
			b.Ref.Data = map[string]any{markKey: markData}
			body, sink := renderRef(t, language(),
				refTree("\t"+refCall+"({{.Data."+markKey+"}}) for {{.Decl.Name}}\n"+slots), b)
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, body,
				[]string{refCall + "(" + markData + ") for " + handleName, proCall},
				"the content where the template places it, then the marker's slots")
		})

		t.Run("places a named slot at its marker", func(t *testing.T) {
			t.Parallel()

			b := withPrologue()
			b.Declare(checksSlot).Append(call(namedCall))
			body, sink := renderRef(t, language(),
				refTree(slot(checksSlot)+stmt(midCall)+slots), b)
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, body, []string{namedCall, midCall, proCall},
				"the named slot first, the catch-all for the rest")
		})

		t.Run("places the prologue at its standard marker", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, language(),
				refTree(stmt(midCall)+slot(render.SlotPrologue)), withPrologue())
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, body, []string{midCall, proCall},
				"the prologue renders where the template places it")
		})

		t.Run("places the epilogue at its standard marker", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Epilogue.Append(call(epiCall))
			body, sink := renderRef(t, language(), refTree(slot(render.SlotEpilogue)+stmt(midCall)), b)
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, body, []string{epiCall, midCall},
				"the epilogue renders where the template places it")
		})

		t.Run("reports nothing for a template that places every slot by name", func(t *testing.T) {
			t.Parallel()

			b := withPrologue()
			b.Declare(checksSlot).Append(call(namedCall))
			b.Epilogue.Append(call(epiCall))
			_, sink := renderRef(t, language(),
				refTree(slot(render.SlotPrologue)+slot(checksSlot)+slot(render.SlotEpilogue)), b)
			coretest.AssertCodes(t, sink)
		})

		t.Run("reports UnresolvedRef for a template only another plugin's tree contains", func(t *testing.T) {
			t.Parallel()

			other := map[plugin.ID]fs.FS{
				otherPlugin: fstest.MapFS{refName: &fstest.MapFile{Data: []byte(slots)}},
			}
			_, sink := renderRef(t, language(), other, withPrologue())
			coretest.AssertCodes(t, sink, render.UnresolvedRef)
		})

		t.Run("reports DroppedSlots naming the emitter for a template that places no marker", func(t *testing.T) {
			t.Parallel()

			_, sink := renderRef(t, language(), refTree(stmt(bareCall)), withPrologue())
			assert.Contains(t, reported(t, sink, render.DroppedSlots), string(emitter),
				"the finding names the emitter")
		})

		t.Run("appends nothing for a template that places no marker", func(t *testing.T) {
			t.Parallel()

			body, _ := renderRef(t, language(), refTree(stmt(bareCall)), withPrologue())
			assert.NotContains(t, body, proCall, "the template decides the layout")
		})

		t.Run("reports nothing for a template without a marker over a body without slots", func(t *testing.T) {
			t.Parallel()

			_, sink := renderRef(t, language(), refTree(stmt(bareCall)), refBody())
			coretest.AssertCodes(t, sink)
		})

		t.Run("resolves a template name in the tree of the plugin that emitted the body", func(t *testing.T) {
			t.Parallel()

			unit := func(p plugin.ID, name string) plugin.Unit {
				u := unitOf(p, storeKey)
				f := &emit.Function{
					Origin: coretest.Struct(coretest.StorePath, name).ID,
					Name:   name,
				}
				f.Body = refBody()
				u.Decls = append(u.Decls, f)
				return u
			}
			pass, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			e := seeded(t, unit(alphaPlugin, firstName), unit(betaPlugin, secondName))
			files, err := pass.Render(&plugin.RenderContext{
				Emit: e, Files: backendtest.Files(e, pass),
				Trees: map[plugin.ID]fs.FS{
					alphaPlugin: fstest.MapFS{refName: &fstest.MapFile{Data: []byte(stmt(earlyCall) + slots)}},
					betaPlugin:  fstest.MapFS{refName: &fstest.MapFile{Data: []byte(stmt(lateCall) + slots)}},
				},
				Sink: sink, Plugin: passName,
			})
			assert.NoError(t, err, "the pass renders every file")
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "both units share one file")
			assert.Equal(t, string(files[0].Body),
				"func "+firstName+"() {\n"+stmt(earlyCall)+"}\n"+
					"func "+secondName+"() {\n"+stmt(lateCall)+"}\n",
				"each body renders its own emitter's template")
		})

		t.Run("resolves a reference in its owner's tree", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Ref.Owner = otherPlugin
			trees := map[plugin.ID]fs.FS{
				emitter:     fstest.MapFS{refName: &fstest.MapFile{Data: []byte(stmt(earlyCall) + slots)}},
				otherPlugin: fstest.MapFS{refName: &fstest.MapFile{Data: []byte(stmt(lateCall) + slots)}},
			}
			body, sink := renderRef(t, language(), trees, b)
			coretest.AssertCodes(t, sink)
			assert.Contains(t, body, lateCall, "the owner's template renders")
		})

		t.Run("reports UnresolvedRef naming an owner without a tree", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Ref.Owner = otherPlugin
			_, sink := renderRef(t, language(), refTree(slots), b)
			assert.Contains(t, reported(t, sink, render.UnresolvedRef), string(otherPlugin),
				"the finding names the owner")
		})

		t.Run("renders a method's body in the fixed composition", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindMethod] = "func (r) {{.Name}}() {\n{{body .}}}\n"
			files, sink := runPass(t, l, seeded(t, method(storeKey, handleName,
				emit.Body{Stmts: []emit.Stmt{call(contentCall)}})))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body),
				"func (r) "+handleName+"() {\n"+stmt(contentCall)+"}\n",
				"the method's own spelling around the composition")
		})

		t.Run("reports RefusedTemplate for a body placed on a declaration without one", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = structTpl + action(render.BuiltinBody, ".")
			_, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "has no body",
				"the finding names what the builtin was passed")
		})

		t.Run("skips a declaration a body is placed on without one", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = structTpl + action(render.BuiltinBody, ".")
			files, _ := runPass(t, l, seeded(t, besideAlpha(fn(storeKey, handleName, emit.Body{}))))
			assert.Equal(t, string(files[0].Body), "func "+handleName+"() {\n}\n",
				"no fragment of the skipped declaration is rendered")
		})

		unspellable := []struct {
			name string
			give func() emit.Body
		}{
			{
				name: "skips a declaration whose prologue statement the scaffold cannot spell",
				give: func() emit.Body {
					var b emit.Body
					b.Prologue.Append(refused())
					return b
				},
			},
			{
				name: "skips a declaration whose named slot statement the scaffold cannot spell",
				give: func() emit.Body {
					var b emit.Body
					b.Declare(checksSlot).Append(refused())
					return b
				},
			},
			{
				name: "skips a declaration whose epilogue statement the scaffold cannot spell",
				give: func() emit.Body {
					var b emit.Body
					b.Epilogue.Append(refused())
					return b
				},
			},
		}
		for _, tt := range unspellable {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := runPass(t, language(), seeded(t, besideAlpha(fn(storeKey, handleName, tt.give()))))
				coretest.AssertCodes(t, sink, render.RefusedTemplate)
				assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n",
					"no half-opened shape of the skipped declaration is rendered")
			})
		}

		// Each unserved reference template is two cases: the finding
		// that names why, and the slots the body falls back to.
		unserved := []struct {
			reports string
			renders string
			trees   map[plugin.ID]fs.FS
			code    diag.Code
			want    string
		}{
			{
				reports: "reports UnresolvedRef for a reference template the tree does not contain",
				renders: "renders the slots for a reference template the tree does not contain",
				trees: map[plugin.ID]fs.FS{
					emitter: fstest.MapFS{otherTpl: &fstest.MapFile{Data: []byte(slots)}},
				},
				code: render.UnresolvedRef,
				want: refName,
			},
			{
				reports: "reports UnresolvedRef for a reference template that does not parse",
				renders: "renders the slots for a reference template that does not parse",
				trees:   refTree("{{"),
				code:    render.UnresolvedRef,
				want:    "does not parse",
			},
			{
				reports: "reports RefusedTemplate for a reference template that fails at execute time",
				renders: "renders the slots for a reference template that fails at execute time",
				trees:   refTree(slot(ghostSlot)),
				code:    render.RefusedTemplate,
				want:    ghostSlot,
			},
		}
		for _, tt := range unserved {
			t.Run(tt.reports, func(t *testing.T) {
				t.Parallel()

				_, sink := renderRef(t, language(), tt.trees, withPrologue())
				assert.Contains(t, reported(t, sink, tt.code), tt.want, "the finding names the reason")
			})

			t.Run(tt.renders, func(t *testing.T) {
				t.Parallel()

				body, _ := renderRef(t, language(), tt.trees, withPrologue())
				assert.Contains(t, body, stmt(proCall), "the slots render in the fixed composition")
			})
		}

		t.Run("places the named slots at the catch-all marker", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare(checksSlot).Append(call(namedCall))
			body, sink := renderRef(t, language(), refTree(stmt(midCall)+slots), b)
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, body, []string{midCall, namedCall},
				"the catch-all places every slot no named marker placed")
		})

		t.Run("places a named slot ahead of the slots declared before it", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare(firstSlot).Append(call(earlyCall))
			b.Declare(secondSlot).Append(call(lateCall))
			body, sink := renderRef(t, language(), refTree(slot(secondSlot)+slots), b)
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, body, []string{lateCall, earlyCall},
				"the named slot where the template places it, the rest after")
		})

		t.Run("reports DroppedSlots counting an unplaced named slot", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare(checksSlot).Append(call(namedCall))
			_, sink := renderRef(t, language(), refTree(stmt(bareCall)), b)
			assert.Contains(t, reported(t, sink, render.DroppedSlots), "1 pending",
				"the finding counts the statements no marker placed")
		})

		unplaceable := []struct {
			name string
			tpl  string
			give func() emit.Body
		}{
			{
				name: "skips a declaration whose prologue the catch-all marker cannot spell",
				tpl:  slots,
				give: func() emit.Body {
					b := refBody()
					b.Prologue.Append(refused())
					return b
				},
			},
			{
				name: "skips a declaration whose named slot the catch-all marker cannot spell",
				tpl:  slots,
				give: func() emit.Body {
					b := refBody()
					b.Declare(checksSlot).Append(refused())
					return b
				},
			},
			{
				name: "skips a declaration whose epilogue the catch-all marker cannot spell",
				tpl:  slots,
				give: func() emit.Body {
					b := refBody()
					b.Epilogue.Append(refused())
					return b
				},
			},
			{
				name: "skips a declaration whose named slot its own marker cannot spell",
				tpl:  slot(checksSlot),
				give: func() emit.Body {
					b := refBody()
					b.Declare(checksSlot).Append(refused())
					return b
				},
			},
		}
		for _, tt := range unplaceable {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				body, sink := renderRef(t, language(), refTree(tt.tpl), tt.give())
				assert.Contains(t, reported(t, sink, render.RefusedTemplate), refName,
					"the finding names the referencing template")
				assert.Equal(t, body, "", "no half-opened shape is rendered")
			})
		}

		qualified := action(qualifyHelper, strconv.Quote(storePkg), strconv.Quote(rowName))

		t.Run("renders a reference template through the vocabulary bound to the file", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, binding(), refTree("\t"+qualified+"()\n"+slots), refBody())
			coretest.AssertCodes(t, sink)
			assert.Equal(t, body,
				"use "+storePkg+" as "+storeLocal+"\n"+
					"func "+handleName+"() {\n\t"+storeLocal+"."+rowName+"()\n}\n",
				"the reference template's binding is the file's import")
		})

		t.Run("withdraws the imports of a reference template that fails", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, binding(), refTree(qualified+failing), refBody())
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
			assert.Equal(t, body, "func "+handleName+"() {\n}\n",
				"the fallback renders without the failed template's import")
		})
	})
}
