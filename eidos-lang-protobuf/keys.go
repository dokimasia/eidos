// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import (
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The satellite's classification and fact keys. Each names a proto
// spelling the projection has no neutral form for: a field's wire
// number decides compatibility and the neutral model has no field
// for it, a reserved range is a promise about names a schema never
// reuses, and a file's options steer another generator. They are the
// residue, declared and stamped.
const (
	// FieldKey is a field's wire number, as written. Two schemas
	// agree on the wire when their numbers agree, so a consumer
	// checking compatibility reads this key.
	FieldKey meta.KeyName = "protobuf.field"

	// ReservedKey is a message's or an enum's reserved ranges and
	// names, comma-joined as written: the numbers and names a schema
	// promises never to reuse.
	ReservedKey meta.KeyName = "protobuf.reserved"

	// OptionsKey is the options a declaration states, one name=value
	// per entry: go_package on a file, deprecated on a field,
	// idempotency_level on an rpc, and any option a schema states for
	// a generator outside this workspace. A features option is
	// stamped under FeaturesKey, because editions give it a meaning
	// the other options lack.
	OptionsKey meta.KeyName = "protobuf.options"

	// SyntaxKey is a file's syntax or edition as written: proto2,
	// proto3 or an edition year. The meaning of an absent value
	// differs between them.
	SyntaxKey meta.KeyName = "protobuf.syntax"

	// PackageKey is a file's proto package as written: the namespace
	// its references resolve in, which differs from the workspace
	// path the file loaded under.
	PackageKey meta.KeyName = "protobuf.package"

	// OneofKey marks the Sum a oneof projected as, so a consumer can
	// tell a oneof from any other sum.
	OneofKey meta.KeyName = "protobuf.oneof"

	// StreamKey marks an rpc whose request or response streams,
	// naming the side: request, response or both.
	StreamKey meta.KeyName = "protobuf.stream"

	// MapEntryKey marks a field the schema wrote as a map, whose
	// projection is the Map form over its key and value.
	MapEntryKey meta.KeyName = "protobuf.mapEntry"

	// FeaturesKey is the edition features a declaration states, as
	// written. Editions replace the syntax keyword with features,
	// and field presence among them changes what an absent value
	// means, so every level that states features is stamped: a file,
	// a message, a field, a oneof, an enum, an enum value, a service
	// and an rpc.
	FeaturesKey meta.KeyName = "protobuf.features"

	// LabelKey is a field's label where the field is required:
	// proto2's required label, or an edition's legacy required
	// presence. The projection has no form for it, because a
	// projected field is required unless its form states otherwise,
	// and a consumer that keeps the distinction reads this key.
	LabelKey meta.KeyName = "protobuf.label"

	// JSONNameKey is a field's stated json_name, which decides the
	// field's spelling in JSON and nothing about its type.
	JSONNameKey meta.KeyName = "protobuf.jsonName"

	// ExtensionsKey is a message's extension ranges as written, each
	// with its options: the numbers the message reserves for
	// extensions declared elsewhere.
	ExtensionsKey meta.KeyName = "protobuf.extensions"

	// ImportKey is a file's imports as written, public and weak
	// marked, because a public import re-exports what it imports and
	// a weak one may be absent at run time.
	ImportKey meta.KeyName = "protobuf.import"
)

// Keys claims the protobuf metadata namespace and registers every
// key in it, each typed as text.
//
// A composition passes it to the workspace builder and a corpus
// fixture passes it to the conformance runner, so both register one
// set. It returns the first error a registration reports and
// registers nothing after it. Calling it twice on one registry is an
// error, because a namespace is claimed once.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace("protobuf", string(Name)); err != nil {
		return err
	}
	file := []symbol.Kind{symbol.KindFile}
	for _, spec := range []meta.KeySpec{
		{
			Name: FieldKey, Kinds: []symbol.Kind{symbol.KindField},
			Doc: "a field's wire number, which decides compatibility",
		},
		{
			Name: ReservedKey, Kinds: []symbol.Kind{symbol.KindStruct, symbol.KindEnum},
			Doc: "the numbers and names a schema promises never to reuse",
		},
		{
			Name: OptionsKey,
			Kinds: []symbol.Kind{
				symbol.KindFile, symbol.KindStruct, symbol.KindField, symbol.KindEnum,
				symbol.KindEnumVariant, symbol.KindSum, symbol.KindInterface, symbol.KindMethod,
			},
			Doc: "the options a declaration states, for generators outside this workspace",
		},
		{
			Name: SyntaxKey, Kinds: file,
			Doc: "a file's syntax or edition, which decides what an absent value means",
		},
		{
			Name: PackageKey, Kinds: file,
			Doc: "a file's proto package, the namespace its references resolve in",
		},
		{
			Name: OneofKey, Kinds: []symbol.Kind{symbol.KindSum},
			Doc: "marks the sum a oneof projected as",
		},
		{
			Name: StreamKey, Kinds: []symbol.Kind{symbol.KindMethod},
			Doc: "marks an rpc that streams, naming the side",
		},
		{
			Name: MapEntryKey, Kinds: []symbol.Kind{symbol.KindField},
			Doc: "marks a field the schema wrote as a map",
		},
		{
			Name: FeaturesKey,
			Kinds: []symbol.Kind{
				symbol.KindFile, symbol.KindStruct, symbol.KindField, symbol.KindSum,
				symbol.KindEnum, symbol.KindEnumVariant, symbol.KindInterface, symbol.KindMethod,
			},
			Doc: "the edition features a declaration states, as written",
		},
		{
			Name: LabelKey, Kinds: []symbol.Kind{symbol.KindField},
			Doc: "a required field's label, which the projection has no form for",
		},
		{
			Name: JSONNameKey, Kinds: []symbol.Kind{symbol.KindField},
			Doc: "a field's stated json_name",
		},
		{
			Name: ExtensionsKey, Kinds: []symbol.Kind{symbol.KindStruct},
			Doc: "a message's extension ranges and their options, as written",
		},
		{
			Name: ImportKey, Kinds: file,
			Doc: "a file's imports as written, public and weak marked",
		},
	} {
		if _, err := meta.Register[string](r, spec); err != nil {
			return err
		}
	}
	return nil
}
