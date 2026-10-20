// Copyright Dokimasia B.V. 2026
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

// The overloads that the order cases declare: the name of the function,
// the discriminators of its two signatures and of its implementation, and
// the name of a declaration that no declaration of a file declares.
const (
	findName     = "find"
	byNumber     = "number"
	byText       = "text"
	byEither     = "either"
	externalName = "External"
)

// The spelling of a function of the order cases: its name, the
// discriminator of its origin, and a semicolon for a signature or an empty
// body for an implementation.
const overloadTpl = "func {{.Name}}({{.Origin.Disc}}){{if .Overload}};{{else}} {}{{end}}\n"

// TestOrder checks the order of the declarations of a file. A language
// that states what a module evaluates writes each declaration after the
// declarations of the file that it evaluates, and a language that is
// implemented writes the signatures of a function before its one
// implementation.
func TestOrder(t *testing.T) {
	t.Parallel()

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      render.Language
			giveDecls []symbol.Symbol
			want      string
		}{
			{
				name:      "keeps the order of the flush for a language that states neither",
				give:      language(),
				giveDecls: []symbol.Symbol{class(betaName, alphaName), class(alphaName)},
				want:      "type " + betaName + " struct{}\n\ntype " + alphaName + " struct{}\n",
			},
			{
				name:      "writes a struct after a struct that it evaluates",
				give:      evaluating(),
				giveDecls: []symbol.Symbol{class(betaName, alphaName), class(alphaName)},
				want:      "type " + alphaName + " struct{}\n\ntype " + betaName + " struct{}\n",
			},
			{
				name:      "writes the declaration at the lowest position of the flush first",
				give:      evaluating(),
				giveDecls: []symbol.Symbol{class(gammaName), class(betaName, alphaName), class(alphaName)},
				want: "type " + gammaName + " struct{}\n\ntype " + alphaName + " struct{}\n\n" +
					"type " + betaName + " struct{}\n",
			},
			{
				name:      "writes a struct before a later struct once its base is written",
				give:      evaluating(),
				giveDecls: []symbol.Symbol{class(betaName, alphaName), class(alphaName), class(gammaName)},
				want: "type " + alphaName + " struct{}\n\ntype " + betaName + " struct{}\n\n" +
					"type " + gammaName + " struct{}\n",
			},
			{
				name:      "keeps the order of the flush for a cycle",
				give:      evaluating(),
				giveDecls: []symbol.Symbol{class(alphaName, betaName), class(betaName, alphaName)},
				want:      "type " + alphaName + " struct{}\n\ntype " + betaName + " struct{}\n",
			},
			{
				name:      "ignores a name that no declaration of the file declares",
				give:      evaluating(),
				giveDecls: []symbol.Symbol{class(betaName, externalName), class(alphaName)},
				want:      "type " + betaName + " struct{}\n\ntype " + alphaName + " struct{}\n",
			},
			{
				name:      "ignores the name of the declaration itself",
				give:      evaluating(),
				giveDecls: []symbol.Symbol{class(betaName, betaName), class(alphaName)},
				want:      "type " + betaName + " struct{}\n\ntype " + alphaName + " struct{}\n",
			},
			{
				name: "writes the overload signatures of a function before its implementation",
				give: implemented(),
				giveDecls: []symbol.Symbol{
					overload(byEither, false), overload(byNumber, true), overload(byText, true),
				},
				want: "func find(number);\n\nfunc find(text);\n\nfunc find(either) {}\n",
			},
			{
				name: "writes the functions of one name at the position of the first",
				give: implemented(),
				giveDecls: []symbol.Symbol{
					overload(byNumber, true), class(alphaName), overload(byEither, false),
				},
				want: "func find(number);\n\nfunc find(either) {}\n\ntype " + alphaName + " struct{}\n",
			},
			{
				name: "writes the functions of one name after a struct that one of them evaluates",
				give: implementedEvaluating(),
				giveDecls: []symbol.Symbol{
					overload(byNumber, true), overload(byEither, false), class(alphaName),
				},
				want: "type " + alphaName + " struct{}\n\nfunc find(number);\n\nfunc find(either) {}\n",
			},
			{
				name:      "keeps two functions of one name in a language that is not implemented",
				give:      evaluating(),
				giveDecls: []symbol.Symbol{overload(byNumber, false), overload(byText, false)},
				want:      "func find(number) {}\n\nfunc find(text) {}\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := runPass(t, tt.give, seeded(t, declared(tt.giveDecls...)))
				coretest.AssertCodes(t, sink)
				assert.Length(t, files, 1, "the declarations render into one file")
				assert.Equal(t, string(files[0].Body), tt.want, "the declarations in the order of the file")
			})
		}

		skips := []struct {
			name      string
			giveDecls []symbol.Symbol
			want      string
		}{
			{
				name:      "skips the overload signatures of a function without an implementation",
				giveDecls: []symbol.Symbol{overload(byNumber, true), class(alphaName), overload(byText, true)},
				want:      "type " + alphaName + " struct{}\n",
			},
			{
				name: "skips the second implementation of a function",
				giveDecls: []symbol.Symbol{
					overload(byNumber, true), overload(byEither, false), overload(byText, false),
				},
				want: "func find(number);\n\nfunc find(either) {}\n",
			},
		}
		for _, tt := range skips {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := runPass(t, implemented(), seeded(t, declared(tt.giveDecls...)))
				coretest.AssertCodes(t, sink, render.UnimplementedOverload)
				assert.Length(t, files, 1, "the rest of the file renders")
				assert.Equal(t, string(files[0].Body), tt.want, "the file without the skipped functions")
			})
		}

		t.Run("reports UnimplementedOverload for each implementation after the first", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, implemented(), seeded(t, declared(
				overload(byEither, false), overload(byNumber, false), overload(byText, false),
			)))
			coretest.AssertCodes(t, sink, render.UnimplementedOverload, render.UnimplementedOverload)
		})

		t.Run("orders a file without the names that a skipped signature evaluates", func(t *testing.T) {
			t.Parallel()

			l := implemented()
			l.Evaluated = func(d symbol.Symbol, yield func(name string)) {
				extending(d, yield)
				if f, is := d.(*emit.Function); is && f.Overload {
					yield(alphaName)
				}
			}
			files, sink := runPass(t, l, seeded(t, declared(
				class(betaName, findName), overload(byNumber, true), class(alphaName),
			)))
			coretest.AssertCodes(t, sink, render.UnimplementedOverload)
			assert.Equal(t, string(files[0].Body), "type "+betaName+" struct{}\n\ntype "+alphaName+" struct{}\n",
				"the skipped signature orders no struct after the struct that it evaluates")
		})

		t.Run("withholds a file whose every declaration the order skips", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, implemented(), seeded(t, declared(overload(byNumber, true))))
			assert.Empty(t, files, "the file has no declaration left to write")
		})

		t.Run("orders the declarations of two units of one file", func(t *testing.T) {
			t.Parallel()

			first := declared(class(betaName, alphaName))
			second := declared(class(alphaName))
			second.Plugin = weaverPlugin
			files, sink := runPass(t, evaluating(), seeded(t, first, second))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n\ntype "+betaName+" struct{}\n",
				"the struct of the second unit comes first")
		})

		t.Run("names the declarations in the order of the file in the import set", func(t *testing.T) {
			t.Parallel()

			l := ordering()
			l.Evaluated = extending
			files, sink := runPass(t, l, seeded(t, declared(class(betaName, alphaName), class(alphaName))))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body),
				"type "+alphaName+" true/true\n\ntype "+betaName+" false/true\n",
				"a declaration that renders later is pending")
		})

		t.Run("renders a cluster at its first member in the order of the file", func(t *testing.T) {
			t.Parallel()

			l := evaluating()
			l.Cluster = func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: blockGroup, Decls: decls[:1]}}
			}
			l.Groups = map[render.GroupName]string{blockGroup: "block\n"}
			files, sink := runPass(t, l, seeded(t, declared(class(betaName, alphaName), class(alphaName))))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n\nblock\n",
				"the cluster of the struct that evaluates the other renders after it")
		})

		t.Run("skips the header of a file in the order", func(t *testing.T) {
			t.Parallel()

			store := seeded(t, headed(declared(class(betaName, alphaName), class(alphaName))))
			files, sink := runPass(t, evaluating(), store)
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n\ntype "+betaName+" struct{}\n",
				"the header renders no declaration")
		})
	})
}

