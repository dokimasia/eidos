// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The names that an implementation declares beside the methods of its
// interface: the field with the value of the interface that each method
// calls, and the receiver of a struct whose name does not start with a
// letter.
const (
	nextField        = "next"
	receiverFallback = "r"
)

// data is the payload of each template reference of a declarative plugin,
// which a body template reads as .Data, and the data of each record of a
// family that claims its files, which a file template reads as the Data
// of the record. Its maps are shared by every reference of one match, so a
// template does not change them.
type data struct {
	// Subject is the name of the subject in its source.
	Subject string
	// Directive contains each param of the gating directive by key. A
	// string and a reference are strings, an int is an int64, a bool is a
	// bool, and a list is a []any of its elements. An optional param that
	// the instance does not write is nil. Directive is nil under a bare
	// rule.
	Directive map[string]any
	// Options contains the options of the plugin by key, with the values
	// that the config populated: a string, an int64, a bool or a []string.
	// It is nil for a plugin without options.
	Options map[string]any
}

// lowered is one rule of a manifest as its handler runs it: the family that
// it emits into, the template of each body, the keys of the params of its
// gating directive, and the options of its plugin.
//
// # Concurrency
//
// A lowered is not changed after the loader builds it, so the workers of a
// phase call run its handlers at the same time.
type lowered struct {
	// word and tag are the word and the tag of the family.
	word string
	tag  eidos.Tag
	// template is the path of the template inside each tree, and empty for
	// a rule whose declaration has no body or whose family claims its
	// files without one.
	template string
	// params are the keys of the params of the gating directive, in the
	// order of the manifest, and nil for a bare rule.
	params []directive.ParamKey
	// options returns the options of the plugin by key, built once.
	options func() map[string]any
	// per is the cardinality of the family.
	per plugin.Cardinality
	// claimed reports that the family claims its files, so the rule emits
	// each declaration as a record with the data of its match.
	claimed bool
}

// function emits the mirror of a function: a function with the type
// parameters, the parameters, the results, the async flag and the announced
// failure types of the subject. The rule's template renders its body where
// the rule has one. It emits nothing where the plan's target cannot spell
// a type of the subject.
func (l *lowered) function(m *eidos.FunctionMatch, e *eidos.Emitter) error {
	f := m.Function
	t := translation{e: e}
	out := &emit.Function{
		Origin:     f.ID,
		Name:       e.JoinName(l.word, f.Name),
		Visibility: f.Visibility,
		Async:      f.Async,
		TypeParams: t.typeParams(f.ID, f.TypeParams),
		Params:     t.params(f.ID, f.Params),
		Returns:    t.returns(f.ID, f.Returns),
		Throws:     t.refs(f.ID, f.Throws),
	}
	if t.refused {
		return nil
	}
	payload := l.payload(m.Directive(), f.Name)
	if l.template != "" {
		out.Body.Ref = e.Ref(l.template, payload)
	}
	l.place(e, payload, out)
	return nil
}

// interfaceMirror emits the mirror of an interface: an interface with the
// subject's type parameters and a mirror of each of its effective methods
// and properties, with the member's documentation. It emits nothing for an
// interface whose members are incomplete, and nothing where the plan's
// target cannot spell a type of the mirror. A target without properties,
// such as Go, refuses a mirror with a property at render.
func (l *lowered) interfaceMirror(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	i := m.Interface
	members, complete := membersOf(m, i, i.Name)
	if !complete {
		return nil
	}
	t := translation{e: e}
	out := &emit.Interface{
		Origin:     i.ID,
		Name:       e.JoinName(l.word, i.Name),
		Visibility: i.Visibility,
		TypeParams: t.typeParams(i.ID, i.TypeParams),
	}
	for _, member := range members {
		switch s := member.Symbol.(type) {
		case *node.Method:
			out.Methods.Append(t.method(s))
		case *node.Field:
			out.Fields.Append(t.field(s))
		}
	}
	if t.refused {
		return nil
	}
	var payload any
	if l.claimed {
		payload = l.payload(m.Directive(), i.Name)
	}
	l.place(e, payload, out)
	return nil
}

