// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/internal/value"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// refusalPrefix opens every refusal of the package. It is the identity of
// the language, as the refusals of every satellite open.
const refusalPrefix = string(graphql.Lang) + ": "

// The arguments of the markers as a refusal states them.
const (
	takesNothing   = "no argument"
	takesOperation = "one argument, query, mutation or subscription"
	takesType      = "one argument, the name of a type"
	takesLocations = "one or more locations of a directive"
)

// locations are the locations of a directive of GraphQL, which a directive
// definition lists after on.
var locations = []ast.DirectiveLocation{
	ast.LocationQuery, ast.LocationMutation, ast.LocationSubscription, ast.LocationField,
	ast.LocationFragmentDefinition, ast.LocationFragmentSpread, ast.LocationInlineFragment,
	ast.LocationVariableDefinition, ast.LocationSchema, ast.LocationScalar, ast.LocationObject,
	ast.LocationFieldDefinition, ast.LocationArgumentDefinition, ast.LocationInterface, ast.LocationUnion,
	ast.LocationEnum, ast.LocationEnumValue, ast.LocationInputObject, ast.LocationInputFieldDefinition,
}

// arity is the rule of the arguments of a marker.
type arity uint8

const (
	// noArgs takes no argument.
	noArgs arity = iota + 1
	// operationArg takes one argument, a kind of operation.
	operationArg
	// nameArg takes one argument, a GraphQL name.
	nameArg
	// locationArgs takes one or more arguments, each a location of a
	// directive.
	locationArgs
)

// admits reports whether args are arguments that the rule r admits.
func (r arity) admits(args []string) bool {
	switch r {
	case operationArg:
		return len(args) == 1 && graphql.Operation(args[0]).Valid()
	case nameArg:
		return len(args) == 1 && naming.IsIdentifier(args[0])
	case locationArgs:
		return len(args) > 0 && !slices.ContainsFunc(args, func(arg string) bool {
			return !slices.Contains(locations, ast.DirectiveLocation(arg))
		})
	default:
		return len(args) == 0
	}
}

// markerSet is the set of the markers that one declaration states, one bit
// for each marker.
type markerSet uint16

// The bits of the markers in a [markerSet].
const (
	inputBit markerSet = 1 << iota
	extendBit
	operationBit
	anonymousBit
	fragmentBit
	queryBit
	mutationBit
	subscriptionBit
	directiveBit
	repeatableBit
)

// checkAnnotations checks the annotations as of the declaration name of the
// kind kind, and returns the set of the markers that as states. An
// annotation whose name starts with [graphql.MarkerPrefix] is a marker: it
// is a marker of the satellite, it marks a declaration of a kind that its
// contract names, it takes the arguments of its contract, and a declaration
// states it once. Every other annotation is a directive: its name is a
// GraphQL name, and each argument is the name of an argument,
// [graphql.NameSep] and a constant value of GraphQL. A directive can repeat.
// checkAnnotations allocates what the parse of each argument of a directive
// allocates, and a refusal allocates its error.
func checkAnnotations(kind symbol.Kind, name string, as symbol.Annotations) (markerSet, error) {
	var set markerSet
	for _, a := range as {
		if !strings.HasPrefix(a.Name, graphql.MarkerPrefix) {
			if err := checkDirective(name, a); err != nil {
				return 0, err
			}
			continue
		}
		rule, known := markerRules[graphql.Marker(a.Name)]
		switch {
		case !known:
			return 0, fmt.Errorf(refusalPrefix+"%s is no marker of the satellite, and %s states it", a.Name, name)
		case !slices.Contains(rule.kinds, kind):
			return 0, fmt.Errorf(refusalPrefix+"the marker %s marks no %s, and %s is one", a.Name, kind, name)
		case !rule.args.admits(a.Args):
			return 0, fmt.Errorf(refusalPrefix+"the marker %s takes %s, and %s states %q",
				a.Name, rule.takes, name, a.Args)
		case set&rule.bit != 0:
			return 0, fmt.Errorf(refusalPrefix+"%s states the marker %s twice", name, a.Name)
		}
		set |= rule.bit
	}
	return set, nil
}

