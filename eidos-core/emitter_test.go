// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/symbol"
)

// refPlugin is the plugin whose handler asks the reference fixture
// for a reference.
const refPlugin plugin.ID = "referrer"

// The reference fixture's template name and the payload it passes.
const (
	refTemplate = "method1.tpl"
	refPayload  = "payload"
)

// The tags of the invocation fixture's families beside its primary
// per-source family.
const (
	packageTag eidos.Tag = "pkg"
	planTag    eidos.Tag = "plan"
)

// The allocations of a phase call over the invocation fixture whose
// handler calls one method of the emitter, its handle or its slot view
// once per invocation. A phase call's state is pooled, so an accessor,
// an append and a slot view allocate nothing per invocation, and a call
// allocates what its effects leave in the store.
const (
	// unitAllocs is a call whose invocations touch one accumulator: the
	// store's acceptance of the one unit, which has no declarations, so
	// the first group of the store's set of keys, the key, and its list
	// of units.
	unitAllocs = 3
	// appendAllocs is a call whose invocations append one declaration
	// each to one accumulator: the unit, its list of declarations and
	// its list of origins. The declaration has no origin, so the store
	// indexes nothing.
	appendAllocs = unitAllocs + 2
	// refAllocs is a call whose invocations take one template reference
	// each, which a body keeps.
	refAllocs = invocationStructs
	// orderAllocs is an emit-phase call: the order of the store's units,
	// which the rule's enumeration sorts once.
	orderAllocs = 1
	// slotAppendAllocs is an emit-phase call whose invocations append one
	// value each into a slot of a value the store contains: each host's
	// slot, the store's map from its declarations to their units, which
	// allocates four times at its final size, and the unit's list of
	// contributors.
	slotAppendAllocs = orderAllocs + invocationStructs + 4 + 1
	// joinNameAllocs is the joined name of a word and a base.
	joinNameAllocs = 1
	// typeAllocs is a call whose invocations each translate a builtin of
	// another language. Each invocation allocates the binding's memo, the
	// memo's first group, the folded shape and the reference that the
	// spoke returns. Once per call, the lane creates its tracked reader,
	// and the match makes its resolver of other languages and its reader
	// of policy overrides.
	typeAllocs = 4*invocationStructs + 3
)

// The translation fixture's target, the key of its one lowering policy,
// and the key's two choices.
const (
	tsxTarget plugin.Target    = "tsx"
	widthKey  plugin.PolicyKey = "tsx.width"
	narrow    plugin.Choice    = "narrow"
	wide      plugin.Choice    = "wide"
)

// The spellings of the translation fixture: two builtins of the scripted
// language, the spoke's spelling of text, a stream of the source
// language, and a type parameter.
const (
	intSpelling    = "int"
	stringSpelling = "string"
	textSpelling   = "text"
	streamSpelling = "chan int"
	paramSpelling  = "T"
)

// The names of the translation fixture's struct and of its fields.
const (
	sessionName = "Session"
	expiresName = "Expires"
	noteName    = "Note"
)

// errUnspelled is the reason of the translation fixture's spoke for a
// form that it has no spelling of.
var errUnspelled = errors.New("eidos_test: the tsx target has no spelling of the form")

// tsxSpoke is the translation fixture's spoke. It spells a scalar as the
// choice of widthKey, text as textSpelling and a reference as the name of
// its referent, and it refuses every other form with errUnspelled. It
// records the shape of its last call.
type tsxSpoke struct{ last rules.TypeShape }

var _ plugin.TypeSpeller = (*tsxSpoke)(nil)

// SpellType spells s under p, and records s.
func (sp *tsxSpoke) SpellType(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	sp.last = s
	switch s.Form {
	case symbol.FormScalar:
		return &emit.TypeRef{Spelling: string(p.Choice(widthKey))}, nil
	case symbol.FormText:
		return &emit.TypeRef{Spelling: textSpelling}, nil
	case symbol.FormReference:
		return &emit.TypeRef{Spelling: s.Ref.Name, Target: s.Ref}, nil
	default:
		return nil, errUnspelled
	}
}

// translationFixture is a generator context over a session struct of
// the fixture's language, whose expires field the cases translate types
// for. Its note field is a string with the optional mark. The context's
// plan targets tsx through spoke, under the default choice of the width
// policy. width is the handle that an override stamps through.
type translationFixture struct {
	ctx     *plugin.GeneratorContext
	width   meta.Key[string]
	session *node.Struct
	expires *node.Field
	note    *node.Field
	spoke   *tsxSpoke
}

