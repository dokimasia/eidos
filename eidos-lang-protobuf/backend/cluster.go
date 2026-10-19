// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"slices"
	"strings"
	"unicode"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ExtendGroup is the name of the group of the extensions of one message. The
// cluster assigns the extensions at file level of each extended message to
// it.
const ExtendGroup render.GroupName = "extend"

// ExtendTemplate renders an extend block at file level through
// [Vocabulary.Extension]: the head, the opening brace, the members and the
// closing brace, and then each declaration that follows the block through
// its kind template, after a blank line.
const ExtendTemplate = "{{$b := " + FuncExtension + " " + render.BuiltinHeader + " .}}{{$b.Head}} {{$b.Open}}" +
	blockItems + "}\n{{range $b.Next}}\n{{" + render.BuiltinNested + ` "" .}}` + "\n{{end}}"

// Groups returns the group templates that the cluster selects. It builds a
// new map on every call for the caller to keep.
func Groups() map[render.GroupName]string {
	return map[render.GroupName]string{ExtendGroup: ExtendTemplate}
}

// Cluster gathers the extensions at file level of a unit into one extend
// block for each message that they extend, by the spelling of the message,
// in the order of the first extension of each. A cluster also claims the
// message of a delimited extension whose type it is and whose name in lower
// case is the name of the extension, because proto2 writes such an extension
// as a group whose block contains the message. The message follows its
// extension in the cluster. Every declaration that is not an extension stays
// a singleton.
//
// # Allocation contract
//
// Cluster allocates the list of blocks and the list of their extended
// messages as they grow, and the list of declarations of each block as it
// grows. A unit without an extension allocates nothing.
func Cluster(decls []symbol.Symbol) []render.Clustered {
	var (
		out []render.Clustered
		// extended contains the spelling of the extended message of each
		// block of out.
		extended []string
	)
	for _, d := range decls {
		f, is := d.(*emit.Field)
		if !is || f.Receives == nil {
			continue
		}
		i := slices.Index(extended, f.Receives.Spelling)
		if i < 0 {
			i = len(out)
			extended = append(extended, f.Receives.Spelling)
			out = append(out, render.Clustered{Group: ExtendGroup})
		}
		out[i].Decls = append(out[i].Decls, d)
		if g := groupOf(f, f.Name, decls); g != nil {
			out[i].Decls = append(out[i].Decls, g)
		}
	}
	return out
}

// groupOf returns the message of the group of the delimited field f, whose
// member takes the name name, among decls: a message whose name is the type
// of the field, and whose name in lower case is name, as protoc names the
// field of a group. It returns nil for a field that is not delimited and for
// a field without such a message.
func groupOf(f *emit.Field, name string, decls []symbol.Symbol) *emit.Struct {
	t := f.Type
	if !marked(f.Annotations, protobuf.DelimitedMarker) || t == nil {
		return nil
	}
	if (t.Form == symbol.FormList || t.Form == symbol.FormOptional) && len(t.Elems) == 1 {
		t = t.Elems[0]
	}
	// A name of protobuf is ASCII, so a name without an upper-case letter
	// that equals the name of a message under case folding is that name in
	// lower case.
	if strings.ContainsFunc(name, unicode.IsUpper) {
		return nil
	}
	for _, d := range decls {
		if g, is := d.(*emit.Struct); is && g.Name == t.Spelling && strings.EqualFold(g.Name, name) {
			return g
		}
	}
	return nil
}
