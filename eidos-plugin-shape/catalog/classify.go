// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"errors"
	"slices"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ShapeID is the name of the plugin that classifies callables.
const ShapeID plugin.ID = "shape"

// The names of the directives of the catalog, and the param of the
// contract directive that separates two instances in one package.
const (
	ShapeDirective    directive.Name     = "shape"
	MixinDirective    directive.Name     = "mixin"
	ContractDirective directive.Name     = "contract"
	IDParam           directive.ParamKey = "id"
)

// callables are the kinds that the plugin shape stamps every key of the
// catalog on.
var callables = []symbol.Kind{symbol.KindFunction, symbol.KindMethod}

// match is the part of a function match or a method match that a handler
// of the catalog uses. It has the gating directive, the bound rules, the
// tracked reader and the reporting methods. [sdk.FunctionMatch] and
// [sdk.MethodMatch] satisfy it.
type match interface {
	shape.Scoped
	Directive() *directive.Directive
	Rules() rules.Bound
	Errorf(c diag.Code, format string, a ...any)
	ErrorfAt(c diag.Code, at position.Pos, format string, a ...any)
}

// callable is the subject of a handler. It has the symbol of the callable,
// and the parameters and the returns whose identities the plugin stamps
// for the bindings.
type callable struct {
	sym     symbol.Symbol
	params  []*node.Param
	returns []*node.Return
}

// classifier is the plugin shape of one workspace. It keeps the specs in
// name order and by name, the detections in the order of precedence, and
// the handles that the registration of its keys returns.
type classifier struct {
	ordered    []shape.Spec
	specs      map[string]*shape.Spec
	detections []shape.Detection
	detected   meta.Key[[]string]
	shapeName  meta.Key[string]
	mixed      meta.Key[bool]
	member     meta.Key[bool]
	classified meta.Key[bool]
	families   map[string]*family
}

// family is the handles of the keys of one classification. A shape and a
// mixin have the flag, and a contract has the role and the id. stamps has
// the function that stamps a validated value of each param under the key
// of the param, and bindings are in the order of the bindings of the
// spec.
type family struct {
	flag     meta.Key[bool]
	role     meta.Key[string]
	id       meta.Key[string]
	stamps   map[directive.ParamKey]func(st *sdk.Stamper, v directive.Value)
	bindings []meta.Key[symbol.Identity]
}

// newClassifier returns the plugin shape. It declares the three
// directives and registers every key of the catalog. It stamps the facts
// of each directive instance at directive authority, and the facts of the
// detected shape that ranks first at plugin authority.
func newClassifier(specs []shape.Spec, detections []shape.Detection) plugin.Annotator {
	c := &classifier{
		ordered:    specs,
		specs:      make(map[string]*shape.Spec, len(specs)),
		detections: detections,
		families:   make(map[string]*family, len(specs)),
	}
	for i := range specs {
		c.specs[specs[i].Name] = &specs[i]
	}
	shapes, mixins, contracts := schemas(specs)
	a, _ := sdk.NewPlugin(ShapeID).
		Provides(shape.Capability).
		Keys(c.register).
		Handle(
			sdk.Directive(shapes,
				sdk.OnFunction(func(m *sdk.FunctionMatch, st *sdk.Stamper) error {
					return c.declare(m, st, callable{m.Function, m.Function.Params, m.Function.Returns})
				}),
				sdk.OnMethod(func(m *sdk.MethodMatch, st *sdk.Stamper) error {
					return c.declare(m, st, callable{m.Method, m.Method.Params, m.Method.Returns})
				})),
			sdk.Directive(mixins,
				sdk.OnFunction(func(m *sdk.FunctionMatch, st *sdk.Stamper) error {
					return c.declare(m, st, callable{m.Function, m.Function.Params, m.Function.Returns})
				}),
				sdk.OnMethod(func(m *sdk.MethodMatch, st *sdk.Stamper) error {
					return c.declare(m, st, callable{m.Method, m.Method.Params, m.Method.Returns})
				})),
			sdk.Directive(contracts,
				sdk.OnFunction(func(m *sdk.FunctionMatch, st *sdk.Stamper) error {
					return c.declare(m, st, callable{m.Function, m.Function.Params, m.Function.Returns})
				}),
				sdk.OnMethod(func(m *sdk.MethodMatch, st *sdk.Stamper) error {
					return c.declare(m, st, callable{m.Method, m.Method.Params, m.Method.Returns})
				})),
			sdk.OnFunction(func(m *sdk.FunctionMatch, st *sdk.Stamper) error {
				return c.detect(m, st, callable{m.Function, m.Function.Params, m.Function.Returns})
			}),
			sdk.OnMethod(func(m *sdk.MethodMatch, st *sdk.Stamper) error {
				return c.detect(m, st, callable{m.Method, m.Method.Params, m.Method.Returns})
			}),
		).
		Build().(plugin.Annotator)
	return a
}

