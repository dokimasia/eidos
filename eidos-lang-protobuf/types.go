// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

// The scalar types of protobuf that are not integers. The integer types
// are the choices of the encoding policies, such as [Int32] and [Sint64].
const (
	TypeDouble = "double"
	TypeFloat  = "float"
	TypeBool   = "bool"
	TypeString = "string"
	TypeBytes  = "bytes"
)

// The well-known types that a target writes for a shape of another
// language, by their fully-qualified names. [WellKnownImport] returns the
// file that declares each of them.
const (
	TypeValue     = WellKnownPackage + NameSep + "Value"
	TypeTimestamp = WellKnownPackage + NameSep + "Timestamp"
	TypeDuration  = WellKnownPackage + NameSep + "Duration"
	TypeEmpty     = WellKnownPackage + NameSep + "Empty"
)

// The files that declare the well-known types, by the import paths that
// protoc and every protobuf toolchain resolve them under.
const (
	AnyFile       = "google/protobuf/any.proto"
	DurationFile  = "google/protobuf/duration.proto"
	EmptyFile     = "google/protobuf/empty.proto"
	FieldMaskFile = "google/protobuf/field_mask.proto"
	StructFile    = "google/protobuf/struct.proto"
	TimestampFile = "google/protobuf/timestamp.proto"
	WrappersFile  = "google/protobuf/wrappers.proto"
)

// wrappers maps each scalar type onto the fully-qualified name of the
// well-known message that wraps a value of the type. The well-known types
// declare a wrapper for each varint type alone, so a zigzag type and a
// fixed type map onto the wrapper of the varint type of their width and
// sign.
var wrappers = map[string]string{
	TypeDouble:       WellKnownPackage + NameSep + "DoubleValue",
	TypeFloat:        WellKnownPackage + NameSep + "FloatValue",
	string(Int64):    WellKnownPackage + NameSep + "Int64Value",
	string(Sint64):   WellKnownPackage + NameSep + "Int64Value",
	string(Sfixed64): WellKnownPackage + NameSep + "Int64Value",
	string(Uint64):   WellKnownPackage + NameSep + "UInt64Value",
	string(Fixed64):  WellKnownPackage + NameSep + "UInt64Value",
	string(Int32):    WellKnownPackage + NameSep + "Int32Value",
	string(Sint32):   WellKnownPackage + NameSep + "Int32Value",
	string(Sfixed32): WellKnownPackage + NameSep + "Int32Value",
	string(Uint32):   WellKnownPackage + NameSep + "UInt32Value",
	string(Fixed32):  WellKnownPackage + NameSep + "UInt32Value",
	TypeBool:         WellKnownPackage + NameSep + "BoolValue",
	TypeString:       WellKnownPackage + NameSep + "StringValue",
	TypeBytes:        WellKnownPackage + NameSep + "BytesValue",
}

// Wrapper returns the fully-qualified name of the well-known message that
// wraps a value of the scalar type scalar, such as
// google.protobuf.Int32Value for int32. A wrapper gives a scalar presence
// in a repeated field and in the value of a map, which protobuf does not
// give an optional value. Wrapper reports false for a spelling that names
// no scalar type. It allocates nothing.
func Wrapper(scalar string) (string, bool) {
	name, wraps := wrappers[scalar]
	return name, wraps
}
