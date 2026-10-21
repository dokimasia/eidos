// Copyright Dokimasia B.V. 2026
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

// parseMode is the mode of the parse of every member. The parse keeps the
// comments, which contain the documentation and the directives. It skips
// the deprecated identifier resolution of go/parser, which the lowering
// does not read. It reports every syntax error, because each error is a
// finding.
const parseMode = parser.ParseComments | parser.SkipObjectResolution | parser.AllErrors

// cgoImport is the import path of the pseudo-package of cgo. Its import
// marks a file that uses cgo and refers to no package.
const cgoImport = "C"

// pendingUnderlying is the shape of one defined type. The parse stamps it
// after the promotion decides whether an enum replaces the type.
type pendingUnderlying struct {
	alias *node.Alias
	kind  string
}

// parse loads one unit. It parses each member with go/parser and lowers
// it into the builder of the unit. It reports every syntax error at its
// position and loads every declaration that the parser recovered, so one
// bad token does not erase a file. A file whose build constraint excludes
// it from the tag set of the load contributes its file node, its imports
// and a golang.constraint stamp, and no declarations. The constraint is
// a go:build line or the suffixes of the file name. Only the package
// clause and the imports of such a file parse, as the go command reads
// it, so a syntax error after its imports reports nothing. Every package
// of the unit has the module of its go.mod.
func (f *goFrontend) parse(_ context.Context, u *plugin.SourceUnit) error {
	place, err := placeUnit(u)
	if err != nil {
		return err
	}
	st := &parseState{
		fset:    token.NewFileSet(),
		intern:  map[string]string{},
		batches: map[string]*constBatch{},
		named:   map[string]bool{},
		place:   place,
	}
	for _, ref := range u.Files() {
		if err := f.parseFile(u, st, ref.Path); err != nil {
			return err
		}
	}
	gb := u.Graph()
	for _, pkgPath := range st.order {
		b := st.batches[pkgPath]
		promoted, moved := promoteEnums(b.files)
		for from, to := range moved {
			// A carrier or a stamp on an alias or a constant that the
			// promotion replaced moves to the enum or the variant that
			// replaces it, so the splice meets no subject that the
			// promotion removed.
			gb.Rehome(from, to)
		}
		for _, pending := range b.underlyings {
			var subject symbol.Symbol = pending.alias
			if enum, replaced := promoted[pending.alias]; replaced {
				subject = enum
			}
			gb.Stamp(subject, meta.RawStamp{
				Key: golang.UnderlyingKindKey, Value: pending.kind, Pos: subject.Position(),
			})
		}
		foldMethods(b.files)
		stampConstValues(u, st.fset, b)
	}
	for _, pkg := range gb.Packages() {
		gb.StampModule(pkg, place.root.module, place.root.dir)
	}
	return nil
}

// parseState is the parse state that the members of one unit share. It
// has the file set of the members, the spelling intern of the lowerings
// and one batch for each package. The constant evaluation runs once over
// each batch, so a constant that refers to a type of a sibling file
// evaluates. It also records the packages that a file inside the build
// named, and the place where the package directory of the unit loads.
type parseState struct {
	fset    *token.FileSet
	intern  map[string]string
	batches map[string]*constBatch
	order   []string
	named   map[string]bool
	place   unitPlace
}

// constBatch has the parsed and the lowered files of one package inside
// the build, and the underlying shapes of its defined types. The parse
// stamps them after the promotion of the package has replaced the types
// that promote.
type constBatch struct {
	name        string
	parsed      []*ast.File
	files       []*node.File
	underlyings []pendingUnderlying
}

// batch returns the batch of a package, and creates it on the first call
// for the package.
func (st *parseState) batch(pkgPath, name string) *constBatch {
	if b, held := st.batches[pkgPath]; held {
		return b
	}
	b := &constBatch{name: name}
	st.batches[pkgPath] = b
	st.order = append(st.order, pkgPath)
	return b
}