// newTranslationFixture returns the translation fixture, with the rules
// of the scripted language registered for the fixture's language.
func newTranslationFixture(tb assert.TB) translationFixture {
	tb.Helper()

	session := coretest.Struct(coretest.StorePath, sessionName)
	session.Pos = position.Pos{File: "session.go", Line: 3, Col: 6}
	expires := coretest.Field(coretest.StorePath, sessionName, expiresName)
	expires.Pos = position.Pos{File: "session.go", Line: 5, Col: 2}
	note := coretest.Field(coretest.StorePath, sessionName, noteName)
	note.Type, note.Optional = &node.TypeRef{Spelling: stringSpelling}, true
	session.Fields = []*node.Field{expires, note}
	g := coretest.Frozen(tb, coretest.Package(coretest.StorePath, session))
	reg := meta.NewRegistry()
	target := reg.For(string(tsxTarget))
	assert.NoError(tb, target.ClaimNamespace(string(tsxTarget)), "the target claims its namespace")
	width, err := meta.Register[string](target, meta.KeySpec{
		Name: meta.KeyName(widthKey), Doc: "the width of an integer in tsx",
	})
	assert.NoError(tb, err, "the policy's key registers")
	facts := meta.NewFacts(reg)
	policy, err := plugin.NewPolicy(tsxTarget, []plugin.PolicySpec{{
		Key: widthKey, Choices: []plugin.Choice{narrow, wide}, Default: narrow, Doc: "the width of an integer in tsx",
	}}, nil)
	assert.NoError(tb, err, "the policy resolves")
	ix, err := plugin.NewIndex(g, facts, nil, nil)
	assert.NoError(tb, err, "the routing surface builds")
	registry := rules.NewRegistry()
	assert.NoError(tb, registry.Register(fixtureLanguage{rulestest.Scripted()}), "the fixture language registers")
	spoke := &tsxSpoke{}
	return translationFixture{
		ctx: &plugin.GeneratorContext{
			Index: ix, Facts: facts, Emit: plugin.NewEmit(), Sink: diag.NewSink(), Rules: registry,
			Plugin: contextPlugin, Bucket: 2, Target: tsxTarget, Types: spoke, Policy: policy,
		},
		width: width, session: session, expires: expires, note: note, spoke: spoke,
	}
}

// override stamps choice under the width key on subject, at authority.
func (f translationFixture) override(tb assert.TB, subject symbol.Identity, choice plugin.Choice, a meta.Authority) {
	tb.Helper()

	claim := meta.Claim{Subject: subject, Authority: a}
	assert.NoError(tb, meta.Stamp(f.ctx.Facts, f.width, string(choice), claim), "the override stamps")
}

// translate runs one phase call over the fixture. Its handler translates
// ref with owner as the declaration of the type, on the session struct.
// It returns what the translation returned.
func (f translationFixture) translate(tb assert.TB, owner symbol.Identity, ref *node.TypeRef) (*emit.TypeRef, bool) {
	tb.Helper()

	var (
		got *emit.TypeRef
		ok  bool
		ran bool
	)
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnStruct(func(_ *eidos.StructMatch, e *eidos.Emitter) error {
			got, ok = e.Type(owner, ref)
			ran = true
			return nil
		})).
		Build()
	assert.NoError(tb, generatorOf(tb, p).Generate(f.ctx), "the phase call passes")
	assert.True(tb, ran, "the handler ran")
	return got, ok
}

