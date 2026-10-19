// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// headerName names the header of a file in a refusal.
const headerName = "the header"

// The syntax of an option name and of an import.
const (
	// extensionOpen and extensionClose enclose the name of a custom option,
	// such as (buf.validate.field).
	extensionOpen  = '('
	extensionClose = ')'
	// importSep separates the modifier of an import from its path in the
	// argument of an import marker, as in public acme/types.proto.
	importSep = " "
)

// The modifiers of an import that an import marker states.
const (
	publicModifier = "public"
	weakModifier   = "weak"
)

// argCount is the number of arguments that a marker takes.
type argCount uint8

// The numbers of arguments of the markers.
const (
	noArgument argCount = iota + 1
	oneArgument
	someArguments
)

// markers maps each kind that takes markers to the markers that it takes,
// with the number of arguments of each. A kind outside the map takes no
// marker, and its annotations are options.
var markers = map[symbol.Kind]map[protobuf.Marker]argCount{
	symbol.KindFile: {
		protobuf.SyntaxMarker: oneArgument, protobuf.PackageMarker: oneArgument, protobuf.ImportMarker: oneArgument,
	},
	symbol.KindStruct: {
		protobuf.ReservedMarker:   someArguments,
		protobuf.ExtensionsMarker: someArguments,
		protobuf.LocalMarker:      noArgument,
	},
	symbol.KindEnum: {
		protobuf.ReservedMarker: someArguments, protobuf.ClosedMarker: noArgument, protobuf.LocalMarker: noArgument,
	},
	symbol.KindField:      {protobuf.RequiredMarker: noArgument, protobuf.DelimitedMarker: noArgument},
	symbol.KindSumVariant: {protobuf.NumberMarker: oneArgument},
}

// nouns names each kind that states annotations in a refusal.
var nouns = map[symbol.Kind]string{
	symbol.KindFile:        "a header",
	symbol.KindStruct:      "a message",
	symbol.KindEnum:        "an enum",
	symbol.KindEnumVariant: "an enum value",
	symbol.KindField:       "a field",
	symbol.KindSum:         "a oneof",
	symbol.KindSumVariant:  "a variant",
	symbol.KindInterface:   "a service",
	symbol.KindMethod:      "an rpc",
	symbol.KindParam:       "a parameter",
}

// countWords states each number of arguments in a refusal.
var countWords = map[argCount]string{
	noArgument:    "no argument",
	oneArgument:   "one argument",
	someArguments: "one or more arguments",
}

// checkAnnotations returns an error for an annotation of a declaration that
// protobuf has no form of. who names the declaration, and kind is its kind.
// An annotation whose name starts with [protobuf.MarkerPrefix] is a marker,
// and every other annotation is an option.
//
// Error modes, each a refusal of the declaration:
//   - a marker that the satellite does not declare;
//   - a marker that the kind does not take, or with another number of
//     arguments than the marker takes;
//   - an import marker with a modifier other than public, weak and option,
//     or without a path;
//   - an option without exactly one value, or with a name that is not the
//     name of an option.
//
// checkAnnotations allocates nothing. A refusal allocates its error.
func checkAnnotations(kind symbol.Kind, who string, as symbol.Annotations) error {
	for _, a := range as {
		name, marked := strings.CutPrefix(a.Name, protobuf.MarkerPrefix)
		if !marked {
			switch {
			case len(a.Args) != 1:
				return fmt.Errorf(
					refusalPrefix+"the option %s takes one value, and %s states %d",
					a.Name,
					who,
					len(a.Args),
				)
			case !optionName(a.Name):
				return fmt.Errorf(refusalPrefix+"%q is not the name of an option, and %s states it", a.Name, who)
			}
			continue
		}
		m := protobuf.Marker(a.Name)
		if !m.Valid() {
			return fmt.Errorf(refusalPrefix+"%s%s is not a marker of protobuf, and %s states it",
				protobuf.MarkerPrefix, name, who)
		}
		want, takes := markers[kind][m]
		switch {
		case !takes:
			return fmt.Errorf(refusalPrefix+"%s does not take the marker %s, and %s states it", nouns[kind], m, who)
		case want == noArgument && len(a.Args) != 0, want == oneArgument && len(a.Args) != 1,
			want == someArguments && len(a.Args) == 0:
			return fmt.Errorf(
				refusalPrefix+"the marker %s takes %s, and %s states %d",
				m,
				countWords[want],
				who,
				len(a.Args),
			)
		case m == protobuf.ImportMarker:
			if err := checkImport(a.Args[0]); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkImport returns an error for the argument arg of an import marker
// that is not a path after an optional modifier public, weak or option.
func checkImport(arg string) error {
	modifier, path := importOf(arg)
	switch {
	case modifier == "":
		return nil
	case modifier != publicModifier && modifier != weakModifier && modifier != optionModifier:
		return fmt.Errorf(refusalPrefix+"an import takes the modifier public, weak or option, and %s states %s",
			headerName, modifier)
	case path == "":
		return fmt.Errorf(refusalPrefix+"an import states a path, and %s states the modifier %s alone",
			headerName, modifier)
	default:
		return nil
	}
}

// optionName reports whether name is the name of an option as protobuf
// writes it: parts joined by dots, where each part is an identifier or the
// name of a custom option in parentheses, such as
// (buf.validate.field).string.min_len. The name in parentheses is dotted
// identifiers, after a dot that makes it fully qualified. It allocates
// nothing.
func optionName(name string) bool {
	for rest := name; ; {
		if rest != "" && rest[0] == extensionOpen {
			end := strings.IndexByte(rest, extensionClose)
			if end < 0 || !dottedName(strings.TrimPrefix(rest[1:end], protobuf.NameSep)) {
				return false
			}
			rest = rest[end+1:]
		} else {
			end := strings.Index(rest, protobuf.NameSep)
			if end < 0 {
				end = len(rest)
			}
			if !naming.IsIdentifier(rest[:end]) {
				return false
			}
			rest = rest[end:]
		}
		if rest == "" {
			return true
		}
		next, dotted := strings.CutPrefix(rest, protobuf.NameSep)
		if !dotted || next == "" {
			return false
		}
		rest = next
	}
}

// dottedName reports whether name is identifiers joined by dots.
func dottedName(name string) bool {
	for part := range strings.SplitSeq(name, protobuf.NameSep) {
		if !naming.IsIdentifier(part) {
			return false
		}
	}
	return true
}
