// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares the fact coverage of Java, the feature table as data.
// The guard of the render reads it, and the conformance suite checks that
// it is total and that the rendered findings match it.
//
//   - A callable returns one value, so several refuse. A second result
//     arrives thrown, which is why throws renders.
//   - The value of an enum constant refuses, because a valued constant
//     takes the constructor form, which the templates do not spell.
//   - The methods of a sum refuse, because the variant classes of the
//     lowering would owe bodies that the model does not state.
//   - Nested types render at member depth through their own kind
//     templates, a class at the type level as static. The lowering
//     refuses static and a narrower scope on a file-level type, where
//     javac rejects both.
//   - The properties of an interface render as its constants, each with
//     its initializer. A property without an initializer refuses through
//     its keyword helper, because a Java interface declares constants
//     alone.
//   - Nothing promotes members, so the coverage refuses an embed with
//     every fact on it.
//   - A Java enum has no underlying type, so the underlying type refuses,
//     and so does the mutability of an index signature. A computed member
//     has no name in Java, so the declaration does not render with the
//     fact or without it.
//   - A field that another type receives refuses, because Java has no
//     extension field. The annotations of a file refuse, because the
//     skeleton does not write the header that a generator states.
//   - The passing of a parameter takes the [render.Holds] verdict, because
//     a Java caller passes every argument by position. A wire name rejects, because a Java file writes
//     the key of a wire format in an annotation.
//
// Coverage builds the table on every call, which the caller keeps: four
// allocations of the fact map, and two of the exception map with two for
// each of its four kinds, 14 allocations.
func Coverage() render.Coverage {
	return render.Coverage{
		RefusesEmbeds: true,
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
			symbol.FactPassing:          render.Holds,
			symbol.FactWire:             render.Rejects,
			symbol.FactNamedReturn:      render.Refuses,
			symbol.FactFields:           render.Renders,
			symbol.FactMethods:          render.Renders,
			symbol.FactValue:            render.Renders,
			symbol.FactReceives:         render.Refuses,
			symbol.FactComment:          render.Renders,
			symbol.FactMutability:       render.Renders,
			symbol.FactTag:              render.Refuses,
			symbol.FactTypes:            render.Renders,
			symbol.FactEmbeds:           render.Refuses,
			symbol.FactExtends:          render.Renders,
			symbol.FactImplements:       render.Renders,
			symbol.FactProperties:       render.Renders,
			symbol.FactDefined:          render.Refuses,
			symbol.FactVariance:         render.Refuses,
			symbol.FactTypeParamDefault: render.Refuses,
			symbol.FactConstParam:       render.Refuses,
			symbol.FactComputed:         render.Rejects,
			symbol.FactUnderlying:       render.Refuses,
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindMethod: {
				symbol.FactMutability: render.Refuses, // no index signatures
			},
			symbol.KindEnumVariant: {
				symbol.FactValue: render.Refuses, // the constructor form
			},
			symbol.KindSum: {
				symbol.FactMethods: render.Refuses, // no per-variant bodies
			},
			symbol.KindFile: {
				symbol.FactAnnotations: render.Refuses, // no header of a generator
			},
		},
	}
}
