// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import "strconv"

// RouteInput is one input of the layout. The zero value is no input.
type RouteInput uint8

const (
	// RouteDefault is the default policy, alongside-source.
	RouteDefault RouteInput = 1
	// RouteHandler is the family that the handler of the plugin chose.
	RouteHandler RouteInput = 2
	// RouteSource is the directory of the source of the unit.
	RouteSource RouteInput = 3
	// RouteTarget is the filename that the target spells.
	RouteTarget RouteInput = 4
	// RoutePlan is the layout of the plan.
	RoutePlan RouteInput = 5
	// RouteGenerator is the refinement of the generator of the unit.
	RouteGenerator RouteInput = 6
	// RouteFamily is the refinement of the family of the unit.
	RouteFamily RouteInput = 7
	// RoutePackage is the path of the kernel out directive on the package
	// of the origin.
	RoutePackage RouteInput = 8
	// RouteOut is the kernel out directive on the origin.
	RouteOut RouteInput = 9
	// RouteDirective is the key out= or tag= on a directive of the plugin.
	RouteDirective RouteInput = 10
)

// String returns the name of the input for a report, such as "generator",
// and "package-out" for [RoutePackage]. For an undeclared input, the zero
// value included, it returns the number in the form RouteInput(n).
func (i RouteInput) String() string {
	switch i {
	case RouteDefault:
		return "default"
	case RouteHandler:
		return "handler"
	case RouteSource:
		return "source"
	case RouteTarget:
		return "target"
	case RoutePlan:
		return "plan"
	case RouteGenerator:
		return "generator"
	case RouteFamily:
		return "family"
	case RoutePackage:
		return "package-out"
	case RouteOut:
		return "out"
	case RouteDirective:
		return "directive"
	default:
		return "RouteInput(" + strconv.Itoa(int(i)) + ")"
	}
}

// Valid reports whether the input is one of the declared inputs. The zero
// value is not.
func (i RouteInput) Valid() bool {
	return i >= RouteDefault && i <= RouteDirective
}

// Route records how the layout placed a unit. Each field is the input that
// decided one part of the path of the file of the unit. The zero Route is
// the route of a unit that the layout has not placed, such as a unit of the
// emit store.
type Route struct {
	// Family is the input that decided the family of the unit.
	Family RouteInput
	// Policy is the input that decided the policy of the directory. It is
	// zero for a per-plan family and for a path override, because no policy
	// applies to either.
	Policy RouteInput
	// Dir is the input that decided the directory of the file.
	Dir RouteInput
	// Name is the input that decided the filename.
	Name RouteInput
}
