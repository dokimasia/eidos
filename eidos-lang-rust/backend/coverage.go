// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares Rust's fact coverage, the feature table as
// data: the render's guard reads it, and the conformance suite
// checks it total and the rendered findings against it.
//
//   - Final takes the [render.Holds] verdict, because nothing
//     subclasses a struct and nothing overrides an inherent method.
//     A trait method's final refuses through its keyword helper,
//     because every implementation may override a trait method.
//   - A trait widens through supertraits, so an interface's extends
//     renders while a struct's refuses. Implements refuses, because
//     the backend writes no impl block for a stated contract.
//   - A trait's nested types render as associated types where every
//     other kind's refuse, because Rust nests nothing else.
//   - Throws renders through the Result fold the lowering spells.
//   - An enum variant's value renders as its discriminant where a
//     field's initializer refuses, because a struct declares no
//     field defaults.
//   - A type parameter's default renders on a type definition. On a
//     function, a method and an associated type it refuses through
//     the parameter-list helper, because Rust takes none there.
func Coverage() render.Coverage {
	return render.Coverage{
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
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			// An embed refuses whole, so the facts on it refuse with it.
			symbol.KindEmbed: {
				symbol.FactComment:     render.Refuses,
				symbol.FactTag:         render.Refuses,
				symbol.FactAnnotations: render.Refuses,
			},
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
