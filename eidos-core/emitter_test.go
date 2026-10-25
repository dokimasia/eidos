// Copyright Dokimasia B.V. 2026
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
	"go.dokimi.dev/eidos/core/store"
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

// The record fixture: the plugin that records each struct into a family
// that claims its files, the family's file template, and the data of two
// calls that record one struct.
const (
	recordPlugin  plugin.ID = "recorder"
	claimTemplate string    = "test.tmpl"
	firstData     string    = "first"
	secondData    string    = "second"
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
	// fileOfAllocs is a call whose invocations each address the file of
	// their subject through FileOf: what unitAllocs counts, and the tracked
	// reader that the lane creates on the first read of the call.
	fileOfAllocs = unitAllocs + 1
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
	// fieldsAllocs is the count of a call whose invocations each translate
	// one field with a builtin type of another language. Each invocation
	// allocates what typeAllocs counts for it, the emit field and the list
	// of fields.
	fieldsAllocs = typeAllocs + 2*invocationStructs
	// typeParamsAllocs is the count of a call whose invocations each
	// translate one type parameter with a builtin of another language as its
	// bound. Each invocation allocates what typeAllocs counts for it, the list
	// of the parameters, the emit parameter and the list of its bounds.
	typeParamsAllocs = typeAllocs + 3*invocationStructs
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

// The name of a const type parameter of the translation fixture, and the
// spelling of its default value.
const (
	constName    = "N"
	constDefault = "4"
)

// The names of the translation fixture's struct and of its fields, and the
// documentation of its count field.
const (
	sessionName = "Session"
	expiresName = "Expires"
	countName   = "Count"
	noteName    = "Note"
	countDoc    = "Count is the number of reads."
)

// errUnspelled is the reason of the translation fixture's spoke for a
// form that it has no spelling of.
var errUnspelled = errors.New("eidos_test: the tsx target has no spelling of the form")

// streamRef is a synchronous stream of the scripted language, which the
// spokes of the translation fixture refuse. The cases only read it.
var streamRef = &node.TypeRef{
	Spelling: streamSpelling, Form: symbol.FormStream, Elems: []*node.TypeRef{{Spelling: intSpelling}},
}

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

// superPrefix opens the supertype spelling of [superSpoke].
const superPrefix = "super:"

// superSpoke is the translation fixture's spoke with a supertype spelling
// of its own: superPrefix followed by the name of the referent.
type superSpoke struct{ tsxSpoke }

var _ plugin.SupertypeSpeller = (*superSpoke)(nil)

// SpellSupertype spells the reference s as superPrefix and the name of its
// referent, and records s.
func (sp *superSpoke) SpellSupertype(s rules.TypeShape, _ plugin.Policy) (*emit.TypeRef, error) {
	sp.last = s
	return &emit.TypeRef{Spelling: superPrefix + s.Ref.Name, Target: s.Ref}, nil
}

// translationFixture is a generator context over a session struct of
// the fixture's language. The cases translate types for its expires field,
// which is an int. Its count field is a string with documentation, and its
// note field is a string with the optional mark. The context's plan targets
// tsx through spoke, under the default choice of the width policy. width is
// the handle that an override stamps through.
type translationFixture struct {
	ctx     *plugin.GeneratorContext
	width   meta.Key[string]
	session *node.Struct
	expires *node.Field
	count   *node.Field
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
	expires.Type = &node.TypeRef{Spelling: intSpelling}
	expires.Pos = position.Pos{File: "session.go", Line: 5, Col: 2}
	count := coretest.Field(coretest.StorePath, sessionName, countName)
	count.Type, count.Doc = &node.TypeRef{Spelling: stringSpelling}, []string{countDoc}
	note := coretest.Field(coretest.StorePath, sessionName, noteName)
	note.Type, note.Optional = &node.TypeRef{Spelling: stringSpelling}, true
	note.Pos = position.Pos{File: "session.go", Line: 7, Col: 2}
	session.Fields = []*node.Field{expires, count, note}
	spoke := &tsxSpoke{}
	ctx, width := tsxContext(tb, coretest.Frozen(tb, coretest.Package(coretest.StorePath, session)), spoke)
	return translationFixture{
		ctx: ctx, width: width, session: session, expires: expires, count: count, note: note, spoke: spoke,
	}
}

// tsxContext returns a generator context over the graph g whose plan targets
// tsx through the spoke types, under the default choice of the width policy,
// with the rules of the scripted language registered for the fixture's
// language. It also returns the handle of the width key, which an override
// stamps through.
func tsxContext(tb assert.TB, g *store.Graph, types plugin.TypeSpeller) (*plugin.GeneratorContext, meta.Key[string]) {
	tb.Helper()

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
	return &plugin.GeneratorContext{
		Index: ix, Facts: facts, Emit: plugin.NewEmit(), Sink: diag.NewSink(), Rules: registry,
		Plugin: contextPlugin, Turn: 2, Target: tsxTarget, Types: types, Policy: policy,
	}, width
}

// The documentation of the handler fixture's rpc, the name of the
// interface of its service, and the owner chain of a nested rpc of the
// fixture's language.
const (
	handlerDoc  = "Get returns the session of a key."
	serviceName = "Store"
	nestedOwner = "Outer.Store"
	outerName   = "Outer"
)

// binderSpoke is the translation fixture's spoke with a binder. Its Handler
// records the rpc of its call and returns err where err is set. Otherwise it
// spells the type of the first parameter through translate, records the
// spelling, and returns method. The emitter does not ask it for a home.
type binderSpoke struct {
	tsxSpoke
	rpc     plugin.RPC
	spelled *emit.TypeRef
	method  *emit.Method
	err     error
}

var _ plugin.Binder = (*binderSpoke)(nil)

// Handler records r, and returns err, the error of translate or method.
func (sp *binderSpoke) Handler(r plugin.RPC, _ plugin.Policy, translate plugin.Translate) (*emit.Method, error) {
	sp.rpc = r
	if sp.err != nil {
		return nil, sp.err
	}
	t, err := translate(r.Callable.Params[0].Ref)
	if err != nil {
		return nil, err
	}
	sp.spelled = t
	return sp.method, nil
}

// Home returns errUnspelled, because the emitter does not ask for a home.
func (*binderSpoke) Home(symbol.Identity, plugin.Placement) (symbol.Identity, error) {
	return symbol.Identity{}, errUnspelled
}

// preview is one preview that a placer of the cases is asked for: the
// subject whose routing applies, and the unit of the handle.
type preview struct {
	subject symbol.Identity
	unit    plugin.Unit
}

// placing is the placer of the cases. It returns placed and err for each
// preview, and keeps each preview in asked where asked is set.
type placing struct {
	placed plugin.Placed
	err    error
	asked  *[]preview
}

var _ plugin.Placer = placing{}

// Place keeps the preview in asked, and returns placed and err.
func (p placing) Place(subject symbol.Identity, u plugin.Unit) (plugin.Placed, error) {
	if p.asked != nil {
		*p.asked = append(*p.asked, preview{subject: subject, unit: u})
	}
	return p.placed, p.err
}

// handlerFixture is a generator context over the interface Store of the
// fixture's language. Its rpc has documentation, a position, a parameter of
// the type int and a return of the type string. The context's plan targets
// tsx through the fixture's spoke, under the default choice of the width
// policy. width is the handle that an override stamps through.
type handlerFixture struct {
	ctx   *plugin.GeneratorContext
	width meta.Key[string]
	store *node.Interface
	rpc   *node.Method
}

// newHandlerFixture returns the handler fixture over the spoke types.
func newHandlerFixture(tb assert.TB, types plugin.TypeSpeller) handlerFixture {
	tb.Helper()

	svc := coretest.Interface(coretest.StorePath, serviceName)
	rpc := svc.Methods[0]
	rpc.Doc, rpc.Pos = []string{handlerDoc}, position.Pos{File: "store.go", Line: 4, Col: 2}
	param := coretest.Param(coretest.StorePath, rpc.Name, coretest.ParamName)
	param.Type = &node.TypeRef{Spelling: intSpelling}
	result := coretest.Return(coretest.StorePath, rpc.Name, coretest.ReturnName)
	result.Type = &node.TypeRef{Spelling: stringSpelling}
	rpc.Params, rpc.Returns = []*node.Param{param}, []*node.Return{result}
	ctx, width := tsxContext(tb, coretest.Frozen(tb, coretest.Package(coretest.StorePath, svc)), types)
	return handlerFixture{ctx: ctx, width: width, store: svc, rpc: rpc}
}

// handle runs one phase call over the fixture. Its handler asks the emitter
// for the handler of rpc on the interface Store, and handle returns what the
// emitter returned.
func (f handlerFixture) handle(tb assert.TB, rpc *node.Method) (*emit.Method, bool) {
	tb.Helper()

	var (
		got *emit.Method
		ok  bool
		ran bool
	)
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnInterface(func(_ *eidos.InterfaceMatch, e *eidos.Emitter) error {
			got, ok = e.Handler(rpc)
			ran = true
			return nil
		})).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(f.ctx), "the phase call passes")
	assert.True(tb, ran, "the handler ran")
	return got, ok
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
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(f.ctx), "the phase call passes")
	assert.True(tb, ran, "the handler ran")
	return got, ok
}

