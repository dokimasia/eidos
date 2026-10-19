// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strconv"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The labels of a field in proto2 and proto3. The writer writes the label
// repeated of a list itself.
const (
	optionalKeyword = "optional "
	requiredKeyword = "required "
)

// The syntax of a field after its type: the assignment of its number, and
// the end of its statement.
const (
	numberAssign = " = "
	memberEnd    = ";"
)

// numberDigits is the number of decimal digits of the largest wire number.
// A field writes its number through a stack buffer of this size, which the
// concatenation of its line reads without a copy.
const numberDigits = 10

// field writes the field f in the scope s behind indent: its documentation,
// its label, its type, its name, its number, its compact options and its
// trailing comment. The scope is the scope of the block that declares the
// field. The label and the features of the field follow the version of its
// file, by the rules of [presence] and [encoding].
func (v *Vocabulary) field(s Scope, f *emit.Field, indent string) (string, error) {
	own, err := s.Within(f.Annotations)
	if err != nil {
		return "", err
	}
	label, added, err := presence(s.Version(), own.Features(), f)
	if err != nil {
		return "", err
	}
	more, err := encoding(s.Version(), own.Features(), f.Name, f.Annotations, f.Type)
	if err != nil {
		return "", err
	}
	n, err := fieldNumber(f.Name, f)
	if err != nil {
		return "", err
	}
	spelled, err := v.writer.Spell(f.Type)
	if err != nil {
		return "", err
	}
	var digits [numberDigits]byte
	return comments(f.Doc, indent) + indent + label + spelled + space + f.Name + numberAssign +
		string(strconv.AppendUint(digits[:0], uint64(n), 10)) +
		compactOptions(f.Value, f.Annotations, append(added, more...), indent) +
		memberEnd + trailing(f.Comment) + "\n", nil
}

// presence returns the label of the field f and the features that the
// backend adds to f. version is the version of the file, and features are
// the features that apply to f. A list writes its label through the writer.
//
//   - An optional takes the label optional in proto2 and proto3, and
//     explicit presence in an edition. A field that a value of its message
//     may lack is an optional.
//   - A field with the marker protobuf:required takes the label required
//     in proto2, and the presence LEGACY_REQUIRED in an edition.
//   - Any other field takes no label in proto3, and implicit presence in an
//     edition. A field of a message type has presence in every version, so
//     the backend states no presence for it.
//   - An extension has explicit presence in an edition, and states no
//     presence.
//
// presence adds the feature field_presence where the resolved value differs
// from the value that the field needs.
//
// Error modes: a field without a type, a stream, a required list, map or
// optional, a required extension, a required field in proto3, a singular
// field without a label in proto2, a default in proto3, a default of a list
// or a map or of a field without presence, and a stated presence that
// contradicts the field.
func presence(version protobuf.Edition, features protobuf.Features, f *emit.Field) (string, []feature, error) {
	t := f.Type
	if t == nil {
		return "", nil, errNoType
	}
	required := marked(f.Annotations, protobuf.RequiredMarker)
	extension := f.Receives != nil
	repeated := t.Form == symbol.FormList || t.Form == symbol.FormMap
	// A field of the optional form has presence, and so does a field that
	// a value of its message may lack, such as an optional property of
	// TypeScript.
	optional := t.Form == symbol.FormOptional || f.Optional && !repeated
	var (
		label    string
		need     protobuf.Presence
		implicit bool
	)
	switch {
	case t.Form == symbol.FormStream:
		return "", nil, fmt.Errorf(refusalPrefix+"a field of a message is not a stream, and %s states one", f.Name)
	case required && (repeated || optional):
		return "", nil, fmt.Errorf(refusalPrefix+"a required field is singular and present, and %s is not", f.Name)
	case required && extension:
		return "", nil, fmt.Errorf(refusalPrefix+"an extension is not required, and %s states the marker %s",
			f.Name, protobuf.RequiredMarker)
	case required && version.Name == protobuf.Proto3:
		return "", nil, fmt.Errorf(refusalPrefix+"proto3 has no required field, and %s states the marker %s",
			f.Name, protobuf.RequiredMarker)
	case repeated:
		// The writer writes the label of a list, and a map has none.
	case optional && version.Syntax:
		label = optionalKeyword
	case optional && !extension:
		need = protobuf.PresenceExplicit
	case required && version.Syntax:
		label = requiredKeyword
	case required:
		need = protobuf.PresenceLegacyRequired
	case version.Name == protobuf.Proto2:
		return "", nil, fmt.Errorf(refusalPrefix+"proto2 writes a singular field with a label, "+
			"and %s is neither optional nor required", f.Name)
	case !optional && !version.Syntax && !extension && !messageType(t):
		need, implicit = protobuf.PresenceImplicit, true
	}
	if f.Value != "" {
		switch {
		case version.Name == protobuf.Proto3:
			return "", nil, fmt.Errorf(refusalPrefix+"proto3 has no default, and %s states one", f.Name)
		case repeated:
			return "", nil, fmt.Errorf(refusalPrefix+"a repeated field has no default, and %s states one", f.Name)
		case implicit:
			return "", nil, fmt.Errorf(
				refusalPrefix+"a field without presence has no default, and %s states one",
				f.Name,
			)
		}
	}
	if need == "" || features.Presence == need {
		return label, nil, nil
	}
	if stated := statedFeatures(f.Annotations).Presence; stated != "" {
		return "", nil, fmt.Errorf(
			refusalPrefix+"%s states the presence %s, and the field needs %s",
			f.Name,
			stated,
			need,
		)
	}
	return label, []feature{{name: protobuf.FeaturePresence, value: string(need)}}, nil
}

