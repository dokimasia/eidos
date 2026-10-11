// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"embed"
	"fmt"
	"reflect"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/defaults"
	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "builder"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// BuilderDirective is the directive that puts a struct under builder.
// DefaultsParam is its param that names a function in scope, whose result
// seeds the value of each new builder.
const (
	BuilderDirective directive.Name     = "builder"
	DefaultsParam    directive.ParamKey = "defaults"
)

// Word is the word of the family of the builders, and TestTag the tag of
// the family of their tests.
const (
	Word    = "builder"
	TestTag = "test"
)

// DefaultsSlot is the named slot of the body of New<T>. It contains the
// call of the function of [DefaultsParam] and the assignment of each
// declared default.
const DefaultsSlot = "defaults"

// The words that the names of the builder, its constructors, its setters,
// its method Build and the checks of the defaults join onto the names of
// the struct and its fields.
const (
	builderWord = "Builder"
	newWord     = "New"
	fromWord    = "From"
	withWord    = "With"
	appendWord  = "Append"
	stringWord  = "String"
	entryWord   = "Entry"
	entriesWord = "Entries"
	buildName   = "Build"
	defaultWord = "Default"
)

// The field of a builder that contains the value, the locals that the
// tests bind, the parameters of the setters, the field that no setter
// sets, and the Go spellings that the plugin writes.
const (
	valueField   = "value"
	seedLocal    = "seed"
	wantLocal    = "want"
	valueParam   = "v"
	textParam    = "s"
	keyParam     = "k"
	keysParam    = "keys"
	entriesParam = "entries"
	blankField   = "_"
	selector     = "."
	pointerMark  = "*"
	stringType   = "string"
	emptyStruct  = "struct{}"
	argsOpen     = "["
	argsClose    = "]"
	argsSep      = ", "
)

// The templates of the bodies of the setters and the constructors, and of
// each test file.
const (
	withTemplate    = "with.tmpl"
	appendTemplate  = "append.tmpl"
	textTemplate    = "text.tmpl"
	entryTemplate   = "entry.tmpl"
	entriesTemplate = "entries.tmpl"
	keyTemplate     = "key.tmpl"
	keysTemplate    = "keys.tmpl"
	pointTemplate   = "point.tmpl"
	newTemplate     = "new.tmpl"
	fromTemplate    = "from.tmpl"
	testTemplate    = "builder.test.tmpl"
)

// The documentation of the declarations. Each format takes the names that
// its declaration states.
const (
	builderDoc = "%s builds a value of %s. Each setter returns the builder."
	newDoc     = "%s returns a builder of %s that starts from the declared defaults."
	fromDoc    = "%s returns a builder that starts from v."
	buildDoc   = "Build returns the value that the builder built."
	setDoc     = "%s sets %s to v."
	replaceDoc = "%s replaces %s with the elements of v."
	appendDoc  = "%s appends the elements of v to %s."
	textDoc    = "%s sets %s to the bytes of s."
	entryDoc   = "%s sets the value of the key k of %s to v."
	entriesDoc = "%s copies the entries into %s."
	keyDoc     = "%s adds the key k to %s."
	keysDoc    = "%s adds the keys to %s."
	pointDoc   = "%s sets %s to the address of v."
)

// form is the form of the type of a field, which decides the setters of
// the field.
type form uint8

// The forms of the type of a field: a scalar, a sequence, bytes, a set, a
// mapping and an optional.
const (
	formScalar   form = 1
	formSequence form = 2
	formBytes    form = 3
	formSet      form = 4
	formMapping  form = 5
	formOptional form = 6
)

// schema is the directive of builder.
var schema = directive.Schema{
	Plugin: string(ID),
	Name:   BuilderDirective,
	Params: []directive.ParamSpec{{
		Key: DefaultsParam, Type: directive.TypeReference, Resolution: directive.ResolveCallableInScope,
		Doc: "the function whose result seeds the value of each new builder",
	}},
	Doc: "generates a builder of the struct, with a setter for each field and checks of each",
}