// lowerFields runs one phase call over the fixture. Its handler translates
// the fields of the session struct that keep reports. It returns what the
// translation returned.
func (f translationFixture) lowerFields(tb assert.TB, keep func(*node.Field) bool) ([]*emit.Field, bool) {
	tb.Helper()

	var (
		got []*emit.Field
		ok  bool
		ran bool
	)
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			got, ok = e.Fields(m.Struct.Fields, keep)
			ran = true
			return nil
		})).
		Generator()
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
				Generator()
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
				Generator()
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
				Generator()
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
			})).Generator()
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
				Generator()
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
				Generator()
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

		t.Run("returns an error for a nil pointer of a declaration kind", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			p := eidos.NewPlugin(contextPlugin).
				Output(plugin.Output{Per: plugin.PerPlan, Word: "impl"}).
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, e *eidos.Emitter) error {
					e.PlanFile().Append((*emit.File)(nil))
					return nil
				})).
				Generator()
			assert.HasError(t, generatorOf(t, p).Generate(ctx), "the flush refuses the unit of a nil header")
		})

		t.Run("orders the declarations of one file by the positions of their origins", func(t *testing.T) {
			t.Parallel()

			zebra := coretest.Struct(coretest.StorePath, "Zebra")
			zebra.Pos = position.Pos{File: "shapes.go", Line: 3, Col: 6}
			apple := coretest.Struct(coretest.StorePath, "Apple")
			apple.Pos = position.Pos{File: "shapes.go", Line: 9, Col: 6}
			assert.Equal(t, declNames(t, orderedUnit(t, zebra, apple)), []string{"ForZebra", "ForApple"},
				"the declarations are in the order of the source")
		})

		t.Run("orders the declaration of an origin without a position after the others", func(t *testing.T) {
			t.Parallel()

			able := coretest.Struct(coretest.StorePath, "Able")
			able.Pos = position.Pos{File: "shapes.go", Line: 3, Col: 6}
			loose := coretest.Struct(coretest.StorePath, "Mid")
			zed := coretest.Struct(coretest.StorePath, "Zed")
			zed.Pos = position.Pos{File: "shapes.go", Line: 9, Col: 6}
			assert.Equal(t, declNames(t, orderedUnit(t, able, loose, zed)), []string{"ForAble", "ForZed", "ForMid"},
				"a declaration without a place in the source comes last")
		})

		t.Run("orders the declarations of origins without a position by their identity", func(t *testing.T) {
			t.Parallel()

			beta := coretest.Struct(coretest.StorePath, "Beta")
			alpha := coretest.Struct(coretest.StorePath, "Alpha")
			assert.Equal(t, declNames(t, orderedUnit(t, beta, alpha)), []string{"ForAlpha", "ForBeta"},
				"the identity of the origin breaks the tie")
		})

		t.Run("lists the origins of a unit in the order of their identities", func(t *testing.T) {
			t.Parallel()

			zebra := coretest.Struct(coretest.StorePath, "Zebra")
			zebra.Pos = position.Pos{File: "shapes.go", Line: 3, Col: 6}
			apple := coretest.Struct(coretest.StorePath, "Apple")
			apple.Pos = position.Pos{File: "shapes.go", Line: 9, Col: 6}
			assert.Equal(t, orderedUnit(t, zebra, apple).Origins, []symbol.Identity{apple.ID, zebra.ID},
				"the provenance is sorted")
		})

		handle := coretest.Struct(coretest.StorePath, "Handle").ID

		t.Run("records the subject of a declaration whose origin is another declaration", func(t *testing.T) {
			t.Parallel()

			_, alpha, _ := fixtureGraph(t)
			u := placedUnit(t, func(m *eidos.StructMatch) []symbol.Symbol {
				if m.Struct.ID != alpha.ID {
					return nil
				}
				return []symbol.Symbol{
					&emit.Struct{Origin: alpha.ID, Name: "AlphaProbe"},
					&emit.Struct{Origin: handle, Name: "HandleProbe"},
				}
			})
			assert.Equal(t, u.Subjects, map[symbol.Identity]symbol.Identity{handle: alpha.ID},
				"the layout routes the double of the handle with its subject")
		})

		t.Run("records the subject of the first placement for an origin that two matches place", func(t *testing.T) {
			t.Parallel()

			_, alpha, _ := fixtureGraph(t)
			u := placedUnit(t, func(m *eidos.StructMatch) []symbol.Symbol {
				return []symbol.Symbol{&emit.Struct{Origin: handle, Name: "HandleFor" + m.Struct.Name}}
			})
			assert.Equal(t, u.Subjects, map[symbol.Identity]symbol.Identity{handle: alpha.ID},
				"Alpha precedes Beta in the order of the unit")
		})

		t.Run("records no subject where every declaration originates from the subject of its match",
			func(t *testing.T) {
				t.Parallel()

				u := placedUnit(t, func(m *eidos.StructMatch) []symbol.Symbol {
					return []symbol.Symbol{&emit.Struct{Origin: m.Struct.ID, Name: "For" + m.Struct.Name}}
				})
				assert.Nil(t, u.Subjects, "each declaration routes with its own origin")
			})

		t.Run("records no subject for a declaration without an origin", func(t *testing.T) {
			t.Parallel()

			u := placedUnit(t, func(m *eidos.StructMatch) []symbol.Symbol {
				return []symbol.Symbol{&emit.Struct{Name: "Loose" + m.Struct.Name}}
			})
			assert.Nil(t, u.Subjects, "a declaration without an origin has nothing to map")
		})
	})

	t.Run("Record", func(t *testing.T) {
		t.Parallel()

		// recorded runs a plugin whose handler records each struct of
		// the fixture graph through record into a family that claims its
		// files, and returns the plugin's one unit.
		recorded := func(t *testing.T, record func(m *eidos.StructMatch, out *eidos.Out)) plugin.Unit {
			t.Helper()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			p := eidos.NewPlugin(recordPlugin).
				Output(plugin.Output{Per: plugin.PerPlan, Word: "test", Template: claimTemplate}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					record(m, e.PlanFile())
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			units := slices.Collect(ctx.Emit.Units())
			assert.Length(t, units, 1, "the plugin flushes one unit of the plan")
			return units[0]
		}
		recordName := func(m *eidos.StructMatch, out *eidos.Out) {
			out.Record(m.Struct.Name, &emit.Struct{Origin: m.Struct.Identity(), Name: "Test" + m.Struct.Name})
		}

		t.Run("attaches the data to each declaration", func(t *testing.T) {
			t.Parallel()

			_, alpha, beta := fixtureGraph(t)
			u := recorded(t, recordName)
			assert.Length(t, u.Decls, 2, "the unit has a record of each struct")
			assert.Equal(t, u.Data, map[symbol.Symbol]any{
				u.Decls[0]: alpha.Name, u.Decls[1]: beta.Name,
			}, "each record has the data of its call")
		})

		t.Run("places the declarations as Append does", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, recorded(t, recordName).Decls, 2, "the unit has a record of each struct")
		})

		t.Run("copies the template of the family to the unit", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, recorded(t, recordName).Template, claimTemplate, "the unit has the family's template")
		})

		t.Run("keeps the data of each record of an origin", func(t *testing.T) {
			t.Parallel()

			twice := func(m *eidos.StructMatch, out *eidos.Out) {
				out.Record(firstData, &emit.Struct{Origin: m.Struct.Identity(), Name: "First" + m.Struct.Name})
				out.Record(secondData, &emit.Struct{Origin: m.Struct.Identity(), Name: "Second" + m.Struct.Name})
			}
			u := recorded(t, twice)
			assert.Length(t, u.Decls, 4, "the unit has two records of each struct")
			expect.Equal(t, u.Data[u.Decls[0]], any(firstData), "the first record has the data of the first call")
			expect.Equal(t, u.Data[u.Decls[1]], any(secondData), "the second record has the data of the second call")
		})

		t.Run("keeps the data of the first call for a declaration that two calls place", func(t *testing.T) {
			t.Parallel()

			again := func(m *eidos.StructMatch, out *eidos.Out) {
				d := &emit.Struct{Origin: m.Struct.Identity(), Name: "Shared" + m.Struct.Name}
				out.Record(firstData, d)
				out.Record(secondData, d)
			}
			u := recorded(t, again)
			assert.Length(t, u.Decls, 4, "the unit places each declaration of each call")
			assert.Equal(t, u.Data[u.Decls[0]], any(firstData), "the first call in canonical match order applies")
		})

		t.Run("records no data for no declarations", func(t *testing.T) {
			t.Parallel()

			empty := func(m *eidos.StructMatch, out *eidos.Out) { out.Record(m.Struct.Name) }
			assert.Nil(t, recorded(t, empty).Data, "a record without a declaration attaches nothing")
		})

		t.Run("returns an error for a nil pointer of a declaration kind", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			p := eidos.NewPlugin(recordPlugin).
				Output(plugin.Output{Per: plugin.PerPlan, Word: "test", Template: claimTemplate}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.PlanFile().Record(m.Struct.Name, (*emit.Struct)(nil))
					return nil
				})).
				Generator()
			assert.HasError(t, generatorOf(t, p).Generate(ctx), "the flush refuses the unit of a nil record")
		})
	})

	t.Run("Internal", func(t *testing.T) {
		t.Parallel()

		// marked runs a plugin with one test family of the plan, which
		// states External where external is set. Its handler places a check
		// of each struct of the fixture graph through place, and marked
		// returns the plugin's one unit.
		marked := func(t *testing.T, external bool, place func(m *eidos.StructMatch, out *eidos.Out)) plugin.Unit {
			t.Helper()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			p := eidos.NewPlugin(recordPlugin).
				Output(plugin.Output{Tag: eidos.TagTest, Per: plugin.PerPlan, Word: "check", External: external}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					place(m, e.PlanFile(eidos.TagTest))
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			units := slices.Collect(ctx.Emit.Units())
			assert.Length(t, units, 1, "the plugin flushes one unit of the plan")
			return units[0]
		}
		_, alpha, beta := fixtureGraph(t)
		// markAlpha marks the check of alpha alone.
		markAlpha := func(m *eidos.StructMatch, out *eidos.Out) {
			if m.Struct.Name == alpha.Name {
				out = out.Internal()
			}
			out.Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Check" + m.Struct.Name})
		}

		t.Run("records the origin of each declaration that the handle places", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, marked(t, true, markAlpha).Inside, []symbol.Identity{alpha.ID},
				"the unit lists the origin of the marked check, and not the origin of the check of beta")
		})

		t.Run("records each origin once", func(t *testing.T) {
			t.Parallel()

			twice := func(m *eidos.StructMatch, out *eidos.Out) {
				out.Internal().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Check" + m.Struct.Name},
					&emit.Struct{Origin: m.Struct.Identity(), Name: "Probe" + m.Struct.Name})
			}
			assert.Equal(t, marked(t, true, twice).Inside, []symbol.Identity{alpha.ID, beta.ID},
				"the unit lists each origin once, in identity order")
		})

		t.Run("records no origin for a family that is not External", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, marked(t, false, markAlpha).Inside, "a family that tests from inside lists no origin")
		})

		t.Run("returns its handle", func(t *testing.T) {
			t.Parallel()

			same := func(m *eidos.StructMatch, out *eidos.Out) {
				assert.Equal(t, out.Internal(), out, "Internal returns the handle", assert.ByIdentity())
			}
			marked(t, true, same)
		})
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		// previewed runs a plugin with a family of each cardinality and a
		// test family that states External, with placer as the placer of the
		// call. Its handler previews the handle that open returns for alpha.
		// previewed returns the package and the error of the preview.
		previewed := func(
			t *testing.T, placer plugin.Placer, open func(e *eidos.Emitter) *eidos.Out,
		) (symbol.Identity, error) {
			t.Helper()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			ctx.Placer = placer
			var (
				got symbol.Identity
				err error
			)
			p := eidos.NewPlugin(recordPlugin).
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Output(plugin.Output{Tag: string(packageTag), Per: plugin.PerPackage, Word: "pkg"}).
				Output(plugin.Output{Tag: string(planTag), Per: plugin.PerPlan, Word: "plan"}).
				Output(plugin.Output{Tag: eidos.TagTest, Per: plugin.PerSource, Word: "check", External: true}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					if m.Struct.ID == alpha.ID {
						got, err = open(e).Package()
					}
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			return got, err
		}
		_, alpha, _ := fixtureGraph(t)
		pkg := coretest.PackageID(coretest.StorePath)
		tests := []struct {
			name     string
			giveOpen func(e *eidos.Emitter) *eidos.Out
			want     plugin.Unit
		}{
			{
				name:     "asks the placer for the file of the source of the subject",
				giveOpen: func(e *eidos.Emitter) *eidos.Out { return e.File() },
				want: plugin.Unit{
					Plugin: contextPlugin, Per: plugin.PerSource, Word: "impl", Key: alpha.Pos.File, Pkg: pkg,
				},
			},
			{
				name:     "asks the placer for the file of the package of the subject",
				giveOpen: func(e *eidos.Emitter) *eidos.Out { return e.PackageFile(packageTag) },
				want: plugin.Unit{
					Plugin: contextPlugin, Tag: string(packageTag), Per: plugin.PerPackage, Word: "pkg",
					Key: coretest.StorePath, Pkg: pkg,
				},
			},
			{
				name:     "asks the placer for the file of the plan",
				giveOpen: func(e *eidos.Emitter) *eidos.Out { return e.PlanFile(planTag) },
				want:     plugin.Unit{Plugin: contextPlugin, Tag: string(planTag), Per: plugin.PerPlan, Word: "plan"},
			},
			{
				name:     "asks the placer for the internal test file of an internal handle of an external family",
				giveOpen: func(e *eidos.Emitter) *eidos.Out { return e.File(eidos.TagTest).Internal() },
				want: plugin.Unit{
					Plugin: contextPlugin, Tag: eidos.TagTest, Per: plugin.PerSource, Word: "check",
					Key: alpha.Pos.File, Pkg: pkg, Inside: []symbol.Identity{alpha.ID},
				},
			},
			{
				name:     "asks the placer for the file of an internal handle of a family that tests from inside",
				giveOpen: func(e *eidos.Emitter) *eidos.Out { return e.File().Internal() },
				want: plugin.Unit{
					Plugin: contextPlugin, Per: plugin.PerSource, Word: "impl", Key: alpha.Pos.File, Pkg: pkg,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var asked []preview
				_, err := previewed(t, placing{asked: &asked}, tt.giveOpen)
				assert.NoError(t, err, "the preview routes")
				assert.Equal(t, asked, []preview{{subject: alpha.ID, unit: tt.want}},
					"the placer previews the unit of the handle for the subject")
			})
		}

		t.Run("returns the package that the placer derives", func(t *testing.T) {
			t.Parallel()

			derived := coretest.PackageID(coretest.StorePath + "_test")
			got, err := previewed(t, placing{placed: plugin.Placed{Package: derived}},
				func(e *eidos.Emitter) *eidos.Out { return e.File() })
			assert.NoError(t, err, "the preview routes")
			assert.Equal(t, got, derived, "Package returns the package of the preview")
		})

		t.Run("returns the error of the placer", func(t *testing.T) {
			t.Parallel()

			errRoute := errors.New("layout: Struct Alpha has no workspace source to be written beside")
			_, err := previewed(t, placing{err: errRoute}, func(e *eidos.Emitter) *eidos.Out { return e.File() })
			assert.ErrorIs(t, err, errRoute, "Package returns the reason of the layout")
		})

		t.Run("returns an error for a plan that does not write files", func(t *testing.T) {
			t.Parallel()

			_, err := previewed(t, nil, func(e *eidos.Emitter) *eidos.Out { return e.File() })
			assert.HasError(t, err, "a plan without a placer derives no package")
			assert.Contains(t, err.Error(), "does not write files", "the error message contains the reason")
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
				Generator()
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

		t.Run("tags each unit of the TagTest family with test", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("suite").
				Output(plugin.Output{Tag: eidos.TagTest, Per: plugin.PerSource, Word: "suite"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.File(eidos.TagTest).Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Suite" + m.Struct.Name,
					})
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			var tags []string
			for u := range ctx.Emit.Units() {
				tags = append(tags, u.Tag)
			}
			assert.Equal(t, tags, []string{"test", "test"},
				"each unit has the tag that the targets spell as a test file")
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
				Generator()
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

	t.Run("FileOf", func(t *testing.T) {
		t.Parallel()

		t.Run("keys the accumulator on the source file of a declaration of the package", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			p := eidos.NewPlugin("perfile").
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Handle(eidos.OnPackage(func(_ *eidos.PackageMatch, e *eidos.Emitter) error {
					e.FileOf(beta.ID).Append(&emit.Struct{Origin: beta.ID, Name: "Gen" + beta.Name})
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			units := slices.Collect(ctx.Emit.Units())
			assert.Length(t, units, 1, "the package match assembles one unit")
			assert.Equal(t, units[0].Key, beta.Pos.File, "the unit has the file of the declaration as its key")
		})

		t.Run("records the read of the declaration", func(t *testing.T) {
			t.Parallel()

			g, _, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin(contextPlugin).
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Handle(eidos.OnPackage(func(_ *eidos.PackageMatch, e *eidos.Emitter) error {
					e.FileOf(beta.ID)
					return nil
				})).
				Generator()
			rec := journaledGenerate(t, genContext(t, g, facts, nil), p)
			assert.Length(t, rec.invoked, 1, "the one package has one invocation")
			assert.Contains(t, rec.invoked[0].identities, beta.ID,
				"a move of the declaration to another file runs the invocation again")
		})

		refusals := []struct {
			name string
			give symbol.Identity
		}{
			{
				name: "reports RefusedFile for a declaration of another package",
				give: coretest.ID(coretest.CachePath, coretest.StructName, symbol.KindStruct),
			},
			{
				name: "reports RefusedFile for an identity that the package of the subject does not declare",
				give: coretest.ID(coretest.StorePath, coretest.ForeignName, symbol.KindStruct),
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ctx, alpha := fileOfRun(t, tt.give)
				one := onlyFinding(t, ctx.Sink)
				expect.Equal(t, one.Code, eidos.RefusedFile, "the finding has the code of the refusal")
				expect.Equal(t, one.Severity, diag.SeverityError, "the refusal is an Error")
				expect.Equal(t, one.Pos, alpha.Pos, "the finding is at the position of the subject")
			})
		}

		t.Run("returns the accumulator of the file of the subject for a refused identity", func(t *testing.T) {
			t.Parallel()

			ctx, alpha := fileOfRun(t, coretest.ID(coretest.StorePath, coretest.ForeignName, symbol.KindStruct))
			units := slices.Collect(ctx.Emit.Units())
			assert.Length(t, units, 1, "the handler appends into one unit")
			assert.Equal(t, units[0].Key, alpha.Pos.File, "the unit has the file of the subject as its key")
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
				Generator()
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
			tail string
			head string
			want string
		}{
			{name: "joins the tail after the head", tail: "stub", head: "store", want: "storeStub"},
			{name: "keeps the case of the head", tail: "stub", head: "Store", want: "StoreStub"},
			{name: "raises a first letter of several bytes", tail: "éclair", head: "store", want: "storeÉclair"},
			{name: "returns the head for an empty tail", tail: "", head: "store", want: "store"},
			{name: "returns the tail for an empty head", tail: "stub", head: "", want: "stub"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, (&eidos.Emitter{}).JoinName(tt.tail, tt.head), tt.want,
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

	t.Run("Supertype", func(t *testing.T) {
		t.Parallel()

		// supertype runs one phase call over f, whose handler translates a
		// reference to the session struct as a supertype of expires.
		supertype := func(t *testing.T, f translationFixture) (*emit.TypeRef, bool) {
			t.Helper()

			var (
				got *emit.TypeRef
				ok  bool
			)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, e *eidos.Emitter) error {
					got, ok = e.Supertype(f.expires.ID, &node.TypeRef{Spelling: sessionName, Target: f.session.ID})
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(f.ctx), "the phase call passes")
			return got, ok
		}

		t.Run("returns the supertype spelling of a backend that declares one", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.ctx.Types = &superSpoke{}
			got, ok := supertype(t, f)
			assert.True(t, ok, "the backend spells the supertype")
			assert.Equal(t, got.Spelling, superPrefix+sessionName, "the spelling is the supertype spelling")
		})

		t.Run("returns the spelling of a value for a backend without a supertype spelling", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := supertype(t, f)
			assert.True(t, ok, "the spoke spells the reference")
			assert.Equal(t, got.Spelling, sessionName, "the spelling is the spoke's")
		})
	})

	t.Run("Type", func(t *testing.T) {
		t.Parallel()

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

		t.Run("passes the spoke the shape of a type parameter with the identity of the parameter",
			func(t *testing.T) {
				t.Parallel()

				f := newTranslationFixture(t)
				param := coretest.MemberID(coretest.StorePath, sessionName, paramSpelling, symbol.KindTypeParam)
				f.translate(t, f.expires.ID, &node.TypeRef{Spelling: paramSpelling, Target: param})
				expect.Equal(t, f.spoke.last.Form, symbol.FormOpaque, "the argument of a parameter decides its shape")
				expect.Equal(t, f.spoke.last.Ref, param, "the shape has the identity of the parameter")
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
				Generator()
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
				Generator()
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
				Generator()
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
				Generator()
			read := widthReads(t, journaledGenerate(t, f.ctx, p))
			assert.Permutation(t, read, []symbol.Identity{f.session.ID, coretest.PackageID(coretest.StorePath)},
				"the invocation reads the subject and the package, each once")
		})

		t.Run("returns false for a shape that the spoke refuses", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := f.translate(t, f.expires.ID, streamRef)
			assert.False(t, ok, "the spoke refuses a stream")
			assert.Nil(t, got, "a refusal returns no reference")
		})

		t.Run("reports RefusedType at owner's position", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.translate(t, f.expires.ID, streamRef)
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
					"eidos: the plan's backend does not implement a type spoke",
				"the message is about the missing spoke")
		})
	})

	t.Run("Fields", func(t *testing.T) {
		t.Parallel()

		// required keeps the fields of the session without the optional note,
		// which the spoke of the fixture cannot spell, and optional keeps the
		// note alone.
		required := func(f *node.Field) bool { return !f.Optional }
		optional := func(f *node.Field) bool { return f.Optional }

		t.Run("returns an emit field for each field that keep reports", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := f.lowerFields(t, required)
			assert.True(t, ok, "the spoke spells the int and the string")
			assert.Equal(t, got, []*emit.Field{
				{
					Origin: f.expires.ID, Name: expiresName, Visibility: symbol.VisibilityPublic,
					Type: &emit.TypeRef{Spelling: string(narrow)},
				},
				{
					Origin: f.count.ID, Doc: []string{countDoc}, Name: countName, Visibility: symbol.VisibilityPublic,
					Type: &emit.TypeRef{Spelling: textSpelling},
				},
			}, "Fields returns the expires and count fields in order, with their translated types")
		})

		t.Run("translates every field for a nil keep", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			_, ok := f.lowerFields(t, nil)
			assert.False(t, ok, "a nil keep translates the note, which the spoke cannot spell")
		})

		t.Run("returns no fields for a field without a spelling", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := f.lowerFields(t, optional)
			assert.False(t, ok, "the spoke cannot spell the optional note")
			assert.Nil(t, got, "Fields returns no fields after a refusal")
		})

		t.Run("reports RefusedType at the position of the field without a spelling", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.lowerFields(t, optional)
			one := onlyFinding(t, f.ctx.Sink)
			expect.Equal(t, one.Code, eidos.RefusedType, "the refusal reports under the code of a refused type")
			expect.Equal(t, one.Pos, f.note.Pos, "the finding is at the field without a spelling")
		})
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		// typeParams runs one phase call over f, whose handler translates tps
		// as the type parameters of the session struct.
		typeParams := func(t *testing.T, f translationFixture, tps []*node.TypeParam) ([]*emit.TypeParam, bool) {
			t.Helper()

			var (
				got []*emit.TypeParam
				ok  bool
			)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, e *eidos.Emitter) error {
					got, ok = e.TypeParams(f.session.ID, tps)
					return nil
				})).
				Generator()
			assert.NoError(t, generatorOf(t, p).Generate(f.ctx), "the phase call passes")
			return got, ok
		}

		t.Run("returns nil for no type parameter", func(t *testing.T) {
			t.Parallel()

			got, ok := typeParams(t, newTranslationFixture(t), nil)
			assert.True(t, ok, "no parameter has nothing to refuse")
			assert.Nil(t, got, "the translation of no parameter is nil")
		})

		t.Run("returns a parameter with each bound in the supertype spelling of the target", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			f.ctx.Types = &superSpoke{}
			bound := &node.TypeRef{Spelling: sessionName, Target: f.session.ID}
			got, ok := typeParams(t, f, []*node.TypeParam{{
				Name: paramSpelling, Variance: symbol.VarianceOut, Bounds: []*node.TypeRef{bound},
			}})
			assert.True(t, ok, "the backend spells the bound")
			assert.Equal(t, got, []*emit.TypeParam{{
				Name:     paramSpelling,
				Variance: symbol.VarianceOut,
				Bounds:   []*emit.TypeRef{{Spelling: superPrefix + sessionName, Target: f.session.ID}},
			}}, "the parameter keeps its name and its variance, and its bound has the supertype spelling")
		})

		t.Run("returns a parameter with its default in the spelling of a type", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := typeParams(t, f, []*node.TypeParam{{
				Name: paramSpelling, Default: &node.TypeRef{Spelling: intSpelling},
			}})
			assert.True(t, ok, "the spoke spells the default")
			want := []*emit.TypeParam{{Name: paramSpelling, Default: &emit.TypeRef{Spelling: string(narrow)}}}
			assert.Equal(t, got, want, "the default translates as a type under the policy of the plan")
		})

		t.Run("returns a const parameter with the type of its value in the spelling of a type", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			got, ok := typeParams(t, f, []*node.TypeParam{{
				Name: constName, Const: true, Type: &node.TypeRef{Spelling: stringSpelling}, DefaultValue: constDefault,
			}})
			assert.True(t, ok, "the spoke spells the type of the value")
			assert.Equal(t, got, []*emit.TypeParam{{
				Name: constName, Const: true, Type: &emit.TypeRef{Spelling: textSpelling}, DefaultValue: constDefault,
			}}, "the parameter keeps its const mark and the spelling of its default value")
		})

		tests := []struct {
			name string
			give *node.TypeParam
		}{
			{
				name: "returns no parameters for a bound without a spelling",
				give: &node.TypeParam{Name: paramSpelling, Bounds: []*node.TypeRef{streamRef}},
			},
			{
				name: "returns no parameters for a default without a spelling",
				give: &node.TypeParam{Name: paramSpelling, Default: streamRef},
			},
			{
				name: "returns no parameters for the type of a value without a spelling",
				give: &node.TypeParam{Name: constName, Const: true, Type: streamRef},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := typeParams(t, newTranslationFixture(t), []*node.TypeParam{tt.give})
				assert.False(t, ok, "the spoke refuses a stream")
				assert.Nil(t, got, "a refusal returns no parameters")
			})
		}

		t.Run("reports RefusedType at the position of owner", func(t *testing.T) {
			t.Parallel()

			f := newTranslationFixture(t)
			typeParams(t, f, []*node.TypeParam{{Name: paramSpelling, Bounds: []*node.TypeRef{streamRef}}})
			one := onlyFinding(t, f.ctx.Sink)
			expect.Equal(t, one.Code, eidos.RefusedType, "the refusal reports under the code of a refused type")
			expect.Equal(t, one.Pos, f.session.Pos, "the finding is at the declaration of the parameters")
		})
	})

	t.Run("Handler", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the method of the binder of the plan's backend", func(t *testing.T) {
			t.Parallel()

			binder := &binderSpoke{method: &emit.Method{Name: handlerDoc}}
			f := newHandlerFixture(t, binder)
			got, ok := f.handle(t, f.rpc)
			assert.True(t, ok, "the binder serves the rpc")
			assert.Equal(t, got, binder.method, "Handler returns the method of the binder", assert.ByIdentity())
		})

		t.Run("passes the binder the rpc with the interface of its service", func(t *testing.T) {
			t.Parallel()

			binder := &binderSpoke{method: &emit.Method{}}
			f := newHandlerFixture(t, binder)
			f.handle(t, f.rpc)
			expect.Equal(t, binder.rpc.ID, f.rpc.ID, "the binder serves the rpc of the call")
			expect.Equal(t, binder.rpc.Service, f.store.ID, "the service is the interface that declares the rpc")
			expect.Equal(t, binder.rpc.Doc, []string{handlerDoc}, "the binder receives the documentation of the rpc")
			expect.Equal(t, binder.rpc.Visibility, f.rpc.Visibility, "the binder receives the visibility of the rpc")
			expect.Equal(t, binder.rpc.Callable.Name, f.rpc.Name, "the projection has the name of the rpc")
		})

		t.Run("passes the interface of the owner chain of a nested rpc as its service", func(t *testing.T) {
			t.Parallel()

			binder := &binderSpoke{method: &emit.Method{}}
			f := newHandlerFixture(t, binder)
			id := f.rpc.ID
			id.Owner = nestedOwner
			f.handle(t, &node.Method{ID: id, Name: f.rpc.Name, Params: f.rpc.Params})
			assert.Equal(t, binder.rpc.Service, symbol.Identity{
				Lang: f.rpc.ID.Lang, Package: f.rpc.ID.Package, Owner: outerName, Name: serviceName,
				Kind: symbol.KindInterface,
			}, "the service is the last name of the owner chain under the rest of the chain")
		})

		t.Run("spells a reference under the override of the policy on the rpc", func(t *testing.T) {
			t.Parallel()

			binder := &binderSpoke{method: &emit.Method{}}
			f := newHandlerFixture(t, binder)
			claim := meta.Claim{Subject: f.rpc.ID, Authority: meta.AuthorityDirective}
			assert.NoError(t, meta.Stamp(f.ctx.Facts, f.width, string(wide), claim), "the override stamps")
			f.handle(t, f.rpc)
			assert.Equal(t, binder.spelled, &emit.TypeRef{Spelling: string(wide)},
				"the spoke spells the parameter under the override on the rpc")
		})

		t.Run("returns the canonical method for a backend without a binder", func(t *testing.T) {
			t.Parallel()

			f := newHandlerFixture(t, &tsxSpoke{})
			got, ok := f.handle(t, f.rpc)
			assert.True(t, ok, "the canonical method serves the rpc")
			expect.Equal(t, got.Origin, f.rpc.ID, "the method derives from the rpc")
			expect.Equal(t, got.Name, f.rpc.Name, "the method has the name of the rpc")
			expect.Equal(t, got.Params.Items(), []*emit.Param{
				{Name: coretest.ParamName, Type: &emit.TypeRef{Spelling: string(narrow)}},
			}, "the method takes the parameter of the rpc in the spelling of the spoke")
			expect.Equal(t, got.Returns.Items(), []*emit.Return{
				{Name: coretest.ReturnName, Type: &emit.TypeRef{Spelling: textSpelling}},
			}, "the method returns the return of the rpc in the spelling of the spoke")
		})

		t.Run("reports RefusedHandler at the rpc for a refusal of the binder", func(t *testing.T) {
			t.Parallel()

			f := newHandlerFixture(t, &binderSpoke{err: errUnspelled})
			got, ok := f.handle(t, f.rpc)
			assert.False(t, ok, "the binder refuses the rpc")
			assert.Nil(t, got, "a refusal returns no method")
			one := onlyFinding(t, f.ctx.Sink)
			expect.Equal(t, one.Code, eidos.RefusedHandler, "the refusal reports under the code of a refused handler")
			expect.Equal(t, one.Severity, diag.SeverityError, "the refusal fails the plan")
			expect.Equal(t, one.Pos, f.rpc.Pos, "the finding is at the rpc")
			expect.Equal(t, one.Msg, "the tsx target cannot serve the rpc "+f.rpc.ID.String()+": "+errUnspelled.Error(),
				"the message contains the target, the rpc and the reason of the binder")
		})

		t.Run("reports RefusedHandler for a backend without a type spoke", func(t *testing.T) {
			t.Parallel()

			f := newHandlerFixture(t, nil)
			_, ok := f.handle(t, f.rpc)
			assert.False(t, ok, "a plan without a spoke serves no rpc")
			one := onlyFinding(t, f.ctx.Sink)
			expect.Equal(t, one.Code, eidos.RefusedHandler, "the refusal reports under the code of a refused handler")
			expect.Contains(t, one.Msg, "does not implement a type spoke", "the message is about the missing spoke")
		})

		t.Run("reports RefusedHandler for a parameter without a type", func(t *testing.T) {
			t.Parallel()

			f := newHandlerFixture(t, &tsxSpoke{})
			f.rpc.Params[0].Type = nil
			_, ok := f.handle(t, f.rpc)
			assert.False(t, ok, "the canonical method cannot spell a parameter without a type")
			assert.Contains(t, onlyFinding(t, f.ctx.Sink).Msg, "has no type",
				"the message is about the untyped parameter")
		})

		t.Run("returns false for a nil rpc", func(t *testing.T) {
			t.Parallel()

			f := newHandlerFixture(t, &tsxSpoke{})
			got, ok := f.handle(t, nil)
			assert.False(t, ok, "no rpc has no handler")
			assert.Nil(t, got, "Handler returns no method for no rpc")
			assert.Empty(t, coretest.Codes(f.ctx.Sink), "Handler reports nothing for no rpc")
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
				Generator()
			assert.Panics(t, func() { _ = generatorOf(t, p).Generate(ctx) },
				"a view of no slot would append nowhere")
		})

		t.Run("is the only path into a slot of a value in the store", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, _ *eidos.Emitter) error {
					host, held := m.Value.(*emit.Struct)
					assert.True(t, held, "a struct rule receives structs")
					host.Fields.Append(&emit.Field{Name: "unattributed"})
					return nil
				})).
				Generator()
			assert.Panics(t, func() { _ = generatorOf(t, p).Generate(ctx) },
				"a direct append into the host panics, because the store sealed its slots")
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
	assert.Equal(t, joined, "storeStub", "JoinName joins the tail after the head")
	assert.MaxAllocs(t, func() { joined = e.JoinName("", "store") }, 0,
		"JoinName allocates nothing for an empty tail")
	assert.Equal(t, joined, "store", "JoinName returns the head for an empty tail")
}

