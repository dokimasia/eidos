// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares Java's fact coverage, the feature table as
// data: the render's guard reads it, and the conformance suite
// holds it total and the rendered findings against it.
//
// A callable returns one value, so several refuse; a second result
// arrives thrown, which is why throws renders. An enum constant's
// value refuses, because a valued constant takes the constructor
// form the templates do not spell, and a sum's methods refuse,
// because the lowering's variant classes would owe bodies the
// model does not carry. Nested types render at member depth
// through their own kind templates.
func Coverage() render.Coverage {
	return render.Coverage{
		Facts: map[symbol.Fact]render.Verdict{
			symbol.FactVisibility:       render.Renders,
			symbol.FactAsync:            render.Refuses,
			symbol.FactAccessor:         render.Refuses,
			symbol.FactIndexer:          render.Refuses,
			symbol.FactConstructs:       render.Refuses,
			symbol.FactHardPrivate:      render.Refuses,
			symbol.FactConstEnum:        render.Refuses,
			symbol.FactOptional:         render.Refuses,
			symbol.FactTypeParams:       render.Renders,
			symbol.FactMultiReturn:      render.Refuses,
			symbol.FactThrows:           render.Renders,
			symbol.FactAnnotations:      render.Renders,
			symbol.FactLevel:            render.Renders,
			symbol.FactAbstract:         render.Renders,
			symbol.FactFinal:            render.Renders,
			symbol.FactSealed:           render.Renders,
			symbol.FactPermits:          render.Renders,
			symbol.FactOverride:         render.Renders,
			symbol.FactDefaultBody:      render.Renders,
			symbol.FactLabel:            render.Refuses,
			symbol.FactParamDefault:     render.Refuses,
			symbol.FactVariadic:         render.Renders,
			symbol.FactNamedReturn:      render.Refuses,
			symbol.FactFields:           render.Renders,
			symbol.FactMethods:          render.Renders,
			symbol.FactValue:            render.Renders,
			symbol.FactComment:          render.Renders,
			symbol.FactMutability:       render.Renders,
			symbol.FactTag:              render.Refuses,
			symbol.FactTypes:            render.Renders,
			symbol.FactEmbeds:           render.Refuses,
			symbol.FactExtends:          render.Renders,
			symbol.FactImplements:       render.Renders,
			symbol.FactProperties:       render.Refuses,
			symbol.FactDefined:          render.Refuses,
			symbol.FactVariance:         render.Refuses,
			symbol.FactTypeParamDefault: render.Refuses,
			symbol.FactConstParam:       render.Refuses,
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindEnumVariant: {
				symbol.FactValue: render.Refuses, // the constructor form
			},
			symbol.KindSum: {
				symbol.FactMethods: render.Refuses, // no per-variant bodies
			},
			symbol.KindParam: {
				symbol.FactAnnotations: render.Refuses,
			},
		},
	}
}
