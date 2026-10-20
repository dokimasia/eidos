// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The marks of a variadic type parameter, which the model keeps in its
// name: the star of a type variable tuple and the two stars of a parameter
// specification.
const (
	tupleMark = "*"
	specMark  = "**"
)

// The parts of a declaration of a type variable: the keyword arguments of
// the call and the assignment of the variable.
const (
	boundKeyword         = "bound="
	covariantKeyword     = "covariant="
	contravariantKeyword = "contravariant="
	defaultKeyword       = "default="
	assign               = " = "
	callOpen             = "("
	callClose            = ")"
)

// The parts of a type parameter of PEP 695: the colon of a bound and the
// equals sign of a default. The constraints of a type variable are a bound
// of the form Union whose spelling opens with constraintsOpen, as (int,
// str), because the frontend loads them so.
const (
	boundSep        = ": "
	defaultSep      = " = "
	constraintsOpen = "("
)

// paramSpelling is the room that a list of PEP 695 reserves for the bound
// and the default of each parameter. A longer list grows once more.
const paramSpelling = 32

// rename is a type parameter of the declaration under writing whose type
// variable of the module takes another name.
type rename struct {
	from string
	to   string
}

// varCall is a call that declares a type variable without constraints: the
// callee, the spelled bound and default, and the variance. The writer keeps
// the path of the entry of each call that it declared, which a pure
// function of the call returns, so a parameter that repeats a call
// allocates no path.
type varCall struct {
	callee, bound, def string
	variance           symbol.Variance
}

// legacy reports whether the declaration with the type parameters ps writes
// them as type variables of the module, as Python 3.11 declares them: for
// Python 3.11, for Python 3.12 where a parameter has a default, which PEP
// 696 adds at 3.13, and in every version where a parameter states a
// variance, which the syntax of PEP 695 infers.
func (w *Writer) legacy(ps []*emit.TypeParam) bool {
	minor := minorOf(w.set)
	return minor < minor312 ||
		minor < minor313 && slices.ContainsFunc(ps, func(p *emit.TypeParam) bool { return p.Default != nil }) ||
		slices.ContainsFunc(ps, func(p *emit.TypeParam) bool { return p.Variance != symbol.VarianceInvariant })
}

// TypeParams writes the type parameter list ps after the name of a
// declaration in the syntax of PEP 695, as [T: Bound = Default, *Ts, **P],
// and nothing for an empty list. Python evaluates the bounds and the
// defaults of that syntax when a reader asks for them, so they are never
// strings. Where the declaration writes its parameters as type variables
// of the module, by the rules of [Writer.legacy], TypeParams writes nothing
// and declares the variables, which the head of the file writes after its
// imports. A parameter whose name the scope of the declaration under
// writing gives another variable joins the scope, so a parameter of a
// method hides the parameter of its type with the same name, and a second
// call with the same parameters adds nothing to the scope.
//
// Error modes: a value parameter, a parameter with more than one bound, a
// bound of a type variable tuple or of a parameter specification, and the
// refusals of the types.
func (w *Writer) TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	if w.legacy(ps) {
		return "", w.declareVars(ps)
	}
	var s strings.Builder
	size := len(argsOpen) + len(argsClose)
	for _, p := range ps {
		size += len(argSep) + len(p.Name) + paramSpelling
	}
	s.Grow(size)
	s.WriteString(argsOpen)
	for i, p := range ps {
		b, err := paramParts(p, w.spellLazy)
		if err != nil {
			return "", err
		}
		if name, _ := varOf(p.Name); w.scoped(name) != name {
			w.scope = append(w.scope, rename{from: name, to: name})
		}
		if i > 0 {
			s.WriteString(argSep)
		}
		s.WriteString(p.Name)
		switch {
		case b.constraints != nil:
			s.WriteString(boundSep + callOpen)
			for j, c := range b.constraints {
				if j > 0 {
					s.WriteString(argSep)
				}
				s.WriteString(c)
			}
			s.WriteString(callClose)
		case b.bound != "":
			s.WriteString(boundSep)
			s.WriteString(b.bound)
		}
		if b.def != "" {
			s.WriteString(defaultSep)
			s.WriteString(b.def)
		}
	}
	s.WriteString(argsClose)
	return s.String(), nil
}

// paramBounds are the spelled parts of a type parameter: its bound, the
// constraints of a type variable that has them, and its default.
type paramBounds struct {
	bound       string
	constraints []string
	def         string
}