// schemas returns the three directives of the catalog, each with one
// variant for each spec of its form. The shape directive is an override
// schema, so an instance of it keeps the detection rules off its callable.
// The mixin and the contract directives are repeatable with one instance
// of each variant on a callable, and the contract directive has the param
// id.
func schemas(specs []shape.Spec) (shapes, mixins, contracts directive.Schema) {
	shapes = directive.Schema{
		Plugin: string(ShapeID), Name: ShapeDirective, Overrides: true,
		Doc: "declares the shape of a callable, which keeps the detectors off the callable",
	}
	mixins = directive.Schema{
		Plugin: string(ShapeID), Name: MixinDirective, Repeatable: true,
		Doc: "declares a mixin of a callable",
	}
	contracts = directive.Schema{
		Plugin: string(ShapeID), Name: ContractDirective, Repeatable: true,
		Params: []directive.ParamSpec{{
			Key: IDParam, Type: directive.TypeString,
			Doc: "the id that separates two instances of the contract in one package",
		}},
		Doc: "declares the role of a callable in an instance of a contract",
	}
	for _, s := range specs {
		switch s.Form {
		case shape.FormShape:
			shapes.Variants = append(shapes.Variants, s.Variant())
		case shape.FormMixin:
			mixins.Variants = append(mixins.Variants, s.Variant())
		case shape.FormContract:
			contracts.Variants = append(contracts.Variants, s.Variant())
		}
	}
	return shapes, mixins, contracts
}

// register claims the namespace of the catalog, and registers the summary
// keys and the keys of every classification. Each key of a classification
// is in the fact group of the classification. register keeps each handle
// for the handlers.
//
// Error modes: the error of the claim of the namespace, such as a
// namespace that another registrant claimed, and otherwise the errors of
// every registration, joined.
func (c *classifier) register(r *meta.Registry) error {
	if err := r.ClaimNamespace(shape.KeyShape.Namespace()); err != nil {
		return err
	}
	var errs []error
	var err error
	c.detected, err = meta.Register[[]string](r, meta.KeySpec{
		Name: shape.KeyDetected, Kinds: callables,
		Doc: "the detected shapes whose detectors report a callable, in the order of precedence",
	})
	errs = append(errs, err)
	c.shapeName, err = meta.Register[string](r, meta.KeySpec{
		Name: shape.KeyShape, Kinds: callables, Doc: "the name of the shape of a callable",
	})
	errs = append(errs, err)
	marks := []struct {
		key  *meta.Key[bool]
		name meta.KeyName
		doc  string
	}{
		{&c.mixed, shape.KeyMixed, "marks a callable with a mixin"},
		{&c.member, shape.KeyMember, "marks a callable with a role in a contract instance"},
		{&c.classified, shape.KeyClassified, "marks a callable with a classification"},
	}
	for _, mark := range marks {
		*mark.key, err = meta.Register[bool](r, meta.KeySpec{Name: mark.name, Kinds: callables, Doc: mark.doc})
		errs = append(errs, err)
	}
	for i := range c.ordered {
		var f *family
		f, err = registerFamily(r, &c.ordered[i])
		errs = append(errs, err)
		c.families[c.ordered[i].Name] = f
	}
	return errors.Join(errs...)
}