// parseFile lowers one member. It parses the package clause and the
// imports first, and a file that its constraint excludes lowers from them
// alone. Every other file then parses whole.
func (f *goFrontend) parseFile(u *plugin.SourceUnit, st *parseState, filePath string) error {
	src, err := u.Read(filePath)
	if err != nil {
		return err
	}

	parsed, err := parser.ParseFile(st.fset, filePath, src, parseMode|parser.ImportsOnly)
	var built constraint.Expr
	excluded := false
	if parsed != nil && parsed.Name != nil && parsed.Package.IsValid() {
		built = constraintOf(parsed, filePath)
		excluded = built != nil && !built.Eval(f.satisfies)
		if !excluded {
			parsed, err = parser.ParseFile(st.fset, filePath, src, parseMode)
		}
	}
	if err != nil {
		reportSyntax(u, filePath, err)
	}
	if parsed == nil || parsed.Name == nil || !parsed.Package.IsValid() {
		// A file without a package clause declares nothing. The parse
		// reports its syntax errors, and no declaration has a position in
		// it.
		return nil
	}
	l := &lowered{
		file: st.fset.File(parsed.Package), src: src,
		intern: st.intern, consumed: map[*ast.CommentGroup]bool{},
		comments: parsed.Comments, std: st.place.std,
	}
	gb := u.Graph()

	pkgPath := st.place.importPath
	pkgName := parsed.Name.Name
	if strings.HasSuffix(pkgName, golang.TestSuffix) {
		pkgPath += golang.TestSuffix
	}
	pkg := gb.Package(pkgPath)

	file := &node.File{Path: filePath, Pos: l.at(parsed.Package)}
	pkg.Files = append(pkg.Files, file)

	cgo := false
	for _, spec := range parsed.Imports {
		lowerImport(u, l, file, spec)
		if imported, unquoteErr := strconv.Unquote(spec.Path.Value); unquoteErr == nil && imported == cgoImport {
			cgo = true
		}
	}
	l.bindings = golang.NewBindings(file.Imports)
	gb.Scope(file, l.bindings)

	// The build constraint of the file is its first annotation, and the tool
	// directives above the package clause follow it.
	parts := l.split(u, parsed.Doc)
	file.Doc = parts.Docs
	if built != nil {
		constrained := symbol.Annotation{Name: golang.BuildAnnotation, Args: []string{built.String()}}
		file.Annotations = append(file.Annotations, constrained)
	}
	file.Annotations = append(file.Annotations, parts.Annotations...)

	if cgo {
		gb.Stamp(file, meta.RawStamp{Key: golang.CgoKey, Value: true, Pos: file.Pos})
	}
	if marker, generated := generatedMarker(parsed); generated {
		gb.Stamp(file, meta.RawStamp{Key: golang.GeneratedKey, Value: marker, Pos: file.Pos})
	}
	if excluded {
		// A file outside the build names its package only where no file
		// inside the build did, and the parse leaves out its package doc
		// and its carriers.
		if pkg.Name == "" {
			pkg.Name = pkgName
		}
		gb.Stamp(file, meta.RawStamp{Key: golang.ConstraintKey, Value: built.String(), Pos: file.Pos})
		if u.Depth() == plugin.DepthSignatures {
			pruneImports(file, nil)
		}
		return nil
	}
	namePackage(u, st, pkg, pkgPath, pkgName, file.Pos)
	// The doc of the package clause is the doc of the package. The package
	// takes the first doc that is not empty among the files of the unit,
	// and the carriers attach to the package and not to the file.
	if len(pkg.Doc) == 0 {
		pkg.Doc = parts.Docs
	}
	attached := len(gb.Attachments())
	u.AttachCarriers(pkg, parts.Carriers, BadCarrier)
	for _, decl := range parsed.Decls {
		f.lowerDecl(u, l, file, decl)
	}
	if u.Depth() == plugin.DepthSignatures {
		used := map[string]bool{}
		recordUses(u, l, pkgPath, file, gb.Attachments()[attached:], used)
		pruneImports(file, used)
	} else {
		recordUses(u, l, pkgPath, file, gb.Attachments()[attached:], nil)
	}
	batch := st.batch(pkgPath, pkgName)
	batch.parsed = append(batch.parsed, parsed)
	batch.files = append(batch.files, file)
	batch.underlyings = append(batch.underlyings, l.underlyings...)

	// The sweep reads each comment that no declaration took, between
	// declarations or above the package clause. Its tool directives are
	// annotations of the file, and its carriers are refused at their
	// positions. The model has no place for its prose, so the prose is
	// dropped. A comment inside a function body belongs to the statements
	// of the body, which the model does not contain, so the sweep passes
	// over it.
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
		u.RefuseCarriers(UnaddressedCarrier, floating.Carriers, "a comment that no declaration takes")
	}
	return nil
}

