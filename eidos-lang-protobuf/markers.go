// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

// NumberTag is the key of the tag entry that states the wire number of a
// field in decimal, such as 3. The protobuf backend writes the number after
// the name of the field and refuses a field of a message without the entry.
const NumberTag = "number"

// MarkerPrefix opens the name of every [Marker]. An option name is an
// identifier, a dotted path or an extension name in parentheses, so no
// option has the name of a marker.
const MarkerPrefix = "protobuf:"

// Marker is the name of an annotation that states a keyword or a statement
// of protobuf that is not an option. A generator states a marker as an
// annotation with the name of the marker, and the backend writes the
// keyword or the statement for it and no option.
type Marker string

// The markers of the protobuf target.
const (
	// SyntaxMarker is a header marker. Its one argument is proto2, proto3,
	// 2023, 2024 or 2026, and the backend writes the syntax or the edition
	// statement of the file from it.
	SyntaxMarker Marker = MarkerPrefix + "syntax"
	// PackageMarker is a header marker. Its one argument is the package
	// that the file declares, such as acme.svc.v1.
	PackageMarker Marker = MarkerPrefix + "package"
	// ImportMarker is a header marker. Its one argument is the path of one
	// import, after public, weak or option where the import has that
	// modifier, as in public acme/svc/v1/types.proto.
	ImportMarker Marker = MarkerPrefix + "import"
	// RequiredMarker marks a required field: the label required in proto2,
	// and the presence LEGACY_REQUIRED in an edition.
	RequiredMarker Marker = MarkerPrefix + "required"
	// DelimitedMarker marks a field whose message is encoded delimited: a
	// group in proto2, and the encoding DELIMITED in an edition.
	DelimitedMarker Marker = MarkerPrefix + "delimited"
	// NumberMarker states the wire number of the oneof member of a sum
	// variant that does not have exactly one field.
	NumberMarker Marker = MarkerPrefix + "number"
	// ReservedMarker states the reserved numbers and names of a message or
	// an enum. Each argument is a number, a range such as 9 to 11 or
	// 40 to max, or a name.
	ReservedMarker Marker = MarkerPrefix + "reserved"
	// ExtensionsMarker states one extension range declaration of a
	// message. Each argument is a range, and a last argument in brackets
	// is the options of the declaration as written.
	ExtensionsMarker Marker = MarkerPrefix + "extensions"
	// ClosedMarker marks a closed enum: every enum of proto2, and the enum
	// type CLOSED in an edition.
	ClosedMarker Marker = MarkerPrefix + "closed"
	// LocalMarker marks a message or an enum that another file cannot
	// reference, which the editions 2024 and 2026 write with local.
	LocalMarker Marker = MarkerPrefix + "local"
	// FollowsMarker places a nested message, enum, oneof or extension
	// among the members of its message. Its one argument is the name of the
	// member that the declaration follows, and the empty name places it
	// before every member. The backend writes a nested declaration without
	// it after the fields.
	FollowsMarker Marker = MarkerPrefix + "follows"
	// BodyMarker marks an rpc that the backend writes with a body in
	// braces, as rpc Get(Row) returns (Row) {}, also where it states no
	// option.
	BodyMarker Marker = MarkerPrefix + "body"
)

// Valid reports whether m is one of the markers that the satellite
// declares. The backend refuses an annotation whose name starts with
// [MarkerPrefix] and is no valid marker. Valid allocates nothing.
func (m Marker) Valid() bool {
	switch m {
	case SyntaxMarker, PackageMarker, ImportMarker, RequiredMarker, DelimitedMarker,
		NumberMarker, ReservedMarker, ExtensionsMarker, ClosedMarker, LocalMarker, FollowsMarker, BodyMarker:
		return true
	default:
		return false
	}
}
