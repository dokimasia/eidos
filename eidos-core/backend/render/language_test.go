// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// A language's Naming, Split and Cluster are the hooks it routes and
// groups its output through, so how the pass applies each is
// contract.
func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("groups units into files through the naming", func(t *testing.T) {
		t.Parallel()

		files, sink := runPass(t, language(), seeded(t,
			unitOf("gen", "store.go", "Alpha"),
			unitOf("gen", "user.go", "Beta"),
		))
		assert.False(t, sink.Failed(), "nothing to report")
		assert.Length(t, files, 2, "one file per name")
		assert.Equal(t, files[0].Name, "store_stub.txt",
			"files come back in name order")
		assert.Equal(t, files[1].Name, "user_stub.txt", "both spelled by the target")
	})

	t.Run("units sharing a name assemble one file", func(t *testing.T) {
		t.Parallel()

		files, _ := runPass(t, language(), seeded(t,
			unitOf("weaver", "store.go", "Omega"),
			unitOf("gen", "store.go", "Alpha"),
		))
		assert.Length(t, files, 1, "two plugins, one file")
		body := string(files[0].Body)
		assert.ContainsInOrder(t, body, []string{"Alpha", "Omega"},
			"contributions arrive in unit order, which is total")
	})

	t.Run("splits units before naming", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Split = func(u plugin.Unit) []plugin.Unit {
			out := make([]plugin.Unit, 0, len(u.Decls))
			for _, d := range u.Decls {
				su := u
				su.Decls = []symbol.Symbol{d}
				su.Key = strings.ToLower(d.(*emit.Struct).Name) + ".go"
				out = append(out, su)
			}
			return out
		}
		files, sink := runPass(t, l, seeded(t,
			unitOf("gen", "store.go", "Alpha", "Beta"),
		))
		assert.False(t, sink.Failed(), "a split render reports nothing")
		names := make([]string, 0, len(files))
		for _, f := range files {
			names = append(names, f.Name)
		}
		assert.Equal(t, names, []string{"alpha_stub.txt", "beta_stub.txt"},
			"one file per split unit, named from the rewritten key")
	})

	t.Run("reports a split that vanishes a populated unit", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Split = func(plugin.Unit) []plugin.Unit { return nil }
		files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
		assert.Length(t, files, 0, "nothing routed, nothing rendered")
		assert.Contains(t, reported(t, sink, render.RefusedTemplate), "into nothing",
			"the vanishing reports instead of narrowing silently")
	})

	t.Run("positions a vanishing split at its unit under the context's identity", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Split = func(plugin.Unit) []plugin.Unit { return nil }
		p, err := render.New("printer", l)
		assert.NoError(t, err, "the language composes")
		sink := diag.NewSink()
		_, err = p.Render(&plugin.RenderContext{
			Emit: seeded(t, unitOf("gen", "store.go", "Alpha")), Sink: sink, Plugin: "composed",
		})
		assert.NoError(t, err, "the pass runs whole")
		coretest.AssertPositioned(t, sink)
		for d := range sink.All() {
			assert.Equal(t, d.Origin, diag.Origin("composed"),
				"the finding reports under the composition's identity")
			assert.Equal(t, d.Pos.File, "store.go", "at the unit it could not route")
		}
	})

	t.Run("clusters declarations under a group template", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Cluster = func(decls []symbol.Symbol) []render.Clustered {
			c := render.Clustered{Group: "block"}
			for _, d := range decls {
				if _, held := d.(*emit.Struct); held {
					c.Decls = append(c.Decls, d)
				}
			}
			if len(c.Decls) == 0 {
				return nil
			}
			return []render.Clustered{c}
		}
		l.Groups = map[render.GroupName]string{
			"block": "types (\n{{range .Decls}}\t{{.Name}}\n{{end}})\n",
		}

		u := unitOf("gen", "store.go", "Alpha")
		load := &emit.Function{
			Origin: coretest.Struct(coretest.StorePath, "Load").ID,
			Name:   "Load",
		}
		load.Body = emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}
		beta := &emit.Struct{
			Origin: coretest.Struct(coretest.StorePath, "Beta").ID,
			Name:   "Beta",
		}
		u.Decls = append(u.Decls, load, beta)

		files, sink := runPass(t, l, seeded(t, u))
		assert.False(t, sink.Failed(), "a clustered render reports nothing")
		assert.Equal(t, len(files), 1, "one file")
		assert.Equal(t, string(files[0].Body),
			"types (\n\tAlpha\n\tBeta\n)\nfunc Load() {\n\treturn\n}\n",
			"the cluster renders at its first member's position, "+
				"the singleton through its kind template")
	})

	t.Run("a cluster naming no declared group is reported", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Cluster = func(decls []symbol.Symbol) []render.Clustered {
			return []render.Clustered{{Group: "ghost", Decls: decls}}
		}
		l.Groups = map[render.GroupName]string{
			"block": "types (\n{{range .Decls}}\t{{.Name}}\n{{end}})\n",
		}
		files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
		var codes []diag.Code
		for d := range sink.All() {
			codes = append(codes, d.Code)
		}
		assert.Equal(t, codes, []diag.Code{render.UnknownGroup},
			"the unknown group is one finding")
		assert.Equal(t, len(files), 1, "the file still renders")
		assert.Equal(t, string(files[0].Body), "",
			"without the skipped cluster's declarations")
	})

	t.Run("a cluster without group templates refuses to compose", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Cluster = func(decls []symbol.Symbol) []render.Clustered { return nil }
		_, err := render.New("printer", l)
		assert.HasError(t, err, "clustering needs group templates")
		assert.Contains(t, err.Error(), "group templates", "naming the gap")
	})

	t.Run("clusters claim a declaration once", func(t *testing.T) {
		t.Parallel()

		grouped := func(c render.Cluster) render.Language {
			l := language()
			l.Cluster = c
			l.Groups = map[render.GroupName]string{
				"first":  "first(\n{{range .Decls}}\t{{.Name}}\n{{end}})\n",
				"second": "second(\n{{range .Decls}}\t{{.Name}}\n{{end}})\n",
			}
			return l
		}

		t.Run("a member the unit does not hold is ignored", func(t *testing.T) {
			t.Parallel()

			stranger := &emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, "Stranger").ID,
				Name:   "Stranger",
			}
			l := grouped(func([]symbol.Symbol) []render.Clustered {
				return []render.Clustered{{
					Group: "first", Decls: []symbol.Symbol{stranger},
				}}
			})
			files, sink := runPass(t, l, seeded(t,
				unitOf("gen", "store.go", "Alpha", "Beta")))
			assert.False(t, sink.Failed(), "an outside claim is ignored, not reported")
			assert.Equal(t, string(files[0].Body),
				"type Alpha struct{}\ntype Beta struct{}\n",
				"every declaration renders as the singleton it stayed")
		})

		t.Run("a declaration two clusters claim goes to the first", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{
					{Group: "first", Decls: decls[:1]},
					{Group: "second", Decls: decls[:1]},
				}
			})
			files, sink := runPass(t, l, seeded(t,
				unitOf("gen", "store.go", "Alpha", "Beta")))
			assert.False(t, sink.Failed(), "the second claim is dropped silently")
			assert.Equal(t, string(files[0].Body),
				"first(\n\tAlpha\n)\ntype Beta struct{}\n",
				"the first cluster renders it, and the second renders nothing")
		})

		t.Run("a group template refusing its cluster reports", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: "first", Decls: decls}}
			})
			l.Groups["first"] = "{{.Missing}}"
			files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "group template",
				"the finding names the template that refused")
			assert.Equal(t, string(files[0].Body), "",
				"and the cluster's declarations render nowhere")
		})
	})
}
