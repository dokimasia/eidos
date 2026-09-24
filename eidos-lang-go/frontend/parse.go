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
// promotion has decided whether an enum replaces the type.
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
		named:   map[string]bool{},
	}
	for _, ref := range u.Files() {
		if err := f.parseFile(u, root, st, ref.Path); err != nil {
			return err
		}
	}
	for _, pkgPath := range st.order {
		foldMethods(st.batches[pkgPath].files)
		stampConstValues(u, st.fset, st.batches[pkgPath])
	}
	stampModule(u, root)
	return nil
}

// stampModule stamps every package the unit declared with the
// kernel's neutral module identity: the module path and the
// directory of its go.mod. A directory outside every module stamps
// nothing, because absence is the negative.
func stampModule(u *plugin.SourceUnit, root moduleRoot) {
	if root.module == "" {
		return
	}
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		gb.Stamp(pkg, meta.RawStamp{Key: meta.ModuleKey, Value: root.module, Pos: pkg.Pos})
		gb.Stamp(pkg, meta.RawStamp{Key: meta.ModuleRootKey, Value: root.dir, Pos: pkg.Pos})
	}
}

// parseState is one unit's shared parse machinery: the file set
// every member joins, the spelling intern the lowerings share, the
// per-package batches the constant evaluation runs over once each,
// so a constant referencing a sibling file's type still evaluates,
// and the packages a file inside the build has named.
type parseState struct {
	fset    *token.FileSet
	intern  map[string]string
	batches map[string]*constBatch
	order   []string
	named   map[string]bool
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
	if parsed == nil || parsed.Name == nil || !parsed.Package.IsValid() {
		// A file without a package clause declares nothing: its
		// syntax errors are reported, and no position in it anchors
		// a declaration.
		return nil
	}
	l := &lowered{
		file: st.fset.File(parsed.Package), src: src,
		intern: st.intern, consumed: map[*ast.CommentGroup]bool{},
		comments: parsed.Comments,
	}
	gb := u.Graph()

	pkgPath := root.importPath(path.Dir(filePath))
	pkgName := parsed.Name.Name
	if strings.HasSuffix(pkgName, "_test") {
		pkgPath += "_test"
	}
	pkg := gb.Package(pkgPath)

	file := &node.File{Path: filePath, Pos: l.at(parsed.Package)}
	pkg.Files = append(pkg.Files, file)

	fileBinds := newBindings()
	cgo := false
	for _, spec := range parsed.Imports {
		lowerImport(u, l, file, fileBinds, spec)
		if imported, unquoteErr := strconv.Unquote(spec.Path.Value); unquoteErr == nil && imported == "C" {
			cgo = true
		}
	}
	gb.Scope(file, fileBinds)

	// Tool directives above the package clause are the file's
	// annotations.
	parts := l.split(u, parsed.Doc)
	file.Doc = parts.Docs
	file.Annotations = append(file.Annotations, parts.Annotations...)

	if cgo {
		gb.Stamp(file, meta.RawStamp{Key: golang.CgoKey, Value: true, Pos: file.Pos})
	}
	if marker, generated := generatedMarker(parsed); generated {
		gb.Stamp(file, meta.RawStamp{Key: golang.GeneratedKey, Value: marker, Pos: file.Pos})
	}
	if line, excluded := f.excluded(parsed, filePath); excluded {
		// A file outside the build speaks for no package: it names
		// one only where no file inside the build has, and its
		// package doc and carriers are left out.
		if pkg.Name == "" {
			pkg.Name = pkgName
		}
		gb.Stamp(file, meta.RawStamp{Key: golang.ConstraintKey, Value: line, Pos: file.Pos})
		return nil
	}
	namePackage(u, st, pkg, pkgPath, pkgName, file.Pos)
	// The package clause's doc belongs to the package: the text
	// hoists to the first non-empty doc across the unit's files,
	// and its carriers attach to the package, not the file.
	if len(pkg.Doc) == 0 {
		pkg.Doc = parts.Docs
	}
	attachCarriers(u, gb, pkg, parts.Carriers)
	for _, decl := range parsed.Decls {
		f.lowerDecl(u, l, file, decl)
	}
	promoted, moved := promoteEnums(file)
	for from, to := range moved {
		// A carrier or stamp recorded against a consumed alias or
		// constant follows the enum or variant that replaces it, so
		// the splice never meets a subject the promotion removed.
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

	// The sweep: a comment no declaration consumed, floating between
	// declarations or above the package clause, gives its tool
	// directives to the file and refuses its carriers positioned. Its
	// prose has no model home and drops. A comment inside a function
	// body belongs to the body's statements, which the model does not
	// contain, so the sweep passes over it.
	bodies := funcBodies(parsed)
	next := 0
	for _, group := range parsed.Comments {
		for next < len(bodies) && bodies[next].to < group.Pos() {
			next++
		}
		if l.consumed[group] || (next < len(bodies) && bodies[next].from < group.Pos()) {
			continue
		}
		floating := l.split(u, group)
		file.Annotations = append(file.Annotations, floating.Annotations...)
		refuseCarriers(u, floating.Carriers, "a comment no declaration owns")
	}
	return nil
}

// span is one function body's extent in a file.
type span struct{ from, to token.Pos }

// funcBodies returns the extents of a file's function bodies, a
// declared function's and a top-level function literal's, in source
// order. A literal inside a body is inside that body's extent, so no
// two extents nest.
func funcBodies(parsed *ast.File) []span {
	var out []span
	ast.Inspect(parsed, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil {
				out = append(out, span{from: fn.Body.Lbrace, to: fn.Body.Rbrace})
			}
			return false
		case *ast.FuncLit:
			out = append(out, span{from: fn.Body.Lbrace, to: fn.Body.Rbrace})
			return false
		}
		return true
	})
	return out
}

