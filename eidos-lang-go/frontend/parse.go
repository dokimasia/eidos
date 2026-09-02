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

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// maxSyntaxFindings caps the per-file error flood; the count past
// the cap reports once.
const maxSyntaxFindings = 10

// pendingUnderlying is one defined type's shape, stamped once the
// promotion has decided whether an enum stands for it.
type pendingUnderlying struct {
	alias *node.Alias
	kind  string
}

// parse loads one unit: each member through go/parser, lowered
// into the unit's builder. A syntax error is the source's problem:
// every error reports positioned and every declaration the parser
// still recovered loads, because one bad token must not erase a
// file. A file whose build constraint — spelled or
// filename-implied — falls outside the load's tag set contributes
// its file node and a golang.constraint stamp and no declarations.
func (f *goFrontend) parse(_ context.Context, u *plugin.SourceUnit) error {
	root := (&moduleProbe{reader: unitReader{u}, roots: map[string]moduleRoot{}}).
		governing(path.Dir(u.Files()[0].Path))
	st := &parseState{
		fset:    token.NewFileSet(),
		intern:  map[string]string{},
		batches: map[string]*constBatch{},
	}
	for _, ref := range u.Files() {
		if err := f.parseFile(u, root, st, ref.Path); err != nil {
			return err
		}
	}
	for _, pkgPath := range st.order {
		stampConstValues(u, st.fset, st.batches[pkgPath])
	}
	return nil
}

// parseState is one unit's shared parse machinery: the file set
// every member joins, the spelling intern the lowerings share, and
// the per-package batches the constant evaluation runs over once
// each, so a constant referencing a sibling file's type still
// evaluates.
type parseState struct {
	fset    *token.FileSet
	intern  map[string]string
	batches map[string]*constBatch
	order   []string
}

// constBatch is one package's included files, parsed and lowered.
type constBatch struct {
	name   string
	parsed []*ast.File
	files  []*node.File
}