// implementation emits the implementation of an interface: a struct with the
// subject's type parameters, the field next of the subject's type, and a
// mirror of each effective method of the subject on a named receiver, whose
// body the rule's template renders. The struct implements the subject. It
// emits nothing for an interface whose members are incomplete, and nothing
// where the plan's target cannot spell a type of the struct.
//
// A body delegates a call to next, and a property has no body. For an
// interface with a property, implementation reports [UndelegatedProperty]
// at the subject with each property, and emits nothing.
func (l *lowered) implementation(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	i := m.Interface
	members, complete := membersOf(m, i, i.Name)
	if !complete {
		return nil
	}
	var properties []string
	for _, member := range members {
		if f, is := member.Symbol.(*node.Field); is {
			properties = append(properties, f.Name)
		}
	}
	if len(properties) > 0 {
		m.Errorf(
			UndelegatedProperty,
			"an implementation of %s cannot delegate its properties, and the plugin emits nothing for it: %s",
			i.Name,
			strings.Join(properties, ", "),
		)
		return nil
	}
	t := translation{e: e}
	name := e.JoinName(l.word, i.Name)
	self := &node.TypeRef{Spelling: i.Name, Target: i.ID}
	for _, tp := range i.TypeParams {
		self.Args = append(self.Args, &node.TypeRef{Spelling: tp.Name, Target: tp.ID})
	}
	out := &emit.Struct{
		Origin:     i.ID,
		Name:       name,
		Visibility: i.Visibility,
		TypeParams: t.typeParams(i.ID, i.TypeParams),
		Implements: []*emit.TypeRef{t.ref(i.ID, self)},
	}
	out.Fields.Append(&emit.Field{Name: nextField, Visibility: symbol.VisibilityPackage, Type: t.ref(i.ID, self)})
	letter := receiverFallback
	if first, _ := utf8.DecodeRuneInString(name); unicode.IsLetter(first) {
		letter = string(unicode.ToLower(first))
	}
	payload := l.payload(m.Directive(), i.Name)
	for _, member := range members {
		// The interface has no property, so each member passes the check.
		if method, is := member.Symbol.(*node.Method); is {
			mirrored := t.method(method)
			// The settle respells the bare reference with the struct, as it
			// respells every reference to a declaration of the package.
			receives := &emit.TypeRef{Spelling: name}
			for _, tp := range out.TypeParams {
				receives.Args = append(receives.Args, &emit.TypeRef{Spelling: tp.Name})
			}
			mirrored.Receiver = &emit.Param{Name: receiverName(letter, mirrored, out.TypeParams), Type: receives}
			if l.template != "" {
				mirrored.Body.Ref = e.Ref(l.template, payload)
			}
			out.Methods.Append(mirrored)
		}
	}
	if t.refused {
		return nil
	}
	l.place(e, payload, out)
	return nil
}

// structMirror emits the mirror of a struct: a struct with the subject's
// type parameters and its effective fields, each with its name, its
// visibility, its documentation and its type. It emits nothing for a struct
// whose members are incomplete, and nothing where the plan's target cannot
// spell a type of the mirror.
func (l *lowered) structMirror(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	members, complete := membersOf(m, s, s.Name)
	if !complete {
		return nil
	}
	t := translation{e: e}
	out := &emit.Struct{
		Origin:     s.ID,
		Name:       e.JoinName(l.word, s.Name),
		Visibility: s.Visibility,
		TypeParams: t.typeParams(s.ID, s.TypeParams),
	}
	for _, member := range members {
		if f, is := member.Symbol.(*node.Field); is {
			out.Fields.Append(t.field(f))
		}
	}
	if t.refused {
		return nil
	}
	var payload any
	if l.claimed {
		payload = l.payload(m.Directive(), s.Name)
	}
	l.place(e, payload, out)
	return nil
}

