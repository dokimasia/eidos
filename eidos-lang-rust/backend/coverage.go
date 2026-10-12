// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares the fact coverage of Rust, the feature table as data:
// the guard of the render reads it, and the conformance suite checks it
// total and the rendered findings against it.
//
//   - Final takes the [render.Holds] verdict, because nothing subclasses a
//     struct and nothing overrides an inherent method. The final of a trait
//     method refuses through its keyword helper, because every
//     implementation may override a trait method.
//   - A trait widens through supertraits, so the extends of an interface
//     renders while the extends of a struct refuses. Implements refuses,
//     because the backend writes no impl block for a stated contract.
//   - The nested types of a trait render as associated types where the
//     nested types of every other kind refuse, because Rust nests nothing
//     else.
//   - Throws renders through the Result fold that the lowering spells.
//   - The value of an enum variant renders as its discriminant where the
//     initializer of a field refuses, because a struct declares no field
//     defaults.
//   - The default of a type parameter renders on a type definition. On a
//     function, a method and an associated type it refuses through the
//     helper of the parameter list, because Rust takes none there.
//   - Nothing promotes members, so the coverage refuses an embed with
//     every fact on it.
//   - The underlying type of an enum renders as its repr attribute. A
//     computed member has no name in Rust, so the declaration does not
//     render with the fact or without it.
//
// Coverage builds the table on every call, which the caller keeps: four
// allocations of the fact map, and two of the exception map with two for
// each of its five kinds, 16 allocations.
func Coverage() render.Coverage {
	return render.Coverage{
		RefusesEmbeds: true,
		Facts: map[symbol.Fact]render.Verdict{
			symbol.FactVisibility:       render.Renders,
			symbol.FactAsync:            render.Renders,
			symbol.FactAccessor:         render.Refuses,
			symbol.FactIndexer:          render.Refuses,
			symbol.FactConstructs:       render.Refuses,
			symbol.FactHardPrivate:      render.Refuses,
			symbol.FactConstEnum:        render.Refuses,
			symbol.FactOptional:         render.Refuses,
			symbol.FactTypeParams:       render.Renders,
			symbol.FactMultiReturn:      render.Renders,
			symbol.FactThrows:           render.Renders,
			symbol.FactAnnotations:      render.Renders,
			symbol.FactLevel:            render.Renders,
			symbol.FactAbstract:         render.Refuses,
			symbol.FactFinal:            render.Holds,
			symbol.FactSealed:           render.Refuses,
			symbol.FactPermits:          render.Refuses,
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
			symbol.FactComputed:         render.Rejects,
			symbol.FactUnderlying:       render.Renders,
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
				symbol.FactLevel:   render.Refuses, // no static types
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