// templates contains the templates of builder in one directory for each
// target.
//
//go:embed templates
var templates embed.FS

// member is the data that the template of a setter reads. Field is the
// name of the field that the setter sets. Type is the type of the field,
// and Elem is the type of the values of a set.
type member struct {
	Field string
	Type  *emit.TypeRef
	Elem  *emit.TypeRef
}

// seeding is the data that the template of New<T> reads. Seeded reports
// that the slot of the defaults declares the value through the function of
// [DefaultsParam]. Value is the type of the value.
type seeding struct {
	Seeded bool
	Value  *emit.TypeRef
}

// setterCheck is one check of a setter in a test file. Field is the field
// that the setter sets, and Pointer reports that the field points at the
// value of the setter.
type setterCheck struct {
	Field   string
	Pointer bool
}

// names are the names of the builder of a struct and of its constructors.
type names struct {
	builder string
	ctor    string
	from    string
}

// checks is the data that the test template reads beside the record of a
// builder. Value is the struct at the witnesses of its type parameters, and
// Args contains the witnesses. Defaults contains the field of each check of
// a default, and Setters each check of a setter, in the order of the
// checks.
type checks struct {
	Value    *emit.TypeRef
	Args     []*emit.TypeRef
	Defaults []string
	Setters  []setterCheck
}

// New returns builder. For each struct under [BuilderDirective], it writes
// the builder <T>Builder, the constructors New<T> and New<T>From, a setter
// for each field and the method Build, into the family [Word]. The form of
// the type of a field decides its setters:
//
//   - A scalar has With<F>.
//   - A sequence has With<F> and Append<F>, both variadic.
//   - Bytes have With<F> and With<F>String.
//   - A mapping has With<F>, With<F>Entry and With<F>Entries.
//   - A set, a map of empty structs, has With<F>, With<F>Entry and
//     With<F>Entries. The last two take keys.
//   - An optional has With<F>, which takes the value and points the field
//     at it.
//
// The setters and the constructors have template bodies. New<T> seeds the
// value with the function of [DefaultsParam] where the directive sets it.
// It then assigns each default that the plugin defaults stamps on a field,
// so builder requires the capability of defaults. The family [Word] under
// the tag [TestTag] claims a test file for each source file. The file
// checks the constructors and the setters with the samples of the fields.
// A generic struct is checked at the witnesses of its type parameters, and
// has no checks where a parameter has no witness. A field whose type refers
// to a type parameter has no checks.
//
// builder reports [BadDefault] at a field whose default is not a literal of
// its type, and writes nothing for the struct. The plugin serves the target
// golang alone. Each call returns a new plugin, because a plugin instance
// belongs to one workspace.
func New() plugin.Generator {
	g, _ := sdk.NewPlugin(ID).
		Version(Version).
		Requires(defaults.Capability).
		Output(plugin.Output{Per: plugin.PerSource, Word: Word}).
		Output(plugin.Output{Tag: TestTag, Per: plugin.PerSource, Word: Word, Template: testTemplate}).
		For(golang.Target, sdk.Templates(tree.Of(templates, golang.Target))).
		Handle(sdk.Directive(schema, sdk.OnStruct(generate))).
		Build().(plugin.Generator)
	return g
}

