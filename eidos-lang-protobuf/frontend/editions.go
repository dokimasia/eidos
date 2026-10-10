// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "github.com/bufbuild/protocompile/experimental/token/keyword"

// The values of the version statements in the frontend's table. A file
// without a statement is proto2.
const (
	versionProto2 = "proto2"
	versionProto3 = "proto3"
	edition2023   = "2023"
	edition2024   = "2024"
	edition2026   = "2026"
)

// The feature values that the resolution compares against, as
// descriptor.proto names them.
const (
	// presenceExplicit gives a singular field presence, which the optional
	// form projects.
	presenceExplicit = "EXPLICIT"
	// presenceImplicit gives a singular field no presence, the way a plain
	// proto3 field has none.
	presenceImplicit = "IMPLICIT"
	// presenceLegacyRequired makes a field required, the way proto2's
	// required label does.
	presenceLegacyRequired = "LEGACY_REQUIRED"
	// enumOpen keeps an undeclared number of an enum field as a value, and
	// enumClosed keeps it as an unknown field.
	enumOpen   = "OPEN"
	enumClosed = "CLOSED"
	// encodingLengthPrefixed and encodingDelimited are the two encodings of
	// a message field. A proto2 group is delimited.
	encodingLengthPrefixed = "LENGTH_PREFIXED"
	encodingDelimited      = "DELIMITED"
	// The default visibilities of a message and an enum that state no
	// export or local keyword.
	visibilityExportAll      = "EXPORT_ALL"
	visibilityExportTopLevel = "EXPORT_TOP_LEVEL"
	visibilityLocalAll       = "LOCAL_ALL"
	visibilityStrict         = "STRICT"
)

// The names of the features that the resolution reads from the options of
// a declaration.
const (
	featurePresence   = featuresPrefix + "field_presence"
	featureEnumType   = featuresPrefix + "enum_type"
	featureEncoding   = featuresPrefix + "message_encoding"
	featureVisibility = featuresPrefix + "default_symbol_visibility"
)

// features is the resolved value of the four features that the frontend
// applies: the presence of a singular field, the openness of an enum, the
// encoding of a message field, and the default visibility of a message
// and an enum. Each value is spelled as descriptor.proto names it.
//
// The zero value is not a resolution. Every resolution starts from the
// defaults of a version.
type features struct {
	presence   string
	enumType   string
	encoding   string
	visibility string
}

// version is one protobuf language version of the frontend's table: the
// keyword and the value of the statement that states it, and the defaults
// of its features, which descriptor.proto of protobuf v36.0 declares.
type version struct {
	keyword  keyword.Keyword
	name     string
	defaults features
}

// versions is the frontend's table of protobuf language versions, in the
// order that protobuf released them. protobuf did not release an Edition
// 2025.
var versions = []version{
	{
		keyword: keyword.Syntax, name: versionProto2,
		defaults: features{
			presence: presenceExplicit, enumType: enumClosed,
			encoding: encodingLengthPrefixed, visibility: visibilityExportAll,
		},
	},
	{
		keyword: keyword.Syntax, name: versionProto3,
		defaults: features{
			presence: presenceImplicit, enumType: enumOpen,
			encoding: encodingLengthPrefixed, visibility: visibilityExportAll,
		},
	},
	{
		keyword: keyword.Edition, name: edition2023,
		defaults: features{
			presence: presenceExplicit, enumType: enumOpen,
			encoding: encodingLengthPrefixed, visibility: visibilityExportAll,
		},
	},
	{
		keyword: keyword.Edition, name: edition2024,
		defaults: features{
			presence: presenceExplicit, enumType: enumOpen,
			encoding: encodingLengthPrefixed, visibility: visibilityExportTopLevel,
		},
	},
	{
		keyword: keyword.Edition, name: edition2026,
		defaults: features{
			presence: presenceExplicit, enumType: enumOpen,
			encoding: encodingLengthPrefixed, visibility: visibilityStrict,
		},
	},
}

// versionNamed returns the version that a statement of a keyword and a
// value states, and reports false for a statement outside the table, such
// as edition 2025 or the syntax keyword with an edition's value.
func versionNamed(stated keyword.Keyword, value string) (version, bool) {
	for _, v := range versions {
		if v.keyword == stated && v.name == value {
			return v, true
		}
	}
	return version{}, false
}

// with returns f after the features options among opts. Each such option
// replaces the value of its feature.
func (f features) with(opts []option) features {
	for _, o := range opts {
		switch o.name {
		case featurePresence:
			f.presence = o.value
		case featureEnumType:
			f.enumType = o.value
		case featureEncoding:
			f.encoding = o.value
		case featureVisibility:
			f.visibility = o.value
		}
	}
	return f
}

// local reports whether a message or an enum is local to its file: the
// keyword local, or the default visibility of its features where it states
// neither keyword. A nested declaration is local by default under
// EXPORT_TOP_LEVEL, and every declaration under LOCAL_ALL and STRICT.
func (f features) local(stated keyword.Keyword, nested bool) bool {
	switch stated {
	case keyword.Local:
		return true
	case keyword.Export:
		return false
	default:
		// A declaration that states neither keyword takes the default
		// visibility of its features below.
	}
	switch f.visibility {
	case visibilityExportTopLevel:
		return nested
	case visibilityLocalAll, visibilityStrict:
		return true
	default:
		return false
	}
}