// namePackage gives a package the name a file inside the build
// declares. The first such file names it, over any name a file
// outside the build filled in, and a later file declaring another
// name reports under [MixedPackage] while the first name is kept.
func namePackage(
	u *plugin.SourceUnit, st *parseState, pkg *node.Package, pkgPath, name string, at position.Pos,
) {
	switch {
	case !st.named[pkgPath]:
		pkg.Name = name
		st.named[pkgPath] = true
	case pkg.Name != name:
		u.Warnf(MixedPackage, at, "package %s is declared %q and %q; the first is kept",
			pkgPath, pkg.Name, name)
	}
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
// set, and the constraint that excludes it: a go:build line read off
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

// satisfies reports whether the load's tag set contains a tag.
func (f *goFrontend) satisfies(tag string) bool {
	return slices.Contains(f.opts.Tags, tag)
}

// generatedMarker returns the Go convention's generated-file
// marker when one precedes the package clause.
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
// with its doc and trailing comment, and the binding Resolve
// reads. A dot import binds every exported name, recorded as a
// wildcard, and a blank import keeps its underscore as the alias,
// so a side-effect import round-trips as one; neither binds a
// qualifier. An unaliased import binds the name its path assumes,
// [golang.AssumedName], and joins the fallback probe, which settles
// a package whose clause declares another name. A carrier on an
// import refuses positioned, because no rule takes an import as its
// subject, and a tool directive there is the file's.
func lowerImport(u *plugin.SourceUnit, l *lowered, file *node.File, b *bindings, spec *ast.ImportSpec) {
	imported, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return
	}
	parts := l.declParts(u, plugin.CommentParts{}, spec.Doc, spec.Comment)
	refuseCarriers(u, parts.Carriers, "an import")
	file.Annotations = append(file.Annotations, parts.Annotations...)
	record := &node.Import{
		Path: imported, Pos: l.at(spec.Pos()),
		Doc: parts.Docs, Comment: commentText(l, u, spec.Comment),
	}
	switch {
	case spec.Name == nil:
		b.named[golang.AssumedName(imported)] = imported
		b.all = append(b.all, imported)
	case spec.Name.Name == golang.BlankAlias:
		record.Alias = golang.BlankAlias
	case spec.Name.Name == golang.DotAlias:
		record.Wildcard = true
		b.dots = append(b.dots, imported)
	default:
		record.Alias = spec.Name.Name
		b.named[spec.Name.Name] = imported
	}
	file.Imports = append(file.Imports, record)
}

