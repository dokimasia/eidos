// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The shapes the matchable vocabulary leaves out, each of which the
// assignment step settles on a branch of its own: a type nested in a
// type, a method attached from outside the type it belongs to, a
// positional field, an embed and a constraint.
const (
	outerName    = "Outer"
	innerName    = "Inner"
	heldName     = "Held"
	detachedName = "Detached"
	baseSpelling = "Base"
)

// The names and type spellings the blank and holey fixtures declare
// their signature slots with.
const (
	serveName     = "Serve"
	blankName     = "_"
	keyName       = "key"
	writerType    = "Writer"
	requestType   = "Request"
	stringType    = "string"
	servedDisc    = writerType + "," + requestType
	secondByPlace = "#1"
)

// overloadTree declares one type carrying two overloads of one
// method name, which only their parameter spellings tell apart.
func overloadTree() fstest.MapFS {
	return fstest.MapFS{
		apiFile: {Data: []byte(
			"package svc/api\ntype User string\nmethod Get string int\nmethod Get\n",
		)},
	}
}

// assigned spells the identity the assignment step gives one
// declaration of a planted package: the frontend's language, not the
// language the fixture builder pre-filled.
func assigned(owner, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{
		Lang:    frontendtest.ScriptedLang,
		Package: coretest.StorePath,
		Owner:   owner,
		Name:    name,
		Kind:    kind,
	}
}

