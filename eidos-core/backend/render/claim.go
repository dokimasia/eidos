// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"io/fs"
	"path"
	"text/template"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// claim is what claims a routed file: the plugin of the file's units and
// the file template of their family.
type claim struct {
	plugin   plugin.ID
	template string
}

// claimView is what a claimed file's template executes over: the file's
// spelled name, its package identity, and its records.
type claimView struct {
	Name    string
	Pkg     symbol.Identity
	Records []fileRecord
}

// fileRecord is one declaration of a claimed file, with the data that
// its plugin attached to the declaration's origin, nil where it attached
// none.
type fileRecord struct {
	Decl symbol.Symbol
	Data any
}

// claimOf returns the claim of a routed file. A file whose units all
// have one plugin and one file template is claimed by them. A file
// without a unit of a claimed family is not claimed, and neither is a
// file that mixes the units of a claimed family with other units, which
// mixed reports.
func claimOf(g *plugin.File) (c claim, claimed, mixed bool) {
	for i, u := range g.Units {
		unit := claim{plugin: u.Plugin, template: u.Template}
		if i == 0 {
			c = unit
			continue
		}
		if unit != c && (c.template != "" || unit.template != "") {
			return claim{}, false, true
		}
	}
	return c, c.template != "", false
}

// fileBuiltins is the builtin set a claimed file's template resolves
// against: the body and the member body of a record's callable, the use
// recorder, the nested declaration, the header of the file and the
// writer of values. The render binds it to a frame and the lint binds
// stubs, so both parse against one set of names.
func fileBuiltins(
	body func(any) (string, error),
	member func(*emit.Method) (string, error),
	use func(string, ...string) (string, error),
	nested func(string, symbol.Symbol) (string, error),
	header func() *emit.File,
	value func(emit.Value) (string, error),
) template.FuncMap {
	return template.FuncMap{
		BuiltinBody: body, BuiltinMemberBody: member, BuiltinUse: use, BuiltinNested: nested,
		BuiltinHeader: header, BuiltinValue: value,
	}
}

// claimed executes the template of a claimed file into the frame's
// buffer, over the file's records in the order of its units and their
// declarations. An emit.File is the header of the file and no record,
// and the template reads it through the header builtin. The skeleton
// then frames the output as it frames the declarations of any other
// file. A false result means the file is withheld, and the finding that
// explains it is on the sink: the template does not resolve, or it
// failed at execution. A record body with slot content that the template
// did not place is reported under [DroppedSlots], and the file renders
// without the content.
func (f *frame) claimed(g *plugin.File, c claim) bool {
	f.plugin = c.plugin
	t, resolved := f.fileTemplate(c)
	if !resolved {
		return false
	}
	view := claimView{Name: path.Base(g.Path), Pkg: g.Pkg}
	for _, u := range g.Units {
		for _, d := range u.Decls {
			if _, header := d.(*emit.File); header {
				continue
			}
			origin, _ := emit.OriginOf(d)
			view.Records = append(view.Records, fileRecord{Decl: d, Data: u.Data[origin]})
		}
	}
	clear(f.placed)
	f.claiming = true
	err := t.Execute(&f.out, view)
	f.claiming = false
	if err != nil {
		f.sink.Errorf(RefusedTemplate, f.at, f.origin,
			"%s's file template %q refused %s: %v", c.plugin, c.template, g.Path, err)
		return false
	}
	if n := f.stranded(view.Records); n > 0 {
		f.sink.Errorf(DroppedSlots, f.at, f.origin,
			"%s's file template %q places no body for %d pending statements", c.plugin, c.template, n)
	}
	return true
}

// fileTemplate returns the parsed template of a claim, from the tree of
// the claiming plugin. Each frame reads and parses a template once per
// plugin and name. The template executes with missingkey=error, as a
// reference template does. A false result means nothing resolved, and
// the finding is on the sink under [UnresolvedRef]: no tree is declared
// for the plugin, its tree does not contain the template, or the
// template does not parse.
func (f *frame) fileTemplate(c claim) (*template.Template, bool) {
	key := refKey{plugin: c.plugin, name: c.template}
	if t, cached := f.fileCache[key]; cached {
		return t, true
	}
	tree, held := f.trees[c.plugin]
	if !held {
		f.sink.Errorf(UnresolvedRef, f.at, f.origin,
			"%s claims its files through %q, and no tree is declared for it", c.plugin, c.template)
		return nil, false
	}
	src, err := fs.ReadFile(tree, c.template)
	if err != nil {
		f.sink.Errorf(UnresolvedRef, f.at, f.origin,
			"%s claims its files through %q, which its tree does not contain", c.plugin, c.template)
		return nil, false
	}
	t, err := template.New(c.template).
		Option(missingKeyError).
		Funcs(f.vocab).Funcs(f.merged).
		Funcs(fileBuiltins(f.body, f.memberBody, f.use, f.nested, f.header, f.value)).
		Parse(string(src))
	if err != nil {
		f.sink.Errorf(UnresolvedRef, f.at, f.origin,
			"%s's file template %q does not parse: %v", c.plugin, c.template, err)
		return nil, false
	}
	if f.fileCache == nil {
		f.fileCache = map[refKey]*template.Template{}
	}
	f.fileCache[key] = t
	return t, true
}

// stranded counts the statements that wait in the slots of the record
// bodies that the template did not place: what a contributing plugin
// appended into a body of a record that the file does not contain.
func (f *frame) stranded(records []fileRecord) int {
	n := 0
	for _, r := range records {
		for s := range emit.All(r.Decl) {
			var b *emit.Body
			switch d := s.(type) {
			case *emit.Function:
				b = &d.Body
			case *emit.Method:
				b = &d.Body
			default:
				continue
			}
			if f.placed[b] {
				continue
			}
			n += b.Prologue.Len() + b.Epilogue.Len()
			for _, named := range b.Slots {
				n += named.Slot.Len()
			}
		}
	}
	return n
}
