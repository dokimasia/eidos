// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"context"
	"path"
	"strings"

	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the language of every declaration that the spec frontend
// loads.
const Lang symbol.Lang = "shapespec"

// ID is the name of the spec frontend, and the origin of its findings.
const ID plugin.ID = "shapespec"

// The version that every unit key of the frontend folds, the files that
// the frontend claims, the opener of a YAML comment, and the separators
// of the frontend's stamps.
const (
	version       = "1"
	selection     = "spec/**/*" + Extension
	commentOpener = "#"
	arityJoin     = "="
	lineBreak     = "\n"
)

// New returns the spec frontend. It claims every YAML file below spec/,
// makes each directory one unit, and declares one package for each
// directory, one file for each spec file, and one struct for each spec:
//
//   - The struct has the spec's name and the claim as its documentation.
//   - The struct has one field for each param, with the param's key, a
//     type reference spelled as the param's type, and the param's doc.
//   - The struct has one field for each binding of a shape, with the
//     binding's name, a type reference spelled as a reference, and the
//     binding's doc.
//
// The frontend stamps the values of a spec that the declarations do not
// state under the keys of [Keys]. A spec refers to nothing outside
// itself, so the frontend's resolution returns no candidate. Each fault
// of a spec is an Error under [SpecInvalid] at its position, and a spec
// with a fault declares no struct.
func New() plugin.Frontend {
	return frontend.New(ID, Lang, plugin.CommentSyntax{Line: []string{commentOpener}}).
		Version(version).
		Match(selection).
		Units(partition).
		Parse(parse).
		Resolve(func(plugin.ImportScope, string) plugin.Candidates { return plugin.Candidates{} }).
		Build()
}

// partition makes each directory of the selected files one unit, in the
// order of the directories' first files.
func partition(_ context.Context, files []plugin.SourceRef, _ plugin.FileReader) ([][]plugin.SourceRef, error) {
	var units [][]plugin.SourceRef
	byDir := map[string]int{}
	for _, f := range files {
		dir := path.Dir(f.Path)
		i, seen := byDir[dir]
		if !seen {
			i = len(units)
			byDir[dir] = i
			units = append(units, nil)
		}
		units[i] = append(units[i], f)
	}
	return units, nil
}

// parse decodes each file of a unit and declares its spec into the unit's
// graph. It returns the error of a file that does not read.
func parse(_ context.Context, u *plugin.SourceUnit) error {
	report := func(at position.Pos, format string, args ...any) {
		u.Errorf(SpecInvalid, at, format, args...)
	}
	gb := u.Graph()
	for _, ref := range u.Files() {
		data, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		dir := path.Dir(ref.Path)
		pkg := gb.Package(dir)
		pkg.Name = path.Base(dir)
		file := &node.File{Path: ref.Path}
		pkg.Files = append(pkg.Files, file)
		if s, valid := decode(ref.Path, data, report); valid {
			declare(gb, file, s)
		}
	}
	return nil
}

// declare declares one spec into a file of the graph, and stamps the
// values that the declarations do not state.
func declare(gb *plugin.GraphBuilder, file *node.File, s spec) {
	st := &node.Struct{Name: s.name, Doc: strings.Split(strings.TrimSpace(s.claim), lineBreak), Pos: s.at}
	file.Decls = append(file.Decls, st)
	gb.Stamp(st, meta.RawStamp{Key: KeyForm, Value: s.form, Pos: s.at})
	if s.detected {
		gb.Stamp(st, meta.RawStamp{Key: KeyDetected, Value: true, Pos: s.at})
	}
	if s.documentary {
		gb.Stamp(st, meta.RawStamp{Key: KeyDocumentary, Value: true, Pos: s.at})
	}
	if len(s.yields) > 0 {
		gb.Stamp(st, meta.RawStamp{Key: KeyYields, Value: s.yields, Pos: s.at})
	}
	if len(s.roles) > 0 {
		roles := make([]string, 0, len(s.roles))
		for _, r := range s.roles {
			roles = append(roles, r.name+arityJoin+string(r.arity))
		}
		gb.Stamp(st, meta.RawStamp{Key: KeyRoles, Value: roles, Pos: s.at})
	}
	for _, p := range s.params {
		f := &node.Field{
			Name: string(p.Key), Type: &node.TypeRef{Spelling: string(p.Type)},
			Doc: strings.Split(strings.TrimSpace(p.Doc), lineBreak), Pos: p.at,
		}
		st.Fields = append(st.Fields, f)
		stampParam(gb, f, p)
	}
	for _, b := range s.bindings {
		f := &node.Field{
			Name: b.name, Type: &node.TypeRef{Spelling: string(TypeReference)},
			Doc: strings.Split(strings.TrimSpace(b.Doc), lineBreak), Pos: b.at,
		}
		st.Fields = append(st.Fields, f)
		gb.Stamp(f, meta.RawStamp{Key: KeyFrom, Value: string(b.From), Pos: b.at})
		gb.Stamp(f, meta.RawStamp{Key: KeyIndex, Value: int64(b.Index), Pos: b.at})
	}
}

// stampParam stamps the values of a param that its field does not state.
func stampParam(gb *plugin.GraphBuilder, f *node.Field, p param) {
	if p.Resolve != "" {
		gb.Stamp(f, meta.RawStamp{Key: KeyResolve, Value: string(p.Resolve), Pos: p.at})
	}
	if p.Required {
		gb.Stamp(f, meta.RawStamp{Key: KeyRequired, Value: true, Pos: p.at})
	}
	if p.Counterexample {
		gb.Stamp(f, meta.RawStamp{Key: KeyCounterexample, Value: true, Pos: p.at})
	}
	if p.Minimum != nil {
		gb.Stamp(f, meta.RawStamp{Key: KeyMinimum, Value: *p.Minimum, Pos: p.at})
	}
	lists := []struct {
		key   meta.KeyName
		names []Name
	}{{KeyApplies, p.Roles}, {KeyExcludes, p.Excludes}, {KeyAlsoOn, p.AlsoOn}}
	for _, l := range lists {
		if len(l.names) == 0 {
			continue
		}
		values := make([]string, 0, len(l.names))
		for _, n := range l.names {
			values = append(values, string(n))
		}
		gb.Stamp(f, meta.RawStamp{Key: l.key, Value: values, Pos: p.at})
	}
}