// batch returns the package's batch, created on first touch.
func (st *parseState) batch(pkgPath, name string) *constBatch {
	if b, held := st.batches[pkgPath]; held {
		return b
	}
	b := &constBatch{name: name}
	st.batches[pkgPath] = b
	st.order = append(st.order, pkgPath)
	return b
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
func (f *goFrontend) parseFile(
	u *plugin.SourceUnit, root moduleRoot, st *parseState, filePath string,
) error {
	src, err := u.Read(filePath)
	if err != nil {
		return err
	}

	parsed, err := parser.ParseFile(st.fset, filePath, src,
		parser.ParseComments|parser.SkipObjectResolution|parser.AllErrors)
	if err != nil {
		reportSyntax(u, filePath, err)
	}
	if parsed == nil || parsed.Name == nil {
		return nil
	}
	l := &lowered{
		file: st.fset.File(parsed.Package), src: src,
		intern: st.intern, consumed: map[*ast.CommentGroup]bool{},
	}
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

	fileBinds := newBindings()
	cgo := false
	for _, spec := range parsed.Imports {
		lowerImport(l, file, fileBinds, spec)
		if imported, unquoteErr := strconv.Unquote(spec.Path.Value); unquoteErr == nil && imported == "C" {
			cgo = true
		}
	}
	gb.Scope(file, fileBinds)

	// The package clause's doc belongs to the package: the text
	// hoists to the first non-empty doc across the unit's files,
	// and its carriers attach to the package rather than the file.
	// Tool directives above the clause have no model home yet and
	// drop, stated in the package documentation.
	parts := l.split(u, parsed.Doc)
	file.Doc = parts.Docs
	if len(pkg.Doc) == 0 {
		pkg.Doc = parts.Docs
	}
	attachCarriers(u, gb, pkg, parts.Carriers)

	if cgo {
		gb.Stamp(file, meta.RawStamp{Key: golang.CgoKey, Value: true, Pos: file.Pos})
	}
	if marker, generated := generatedMarker(parsed); generated {
		gb.Stamp(file, meta.RawStamp{Key: golang.GeneratedKey, Value: marker, Pos: file.Pos})
	}
	if line, excluded := f.excluded(parsed, filePath); excluded {
		gb.Stamp(file, meta.RawStamp{Key: golang.ConstraintKey, Value: line, Pos: file.Pos})
		return nil
	}
	for _, decl := range parsed.Decls {
		f.lowerDecl(u, l, file, decl)
	}
	promoted, moved := promoteEnums(file)
	for from, to := range moved {
		// A carrier or stamp recorded against a consumed alias or
		// constant follows the enum or variant that stands, so the
		// splice never meets a subject the promotion removed.
		gb.Rehome(from, to)
	}
	for _, pending := range l.underlyings {
		var subject symbol.Symbol = pending.alias
		if enum, replaced := promoted[pending.alias]; replaced {
			subject = enum
		}
		gb.Stamp(subject, meta.RawStamp{
			Key: golang.UnderlyingKey, Value: pending.kind, Pos: subject.Position(),
		})
	}
	batch := st.batch(pkgPath, pkgName)
	batch.parsed = append(batch.parsed, parsed)
	batch.files = append(batch.files, file)

	// The sweep: a carrier in a comment no declaration consumed —
	// floating between declarations, or beside a parameter, whose
	// comments the parser attaches to nothing — refuses positioned
	// rather than vanishing.
	for _, group := range parsed.Comments {
		if l.consumed[group] {
			continue
		}
		floating := l.split(u, group)
		refuseCarriers(u, floating.Carriers, "a comment no declaration owns")
	}
	return nil
}

// reportSyntax reports each recovered syntax error at its own
// position, capped, the remainder counted once.
func reportSyntax(u *plugin.SourceUnit, filePath string, err error) {
	var list scanner.ErrorList
	if !errors.As(err, &list) || len(list) == 0 {
		u.Errorf(UnparsedFile, position.Pos{File: filePath, Line: 1, Col: 1}, "%v", err)
		return
	}
	for i, e := range list {
		if i == maxSyntaxFindings {
			u.Errorf(UnparsedFile,
				position.Pos{File: filePath, Line: e.Pos.Line, Col: e.Pos.Column},
				"and %d more syntax errors", len(list)-maxSyntaxFindings)
			break
		}
		u.Errorf(UnparsedFile,
			position.Pos{File: filePath, Line: e.Pos.Line, Col: e.Pos.Column}, "%s", e.Msg)
	}
}

// excluded reports whether a file falls outside the load's tag
// set, and the constraint that says so: a go:build line read off
// the parsed comments before the package clause — a spelling
// inside a block comment is not a constraint, which a raw line
// scan gets wrong — or the filename's own implied GOOS and GOARCH
// suffixes.
func (f *goFrontend) excluded(parsed *ast.File, filePath string) (string, bool) {
	for _, group := range parsed.Comments {
		if group.Pos() >= parsed.Package {
			break
		}
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, "//") || !constraint.IsGoBuild(c.Text) {
				continue
			}
			expr, err := constraint.Parse(c.Text)
			if err != nil {
				continue
			}
			if !expr.Eval(f.satisfies) {
				return c.Text, true
			}
		}
	}
	if implied, constrained := filenameConstraint(path.Base(filePath)); constrained {
		for _, tag := range implied {
			if !f.satisfies(tag) {
				return strings.Join(implied, " && "), true
			}
		}
	}
	return "", false
}

// satisfies reports whether the load's tag set holds a tag.
func (f *goFrontend) satisfies(tag string) bool {
	return slices.Contains(f.opts.Tags, tag)
}

// generatedMarker returns the Go convention's generated-file
// marker when one sits before the package clause.
func generatedMarker(parsed *ast.File) (string, bool) {
	for _, group := range parsed.Comments {
		if group.Pos() >= parsed.Package {
			break
		}
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "// Code generated ") &&
				strings.HasSuffix(c.Text, " DO NOT EDIT.") {
				return c.Text, true
			}
		}
	}
	return "", false
}