// Identities are the join everything long-lived keys on, so the
// canonical shapes the assignment step spells are pinned here.
func TestAssign(t *testing.T) {
	t.Parallel()

	t.Run("file", func(t *testing.T) {
		t.Parallel()

		t.Run("names a file by its whole workspace-relative path", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, stdTree())
			_, held := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: apiPath, Name: apiFile,
				Kind: symbol.KindFile,
			})
			assert.True(t, held,
				"two files of one base name can meet in one package, so the path names it")
		})

		t.Run("panics on a file with no path", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				_, _, _ = load.Load(context.Background(), mustConfig(
					oneFileTree(), with(&pathless{frontendtest.NewScripted()}),
				))
			}, "a file nothing can name is the frontend's defect")
			assert.Contains(t, got, "no path", "and says so")
		})
	})

	t.Run("decl", func(t *testing.T) {
		t.Parallel()

		t.Run("spells every matchable kind canonically", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, oneFileTree(), with(&everyKind{frontendtest.NewScripted()}))
			want := map[symbol.Kind]symbol.Identity{
				symbol.KindStruct:    assigned("", coretest.StructName, symbol.KindStruct),
				symbol.KindInterface: assigned("", coretest.InterfaceName, symbol.KindInterface),
				symbol.KindEnum:      assigned("", coretest.EnumName, symbol.KindEnum),
				symbol.KindSum:       assigned("", coretest.SumName, symbol.KindSum),
				symbol.KindAlias:     assigned("", coretest.AliasName, symbol.KindAlias),
				symbol.KindFunction:  assigned("", coretest.FunctionName, symbol.KindFunction),
				symbol.KindVariable:  assigned("", coretest.VariableName, symbol.KindVariable),
				symbol.KindConstant:  assigned("", coretest.ConstantName, symbol.KindConstant),
				symbol.KindField: assigned(
					coretest.StructName, coretest.FieldName, symbol.KindField,
				),
				symbol.KindMethod: assigned(
					coretest.StructName, coretest.MethodName, symbol.KindMethod,
				),
				symbol.KindParam: assigned(
					coretest.FunctionName, coretest.ParamName, symbol.KindParam,
				),
				symbol.KindReturn: assigned(
					coretest.FunctionName, coretest.ReturnName, symbol.KindReturn,
				),
			}
			for _, kind := range coretest.MatchableKinds() {
				id, stated := want[kind]
				assert.True(t, stated,
					"the case names an identity for every matchable kind, including "+kind.String())
				_, held := g.Lookup(id)
				assert.True(t, held,
					"the assignment step spells a "+kind.String()+" canonically")
			}
		})

		t.Run("spells a variant under the type that declares it", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, oneFileTree(), with(&everyKind{frontendtest.NewScripted()}))

			enumVariant := assigned(
				coretest.EnumName, coretest.EnumVariantName, symbol.KindEnumVariant,
			)
			held, found := g.Lookup(enumVariant)
			assert.True(t, found, "an enum variant is a member of its enum")
			assert.Equal(t, held.(*node.EnumVariant).Host,
				assigned("", coretest.EnumName, symbol.KindEnum),
				"hosted by the enum that stands")

			sumVariant := assigned(
				coretest.SumName, coretest.SumVariantName, symbol.KindSumVariant,
			)
			held, found = g.Lookup(sumVariant)
			assert.True(t, found, "a sum variant is a member of its sum")
			assert.Equal(t, held.(*node.SumVariant).Host,
				assigned("", coretest.SumName, symbol.KindSum),
				"hosted by the sum that stands")
		})

		t.Run("hosts a member on the identity that stands", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, oneFileTree(), with(&everyKind{frontendtest.NewScripted()}))
			host := assigned("", coretest.StructName, symbol.KindStruct)

			field, found := g.Lookup(
				assigned(coretest.StructName, coretest.FieldName, symbol.KindField),
			)
			assert.True(t, found, "the struct's field is indexed")
			assert.Equal(t, field.(*node.Field).Host, host,
				"carrying the host the assignment step filled, not the one it was built with")

			method, found := g.Lookup(
				assigned(coretest.StructName, coretest.MethodName, symbol.KindMethod),
			)
			assert.True(t, found, "the struct's method is indexed")
			assert.Equal(t, method.(*node.Method).Host, host, "under the same host")
		})

		t.Run("owns a detached method by the type it receives", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, oneFileTree(), with(&nested{frontendtest.NewScripted()}))
			_, held := g.Lookup(assigned(outerName, detachedName, symbol.KindMethod))
			assert.True(t, held,
				"a method declared outside its type is owned by the type it attaches to")
		})

		t.Run("leaves a positional field unnamed and hosted", func(t *testing.T) {
			t.Parallel()

			outer := outerOf(t, loadNested(t))
			assert.True(t, outer.Fields[0].ID.IsZero(),
				"only its type names a positional field, so nothing indexes it")
			assert.Equal(t, outer.Fields[0].Host, assigned("", outerName, symbol.KindStruct),
				"it still carries the declaration that holds it")
		})

		t.Run("names an embed by the embedded type's bare name", func(t *testing.T) {
			t.Parallel()

			g := loadNested(t)
			outer := outerOf(t, g)
			assert.Length(t, outer.Embeds, 1, "the embed survives the assignment")
			assert.Equal(t, outer.Embeds[0].Host, assigned("", outerName, symbol.KindStruct),
				"it carries the declaration that holds it")
			want := assigned(outerName, baseSpelling, symbol.KindEmbed)
			assert.Equal(t, outer.Embeds[0].ID, want,
				"and is named by the type it embeds, under its host's owner chain")
			_, held := g.Lookup(want)
			assert.True(t, held, "so a directive on the embed has a subject the graph holds")
		})

		t.Run("strips the decoration and the qualifier off an embed's name", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, oneFileTree(), with(&decorated{frontendtest.NewScripted()}))
			outer := outerOf(t, g)
			assert.Equal(t, outer.Embeds[0].ID.Name, baseSpelling,
				"a pointer to a qualified, instantiated type embeds under the bare name")
			assert.Equal(t, outer.Embeds[1].ID.Name, baseSpelling+"2",
				"and a structural reference names its one named child")
		})

		t.Run("panics on a symbol outside the node model", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				_, _, _ = load.Load(context.Background(), mustConfig(
					oneFileTree(), with(&foreign{frontendtest.NewScripted()}),
				))
			}, "a malformed graph is the frontend's defect, found at the assignment")
			assert.Contains(t, got, "not a node declaration", "and says so")
		})
	})

	t.Run("members", func(t *testing.T) {
		t.Parallel()

		t.Run("descends into a type nested in a type", func(t *testing.T) {
			t.Parallel()

			g := loadNested(t)
			_, held := g.Lookup(assigned(outerName, innerName, symbol.KindStruct))
			assert.True(t, held, "a nested type is owned by the type that declares it")

			_, held = g.Lookup(assigned(outerName+"."+innerName, heldName, symbol.KindField))
			assert.True(t, held,
				"and its own members carry the dotted chain of enclosing type names")
		})
	})

	t.Run("derive", func(t *testing.T) {
		t.Parallel()

		t.Run("spells overloads apart by their discriminator", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, overloadTree())
			wide, held := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: apiPath, Owner: userName, Name: "Get",
				Kind: symbol.KindMethod, Disc: "string,int",
			})
			assert.True(t, held, "the parameter spellings discriminate")
			assert.Length(t, wide.(*node.Method).Params, 2, "the wide overload")

			_, held = g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: apiPath, Owner: userName, Name: "Get",
				Kind: symbol.KindMethod,
			})
			assert.True(t, held, "the nullary overload spells an empty discriminator")
			coretest.AssertCodes(t, sink)
		})

		t.Run("panics on a named kind that names nothing", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				_, _, _ = load.Load(context.Background(), mustConfig(
					oneFileTree(), with(&nameless{frontendtest.NewScripted()}),
				))
			}, "a nameless declaration of a named kind is a structural defect")
			assert.Contains(t, got, "names nothing", "and says so")
		})
	})

	t.Run("signature", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a repeated blank by its position", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, oneFileTree(), with(&blanks{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			slot := func(name string, kind symbol.Kind) symbol.Identity {
				id := assigned(serveName, name, kind)
				id.Disc = servedDisc
				return id
			}
			for _, id := range []symbol.Identity{
				slot(blankName, symbol.KindParam),
				slot(secondByPlace, symbol.KindParam),
				slot(blankName, symbol.KindReturn),
				slot(secondByPlace, symbol.KindReturn),
			} {
				_, held := g.Lookup(id)
				assert.True(t, held,
					"the first blank keeps its spelling and the second takes its position: "+
						id.String())
			}
		})

		t.Run("panics on a nil entry, naming the frontend", func(t *testing.T) {
			t.Parallel()

			key := &node.Param{Name: keyName, Type: &node.TypeRef{Spelling: stringType}}
			for _, tt := range []struct {
				name string
				decl *node.Function
				kind symbol.Kind
			}{
				{
					name: "a nil parameter",
					decl: &node.Function{Name: serveName, Params: []*node.Param{nil, key}},
					kind: symbol.KindParam,
				},
				{
					name: "a nil return",
					decl: &node.Function{Name: serveName, Returns: []*node.Return{nil}},
					kind: symbol.KindReturn,
				},
				{
					name: "a nil type parameter",
					decl: &node.Function{Name: serveName, TypeParams: []*node.TypeParam{nil}},
					kind: symbol.KindTypeParam,
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got := assert.Panics(t, func() {
						_, _, _ = load.Load(context.Background(), mustConfig(oneFileTree(), with(
							&holey{Scripted: frontendtest.NewScripted(), decl: tt.decl},
						)))
					}, "the store cannot index a nil declaration, so the frontend's defect stops the load")
					assert.Contains(t, got, "nil "+tt.kind.String(), "naming the entry")
					assert.Contains(t, got, string(frontendtest.ScriptedID),
						"and the frontend that built it")
				})
			}
		})
	})
}