// span is the extent of one function body in a file.
type span struct{ from, to token.Pos }

// funcBodies returns the extents of the function bodies of a file in
// source order. A body belongs to a declared function or to a top-level
// function literal. A literal inside a body is inside the extent of that
// body, so no two extents nest.
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

// namePackage gives a package the name that a file inside the build
// declares. The first such file names the package, over a name that a file
// outside the build set. A later file that declares another name reports
// under [MixedPackage], and the package keeps the first name.
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

// reportSyntax reports the syntax errors of a member through the syntax
// report of the unit. An error without a list of syntax errors reports at
// the start of the file.
func reportSyntax(u *plugin.SourceUnit, filePath string, err error) {
	var list scanner.ErrorList
	if !errors.As(err, &list) || len(list) == 0 {
		u.Errorf(UnparsedFile, position.Pos{File: filePath, Line: 1, Col: 1}, "%v", err)
		return
	}
	u.ReportSyntax(UnparsedFile, func(yield func(position.Pos, string) bool) {
		for _, e := range list {
			if !yield(position.Pos{File: filePath, Line: e.Pos.Line, Col: e.Pos.Column}, e.Msg) {
				return
			}
		}
	})
}

// constraintOf returns the build constraint of a file as one expression,
// and nil for a file without one. The expression is the conjunction of
// the //go:build lines among the parsed comments before the package
// clause, and of the GOOS and GOARCH tags that the name of the file
// implies. A //go:build spelling inside a block comment is not a
// constraint, so constraintOf reads the parsed comments and not the raw
// source. A line that does not parse adds nothing.
//
// A file without a //go:build line reads its +build lines as go/build
// does. Such a line constrains the file where it is in a comment group
// before the package clause and outside the doc of the package. Every
// such line must hold, and constraint.Parse reads a tag that is not valid
// as the tag ignore, which no load sets.
func constraintOf(parsed *ast.File, filePath string) constraint.Expr {
	var build, plus constraint.Expr
	stated := false
	for _, group := range parsed.Comments {
		if group.Pos() >= parsed.Package {
			break
		}
		for _, c := range group.List {
			switch {
			case !strings.HasPrefix(c.Text, "//"):
				continue
			case constraint.IsGoBuild(c.Text):
				stated = true
				if expr, err := constraint.Parse(c.Text); err == nil {
					build = conjoined(build, expr)
				}
			case constraint.IsPlusBuild(c.Text) && group != parsed.Doc:
				if expr, err := constraint.Parse(c.Text); err == nil {
					plus = conjoined(plus, expr)
				}
			}
		}
	}
	if !stated {
		build = plus
	}
	if implied, constrained := filenameConstraint(path.Base(filePath)); constrained {
		for _, tag := range implied {
			build = conjoined(build, &constraint.TagExpr{Tag: tag})
		}
	}
	return build
}

// conjoined returns the conjunction of x and y, and y alone where x is nil.
func conjoined(x, y constraint.Expr) constraint.Expr {
	if x == nil {
		return y
	}
	return &constraint.AndExpr{X: x, Y: y}
}