// lowerImport records one import: the model's record on the file,
// and the binding Resolve reads. A dot import binds every exported
// name, carried as a wildcard the resolution does not yet probe; a
// blank import keeps its underscore as the alias, so a side-effect
// import round-trips as one. The default local name is the path's
// last segment, a stated heuristic: without a cross-package read,
// a package whose clause differs from its path resolves nothing,
// and the spelling stays visible.
func lowerImport(l *lowered, file *node.File, b *bindings, spec *ast.ImportSpec) {
	imported, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return
	}
	record := &node.Import{Path: imported, Pos: l.at(spec.Pos())}
	switch {
	case spec.Name == nil:
		b.named[path.Base(imported)] = imported
	case spec.Name.Name == "_":
		record.Alias = "_"
	case spec.Name.Name == ".":
		record.Wildcard = true
		b.dots = append(b.dots, imported)
	default:
		record.Alias = spec.Name.Name
		b.named[spec.Name.Name] = imported
	}
	b.all = append(b.all, imported)
	file.Imports = append(file.Imports, record)
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
		group := l.split(u, d.Doc)
		// An empty const spec repeats the previous one, the type
		// included: Go's own inheritance rule, applied here so an
		// implicit iota row still knows what it is typed by.
		var inherited ast.Expr
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if u.Depth() == plugin.DepthSignatures && !ast.IsExported(s.Name.Name) {
					continue
				}
				f.lowerType(u, l, file, group, s)
			case *ast.ValueSpec:
				specType := s.Type
				if d.Tok == token.CONST {
					switch {
					case s.Type != nil:
						inherited = s.Type
					case len(s.Values) == 0:
						specType = inherited
					default:
						inherited = nil
					}
				}
				f.lowerValues(u, l, file, group, d.Tok, s, specType)
			}
		}
	}
}

// declParts joins a spec's own comments with its group's: the
// nearer doc text stands, and carriers and annotations union
// across the group doc, the spec doc and the trailing comment,
// because a directive is authored intent wherever it sits.
func (l *lowered) declParts(
	u *plugin.SourceUnit, group plugin.CommentParts, doc, trailing *ast.CommentGroup,
) plugin.CommentParts {
	parts := merge(l.split(u, doc), group)
	tail := l.split(u, trailing)
	parts.Carriers = append(parts.Carriers, tail.Carriers...)
	parts.Annotations = append(parts.Annotations, tail.Annotations...)
	return parts
}

// lowerFunc lowers a function or, with a receiver, a method whose
// owner is the receiver's named type. A receiver list the parser
// accepted empty lowers as a function, because there is no type to
// own it.
func (*goFrontend) lowerFunc(u *plugin.SourceUnit, l *lowered, file *node.File, d *ast.FuncDecl) {
	parts := l.declParts(u, plugin.CommentParts{}, d.Doc, nil)
	if d.Recv == nil || len(d.Recv.List) == 0 {
		fn := &node.Function{
			Name: d.Name.Name, Pos: l.at(d.Pos()), Doc: parts.Docs,
			Visibility:  visibilityOf(d.Name.Name),
			TypeParams:  l.typeParams(d.Type.TypeParams),
			Params:      l.params(d.Type.Params),
			Returns:     l.returns(d.Type.Results),
			Annotations: parts.Annotations,
		}
		file.Decls = append(file.Decls, fn)
		attachCarriers(u, u.Graph(), fn, parts.Carriers)
		stampIter(u, fn, fn.Returns)
		return
	}
	recv := d.Recv.List[0]
	m := &node.Method{
		Name: d.Name.Name, Pos: l.at(d.Pos()), Doc: parts.Docs,
		Visibility:  visibilityOf(d.Name.Name),
		Receives:    &node.TypeRef{Spelling: l.bareName(recv.Type), Pos: l.at(recv.Pos())},
		Receiver:    l.param(recv),
		TypeParams:  l.typeParams(d.Type.TypeParams),
		Params:      l.params(d.Type.Params),
		Returns:     l.returns(d.Type.Results),
		Annotations: parts.Annotations,
	}
	file.Decls = append(file.Decls, m)
	attachCarriers(u, u.Graph(), m, parts.Carriers)
	stampIter(u, m, m.Returns)
	if _, pointer := unparen(recv.Type).(*ast.StarExpr); pointer {
		u.Graph().Stamp(m, meta.RawStamp{
			Key: golang.ReceiverPointerKey, Value: true, Pos: m.Pos,
		})
	}
}