// registerFamily registers the keys of one classification into its group.
// They are the family key, the id key of a contract, and the key of each
// param and each binding.
//
// Error modes: the errors of the registrations, joined.
func registerFamily(r *meta.Registry, s *shape.Spec) (*family, error) {
	f := &family{stamps: make(map[directive.ParamKey]func(st *sdk.Stamper, v directive.Value), len(s.Params))}
	var errs []error
	var err error
	switch s.Form {
	case shape.FormContract:
		f.role, err = meta.Register[string](r, meta.KeySpec{
			Name: s.Key, Kinds: callables, Group: s.Group,
			Doc: "the role of a callable in an instance of the contract " + s.Name,
		})
		errs = append(errs, err)
		f.id, err = meta.Register[string](r, meta.KeySpec{
			Name: s.ID, Kinds: callables, Group: s.Group,
			Doc: "the id of the instance of the contract " + s.Name + " that a callable has a role in",
		})
		errs = append(errs, err)
	default:
		f.flag, err = meta.Register[bool](r, meta.KeySpec{
			Name: s.Key, Kinds: callables, Group: s.Group,
			Doc: "marks a callable with the " + s.Form.String() + " " + s.Name,
		})
		errs = append(errs, err)
	}
	for _, p := range s.Params {
		spec := meta.KeySpec{Name: p.Fact, Kinds: callables, Group: s.Group, Doc: p.Doc}
		f.stamps[p.Key], err = paramStamp(r, spec, p.Type)
		errs = append(errs, err)
	}
	for _, b := range s.Bindings {
		var k meta.Key[symbol.Identity]
		k, err = meta.Register[symbol.Identity](r, meta.KeySpec{
			Name: b.Fact, Kinds: callables, Group: s.Group, Doc: b.Doc,
		})
		errs = append(errs, err)
		f.bindings = append(f.bindings, k)
	}
	return f, errors.Join(errs...)
}

// paramStamp registers the key of one param by the type of the param, and
// returns the function that stamps a validated value under the key. The
// function stamps the string, the int, or the identity of the declaration
// that a reference resolved to.
//
// Error modes: the errors of the registry.
func paramStamp(
	r *meta.Registry, spec meta.KeySpec, t directive.ParamType,
) (func(*sdk.Stamper, directive.Value), error) {
	switch t {
	case directive.TypeInt:
		k, err := meta.Register[int64](r, spec)
		return func(st *sdk.Stamper, v directive.Value) { sdk.Stamp(st, k, v.Int) }, err
	case directive.TypeReference:
		k, err := meta.Register[symbol.Identity](r, spec)
		return func(st *sdk.Stamper, v directive.Value) { sdk.Stamp(st, k, v.Target) }, err
	default:
		k, err := meta.Register[string](r, spec)
		return func(st *sdk.Stamper, v directive.Value) { sdk.Stamp(st, k, v.Str) }, err
	}
}

// declare stamps the facts of one directive instance on its callable. It
// stamps the family key and the summary keys of the classification, the
// bindings of a shape, the role and the id of a contract, and each param
// that the instance writes. It first reports the faults of the instance
// that the validation of the directive does not check.
func (c *classifier) declare(m match, st *sdk.Stamper, subject callable) error {
	d := m.Directive()
	s := c.specs[d.Variant]
	f := c.families[s.Name]
	check(m, d, s)
	switch s.Form {
	case shape.FormShape:
		var view rules.Callable
		if len(s.Bindings) > 0 {
			view, _ = m.Rules().CallableOf(subject.sym)
		}
		c.stampShape(st, s, subject, view)
	case shape.FormMixin:
		sdk.Stamp(st, f.flag, true)
		sdk.Stamp(st, c.mixed, true)
		sdk.Stamp(st, c.classified, true)
	case shape.FormContract:
		sdk.Stamp(st, f.role, d.Role)
		if id, held := d.Param(IDParam); held && id.Str != "" {
			sdk.Stamp(st, f.id, id.Str)
		}
		sdk.Stamp(st, c.member, true)
		sdk.Stamp(st, c.classified, true)
	}
	for _, p := range s.Params {
		if v, held := d.Param(p.Key); held {
			f.stamps[p.Key](st, v)
		}
	}
	return nil
}