// generate writes the builder of one struct into the family of the
// builders, and records its checks into the family of the tests.
func generate(m *sdk.StructMatch, e *sdk.Emitter) error {
	s := m.Struct
	b := m.Rules()
	seed, seeded := m.Directive().Params[DefaultsParam]
	stmts := make([]emit.Stmt, 0, len(s.Fields)+1)
	if seeded {
		call := emit.RefExpr(seed.Target)
		stmts = append(stmts, emit.Stmt{
			Kind: emit.StmtAssign, Names: []string{valueField}, Declare: true,
			Value: emit.Expr{Kind: emit.ExprCall, Fn: &call},
		})
	}
	values := map[*node.Field]emit.Value{}
	for _, f := range fields(s) {
		text, declared := sdk.FactOf(m, f.ID, meta.Named[string](defaults.KeyValue))
		if !declared {
			continue
		}
		value, lifted := lift(b, f, text)
		if !lifted {
			m.ErrorfAt(
				BadDefault,
				f.Pos,
				"the default %q of %s is no literal of its type %s",
				text,
				f.Name,
				f.Type.Spelling,
			)
			return nil
		}
		values[f] = value
		stmts = append(stmts, emit.Stmt{
			Kind: emit.StmtAssign, Names: []string{valueField + selector + f.Name}, Value: emit.ValueExpr(value),
		})
	}

	name := e.JoinName(builderWord, s.Name)
	newName := e.JoinName(s.Name, newWord)
	fromName := e.JoinName(fromWord, newName)
	args := typeArgs(s)
	builder := &emit.Struct{
		Origin:     s.ID,
		Doc:        []string{fmt.Sprintf(builderDoc, name, s.Name)},
		Name:       name,
		Visibility: s.Visibility,
		TypeParams: typeParams(s),
	}
	builder.Fields.Append(&emit.Field{
		Origin: s.ID, Name: valueField, Visibility: symbol.VisibilityPackage,
		Type: &emit.TypeRef{Spelling: s.Name, Target: s.ID, Args: args},
	})
	for _, f := range fields(s) {
		builder.Methods.Append(setters(b, e, s, name, f)...)
	}
	build := golang.PointerReceiver(&emit.Method{
		Origin:     s.ID,
		Doc:        []string{buildDoc},
		Name:       buildName,
		Visibility: s.Visibility,
		Receives:   &emit.TypeRef{Spelling: name, Args: args},
		Returns:    []*emit.Return{{Type: &emit.TypeRef{Spelling: s.Name, Target: s.ID, Args: args}}},
	})
	build.Body.Stmts = []emit.Stmt{{
		Kind: emit.StmtReturn, Value: emit.Expr{Kind: emit.ExprName, Name: build.Receiver.Name + selector + valueField},
	}}
	builder.Methods.Append(build)

	ctor := &emit.Function{
		Origin:     s.ID,
		Doc:        []string{fmt.Sprintf(newDoc, newName, s.Name)},
		Name:       newName,
		Visibility: s.Visibility,
		TypeParams: typeParams(s),
		Returns:    []*emit.Return{{Type: pointerTo(&emit.TypeRef{Spelling: name, Args: args})}},
	}
	ctor.Body.Declare(DefaultsSlot)
	slot, _ := ctor.Body.Slot(DefaultsSlot)
	slot.Append(stmts...)
	ctor.Body.Ref = e.Ref(
		newTemplate,
		seeding{Seeded: seeded, Value: &emit.TypeRef{Spelling: s.Name, Target: s.ID, Args: args}},
	)
	from := &emit.Function{
		Origin:     s.ID,
		Doc:        []string{fmt.Sprintf(fromDoc, fromName)},
		Name:       fromName,
		Visibility: s.Visibility,
		TypeParams: typeParams(s),
		Params:     []*emit.Param{{Name: valueParam, Type: &emit.TypeRef{Spelling: s.Name, Target: s.ID, Args: args}}},
		Returns:    []*emit.Return{{Type: pointerTo(&emit.TypeRef{Spelling: name, Args: args})}},
	}
	from.Body.Ref = e.Ref(fromTemplate, nil)
	e.File().Append(builder, ctor, from)

	if test, data, checked := tests(m, e, names{builder: name, ctor: newName, from: fromName}, values); checked {
		e.File(TestTag).Record(data, test)
	}
	return nil
}