// paramParts writes the bound or the constraints and the default of a type
// parameter through spell, and refuses a value parameter, a parameter of
// two bounds or more, and a bound of a type variable tuple or of a parameter
// specification. A bound of the form Union whose spelling opens with a
// parenthesis is the constraints of the parameter.
func paramParts(p *emit.TypeParam, spell func(*emit.TypeRef) (string, error)) (paramBounds, error) {
	var out paramBounds
	switch {
	case p.Const:
		return out, refuse("a type parameter takes a type, and %s takes a value", p.Name)
	case len(p.Bounds) > 0 && strings.HasPrefix(p.Name, tupleMark):
		return out, refuse("a type variable tuple and a parameter specification take no bound, and %s states one",
			p.Name)
	case len(p.Bounds) > 1:
		return out, refuse("a type parameter of Python has one bound, and %s states %d", p.Name, len(p.Bounds))
	case len(p.Bounds) == 1 && p.Bounds[0].Form == symbol.FormUnion &&
		strings.HasPrefix(p.Bounds[0].Spelling, constraintsOpen):
		out.constraints = make([]string, 0, len(p.Bounds[0].Elems))
		for _, e := range p.Bounds[0].Elems {
			part, err := spell(e)
			if err != nil {
				return out, err
			}
			out.constraints = append(out.constraints, part)
		}
	case len(p.Bounds) == 1:
		bound, err := spell(p.Bounds[0])
		if err != nil {
			return out, err
		}
		out.bound = bound
	}
	if p.Default != nil {
		def, err := spell(p.Default)
		if err != nil {
			return out, err
		}
		out.def = def
	}
	return out, nil
}

// declareVars declares the type variable of each type parameter of ps, as
// [Writer.declareVar] declares it, and adds the name of each variable to
// the scope of the declaration under writing where the scope gives the
// parameter another variable.
func (w *Writer) declareVars(ps []*emit.TypeParam) error {
	for _, p := range ps {
		variable, err := w.declareVar(p)
		if err != nil {
			return err
		}
		if name, _ := varOf(p.Name); w.scoped(name) != variable {
			w.scope = append(w.scope, rename{from: name, to: variable})
		}
	}
	return nil
}

// declareVar declares the type variable of the type parameter p in the
// import set of the file, as T = TypeVar("T", bound=B), which the head of
// the file writes, and returns the name of the variable. A type variable
// tuple *Ts declares Ts = TypeVarTuple("Ts") and a parameter specification
// **P declares P = ParamSpec("P"). A default declares through
// typing_extensions below Python 3.13. A declaration of the file that takes
// the name of the variable declares it itself, as the assignment of the
// variable in a source written for Python 3.11 does.
//
// A module declares a name once, so a type variable whose name the module
// binds otherwise, such as a parameter T with a bound beside a parameter T
// without one, takes the name with the lowest free number after it, as T2.
// The import set assigns the name, so every parameter of the file with the
// same declaration takes the same variable.
func (w *Writer) declareVar(p *emit.TypeParam) (string, error) {
	name, callee := varOf(p.Name)
	if w.set.Reserved(name) {
		return name, nil
	}
	b, err := paramParts(p, w.spellLater)
	if err != nil {
		return "", err
	}
	module := typingModule
	if b.def != "" && minorOf(w.set) < minor313 {
		module = typingExtensionsModule
	}
	callee = w.set.BindItem(module, callee, false)
	if b.constraints != nil {
		return w.set.BindItem(typeVarCall(callee, b, p.Variance), name, false), nil
	}
	key := varCall{callee: callee, bound: b.bound, def: b.def, variance: p.Variance}
	path, known := w.calls[key]
	if !known {
		path = typeVarCall(callee, b, p.Variance)
		if w.calls == nil {
			w.calls = map[varCall]string{}
		}
		w.calls[key] = path
	}
	return w.set.BindItem(path, name, false), nil
}

// typeVarCall returns the path of the entry of the import set that declares
// a type variable with the callee callee, the parts b and the variance v:
// the mark of a type variable, the callee and the arguments of the call
// after the name of the variable. [Writer.TypeVars] writes the name of the
// entry into the call.
func typeVarCall(callee string, b paramBounds, v symbol.Variance) string {
	size := len(typeVarMark) + len(callee) + len(callOpen) + len(argSep) + len(boundKeyword) + len(b.bound) +
		len(argSep) + len(contravariantKeyword) + len(trueLiteral) + len(argSep) + len(defaultKeyword) +
		len(b.def) + len(callClose)
	for _, c := range b.constraints {
		size += len(argSep) + len(c)
	}
	var s strings.Builder
	s.Grow(size)
	s.WriteString(typeVarMark)
	s.WriteString(callee)
	s.WriteString(callOpen)
	for _, c := range b.constraints {
		s.WriteString(argSep)
		s.WriteString(c)
	}
	if b.bound != "" {
		s.WriteString(argSep)
		s.WriteString(boundKeyword)
		s.WriteString(b.bound)
	}
	switch v {
	case symbol.VarianceOut:
		s.WriteString(argSep + covariantKeyword + trueLiteral)
	case symbol.VarianceIn:
		s.WriteString(argSep + contravariantKeyword + trueLiteral)
	case symbol.VarianceInvariant:
	}
	if b.def != "" {
		s.WriteString(argSep)
		s.WriteString(defaultKeyword)
		s.WriteString(b.def)
	}
	s.WriteString(callClose)
	return s.String()
}