// satisfies reports whether the tag set of the load contains a tag.
func (f *goFrontend) satisfies(tag string) bool {
	return slices.Contains(f.opts.Tags, tag)
}

// generatedMarker returns the marker of a generated file by the convention
// of Go, where such a marker is before the package clause.
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

// lowerImport records one import of the file, with its doc and its
// trailing comment. A dot import is a wildcard. Every other alias is
// recorded as written, so a blank import keeps its underscore and renders
// again as a side-effect import. [golang.NewBindings] derives the
// bindings of the file from these records. No rule takes an import as its subject, so a carrier
// on an import is refused at its position. A tool directive on an import
// is an annotation of the file. A file of the standard library imports a
// package outside the standard library only from the modules that the
// standard library vendors. The go command reads such a package from
// vendor/, so the record has the vendored path.
func lowerImport(u *plugin.SourceUnit, l *lowered, file *node.File, spec *ast.ImportSpec) {
	imported, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return
	}
	if l.std && !golang.Standard(imported) {
		imported = vendorDir + "/" + imported
	}
	parts := l.declParts(u, plugin.CommentParts{}, spec.Doc, spec.Comment)
	u.RefuseCarriers(UnaddressedCarrier, parts.Carriers, "an import")
	file.Annotations = append(file.Annotations, parts.Annotations...)
	record := &node.Import{
		Path: imported, Pos: l.at(spec.Pos()),
		Doc: parts.Docs, Comment: commentText(l, u, spec.Comment),
	}
	switch {
	case spec.Name == nil:
	case spec.Name.Name == golang.DotAlias:
		record.Wildcard = true
	default:
		record.Alias = spec.Name.Name
	}
	file.Imports = append(file.Imports, record)
}

// lowerDecl lowers one top-level declaration at the depth of the unit. At
// the signature depth it leaves out an unexported declaration and marks
// its comments as taken, because they belong to the declaration that the
// load left out. It leaves out a function that no code can name at every
// depth, and refuses the carriers on it. The tool directives in the doc of
// an import declaration are annotations of the file, and its carriers are
// refused, as on the doc of an import.
func (f *goFrontend) lowerDecl(u *plugin.SourceUnit, l *lowered, file *node.File, decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if unaddressable(d) {
			parts := l.declParts(u, plugin.CommentParts{}, d.Doc, l.trailing(d.End(), token.NoPos))
			u.RefuseCarriers(UnaddressedCarrier, parts.Carriers, "a function that no code can name")
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
			u.RefuseCarriers(UnaddressedCarrier, parts.Carriers, "an import")
			file.Annotations = append(file.Annotations, parts.Annotations...)
			return
		}
		group := l.split(u, d.Doc)
		// An empty const spec repeats the previous spec with its type, by
		// the rule of Go, so an implicit iota row has the type of the row
		// that it repeats.
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

// unaddressable reports whether no Go code can name a function. Such a
// function is a blank function or method, or the init function of a
// package, which the language runs and forbids to name. A package declares
// any number of either, so their lowering would produce one identity
// twice.
func unaddressable(d *ast.FuncDecl) bool {
	return d.Name.Name == blankName || d.Recv == nil && d.Name.Name == initName
}

// Go reserves two names in a declaration. The blank identifier binds
// nothing, and init names the initializer of a package.
const (
	blankName = "_"
	initName  = "init"
)

// declParts joins the comments of a spec with the comments of its group.
// The spec keeps the nearer doc text. Its carriers and annotations are
// those of the group doc, the spec doc and the trailing comment together,
// because a directive applies wherever the author wrote it.
func (l *lowered) declParts(
	u *plugin.SourceUnit, group plugin.CommentParts, doc, trailing *ast.CommentGroup,
) plugin.CommentParts {
	parts := merge(l.split(u, doc), group)
	tail := l.split(u, trailing)
	parts.Carriers = append(parts.Carriers, tail.Carriers...)
	parts.Annotations = append(parts.Annotations, tail.Annotations...)
	return parts
}

// lowerFunc lowers a function, or a method where the declaration has a
// receiver. The named type of the receiver is the owner of the method. An
// empty receiver list, which the parser accepts, lowers as a function,
// because the method has no owner type.
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
		u.AttachCarriers(fn, parts.Carriers, BadCarrier)
		stampIter(u, fn, fn.Returns)
		return
	}
	recv := d.Recv.List[0]
	// A receiver has no identity, so no rule takes it as its subject.
	recvComment, recvCarriers := l.memberComment(u, recv, d.Recv.Opening, d.Recv.Closing)
	u.RefuseCarriers(UnaddressedCarrier, recvCarriers, "a receiver")
	m := &node.Method{
		Name: d.Name.Name, Pos: l.at(d.Pos()), Doc: parts.Docs,
		Comment:     commentText(l, u, tail),
		Visibility:  visibilityOf(d.Name.Name),
		Receives:    &node.TypeRef{Spelling: l.bareName(recv.Type), Pos: l.at(recv.Pos())},
		Receiver:    l.param(recv, recvComment),
		TypeParams:  l.typeParams(d.Type.TypeParams),
		Params:      l.params(u, d.Type.Params),
		Returns:     l.returns(u, d.Type.Results),
		Annotations: parts.Annotations,
	}
	file.Decls = append(file.Decls, m)
	u.AttachCarriers(m, parts.Carriers, BadCarrier)
	stampIter(u, m, m.Returns)
	if _, pointer := unparen(recv.Type).(*ast.StarExpr); pointer {
		u.Graph().Stamp(m, meta.RawStamp{
			Key: golang.ReceiverIsPointerKey, Value: true, Pos: m.Pos,
		})
	}
}