// encoding returns the features that the backend adds to the field or the
// member name of the type t and the annotations as in the version version,
// where features are the features that apply to it. A field with the
// marker protobuf:delimited takes the encoding DELIMITED in an edition. A
// field of a message type without the marker takes the encoding
// LENGTH_PREFIXED in an edition whose resolved encoding is DELIMITED.
// proto2 writes a delimited field as a group, which the block of its
// message writes.
//
// Error modes: a delimited map, a delimited field in proto2 without the
// message of its group, a delimited field in proto3, and a stated encoding
// that contradicts the marker.
func encoding(version protobuf.Edition, features protobuf.Features, name string, as symbol.Annotations,
	t *emit.TypeRef,
) ([]feature, error) {
	delimited := marked(as, protobuf.DelimitedMarker)
	elem := t
	if (t.Form == symbol.FormList || t.Form == symbol.FormOptional) && len(t.Elems) == 1 {
		elem = t.Elems[0]
	}
	var need protobuf.Encoding
	switch {
	case delimited && t.Form == symbol.FormMap:
		return nil, fmt.Errorf(refusalPrefix+"a map is not delimited, and %s states the marker %s",
			name, protobuf.DelimitedMarker)
	case delimited && version.Name == protobuf.Proto2:
		return nil, fmt.Errorf(refusalPrefix+"proto2 writes a delimited field as a group, "+
			"and %s has no message of its group", name)
	case delimited && version.Name == protobuf.Proto3:
		return nil, fmt.Errorf(refusalPrefix+"proto3 has no delimited field, and %s states the marker %s",
			name, protobuf.DelimitedMarker)
	case delimited:
		need = protobuf.EncodingDelimited
	case !version.Syntax && features.Encoding == protobuf.EncodingDelimited && t.Form != symbol.FormMap &&
		messageType(elem):
		need = protobuf.EncodingLengthPrefixed
	}
	if need == "" || features.Encoding == need {
		return nil, nil
	}
	if stated := statedFeatures(as).Encoding; stated != "" {
		return nil, fmt.Errorf(refusalPrefix+"%s states the encoding %s, and the field needs %s", name, stated, need)
	}
	return []feature{{name: protobuf.FeatureEncoding, value: string(need)}}, nil
}
