// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/scanner"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// carrierMark opens a directive carrier line inside a doc comment.
const carrierMark = "+"

// parse loads one unit: each member through go/parser, lowered
// into the unit's builder. A syntax error is the source's problem
// and reports positioned; a file whose build constraint falls
// outside the load's tag set contributes its file node and a
// constraint stamp and no declarations.
func (f *goFrontend) parse(_ context.Context, u *plugin.SourceUnit) error {
	root := (&moduleProbe{reader: unitReader{u}, roots: map[string]moduleRoot{}}).
		governing(path.Dir(u.Files()[0].Path))
	for _, ref := range u.Files() {
		if err := f.parseFile(u, root, ref.Path); err != nil {
			return err
		}
	}
	return nil
}

// unitReader adapts the unit's jailed door to the partition's
// probe, so parse derives the same import paths partition did:
// the go.mod is a declared shared input, so the read admits.
type unitReader struct {
	u *plugin.SourceUnit
}

// Read reads through the unit's jail.
func (r unitReader) Read(path string) ([]byte, error) { return r.u.Read(path) }

// parseFile lowers one member.
func (f *goFrontend) parseFile(u *plugin.SourceUnit, root moduleRoot, filePath string) error {
	src, err := u.Read(filePath)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filePath, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		u.Errorf(UnparsedFile, parseErrorPos(filePath, err), "%v", err)
		return nil
	}
	l := &lowered{fset: fset, src: src}
	gb := u.Graph()

	pkgPath := root.importPath(path.Dir(filePath))
	pkgName := parsed.Name.Name
	if strings.HasSuffix(pkgName, "_test") {
		pkgPath += "_test"
	}
	pkg := gb.Package(pkgPath)
	pkg.Name = pkgName

	file := &node.File{Path: filePath, Pos: l.at(parsed.Package)}
	pkg.Files = append(pkg.Files, file)

	fileBinds := bindings{}
	for _, spec := range parsed.Imports {
		lowerImport(l, file, fileBinds, spec)
	}
	gb.Scope(file, fileBinds)

	docs, carriers := f.split(u, parsed.Doc)
	file.Doc = docs
	attach(u, gb, file, carriers)

	if line, excluded := f.excluded(src); excluded {
		gb.Stamp(file, meta.RawStamp{
			Key: ConstraintKey, Value: line, Pos: file.Pos,
		})
		return nil
	}
	for _, decl := range parsed.Decls {
		f.lowerDecl(u, l, file, decl)
	}
	return nil
}

// excluded reports whether a file's go:build constraint falls
// outside the load's tag set, and the constraint line when it
// does. The legacy +build form is not read: the gofmt of every
// supported toolchain writes go:build, and a file carrying only
// the legacy form loads unconditionally.
func (f *goFrontend) excluded(src []byte) (string, bool) {
	for line := range strings.SplitSeq(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			break
		}
		if !constraint.IsGoBuild(trimmed) {
			continue
		}
		expr, err := constraint.Parse(trimmed)
		if err != nil {
			continue
		}
		satisfied := expr.Eval(func(tag string) bool {
			return slices.Contains(f.opts.Tags, tag)
		})
		if !satisfied {
			return trimmed, true
		}
	}
	return "", false
}

// parseErrorPos positions a parse failure at its first error.
func parseErrorPos(filePath string, err error) position.Pos {
	var list scanner.ErrorList
	if errors.As(err, &list) && len(list) > 0 {
		return position.Pos{File: filePath, Line: list[0].Pos.Line, Col: list[0].Pos.Column}
	}
	return position.Pos{File: filePath, Line: 1, Col: 1}
}