// place emits decl into the accumulator of the rule's family for the
// match of e: the subject's source file, its package or the plan, by the
// family's cardinality. In a family that claims its files, decl is a
// record with payload as its data, and in any other family it is a
// declaration of the file.
func (l *lowered) place(e *eidos.Emitter, payload any, decl symbol.Symbol) {
	var out *eidos.Out
	switch l.per {
	case plugin.PerPackage:
		out = e.PackageFile(l.tag)
	case plugin.PerPlan:
		out = e.PlanFile(l.tag)
	default:
		out = e.File(l.tag)
	}
	if l.claimed {
		out.Record(payload, decl)
		return
	}
	out.Append(decl)
}

// payload returns the data of the template references of one match: the
// subject's name, the params of the gating instance d, which is nil under a
// bare rule, and the options of the plugin. It allocates the map of the
// params and the boxed data.
func (l *lowered) payload(d *directive.Directive, subject string) any {
	var params map[string]any
	if d != nil {
		params = make(map[string]any, len(l.params))
		for _, k := range l.params {
			params[string(k)] = nil
			if v, written := d.Param(k); written {
				params[string(k)] = valueOf(v)
			}
		}
	}
	return data{Subject: subject, Directive: params, Options: l.options()}
}

// valueOf returns a validated param value as the template data contains it:
// a string, a reference's spelling, an int64, a bool, or a []any of the
// elements of a list.
func valueOf(v directive.Value) any {
	switch v.Kind {
	case directive.TypeInt:
		return v.Int
	case directive.TypeBool:
		return v.Bool
	case directive.TypeReference:
		return v.Ref
	case directive.TypeList:
		out := make([]any, 0, len(v.List))
		for _, e := range v.List {
			out = append(out, valueOf(e))
		}
		return out
	default:
		return v.Str
	}
}

// subject is the part of the match of a type that the handlers read: the
// rules of the subject's language, and the report at the subject.
type subject interface {
	Rules() rules.Bound
	Errorf(c diag.Code, format string, a ...any)
}

// membersOf returns the effective members of the type s that m matched,
// named name in its source. For a set with gaps, it reports
// [IncompleteMembers] at the subject with each contributor that the walk
// could not follow, and returns false.
func membersOf(m subject, s symbol.Symbol, name string) ([]rules.Member, bool) {
	set, _ := m.Rules().MembersOf(s)
	if set.Complete() {
		return set.Members, true
	}
	gaps := make([]string, 0, len(set.Gaps))
	for _, g := range set.Gaps {
		contributor := name
		if g.Contributor != nil {
			contributor = g.Contributor.Spelling
		}
		gaps = append(gaps, contributor+" ("+g.Reason.String()+")")
	}
	m.Errorf(IncompleteMembers, "the members of %s are incomplete, and the plugin emits nothing for it: %s",
		name, strings.Join(gaps, ", "))
	return nil, false
}

// receiverName returns the name of the receiver of the method m of a struct
// with the type parameters tps: letter, or letter followed by the smallest
// number from 0 that no parameter, result or type parameter of m, and no
// type parameter of the struct, has as its name.
func receiverName(letter string, m *emit.Method, tps []*emit.TypeParam) string {
	taken := func(name string) bool {
		return slices.ContainsFunc(m.Params, func(p *emit.Param) bool { return p.Name == name }) ||
			slices.ContainsFunc(m.Returns, func(r *emit.Return) bool { return r.Name == name }) ||
			slices.ContainsFunc(m.TypeParams, func(tp *emit.TypeParam) bool { return tp.Name == name }) ||
			slices.ContainsFunc(tps, func(tp *emit.TypeParam) bool { return tp.Name == name })
	}
	if !taken(letter) {
		return letter
	}
	for i := 0; ; i++ {
		if candidate := letter + strconv.Itoa(i); !taken(candidate) {
			return candidate
		}
	}
}

