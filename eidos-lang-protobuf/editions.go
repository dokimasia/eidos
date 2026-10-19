// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import "strings"

// The names of the protobuf versions that the satellite reads and
// writes: the value of the syntax statement of proto2 and proto3, and the
// value of the edition statement of an edition. A file without a
// statement is proto2.
const (
	Proto2      = "proto2"
	Proto3      = "proto3"
	Edition2023 = "2023"
	Edition2024 = "2024"
	Edition2026 = "2026"
)

// The options through which a declaration sets its features. FeaturesOption
// sets features through a message literal, as in
// features = { field_presence: IMPLICIT }, and an option of one feature sets
// that feature, as features.field_presence = IMPLICIT does.
const (
	FeaturesOption    = "features"
	FeaturePresence   = FeaturesOption + ".field_presence"
	FeatureEnumType   = FeaturesOption + ".enum_type"
	FeatureEncoding   = FeaturesOption + ".message_encoding"
	FeatureVisibility = FeaturesOption + ".default_symbol_visibility"
)

// Presence is the value of the feature field_presence, as descriptor.proto
// names it.
type Presence string

// The values of field_presence. A singular field with explicit presence
// tells an absent value from its default, and a field with implicit
// presence does not. Legacy required presence makes a field required, the
// way the required label of proto2 does.
const (
	PresenceExplicit       Presence = "EXPLICIT"
	PresenceImplicit       Presence = "IMPLICIT"
	PresenceLegacyRequired Presence = "LEGACY_REQUIRED"
)

// EnumType is the value of the feature enum_type, as descriptor.proto
// names it.
type EnumType string

// The values of enum_type. A field of an open enum keeps a number that the
// enum does not declare as a value, and a field of a closed enum keeps it
// as an unknown field.
const (
	EnumOpen   EnumType = "OPEN"
	EnumClosed EnumType = "CLOSED"
)

// Encoding is the value of the feature message_encoding, as
// descriptor.proto names it.
type Encoding string

// The values of message_encoding. A delimited message field is encoded the
// way a group of proto2 is.
const (
	EncodingLengthPrefixed Encoding = "LENGTH_PREFIXED"
	EncodingDelimited      Encoding = "DELIMITED"
)

// DefaultVisibility is the value of the feature default_symbol_visibility,
// as descriptor.proto names it. It decides whether a message or an enum
// that states neither local nor export is local to its file.
type DefaultVisibility string

// The values of default_symbol_visibility. Under EXPORT_ALL no declaration
// is local by default. Under EXPORT_TOP_LEVEL a nested declaration is
// local by default. Under LOCAL_ALL and STRICT every declaration is local by
// default.
const (
	VisibilityExportAll      DefaultVisibility = "EXPORT_ALL"
	VisibilityExportTopLevel DefaultVisibility = "EXPORT_TOP_LEVEL"
	VisibilityLocalAll       DefaultVisibility = "LOCAL_ALL"
	VisibilityStrict         DefaultVisibility = "STRICT"
)

// Features is the resolved value of the four features that the satellite
// applies: the presence of a singular field, the openness of an enum, the
// encoding of a message field, and the default visibility of a message
// and an enum. The other features apply to the wire format, the JSON
// mapping and the checks of protoc.
//
// The zero value is not a resolution. Every resolution starts from the
// defaults of an [Edition].
type Features struct {
	Presence   Presence
	EnumType   EnumType
	Encoding   Encoding
	Visibility DefaultVisibility
}

// Edition is one protobuf language version, as descriptor.proto names the
// legacy syntaxes and the editions alike: the value of the statement that
// states it, whether that statement is a syntax statement, and the
// defaults of its features.
type Edition struct {
	Name     string
	Syntax   bool
	Defaults Features
}

// editions is the table of the protobuf versions, in the order that
// protobuf released them, with the defaults that descriptor.proto of
// protobuf v36.0 declares. protobuf did not release an edition 2025.
var editions = [...]Edition{
	{
		Name: Proto2, Syntax: true,
		Defaults: Features{
			Presence: PresenceExplicit, EnumType: EnumClosed,
			Encoding: EncodingLengthPrefixed, Visibility: VisibilityExportAll,
		},
	},
	{
		Name: Proto3, Syntax: true,
		Defaults: Features{
			Presence: PresenceImplicit, EnumType: EnumOpen,
			Encoding: EncodingLengthPrefixed, Visibility: VisibilityExportAll,
		},
	},
	{
		Name: Edition2023,
		Defaults: Features{
			Presence: PresenceExplicit, EnumType: EnumOpen,
			Encoding: EncodingLengthPrefixed, Visibility: VisibilityExportAll,
		},
	},
	{
		Name: Edition2024,
		Defaults: Features{
			Presence: PresenceExplicit, EnumType: EnumOpen,
			Encoding: EncodingLengthPrefixed, Visibility: VisibilityExportTopLevel,
		},
	},
	{
		Name: Edition2026,
		Defaults: Features{
			Presence: PresenceExplicit, EnumType: EnumOpen,
			Encoding: EncodingLengthPrefixed, Visibility: VisibilityStrict,
		},
	},
}

