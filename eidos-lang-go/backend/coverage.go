// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares the fact coverage of Go, the feature table as data.
// The guard of the render reads it, and the conformance suite checks that
// it is total and that the rendered findings match it.
//
// Visibility renders through the respell, which applies the case
// convention of Go. Final and the implements of a struct take the
// [render.Holds] verdict, because nothing subclasses and satisfaction is
// structural. Throws lowers into the appended error return. The
// mutability of a field or a variable renders, because Go writes a mutable
// binding without a keyword, and [Guard] returns an error for an immutable
// one. The initializer of a field refuses where the initializer of a
// variable renders, because Go declares no field defaults. The annotations
// of a parameter refuse, because Go has no syntax for them.
//
// Go writes no async, override, default body, type level or computed
// name, and a declaration without such a fact is a declaration of another
// meaning. These facts take the [render.Rejects] verdict, and so does the
// mutability of an index signature.
//
// Coverage builds the table on every call, which the caller keeps: the
// fact map's four allocations, and the exception map's two with its three
// kinds' two each, 12 allocations.
func Coverage() render.Coverage {
	return render.Coverage{
		Facts: map[symbol.Fact]render.Verdict{
			symbol.FactVisibility:       render.Renders,
			symbol.FactAsync:            render.Rejects,
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
			symbol.FactLevel:            render.Rejects,
			symbol.FactAbstract:         render.Refuses,
			symbol.FactFinal:            render.Holds,
			symbol.FactSealed:           render.Refuses,
			symbol.FactPermits:          render.Refuses,
			symbol.FactOverride:         render.Rejects,
			symbol.FactDefaultBody:      render.Rejects,
			symbol.FactLabel:            render.Refuses,
			symbol.FactParamDefault:     render.Refuses,
			symbol.FactVariadic:         render.Renders,
			symbol.FactNamedReturn:      render.Renders,
			symbol.FactFields:           render.Refuses,
			symbol.FactMethods:          render.Refuses,
			symbol.FactValue:            render.Renders,
			symbol.FactComment:          render.Renders,
			symbol.FactMutability:       render.Renders,
			symbol.FactTag:              render.Renders,
			symbol.FactTypes:            render.Refuses,
			symbol.FactEmbeds:           render.Renders,
			symbol.FactExtends:          render.Renders,
			symbol.FactImplements:       render.Holds,
			symbol.FactProperties:       render.Refuses,
			symbol.FactDefined:          render.Renders,
			symbol.FactVariance:         render.Refuses,
			symbol.FactTypeParamDefault: render.Refuses,
			symbol.FactConstParam:       render.Refuses,
			symbol.FactComputed:         render.Rejects,
			symbol.FactUnderlying:       render.Renders,
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindField: {
				symbol.FactValue: render.Refuses, // no field defaults
			},
			symbol.KindParam: {
				symbol.FactAnnotations: render.Refuses, // no parameter annotations
			},
			symbol.KindMethod: {
				symbol.FactMutability: render.Rejects, // no index signatures
			},
		},
	}
}