// The emitter is the handler's write surface: family misuse is a
// defect that panics, every family has its own handle, an empty
// append changes nothing, and the spellings a target decides arrive
// through it.
func TestEmitter(t *testing.T) {
	t.Parallel()

	t.Run("PlanFile", func(t *testing.T) {
		t.Parallel()

		t.Run("panics on more than one tag", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin("t").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "a"}).
				Output(plugin.Output{Tag: "x", Per: plugin.PerPlan, Word: "b"}).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					e.PlanFile("x", "x")
					return nil
				})).
				Build()
			assert.Panics(t, func() {
				_ = generatorOf(t, p).Generate(genContext(t, g, facts, nil))
			}, "two tags address nothing")
		})

		t.Run("panics on the wrong cardinality", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin("t").
				Output(plugin.Output{Per: plugin.PerSource, Word: "stub"}).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					e.PlanFile()
					return nil
				})).
				Build()
			assert.Panics(t, func() {
				_ = generatorOf(t, p).Generate(genContext(t, g, facts, nil))
			}, "a per-source family is not a plan file")
		})

		t.Run("returns a distinct handle per family in one invocation", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("splitter").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "audit"}).
				Output(plugin.Output{Tag: "aux", Per: plugin.PerPackage, Word: "aux"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					plan := e.PlanFile()
					aux := e.PackageFile("aux")
					plan.Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Plan" + m.Struct.Name,
					})
					aux.Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Aux" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			byTag := map[string]plugin.Unit{}
			for u := range ctx.Emit.Units() {
				byTag[u.Tag] = u
			}
			assert.Length(t, byTag, 2, "each family assembled its own unit")
			assert.Length(t, byTag[""].Decls, 2, "the plan handle took only its own appends")
			assert.Length(t, byTag["aux"].Decls, 2, "the aux handle took only its own appends")
		})

		t.Run("returns distinct handles past the handle pool", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			tags := []eidos.Tag{"", "b", "c", "d", "e", "f"}
			b := eidos.NewPlugin("fanout")
			for _, tag := range tags {
				b.Output(plugin.Output{
					Tag: string(tag), Per: plugin.PerPlan, Word: "w" + string(tag),
				})
			}
			p := b.Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
				handles := make([]*eidos.Out, 0, len(tags))
				for _, tag := range tags {
					handles = append(handles, e.PlanFile(tag))
				}
				for i, h := range handles {
					h.Append(&emit.Struct{
						Origin: m.Struct.Identity(),
						Name:   string(tags[i]) + m.Struct.Name,
					})
				}
				return nil
			})).Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			units := slices.Collect(ctx.Emit.Units())
			assert.Length(t, units, len(tags), "one unit per family")
			for _, u := range units {
				assert.Length(t, u.Decls, 2, "every family took its two appends")
				first, held := u.Decls[0].(*emit.Struct)
				assert.True(t, held, "the fixture emits structs")
				assert.HasPrefix(t, first.Name, u.Tag,
					"each append arrived under the handle that made it")
			}
		})
	})

	t.Run("Append", func(t *testing.T) {
		t.Parallel()

		t.Run("records nothing for no declarations", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))

			p := eidos.NewPlugin("weaver").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "audit"}).
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						e.PlanFile().Append()
						return nil
					})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			var flushed []plugin.Unit
			for u := range ctx.Emit.Units() {
				if u.Plugin == "weaver" {
					flushed = append(flushed, u)
				}
			}
			assert.Length(t, flushed, 1, "the touched accumulator still flushes")
			expect.Empty(t, flushed[0].Decls, "the unit has no declaration")
			expect.Empty(t, flushed[0].Origins, "an empty append fabricates no provenance")
		})

		t.Run("places one origin's declarations in gating instance order across rules", func(t *testing.T) {
			t.Parallel()

			const instances = 2
			g, alpha := gatedStruct(t, instances)
			_, facts := boolKey(t)
			schema := stubSchema(gateName)
			schema.Repeatable = true
			ctx := genContext(t, g, facts, repeated(alpha.ID, schema, instances))
			placing := func(rule string) emitHandler {
				return func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.File().Append(&emit.Struct{
						Origin: m.Struct.Identity(),
						Name:   rule + strconv.Itoa(m.Directive().Instance),
					})
					return nil
				}
			}
			p := eidos.NewPlugin(contextPlugin).
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Handle(eidos.Directive(schema, eidos.OnStruct(placing("A")), eidos.OnStruct(placing("B")))).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			var names []string
			for u := range ctx.Emit.Units() {
				for _, d := range u.Decls {
					s, held := d.(*emit.Struct)
					assert.True(t, held, "the fixture emits structs")
					names = append(names, s.Name)
				}
			}
			assert.Equal(t, names, []string{"A0", "B0", "A1", "B1"},
				"each instance's declarations precede the next instance's, in rule order")
		})
	})

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		t.Run("keys one accumulator per subject's source file", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("perfile").
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.File().Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Gen" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			byKey := map[string]plugin.Unit{}
			for u := range ctx.Emit.Units() {
				byKey[u.Key] = u
			}
			assert.Length(t, byKey, 2, "two subjects in two files assemble two units")
			assert.Equal(t, byKey[alpha.Pos.File].Per, plugin.PerSource, "each is keyed per source")
			assert.Length(t, byKey[alpha.Pos.File].Decls, 1,
				"the first file's unit took only its own subject")
			assert.Length(t, byKey[beta.Pos.File].Decls, 1,
				"the second file's unit took only its own subject")
		})

		t.Run("keys the empty string for a subject with no position", func(t *testing.T) {
			t.Parallel()

			placed := coretest.Struct(coretest.StorePath, "Placed")
			placed.Pos = position.Pos{File: "placed.go", Line: 1, Col: 1}
			loose := coretest.Struct(coretest.StorePath, "Loose")
			g := coretest.Frozen(t,
				coretest.Package(coretest.StorePath, placed, loose))
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("perfile").
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.File().Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Gen" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			keys := map[string]int{}
			for u := range ctx.Emit.Units() {
				keys[u.Key] = len(u.Decls)
			}
			assert.Equal(t, keys[""], 1,
				"a subject with no position keys the empty string and joins no other file's unit")
			assert.Equal(t, keys[placed.Pos.File], 1, "the positioned subject keeps its own file")
		})
	})

	t.Run("PackageFile", func(t *testing.T) {
		t.Parallel()

		t.Run("keys one accumulator per package path of each language", func(t *testing.T) {
			t.Parallel()

			const otherLang symbol.Lang = "other"
			native := coretest.Struct(coretest.StorePath, "Native")
			foreign := coretest.Struct(coretest.StorePath, "Foreign")
			foreign.ID.Lang = otherLang
			samePath := coretest.Package(coretest.StorePath, foreign)
			samePath.ID.Lang = otherLang
			samePath.Files[0].ID.Lang = otherLang
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, native), samePath)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("perpkg").
				Output(plugin.Output{Per: plugin.PerPackage, Word: "audit"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.PackageFile().Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "For" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			var langs []symbol.Lang
			for u := range ctx.Emit.Units() {
				assert.Equal(t, u.Key, coretest.StorePath, "every unit keys the one package path")
				assert.Length(t, u.Decls, 1, "each unit has its own language's subject alone")
				langs = append(langs, u.Pkg.Lang)
			}
			assert.Equal(t, langs, []symbol.Lang{coretest.Lang, otherLang},
				"two languages spelling one path assemble two units, in package order")
		})
	})

	t.Run("JoinName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			word string
			base string
			want string
		}{
			{name: "joins the word after the base", word: "stub", base: "store", want: "storeStub"},
			{name: "keeps the case of the base", word: "stub", base: "Store", want: "StoreStub"},
			{name: "raises a first letter of several bytes", word: "éclair", base: "store", want: "storeÉclair"},
			{name: "returns the base for an empty word", word: "", base: "store", want: "store"},
			{name: "returns the word for an empty base", word: "stub", base: "", want: "stub"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, (&eidos.Emitter{}).JoinName(tt.word, tt.base), tt.want,
					"the join is the neutral spelling")
			})
		}
	})

	t.Run("Ref", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a reference whose owner is the plugin the phase call runs as", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, referenced(t).Owner, contextPlugin,
				"the owner is the identity the call attributes its units to")
		})

		t.Run("returns a reference to the named template", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, referenced(t).Name, refTemplate, "the name is the one given")
		})

		t.Run("returns a reference with the given payload", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, referenced(t).Data, any(refPayload), "the payload is the one given")
		})
	})

	t.Run("Type", func(t *testing.T) {
		t.Parallel()

		// stream is a synchronous stream of the scripted language, which
		// the fixture's spoke refuses. The cases only read it.
		stream := &node.TypeRef{
			Spelling: streamSpelling, Form: symbol.FormStream, Elems: []*node.TypeRef{{Spelling: intSpelling}},
		}

		t.Run("returns nil for a nil reference", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := f.translate(t, f.expires.ID, nil)
			assert.True(t, ok, "a nil reference has nothing to refuse")
			assert.Nil(t, got, "the translation of a nil reference is nil")
		})

		t.Run("returns the reference as written in the target's language", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			owner := symbol.Identity{
				Lang:    symbol.Lang(tsxTarget),
				Package: "svc/store",
				Name:    "Row",
				Kind:    symbol.KindField,
			}
			ref := &node.TypeRef{
				Spelling: sessionName, Package: "./session",
				Target: symbol.Identity{Lang: symbol.Lang(tsxTarget), Package: "svc/session", Name: sessionName},
			}
			got, ok := f.translate(t, owner, ref)
			assert.True(t, ok, "the target spells its own language's references")
			assert.Equal(t, got, rules.EmitRef(ref), "the reference is the one that EmitRef restates")
		})

		t.Run("returns the parameter's name for a reference to a type parameter", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			param := coretest.MemberID(coretest.StorePath, sessionName, paramSpelling, symbol.KindTypeParam)
			got, ok := f.translate(t, f.expires.ID, &node.TypeRef{Spelling: paramSpelling, Target: param})
			assert.True(t, ok, "a type parameter translates")
			assert.Equal(t, got, &emit.TypeRef{Spelling: paramSpelling}, "the reference has the name and no target")
		})

		t.Run("returns the spoke's spelling of a reference of another language", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := f.translate(t, f.expires.ID, &node.TypeRef{Spelling: stringSpelling})
			assert.True(t, ok, "the spoke spells text")
			assert.Equal(t, got.Spelling, textSpelling, "the spelling is the spoke's")
		})

		t.Run("passes the spoke the shape that the rules of owner's language fold", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.translate(t, f.expires.ID, &node.TypeRef{Spelling: sessionName, Target: f.session.ID})
			assert.Equal(t, f.spoke.last.Form, symbol.FormReference, "a declaration folds to a Reference")
			assert.Equal(t, f.spoke.last.Ref, f.session.ID, "the shape refers to the declaration")
		})

		t.Run("passes the spoke the type of a field with presence as an optional", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.translate(t, f.note.ID, f.note.Type)
			assert.Equal(t, f.spoke.last.Form, symbol.FormOptional, "the field's optional mark becomes an optional")
			assert.Length(t, f.spoke.last.Elems, 1, "of one type")
			assert.Equal(t, f.spoke.last.Elems[0].Form, symbol.FormText, "the field's own type")
		})

		t.Run("passes the spoke a reference other than the field's type without presence", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := f.translate(t, f.note.ID, &node.TypeRef{Spelling: stringSpelling})
			assert.True(t, ok, "the spoke spells text")
			assert.Equal(t, got.Spelling, textSpelling, "the reference translates as itself")
		})

		t.Run("passes the spoke the type of a field outside the graph without presence", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			missing := coretest.MemberID(coretest.StorePath, sessionName, "Missing", symbol.KindField)
			got, ok := f.translate(t, missing, &node.TypeRef{Spelling: stringSpelling})
			assert.True(t, ok, "the spoke spells text")
			assert.Equal(t, got.Spelling, textSpelling, "the reference translates as itself")
		})

		t.Run("binds the rules of owner's language on a graph match", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			var ok bool
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnGraph(func(_ *eidos.GraphMatch, e *eidos.Emitter) error {
					_, ok = e.Type(f.expires.ID, &node.TypeRef{Spelling: intSpelling})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(f.ctx), "the phase call passes")
			assert.True(t, ok, "the spoke spells the scalar")
			assert.Equal(t, f.spoke.last.Form, symbol.FormScalar, "the builtin folds under the scripted rules")
		})

		t.Run("spells a choice of the plan's policy without an override", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, _ := f.translate(t, f.expires.ID, &node.TypeRef{Spelling: intSpelling})
			assert.Equal(t, got.Spelling, string(narrow), "the policy's default applies")
		})

		overrides := []struct {
			name    string
			subject func(f translationFixture) symbol.Identity
		}{
			{
				name:    "reads an override on owner",
				subject: func(f translationFixture) symbol.Identity { return f.expires.ID },
			},
			{
				name:    "reads an override on the match's subject",
				subject: func(f translationFixture) symbol.Identity { return f.session.ID },
			},
			{
				name:    "reads an override on owner's package",
				subject: func(translationFixture) symbol.Identity { return coretest.PackageID(coretest.StorePath) },
			},
		}
		for _, tt := range overrides {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := newTranslationFixture(t)
				f.override(t, tt.subject(f), wide, meta.AuthorityDirective)
				got, _ := f.translate(t, f.expires.ID, &node.TypeRef{Spelling: intSpelling})
				assert.Equal(t, got.Spelling, string(wide), "the override replaces the policy's choice")
			})
		}

		t.Run("prefers an override on owner to one on the match's subject", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.override(t, f.expires.ID, narrow, meta.AuthorityDirective)
			f.override(t, f.session.ID, wide, meta.AuthorityDirective)
			got, _ := f.translate(t, f.expires.ID, &node.TypeRef{Spelling: intSpelling})
			assert.Equal(t, got.Spelling, string(narrow), "the declaration with the type decides first")
		})

		t.Run("ignores a value below directive authority", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.override(t, f.expires.ID, wide, meta.AuthorityPlugin)
			got, _ := f.translate(t, f.expires.ID, &node.TypeRef{Spelling: intSpelling})
			assert.Equal(t, got.Spelling, string(narrow), "a stamp of a plugin does not override the policy")
		})

		widthReads := func(tb assert.TB, rec *recorder) []symbol.Identity {
			tb.Helper()

			assert.Length(tb, rec.invoked, 1, "the session's invocation journals")
			var read []symbol.Identity
			for _, r := range rec.invoked[0].facts {
				if r.Key == meta.KeyName(widthKey) {
					read = append(read, r.Subject)
				}
			}
			return read
		}

		t.Run("records the read of each override that it looks for", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, e *eidos.Emitter) error {
					e.Type(f.expires.ID, &node.TypeRef{Spelling: intSpelling})
					return nil
				})).
				Build()
			read := widthReads(t, journaledGenerate(t, f.ctx, p))
			assert.Permutation(t, read,
				[]symbol.Identity{f.expires.ID, f.session.ID, coretest.PackageID(coretest.StorePath)},
				"a missing override is a read, so a new override runs the invocation again")
		})

		t.Run("does not record an override read for a type that no policy decides", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, e *eidos.Emitter) error {
					e.Type(f.expires.ID, &node.TypeRef{Spelling: stringSpelling})
					return nil
				})).
				Build()
			assert.Empty(t, widthReads(t, journaledGenerate(t, f.ctx, p)),
				"an override of a key that the spoke does not read changes nothing")
		})

		t.Run("reads the override on an owner that is the match's subject once", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.Type(m.Struct.ID, &node.TypeRef{Spelling: intSpelling})
					return nil
				})).
				Build()
			read := widthReads(t, journaledGenerate(t, f.ctx, p))
			assert.Permutation(t, read, []symbol.Identity{f.session.ID, coretest.PackageID(coretest.StorePath)},
				"the invocation reads the subject and the package, each once")
		})

		t.Run("returns false for a shape that the spoke refuses", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := f.translate(t, f.expires.ID, stream)
			assert.False(t, ok, "the spoke refuses a stream")
			assert.Nil(t, got, "a refusal returns no reference")
		})

		t.Run("reports RefusedType at owner's position", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.translate(t, f.expires.ID, stream)
			one := onlyFinding(t, f.ctx.Sink)
			expect.Equal(t, one.Code, eidos.RefusedType, "the refusal reports under the code of a refused type")
			expect.Equal(t, one.Severity, diag.SeverityError, "the refusal fails the plan")
			expect.Equal(t, one.Pos, f.expires.Pos, "the finding is at the declaration with the type")
			expect.Equal(t, one.Msg,
				"the tsx target cannot spell the golang type chan int: "+errUnspelled.Error(),
				"the message contains the target, the source type and the spoke's reason")
		})

		t.Run("reports RefusedType for every reference of another language without a spoke", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.ctx.Types = nil
			got, ok := f.translate(t, f.expires.ID, &node.TypeRef{Spelling: stringSpelling})
			assert.False(t, ok, "a plan without a spoke does not spell a type of another language")
			assert.Nil(t, got, "a refusal returns no reference")
			assert.Equal(t, onlyFinding(t, f.ctx.Sink).Msg,
				"the tsx target cannot spell the golang type string: "+
					"the plan's backend does not implement a type spoke",
				"the message is about the missing spoke")
		})
	})

	t.Run("Slot", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a view whose appends apply to the slot", func(t *testing.T) {
			t.Parallel()

			host, _ := woven(t, 2)
			assert.Equal(t, fieldNames(host), []string{"audited", "audited"}, "both appends apply")
		})

		t.Run("returns a view that names the plugin among the host unit's contributors", func(t *testing.T) {
			t.Parallel()

			_, unit := woven(t, 1)
			assert.Equal(t, unit.Contributors, []plugin.ID{contextPlugin},
				"the weaver is attributed in the unit it appended into")
		})

		t.Run("panics on a nil slot", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
					e.Slot[*emit.Field](nil)
					return nil
				})).
				Build()
			assert.Panics(t, func() { _ = generatorOf(t, p).Generate(ctx) },
				"a view of no slot would append nowhere")
		})
	})

	t.Run("SlotView.Append", func(t *testing.T) {
		t.Parallel()

		t.Run("records no contributor for no values", func(t *testing.T) {
			t.Parallel()

			_, unit := woven(t, 0)
			assert.Empty(t, unit.Contributors, "an empty append contributes nothing")
		})

		t.Run("panics on the zero view", func(t *testing.T) {
			t.Parallel()

			var view eidos.SlotView[*emit.Field]
			assert.Panics(t, func() { view.Append(&emit.Field{Name: "lost"}) },
				"the zero view has no invocation to buffer with")
		})
	})
}