// tests returns the record of the checks of the builder of the struct of m,
// with its data. The record is a copy of the builder, which the settle
// respells as it respells the builder. Its first method is Build, and each
// further method is a setter with a check, whose body binds the sample of
// the check to want. Its first nested function is New<T>, and its second is
// New<T>From, whose body seeds each field with a sample. Each further
// nested function is the check of one default, whose body binds the default
// to want. A field whose type refers to a type parameter of the struct has
// no checks. tests reports false for a generic struct whose type parameters
// have no witnesses.
func tests(
	m *sdk.StructMatch,
	e *sdk.Emitter,
	n names,
	values map[*node.Field]emit.Value,
) (*emit.Struct, checks, bool) {
	s := m.Struct
	b := m.Rules()
	generics, substitutes := b.Source().(rules.GenericsRules)
	witnesses := b.Witnesses(s.TypeParams)
	if !substitutes || len(s.TypeParams) > 0 && witnesses == nil {
		return nil, checks{}, false
	}
	args := make([]*emit.TypeRef, 0, len(witnesses))
	for _, w := range witnesses {
		args = append(args, rules.EmitRef(w))
	}
	data := checks{Value: &emit.TypeRef{Spelling: s.Name, Target: s.ID, Args: args}, Args: args}
	test := &emit.Struct{Origin: s.ID, Name: n.builder, Visibility: s.Visibility}
	test.Methods.Append(&emit.Method{Origin: s.ID, Name: buildName, Visibility: s.Visibility})
	from := &emit.Function{Origin: s.ID, Name: n.from, Visibility: s.Visibility}
	test.Types.Append(&emit.Function{Origin: s.ID, Name: n.ctor, Visibility: s.Visibility}, from)
	for _, f := range fields(s) {
		ref := f.Type
		if generics.Substitute(ref, s.TypeParams, witnesses) != ref {
			continue
		}
		if sample, _ := b.SamplesOf(f.ID, ref, f.Name); sample.OK() {
			from.Body.Stmts = append(from.Body.Stmts, emit.Stmt{
				Kind:  emit.StmtAssign,
				Names: []string{seedLocal + selector + f.Name},
				Value: emit.ValueExpr(sample.Value),
			})
		}
		if value, declared := values[f]; declared {
			if zero, has := b.ZeroValue(f.Type); !has || !reflect.DeepEqual(value, zero) {
				test.Types.Append(&emit.Function{
					Origin: s.ID, Name: e.JoinName(defaultWord, f.Name), Visibility: s.Visibility,
					Params: []*emit.Param{{Name: wantLocal, Type: rules.EmitRef(ref)}},
					Body: emit.Body{Stmts: []emit.Stmt{{
						Kind: emit.StmtAssign, Names: []string{wantLocal}, Value: emit.ValueExpr(value),
					}}},
				})
				data.Defaults = append(data.Defaults, f.Name)
			}
		}
		param := &emit.Param{Name: valueParam, Type: rules.EmitRef(ref)}
		sampled, pointer := ref, false
		switch formOf(b, f) {
		case formSequence:
			param = &emit.Param{
				Name:     valueParam,
				Type:     rules.EmitRef(ref.Elems[0]),
				Variadic: symbol.VariadicPositional,
			}
		case formOptional:
			sampled, pointer = ref.Elems[0], true
			param = &emit.Param{Name: valueParam, Type: rules.EmitRef(sampled)}
		default:
			// The setter With<F> of a scalar, bytes, a set and a mapping takes
			// the type of the field.
		}
		if sample, _ := b.SamplesOf(f.ID, sampled, f.Name); sample.OK() {
			test.Methods.Append(&emit.Method{
				Origin: s.ID, Name: e.JoinName(f.Name, withWord), Visibility: s.Visibility,
				Params: []*emit.Param{param},
				Body: emit.Body{Stmts: []emit.Stmt{{
					Kind: emit.StmtAssign, Names: []string{wantLocal}, Value: emit.ValueExpr(sample.Value),
				}}},
			})
			data.Setters = append(data.Setters, setterCheck{Field: f.Name, Pointer: pointer})
		}
	}
	return test, data, true
}

