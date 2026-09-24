// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// A declaration renders with its whole body or not at all, so the
// fixed composition order, reference resolution in the emitter's
// tree and the marker placement of pending slots are contract.
func TestBody(t *testing.T) {
	t.Parallel()

	t.Run("renders the body forms", func(t *testing.T) {
		t.Parallel()

		t.Run("a zero body renders nothing but its shape", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", emit.Body{})))
			assert.False(t, sink.Failed(), "the default form is valid")
			assert.Equal(t, string(files[0].Body), "func Handle() {\n}\n",
				"the kind template's own shape, no content")
		})

		t.Run("scaffolding composes with the slots in the fixed order", func(t *testing.T) {
			t.Parallel()

			var body emit.Body
			body.Prologue.Append(call("pro"))
			body.Declare("checks").Append(call("named"))
			body.Epilogue.Append(call("epi"))
			body.Stmts = []emit.Stmt{call("content")}
			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", body)))
			assert.False(t, sink.Failed(), "the scaffold form is valid")
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"pro", "content", "named", "epi"},
				"prologue, content, named slots, epilogue: the fixed composition")
		})

		t.Run("verbatim arrives literally", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", emit.Body{Verbatim: "\treturn nil\n"})))
			assert.False(t, sink.Failed(), "the verbatim form is valid")
			assert.Contains(t, string(files[0].Body), "\treturn nil\n",
				"byte for byte, as literal text")
		})

		t.Run("two forms is a finding and the slots still render", func(t *testing.T) {
			t.Parallel()

			var body emit.Body
			body.Prologue.Append(call("pro"))
			body.Stmts = []emit.Stmt{call("content")}
			body.Verbatim = "\tnope\n"
			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", body)))
			assert.True(t, sink.Failed(), "a two-form body is a defect")
			assert.Contains(t, string(files[0].Body), "pro",
				"the standard slots survive the refusal")
			assert.NotContains(t, string(files[0].Body), "content",
				"and no contested content is guessed at")
		})

		t.Run("a scaffold the language cannot spell reports", func(t *testing.T) {
			t.Parallel()

			body := emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtGuard, Name: "err"}}}
			_, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", body)))
			assert.True(t, sink.Failed(), "the printer's refusal is an Error")
		})
	})

	t.Run("resolves references in the emitter's tree", func(t *testing.T) {
		t.Parallel()

		trees := func(tpl string) map[plugin.ID]fs.FS {
			return map[plugin.ID]fs.FS{
				"gen": fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte(tpl)}},
			}
		}
		refBody := func(data any) emit.Body {
			var b emit.Body
			b.Prologue.Append(call("pro"))
			b.Ref = &emit.TemplateRef{Name: "method1.tpl", Data: data}
			return b
		}
		runRef := func(t *testing.T, trees map[plugin.ID]fs.FS, b emit.Body) (string, *diag.Sink) {
			t.Helper()
			pass, err := render.New("printer", language())
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			files, err := pass.Render(&plugin.RenderContext{
				Emit:  seeded(t, fn("store.go", "Handle", b)),
				Trees: trees, Sink: sink, Plugin: "printer",
			})
			assert.NoError(t, err, "the pass runs whole")
			if len(files) == 0 {
				return "", sink
			}
			return string(files[0].Body), sink
		}

		t.Run("the template drives the layout and the data is passed through", func(t *testing.T) {
			t.Parallel()

			body, sink := runRef(t,
				trees("\tref({{.Data.mark}}) for {{.Decl.Name}}\n{{slots}}"),
				refBody(map[string]any{"mark": "x"}))
			assert.False(t, sink.Failed(), "a placed marker is valid")
			assert.ContainsInOrder(t, body, []string{"ref(x) for Handle", "pro"},
				"content where the template says, then the marker's slots")
		})

		t.Run("a named marker places one slot", func(t *testing.T) {
			t.Parallel()

			b := refBody(nil)
			b.Declare("checks").Append(call("named"))
			body, sink := runRef(t,
				trees("{{slot \"checks\"}}\tmid()\n{{slots}}"), b)
			assert.False(t, sink.Failed(), "named markers are valid")
			assert.ContainsInOrder(t, body, []string{"named", "mid", "pro"},
				"the named slot arrives first, the catch-all takes the rest")
		})

		t.Run("the tree is the emitter's alone", func(t *testing.T) {
			t.Parallel()

			other := map[plugin.ID]fs.FS{
				"other": fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("{{slots}}")}},
			}
			body, sink := runRef(t, other, refBody(nil))
			assert.True(t, sink.Failed(), "another plugin's tree resolves nothing")
			assert.Contains(t, body, "pro",
				"and the slots survive as the fallback")
		})

		t.Run("a dropped marker with pending content is an Error", func(t *testing.T) {
			t.Parallel()

			body, sink := runRef(t, trees("\tbare()\n"), refBody(nil))
			assert.True(t, sink.Failed(), "the pending prologue reports")
			found := false
			for d := range sink.All() {
				if d.Code == render.DroppedSlots {
					found = true
					assert.Contains(t, d.Msg, "gen", "naming the emitter")
				}
			}
			assert.True(t, found, "under the marker rule's code")
			assert.NotContains(t, body, "pro",
				"the template owns the layout, so nothing is appended for it")
		})

		t.Run("no pending content needs no marker", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Ref = &emit.TemplateRef{Name: "method1.tpl"}
			_, sink := runRef(t, trees("\tbare()\n"), b)
			assert.False(t, sink.Failed(), "an empty slot set has nothing to drop")
		})

		t.Run("two plugins' templates of one name resolve in each plugin's tree", func(t *testing.T) {
			t.Parallel()

			unit := func(p plugin.ID, name string) plugin.Unit {
				u := unitOf(p, "store.go")
				f := &emit.Function{
					Origin: coretest.Struct(coretest.StorePath, name).ID,
					Name:   name,
				}
				f.Body = emit.Body{Ref: &emit.TemplateRef{Name: "method1.tpl"}}
				u.Decls = append(u.Decls, f)
				return u
			}
			pass, err := render.New("printer", language())
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			files, err := pass.Render(&plugin.RenderContext{
				Emit: seeded(t, unit("alpha", "First"), unit("beta", "Second")),
				Trees: map[plugin.ID]fs.FS{
					"alpha": fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("\talpha()\n{{slots}}")}},
					"beta":  fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("\tbeta()\n{{slots}}")}},
				},
				Sink: sink, Plugin: "printer",
			})
			assert.NoError(t, err, "the pass runs whole")
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "both units share one file")
			assert.Equal(t, string(files[0].Body),
				"func First() {\n\talpha()\n}\nfunc Second() {\n\tbeta()\n}\n",
				"each body renders its own emitter's template, whatever the worker rendered first")
		})
	})

	t.Run("places a body only where one is carried", func(t *testing.T) {
		t.Parallel()

		t.Run("a method's body composes like a function's", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindMethod] = "func (r) {{.Name}}() {\n{{body .}}}\n"
			files, sink := runPass(t, l, seeded(t, method("store.go", "Handle",
				emit.Body{Stmts: []emit.Stmt{call("content")}})))
			assert.False(t, sink.Failed(), "a method carries a body")
			assert.Equal(t, string(files[0].Body),
				"func (r) Handle() {\n\tcontent()\n}\n",
				"the same fixed composition, under the method's own spelling")
		})

		t.Run("a declaration carrying none refuses the template", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = "type {{.Name}} struct{}\n{{body .}}"
			files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "carries no body",
				"the refusal names what the builtin was handed")
			assert.Equal(t, string(files[0].Body), "",
				"and no fragment reaches the file the finding says was skipped")
		})
	})

	t.Run("a scaffold refusal fails the declaration wherever it sits", func(t *testing.T) {
		t.Parallel()

		placements := []struct {
			name string
			body func() emit.Body
		}{
			{
				name: "in the prologue",
				body: func() emit.Body {
					var b emit.Body
					b.Prologue.Append(refused())
					return b
				},
			},
			{
				name: "in a named slot",
				body: func() emit.Body {
					var b emit.Body
					b.Declare("checks").Append(refused())
					return b
				},
			},
			{
				name: "in the epilogue",
				body: func() emit.Body {
					var b emit.Body
					b.Epilogue.Append(refused())
					return b
				},
			},
		}
		for _, tt := range placements {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := runPass(t, language(), seeded(t,
					fn("store.go", "Handle", tt.body())))
				coretest.AssertCodes(t, sink, render.RefusedTemplate)
				assert.Equal(t, string(files[0].Body), "",
					"the refusal withholds the whole declaration, "+
						"so no half-opened shape reaches the file")
			})
		}
	})

	t.Run("a reference the tree cannot serve falls back to the slots", func(t *testing.T) {
		t.Parallel()

		withPrologue := func() emit.Body {
			b := refBody()
			b.Prologue.Append(call("pro"))
			return b
		}

		t.Run("a name the tree does not hold", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, map[plugin.ID]fs.FS{
				"gen": fstest.MapFS{"other.tpl": &fstest.MapFile{Data: []byte("{{slots}}")}},
			}, withPrologue())
			assert.Contains(t, reported(t, sink, render.UnresolvedRef), refName,
				"the finding names the template the tree does not hold")
			assert.Contains(t, body, "\tpro()\n",
				"and the slots survive the broken claim")
		})

		t.Run("a template that does not parse", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, refTree("{{"), withPrologue())
			assert.Contains(t, reported(t, sink, render.UnresolvedRef), "does not parse",
				"the finding names why nothing resolved")
			assert.Contains(t, body, "\tpro()\n",
				"and the extension points survive it")
		})

		t.Run("a template that refuses at execute time", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, refTree("{{slot \"ghost\"}}"), withPrologue())
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "ghost",
				"the finding names the slot the body never declared")
			assert.Contains(t, body, "\tpro()\n",
				"and the fallback still places the pending content")
		})
	})

	t.Run("places the pending slots a marker asks for", func(t *testing.T) {
		t.Parallel()

		t.Run("the catch-all takes the named slots too", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare("checks").Append(call("named"))
			body, sink := renderRef(t, refTree("\tmid()\n{{slots}}"), b)
			assert.False(t, sink.Failed(), "a placed marker is valid")
			assert.ContainsInOrder(t, body, []string{"mid", "named"},
				"the catch-all places every slot no named marker claimed")
		})

		t.Run("a named marker skips the slots before it", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare("first").Append(call("early"))
			b.Declare("second").Append(call("late"))
			body, sink := renderRef(t, refTree("{{slot \"second\"}}{{slots}}"), b)
			assert.False(t, sink.Failed(), "named markers are valid")
			assert.ContainsInOrder(t, body, []string{"late", "early"},
				"the named slot arrives where the template says, the rest after")
		})

		t.Run("an unplaced named slot counts against the marker rule", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare("checks").Append(call("named"))
			body, sink := renderRef(t, refTree("\tbare()\n"), b)
			assert.Contains(t, reported(t, sink, render.DroppedSlots), "1 pending",
				"the finding counts the statements no marker placed")
			assert.NotContains(t, body, "named",
				"and nothing is appended for a template that owns its layout")
		})

		markers := []struct {
			name string
			tpl  string
			body func() emit.Body
		}{
			{
				name: "the catch-all over a refused prologue",
				tpl:  "{{slots}}",
				body: func() emit.Body {
					b := refBody()
					b.Prologue.Append(refused())
					return b
				},
			},
			{
				name: "the catch-all over a refused named slot",
				tpl:  "{{slots}}",
				body: func() emit.Body {
					b := refBody()
					b.Declare("checks").Append(refused())
					return b
				},
			},
			{
				name: "the catch-all over a refused epilogue",
				tpl:  "{{slots}}",
				body: func() emit.Body {
					b := refBody()
					b.Epilogue.Append(refused())
					return b
				},
			},
			{
				name: "a named marker over a refused slot",
				tpl:  "{{slot \"checks\"}}",
				body: func() emit.Body {
					b := refBody()
					b.Declare("checks").Append(refused())
					return b
				},
			},
		}
		for _, tt := range markers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				body, sink := renderRef(t, refTree(tt.tpl), tt.body())
				assert.Contains(t, reported(t, sink, render.RefusedTemplate),
					refName, "the printer's refusal reaches the referencing template")
				assert.Equal(t, body, "",
					"the refusal withholds the whole declaration, "+
						"so no half-opened shape reaches the file")
			})
		}
	})
}
