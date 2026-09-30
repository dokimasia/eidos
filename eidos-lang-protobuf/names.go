// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// NameSep separates the segments of a proto name. A package, a
// nested declaration's owner chain and a fully-qualified reference
// are all dotted, so a spelling states no boundary between the
// package and the declarations.
const NameSep = "."

// WellKnownPackage is the proto package the well-known types are
// declared in.
const WellKnownPackage = "google.protobuf"

// scalars are protobuf's fifteen scalar types. A reference to one
// names no declaration.
var scalars = map[string]bool{
	"double": true, "float": true, "int32": true, "int64": true,
	"uint32": true, "uint64": true, "sint32": true, "sint64": true,
	"fixed32": true, "fixed64": true, "sfixed32": true, "sfixed64": true,
	"bool": true, "string": true, "bytes": true,
}

// wellKnown maps the well-known types a projection maps by name, by
// their names inside [WellKnownPackage], to the import path of the
// file that declares each, the path protoc and every protobuf
// toolchain resolve it under.
var wellKnown = map[string]string{
	"Any":         "google/protobuf/any.proto",
	"Duration":    "google/protobuf/duration.proto",
	"Empty":       "google/protobuf/empty.proto",
	"FieldMask":   "google/protobuf/field_mask.proto",
	"Struct":      "google/protobuf/struct.proto",
	"Value":       "google/protobuf/struct.proto",
	"ListValue":   "google/protobuf/struct.proto",
	"NullValue":   "google/protobuf/struct.proto",
	"Timestamp":   "google/protobuf/timestamp.proto",
	"DoubleValue": "google/protobuf/wrappers.proto",
	"FloatValue":  "google/protobuf/wrappers.proto",
	"Int64Value":  "google/protobuf/wrappers.proto",
	"UInt64Value": "google/protobuf/wrappers.proto",
	"Int32Value":  "google/protobuf/wrappers.proto",
	"UInt32Value": "google/protobuf/wrappers.proto",
	"BoolValue":   "google/protobuf/wrappers.proto",
	"StringValue": "google/protobuf/wrappers.proto",
	"BytesValue":  "google/protobuf/wrappers.proto",
}

// IsScalar reports whether a spelling names one of protobuf's scalar
// types.
func IsScalar(spelling string) bool { return scalars[spelling] }

// WellKnown returns the fully-qualified name a spelling states for a
// well-known type, with the leading dot removed, and reports whether
// the spelling names one. A well-known type maps by this name whether
// or not the workspace loads its declaration, so no reference to one
// resolves against the graph.
func WellKnown(spelling string) (string, bool) {
	name, _, known := wellKnownOf(spelling)
	return name, known
}

// WellKnownImport returns the import path of the file that declares
// the well-known type a spelling names, such as
// google/protobuf/timestamp.proto, and reports whether the spelling
// names one. The frontend records it as the reference's package,
// because no reference to a well-known type resolves against the
// graph.
func WellKnownImport(spelling string) (string, bool) {
	_, file, known := wellKnownOf(spelling)
	return file, known
}

// wellKnownOf returns the fully-qualified name and the declaring
// file of the well-known type a spelling names.
func wellKnownOf(spelling string) (name, file string, known bool) {
	name = strings.TrimPrefix(strings.TrimSpace(spelling), NameSep)
	local, inPackage := strings.CutPrefix(name, WellKnownPackage+NameSep)
	if !inPackage {
		return "", "", false
	}
	file, known = wellKnown[local]
	if !known {
		return "", "", false
	}
	return name, file, true
}

// Candidates returns what a type spelling could mean from inside a
// scope, in protoc's probe order, one tier per scope.
//
// The scope is a package and the chain of messages the reference is
// written in. A relative spelling probes the innermost message
// first, then each enclosing message, the package, each namespace
// above it and the root, so an inner declaration shadows an outer
// one of the same name without an ambiguity. A leading dot states
// the whole path, and the one tier it returns probes nothing
// outward.
//
// A package and a nested declaration share the dotted grammar, so
// each tier offers its fully-qualified name at every split into a
// package and an owner chain, the longest package first. A scalar
// and a well-known type return no candidate, because neither names
// a declaration the graph is asked for.
//
// protoc stops at the innermost scope that declares the first
// segment of a compound spelling, and fails where that scope does
// not declare the rest. These tiers continue outward past such a
// scope, so a spelling protoc rejects can resolve here. Every
// spelling protoc accepts resolves to the declaration protoc binds.
func Candidates(pkg, chain, spelling string) plugin.Candidates {
	name := strings.TrimSpace(spelling)
	if name == "" || IsScalar(name) {
		return nil
	}
	if _, known := WellKnown(name); known {
		return nil
	}
	if qualified, rooted := strings.CutPrefix(name, NameSep); rooted {
		return plugin.Candidates{splits(qualified)}
	}
	within := scopes(pkg, chain)
	out := make(plugin.Candidates, 0, len(within))
	for _, scope := range within {
		out = append(out, splits(joinName(scope, name)))
	}
	return out
}

// scopes returns the fully-qualified scopes a relative spelling
// resolves in, innermost first: the message chain inside the
// package, the package, each namespace above it, and the root, which
// is the empty scope.
func scopes(pkg, chain string) []string {
	scope := joinName(pkg, chain)
	out := make([]string, 0, strings.Count(scope, NameSep)+2)
	for scope != "" {
		out = append(out, scope)
		at := strings.LastIndex(scope, NameSep)
		if at < 0 {
			break
		}
		scope = scope[:at]
	}
	return append(out, "")
}

// splits returns the identities one fully-qualified name could be,
// one per split of its qualifier into a package and an owner chain,
// the longest package first.
func splits(qualified string) []symbol.Identity {
	segments := strings.Split(qualified, NameSep)
	name := segments[len(segments)-1]
	qualifier := segments[:len(segments)-1]
	out := make([]symbol.Identity, 0, len(qualifier)+1)
	for cut := len(qualifier); cut >= 0; cut-- {
		out = append(out, symbol.Identity{
			Lang:    Lang,
			Package: strings.Join(qualifier[:cut], NameSep),
			Owner:   strings.Join(qualifier[cut:], NameSep),
			Name:    name,
		})
	}
	return out
}

// joinName joins two dotted names, either of which may be empty.
func joinName(outer, inner string) string {
	switch {
	case outer == "":
		return inner
	case inner == "":
		return outer
	default:
		return outer + NameSep + inner
	}
}