// stampIter marks a callable whose first result is iter.Seq or iter.Seq2.
// It compares the package that the reference imports and the unqualified
// name, so any alias of the import matches. An instantiation lowers to its
// bare name with its arguments apart, so the comparison reads the bare
// name alone.
func stampIter(u *plugin.SourceUnit, callable symbol.Symbol, returns []*node.Return) {
	if len(returns) == 0 || returns[0].Type == nil || returns[0].Type.Package != golang.IterPackage {
		return
	}
	switch golang.Unqualified(returns[0].Type.Spelling) {
	case golang.IterSeq2:
		u.Graph().Stamp(callable, meta.RawStamp{
			Key: golang.IterSeq2Key, Value: true, Pos: callable.Position(),
		})
	case golang.IterSeq:
		u.Graph().Stamp(callable, meta.RawStamp{
			Key: golang.IterSeqKey, Value: true, Pos: callable.Position(),
		})
	}
}

// unparen returns an expression without its parentheses.
func unparen(e ast.Expr) ast.Expr {
	for {
		paren, wrapped := e.(*ast.ParenExpr)
		if !wrapped {
			return e
		}
		e = paren.X
	}
}

// lowerType lowers one type spec. It checks for an alias before the shape.
// `type A = struct{...}` is a transparent alias of an inline shape, and a
// defined struct would give it a type identity that the source does not
// declare.
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
	u.AttachCarriers(declared, parts.Carriers, BadCarrier)
}

// aliasOf returns the alias that a type spec lowers to where its target is
// a spelling.
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

