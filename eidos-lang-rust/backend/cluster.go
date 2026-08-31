// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// ImplGroup names the impl-block group the cluster selects.
const ImplGroup render.GroupName = "impl"

// ImplTemplate spells an impl block: the methods attached to one
// type, each taking the receiver by reference with its own type
// parameter list, its body placed, each under its own doc lines.
// A generic receiver's arguments restate as the impl binder, so a
// method on Box<T> opens impl<T> Box<T>.
const ImplTemplate = "impl{{binder (index .Decls 0).Receives}}" +
	" {{spell (index .Decls 0).Receives}} {\n" +
	"{{- range .Decls}}\n{{docs .Doc \"    \"}}" +
	"    pub fn {{.Name}}{{typeparams .TypeParams}}(&self{{with params .Params}}, {{.}}{{end}})" +
	"{{results .Returns}} {\n{{body .}}    }\n" +
	"{{- end}}\n}\n"

// Groups returns the group templates the cluster selects.
func Groups() map[render.GroupName]string {
	return map[render.GroupName]string{ImplGroup: ImplTemplate}
}

// Cluster gathers a unit's methods into one impl block per
// attached type, in first-appearance order, because Rust renders
// a method only inside an impl grouped by the type it attaches
// to. A method attaching to no type stays unassigned, and the
// render reports it as a kind the target cannot spell; everything
// that is not a method stays a singleton.
func Cluster(decls []symbol.Symbol) []render.Clustered {
	byType := map[string]int{}
	var out []render.Clustered
	for _, d := range decls {
		m, held := d.(*emit.Method)
		if !held || m.Receives == nil || m.Receives.Spelling == "" {
			continue
		}
		i, held := byType[m.Receives.Spelling]
		if !held {
			i = len(out)
			byType[m.Receives.Spelling] = i
			out = append(out, render.Clustered{Group: ImplGroup})
		}
		out[i].Decls = append(out[i].Decls, d)
	}
	return out
}