// translation translates the types of one declaration that a rule emits
// into references of the plan's target. [eidos.Emitter.Type] reports each
// type that the target cannot spell under [eidos.RefusedType], and the
// translation records that one did not translate, so the rule does not emit
// the declaration.
type translation struct {
	e *eidos.Emitter
	// refused reports that a type did not translate.
	refused bool
}

// ref returns the reference of the plan's target for ref, whose owner is
// the declaration owner, and nil for a nil ref.
func (t *translation) ref(owner symbol.Identity, ref *node.TypeRef) *emit.TypeRef {
	out, spelled := t.e.Type(owner, ref)
	t.refused = t.refused || !spelled
	return out
}

// refs returns the references of the plan's target for refs, and nil for no
// reference.
func (t *translation) refs(owner symbol.Identity, refs []*node.TypeRef) []*emit.TypeRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]*emit.TypeRef, 0, len(refs))
	for _, ref := range refs {
		out = append(out, t.ref(owner, ref))
	}
	return out
}

// typeParams returns the type parameters tps of owner with their bounds,
// their defaults and the types of their values translated, and nil for no
// type parameter.
func (t *translation) typeParams(owner symbol.Identity, tps []*node.TypeParam) []*emit.TypeParam {
	if len(tps) == 0 {
		return nil
	}
	out := make([]*emit.TypeParam, 0, len(tps))
	for _, tp := range tps {
		out = append(out, &emit.TypeParam{
			Name:         tp.Name,
			Variance:     tp.Variance,
			Const:        tp.Const,
			Bounds:       t.refs(owner, tp.Bounds),
			Default:      t.ref(owner, tp.Default),
			Type:         t.ref(owner, tp.Type),
			DefaultValue: tp.DefaultValue,
		})
	}
	return out
}

// params returns the parameters ps of owner with their names, labels,
// defaults and optional and variadic forms, their types translated, and nil
// for no parameter.
func (t *translation) params(owner symbol.Identity, ps []*node.Param) []*emit.Param {
	if len(ps) == 0 {
		return nil
	}
	out := make([]*emit.Param, 0, len(ps))
	for _, p := range ps {
		out = append(out, &emit.Param{
			Name:     p.Name,
			Label:    p.Label,
			Type:     t.ref(owner, p.Type),
			Default:  p.Default,
			Optional: p.Optional,
			Variadic: p.Variadic,
		})
	}
	return out
}

// returns translates the results rs of owner. Each result keeps its name,
// and the translation returns nil for no result.
func (t *translation) returns(owner symbol.Identity, rs []*node.Return) []*emit.Return {
	if len(rs) == 0 {
		return nil
	}
	out := make([]*emit.Return, 0, len(rs))
	for _, r := range rs {
		out = append(out, &emit.Return{Name: r.Name, Type: t.ref(owner, r.Type)})
	}
	return out
}

// field returns the mirror of the field or the property f: its
// documentation, name and visibility, and its type translated with f as
// its owner.
func (t *translation) field(f *node.Field) *emit.Field {
	return &emit.Field{
		Origin:     f.ID,
		Doc:        slices.Clone(f.Doc),
		Name:       f.Name,
		Visibility: f.Visibility,
		Type:       t.ref(f.ID, f.Type),
	}
}

// method returns the mirror of the method m. The mirror has the
// documentation, the name, the visibility, the level and the async flag of
// m, and the type parameters, the parameters, the results and the announced
// failure types of m with each type translated with m as its owner. The
// mirror has no receiver and no body.
func (t *translation) method(m *node.Method) *emit.Method {
	return &emit.Method{
		Origin:     m.ID,
		Doc:        slices.Clone(m.Doc),
		Name:       m.Name,
		Visibility: m.Visibility,
		Level:      m.Level,
		Async:      m.Async,
		TypeParams: t.typeParams(m.ID, m.TypeParams),
		Params:     t.params(m.ID, m.Params),
		Returns:    t.returns(m.ID, m.Returns),
		Throws:     t.refs(m.ID, m.Throws),
	}
}
