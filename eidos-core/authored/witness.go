// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored

import (
	"slices"
	"strings"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
)

// Witness returns the annotator behind the witness directive. It
// runs once per validated instance on every generic kind, a
// struct, an interface, a function, a method, an alias and a sum,
// and stamps each key's resolved type under gen.witness on the
// type parameter the key names. A key naming none of the
// declaration's parameters is refused under [UnknownWitnessParam]
// at the carrier, and the rest still stamp.
func Witness() plugin.Annotator {
	p := eidos.NewPlugin(WitnessPlugin).
		Handle(eidos.Gated(directive.KernelWitness,
			eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
				return stampWitness(m, st, m.Struct.TypeParams)
			}),
			eidos.OnInterface(func(m *eidos.InterfaceMatch, st *eidos.Stamper) error {
				return stampWitness(m, st, m.Interface.TypeParams)
			}),
			eidos.OnFunction(func(m *eidos.FunctionMatch, st *eidos.Stamper) error {
				return stampWitness(m, st, m.Function.TypeParams)
			}),
			eidos.OnMethod(func(m *eidos.MethodMatch, st *eidos.Stamper) error {
				return stampWitness(m, st, m.Method.TypeParams)
			}),
			eidos.OnAlias(func(m *eidos.AliasMatch, st *eidos.Stamper) error {
				return stampWitness(m, st, m.Alias.TypeParams)
			}),
			eidos.OnSum(func(m *eidos.SumMatch, st *eidos.Stamper) error {
				return stampWitness(m, st, m.Sum.TypeParams)
			}),
		)).
		Build()
	return annotator(p)
}

// reporter is the finding surface a witness handler needs beside
// the carrier: a refusal positioned at the directive's own line.
type reporter interface {
	carrier
	ErrorfAt(at position.Pos, c diag.Code, format string, a ...any)
}

// stampWitness stamps one instance's keys onto the declaration's
// type parameters, in key order so the findings arrive
// deterministically. A key is a type parameter's name; the reserved
// routing keys are never read as one.
func stampWitness(m reporter, st *eidos.Stamper, params []*node.TypeParam) error {
	d := m.Directive()
	keys := make([]string, 0, len(d.Params))
	for key := range d.Params {
		if key == directive.ReservedOut || key == directive.ReservedTag {
			continue
		}
		keys = append(keys, string(key))
	}
	slices.Sort(keys)
	for _, key := range keys {
		tp := paramNamed(params, key)
		if tp == nil {
			m.ErrorfAt(d.Pos, UnknownWitnessParam,
				"witness names %s, which the declaration does not declare; its type parameters are %s",
				key, spellParams(params))
			continue
		}
		v, _ := d.Param(directive.ParamKey(key))
		if v.Target.IsZero() {
			m.ErrorfAt(d.Pos, eidos.RefusedStamp,
				"witness %s carries no resolved type: validation ran without a resolver", key)
			continue
		}
		eidos.StampOn(st, tp.ID, m.Kernel().Witness, v.Target)
	}
	return nil
}

// paramNamed returns the type parameter with one name, or nil.
func paramNamed(params []*node.TypeParam, name string) *node.TypeParam {
	for _, tp := range params {
		if tp != nil && tp.Name == name {
			return tp
		}
	}
	return nil
}

// spellParams lists the type parameters' names for a refusal, and
// "none" for a declaration without any.
func spellParams(params []*node.TypeParam) string {
	names := make([]string, 0, len(params))
	for _, tp := range params {
		if tp != nil {
			names = append(names, tp.Name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}