// lowerImport records one import: the model's record on the file,
// and the binding Resolve reads. A blank import binds nothing and
// a dot import binds every exported name, which the record carries
// as a wildcard and resolution does not yet probe.
func lowerImport(l *lowered, file *node.File, b bindings, spec *ast.ImportSpec) {
	imported, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return
	}
	record := &node.Import{Path: imported, Pos: l.at(spec.Pos())}
	local := path.Base(imported)
	switch {
	case spec.Name == nil:
		b[local] = imported
	case spec.Name.Name == "_":
	case spec.Name.Name == ".":
		record.Wildcard = true
	default:
		record.Alias = spec.Name.Name
		b[spec.Name.Name] = imported
	}
	file.Imports = append(file.Imports, record)
}

// split strips one doc comment through the unit's pipeline and
// separates the carrier lines: a +-prefixed line is a directive,
// never documentation.
func (*goFrontend) split(u *plugin.SourceUnit, doc *ast.CommentGroup) ([]string, []string) {
	if doc == nil {
		return nil, nil
	}
	raw := make([]string, 0, len(doc.List))
	for _, c := range doc.List {
		raw = append(raw, c.Text)
	}
	var docs, carriers []string
	for _, line := range u.Doc(strings.Join(raw, "\n")) {
		if rest, carried := strings.CutPrefix(line, carrierMark); carried {
			carriers = append(carriers, rest)
			continue
		}
		docs = append(docs, line)
	}
	return docs, carriers
}

// attach parses each carrier under the kernel grammar and records
// it on its subject; a carrier outside the grammar reports and
// attaches nothing.
func attach(u *plugin.SourceUnit, gb *plugin.GraphBuilder, subject symbol.Symbol, carriers []string) {
	for _, payload := range carriers {
		raw, err := directive.Parse(payload)
		if err != nil {
			u.Errorf(BadCarrier, subject.Position(), "%q: %v", carrierMark+payload, err)
			continue
		}
		raw.Pos = subject.Position()
		gb.Attach(subject, raw)
	}
}

// lowerDecl lowers one top-level declaration, observing the unit's
// depth: at signatures, an unexported declaration stays out.
func (f *goFrontend) lowerDecl(u *plugin.SourceUnit, l *lowered, file *node.File, decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if u.Depth() == plugin.DepthSignatures && !ast.IsExported(d.Name.Name) {
			return
		}
		f.lowerFunc(u, l, file, d)
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if u.Depth() == plugin.DepthSignatures && !ast.IsExported(s.Name.Name) {
					continue
				}
				f.lowerType(u, l, file, d, s)
			case *ast.ValueSpec:
				f.lowerValues(u, l, file, d, s)
			}
		}
	}
}

// lowerFunc lowers a function or, with a receiver, a method whose
// owner is the receiver's named type.
func (f *goFrontend) lowerFunc(u *plugin.SourceUnit, l *lowered, file *node.File, d *ast.FuncDecl) {
	docs, carriers := f.split(u, d.Doc)
	if d.Recv == nil {
		fn := &node.Function{
			Name: d.Name.Name, Pos: l.at(d.Pos()), Doc: docs,
			Visibility: visibilityOf(d.Name.Name),
			TypeParams: l.typeParams(d.Type.TypeParams),
			Params:     l.params(d.Type.Params),
			Returns:    l.returns(d.Type.Results),
		}
		file.Decls = append(file.Decls, fn)
		attach(u, u.Graph(), fn, carriers)
		return
	}
	recv := d.Recv.List[0]
	m := &node.Method{
		Name: d.Name.Name, Pos: l.at(d.Pos()), Doc: docs,
		Visibility: visibilityOf(d.Name.Name),
		Receives:   &node.TypeRef{Spelling: l.bareName(recv.Type), Pos: l.at(recv.Pos())},
		Receiver:   l.param(recv),
		TypeParams: l.typeParams(d.Type.TypeParams),
		Params:     l.params(d.Type.Params),
		Returns:    l.returns(d.Type.Results),
	}
	file.Decls = append(file.Decls, m)
	attach(u, u.Graph(), m, carriers)
}