// markersOf returns the set of the markers that the annotations as state. An
// annotation that is not a marker of the satellite adds nothing. It
// allocates nothing.
func markersOf(as symbol.Annotations) markerSet {
	var set markerSet
	for _, a := range as {
		set |= markerRules[graphql.Marker(a.Name)].bit
	}
	return set
}

// markerRule is the contract of one marker: the kinds of the declarations
// that it marks, its bit and the rule of its arguments.
type markerRule struct {
	kinds []symbol.Kind
	// takes states the arguments of the marker in a refusal.
	takes string
	bit   markerSet
	args  arity
}

// markerRules are the contracts of the markers of the satellite.
var markerRules = map[graphql.Marker]markerRule{
	graphql.InputMarker: {
		kinds: []symbol.Kind{symbol.KindStruct, symbol.KindSum}, takes: takesNothing, bit: inputBit, args: noArgs,
	},
	graphql.ExtendMarker: {
		kinds: []symbol.Kind{
			symbol.KindFile, symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindAlias,
		},
		takes: takesNothing, bit: extendBit, args: noArgs,
	},
	graphql.OperationMarker: {
		kinds: []symbol.Kind{symbol.KindStruct, symbol.KindFunction}, takes: takesOperation,
		bit: operationBit, args: operationArg,
	},
	graphql.AnonymousMarker: {
		kinds: []symbol.Kind{symbol.KindFunction}, takes: takesNothing, bit: anonymousBit, args: noArgs,
	},
	graphql.FragmentMarker: {
		kinds: []symbol.Kind{symbol.KindFunction}, takes: takesType, bit: fragmentBit, args: nameArg,
	},
	graphql.QueryMarker: {
		kinds: []symbol.Kind{symbol.KindFile}, takes: takesType, bit: queryBit, args: nameArg,
	},
	graphql.MutationMarker: {
		kinds: []symbol.Kind{symbol.KindFile}, takes: takesType, bit: mutationBit, args: nameArg,
	},
	graphql.SubscriptionMarker: {
		kinds: []symbol.Kind{symbol.KindFile}, takes: takesType, bit: subscriptionBit, args: nameArg,
	},
	graphql.DirectiveMarker: {
		kinds: []symbol.Kind{symbol.KindInterface}, takes: takesLocations, bit: directiveBit, args: locationArgs,
	},
	graphql.RepeatableMarker: {
		kinds: []symbol.Kind{symbol.KindInterface}, takes: takesNothing, bit: repeatableBit, args: noArgs,
	},
}

// checkDirective checks the annotation a of the declaration name as an
// applied directive: a GraphQL name, and arguments that are the name of an
// argument, [graphql.NameSep] and a constant value of GraphQL.
func checkDirective(name string, a symbol.Annotation) error {
	if !naming.IsIdentifier(a.Name) {
		return fmt.Errorf(refusalPrefix+"the name of a directive is a GraphQL name, and %s states the annotation %q",
			name, a.Name)
	}
	for _, arg := range a.Args {
		argName, text, cut := strings.Cut(arg, graphql.NameSep)
		if !cut || !naming.IsIdentifier(argName) {
			return fmt.Errorf(refusalPrefix+"an argument of a directive is a name, a colon and a value, "+
				"and the directive %s of %s states %q", a.Name, name, arg)
		}
		if _, err := value.Parse(text); err != nil {
			return fmt.Errorf(refusalPrefix+"the argument %s of the directive %s of %s is no value of GraphQL: %w",
				argName, a.Name, name, err)
		}
	}
	return nil
}

// markerArg returns the first argument of the marker m in the annotations
// as, and the empty text where as does not state m or m has no argument. It
// allocates nothing.
func markerArg(as symbol.Annotations, m graphql.Marker) string {
	i := slices.IndexFunc(as, func(a symbol.Annotation) bool { return a.Name == string(m) })
	if i < 0 || len(as[i].Args) == 0 {
		return ""
	}
	return as[i].Args[0]
}
