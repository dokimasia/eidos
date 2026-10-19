// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The syntax of an extend block.
const (
	extendKeyword = "extend "
	blockClose    = "}"
)

// errNoExtension is the refusal of a cluster without an extension.
var errNoExtension = errors.New(
	refusalPrefix + "an extend block extends a message, and the cluster states no extension",
)

// descriptorOptions are the messages of descriptor.proto whose fields are
// the options of a schema. proto3 extends them alone.
var descriptorOptions = map[string]bool{
	"google.protobuf.FileOptions":           true,
	"google.protobuf.MessageOptions":        true,
	"google.protobuf.FieldOptions":          true,
	"google.protobuf.OneofOptions":          true,
	"google.protobuf.ExtensionRangeOptions": true,
	"google.protobuf.EnumOptions":           true,
	"google.protobuf.EnumValueOptions":      true,
	"google.protobuf.ServiceOptions":        true,
	"google.protobuf.MethodOptions":         true,
}

// Extension returns the extend block at file level of the cluster c of the
// file whose header is h: the extensions of one message, which the
// [Cluster] gathers. Each field writes as a field of a block. proto2 writes
// a delimited field as a group whose message is a message of the cluster,
// and the cluster lists that message after its field. In any other version
// such a message is a message of the file, and the block lists it among the
// declarations that follow it.
//
// Error modes, each a refusal of the cluster: a cluster without an
// extension, an extended message outside descriptor.proto in proto3, and
// the refusals of each field.
func (v *Vocabulary) Extension(h *emit.File, c render.Clustered) (*Block, error) {
	clear(v.parents)
	clear(v.groups)
	s, err := FileScope(h)
	if err != nil {
		return nil, err
	}
	var receives *emit.TypeRef
	for _, d := range c.Decls {
		if f, is := d.(*emit.Field); is && f.Receives != nil {
			receives = f.Receives
			break
		}
	}
	if receives == nil {
		return nil, errNoExtension
	}
	extendee, err := v.extendee(s.Version(), receives)
	if err != nil {
		return nil, err
	}
	b := &Block{Head: extendKeyword + extendee}
	if s.Version().Name == protobuf.Proto2 {
		for _, d := range c.Decls {
			if f, is := d.(*emit.Field); is {
				v.markGroup(parent{scope: s, group: f}, f.Name, c.Decls)
			}
		}
	}
	for _, d := range c.Decls {
		f, is := d.(*emit.Field)
		switch {
		case is:
			if g, grouped := v.groups[f]; grouped {
				b.Items = append(b.Items, Item{Nested: g, Indent: memberIndent})
				continue
			}
			text, err := v.field(s, f, memberIndent)
			if err != nil {
				return nil, err
			}
			b.Items = append(b.Items, Item{Text: text})
		case v.parents[d].group == nil:
			v.parents[d] = parent{scope: s}
			b.Next = append(b.Next, d)
		}
	}
	b.Open = opening("", len(b.Items) > 0)
	return b, nil
}

// extendBlock returns the items of the extend block inside a message of the
// extension f at the index at of the nested declarations types of the
// message, where f is the first extension of its message among types. The
// block lists every extension of the message of f in order, behind the
// indentation of a nested block. The scope of the members of the message is
// inner. extendBlock returns no item for a later extension, which the
// block of the first one lists.
func (v *Vocabulary) extendBlock(inner Scope, types []symbol.Symbol, at int, f *emit.Field) ([]Item, error) {
	for _, t := range types[:at] {
		if prior, is := t.(*emit.Field); is && prior.Receives != nil && prior.Receives.Spelling == f.Receives.Spelling {
			return nil, nil
		}
	}
	extendee, err := v.extendee(inner.Version(), f.Receives)
	if err != nil {
		return nil, err
	}
	items := []Item{{Text: memberIndent + extendKeyword + extendee + space + blockOpenLine}}
	for _, t := range types[at:] {
		ext, is := t.(*emit.Field)
		if !is || ext.Receives == nil || ext.Receives.Spelling != f.Receives.Spelling {
			continue
		}
		if g, grouped := v.groups[ext]; grouped {
			items = append(items, Item{Nested: g, Indent: nestedIndent})
			continue
		}
		text, err := v.field(inner, ext, nestedIndent)
		if err != nil {
			return nil, err
		}
		items = append(items, Item{Text: text})
	}
	return append(items, Item{Text: memberIndent + blockClose + "\n"}), nil
}

// extendee returns the spelling of the message t that an extension extends,
// and records its import. version is the version of the file.
//
// Error modes: a message outside the options of descriptor.proto in proto3,
// and the refusals of the writer.
func (v *Vocabulary) extendee(version protobuf.Edition, t *emit.TypeRef) (string, error) {
	spelled, err := v.writer.Spell(t)
	if err != nil {
		return "", err
	}
	if version.Name == protobuf.Proto3 && !descriptorOptions[strings.TrimPrefix(t.Spelling, protobuf.NameSep)] {
		return "", fmt.Errorf(refusalPrefix+"proto3 extends the options of descriptor.proto alone, "+
			"and an extension extends %s", t.Spelling)
	}
	return spelled, nil
}
