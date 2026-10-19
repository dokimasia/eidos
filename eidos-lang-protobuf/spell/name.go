// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// caser is the case conversion of protobuf. The style guide writes an
// abbreviation as one word, as GetDnsRequest, so the conversion recognises
// no initialism. A name that the source declares in its style, such as
// HttpRule, keeps its spelling.
var caser = naming.New()

// Name spells one declared name in the convention of protobuf's style
// guide. host is the kind of the declaration that declares the name, and
// [symbol.KindInvalid] at file level. The kinds take these spellings:
//
//   - a message, an enum, a service and an rpc take Pascal case, and so
//     does a sum at file level, which protobuf writes as a message;
//   - a sum that a message declares is a oneof and takes snake case, as a
//     field, an extension, a member of a oneof, a parameter and a named
//     result do;
//   - an enum value takes screaming snake case;
//   - a type parameter keeps its spelling.
//
// The conversion raises no initialism, so get_dns_request becomes
// GetDnsRequest, and a name in its style, such as HttpRule, keeps its
// spelling.
//
// Every other kind is a kind that the backend refuses, and it takes snake
// case. A name at a visibility other than public and the unstated one is
// refused, because protobuf declares every name in public. A name that is
// not an identifier is refused, because protobuf has no quoted name, and
// so is a name whose spelling comes out empty, such as "__".
//
// Name does not join the name of an enum to its values, because the hook
// receives the kind of the host and not its name.
//
// # Allocation contract
//
// Name allocates a converted spelling, one allocation, and nothing for a
// name the convention spells as it is. A refusal allocates its error.
func Name(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
	if v != symbol.VisibilityUnknown && v != symbol.VisibilityPublic {
		return "", fmt.Errorf(refusalPrefix+"%s is not public, and protobuf declares no other scope", name)
	}
	if !naming.IsIdentifier(name) {
		return "", fmt.Errorf(refusalPrefix+"%q is not an identifier, and protobuf has no quoted name", name)
	}
	var spelled string
	switch {
	case kind == symbol.KindTypeParam:
		spelled = name
	case kind == symbol.KindSum && host == symbol.KindStruct:
		spelled = caser.Snake(name)
	case kind == symbol.KindStruct || kind == symbol.KindEnum || kind == symbol.KindInterface ||
		kind == symbol.KindMethod || kind == symbol.KindSum || kind == symbol.KindAlias:
		spelled = caser.Pascal(name)
	case kind == symbol.KindEnumVariant:
		spelled = caser.ScreamingSnake(name)
	default:
		spelled = caser.Snake(name)
	}
	if spelled == "" {
		return "", fmt.Errorf(refusalPrefix+"%q spells nothing in protobuf's convention", name)
	}
	return spelled, nil
}
