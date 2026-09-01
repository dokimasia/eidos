// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Coverage declares Go's fact coverage, the feature table as data:
// the render's guard reads it, and the conformance suite holds it
// total and the rendered findings against it.
//
// Visibility renders through the respell, which is where Go's case
// convention lives. Final and a struct's implements hold: nothing
// subclasses, and satisfaction is structural. Throws
// lowers into the appended error return. A field's initializer refuses where a
// variable's renders, because Go declares no field defaults.
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
			symbol.FactMultiReturn:      render.Renders,
			symbol.FactThrows:           render.Renders,
			symbol.FactAnnotations:      render.Renders,
			symbol.FactLevel:            render.Refuses,
			symbol.FactAbstract:         render.Refuses,
			symbol.FactFinal:            render.Holds,
			symbol.FactSealed:           render.Refuses,
			symbol.FactPermits:          render.Refuses,
			symbol.FactOverride:         render.Refuses,
			symbol.FactDefaultBody:      render.Refuses,
			symbol.FactLabel:            render.Refuses,
			symbol.FactParamDefault:     render.Refuses,
			symbol.FactVariadic:         render.Renders,
			symbol.FactNamedReturn:      render.Renders,
			symbol.FactFields:           render.Refuses,
			symbol.FactMethods:          render.Refuses,
			symbol.FactValue:            render.Renders,
			symbol.FactComment:          render.Renders,
			symbol.FactMutability:       render.Refuses,
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
		},
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindField: {
				symbol.FactValue: render.Refuses, // no field defaults
			},
		},
	}
}
