// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares the fact coverage of Python, the feature table as data.
// The guard of the render reads it, and the conformance suite checks that
// it is total and that the rendered findings match it.
//
// A visibility renders through the name, with one underscore for a member
// that is not public and two for a private member. Several results render
// as a tuple, which is how Python returns two values. An embed renders as a
// base, because the method resolution order of Python finds the members of
// a base as promotion finds the members of an embedded type, and the
// comment and the annotations of an embed refuse, because a base has
// neither. A nested class is always at the level of its type, so the level
// of a struct holds. Python raises without declaring, so the failures of a
// signature hold, and so do a const enum and a sealed type, which Python
// states nowhere. A field that another type receives rejects the
// declaration, because Python declares a member in its class alone. A
// result of a signature has no comment of its own. Python passes an
// argument by position or by keyword, so the passing of a parameter
// renders. The fields of an enum render as attributes of its members that
// are no members. The contracts of an enum refuse, because the metaclass of
// an enum conflicts with the metaclass of a Protocol. Each variant class of
// a sum takes the contracts of the sum as its bases. An overload signature
// renders under the decorator overload of typing.
//
// Coverage builds the table on every call, which the caller keeps.
func Coverage() render.Coverage {
	return render.Coverage{
		Facts: map[symbol.Fact]render.Verdict{
			symbol.FactComment:          render.Renders,
			symbol.FactVisibility:       render.Renders,
			symbol.FactAsync:            render.Renders,
			symbol.FactTypeParams:       render.Renders,
			symbol.FactMultiReturn:      render.Renders,
			symbol.FactThrows:           render.Holds,
			symbol.FactAnnotations:      render.Renders,
			symbol.FactLevel:            render.Renders,
			symbol.FactAbstract:         render.Renders,
			symbol.FactFinal:            render.Renders,
			symbol.FactOverride:         render.Renders,
			symbol.FactOverload:         render.Renders,
			symbol.FactDefaultBody:      render.Renders,
			symbol.FactAccessor:         render.Renders,
			symbol.FactIndexer:          render.Renders,
			symbol.FactConstructs:       render.Renders,
			symbol.FactHardPrivate:      render.Renders,
			symbol.FactOptional:         render.Renders,
			symbol.FactMutability:       render.Renders,
			symbol.FactComputed:         render.Refuses,
			symbol.FactLabel:            render.Refuses,
			symbol.FactParamDefault:     render.Renders,
			symbol.FactVariadic:         render.Renders,
			symbol.FactPassing:          render.Renders,
			symbol.FactNamedReturn:      render.Refuses,
			symbol.FactConstEnum:        render.Holds,
			symbol.FactUnderlying:       render.Renders,
			symbol.FactFields:           render.Renders,
			symbol.FactMethods:          render.Renders,
			symbol.FactValue:            render.Renders,
			symbol.FactWire:             render.Renders,
			symbol.FactReceives:         render.Rejects,
			symbol.FactTag:              render.Refuses,
			symbol.FactSealed:           render.Holds,
			symbol.FactTypes:            render.Renders,
			symbol.FactEmbeds:           render.Renders,
			symbol.FactExtends:          render.Renders,
			symbol.FactImplements:       render.Renders,
			symbol.FactPermits:          render.Refuses,
			symbol.FactProperties:       render.Renders,
			symbol.FactDefined:          render.Renders,
			symbol.FactVariance:         render.Renders,
			symbol.FactConstParam:       render.Refuses,
			symbol.FactTypeParamDefault: render.Renders,
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindStruct: {
				symbol.FactLevel: render.Holds, // a nested class is at the level of its type
			},
			symbol.KindEnum: {
				symbol.FactImplements: render.Refuses, // the metaclasses of Enum and Protocol conflict
			},
			symbol.KindSum: {
				symbol.FactMethods: render.Refuses, // a union has no members
			},
			symbol.KindMethod: {
				symbol.FactOptional: render.Refuses, // a class has every method that it declares
			},
			symbol.KindReturn: {
				symbol.FactComment: render.Refuses, // a result has no line of its own
			},
			symbol.KindEmbed: {
				symbol.FactComment:     render.Refuses, // a base has no comment
				symbol.FactAnnotations: render.Refuses, // nor a decorator
			},
		},
	}
}