// lowerType lowers one type spec: an alias, a struct, an
// interface, or a defined type over another spelling.
func (f *goFrontend) lowerType(u *plugin.SourceUnit, l *lowered, file *node.File, d *ast.GenDecl, s *ast.TypeSpec) {
	doc := s.Doc
	if doc == nil {
		doc = d.Doc
	}
	docs, carriers := f.split(u, doc)
	name, pos := s.Name.Name, l.at(s.Pos())
	vis := visibilityOf(name)
	tps := l.typeParams(s.TypeParams)

	var lowered symbol.Symbol
	switch t := s.Type.(type) {
	case *ast.StructType:
		st := &node.Struct{
			Name: name, Pos: pos, Doc: docs, Visibility: vis, TypeParams: tps,
		}
		f.lowerStructBody(u, l, st, t)
		lowered = st
	case *ast.InterfaceType:
		it := &node.Interface{
			Name: name, Pos: pos, Doc: docs, Visibility: vis, TypeParams: tps,
		}
		f.lowerInterfaceBody(u, l, it, t)
		lowered = it
	default:
		lowered = &node.Alias{
			Name: name, Pos: pos, Doc: docs, Visibility: vis, TypeParams: tps,
			Defined: !s.Assign.IsValid(),
			Target:  l.typeRef(s.Type),
		}
	}
	file.Decls = append(file.Decls, lowered)
	attach(u, u.Graph(), lowered, carriers)
}

// lowerStructBody lowers fields and embeds, unexported fields
// staying out at signature depth.
func (f *goFrontend) lowerStructBody(u *plugin.SourceUnit, l *lowered, st *node.Struct, t *ast.StructType) {
	for _, field := range t.Fields.List {
		if len(field.Names) == 0 {
			st.Embeds = append(st.Embeds, &node.Embed{
				Ref: l.typeRef(field.Type), Pos: l.at(field.Pos()),
			})
			continue
		}
		docs, _ := f.split(u, field.Doc)
		for _, name := range field.Names {
			if u.Depth() == plugin.DepthSignatures && !ast.IsExported(name.Name) {
				continue
			}
			st.Fields = append(st.Fields, &node.Field{
				Name: name.Name, Pos: l.at(name.Pos()), Doc: docs,
				Visibility: visibilityOf(name.Name),
				Type:       l.typeRef(field.Type),
				Tag:        tagOf(field.Tag),
				Comment:    lineComment(field.Comment),
			})
		}
	}
}

// lowerInterfaceBody lowers method signatures and embedded
// interfaces.
func (f *goFrontend) lowerInterfaceBody(u *plugin.SourceUnit, l *lowered, it *node.Interface, t *ast.InterfaceType) {
	for _, member := range t.Methods.List {
		if len(member.Names) == 0 {
			it.Embeds = append(it.Embeds, &node.Embed{
				Ref: l.typeRef(member.Type), Pos: l.at(member.Pos()),
			})
			continue
		}
		sig, is := member.Type.(*ast.FuncType)
		if !is {
			continue
		}
		name := member.Names[0].Name
		if u.Depth() == plugin.DepthSignatures && !ast.IsExported(name) {
			continue
		}
		docs, _ := f.split(u, member.Doc)
		it.Methods = append(it.Methods, &node.Method{
			Name: name, Pos: l.at(member.Pos()), Doc: docs,
			Visibility: visibilityOf(name),
			Abstract:   true,
			Params:     l.params(sig.Params),
			Returns:    l.returns(sig.Results),
		})
	}
}

