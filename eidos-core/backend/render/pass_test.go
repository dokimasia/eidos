// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The pass is the procedure every language shares: group through the
// naming, render kinds in canonical order, finalise per file and
// continue past a failure.
func TestPass(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("names every gap of an empty language at once", func(t *testing.T) {
			t.Parallel()

			_, err := render.New("printer", render.Language{})
			assert.HasError(t, err, "a language declaring nothing composes into nothing")
			for _, gap := range []string{
				"spells no kinds", "spells no filenames", "spells no scaffolding",
				"renders no import block", "holds no formatter",
			} {
				assert.Contains(t, err.Error(), gap,
					"a composition reads every fault at once, "+gap+" included")
			}
		})

		t.Run("a kind template that does not parse is a fault", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindEnum] = "{{"
			_, err := render.New("printer", l)
			assert.HasError(t, err, "an unparseable spelling never reaches a render")
			assert.Contains(t, err.Error(), symbol.KindEnum.String(),
				"and the fault names the kind it could not parse")
		})

		t.Run("a group template that does not parse is a fault", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Groups = map[render.GroupName]string{"block": "{{"}
			_, err := render.New("printer", l)
			assert.HasError(t, err, "an unparseable group spelling is the same fault")
			assert.Contains(t, err.Error(), "block",
				"and the fault names the group it could not parse")
		})

		t.Run("a file skeleton that does not parse is a fault", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = "{{"
			_, err := render.New("printer", l)
			assert.HasError(t, err, "an unparseable skeleton never assembles a file")
			assert.Contains(t, err.Error(), "file skeleton",
				"and the fault names the skeleton")
		})
	})

	t.Run("Coverage", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the language's declaration back", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Coverage = render.Coverage{Facts: map[symbol.Fact]render.Verdict{
				symbol.FactAbstract: render.Refuses,
			}}
			p, err := render.New("printer", l)
			assert.NoError(t, err, "the language composes")
			assert.Equal(t, p.Coverage().Of(symbol.KindStruct, symbol.FactAbstract),
				render.Refuses,
				"a consumer holding the pass reads the data the guard reads")
		})

		t.Run("reads undeclared where the language declared none", func(t *testing.T) {
			t.Parallel()

			p, err := render.New("printer", language())
			assert.NoError(t, err, "the language composes")
			assert.False(t, p.Coverage().Declared(),
				"an undeclared coverage leaves the guard off")
		})
	})

	t.Run("refuses a context it cannot render from", func(t *testing.T) {
		t.Parallel()

		p, err := render.New("printer", language())
		assert.NoError(t, err, "the language composes")

		_, err = p.Render(nil)
		assert.HasError(t, err, "a nil context carries no store")
		assert.Contains(t, err.Error(), "emit store", "and names what it needs")

		_, err = p.Render(&plugin.RenderContext{Sink: diag.NewSink()})
		assert.HasError(t, err, "and neither does a context holding no store")

		_, err = p.Render(&plugin.RenderContext{Emit: seeded(t)})
		assert.HasError(t, err, "a context without a sink has nowhere to report")
		assert.Contains(t, err.Error(), "sink", "and names what it needs")
	})

	t.Run("two packages spelling one filename stay two files", func(t *testing.T) {
		t.Parallel()

		left := unitOf("gen", "example.com/left/store.go", "Alpha")
		left.Pkg = coretest.Struct("example.com/left", "Anchor").ID
		right := unitOf("gen", "example.com/right/store.go", "Beta")
		right.Pkg = coretest.Struct("example.com/right", "Anchor").ID
		files, _ := runPass(t, language(), seeded(t, left, right))
		assert.Length(t, files, 2, "the owning package addresses the file")
		assert.Equal(t, files[0].Name, files[1].Name, "one spelled name")
		assert.NotEqual(t, files[0].Pkg, files[1].Pkg, "two owners, two files")
	})

	t.Run("a file carries the derivation it was assembled from", func(t *testing.T) {
		t.Parallel()

		files, sink := runPass(t, language(), seeded(t,
			unitOf("beta", "store.go", "Beta"),
			unitOf("alpha", "store.go", "Alpha"),
		))
		assert.False(t, sink.Failed(), "the fixture renders clean")
		assert.Length(t, files, 1, "two plugins sharing a filename assemble one file")
		assert.Equal(t, files[0].Plugins, []plugin.ID{"alpha", "beta"},
			"every emitter that contributed, distinct and sorted")
		assert.Equal(t, files[0].Sources, []string{"store.go"},
			"and the routing key they share, named once")
	})

	t.Run("a plan file derives from nothing", func(t *testing.T) {
		t.Parallel()

		plan := unitOf("gen", "", "Registry")
		plan.Per = plugin.PerPlan
		files, _ := runPass(t, language(), seeded(t, plan))
		assert.Length(t, files, 1, "the plan unit renders")
		assert.Equal(t, files[0].Plugins, []plugin.ID{"gen"}, "its emitter alone")
		assert.Length(t, files[0].Sources, 0,
			"and no source, because a plan file derives from no declaration")
	})

	t.Run("findings carry the context's plugin as origin", func(t *testing.T) {
		t.Parallel()

		u := unitOf("gen", "store.go", "Alpha")
		u.Decls = append(u.Decls, &emit.Method{
			Origin: coretest.Struct(coretest.StorePath, "M").ID,
			Name:   "Handle",
		})

		p, err := render.New("printer", language())
		assert.NoError(t, err, "the language composes")
		sink := diag.NewSink()
		_, err = p.Render(&plugin.RenderContext{
			Emit: seeded(t, u), Sink: sink, Plugin: "composed",
		})
		assert.NoError(t, err, "the pass runs whole")
		for d := range sink.All() {
			assert.Equal(t, d.Origin, diag.Origin("composed"),
				"the composition's identity, not the pass's own name")
		}

		sink = diag.NewSink()
		_, err = p.Render(&plugin.RenderContext{Emit: seeded(t, u), Sink: sink})
		assert.NoError(t, err, "the pass runs whole")
		for d := range sink.All() {
			assert.Equal(t, d.Origin, diag.Origin("printer"),
				"a zero context identity falls back to the pass's name")
		}
	})

	t.Run("a format failure continues with the remaining files", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Finalise = func(src []byte) ([]byte, error) {
			if strings.Contains(string(src), "Broken") {
				return nil, errors.New("unparseable")
			}
			return src, nil
		}
		files, sink := runPass(t, l, seeded(t,
			unitOf("gen", "broken.go", "Broken"),
			unitOf("gen", "store.go", "Alpha"),
		))
		assert.True(t, sink.Failed(), "the failure is an Error")
		assert.Length(t, files, 1, "the sink never receives the unformatted file")
		assert.Equal(t, files[0].Name, "store_stub.txt",
			"while its siblings render whole")
	})

	t.Run("findings arrive in file order whatever order the workers finish in", func(t *testing.T) {
		t.Parallel()

		released := make(chan struct{})
		l := language()
		l.Finalise = func(src []byte) ([]byte, error) {
			switch {
			case strings.Contains(string(src), "Alpha"):
				// The first file waits for the second to finish, so
				// with two workers the second file's finding is
				// reported first. One worker cannot run the second
				// file concurrently, so the wait ends by timeout and
				// the order is file order either way.
				select {
				case <-released:
				case <-time.After(2 * time.Second):
				}
				return nil, errors.New("alpha is unformattable")
			default:
				defer close(released)
				return nil, errors.New("beta is unformattable")
			}
		}
		_, sink := runPass(t, l, seeded(t,
			unitOf("gen", "alpha.go", "Alpha"),
			unitOf("gen", "beta.go", "Beta"),
		))
		got := []string{}
		for d := range sink.All() {
			got = append(got, d.Pos.File)
		}
		assert.Equal(t, got, []string{"alpha_stub.txt", "beta_stub.txt"},
			"the report order is the file order")
	})

	t.Run("positions findings at the package-qualified file", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Finalise = func([]byte) ([]byte, error) { return nil, errors.New("unparseable") }
		u := unitOf("gen", "example.com/left/store.go", "Alpha")
		u.Pkg = coretest.Struct("example.com/left", "Anchor").ID
		_, sink := runPass(t, l, seeded(t, u))
		for d := range sink.All() {
			assert.Equal(t, d.Pos.File, "example.com/left/store_stub.txt",
				"the position names the package the file belongs to")
		}
		coretest.AssertReports(t, sink, render.UnformattedFile)
	})

	t.Run("two runs produce the same bytes", func(t *testing.T) {
		t.Parallel()

		build := func() *plugin.Emit {
			return seeded(t,
				unitOf("weaver", "store.go", "Omega"),
				unitOf("gen", "store.go", "Alpha", "Beta"),
				unitOf("gen", "user.go", "Gamma"),
			)
		}
		first, _ := runPass(t, language(), build())
		second, _ := runPass(t, language(), build())
		assert.Equal(t, second, first, "byte identity is the contract")
	})

	t.Run("a skeleton refusing a file withholds it", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.File = "{{.Missing}}"
		files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
		assert.Contains(t, reported(t, sink, render.RefusedTemplate), "store_stub.txt",
			"the finding names the file the skeleton refused")
		assert.Length(t, files, 0, "and the sink never receives a half-assembled file")
	})
}

