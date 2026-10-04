// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ImplGroup names the impl-block group the cluster selects.
const ImplGroup render.GroupName = "impl"

// ImplTemplate spells an impl block: the methods attached to one
// type, each taking the receiver the method states, by shared
// reference where it states none, and no receiver at type level. A
// method's visibility and asynchrony go before fn, its own type
// parameter list behind the name, its body in braces and its
// trailing comment behind them, each under its own doc lines and
// attributes. The receiver's type parameters open the impl binder,
// so a method on Box<T> opens impl<T> Box<T>.
const ImplTemplate = "impl{{binder (index .Decls 0).Receives}}" +
	" {{spell (index .Decls 0).Receives}} {\n" +
	"{{- range .Decls}}\n{{docs .Doc \"    \"}}{{attrs .Annotations \"    \"}}" +
	"    {{implfn .}}fn {{.Name}}{{fnparams .TypeParams}}({{selfparams .}})" +
	"{{results .Returns}} {\n{{body .}}    }{{with .Comment}} // {{.}}{{end}}\n" +
	"{{- end}}\n}\n"

// Groups returns the group templates the cluster selects. It builds
// the map on every call, which the caller keeps: the map and the one
// table of slots that stores the template, two allocations.
func Groups() map[render.GroupName]string {
	return map[render.GroupName]string{ImplGroup: ImplTemplate}
}

// Cluster gathers a unit's methods into one impl block per
// attached type, in first-appearance order, because Rust renders
// a method only inside an impl grouped by the type it attaches
// to. The type is the whole reference, arguments included, so a
// method on Wrapper<String> and one on Wrapper<i32> open two
// blocks. A method attaching to no type is left unassigned, and
// the lowering refuses such a method before any render.
// Everything that is not a method is left a singleton.
//
// # Allocation contract
//
// Cluster allocates the key of each instantiated receiver, the list of
// blocks as it grows, and each block's list of methods as it grows: six
// allocations for two methods of one type beside a method of an
// instantiated type. The index of receiver types is on the stack for up
// to eight types, and allocates a table on the heap above that.
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