// renamed returns the name of the type variable of the module that a type
// parameter of the declaration under writing takes in place of its own
// name, and reports whether it takes another one. The last parameter of
// the scope with the name decides, so a parameter of a method hides the
// parameter of its type with the same name. In a body, the declaration
// under writing is the callable of the body and its host, which the import
// set names. The stars of an unpacked type variable tuple stay before the
// name.
//
// Error modes: the refusals of the bound and the default of a parameter
// that a body names before its signature declares it.
func (w *Writer) renamed(name string) (string, bool, error) {
	bare := strings.TrimLeft(name, variadicMark)
	variable := w.scoped(bare)
	if callable, host := w.set.Body(); callable != nil {
		var err error
		if variable, err = w.bodyVar(callable, host, bare); err != nil {
			return "", false, err
		}
	}
	if variable == bare {
		return "", false, nil
	}
	return name[:len(name)-len(bare)] + variable, true, nil
}

// scoped returns the name of the type variable of the module that the type
// parameter name of the scope of the declaration under writing takes: the
// name of the last parameter of the scope with the name, and name itself
// where the scope has none.
func (w *Writer) scoped(name string) string {
	for _, r := range slices.Backward(w.scope) {
		if r.from == name {
			return r.to
		}
	}
	return name
}

// bodyVar returns the name of the type variable of the module that the
// type parameter name of a body takes: the parameter of the callable with
// that name, or else the parameter of its host, which the callable hides. A
// name that neither declares is no parameter and keeps its spelling, and so
// does a parameter of the syntax of PEP 695. The variable is the one that
// the signature declares, because the import set assigns its name.
//
// Error modes: the refusals of [Writer.declareVar].
func (w *Writer) bodyVar(callable, host symbol.Symbol, name string) (string, error) {
	scopes := [2][]*emit.TypeParam{typeParamsOf(callable)}
	if host != callable {
		scopes[1] = typeParamsOf(host)
	}
	for _, ps := range scopes {
		for _, p := range ps {
			if bare, _ := varOf(p.Name); bare != name {
				continue
			}
			if !w.legacy(ps) {
				return name, nil
			}
			return w.declareVar(p)
		}
	}
	return name, nil
}

// varOf returns the name of the type variable of a type parameter and the
// callee that declares it: TypeVarTuple for *Ts, ParamSpec for **P, and
// TypeVar for every other name.
func varOf(param string) (name, callee string) {
	if rest, spec := strings.CutPrefix(param, specMark); spec {
		return rest, python.ParamSpec
	}
	if rest, tuple := strings.CutPrefix(param, tupleMark); tuple {
		return rest, python.TypeVarTuple
	}
	return param, python.TypeVar
}

// writeGenericArgs writes the type parameters ps into b as the arguments of
// Generic or Protocol among the bases of a class whose parameters are type
// variables of the module: a type variable tuple with its star, and every
// other parameter by the name of the type variable that the scope of the
// class gives it. It writes nothing for no parameters.
func (w *Writer) writeGenericArgs(b *strings.Builder, ps []*emit.TypeParam) {
	if len(ps) == 0 {
		return
	}
	b.WriteString(argsOpen)
	for i, p := range ps {
		if i > 0 {
			b.WriteString(argSep)
		}
		name, callee := varOf(p.Name)
		if callee == python.TypeVarTuple {
			b.WriteString(tupleMark)
		}
		b.WriteString(w.scoped(name))
	}
	b.WriteString(argsClose)
}

// TypeVars writes the declarations of the type variables of the file, one
// statement on each line, in the order of their names, and nothing for a
// file without type variables. The head of the file writes them after its
// imports. Each entry of a type variable binds the name of the variable to
// its call, whose arguments follow the name in the statement.
//
// # Allocation contract
//
// TypeVars allocates the sorted entries of the set and the statements,
// which it sizes once.
func (w *Writer) TypeVars() string {
	entries := w.set.Entries()
	start := slices.IndexFunc(entries, func(e render.Entry) bool { return strings.HasPrefix(e.Path, typeVarMark) })
	if start < 0 {
		return ""
	}
	end := start
	size := 0
	for end < len(entries) && strings.HasPrefix(entries[end].Path, typeVarMark) {
		e := entries[end]
		size += 2*len(e.Name) + len(assign) + 2*len(doubleQuote) + len(e.Path) - len(typeVarMark) + len(statementEnd)
		end++
	}
	vars := entries[start:end]
	slices.SortFunc(vars, func(a, b render.Entry) int { return strings.Compare(a.Name, b.Name) })
	var b strings.Builder
	b.Grow(size)
	for _, e := range vars {
		callee, args, _ := strings.Cut(e.Path[len(typeVarMark):], callOpen)
		b.WriteString(e.Name)
		b.WriteString(assign)
		b.WriteString(callee)
		b.WriteString(callOpen)
		b.WriteString(doubleQuote)
		b.WriteString(e.Name)
		b.WriteString(doubleQuote)
		b.WriteString(args)
		b.WriteString(statementEnd)
	}
	return b.String()
}