// Each method of the emitter, its handle and its slot view allocates
// nothing per invocation in the ordinary run, which runs no benchmark,
// and a phase call allocates what its effects leave in the store. A
// name join allocates the name. The check runs alone, because the count
// includes every goroutine's allocations.
func TestEmitterAllocs(t *testing.T) {
	checkPhaseAllocs(t, emitterCases(t))

	e := &eidos.Emitter{}
	var joined string
	assert.MaxAllocs(t, func() { joined = e.JoinName("stub", "store") }, joinNameAllocs,
		"JoinName allocates the joined name")
	assert.Equal(t, joined, "storeStub", "JoinName joins the word after the base")
	assert.MaxAllocs(t, func() { joined = e.JoinName("", "store") }, 0,
		"JoinName allocates nothing for an empty word")
	assert.Equal(t, joined, "store", "JoinName returns the base for an empty word")
}

// BenchmarkEmitter measures a phase call over the invocation fixture
// for each method of the emitter, its handle and its slot view, each
// called once per invocation, and the join of a name.
func BenchmarkEmitter(b *testing.B) {
	benchPhases(b, emitterCases(b))

	e := &eidos.Emitter{}
	b.Run("JoinName", func(b *testing.B) {
		b.Run("a word and a base", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(joinNameAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = e.JoinName("stub", "store")
			}
			assert.Equal(b, got, "storeStub", "JoinName joins the word after the base")
		})

		b.Run("an empty word", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got = e.JoinName("", "store")
			}
			assert.Equal(b, got, "store", "JoinName returns the base for an empty word")
		})
	})
}