// BenchmarkPass measures the procedure at the canonical scale: 1000
// per-package files of 200 declarations, 200k template executions,
// through a pass-through formatter, so the number is the pass and
// the engine, not a real language's spelling.
func BenchmarkPass(b *testing.B) {
	const packages, decls = 1_000, 200
	e := plugin.NewEmit()
	for i := range packages {
		path := "example.com/pkg" + strconv.Itoa(i)
		u := unitOf("gen", path+"/store.go")
		u.Pkg = coretest.Struct(path, "Anchor").ID
		for d := range decls {
			u.Decls = append(u.Decls, &emit.Struct{
				Origin: coretest.Struct(path, "S"+strconv.Itoa(d)).ID,
				Name:   "S" + strconv.Itoa(d),
			})
		}
		if err := e.Add(u); err != nil {
			b.Fatalf("Add: unexpected error: %v", err)
		}
	}
	p, err := render.New("printer", language())
	if err != nil {
		b.Fatalf("New: unexpected error: %v", err)
	}

	c := bench.Start(b).MaxAllocs(680_000)
	defer c.End()
	for c.Loop() {
		sink := diag.NewSink()
		files, err := p.Render(&plugin.RenderContext{
			Emit: e, Sink: sink, Plugin: "printer",
		})
		if err != nil {
			b.Fatalf("Render: unexpected error: %v", err)
		}
		if len(files) != packages || sink.Failed() {
			b.Fatal("every file renders clean")
		}
	}
}