// lowerDecl lowers one top-level declaration, observing the unit's
// depth: at signatures, an unexported declaration is left out, and
// its comments are consumed with it, because they belong to the
// declaration the load left out. A function no code can address is
// left out at every depth, and a carrier on it refuses. The doc of
// an import declaration gives its tool directives to the file and
// refuses its carriers, as an import's own doc does.
func (f *goFrontend) lowerDecl(u *plugin.SourceUnit, l *lowered, file *node.File, decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if unaddressable(d) {
			parts := l.declParts(u, plugin.CommentParts{}, d.Doc, l.trailing(d.End(), token.NoPos))
			refuseCarriers(u, parts.Carriers, "a function no code can name")
			return
		}
		if u.Depth() == plugin.DepthSignatures && !ast.IsExported(d.Name.Name) {
			l.skip(d.Doc, l.trailing(d.End(), token.NoPos))
			return
		}
		f.lowerFunc(u, l, file, d)
	case *ast.GenDecl:
		if d.Tok == token.IMPORT {
			parts := l.split(u, d.Doc)
			refuseCarriers(u, parts.Carriers, "an import")
			file.Annotations = append(file.Annotations, parts.Annotations...)
			return
		}
		group := l.split(u, d.Doc)
		// An empty const spec repeats the previous one, the type
		// included: Go's own inheritance rule, applied here so an
		// implicit iota row still knows what it is typed by.
		var inherited ast.Expr
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if u.Depth() == plugin.DepthSignatures && !ast.IsExported(s.Name.Name) {
					l.skip(s.Doc, s.Comment)
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

// unaddressable reports a function no Go code can name: a blank
// function or method, and a package's init, which the language runs
// and forbids referring to. A package declares any number of
// either, so lowering them would spell one identity twice.
func unaddressable(d *ast.FuncDecl) bool {
	return d.Name.Name == blankName || d.Recv == nil && d.Name.Name == initName
}

// The names Go reserves a meaning for at declaration: the blank
// identifier, which binds nothing, and the package initializer.
const (
	blankName = "_"
	initName  = "init"
)

// declParts joins a spec's own comments with its group's: the
// nearer doc text is kept, and carriers and annotations union
// across the group doc, the spec doc and the trailing comment,
// because a directive is authored intent wherever it is written.
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
	tail := l.trailing(d.End(), token.NoPos)
	parts := l.declParts(u, plugin.CommentParts{}, d.Doc, tail)
	if d.Recv == nil || len(d.Recv.List) == 0 {
		fn := &node.Function{
			Name: d.Name.Name, Pos: l.at(d.Pos()), Doc: parts.Docs,
			Comment:     commentText(l, u, tail),
			Visibility:  visibilityOf(d.Name.Name),
			TypeParams:  l.typeParams(d.Type.TypeParams),
			Params:      l.params(u, d.Type.Params),
			Returns:     l.returns(u, d.Type.Results),
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
		Comment:     commentText(l, u, tail),
		Visibility:  visibilityOf(d.Name.Name),
		Receives:    &node.TypeRef{Spelling: l.bareName(recv.Type), Pos: l.at(recv.Pos())},
		Receiver:    l.param(recv, l.memberComment(u, recv, d.Recv.Closing, "a receiver")),
		TypeParams:  l.typeParams(d.Type.TypeParams),
		Params:      l.params(u, d.Type.Params),
		Returns:     l.returns(u, d.Type.Results),
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
			declared = l.aliasOf(u, s, parts, name, pos, vis, tps)
			break
		}
		st := &node.Struct{
			Name: name, Pos: pos, Doc: parts.Docs, Visibility: vis,
			Comment:    commentText(l, u, s.Comment),
			TypeParams: tps, Annotations: parts.Annotations,
		}
		f.lowerStructBody(u, l, st, t)
		declared = st
	case *ast.InterfaceType:
		if s.Assign.IsValid() {
			declared = l.aliasOf(u, s, parts, name, pos, vis, tps)
			break
		}
		it := &node.Interface{
			Name: name, Pos: pos, Doc: parts.Docs, Visibility: vis,
			Comment:    commentText(l, u, s.Comment),
			TypeParams: tps, Annotations: parts.Annotations,
		}
		f.lowerInterfaceBody(u, l, it, t)
		declared = it
	default:
		alias := l.aliasOf(u, s, parts, name, pos, vis, tps)
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
// target remains a spelling.
func (l *lowered) aliasOf(
	u *plugin.SourceUnit, s *ast.TypeSpec, parts plugin.CommentParts, name string,
	pos position.Pos, vis symbol.Visibility, tps []*node.TypeParam,
) *node.Alias {
	return &node.Alias{
		Name: name, Pos: pos, Doc: parts.Docs, Visibility: vis,
		Comment:    commentText(l, u, s.Comment),
		TypeParams: tps, Target: l.typeRef(s.Type), Annotations: parts.Annotations,
	}
}

// lowerStructBody lowers fields and embeds, unexported fields left
// out at signature depth. An embedded field is a
// declaration like any field: its doc, tag, trailing comment and
// annotations lower with it, and its carriers attach to it. An
// embed whose named type the source spells nothing for, as one the
// parser synthesized past the end of a broken file, is left out,
// because the load names an embed by that spelling. A blank field
// is padding no code can address, and a struct may declare any
// number of them, so it is left out. A field of blank names alone
// leaves its comments unread, so a carrier on it refuses as
// floating.
func (*goFrontend) lowerStructBody(u *plugin.SourceUnit, l *lowered, st *node.Struct, t *ast.StructType) {
	for _, field := range t.Fields.List {
		if len(field.Names) > 0 && !slices.ContainsFunc(field.Names, named) {
			continue
		}
		parts := l.declParts(u, plugin.CommentParts{}, field.Doc, field.Comment)
		if len(field.Names) == 0 {
			if l.spelling(undecorated(field.Type)) == "" {
				continue
			}
			embed := &node.Embed{
				Ref: l.typeRef(field.Type), Pos: l.at(field.Pos()), Doc: parts.Docs,
				Comment:     commentText(l, u, field.Comment),
				Tag:         tagOf(field.Tag),
				Annotations: parts.Annotations,
			}
			st.Embeds = append(st.Embeds, embed)
			attachCarriers(u, u.Graph(), embed, parts.Carriers)
			continue
		}
		for _, name := range field.Names {
			if !named(name) {
				continue
			}
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

// named reports whether a declared name binds something: every name
// but the blank identifier.
func named(name *ast.Ident) bool { return name.Name != blankName }

// lowerInterfaceBody lowers method signatures and embedded
// interfaces. A constraint element — a union or an approximation —
// is not an embed: its verbatim spellings stamp onto the interface
// as its type set, the language metadata the model routes them to.
// An embed whose type spells no name is left out, as a struct's is.
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
			if l.spelling(undecorated(member.Type)) == "" {
				continue
			}
			embed := &node.Embed{
				Ref: l.typeRef(member.Type), Pos: l.at(member.Pos()), Doc: parts.Docs,
				Comment:     commentText(l, u, member.Comment),
				Annotations: parts.Annotations,
			}
			it.Embeds = append(it.Embeds, embed)
			attachCarriers(u, u.Graph(), embed, parts.Carriers)
			continue
		}
		sig, is := member.Type.(*ast.FuncType)
		if !is {
			continue
		}
		name := member.Names[0].Name
		if name == blankName {
			continue // a blank method binds nothing, as blank values do
		}
		if u.Depth() == plugin.DepthSignatures && !ast.IsExported(name) {
			continue
		}
		m := &node.Method{
			Name: name, Pos: l.at(member.Pos()), Doc: parts.Docs,
			Comment:     commentText(l, u, member.Comment),
			Visibility:  visibilityOf(name),
			Abstract:    true,
			Params:      l.params(u, sig.Params),
			Returns:     l.returns(u, sig.Results),
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

// constraintElement reports a type-set element no embed can
// represent: a union, an approximation, a predeclared basic type, a
// type literal other than an interface, or a parenthesized nest of
// any of them. A named type, qualified or instantiated, remains an
// embed, because only its declaration shows whether it is an
// interface.
func constraintElement(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.BinaryExpr, *ast.UnaryExpr:
		return true
	case *ast.ParenExpr:
		return constraintElement(t.X)
	case *ast.Ident:
		return golang.Basic(t.Name)
	case *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType,
		*ast.StructType, *ast.StarExpr:
		return true
	default:
		return false
	}
}

// lowerValues lowers one const or var spec, a declaration per
// bound name. A blank name binds nothing and is skipped. An implicit
// constant, a row with no expression of its own, keeps an empty
// value, unevaluated by contract, and one call initializing several
// names gives its spelling to each.
func (*goFrontend) lowerValues(
	u *plugin.SourceUnit, l *lowered, file *node.File, group plugin.CommentParts,
	tok token.Token, s *ast.ValueSpec, specType ast.Expr,
) {
	parts := l.declParts(u, group, s.Doc, s.Comment)
	for i, name := range s.Names {
		if !named(name) {
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

// typeParams lowers a generic parameter list, each bound as a
// reference.
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
// own position and one for an unnamed parameter. A comment the
// parser attaches to a parameter lowers as its trailing comment,
// read once per field and shared by every name the field binds, as
// a result's is. A carrier there refuses positioned, because no
// rule takes a parameter as its subject.
func (l *lowered) params(u *plugin.SourceUnit, fields *ast.FieldList) []*node.Param {
	if fields == nil {
		return nil
	}
	var out []*node.Param
	for _, field := range fields.List {
		comment := l.memberComment(u, field, fields.Closing, "a parameter")
		if len(field.Names) == 0 {
			out = append(out, l.param(field, comment))
			continue
		}
		for _, name := range field.Names {
			p := l.param(field, comment)
			p.Name = name.Name
			p.Pos = l.at(name.Pos())
			out = append(out, p)
		}
	}
	return out
}

// param lowers one field into a parameter with a trailing comment,
// variadic when its type is an ellipsis.
func (l *lowered) param(field *ast.Field, comment string) *node.Param {
	p := &node.Param{
		Pos:     l.at(field.Pos()),
		Comment: comment,
	}
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

// returns lowers a result list, blank names kept as written: Go
// admits a mixed list only fully named, so dropping the
// underscore would strand the siblings. A comment on a result
// lowers as its trailing comment, its carriers refused.
func (l *lowered) returns(u *plugin.SourceUnit, fields *ast.FieldList) []*node.Return {
	if fields == nil {
		return nil
	}
	var out []*node.Return
	for _, field := range fields.List {
		comment := l.memberComment(u, field, fields.Closing, "a result")
		if len(field.Names) == 0 {
			out = append(out, &node.Return{
				Pos: l.at(field.Pos()), Type: l.typeRef(field.Type), Comment: comment,
			})
			continue
		}
		for _, name := range field.Names {
			// The underscore is kept: a mixed list re-renders only
			// with every slot named, and go/parser rejects the
			// half-named form dropping it would produce.
			out = append(out, &node.Return{
				Name: name.Name, Pos: l.at(name.Pos()), Type: l.typeRef(field.Type),
				Comment: comment,
			})
		}
	}
	return out
}

// memberComment lowers a signature member's comments: the doc the
// parser attached, and the trailing comment on the member's own
// line inside its list, which the parser leaves free. The prose
// becomes the trailing comment, and a carrier refuses positioned,
// naming the member it is written on. An unparenthesized list
// bounds nothing and takes no trailing comment, because the line
// it ends on belongs to the declaration that follows.
func (l *lowered) memberComment(
	u *plugin.SourceUnit, field *ast.Field, closing token.Pos, what string,
) string {
	tail := field.Comment
	if tail == nil && closing.IsValid() {
		tail = l.trailing(field.End(), closing)
	}
	parts := l.declParts(u, plugin.CommentParts{}, field.Doc, tail)
	refuseCarriers(u, parts.Carriers, what)
	return strings.Join(l.split(u, tail).Docs, " ")
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
// implies under the go tool's own rule: the name is cut at its first
// dot, everything before its first underscore is ignored, and a
// final test element drops. The last two elements then name an OS
// and an architecture, or the last names either. linux_amd64.go
// implies amd64 alone, and x_windows.pb.go implies windows.
func filenameConstraint(base string) ([]string, bool) {
	name, _, _ := strings.Cut(base, ".")
	underscore := strings.Index(name, "_")
	if underscore < 0 {
		return nil, false
	}
	parts := strings.Split(name[underscore:], "_")
	if n := len(parts); n > 0 && parts[n-1] == testElement {
		parts = parts[:n-1]
	}
	n := len(parts)
	switch {
	case n >= 2 && knownOS[parts[n-2]] && knownArch[parts[n-1]]:
		return []string{parts[n-2], parts[n-1]}, true
	case n >= 1 && (knownOS[parts[n-1]] || knownArch[parts[n-1]]):
		return []string{parts[n-1]}, true
	}
	return nil, false
}

// testElement is the filename element the go tool reads as a test
// file's mark.
const testElement = "test"

// The go tool's own suffix vocabularies, pinned here because no
// stdlib package exports them. A new port extends the lists.
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
