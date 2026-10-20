// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares the fact coverage of protobuf, the feature table as
// data. The guard of the render reads it, and the conformance suite checks
// that it is total and that the rendered findings match it.
//
//   - Visibility takes [render.Holds], because a message without the marker
//     protobuf:local is visible to every importing file, and the respell
//     refuses a name at a visibility other than public.
//   - Mutability takes Holds, because a message is mutable in every
//     protobuf runtime, and the lowering refuses a read-only field.
//   - Async and Throws take Holds, because an rpc crosses a connection and
//     can fail with a status. Abstract takes Holds on a method, because an
//     rpc has no body, and Final holds because nothing overrides an rpc or
//     extends a message.
//   - The level of a nested message holds, because a nested message binds
//     to no instance of its message. A static field or method is not data
//     of a message, so the declaration does not render with the fact or
//     without it.
//   - The methods of a struct refuse, so a struct with methods renders as a
//     message without them. The fields of an enum refuse in the same way.
//   - MultiReturn, NamedReturn and Variadic render through the requests and
//     the responses that the lowering declares. The nested types of a
//     message render, where a service nests nothing.
//   - Nothing promotes members, so the coverage refuses an embed with every
//     fact on it.
//   - A type parameter, an accessor, an index signature, a construct
//     signature and a computed name have no form in a schema, so the
//     declaration does not render with the fact or without it.
//   - The passing of a parameter takes the [render.Holds] verdict, because
//     an rpc takes one request.
//     A wire name renders as the name of its field, and the lowering
//     refuses a wire name that is not an identifier.
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
			symbol.FactOptional:         render.Refuses,
			symbol.FactMutability:       render.Holds,
			symbol.FactComputed:         render.Rejects,
			symbol.FactLabel:            render.Refuses,
			symbol.FactParamDefault:     render.Refuses,
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
			symbol.FactTag:              render.Renders,
			symbol.FactSealed:           render.Refuses,
			symbol.FactTypes:            render.Renders,
			symbol.FactEmbeds:           render.Refuses,
			symbol.FactExtends:          render.Refuses,
			symbol.FactImplements:       render.Refuses,
			symbol.FactPermits:          render.Refuses,
			symbol.FactProperties:       render.Refuses,
			symbol.FactDefined:          render.Refuses,
			symbol.FactVariance:         render.Refuses,
			symbol.FactConstParam:       render.Refuses,
			symbol.FactTypeParamDefault: render.Refuses,
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindStruct: {
				symbol.FactLevel: render.Holds, // a nested message binds to no instance
			},
			symbol.KindMethod: {
				symbol.FactAbstract: render.Holds, // an rpc has no body
			},
			symbol.KindField: {
				symbol.FactOptional: render.Renders, // the presence of the field
			},
			symbol.KindInterface: {
				symbol.FactTypes: render.Refuses, // a service nests nothing
			},
		},
	}
}