// BenchmarkEmitter measures a phase call over the invocation fixture
// for each method of the emitter, its handle and its slot view, each
// called once per invocation, and the join of a name.
func BenchmarkEmitter(b *testing.B) {
	benchPhases(b, emitterCases(b))

	e := &eidos.Emitter{}
	b.Run("JoinName", func(b *testing.B) {
		b.Run("a tail and a head", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(joinNameAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = e.JoinName("stub", "store")
			}
			assert.Equal(b, got, "storeStub", "JoinName joins the tail after the head")
		})

		b.Run("an empty tail", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got = e.JoinName("", "store")
			}
			assert.Equal(b, got, "store", "JoinName returns the head for an empty tail")
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
		Generator()
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
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	var earlier plugin.Unit
	for u := range ctx.Emit.Units() {
		if u.Plugin == "earlier" {
			earlier = u
		}
	}
	return host, earlier
}

// orderedUnit runs a generator over a package of structs whose handler
// appends a declaration For<Name> of each struct into one package file. It
// returns the one unit of the run.
func orderedUnit(tb assert.TB, structs ...*node.Struct) plugin.Unit {
	tb.Helper()

	decls := make([]symbol.Symbol, 0, len(structs))
	for _, s := range structs {
		decls = append(decls, s)
	}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, decls...)), "the fixture package is admitted")
	g.Freeze()
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	p := eidos.NewPlugin(contextPlugin).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "audit"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.PackageFile().Append(&emit.Struct{Origin: m.Struct.ID, Name: "For" + m.Struct.Name})
			return nil
		})).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	units := slices.Collect(ctx.Emit.Units())
	assert.Length(tb, units, 1, "the package file assembles one unit")
	return units[0]
}