// detect runs every detector over the projection of the callable. It
// stamps the detected shapes, and the facts of the first of them. A
// callable of a language without rules has no projection, so detect
// stamps nothing on it.
func (c *classifier) detect(m match, st *sdk.Stamper, subject callable) error {
	b := m.Rules()
	if rules.IsAbsent(b.Source()) {
		return nil
	}
	view, _ := b.CallableOf(subject.sym)
	var detected []string
	for _, d := range c.detections {
		if d.Detect(view, b) {
			detected = append(detected, string(d.Shape))
		}
	}
	if len(detected) == 0 {
		return nil
	}
	sdk.Stamp(st, c.detected, detected)
	c.stampShape(st, c.specs[detected[0]], subject, view)
	return nil
}

// stampShape stamps a shape on a callable. It stamps the family key, the
// summary keys, and the identity of the parameter or the return at the
// position of each binding, which the projection of the callable gives. A
// binding at a position that the callable does not have stamps nothing.
func (c *classifier) stampShape(st *sdk.Stamper, s *shape.Spec, subject callable, view rules.Callable) {
	f := c.families[s.Name]
	sdk.Stamp(st, f.flag, true)
	sdk.Stamp(st, c.shapeName, s.Name)
	sdk.Stamp(st, c.classified, true)
	for i, b := range s.Bindings {
		if id, bound := bindingAt(view, subject, b); bound {
			sdk.Stamp(st, f.bindings[i], id)
		}
	}
}

// bindingAt returns the identity of the parameter or the return at the
// position of a binding. An input binding counts the parameters with the
// input role, and a result binding counts the returns with the value or
// the stream role.
func bindingAt(view rules.Callable, subject callable, b shape.Binding) (symbol.Identity, bool) {
	n := 0
	switch b.From {
	case shape.SourceInput:
		for i, p := range view.Params {
			if p.Role != rules.ParamInput {
				continue
			}
			if n == b.Index {
				return subject.params[i].ID, true
			}
			n++
		}
	case shape.SourceResult:
		for i, r := range view.Returns {
			if r.Role != rules.ReturnValue && r.Role != rules.ReturnStream {
				continue
			}
			if n == b.Index {
				return subject.returns[i].ID, true
			}
			n++
		}
	}
	return symbol.Identity{}, false
}

// check reports the faults of a directive instance that the validation of
// the directive does not check. They are an int below its minimum, two
// params that exclude each other, and a host parameter that the callable
// of another param does not declare. Each finding is an Error at the
// directive.
func check(m match, d *directive.Directive, s *shape.Spec) {
	for _, p := range s.Params {
		v, held := d.Param(p.Key)
		if !held {
			continue
		}
		if p.HasMinimum && v.Int < p.Minimum {
			m.ErrorfAt(ParamRange, d.Pos, "%s=%d is below %d, the least value of %s on the %s %s",
				p.Key, v.Int, p.Minimum, p.Key, s.Form, s.Name)
		}
		for _, x := range p.Excludes {
			if _, both := d.Param(x); both && p.Key < x {
				m.ErrorfAt(ExclusiveParams, d.Pos, "the %s %s takes %s= or %s=, and this directive writes both",
					s.Form, s.Name, p.Key, x)
			}
		}
		for _, o := range p.AlsoOn {
			if callee, written := d.Param(o); written && !declares(m, callee.Target, v.Ref) {
				m.ErrorfAt(UnsharedParam, d.Pos, "the callable %s of %s= has no parameter %s, "+
					"so a check cannot pass one value of %s= to both callables", callee.Ref, o, v.Ref, p.Key)
			}
		}
	}
}

// declares reports whether the callable of an identity declares a
// parameter of a name. A callable that the view does not contain declares
// no parameter.
func declares(m match, id symbol.Identity, name string) bool {
	sym, _ := m.Reader().Lookup(id)
	var params []*node.Param
	switch callee := sym.(type) {
	case *node.Function:
		params = callee.Params
	case *node.Method:
		params = callee.Params
	}
	return slices.ContainsFunc(params, func(p *node.Param) bool { return p.Name == name })
}