// stampIter marks a callable whose first result is one of the
// iterator shapes, read off the spelling: iter.Seq and iter.Seq2
// are their own announcement. An instantiation lowers to the bare
// name with its arguments split out, so the bare spelling is the
// whole comparison.
func stampIter(u *plugin.SourceUnit, callable symbol.Symbol, returns []*node.Return) {
	if len(returns) == 0 || returns[0].Type == nil {
		return
	}
	switch returns[0].Type.Spelling {
	case "iter.Seq2":
		u.Graph().Stamp(callable, meta.RawStamp{
			Key: golang.IterSeq2Key, Value: true, Pos: callable.Position(),
		})
	case "iter.Seq":
		u.Graph().Stamp(callable, meta.RawStamp{
			Key: golang.IterSeqKey, Value: true, Pos: callable.Position(),
		})
	}
}

// unparen unwraps the parentheses an expression may wear.
func unparen(e ast.Expr) ast.Expr {
	for {
		paren, wrapped := e.(*ast.ParenExpr)
		if !wrapped {
			return e
		}
		e = paren.X
	}
}

// lowerType lowers one type spec. The alias check comes before the
// shape switch: `type A = struct{...}` states a transparent alias
// of an inline shape, and lowering it as a defined struct would
// fabricate type identity the source refused.
func (f *goFrontend) lowerType(
	u *plugin.SourceUnit, l *lowered, file *node.File, group plugin.CommentParts, s *ast.TypeSpec,
) {
	parts := l.declParts(u, group, s.Doc, s.Comment)
	name, pos := s.Name.Name, l.at(s.Pos())
	vis := visibilityOf(name)
	tps := l.typeParams(s.TypeParams)

	var declared symbol.Symbol
	switch t := s.Type.(type) {
	case *ast.StructType:
		if s.Assign.IsValid() {
			declared = l.aliasOf(s, parts, name, pos, vis, tps)
			break
		}
		st := &node.Struct{
			Name: name, Pos: pos, Doc: parts.Docs, Visibility: vis,
			TypeParams: tps, Annotations: parts.Annotations,
		}
		f.lowerStructBody(u, l, st, t)
		declared = st
	case *ast.InterfaceType:
		if s.Assign.IsValid() {
			declared = l.aliasOf(s, parts, name, pos, vis, tps)
			break
		}
		it := &node.Interface{
			Name: name, Pos: pos, Doc: parts.Docs, Visibility: vis,
			TypeParams: tps, Annotations: parts.Annotations,
		}
		f.lowerInterfaceBody(u, l, it, t)
		declared = it
	default:
		alias := l.aliasOf(s, parts, name, pos, vis, tps)
		alias.Defined = !s.Assign.IsValid()
		if alias.Defined {
			l.underlyings = append(l.underlyings, pendingUnderlying{
				alias: alias, kind: underlyingKind(s.Type),
			})
		}
		declared = alias
	}
	file.Decls = append(file.Decls, declared)
	attachCarriers(u, u.Graph(), declared, parts.Carriers)
}

// aliasOf builds the alias shape a type spec lowers to when its
// target stays a spelling.
func (l *lowered) aliasOf(
	s *ast.TypeSpec, parts plugin.CommentParts, name string, pos position.Pos,
	vis symbol.Visibility, tps []*node.TypeParam,
) *node.Alias {
	return &node.Alias{
		Name: name, Pos: pos, Doc: parts.Docs, Visibility: vis,
		TypeParams: tps, Target: l.typeRef(s.Type), Annotations: parts.Annotations,
	}
}