// fields returns the fields of s that a builder sets: each declared field
// with a name other than the blank one.
func fields(s *node.Struct) []*node.Field {
	out := make([]*node.Field, 0, len(s.Fields))
	for _, f := range s.Fields {
		if f.Name != blankField {
			out = append(out, f)
		}
	}
	return out
}

// typeParams returns a copy of the type parameters of s in the emit model,
// for one declaration of the builder of s, and nil for a struct without
// type parameters.
func typeParams(s *node.Struct) []*emit.TypeParam {
	if len(s.TypeParams) == 0 {
		return nil
	}
	out := make([]*emit.TypeParam, 0, len(s.TypeParams))
	for _, tp := range s.TypeParams {
		bounds := make([]*emit.TypeRef, 0, len(tp.Bounds))
		for _, bound := range tp.Bounds {
			bounds = append(bounds, rules.EmitRef(bound))
		}
		out = append(out, &emit.TypeParam{
			Name: tp.Name, Variance: tp.Variance, Const: tp.Const, Bounds: bounds,
			Default: rules.EmitRef(tp.Default), Type: rules.EmitRef(tp.Type), DefaultValue: tp.DefaultValue,
		})
	}
	return out
}

// typeArgs returns the type parameters of s as the arguments of a reference
// inside a declaration of the builder of s, and nil for a struct without
// type parameters.
func typeArgs(s *node.Struct) []*emit.TypeRef {
	if len(s.TypeParams) == 0 {
		return nil
	}
	out := make([]*emit.TypeRef, 0, len(s.TypeParams))
	for _, tp := range s.TypeParams {
		out = append(out, &emit.TypeRef{Spelling: tp.Name})
	}
	return out
}

// pointerTo returns a reference to a pointer to t, as Go writes it: the
// pointer mark before the name of t and the arguments of t in brackets.
func pointerTo(t *emit.TypeRef) *emit.TypeRef {
	spelling := t.Spelling
	if len(t.Args) > 0 {
		spelled := make([]string, 0, len(t.Args))
		for _, a := range t.Args {
			spelled = append(spelled, a.Spelling)
		}
		spelling += argsOpen + strings.Join(spelled, argsSep) + argsClose
	}
	return &emit.TypeRef{Spelling: pointerMark + spelling, Form: symbol.FormOptional, Elems: []*emit.TypeRef{t}}
}

// lift returns the value of the text of the default of the field f. The
// text of a field of text is its content, as the text of an authored sample
// is. The text of every other field is a literal of the language of the
// field, which the rules read. lift reports false for a text that is no
// literal of the type of f.
func lift(b rules.Bound, f *node.Field, text string) (emit.Value, bool) {
	if b.TypeOf(f.Type).Form == symbol.FormText {
		return emit.Literal(emit.LiteralString, text), true
	}
	return b.LiteralFor(nil, f.Type, text)
}

// formOf returns the form of the type of the field f. The structure of the
// reference decides it, and the rules tell bytes from a sequence. A map
// whose values are empty structs is a set.
func formOf(b rules.Bound, f *node.Field) form {
	switch f.Type.Form {
	case symbol.FormList:
		if b.TypeOf(f.Type).Form == symbol.FormBytes {
			return formBytes
		}
		return formSequence
	case symbol.FormMap:
		if v := f.Type.Elems[1]; v.Form == symbol.FormInline &&
			strings.Join(strings.Fields(v.Spelling), "") == emptyStruct {

			return formSet
		}
		return formMapping
	case symbol.FormOptional:
		return formOptional
	default:
		return formScalar
	}
}