// referenced returns the reference a graph handler of refPlugin asks
// its emitter for.
func referenced(t *testing.T) *emit.TemplateRef {
	t.Helper()

	g, _, _ := fixtureGraph(t)
	_, facts := boolKey(t)
	var got *emit.TemplateRef
	p := eidos.NewPlugin(refPlugin).
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
			got = e.Ref(refTemplate, refPayload)
			return nil
		})).
		Build()
	assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)), "the phase call passes")
	assert.NotNil(t, got, "the handler ran")
	return got
}

// woven runs a weaver whose handler appends the given number of fields
// through the view into the seeded value's slot, then appends nothing,
// and returns the seeded value and the seeded unit after the phase
// call.
func woven(tb assert.TB, fields int) (*emit.Struct, plugin.Unit) {
	tb.Helper()

	g, alpha, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	host := emitted(alpha)
	seed(tb, ctx, host)
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
			s, held := m.Value.(*emit.Struct)
			assert.True(tb, held, "a struct rule receives structs")
			view := e.Slot(&s.Fields)
			for range fields {
				view.Append(&emit.Field{Name: "audited"})
			}
			view.Append()
			return nil
		})).
		Build()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	var earlier plugin.Unit
	for u := range ctx.Emit.Units() {
		if u.Plugin == "earlier" {
			earlier = u
		}
	}
	return host, earlier
}