// lowerStructBody lowers fields and embeds, unexported fields
// staying out at signature depth. A carrier on an embedded field
// refuses positioned: the model cannot address an embed.
func (*goFrontend) lowerStructBody(u *plugin.SourceUnit, l *lowered, st *node.Struct, t *ast.StructType) {
	for _, field := range t.Fields.List {
		parts := l.declParts(u, plugin.CommentParts{}, field.Doc, field.Comment)
		if len(field.Names) == 0 {
			st.Embeds = append(st.Embeds, &node.Embed{
				Ref: l.typeRef(field.Type), Pos: l.at(field.Pos()),
			})
			refuseCarriers(u, parts.Carriers, "an embedded field")
			continue
		}
		for _, name := range field.Names {
			if u.Depth() == plugin.DepthSignatures && !ast.IsExported(name.Name) {
				continue
			}
			lowered := &node.Field{
				Name: name.Name, Pos: l.at(name.Pos()), Doc: parts.Docs,
				Visibility:  visibilityOf(name.Name),
				Type:        l.typeRef(field.Type),
				Tag:         tagOf(field.Tag),
				Comment:     commentText(l, u, field.Comment),
				Annotations: parts.Annotations,
			}
			st.Fields = append(st.Fields, lowered)
			attachCarriers(u, u.Graph(), lowered, parts.Carriers)
		}
	}
}

// lowerInterfaceBody lowers method signatures and embedded
// interfaces. A constraint element — a union or an approximation —
// is not an embed: its verbatim spellings stamp onto the interface
// as its type set, the language metadata the model routes them to.
func (*goFrontend) lowerInterfaceBody(u *plugin.SourceUnit, l *lowered, it *node.Interface, t *ast.InterfaceType) {
	var terms []string
	for _, member := range t.Methods.List {
		parts := l.declParts(u, plugin.CommentParts{}, member.Doc, member.Comment)
		if len(member.Names) == 0 {
			if constraintElement(member.Type) {
				terms = append(terms, l.spelling(member.Type))
				refuseCarriers(u, parts.Carriers, "a constraint element")
				continue
			}
			it.Embeds = append(it.Embeds, &node.Embed{
				Ref: l.typeRef(member.Type), Pos: l.at(member.Pos()),
			})
			refuseCarriers(u, parts.Carriers, "an embedded interface")
			continue
		}
		sig, is := member.Type.(*ast.FuncType)
		if !is {
			continue
		}
		name := member.Names[0].Name
		if name == "_" {
			continue // a blank method binds nothing, as blank values do
		}
		if u.Depth() == plugin.DepthSignatures && !ast.IsExported(name) {
			continue
		}
		m := &node.Method{
			Name: name, Pos: l.at(member.Pos()), Doc: parts.Docs,
			Visibility:  visibilityOf(name),
			Abstract:    true,
			Params:      l.params(sig.Params),
			Returns:     l.returns(sig.Results),
			Annotations: parts.Annotations,
		}
		it.Methods = append(it.Methods, m)
		attachCarriers(u, u.Graph(), m, parts.Carriers)
	}
	if len(terms) > 0 {
		u.Graph().Stamp(it, meta.RawStamp{Key: golang.TypeSetKey, Value: terms, Pos: it.Pos})
		u.Graph().Stamp(it, meta.RawStamp{
			Key: golang.ConstraintInterfaceKey, Value: true, Pos: it.Pos,
		})
	}
	if len(it.Methods) == 0 && len(it.Embeds) == 0 && len(terms) == 0 {
		u.Graph().Stamp(it, meta.RawStamp{
			Key: golang.EmptyInterfaceKey, Value: true, Pos: it.Pos,
		})
	}
}

// constraintElement reports a type-set element no embed can carry:
// a union, an approximation, or a parenthesized nest of either.
func constraintElement(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.BinaryExpr, *ast.UnaryExpr:
		return true
	case *ast.ParenExpr:
		return constraintElement(t.X)
	default:
		return false
	}
}