// lowerValues lowers one const or var spec, a declaration per
// bound name. An implicit constant — an iota carrier with no
// expression of its own — keeps an empty value, unevaluated by
// contract.
func (f *goFrontend) lowerValues(u *plugin.SourceUnit, l *lowered, file *node.File, d *ast.GenDecl, s *ast.ValueSpec) {
	docs, carriers := f.split(u, firstDoc(s.Doc, d.Doc))
	for i, name := range s.Names {
		if name.Name == "_" {
			continue
		}
		if u.Depth() == plugin.DepthSignatures && !ast.IsExported(name.Name) {
			continue
		}
		var value string
		if i < len(s.Values) {
			value = l.spelling(s.Values[i])
		}
		var declared symbol.Symbol
		if d.Tok == token.CONST {
			declared = &node.Constant{
				Name: name.Name, Pos: l.at(name.Pos()), Doc: docs,
				Visibility: visibilityOf(name.Name),
				Type:       l.typeRef(s.Type),
				Value:      value,
				Comment:    lineComment(s.Comment),
			}
		} else {
			declared = &node.Variable{
				Name: name.Name, Pos: l.at(name.Pos()), Doc: docs,
				Visibility: visibilityOf(name.Name),
				Mutability: symbol.MutabilityMutable,
				Type:       l.typeRef(s.Type),
				Value:      value,
				Comment:    lineComment(s.Comment),
			}
		}
		file.Decls = append(file.Decls, declared)
		attach(u, u.Graph(), declared, carriers)
	}
}

// typeParams lowers a generic parameter list, each bound carried
// as a reference.
func (l *lowered) typeParams(fields *ast.FieldList) []*node.TypeParam {
	if fields == nil {
		return nil
	}
	var out []*node.TypeParam
	for _, field := range fields.List {
		for _, name := range field.Names {
			out = append(out, &node.TypeParam{
				Name: name.Name, Pos: l.at(name.Pos()),
				Bounds: []*node.TypeRef{l.typeRef(field.Type)},
			})
		}
	}
	return out
}

// params lowers a parameter list, one entry per bound name and one
// for an unnamed parameter.
func (l *lowered) params(fields *ast.FieldList) []*node.Param {
	if fields == nil {
		return nil
	}
	var out []*node.Param
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			out = append(out, l.param(field))
			continue
		}
		for _, name := range field.Names {
			p := l.param(field)
			p.Name = name.Name
			out = append(out, p)
		}
	}
	return out
}

// param lowers one field into a parameter, variadic when its type
// is an ellipsis.
func (l *lowered) param(field *ast.Field) *node.Param {
	p := &node.Param{Pos: l.at(field.Pos())}
	if len(field.Names) == 1 {
		p.Name = field.Names[0].Name
	}
	if ellipsis, variadic := field.Type.(*ast.Ellipsis); variadic {
		p.Variadic = symbol.VariadicPositional
		p.Type = l.typeRef(ellipsis.Elt)
		return p
	}
	p.Type = l.typeRef(field.Type)
	return p
}

// returns lowers a result list.
func (l *lowered) returns(fields *ast.FieldList) []*node.Return {
	if fields == nil {
		return nil
	}
	var out []*node.Return
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			out = append(out, &node.Return{Pos: l.at(field.Pos()), Type: l.typeRef(field.Type)})
			continue
		}
		for _, name := range field.Names {
			out = append(out, &node.Return{
				Name: name.Name, Pos: l.at(name.Pos()), Type: l.typeRef(field.Type),
			})
		}
	}
	return out
}

// visibilityOf reads Go's visibility off the name's case.
func visibilityOf(name string) symbol.Visibility {
	if ast.IsExported(name) {
		return symbol.VisibilityPublic
	}
	return symbol.VisibilityPackage
}

// tagOf strips a struct tag's delimiters, the text unevaluated:
// the literal unquotes, whichever quoting the author wrote.
func tagOf(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}
	unquoted, err := strconv.Unquote(tag.Value)
	if err != nil {
		return strings.Trim(tag.Value, "`")
	}
	return unquoted
}

// lineComment returns a field's trailing comment text, empty when
// none.
func lineComment(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}

// firstDoc prefers a spec's own doc over its group's.
func firstDoc(spec, group *ast.CommentGroup) *ast.CommentGroup {
	if spec != nil {
		return spec
	}
	return group
}
