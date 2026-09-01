// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares TypeScript's fact coverage, the feature table
// as data: the render's guard reads it, and the conformance suite
// holds it total and the rendered findings against it.
//
// Decorators apply to classes and their members, so annotations
// refuse everywhere else, the parameter position included. A
// parameter's default renders behind its equals sign. Field tags
// stay a Go idiom and refuse.
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
			symbol.FactAbstract:         render.Renders,
			symbol.FactFinal:            render.Refuses,
			symbol.FactSealed:           render.Refuses,
			symbol.FactPermits:          render.Refuses,
			symbol.FactOverride:         render.Renders,
			symbol.FactDefaultBody:      render.Refuses,
			symbol.FactLabel:            render.Refuses,
			symbol.FactParamDefault:     render.Renders,
			symbol.FactVariadic:         render.Renders,
			symbol.FactNamedReturn:      render.Refuses,
			symbol.FactFields:           render.Refuses,
			symbol.FactMethods:          render.Refuses,
			symbol.FactValue:            render.Renders,
			symbol.FactComment:          render.Renders,
			symbol.FactMutability:       render.Renders,
			symbol.FactTag:              render.Refuses,
			symbol.FactTypes:            render.Refuses,
			symbol.FactEmbeds:           render.Refuses,
			symbol.FactExtends:          render.Renders,
			symbol.FactImplements:       render.Renders,
			symbol.FactProperties:       render.Renders,
			symbol.FactDefined:          render.Refuses,
			symbol.FactVariance:         render.Renders,
			symbol.FactTypeParamDefault: render.Renders,
			symbol.FactConstParam:       render.Refuses,
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindInterface: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindFunction: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindAlias: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindEnum: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindEnumVariant: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindSum: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindSumVariant: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindConstant: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindVariable: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindParam: {
				symbol.FactAnnotations: render.Refuses,
			},
			symbol.KindStruct: {
				symbol.FactLevel: render.Refuses, // no static classes
			},
		},
	}
}