// evaluating returns the fixture language, which evaluates the structs
// that a struct extends.
func evaluating() render.Language {
	l := language()
	l.Kinds[symbol.KindFunction] = overloadTpl
	l.Evaluated = extending
	return l
}

// implemented returns the fixture language, which writes one
// implementation after the overload signatures of a function.
func implemented() render.Language {
	l := language()
	l.Kinds[symbol.KindFunction] = overloadTpl
	l.Implemented = true
	return l
}

// implementedEvaluating returns the language of [implemented], which also
// evaluates the struct alphaName for the implementation of findName.
func implementedEvaluating() render.Language {
	l := implemented()
	l.Evaluated = func(d symbol.Symbol, yield func(name string)) {
		if f, is := d.(*emit.Function); is && !f.Overload {
			yield(alphaName)
		}
	}
	return l
}

// extending yields the spelling of each struct that the struct d extends,
// as a language yields the bases of a class.
func extending(d symbol.Symbol, yield func(name string)) {
	s, is := d.(*emit.Struct)
	if !is {
		return
	}
	for _, base := range s.Extends {
		yield(base.Spelling)
	}
}

// class returns a struct named name that extends each of bases.
func class(name string, bases ...string) *emit.Struct {
	s := &emit.Struct{Origin: coretest.Struct(coretest.StorePath, name).ID, Name: name}
	for _, base := range bases {
		s.Extends = append(s.Extends, &emit.TypeRef{Spelling: base})
	}
	return s
}

// overload returns the function findName whose origin has the
// discriminator disc, marked as an overload signature where signature is
// set.
func overload(disc string, signature bool) *emit.Function {
	origin := coretest.ID(coretest.StorePath, findName, symbol.KindFunction)
	origin.Disc = disc
	return &emit.Function{Origin: origin, Name: findName, Overload: signature}
}

// declared returns a unit of the emitter at storeKey of the declarations.
func declared(decls ...symbol.Symbol) plugin.Unit {
	u := unitOf(emitter, storeKey)
	u.Decls = decls
	return u
}
