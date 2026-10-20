// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares the fact coverage of TypeScript, the feature table as
// data. The guard of the render reads it, and the conformance suite checks
// that it is total and that the rendered findings match it.
//
// Decorators apply to classes and their members, so annotations refuse
// everywhere else, the parameter position included. The default of a
// parameter renders behind its equals sign. Field tags are an idiom of Go
// and refuse. The methods of a class render as its members, where the
// methods of an enum and of a sum refuse, because neither has members in
// TypeScript. An index signature and a constructor take a narrower set of
// modifiers than a method, and their keyword helpers refuse the rest.
// Nothing promotes members, so the coverage refuses an embed with every
// fact on it. A field that another type receives refuses, because the
// backend does not write a declaration merge. The annotations of a file
// refuse, because the skeleton does not write the header that a
// generator states. The passing of a parameter takes the [render.Holds]
// verdict, because a caller passes every argument by position. A wire name renders as the key of
// its property, bare where it is an identifier and quoted otherwise.
//
// Coverage builds the table on every call, which the caller keeps: four
// allocations of the fact map, and four of the exception map with two for
// each of its twelve kinds, 32 allocations.
func Coverage() render.Coverage {
	return render.Coverage{
		RefusesEmbeds: true,
		Facts: map[symbol.Fact]render.Verdict{
			symbol.FactVisibility:       render.Renders,
			symbol.FactAsync:            render.Renders,
			symbol.FactAccessor:         render.Renders,
			symbol.FactIndexer:          render.Renders,
			symbol.FactConstructs:       render.Renders,
			symbol.FactHardPrivate:      render.Renders,
			symbol.FactConstEnum:        render.Renders,
			symbol.FactOptional:         render.Renders,
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
			symbol.FactPassing:          render.Holds,
			symbol.FactWire:             render.Renders,
			symbol.FactNamedReturn:      render.Refuses,
			symbol.FactFields:           render.Refuses,
			symbol.FactMethods:          render.Refuses,
			symbol.FactValue:            render.Renders,
			symbol.FactReceives:         render.Refuses,
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
			symbol.FactComputed:         render.Renders,
			symbol.FactUnderlying:       render.Refuses,
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
				symbol.FactLevel:   render.Refuses, // no static classes
				symbol.FactMethods: render.Renders, // class members
			},
			symbol.KindFile: {
				symbol.FactAnnotations: render.Refuses, // no header of a generator
			},
		},
	}
}
