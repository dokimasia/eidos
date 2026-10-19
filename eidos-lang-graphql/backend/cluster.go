// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"slices"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ExtendGroup is the name of the group of the extension members of one
// type. The cluster assigns the fields and the methods at file level that
// receive one type to it.
const ExtendGroup render.GroupName = "extend"

// ExtendTemplate renders one extend block through [Vocabulary.Extension].
const ExtendTemplate = "{{" + FuncExtension + " .}}"

// Groups returns the group templates that the cluster selects. It builds a
// new map on every call for the caller to keep.
func Groups() map[render.GroupName]string {
	return map[render.GroupName]string{ExtendGroup: ExtendTemplate}
}

// Cluster gathers the fields and the methods at file level of a unit into
// one extend block for each type that they receive, by the spelling of
// their Receives, in the order of the first member of each. Every other
// declaration stays a singleton.
//
// # Allocation contract
//
// Cluster allocates the list of the blocks once, for the number of the
// extension members. A block of one member lists the entry of decls without
// an allocation, and a block of two or more members allocates a list of its
// own. A unit without an extension member allocates nothing.
func Cluster(decls []symbol.Symbol) []render.Clustered {
	n := 0
	for _, d := range decls {
		if emit.ReceivesOf(d) != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	out := make([]render.Clustered, 0, n)
	for k, d := range decls {
		receives := emit.ReceivesOf(d)
		if receives == nil {
			continue
		}
		i := slices.IndexFunc(out, func(c render.Clustered) bool {
			return emit.ReceivesOf(c.Decls[0]).Spelling == receives.Spelling
		})
		if i < 0 {
			out = append(out, render.Clustered{Group: ExtendGroup, Decls: decls[k : k+1 : k+1]})
			continue
		}
		out[i].Decls = append(out[i].Decls, d)
	}
	return out
}

// Extension writes one extend block of [Cluster]: extend interface where the
// target of the Receives of its first member is an interface, and extend
// type otherwise, with the members of the block in their order. An
// extension of an input object is a struct with the marker graphql:extend,
// because the identity of a struct does not state whether it is an input
// object. A block without members, or whose first member receives no type,
// writes nothing.
func (v *Vocabulary) Extension(c render.Clustered) string {
	if len(c.Decls) == 0 {
		return ""
	}
	receives := emit.ReceivesOf(c.Decls[0])
	if receives == nil {
		return ""
	}
	b := &v.b
	b.Reset()
	b.WriteString(extendKeyword)
	if receives.Target.Kind == symbol.KindInterface {
		b.WriteString(interfaceKeyword)
	} else {
		b.WriteString(typeKeyword)
	}
	b.WriteString(receives.Spelling)
	b.WriteString(bodyOpen)
	b.WriteString(newline)
	for _, d := range c.Decls {
		switch x := d.(type) {
		case *emit.Field:
			writeField(b, x)
		case *emit.Method:
			writeMethod(b, x)
		}
	}
	b.WriteString(bodyClose)
	b.WriteString(newline)
	return b.String()
}