// loadNested drives one load over the edge-shape package.
func loadNested(tb assert.TB) *store.Graph {
	tb.Helper()

	g, _, _ := loadTree(tb, oneFileTree(), with(&nested{frontendtest.NewScripted()}))
	return g
}

// outerOf returns the enclosing struct of the edge-shape package.
func outerOf(tb assert.TB, g *store.Graph) *node.Struct {
	tb.Helper()

	held, found := g.Lookup(assigned("", outerName, symbol.KindStruct))
	assert.True(tb, found, "the enclosing struct is indexed")
	return held.(*node.Struct)
}

// everyKind plants one declaration of every kind a rule can match,
// so the assignment step meets the whole vocabulary rather than the
// struct the scripted language writes.
type everyKind struct {
	*frontendtest.Scripted
}

// Parse hands the fixture package's files to the unit's builder,
// identities and all, so what the assignment step spells overwrites
// what the fixture was built with.
func (*everyKind) Parse(_ context.Context, u *plugin.SourceUnit) error {
	built := coretest.EveryKind(coretest.StorePath)
	pkg := u.Graph().Package(coretest.StorePath)
	pkg.Name = built.Name
	pkg.Files = append(pkg.Files, built.Files...)
	return nil
}

// nested plants the shapes the matchable vocabulary leaves out.
type nested struct {
	*frontendtest.Scripted
}