// emitterCases returns a phase call over the invocation fixture for each
// method of the emitter, its handle and its slot view, each called once
// per invocation. The plugin declares a family at each cardinality.
func emitterCases(tb assert.TB) []phaseCase {
	tb.Helper()

	fresh, _, _ := invocationContexts(tb, false)
	seeded, _, _ := invocationContexts(tb, true)
	families := func(h func(*eidos.StructMatch, *eidos.Emitter)) plugin.Generator {
		return generatorOf(tb, eidos.NewPlugin(contextPlugin).
			Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
			Output(plugin.Output{Tag: string(packageTag), Per: plugin.PerPackage, Word: "pkg"}).
			Output(plugin.Output{Tag: string(planTag), Per: plugin.PerPlan, Word: "plan"}).
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
				h(m, e)
				return nil
			})).
			Build())
	}
	weaver := func(h func(*emit.Struct, *eidos.Emitter)) plugin.Generator {
		return generatorOf(tb, eidos.NewPlugin(contextPlugin).
			Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
				s, held := m.Value.(*emit.Struct)
				assert.True(tb, held, "a struct rule receives structs")
				h(s, e)
				return nil
			})).
			Build())
	}
	oneUnit := func(per plugin.Cardinality, decls int) func(assert.TB, *plugin.GeneratorContext) {
		return func(tb assert.TB, ctx *plugin.GeneratorContext) {
			units := slices.Collect(ctx.Emit.Units())
			assert.Length(tb, units, 1, "the call flushes one unit")
			assert.Equal(tb, units[0].Per, per, "at the family's cardinality")
			assert.Length(tb, units[0].Decls, decls, "with the call's appends")
		}
	}
	unchanged := func(tb assert.TB, ctx *plugin.GeneratorContext) {
		assert.Length(tb, slices.Collect(ctx.Emit.Units()), 1, "the seeded unit alone remains")
	}

	declaration := &emit.Struct{Name: "Appended"}
	var ref *emit.TemplateRef
	field := &emit.Field{Name: "audited"}
	policy, err := plugin.NewPolicy(tsxTarget, []plugin.PolicySpec{{
		Key: widthKey, Choices: []plugin.Choice{narrow, wide}, Default: narrow, Doc: "the width of an integer in tsx",
	}}, nil)
	assert.NoError(tb, err, "the policy resolves")
	translating := func() *plugin.GeneratorContext {
		ctx := fresh()
		ctx.Target, ctx.Types, ctx.Policy = tsxTarget, &tsxSpoke{}, policy
		return ctx
	}
	scalar := &node.TypeRef{Spelling: intSpelling}
	var typed *emit.TypeRef
	return []phaseCase{
		{
			name: "File", allocs: unitAllocs, fresh: fresh, check: oneUnit(plugin.PerSource, 0),
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.File() }),
		},
		{
			name: "PackageFile", allocs: unitAllocs, fresh: fresh, check: oneUnit(plugin.PerPackage, 0),
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.PackageFile(packageTag) }),
		},
		{
			name: "PlanFile", allocs: unitAllocs, fresh: fresh, check: oneUnit(plugin.PerPlan, 0),
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.PlanFile(planTag) }),
		},
		{
			name: "Out.Append", allocs: appendAllocs, fresh: fresh,
			check: oneUnit(plugin.PerSource, invocationStructs),
			gen:   families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.File().Append(declaration) }),
		},
		{
			name: "Ref", allocs: refAllocs, fresh: fresh,
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { ref = e.Ref(refTemplate, refPayload) }),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				assert.Equal(tb, ref.Owner, ctx.Plugin, "the reference names the plugin the call runs as")
			},
		},
		{
			name: "Type", allocs: typeAllocs, fresh: translating,
			gen: families(func(m *eidos.StructMatch, e *eidos.Emitter) { typed, _ = e.Type(m.Struct.ID, scalar) }),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				assert.Equal(tb, typed.Spelling, string(narrow), "the spoke spells the scalar under the policy")
				assert.False(tb, ctx.Sink.Failed(), "no translation is refused")
			},
		},
		{
			name: "Slot", allocs: orderAllocs, fresh: seeded, check: unchanged,
			gen: weaver(func(s *emit.Struct, e *eidos.Emitter) { e.Slot(&s.Fields) }),
		},
		{
			name: "SlotView.Append", allocs: slotAppendAllocs, fresh: seeded,
			gen: weaver(func(s *emit.Struct, e *eidos.Emitter) { e.Slot(&s.Fields).Append(field) }),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				for u := range ctx.Emit.Units() {
					assert.Equal(
						tb,
						u.Contributors,
						[]plugin.ID{ctx.Plugin},
						"the weaver contributes to the seeded unit",
					)
					for _, d := range u.Decls {
						s, held := d.(*emit.Struct)
						assert.True(tb, held, "the seeded unit has structs")
						assert.Equal(tb, fieldNames(s), []string{field.Name}, "each host has the appended field")
					}
				}
			},
		},
	}
}
