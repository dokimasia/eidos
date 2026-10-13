// Copyright Dokimasia B.V. 2026
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

// noTypeParams is what a refusal lists for a declaration without type
// parameters.
const noTypeParams = "none"

// Witness returns the annotator of the witness directive. It runs once
// for each validated instance on a struct, an interface, a function, a
// method, an alias or a sum. For each key, it stamps the resolved type
// under gen.witness on the type parameter that the key names. It refuses
// a key that names none of the parameters of the declaration under
// [UnknownWitnessParam] at the carrier, and stamps the other keys. It
// refuses an instance without a key under [EmptyWitness] at the carrier.
func Witness() plugin.Annotator {
	return eidos.NewPlugin(WitnessPlugin).
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
		Annotator()
}

// reporter is the part of a match that a witness handler needs beside the
// carrier. It reports a refusal at the line of the directive.
type reporter interface {
	carrier
	ErrorfAt(c diag.Code, at position.Pos, format string, a ...any)
}

// stampWitness stamps the keys of one instance on the type parameters of
// the declaration, in key order, so the findings are deterministic. A key
// is the name of a type parameter, and stampWitness skips the reserved
// routing keys. It reports an instance without a key under
// [EmptyWitness], with the type parameters of the declaration.
func stampWitness(m reporter, st *eidos.Stamper, params []*node.TypeParam) error {
	d := m.Directive()
	keys := make([]string, 0, len(d.Params))
	for key := range d.Params {
		if key == directive.ReservedOut || key == directive.ReservedTag {
			continue
		}
		keys = append(keys, string(key))
	}
	if len(keys) == 0 {
		m.ErrorfAt(EmptyWitness, d.Pos,
			"witness names no type parameter. The type parameters of the declaration are %s", spellParams(params))
		return nil
	}
	slices.Sort(keys)
	for _, key := range keys {
		tp := paramNamed(params, key)
		if tp == nil {
			m.ErrorfAt(UnknownWitnessParam, d.Pos,
				"witness names %s, which the declaration does not declare. Its type parameters are %s",
				key, spellParams(params))
			continue
		}
		v, _ := d.Param(directive.ParamKey(key))
		if v.Target.IsZero() {
			m.ErrorfAt(eidos.RefusedStamp, d.Pos,
				"witness %s has no resolved type: validation ran without a resolver", key)
			continue
		}
		eidos.StampOn(st, tp.ID, m.Kernel().Witness, v.Target)
	}
	return nil
}

// paramNamed returns the type parameter named name, or nil.
func paramNamed(params []*node.TypeParam, name string) *node.TypeParam {
	for _, tp := range params {
		if tp != nil && tp.Name == name {
			return tp
		}
	}
	return nil
}

// spellParams returns the names of the type parameters for a refusal, and
// [noTypeParams] for a declaration without type parameters.
func spellParams(params []*node.TypeParam) string {
	names := make([]string, 0, len(params))
	for _, tp := range params {
		if tp != nil {
			names = append(names, tp.Name)
		}
	}
	if len(names) == 0 {
		return noTypeParams
	}
	return strings.Join(names, ", ")
}