// lowerStructBody lowers the fields and the embeds of a struct body. At
// the signature depth it leaves out the unexported fields. An embedded
// field is a declaration like any field. Its doc, tag, trailing comment
// and annotations lower with it, and its carriers attach to it. The
// lowering leaves out an embed whose named type has no spelling in the
// source, such as one that the parser synthesized past the end of a broken
// file, because the load names an embed by that spelling. It leaves out a
// blank field, which is padding that no code can name, because a struct
// can declare any number of them. Nothing reads the comments of a field of
// blank names alone, so the sweep refuses a carrier on it. The lowering
// stamps golang.hiddenFields on a struct where it leaves out a blank or an
// unexported field, because Go compares a struct through the type of each
// field.
func (*goFrontend) lowerStructBody(u *plugin.SourceUnit, l *lowered, st *node.Struct, t *ast.StructType) {
	hidden := false
	for _, field := range t.Fields.List {
		if len(field.Names) > 0 && !slices.ContainsFunc(field.Names, named) {
			hidden = true
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
			u.AttachCarriers(embed, parts.Carriers, BadCarrier)
			continue
		}
		for _, name := range field.Names {
			if !named(name) || u.Depth() == plugin.DepthSignatures && !ast.IsExported(name.Name) {
				hidden = true
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
			u.AttachCarriers(lowered, parts.Carriers, BadCarrier)
		}
	}
	if hidden {
		u.Graph().Stamp(st, meta.RawStamp{Key: golang.HiddenFieldsKey, Value: true, Pos: st.Pos})
	}
}

// named reports whether a declared name is not the blank identifier, which
// binds nothing.
func named(name *ast.Ident) bool { return name.Name != blankName }

// lowerInterfaceBody lowers the method signatures and the embedded
// interfaces of an interface body. A constraint element, such as a union
// or an approximation, is not an embed. A stamp of the language records
// the verbatim spellings of the constraint elements as the type set of the
// interface. The lowering leaves out an embed whose type has no name, as
// it does in a struct.
func (*goFrontend) lowerInterfaceBody(u *plugin.SourceUnit, l *lowered, it *node.Interface, t *ast.InterfaceType) {
	var terms []string
	for _, member := range t.Methods.List {
		parts := l.declParts(u, plugin.CommentParts{}, member.Doc, member.Comment)
		if len(member.Names) == 0 {
			if constraintElement(member.Type) {
				terms = append(terms, l.spelling(member.Type))
				u.RefuseCarriers(UnaddressedCarrier, parts.Carriers, "a constraint element")
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
			u.AttachCarriers(embed, parts.Carriers, BadCarrier)
			continue
		}
		sig, is := member.Type.(*ast.FuncType)
		if !is {
			continue
		}
		name := member.Names[0].Name
		if name == blankName {
			continue // a blank method binds nothing, as a blank value does
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
		u.AttachCarriers(m, parts.Carriers, BadCarrier)
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

// constraintElement reports whether a type-set element is one that no
// embed can represent. Such an element is a union, an approximation, a
// predeclared basic type, a type literal other than an interface, or any
// of them in parentheses. A named type, qualified or instantiated, is an
// embed, because only its declaration shows whether it is an interface.
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

// lowerValues lowers one const or var spec to one declaration for each
// bound name. It skips a blank name, which binds nothing. An implicit
// constant is a row without an expression of its own, and it keeps an
// empty value, which the contract leaves unevaluated. One call that
// initializes several names gives its spelling to each.
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
		u.AttachCarriers(declared, parts.Carriers, BadCarrier)
	}
}

// typeParams lowers the type parameter list of a generic declaration, with
// the bound of each parameter as a reference.
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

// params lowers a parameter list, with one entry for each bound name at
// its own position and one for an unnamed parameter. A comment that the
// parser attaches to a parameter lowers as its trailing comment. The
// lowering reads it once for each field, and every name of the field
// shares it and its carriers, as for a result.
func (l *lowered) params(u *plugin.SourceUnit, fields *ast.FieldList) []*node.Param {
	if fields == nil {
		return nil
	}
	var out []*node.Param
	after := fields.Opening
	for _, field := range fields.List {
		comment, carriers := l.memberComment(u, field, after, fields.Closing)
		after = field.End()
		start := len(out)
		out = l.appendParams(out, field, comment)
		for _, p := range out[start:] {
			u.AttachCarriers(p, carriers, BadCarrier)
		}
	}
	return out
}

// appendParams appends the parameters of one field to out, each with the
// trailing comment. It appends one parameter for each bound name at its
// own position, and one for an unnamed parameter.
func (l *lowered) appendParams(out []*node.Param, field *ast.Field, comment string) []*node.Param {
	if len(field.Names) == 0 {
		return append(out, l.param(field, comment))
	}
	for _, name := range field.Names {
		p := l.param(field, comment)
		p.Name = name.Name
		p.Pos = l.at(name.Pos())
		out = append(out, p)
	}
	return out
}

// param lowers one field to a parameter with a trailing comment. The
// parameter is variadic where its type is an ellipsis.
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

// returns lowers a result list and keeps blank names as written. Go admits
// a list of named results only where every result has a name, so a
// dropped underscore would leave the list invalid. A comment on a result
// lowers as its trailing comment, and its carriers attach to each result
// of the field.
func (l *lowered) returns(u *plugin.SourceUnit, fields *ast.FieldList) []*node.Return {
	if fields == nil {
		return nil
	}
	var out []*node.Return
	after := fields.Opening
	for _, field := range fields.List {
		comment, carriers := l.memberComment(u, field, after, fields.Closing)
		after = field.End()
		start := len(out)
		out = l.appendReturns(out, field, comment)
		for _, r := range out[start:] {
			u.AttachCarriers(r, carriers, BadCarrier)
		}
	}
	return out
}

// appendReturns appends the results of one field to out, each with the
// trailing comment. It appends one result for each bound name at its own
// position, and one for an unnamed result.
func (l *lowered) appendReturns(out []*node.Return, field *ast.Field, comment string) []*node.Return {
	if len(field.Names) == 0 {
		return append(out, &node.Return{Pos: l.at(field.Pos()), Type: l.typeRef(field.Type), Comment: comment})
	}
	for _, name := range field.Names {
		// The result keeps its underscore. A list renders again only where
		// every result has a name, and go/parser rejects a list where some
		// results have none.
		out = append(out, &node.Return{
			Name: name.Name, Pos: l.at(name.Pos()), Type: l.typeRef(field.Type), Comment: comment,
		})
	}
	return out
}

// memberComment lowers the comments of a signature member. They are the
// leading comment on the line above the member, after the member before or
// the opening of its list at after, and the trailing comment on the line
// of the member inside its list. The parser attaches neither to the
// member. memberComment returns the text of the trailing comment and the
// carriers of both comments. The text of the leading comment has no place
// in the model. A list without parentheses takes no comment of its own,
// because its lines belong to the declarations around it.
func (l *lowered) memberComment(
	u *plugin.SourceUnit, field *ast.Field, after, closing token.Pos,
) (string, []plugin.Carrier) {
	doc := field.Doc
	if doc == nil {
		doc = l.leading(after, field.Pos())
	}
	tail := field.Comment
	if tail == nil && closing.IsValid() {
		tail = l.trailing(field.End(), closing)
	}
	parts := l.declParts(u, plugin.CommentParts{}, doc, tail)
	return strings.Join(l.split(u, tail).Docs, " "), parts.Carriers
}

// visibilityOf returns public for an exported name and package for every
// other name, because Go decides visibility by the case of a name.
func visibilityOf(name string) symbol.Visibility {
	if ast.IsExported(name) {
		return symbol.VisibilityPublic
	}
	return symbol.VisibilityPackage
}

// tagOf returns the text of a struct tag without its delimiters. It
// unquotes the literal in either quoting and does not evaluate the text.
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

// commentText returns the documentation text of a trailing comment without
// its carriers and directives. It returns the empty string where no text
// is left.
func commentText(l *lowered, u *plugin.SourceUnit, group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.Join(l.split(u, group).Docs, " ")
}

// filenameConstraint returns the GOOS and GOARCH tags that a file name
// implies by the rule of the go command. The rule cuts the name at its
// first dot, ignores everything before its first underscore and drops a
// final test element. The last two elements are then an operating system
// and an architecture, or the last element is either. linux_amd64.go
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

// testElement is the element of a file name that marks a test file for
// the go command.
const testElement = "test"

// knownOS and knownArch are the operating systems and the architectures
// that the go command knows. The standard library exports neither list,
// so the frontend declares both. A new port extends the lists.
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