// EditionNamed returns the version whose statement has the value name, and
// reports false for a name outside the table, such as the edition 2025,
// which protobuf did not release, or UNSTABLE, which protoc reserves for
// features in development. A caller that reads a statement also compares
// its keyword with [Edition.Syntax]. EditionNamed allocates nothing.
func EditionNamed(name string) (Edition, bool) {
	for _, e := range editions {
		if e.Name == name {
			return e, true
		}
	}
	return Edition{}, false
}

// Apply returns f after one option of a declaration, whose name and value
// are as written. The option of one feature replaces the value of that
// feature. [FeaturesOption] with a message literal replaces the value of
// each of the four features that the top level of the literal sets. A field
// of the literal without a key, with a value that is a message or a string,
// or with a key of another feature sets nothing, and so does every other
// option. Apply allocates nothing.
func (f Features) Apply(name, value string) Features {
	if name != FeaturesOption {
		return f.set(name, value)
	}
	text := strings.TrimSpace(value)
	if !strings.HasPrefix(text, "{") {
		return f
	}
	// The scan reads the literal one field at a time. depth counts the open
	// braces, so a field of a nested message sets nothing.
	depth := 0
	for i := 0; i < len(text); {
		c := text[i]
		switch {
		case c == '{':
			depth++
			i++
		case c == '}':
			depth--
			i++
		case c == '"' || c == '\'':
			i = skipString(text, i)
		case c == '[':
			// An extension name in brackets is the key of a field that no
			// feature of the four has.
			i = skipPast(text, i, ']')
		case isWordByte(c) && depth == 1:
			key, end := word(text, i)
			value, next, ok := fieldValue(text, end)
			if ok {
				f = f.set(FeaturesOption+"."+key, value)
			}
			i = next
		default:
			i++
		}
	}
	return f
}

// LocalByDefault reports whether the default visibility of f makes a
// message or an enum that states neither local nor export local to its
// file. nested reports a declaration that a message declares. It allocates
// nothing.
func (f Features) LocalByDefault(nested bool) bool {
	switch f.Visibility {
	case VisibilityExportTopLevel:
		return nested
	case VisibilityLocalAll, VisibilityStrict:
		return true
	default:
		return false
	}
}

// set returns f with value as the value of the feature whose option is
// name. The name of another option leaves f as it is.
func (f Features) set(name, value string) Features {
	switch name {
	case FeaturePresence:
		f.Presence = Presence(value)
	case FeatureEnumType:
		f.EnumType = EnumType(value)
	case FeatureEncoding:
		f.Encoding = Encoding(value)
	case FeatureVisibility:
		f.Visibility = DefaultVisibility(value)
	}
	return f
}

// fieldValue reads the value of a field of a message literal whose key
// ends at i: an optional colon, and then a word. It returns the word and
// the index after it. It reports false for a value that is no word, such
// as a message or a string, and returns the index of the value, which the
// scan of [Features.Apply] reads next.
func fieldValue(text string, i int) (value string, next int, ok bool) {
	i = skipSpace(text, i)
	if i < len(text) && text[i] == ':' {
		i = skipSpace(text, i+1)
	}
	if i >= len(text) || !isWordByte(text[i]) {
		return "", i, false
	}
	value, next = word(text, i)
	return value, next, true
}

// word returns the run of word bytes that starts at i, and the index after
// it.
func word(text string, i int) (string, int) {
	end := i
	for end < len(text) && isWordByte(text[end]) {
		end++
	}
	return text[i:end], end
}

// skipSpace returns the index of the first byte at or after i that is not
// white space.
func skipSpace(text string, i int) int {
	for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
		i++
	}
	return i
}

// skipString returns the index after the string literal that opens at i,
// and the end of the text for a literal that does not close. A backslash
// escapes the byte after it.
func skipString(text string, i int) int {
	quote := text[i]
	for i++; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(text)
}

// skipPast returns the index after the first byte closing at or after i,
// and the end of the text where none follows.
func skipPast(text string, i int, closing byte) int {
	if at := strings.IndexByte(text[i:], closing); at >= 0 {
		return i + at + 1
	}
	return len(text)
}

// isWordByte reports whether a byte continues an identifier or a number of
// the text format: a letter, a digit, an underscore, a dot or a minus sign.
func isWordByte(c byte) bool {
	return c == '_' || c == '.' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