// Parse builds one enclosing type holding a nested type, a
// positional field and an embed, beside a detached method and a
// constraint.
func (*nested) Parse(_ context.Context, u *plugin.SourceUnit) error {
	inner := &node.Struct{
		Name:   innerName,
		Fields: []*node.Field{{Name: heldName}},
	}
	outer := &node.Struct{
		Name:   outerName,
		Fields: []*node.Field{{Type: &node.TypeRef{Spelling: baseSpelling}}},
		Types:  node.Symbols{inner},
		Embeds: []*node.Embed{{Ref: &node.TypeRef{Spelling: baseSpelling}}},
	}
	pkg := u.Graph().Package(coretest.StorePath)
	pkg.Files = append(pkg.Files, &node.File{
		Path: u.Files()[0].Path,
		Decls: node.Symbols{
			outer,
			&node.Method{Name: detachedName, Receives: &node.TypeRef{Spelling: outerName}},
		},
	})
	return nil
}

// pathless builds a file nothing can name.
type pathless struct {
	*frontendtest.Scripted
}

// Parse declares a package holding a file with no path.
func (*pathless) Parse(_ context.Context, u *plugin.SourceUnit) error {
	pkg := u.Graph().Package(coretest.StorePath)
	pkg.Files = append(pkg.Files, &node.File{})
	return nil
}

// nameless builds a declaration of a named kind that names nothing.
type nameless struct {
	*frontendtest.Scripted
}

// Parse declares a struct with no name.
func (*nameless) Parse(_ context.Context, u *plugin.SourceUnit) error {
	pkg := u.Graph().Package(coretest.StorePath)
	pkg.Files = append(pkg.Files, &node.File{
		Path:  u.Files()[0].Path,
		Decls: node.Symbols{&node.Struct{}},
	})
	return nil
}

// foreign builds a declaration the node model does not admit at
// file level.
type foreign struct {
	*frontendtest.Scripted
}

// Parse ignores the source and plants a Param in the file's
// declarations.
func (*foreign) Parse(_ context.Context, u *plugin.SourceUnit) error {
	pkg := u.Graph().Package(coretest.StorePath)
	pkg.Files = append(pkg.Files, &node.File{
		Path:  u.Files()[0].Path,
		Decls: node.Symbols{&node.Param{Name: "loose"}},
	})
	return nil
}

// blanks declares a function whose two parameters and two results
// are all written as the blank, which Go and Rust admit.
type blanks struct {
	*frontendtest.Scripted
}

// Parse declares the one function.
func (*blanks) Parse(_ context.Context, u *plugin.SourceUnit) error {
	pkg := u.Graph().Package(coretest.StorePath)
	pkg.Files = append(pkg.Files, &node.File{
		Path: u.Files()[0].Path,
		Decls: node.Symbols{&node.Function{
			Name: serveName,
			Params: []*node.Param{
				{Name: blankName, Type: &node.TypeRef{Spelling: writerType}},
				{Name: blankName, Type: &node.TypeRef{Spelling: requestType}},
			},
			Returns: []*node.Return{{Name: blankName}, {Name: blankName}},
		}},
	})
	return nil
}

// holey declares one function with a nil entry in its signature, in
// the list the case chooses.
type holey struct {
	*frontendtest.Scripted
	decl *node.Function
}

// Parse declares the one function.
func (h *holey) Parse(_ context.Context, u *plugin.SourceUnit) error {
	pkg := u.Graph().Package(coretest.StorePath)
	pkg.Files = append(pkg.Files, &node.File{
		Path:  u.Files()[0].Path,
		Decls: node.Symbols{h.decl},
	})
	return nil
}

// decorated plants a struct embedding a decorated spelling and a
// structural reference, so the assignment's naming of an embed is
// held at both shapes a frontend can build.
type decorated struct {
	*frontendtest.Scripted
}

// Parse builds the file with the two embeds.
func (d *decorated) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := d.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	outer := &node.Struct{
		Name: outerName,
		Embeds: []*node.Embed{
			{Ref: &node.TypeRef{Spelling: "*pkg." + baseSpelling + "[int]"}},
			{Ref: &node.TypeRef{
				Spelling: "*" + baseSpelling + "2", Form: symbol.FormOptional,
				Elems: []*node.TypeRef{{Spelling: baseSpelling + "2"}},
			}},
		},
	}
	pkg := u.Graph().Package(coretest.StorePath)
	pkg.Files = append(pkg.Files, &node.File{
		Path: u.Files()[0].Path, Decls: node.Symbols{outer},
	})
	return nil
}
