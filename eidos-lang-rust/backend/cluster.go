// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ImplGroup is the name of the impl-block group. The cluster assigns the
// methods of each receiver type to it.
const ImplGroup render.GroupName = "impl"

// ImplTemplate renders an impl block with the methods of one receiver
// type. Each method takes its own receiver, a shared reference where it
// has none, and no receiver at type level. The visibility and async of a
// method precede fn, its type parameter list follows the name, and its
// trailing comment follows the braces of its body. Each method has its
// own doc lines and attributes. The type parameters of the receiver open
// the impl binder, so a method on Box<T> opens impl<T> Box<T>.
const ImplTemplate = "impl{{binder (index .Decls 0).Receives}}" +
	" {{spell (index .Decls 0).Receives}} {\n" +
	"{{- range .Decls}}\n{{memberdocs .Doc}}{{memberattrs .Annotations}}" +
	"    {{implfn .}}fn {{.Name}}{{fnparams .TypeParams}}({{selfparams .}})" +
	"{{results .Returns}} {\n{{body .}}    }{{with .Comment}} // {{.}}{{end}}\n" +
	"{{- end}}\n}\n"

// Groups returns the group templates that the cluster selects. It builds
// a new map on every call for the caller to keep. The map and the one
// table of slots that contains the template allocate twice.
func Groups() map[render.GroupName]string {
	return map[render.GroupName]string{ImplGroup: ImplTemplate}
}

// Cluster gathers the methods of a unit into one impl block per receiver
// type, in the order of first appearance, because Rust renders a method
// only inside the impl block of its type. The type is the whole
// reference with its arguments, so a method on Wrapper<String> and a
// method on Wrapper<i32> open two blocks. Cluster leaves a method without
// a Receives type unassigned, and the lowering refuses such a method
// before any render. Every declaration that is not a method stays a
// singleton.
//
// # Allocation contract
//
// Cluster allocates the key of each instantiated receiver, the list of
// blocks as it grows, and the method list of each block as it grows. Two
// methods of one type and a method of an instantiated type allocate six
// times. The index of receiver types is on the stack for up to eight
// types, and allocates a table on the heap above that.
func Cluster(decls []symbol.Symbol) []render.Clustered {
	byType := map[string]int{}
	var out []render.Clustered
	for _, d := range decls {
		m, is := d.(*emit.Method)
		if !is || m.Receives == nil || m.Receives.Spelling == "" {
			continue
		}
		key := spellref.Spell(m.Receives, genericsOpener, genericsCloser, unitSpelling)
		i, seen := byType[key]
		if !seen {
			i = len(out)
			byType[key] = i
			out = append(out, render.Clustered{Group: ImplGroup})
		}
		out[i].Decls = append(out[i].Decls, d)
	}
	return out
}