// BenchmarkPassReferences measures the body-claiming path at the
// same scale: 1000 files of 200 functions, each body a reference to
// one template in the emitting plugin's tree, so every declaration
// executes a parsed reference template.
func BenchmarkPassReferences(b *testing.B) {
	const packages, decls = 1_000, 200
	e := plugin.NewEmit()
	for i := range packages {
		path := "example.com/pkg" + strconv.Itoa(i)
		u := unitOf("gen", path+"/store.go")
		u.Pkg = coretest.Struct(path, "Anchor").ID
		for d := range decls {
			f := &emit.Function{
				Origin: coretest.Struct(path, "F"+strconv.Itoa(d)).ID,
				Name:   "F" + strconv.Itoa(d),
			}
			f.Body = refBody()
			u.Decls = append(u.Decls, f)
		}
		if err := e.Add(u); err != nil {
			b.Fatalf("Add: unexpected error: %v", err)
		}
	}
	p, err := render.New("printer", language())
	if err != nil {
		b.Fatalf("New: unexpected error: %v", err)
	}
	trees := refTree("\tref()\n{{slots}}")

	c := bench.Start(b).MaxAllocs(3_500_000)
	defer c.End()
	for c.Loop() {
		sink := diag.NewSink()
		files, err := p.Render(&plugin.RenderContext{
			Emit: e, Trees: trees, Sink: sink, Plugin: "printer",
		})
		if err != nil {
			b.Fatalf("Render: unexpected error: %v", err)
		}
		if len(files) != packages || sink.Failed() {
			b.Fatal("every file renders clean")
		}
	}
}
