// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The syntax of the range statements of a message and of an enum.
const (
	reservedKeyword   = "reserved "
	extensionsKeyword = "extensions "
	rangeSep          = ", "
	// optionsOpen opens the last argument of an extensions marker that is
	// the options of the statement.
	optionsOpen = "["
)

// The kinds of the runs of a reserved statement.
const (
	noRun = iota
	numberRun
	nameRun
)

// reservedStatements writes the reserved statements behind indent that the
// reserved markers of the annotations as of the declaration who state.
// protobuf refuses numbers and names in one statement, so each run of
// numbers and ranges and each run of names among the arguments of a marker
// is one statement, in the order of the arguments. An argument that starts
// with a digit or a minus sign is a number or a range, and any other is a
// name. proto2 and proto3 write a name in quotes, and an edition writes it
// as an identifier.
//
// Error modes: a name that is not an identifier, because protobuf reserves
// the names of fields and values alone.
func reservedStatements(version protobuf.Edition, who string, as symbol.Annotations, indent string) (string, error) {
	var b strings.Builder
	for _, a := range as {
		if a.Name != string(protobuf.ReservedMarker) {
			continue
		}
		run := noRun
		for _, arg := range a.Args {
			kind := nameRun
			if arg != "" && (arg[0] == '-' || arg[0] >= '0' && arg[0] <= '9') {
				kind = numberRun
			} else if !naming.IsIdentifier(arg) {
				return "", fmt.Errorf(refusalPrefix+"a reserved name is an identifier, and %s reserves %q", who, arg)
			}
			if kind == run {
				b.WriteString(rangeSep)
			} else {
				if run != noRun {
					b.WriteString(statementEnd)
				}
				b.WriteString(indent)
				b.WriteString(reservedKeyword)
				run = kind
			}
			if kind == nameRun && version.Syntax {
				b.WriteString(strconv.Quote(arg))
			} else {
				b.WriteString(arg)
			}
		}
		if run != noRun {
			b.WriteString(statementEnd)
		}
	}
	return b.String(), nil
}

// extensionStatements writes one extensions statement behind indent for
// each extensions marker of the annotations as of the message who: the
// ranges of the marker, and a last argument in brackets as the options of
// the statement.
//
// Error modes: an extensions marker in proto3, which declares no extension
// range.
func extensionStatements(version protobuf.Edition, who string, as symbol.Annotations, indent string) (string, error) {
	var b strings.Builder
	for _, a := range as {
		if a.Name != string(protobuf.ExtensionsMarker) {
			continue
		}
		if version.Name == protobuf.Proto3 {
			return "", fmt.Errorf(refusalPrefix+"proto3 declares no extension range, and %s states one", who)
		}
		ranges, options := a.Args, ""
		if n := len(ranges); n > 0 && strings.HasPrefix(ranges[n-1], optionsOpen) {
			ranges, options = ranges[:n-1], ranges[n-1]
		}
		b.WriteString(indent)
		b.WriteString(extensionsKeyword)
		b.WriteString(strings.Join(ranges, rangeSep))
		if options != "" {
			b.WriteString(space)
			b.WriteString(options)
		}
		b.WriteString(statementEnd)
	}
	return b.String(), nil
}
