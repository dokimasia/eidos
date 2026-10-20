// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored

import (
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// The names of the two annotators, which their findings and claims record
// as the origin.
const (
	// SamplePlugin is the name of the sample annotator.
	SamplePlugin plugin.ID = "gen.sample"
	// WitnessPlugin is the name of the witness annotator.
	WitnessPlugin plugin.ID = "gen.witness"
)

// carrier is the part of a match that both handlers read, which is the
// gating instance and the keys of the kernel.
type carrier interface {
	Directive() *directive.Directive
	Kernel() meta.KernelKeys
}

// Sample returns the annotator of the sample directive. It runs once for
// each validated instance on a field, a parameter, a return, a variable, a
// constant, an alias, a struct, an interface, an enum or a sum. It stamps
// the value under gen.sample and the alternate under gen.alternate where
// the instance states them, both as the text that the author wrote. It
// refuses an instance that states neither under [EmptySample] at the
// carrier. A callable has several types or none, so the fact store
// refuses the stamp at the subject.
func Sample() plugin.Annotator {
	return eidos.NewPlugin(SamplePlugin).
		Handle(eidos.Gated(directive.KernelSample,
			eidos.OnField(func(m *eidos.FieldMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnParam(func(m *eidos.ParamMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnReturn(func(m *eidos.ReturnMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnVariable(func(m *eidos.VariableMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnConstant(func(m *eidos.ConstantMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnAlias(func(m *eidos.AliasMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnInterface(func(m *eidos.InterfaceMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnEnum(func(m *eidos.EnumMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnSum(func(m *eidos.SumMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			// A callable has several types or none, so the key does not
			// admit it. The fact store refuses the stamp at the subject
			// under its own code.
			eidos.OnFunction(func(m *eidos.FunctionMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
			eidos.OnMethod(func(m *eidos.MethodMatch, st *eidos.Stamper) error { return stampSample(m, st) }),
		)).
		Annotator()
}

// stampSample stamps the value and the alternate of one instance on the
// subject. The validation checked the instance against its schema, so
// both are strings where the instance states them. It reports an instance
// that states neither under [EmptySample], and stamps nothing.
func stampSample(m reporter, st *eidos.Stamper) error {
	d := m.Directive()
	keys := m.Kernel()
	value, valued := d.Param(directive.SampleValue)
	alternate, alternated := d.Param(directive.SampleAlternate)
	if !valued && !alternated {
		m.ErrorfAt(EmptySample, d.Pos, "sample states neither a value nor an alternate")
		return nil
	}
	if valued {
		eidos.Stamp(st, keys.Sample, value.Str)
	}
	if alternated {
		eidos.Stamp(st, keys.Alternate, alternate.Str)
	}
	return nil
}