// lowerValues lowers one const or var spec, a declaration per
// bound name; a blank name binds nothing and is skipped, as the
// old frontend skipped it. An implicit constant — an iota carrier
// with no expression of its own — keeps an empty value,
// unevaluated by contract; one call initializing several names
// carries its spelling to each.
func (*goFrontend) lowerValues(
	u *plugin.SourceUnit, l *lowered, file *node.File, group plugin.CommentParts,
	tok token.Token, s *ast.ValueSpec, specType ast.Expr,
) {
	parts := l.declParts(u, group, s.Doc, s.Comment)
	for i, name := range s.Names {
		if name.Name == "_" {
			continue
		}
		if u.Depth() == plugin.DepthSignatures && !ast.IsExported(name.Name) {
			continue
		}
		var value string
		switch {
		case i < len(s.Values):
			value = l.spelling(s.Values[i])
		case len(s.Values) == 1:
			value = l.spelling(s.Values[0])
		}
		var declared symbol.Symbol
		if tok == token.CONST {
			declared = &node.Constant{
				Name: name.Name, Pos: l.at(name.Pos()), Doc: parts.Docs,
				Visibility:  visibilityOf(name.Name),
				Type:        l.typeRef(specType),
				Value:       value,
				Comment:     commentText(l, u, s.Comment),
				Annotations: parts.Annotations,
			}
		} else {
			declared = &node.Variable{
				Name: name.Name, Pos: l.at(name.Pos()), Doc: parts.Docs,
				Visibility:  visibilityOf(name.Name),
				Mutability:  symbol.MutabilityMutable,
				Type:        l.typeRef(specType),
				Value:       value,
				Comment:     commentText(l, u, s.Comment),
				Annotations: parts.Annotations,
			}
		}
		file.Decls = append(file.Decls, declared)
		attachCarriers(u, u.Graph(), declared, parts.Carriers)
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

// params lowers a parameter list, one entry per bound name at its
// own position and one for an unnamed parameter. The parser
// attaches no comments to parameters, so a carrier beside one is
// the sweep's to refuse.
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
			p.Pos = l.at(name.Pos())
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

// returns lowers a result list; a blank result name normalizes to
// none.
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
			bound := name.Name
			if bound == "_" {
				bound = ""
			}
			out = append(out, &node.Return{
				Name: bound, Pos: l.at(name.Pos()), Type: l.typeRef(field.Type),
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

// commentText returns a trailing comment's documentation text,
// carriers and directives excluded, empty when nothing remains.
func commentText(l *lowered, u *plugin.SourceUnit, group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.Join(l.split(u, group).Docs, " ")
}

// filenameConstraint returns the GOOS and GOARCH tags a filename
// implies under the go tool's own suffix rules, after the _test
// suffix strips.
func filenameConstraint(base string) ([]string, bool) {
	name := strings.TrimSuffix(base, ".go")
	name = strings.TrimSuffix(name, "_test")
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return nil, false
	}
	last := parts[len(parts)-1]
	prev := parts[len(parts)-2]
	switch {
	case knownArch[last] && knownOS[prev]:
		return []string{prev, last}, true
	case knownArch[last] || knownOS[last]:
		return []string{last}, true
	}
	return nil, false
}

// The go tool's own suffix vocabularies, pinned here because no
// stdlib package exports them; a new port extends the lists.
var knownOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "hurd": true, "illumos": true, "ios": true,
	"js": true, "linux": true, "nacl": true, "netbsd": true,
	"openbsd": true, "plan9": true, "solaris": true, "wasip1": true,
	"windows": true, "zos": true,
}

var knownArch = map[string]bool{
	"386": true, "amd64": true, "amd64p32": true, "arm": true,
	"arm64": true, "arm64be": true, "armbe": true, "loong64": true,
	"mips": true, "mips64": true, "mips64le": true, "mips64p32": true,
	"mips64p32le": true, "mipsle": true, "ppc": true, "ppc64": true,
	"ppc64le": true, "riscv": true, "riscv64": true, "s390": true,
	"s390x": true, "sparc": true, "sparc64": true, "wasm": true,
}
