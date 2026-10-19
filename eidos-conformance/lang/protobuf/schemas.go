// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import (
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	protolang "go.dokimi.dev/eidos/lang/protobuf"
	protobackend "go.dokimi.dev/eidos/lang/protobuf/backend"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The plan of the schemas fixture, its generator, the family word of the
// generator, the directory that the plan writes under, and the name of
// every file of the fixture. The directory is the root of the imports of
// the written files, so a written file has the import path of its source.
// Every file of a tree has the one name, which the plan gives every file
// that it writes.
const (
	MirrorDir             = "mirror"
	mirrorPlan            = "proto-mirror"
	mirrorgenID plugin.ID = "mirrorgen"
	mirrorWord            = "mirror"
	schemaName            = "schema.proto"
	importRoot            = "."
)

// ComposeSchemas returns the composition of the schemas fixture over root
// without its plans. The composition has the protobuf frontend and rules,
// the protobuf target, a disk sink at root and the ledger of the state
// directory under root. [SchemaPlans] returns the plan that it runs.
func ComposeSchemas(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(protofrontend.New()).
		Rules(protorules.New()).
		Targets(protolang.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// SchemaPlans returns the plan of the schemas fixture. The plan
// proto-mirror runs mirrorgen toward the protobuf backend, and it writes
// each schema of the tree under the mirror directory, at the path of its
// source. Each call returns a new generator and a new backend.
func SchemaPlans() []workspace.Plan {
	return []workspace.Plan{MirrorPlan(mirrorPlan, schemaName)}
}

// MirrorPlan returns a plan named name that runs mirrorgen toward the
// protobuf backend. It writes the mirror of each schema under the mirror
// directory, at the directory of its source and under the name file, and
// the mirror directory is the root of the imports of the written files. A
// tree whose files of one directory share the name file therefore writes
// each mirror at the path of its source.
func MirrorPlan(name, file string) workspace.Plan {
	return workspace.Plan{
		Name:       name,
		Generators: []plugin.Generator{mirrorgen()},
		Backend:    protobackend.New(),
		Layout: layout.Config{
			Policy:     layout.PolicyCentralised,
			Dir:        MirrorDir,
			ImportBase: importRoot,
			Families:   map[layout.Family]layout.Refinement{{Plugin: mirrorgenID}: {File: file}},
		},
	}
}

// mirrorgen returns the mirror generator of the protobuf fixtures. It
// writes each declaration at file level of a protobuf source in the
// protobuf target as the source states it: a message with its fields and
// its nested declarations, an enum, a service and an extension. Each
// declaration has the statement of its residue, and each file has the
// header of its source.
func mirrorgen() plugin.Generator {
	return eidos.NewPlugin(mirrorgenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: mirrorWord}).
		Handle(
			eidos.OnStruct(mirrorStruct), eidos.OnEnum(mirrorEnum), eidos.OnInterface(mirrorService),
			eidos.OnField(mirrorExtension),
		).
		Generator()
}

// mirrorStruct writes a message at file level with its members. A nested
// message is a member of the message that declares it, so mirrorStruct
// writes none of its own. It withholds the message where the target has
// no spelling of a type, after the Emitter reports the refusal.
func mirrorStruct(m *eidos.StructMatch, e *eidos.Emitter) error {
	s := m.Struct
	if s.ID.Owner != "" {
		return nil
	}
	out, mirrored := message(m, e, s)
	if mirrored {
		e.File().Append(protolang.Header(m, s), out)
	}
	return nil
}

// mirrorEnum writes an enum at file level with its values. A nested enum
// is a member of the message that declares it.
func mirrorEnum(m *eidos.EnumMatch, e *eidos.Emitter) error {
	en := m.Enum
	if en.ID.Owner != "" {
		return nil
	}
	e.File().Append(protolang.Header(m, en), enum(m, en))
	return nil
}

// mirrorService writes a service with its rpcs. Each rpc takes its request
// and returns its response, under the stream form where a side streams. It
// withholds the service where the target has no spelling of a type, after
// the Emitter reports the refusal.
func mirrorService(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	i := m.Interface
	stated := protolang.Stated(m, i.ID)
	out := &emit.Interface{
		Origin: i.ID, Doc: i.Doc, Comment: i.Comment, Name: i.Name, Visibility: i.Visibility,
		Annotations: stated.Annotations,
	}
	for _, method := range i.Methods {
		rpc := &emit.Method{
			Origin: method.ID, Doc: method.Doc, Comment: method.Comment, Name: method.Name,
			Visibility: method.Visibility, Abstract: true, Async: method.Async,
			Annotations: protolang.Stated(m, method.ID).Annotations,
		}
		for _, p := range method.Params {
			t, spelled := e.Type(method.ID, p.Type)
			if !spelled {
				return nil
			}
			rpc.Params.Append(&emit.Param{Name: p.Name, Type: t})
		}
		for _, r := range method.Returns {
			t, spelled := e.Type(method.ID, r.Type)
			if !spelled {
				return nil
			}
			rpc.Returns.Append(&emit.Return{Type: t})
		}
		out.Methods.Append(rpc)
	}
	e.File().Append(protolang.Header(m, i), out)
	return nil
}

// mirrorExtension writes an extension at file level. An extension inside a
// message is a member of that message, and every other field is a field of
// its message.
func mirrorExtension(m *eidos.FieldMatch, e *eidos.Emitter) error {
	f := m.Field
	if f.ID.Owner != "" || f.Receives == nil {
		return nil
	}
	out, mirrored := field(m, e, f)
	if mirrored {
		e.File().Append(protolang.Header(m, f), out)
	}
	return nil
}

// message returns the mirror of the message s: its fields in order, and
// then its oneofs, its nested messages and enums and its extensions in
// order, as the protobuf backend writes them. It reports false where the
// target has no spelling of a type.
func message(m eidos.Matcher, e *eidos.Emitter, s *node.Struct) (*emit.Struct, bool) {
	out := &emit.Struct{
		Origin: s.ID, Doc: s.Doc, Comment: s.Comment, Name: s.Name, Visibility: s.Visibility,
		Annotations: protolang.Stated(m, s.ID).Annotations,
	}
	for _, f := range s.Fields {
		mirrored, spelled := field(m, e, f)
		if !spelled {
			return nil, false
		}
		out.Fields.Append(mirrored)
	}
	for _, t := range s.Types {
		var (
			nested  symbol.Symbol
			spelled = true
		)
		switch n := t.(type) {
		case *node.Struct:
			nested, spelled = message(m, e, n)
		case *node.Enum:
			nested = enum(m, n)
		case *node.Sum:
			nested, spelled = oneof(m, e, n)
		case *node.Field:
			nested, spelled = field(m, e, n)
		}
		if !spelled {
			return nil, false
		}
		out.Types.Append(nested)
	}
	return out, true
}

// field returns the mirror of the field f: its type and the message that
// it extends as written, its default, its comments, the tag entry of its
// number and the annotations of its residue. It reports false where the
// target has no spelling of a type.
func field(m eidos.Matcher, e *eidos.Emitter, f *node.Field) (*emit.Field, bool) {
	t, spelled := e.Type(f.ID, f.Type)
	if !spelled {
		return nil, false
	}
	receives, spelled := e.Type(f.ID, f.Receives)
	if !spelled {
		return nil, false
	}
	stated := protolang.Stated(m, f.ID)
	out := &emit.Field{
		Origin: f.ID, Doc: f.Doc, Comment: f.Comment, Name: f.Name, Visibility: f.Visibility, Type: t,
		Receives: receives, Value: f.Value, Annotations: stated.Annotations,
	}
	if stated.Number != nil {
		out.Tags = emit.SlotOf(stated.Number)
	}
	return out, true
}

// enum returns the mirror of the enum en: its values with their numbers as
// written, and the annotations of each residue.
func enum(m eidos.Matcher, en *node.Enum) *emit.Enum {
	out := &emit.Enum{
		Origin: en.ID, Doc: en.Doc, Comment: en.Comment, Name: en.Name, Visibility: en.Visibility,
		Annotations: protolang.Stated(m, en.ID).Annotations,
	}
	for _, v := range en.Variants {
		out.Variants.Append(&emit.EnumVariant{
			Origin: v.ID, Doc: v.Doc, Comment: v.Comment, Name: v.Name, Value: v.Value,
			Annotations: protolang.Stated(m, v.ID).Annotations,
		})
	}
	return out
}

// oneof returns the mirror of the oneof s: one variant for each member,
// under the name of the member, with the mirror of its one field. It
// reports false where the target has no spelling of a type.
func oneof(m eidos.Matcher, e *eidos.Emitter, s *node.Sum) (*emit.Sum, bool) {
	out := &emit.Sum{
		Origin: s.ID, Doc: s.Doc, Comment: s.Comment, Name: s.Name, Visibility: s.Visibility,
		Annotations: protolang.Stated(m, s.ID).Annotations,
	}
	for _, v := range s.Variants {
		variant := &emit.SumVariant{Origin: v.ID, Name: v.Name}
		for _, f := range v.Fields {
			mirrored, spelled := field(m, e, f)
			if !spelled {
				return nil, false
			}
			variant.Fields.Append(mirrored)
		}
		out.Variants.Append(variant)
	}
	return out, true
}