// placedUnit runs a generator over the fixture graph whose handler
// appends the declarations that place returns for its subject into one
// package file. It returns the one unit of the run.
func placedUnit(tb assert.TB, place func(m *eidos.StructMatch) []symbol.Symbol) plugin.Unit {
	tb.Helper()

	g, _, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	p := eidos.NewPlugin(contextPlugin).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "probe"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.PackageFile().Append(place(m)...)
			return nil
		})).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	units := slices.Collect(ctx.Emit.Units())
	assert.Length(tb, units, 1, "the package file assembles one unit")
	return units[0]
}

// declNames returns the names of the emit structs of u, in the order of
// the unit.
func declNames(tb assert.TB, u plugin.Unit) []string {
	tb.Helper()

	names := make([]string, 0, len(u.Decls))
	for _, d := range u.Decls {
		s, ok := d.(*emit.Struct)
		assert.True(tb, ok, "the unit contains what the handler appended")
		names = append(names, s.Name)
	}
	return names
}

// fileOfRun runs a generator over the fixture graph whose handler appends a
// declaration of the first struct through the accumulator that FileOf
// returns for give. It returns the context of the run and the first
// struct.
func fileOfRun(tb assert.TB, give symbol.Identity) (*plugin.GeneratorContext, *node.Struct) {
	tb.Helper()

	g, alpha, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	p := eidos.NewPlugin("perfile").
		Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if m.Struct.ID == alpha.ID {
				e.FileOf(give).Append(&emit.Struct{Origin: alpha.ID, Name: "Gen" + alpha.Name})
			}
			return nil
		})).
		Generator()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	return ctx, alpha
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
			Generator())
	}
	weaver := func(h func(*emit.Struct, *eidos.Emitter)) plugin.Generator {
		return generatorOf(tb, eidos.NewPlugin(contextPlugin).
			Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
				s, held := m.Value.(*emit.Struct)
				assert.True(tb, held, "a struct rule receives structs")
				h(s, e)
				return nil
			})).
			Generator())
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
	placingContext := func() *plugin.GeneratorContext {
		ctx := fresh()
		ctx.Placer = placing{}
		return ctx
	}
	scalar := &node.TypeRef{Spelling: intSpelling}
	var typed *emit.TypeRef
	lowered := []*node.Field{{
		ID: coretest.MemberID(coretest.StorePath, sessionName, expiresName, symbol.KindField), Name: expiresName,
		Type: scalar,
	}}
	var translated []*emit.Field
	bounded := []*node.TypeParam{{Name: paramSpelling, Bounds: []*node.TypeRef{scalar}}}
	var params []*emit.TypeParam
	return []phaseCase{
		{
			name: "File", allocs: unitAllocs, fresh: fresh, check: oneUnit(plugin.PerSource, 0),
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.File() }),
		},
		{
			name: "FileOf", allocs: fileOfAllocs, fresh: fresh, check: oneUnit(plugin.PerSource, 0),
			gen: families(func(m *eidos.StructMatch, e *eidos.Emitter) { e.FileOf(m.Struct.ID) }),
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
			name: "Out.Package", allocs: unitAllocs, fresh: placingContext, check: oneUnit(plugin.PerSource, 0),
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) {
				_, err := e.File().Package()
				assert.NoError(tb, err, "the preview of the fixture routes")
			}),
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
			name: "Fields", allocs: fieldsAllocs, fresh: translating,
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { translated, _ = e.Fields(lowered, nil) }),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				assert.Length(tb, translated, 1, "Fields returns the one field")
				spelled := translated[0].Type.Spelling
				assert.Equal(tb, spelled, string(narrow), "the spoke spells the scalar under the policy")
				assert.False(tb, ctx.Sink.Failed(), "no translation is refused")
			},
		},
		{
			name:   "TypeParams",
			allocs: typeParamsAllocs,
			fresh:  translating,
			gen: families(
				func(m *eidos.StructMatch, e *eidos.Emitter) { params, _ = e.TypeParams(m.Struct.ID, bounded) },
			),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				assert.Length(tb, params, 1, "TypeParams returns the one parameter")
				assert.Equal(tb, params[0].Bounds, []*emit.TypeRef{{Spelling: string(narrow)}},
					"the spoke spells the bound under the policy")
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