// setters returns the setters of the field f of the struct s on the builder
// named name, by the form of the type of f.
func setters(b rules.Bound, e *sdk.Emitter, s *node.Struct, name string, f *node.Field) []*emit.Method {
	args := typeArgs(s)
	setter := func(setName, doc, tmpl string, data member, params ...*emit.Param) *emit.Method {
		m := golang.PointerReceiver(&emit.Method{
			Origin:     s.ID,
			Doc:        []string{fmt.Sprintf(doc, setName, f.Name)},
			Name:       setName,
			Visibility: s.Visibility,
			Receives:   &emit.TypeRef{Spelling: name, Args: args},
			Params:     params,
			Returns:    []*emit.Return{{Type: pointerTo(&emit.TypeRef{Spelling: name, Args: args})}},
		})
		m.Body.Ref = e.Ref(tmpl, data)
		return m
	}
	with := e.JoinName(f.Name, withWord)
	set := member{Field: f.Name, Type: rules.EmitRef(f.Type)}
	switch formOf(b, f) {
	case formSequence:
		return []*emit.Method{
			setter(
				with,
				replaceDoc,
				withTemplate,
				set,
				&emit.Param{
					Name:     valueParam,
					Type:     rules.EmitRef(f.Type.Elems[0]),
					Variadic: symbol.VariadicPositional,
				},
			),
			setter(
				e.JoinName(f.Name, appendWord),
				appendDoc,
				appendTemplate,
				set,
				&emit.Param{
					Name:     valueParam,
					Type:     rules.EmitRef(f.Type.Elems[0]),
					Variadic: symbol.VariadicPositional,
				},
			),
		}
	case formBytes:
		return []*emit.Method{
			setter(with, setDoc, withTemplate, set, &emit.Param{Name: valueParam, Type: rules.EmitRef(f.Type)}),
			setter(e.JoinName(stringWord, with), textDoc, textTemplate, set,
				&emit.Param{Name: textParam, Type: &emit.TypeRef{Spelling: stringType}}),
		}
	case formSet:
		keyed := member{Field: f.Name, Type: rules.EmitRef(f.Type), Elem: rules.EmitRef(f.Type.Elems[1])}
		return []*emit.Method{
			setter(with, setDoc, withTemplate, set, &emit.Param{Name: valueParam, Type: rules.EmitRef(f.Type)}),
			setter(e.JoinName(entryWord, with), keyDoc, keyTemplate, keyed,
				&emit.Param{Name: keyParam, Type: rules.EmitRef(f.Type.Elems[0])}),
			setter(
				e.JoinName(entriesWord, with),
				keysDoc,
				keysTemplate,
				keyed,
				&emit.Param{
					Name:     keysParam,
					Type:     rules.EmitRef(f.Type.Elems[0]),
					Variadic: symbol.VariadicPositional,
				},
			),
		}
	case formMapping:
		return []*emit.Method{
			setter(with, setDoc, withTemplate, set, &emit.Param{Name: valueParam, Type: rules.EmitRef(f.Type)}),
			setter(e.JoinName(entryWord, with), entryDoc, entryTemplate, set,
				&emit.Param{Name: keyParam, Type: rules.EmitRef(f.Type.Elems[0])},
				&emit.Param{Name: valueParam, Type: rules.EmitRef(f.Type.Elems[1])}),
			setter(e.JoinName(entriesWord, with), entriesDoc, entriesTemplate, set,
				&emit.Param{Name: entriesParam, Type: rules.EmitRef(f.Type)}),
		}
	case formOptional:
		return []*emit.Method{
			setter(
				with,
				pointDoc,
				pointTemplate,
				set,
				&emit.Param{Name: valueParam, Type: rules.EmitRef(f.Type.Elems[0])},
			),
		}
	default:
		return []*emit.Method{
			setter(with, setDoc, withTemplate, set, &emit.Param{Name: valueParam, Type: rules.EmitRef(f.Type)}),
		}
	}
}
