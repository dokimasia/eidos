// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares the fact coverage of GraphQL, the feature table as data.
// The guard of the render reads it, and the conformance suite checks that it
// is total and that the rendered findings match it.
//
//   - Visibility takes [render.Holds], because a schema exposes every type
//     that it declares, and the respell refuses a name at a visibility other
//     than public.
//   - Async takes Holds, because a resolver can return its value
//     asynchronously. Throws takes Holds, because every field can fail with
//     an error in the response.
//   - The methods of a struct render as fields with arguments, and the fields
//     of an interface render as its fields. The nested types of a type render
//     at file level, because GraphQL has no nested types.
//   - Variadic renders as a list argument, and MultiReturn and NamedReturn
//     render through the payloads of the lowering. On a function, MultiReturn
//     holds, because the selection set of an executable definition states
//     its result.
//   - A return writes no comment, because a field has one trailing comment,
//     which is the comment of its method.
//   - A value of an enum refuses, because GraphQL serializes a value by its
//     name.
//   - The level of a nested type holds, because a type at file level binds
//     to no instance. A static field or method has no form in a schema, so
//     the declaration does not render with the fact or without it.
//   - A type parameter, an accessor, an index signature, a construct
//     signature and a computed name have no form in a schema either.
//   - The passing of a parameter takes the [render.Holds] verdict, because
//     a caller always names an argument. A wire name renders as the name of its field, and the
//     lowering refuses a wire name that is not a GraphQL name or that
//     starts with two underscores, because a schema cannot declare it.
//
// Coverage builds the table on every call, which the caller keeps.
func Coverage() render.Coverage {
	return render.Coverage{
		RefusesEmbeds: true,
		Facts: map[symbol.Fact]render.Verdict{
			symbol.FactComment:          render.Renders,
			symbol.FactVisibility:       render.Holds,
			symbol.FactAsync:            render.Holds,
			symbol.FactTypeParams:       render.Rejects,
			symbol.FactMultiReturn:      render.Renders,
			symbol.FactThrows:           render.Holds,
			symbol.FactAnnotations:      render.Renders,
			symbol.FactLevel:            render.Rejects,
			symbol.FactAbstract:         render.Refuses,
			symbol.FactFinal:            render.Holds,
			symbol.FactOverride:         render.Refuses,
			symbol.FactDefaultBody:      render.Refuses,
			symbol.FactAccessor:         render.Rejects,
			symbol.FactIndexer:          render.Rejects,
			symbol.FactConstructs:       render.Rejects,
			symbol.FactHardPrivate:      render.Refuses,
			symbol.FactOptional:         render.Renders,
			symbol.FactMutability:       render.Holds,
			symbol.FactComputed:         render.Rejects,
			symbol.FactLabel:            render.Refuses,
			symbol.FactParamDefault:     render.Renders,
			symbol.FactVariadic:         render.Renders,
			symbol.FactPassing:          render.Holds,
			symbol.FactWire:             render.Renders,
			symbol.FactNamedReturn:      render.Renders,
			symbol.FactConstEnum:        render.Refuses,
			symbol.FactUnderlying:       render.Refuses,
			symbol.FactFields:           render.Refuses,
			symbol.FactMethods:          render.Refuses,
			symbol.FactValue:            render.Renders,
			symbol.FactReceives:         render.Renders,
			symbol.FactTag:              render.Refuses,
			symbol.FactSealed:           render.Refuses,
			symbol.FactTypes:            render.Renders,
			symbol.FactEmbeds:           render.Refuses,
			symbol.FactExtends:          render.Renders,
			symbol.FactImplements:       render.Renders,
			symbol.FactPermits:          render.Refuses,
			symbol.FactProperties:       render.Renders,
			symbol.FactDefined:          render.Renders,
			symbol.FactVariance:         render.Refuses,
			symbol.FactConstParam:       render.Refuses,
			symbol.FactTypeParamDefault: render.Refuses,
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindStruct: {
				symbol.FactMethods: render.Renders, // the fields with arguments of a type
				symbol.FactExtends: render.Refuses, // a type implements interfaces and extends nothing
				symbol.FactLevel:   render.Holds,   // a type at file level binds to no instance
			},
			symbol.KindMethod: {
				symbol.FactAbstract: render.Holds, // a field has no body
			},
			symbol.KindFunction: {
				symbol.FactMultiReturn: render.Holds, // the selection set states the result
			},
			symbol.KindEnumVariant: {
				symbol.FactValue: render.Refuses, // a value serializes by its name
			},
			symbol.KindReturn: {
				symbol.FactComment: render.Refuses, // a field has the one comment of its method
			},
		},
	}
}
