// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored

import (
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// The two annotators' identities: the origin their findings and
// claims carry.
const (
	// SamplePlugin is the sample annotator's identity.
	SamplePlugin plugin.ID = "gen.sample"
	// WitnessPlugin is the witness annotator's identity.
	WitnessPlugin plugin.ID = "gen.witness"
)

// carrier is what both handlers read off any match: the gating
// instance and the kernel's keys.
type carrier interface {
	Directive() *directive.Directive
	Kernel() meta.KernelKeys
}

// Sample returns the annotator behind the sample directive. It runs
// once per validated instance on every kind the directive admits,
// a field, a parameter, a return, a variable, a constant, an alias,
// a struct, an enum and a sum, and stamps the value under
// gen.sample and the alternate, where stated, under gen.alternate,
// both as the text the author wrote. On a callable or an
// interface, which carry several types or none, the stamp is
// refused at the subject.
func Sample() plugin.Annotator {
	p := eidos.NewPlugin(SamplePlugin).
		Handle(eidos.Gated(directive.KernelSample,
			eidos.OnField(func(m *eidos.FieldMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnParam(func(m *eidos.ParamMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnReturn(func(m *eidos.ReturnMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnVariable(func(m *eidos.VariableMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnConstant(func(m *eidos.ConstantMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnAlias(func(m *eidos.AliasMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnEnum(func(m *eidos.EnumMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnSum(func(m *eidos.SumMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			// A callable and an interface carry several types or
			// none, so the key admits neither: the stamp is refused
			// at the subject under the fact store's own code.
			eidos.OnFunction(func(m *eidos.FunctionMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnMethod(func(m *eidos.MethodMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnInterface(func(m *eidos.InterfaceMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
		)).
		Build()
	return annotator(p)
}

// annotator returns a built plugin as the annotator its stamper
// handlers make it. The handlers all stamp, so a plugin that is
// not one is a defect in this package.
func annotator(p plugin.Plugin) plugin.Annotator {
	a, held := p.(plugin.Annotator)
	if !held {
		panic("authored: a plugin of stamper handlers lowers to an annotator")
	}
	return a
}

// stampSample stamps one instance's value and alternate on the
// subject. Validation held the instance to its schema, so the value
// is present and both are strings.
func stampSample(m carrier, st *eidos.Stamper) error {
	d := m.Directive()
	keys := m.Kernel()
	if v, held := d.Param(directive.SampleValue); held {
		eidos.Stamp(st, keys.Sample, v.Str)
	}
	if v, held := d.Param(directive.SampleAlternate); held {
		eidos.Stamp(st, keys.Alternate, v.Str)
	}
	return nil
}
