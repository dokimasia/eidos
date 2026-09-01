// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// Coverage declares Rust's fact coverage, the feature table as
// data: the render's guard reads it, and the conformance suite
// holds it total and the rendered findings against it.
//
// Final holds, because nothing subclasses. A trait widens through
// supertraits, so an interface's extends renders while a struct's
// refuses, and implements refuses everywhere until something
// spells impl blocks for stated contracts. A trait's nested types
// render as associated types where every other kind's refuse,
// because Rust nests nothing else. Throws refuses today; the
// Result lowering is the recorded idiom and lands as a satellite
// change. An enum variant's value renders as its discriminant
// where a field's initializer refuses, because a struct declares
// no field defaults.
func Coverage() render.Coverage {
	return render.Coverage{
		Facts: map[symbol.Fact]render.Verdict{
			symbol.FactVisibility:       render.Renders,
			symbol.FactAsync:            render.Renders,
			symbol.FactTypeParams:       render.Renders,
			symbol.FactMultiReturn:      render.Renders,
			symbol.FactThrows:           render.Refuses,
			symbol.FactAnnotations:      render.Renders,
			symbol.FactLevel:            render.Renders,
			symbol.FactAbstract:         render.Refuses,
			symbol.FactFinal:            render.Holds,
			symbol.FactOverride:         render.Refuses,
			symbol.FactDefaultBody:      render.Renders,
			symbol.FactLabel:            render.Refuses,
			symbol.FactParamDefault:     render.Refuses,
			symbol.FactVariadic:         render.Refuses,
			symbol.FactNamedReturn:      render.Refuses,
			symbol.FactFields:           render.Refuses,
			symbol.FactMethods:          render.Refuses,
			symbol.FactValue:            render.Refuses,
			symbol.FactComment:          render.Renders,
			symbol.FactMutability:       render.Refuses,
			symbol.FactTag:              render.Refuses,
			symbol.FactTypes:            render.Refuses,
			symbol.FactEmbeds:           render.Refuses,
			symbol.FactExtends:          render.Renders,
			symbol.FactImplements:       render.Refuses,
			symbol.FactProperties:       render.Refuses,
			symbol.FactDefined:          render.Refuses,
			symbol.FactVariance:         render.Refuses,
			symbol.FactTypeParamDefault: render.Renders,
			symbol.FactConstParam:       render.Renders,
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindField: {
				symbol.FactLevel: render.Refuses, // no statics inside types
			},
			symbol.KindEnumVariant: {
				symbol.FactValue: render.Renders, // the discriminant
			},
			symbol.KindStruct: {
				symbol.FactExtends: render.Refuses, // nothing inherits
			},
			symbol.KindInterface: {
				symbol.FactTypes: render.Renders, // associated types
			},
			symbol.KindParam: {
				symbol.FactAnnotations: render.Refuses,
			},
		},
	}
}
