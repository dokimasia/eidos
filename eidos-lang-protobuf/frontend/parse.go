// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/bufbuild/protocompile/experimental/ast"
	"github.com/bufbuild/protocompile/experimental/report"
	"github.com/bufbuild/protocompile/experimental/token"
	"github.com/bufbuild/protocompile/experimental/token/keyword"

	"go.dokimi.dev/eidos/lang/protobuf"
	protoparse "go.dokimi.dev/eidos/lang/protobuf/internal/parse"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of proto that the lowering reads.
const (
	// listSep joins the entries of the reserved stamp, and the ranges of
	// one extension range declaration.
	listSep = ", "
	// streamRequest, streamResponse and streamBoth are the values that the
	// stream stamp records for the side of an rpc that streams.
	streamRequest  = "request"
	streamResponse = "response"
	streamBoth     = "both"
	// streamKeyword marks a streaming side of an rpc.
	streamKeyword = "stream"
	// bodyValue is the value of the body stamp of an rpc with a body.
	bodyValue = "{}"
	// requestParam is the name of the one parameter that an rpc takes.
	requestParam = "request"
	// mapSpelling is the value of the map-entry stamp of a field that the
	// schema wrote as a map. The model projects such a field as the Map
	// form over its key and its value.
	mapSpelling = "map"
	// labelRepeated and labelOptional are the field labels that the
	// projection has a form for. A repeated field is a list, and an
	// optional field has presence.
	labelRepeated = "repeated"
	labelOptional = "optional"
	// labelRequired is the required label of proto2. The projection has
	// no form for it, so the frontend stamps it.
	labelRequired = "required"
	// importPublic, importWeak and importOption are the modifiers of an
	// import.
	importPublic = "public"
	importWeak   = "weak"
	importOption = "option"
	// localValue is the value of the marker of a message or an enum that
	// another file cannot reference.
	localValue = "local"
)

// unattributed describes a comment that protoc attributes to no
// declaration, in the refusal of a carrier in it.
const unattributed = "a comment that protoc attributes to no declaration"

// parse loads one proto file. It parses the syntax with the experimental
// parser of protocompile, reads the version from the syntax or edition
// statement of the file, lowers the tree into the node model and stamps
// the residue.
//
// parse reports a syntax error under [UnparsedFile] at its position, and
// the file still contributes every declaration that the parser recovered.
// parse reports a version outside the table of the frontend under
// [UnknownEdition], and the file does not contribute any declaration,
// because the defaults that define presence and the openness of enums are
// unknown.
//
// The depth does not change the result. A schema has no bodies and no
// unexported names, so [plugin.DepthSignatures] and [plugin.DepthFull]
// load the same declarations under the same identities.
//
// Error modes: parse returns the error of a context that is done, and the
// error of a member that does not read.
func parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref := u.Files()[0]
	src, err := u.Read(ref.Path)
	if err != nil {
		return err
	}
	tree, r, statement, v, known := parseSource(ref.Path, src)
	l := &lowered{unit: u, tree: tree, stream: tree.Stream(), path: ref.Path, version: v}
	if !known {
		u.Errorf(UnknownEdition, l.at(statement.Span()),
			"%s %s is outside the frontend's table of protobuf versions, so the frontend does not load the file",
			statement.KeywordToken().Text(), statement.Value().Span().Text())
		return nil
	}
	l.reportErrors(r)
	l.attribute()
	l.file(statement)
	return nil
}

// parseSource parses the source of a file through [protoparse.File]. It returns
// the tree, the report of the parser, the syntax or edition statement and
// the version that the statement declares. It reports false for a version
// outside the table of the frontend. A file of edition 2026 parses from a
// copy that states 2024, so its version is the version of its own text and
// not of the statement of the tree.
func parseSource(path string, src []byte) (*ast.File, *report.Report, ast.DeclSyntax, protobuf.Edition, bool) {
	tree, r, patched := protoparse.File(path, string(src))
	statement := tree.Syntax()
	if patched {
		v, known := versionNamed(keyword.Edition, protobuf.Edition2026)
		return tree, r, statement, v, known
	}
	v, known := versionOf(statement)
	return tree, r, statement, v, known
}

// versionOf returns the version that the syntax or edition statement of a
// file declares, and proto2 for a file without a statement. It reports
// false for a statement whose value is not a string of the table of the
// frontend.
func versionOf(statement ast.DeclSyntax) (protobuf.Edition, bool) {
	if statement.IsZero() {
		return versionNamed(keyword.Syntax, protobuf.Proto2)
	}
	value := statement.Value().AsLiteral().AsString()
	if value.IsZero() {
		return protobuf.Edition{}, false
	}
	return versionNamed(statement.Keyword(), value.Text())
}

// lowered is the lowering of one file. It has the unit that it writes
// through, the parsed tree that it reads, and the path of every position.
type lowered struct {
	unit *plugin.SourceUnit
	tree *ast.File
	// stream is the token stream of tree. The parser mints no synthetic
	// token, so the stream contains the tokens of the source alone.
	stream *token.Stream
	path   string
	// version is the protobuf language version of the file. The
	// resolution of every feature starts from its defaults.
	version protobuf.Edition
	// head is the first token of the file other than white space and
	// comments. The position of head is the position of the file.
	head token.Token
	// pkg is the proto package of the file, which is the namespace that
	// its declarations load under.
	pkg string
	// imports are the entries of the import stamp, one for each import in
	// source order. paths are the paths of the imports that import types.
	// [ImportPath] matches a declaring file against them in the
	// resolution step.
	imports []string
	paths   []string
	// leading and trailing are the comment groups that lead and trail a
	// token, by the token, as [lowered.attribute] attributed them.
	leading, trailing map[token.ID]commentGroup
	// consumed records every comment that a declaration took, so the
	// sweep reads only the comments that no declaration took.
	consumed map[token.ID]bool
}

// reportErrors reports each diagnostic in the report of the parser through
// the syntax report of the unit. The report contains errors alone, because
// the parse suppresses warnings.
func (l *lowered) reportErrors(r *report.Report) {
	if len(r.Diagnostics) == 0 {
		return
	}
	l.unit.ReportSyntax(UnparsedFile, func(yield func(position.Pos, string) bool) {
		// The first false result of yield ends the calls, as the protocol of
		// an iterator requires.
		more := true
		for i := range r.Diagnostics {
			d := &r.Diagnostics[i]
			more = more && yield(l.at(d.Primary()), d.Message())
		}
	})
}

// file lowers the whole tree. It lowers the package clause and the syntax
// or edition first, because every declaration loads under them, and then
// the declarations in source order.
//
// The comments of the syntax or edition statement are the comments of the
// file, and the comments of the package statement are the comments of the
// package. A licence header that a blank line separates from the first
// statement belongs to neither, as protoc attributes it.
func (l *lowered) file(statement ast.DeclSyntax) {
	decls := l.tree.Decls()
	var pkgDecl ast.DeclPackage
	for i := range decls.Len() {
		if p := decls.At(i).AsPackage(); !p.IsZero() {
			pkgDecl = p
			break
		}
	}
	l.pkg = pkgDecl.Path().Canonicalized()
	gb := l.unit.Graph()
	pkg := gb.Package(l.pkg)
	if l.pkg != "" {
		pkg.Path = strings.Split(l.pkg, protobuf.NameSep)
		pkg.Name = pkg.Path[len(pkg.Path)-1]
	}
	file := &node.File{Path: l.path, Pos: l.at(l.head.LeafSpan())}
	pkg.Files = append(pkg.Files, file)

	if !statement.IsZero() {
		c := l.commentsOf(statement.KeywordToken(), statement.Semicolon(), file)
		file.Doc, file.Annotations = c.doc, append(file.Annotations, c.annotations...)
	}
	fileOptions := statementOptions(decls)
	scope := with(l.version.Defaults, fileOptions)

	for i := range decls.Len() {
		decl := decls.At(i)
		switch decl.Kind() {
		case ast.DeclKindPackage:
			p := decl.AsPackage()
			c := l.commentsOf(p.KeywordToken(), p.Semicolon(), pkg)
			pkg.Doc = c.doc
			file.Annotations = append(file.Annotations, c.annotations...)
		case ast.DeclKindImport:
			file.Imports = append(file.Imports, l.importOf(decl.AsImport(), file))
		case ast.DeclKindDef:
			def := decl.AsDef()
			var out symbol.Symbol
			switch def.Classify() {
			case ast.DefKindMessage:
				out = l.message(def, scope, false)
			case ast.DefKindEnum:
				out = l.enum(def, scope, false)
			case ast.DefKindService:
				out = l.service(def)
			case ast.DefKindExtend:
				file.Decls = append(file.Decls, l.extend(def, scope)...)
				continue
			default:
				continue
			}
			if file.Decls == nil {
				// The declarations from this one on bound the declarations
				// of the file, so the slice is allocated once.
				file.Decls = make([]symbol.Symbol, 0, decls.Len()-i)
			}
			file.Decls = append(file.Decls, out)
		default:
			// The syntax statement and the other declarations of a file load
			// nothing.
		}
	}

	gb.Scope(file, &bindings{pkg: l.pkg, imports: l.paths})
	if l.pkg != "" {
		gb.Stamp(file, meta.RawStamp{Key: protobuf.PackageKey, Value: l.pkg, Pos: file.Pos})
	}
	if !statement.IsZero() {
		gb.Stamp(file, meta.RawStamp{Key: protobuf.SyntaxKey, Value: l.version.Name, Pos: file.Pos})
	}
	l.stampOptions(file, file.Pos, fileOptions)
	if len(l.imports) > 0 {
		gb.Stamp(file, meta.RawStamp{Key: protobuf.ImportKey, Value: l.imports, Pos: file.Pos})
	}
	l.sweep(file)
}

// importOf lowers one import and records how it is written for the
// import stamp. An import option imports the custom options of a file
// and none of its types, so the resolution step does not read it. An
// import has no identity, so a carrier on it reports as unaddressed, and
// its annotations are annotations of the file. An import with more than
// one modifier takes the form of its first modifier, and the parser
// reports such an import.
func (l *lowered) importOf(d ast.DeclImport, file *node.File) *node.Import {
	path := d.ImportPath().AsLiteral().AsString().Text()
	modifier := keyword.Unknown
	if mods := d.Modifiers(); mods.Len() > 0 {
		modifier = mods.At(0)
	}
	switch modifier {
	case keyword.Option:
		l.imports = append(l.imports, importOption+" "+path)
	case keyword.Public:
		l.imports = append(l.imports, importPublic+" "+path)
		l.paths = append(l.paths, path)
	case keyword.Weak:
		l.imports = append(l.imports, importWeak+" "+path)
		l.paths = append(l.paths, path)
	default:
		l.imports = append(l.imports, path)
		l.paths = append(l.paths, path)
	}
	first := d.KeywordToken()
	c := l.commentsOf(first, d.Semicolon(), nil)
	file.Annotations = append(file.Annotations, c.annotations...)
	return &node.Import{Pos: l.at(first.LeafSpan()), Path: path, Doc: c.doc, Comment: c.comment}
}

// sweep reads the comments that no declaration took. Their tool
// directives are annotations of the file. Their carriers are refused, so
// an authored directive above nothing is reported and not dropped.
func (l *lowered) sweep(file *node.File) {
	for tok := range l.stream.All() {
		if tok.Kind() != token.Comment || l.consumed[tok.ID()] {
			continue
		}
		parts := l.unit.Comment(tok.Text(), l.at(tok.LeafSpan()))
		file.Annotations = append(file.Annotations, parts.Annotations...)
		l.unit.RefuseCarriers(UnaddressedCarrier, parts.Carriers, unattributed)
	}
}

// message lowers a message into a struct, with the comments that
// protoc attributes to it, and its members through [lowered.members].
// outer is the features of the scope that declares the message, and
// nested reports a message that another message declares.
func (l *lowered) message(def ast.DeclDef, outer protobuf.Features, nested bool) *node.Struct {
	first, last := l.bounds(def)
	s := &node.Struct{
		Pos:        l.at(first.LeafSpan()),
		Name:       def.Name().AsIdent().Text(),
		Visibility: symbol.VisibilityPublic,
	}
	c := l.commentsOf(first, last, s)
	s.Doc, s.Annotations, s.Comment = c.doc, c.annotations, c.comment
	l.members(def, s, outer, nested)
	return s
}

// members lowers the body of a message into a struct. Its fields lower as
// fields, its oneofs as nested sums, and its groups, nested messages and
// nested enums as nested types. The fields of its extend blocks are nested
// declarations too, because an extension adds no field to the message
// that declares it. Its reserved and extension ranges, its options and its
// visibility are stamps.
//
// A nested declaration is a nested type and not a file-level one, as
// protobuf addresses it. The features of the message override outer for
// its members and its nested declarations.
func (l *lowered) members(def ast.DeclDef, s *node.Struct, outer protobuf.Features, nested bool) {
	decls := def.Body().Decls()
	options := statementOptions(decls)
	scope := with(outer, options)

	var reserved, extensions []string
	for i := range decls.Len() {
		decl := decls.At(i)
		if rng := decl.AsRange(); !rng.IsZero() {
			if !rng.IsReserved() {
				extensions = append(extensions, spelledExtensions(rng))
			} else if text := spelledReserved(rng); text != "" {
				reserved = append(reserved, text)
			}
			continue
		}
		d := decl.AsDef()
		switch d.Classify() {
		case ast.DefKindField:
			if s.Fields == nil {
				// The declarations from this one on bound the fields of the
				// message, so the slice is allocated once.
				s.Fields = make([]*node.Field, 0, decls.Len()-i)
			}
			s.Fields = append(s.Fields, l.field(d, scope, false))
		case ast.DefKindGroup:
			field, group := l.group(d, scope, false)
			s.Fields = append(s.Fields, field)
			s.Types = append(s.Types, group)
		case ast.DefKindOneof:
			sum, groups := l.oneof(d, scope)
			s.Types = append(s.Types, sum)
			s.Types = append(s.Types, groups...)
		case ast.DefKindMessage:
			s.Types = append(s.Types, l.message(d, scope, true))
		case ast.DefKindEnum:
			s.Types = append(s.Types, l.enum(d, scope, true))
		case ast.DefKindExtend:
			s.Types = append(s.Types, l.extend(d, scope)...)
		default:
			// An option of the message loads as a stamp, and no other
			// definition is a member of a message.
		}
	}
	gb := l.unit.Graph()
	if len(reserved) > 0 {
		gb.Stamp(s, meta.RawStamp{
			Key: protobuf.ReservedKey, Value: strings.Join(reserved, listSep), Pos: s.Pos,
		})
	}
	if len(extensions) > 0 {
		gb.Stamp(s, meta.RawStamp{Key: protobuf.ExtensionsKey, Value: extensions, Pos: s.Pos})
	}
	l.stampOptions(s, s.Pos, options)
	l.stampLocal(s, def, outer, nested)
}

// field lowers one plain field: its type under its label and its
// presence, and then the parts that every field form shares. A field that
// the schema wrote as a map lowers through [lowered.mapField]. outer is
// the features of the message or the oneof that declares the field, and
// member reports a member of a oneof, which has presence through its
// oneof.
//
// A field of a type other than a scalar has the delimited mark where its
// resolved message_encoding is DELIMITED. The parse does not resolve the
// type, so a field of an enum type has the mark under an inherited
// DELIMITED as well. protoc ignores the encoding of an enum field.
func (l *lowered) field(def ast.DeclDef, outer protobuf.Features, member bool) *node.Field {
	f := def.AsField()
	options := compactOptions(f.Options)
	ty, label := labelled(f.Type)
	if key, value := ty.AsGeneric().AsMap(); !key.IsZero() {
		return l.mapField(def, ty, key, value, options)
	}
	scope := with(outer, options)
	inner := namedRef(spelled(ty), l.at(ty.Span()))
	typ, required := fieldType(inner, label, scope.Presence, member)
	out := l.fieldDecl(def, f.Name.Text(), f.Tag, typ, options)
	gb := l.unit.Graph()
	if required {
		// The projection has no form for a required field, because
		// every projected field is required unless its form is an
		// optional. The frontend stamps the label instead.
		gb.Stamp(out, meta.RawStamp{Key: protobuf.LabelKey, Value: labelRequired, Pos: out.Pos})
	}
	if scope.Encoding == protobuf.EncodingDelimited && !protobuf.IsScalar(inner.Spelling) {
		gb.Stamp(
			out,
			meta.RawStamp{Key: protobuf.DelimitedKey, Value: string(protobuf.EncodingDelimited), Pos: out.Pos},
		)
	}
	return out
}

// fieldType lowers the type of a field under its label and its resolved
// presence, and reports whether the field is required. A repeated field
// is a list of its element. A member of a oneof is its type, because the
// sum of the oneof contains its presence. A field with presence is the
// optional form over its type. proto2 and proto3 give a field presence
// with the optional label, and an edition gives it with explicit
// presence. A required field and a field without presence are the type
// itself. proto2 marks a required field with its label, and an edition
// marks it with legacy required presence.
func fieldType(
	inner *node.TypeRef, label keyword.Keyword, presence protobuf.Presence, member bool,
) (*node.TypeRef, bool) {
	switch {
	case label == keyword.Repeated:
		return plugin.Composite(labelRepeated+" "+inner.Spelling, inner.Pos, symbol.FormList, inner), false
	case member:
		return inner, false
	case label == keyword.Optional:
		return plugin.Composite(labelOptional+" "+inner.Spelling, inner.Pos, symbol.FormOptional, inner), false
	case label == keyword.Required:
		return inner, true
	case presence == protobuf.PresenceExplicit:
		// An edition sets presence with a feature and not with a label,
		// so the spelling is the type as written.
		return plugin.Composite(inner.Spelling, inner.Pos, symbol.FormOptional, inner), false
	default:
		return inner, presence == protobuf.PresenceLegacyRequired
	}
}

// labelled returns a type under its outermost prefix, and the keyword of
// the prefix: the label of a field, or the stream mark of a side of an
// rpc. For a type without a prefix, it returns the type and the unknown
// keyword.
func labelled(t ast.TypeAny) (ast.TypeAny, keyword.Keyword) {
	if p := t.AsPrefixed(); !p.IsZero() {
		return p.Type(), p.Prefix()
	}
	return t, keyword.Unknown
}

// spelled returns the dotted spelling of a type path, which the
// resolution step reads, and the source text of any other type.
func spelled(t ast.TypeAny) string {
	if p := t.AsPath(); !p.IsZero() {
		return p.Canonicalized()
	}
	return t.Span().Text()
}

// namedRef lowers one named reference as the schema wrote it, and a
// well-known type with the import path of the file that declares it,
// because the workspace does not load that file and no resolution finds
// the import.
func namedRef(spelling string, at position.Pos) *node.TypeRef {
	ref := &node.TypeRef{Spelling: spelling, Pos: at}
	if file, known := protobuf.WellKnownImport(spelling); known {
		ref.Package = file
	}
	return ref
}

// mapField lowers a map field into the Map form over its key and its
// value, with the spelling that the schema wrote, and marks it as a map.
func (l *lowered) mapField(def ast.DeclDef, generic, key, value ast.TypeAny, options []option) *node.Field {
	typ := plugin.Composite(generic.Span().Text(), l.at(generic.Span()), symbol.FormMap,
		&node.TypeRef{Spelling: spelled(key), Pos: l.at(key.Span())},
		namedRef(spelled(value), l.at(value.Span())))
	out := l.fieldDecl(def, def.Name().AsIdent().Text(), def.Value(), typ, options)
	l.unit.Graph().Stamp(out, meta.RawStamp{Key: protobuf.MapEntryKey, Value: mapSpelling, Pos: out.Pos})
	return out
}

// fieldDecl lowers what every field form states: the name, the comments,
// the wire number, a default, a json_name and the other options. A plain
// field, a map field and the field of a group differ in their type alone.
// The field of a group takes its trailing comment after the opening brace
// of the body of the group, as protoc does.
func (l *lowered) fieldDecl(
	def ast.DeclDef, name string, tag ast.ExprAny, typ *node.TypeRef, options []option,
) *node.Field {
	first, last := l.bounds(def)
	out := &node.Field{
		Pos:        l.at(first.LeafSpan()),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Mutability: symbol.MutabilityMutable,
		Level:      symbol.LevelInstance,
		Type:       typ,
	}
	c := l.commentsOf(first, last, out)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	gb := l.unit.Graph()
	if number := tag.AsLiteral(); number.Kind() == token.Number {
		if n, exact := number.AsNumber().Int(); exact {
			gb.Stamp(out, meta.RawStamp{Key: protobuf.FieldKey, Value: strconv.FormatUint(n, 10), Pos: out.Pos})
		}
	}
	if o, stated := named(options, defaultOption); stated {
		// The default of proto2 is the value of an absent field, and the
		// model stores it on the field.
		out.Value = o.value
	}
	if o, stated := named(options, jsonNameOption); stated {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.JSONNameKey, Value: o.stringValue(), Pos: out.Pos})
	}
	l.stampOptions(out, out.Pos, withoutCarried(options))
	return out
}

// group lowers a proto2 group into a nested message and a field, because a
// group declares both in one statement. The message has the name and the
// members of the group. The field has the type of the message, and the
// name of the group in lower case, as protoc names it. The field also has
// the label, the number, the options and the comments of the group, and
// the marker of a delimited field, because a group is encoded delimited.
// The message repeats the documentation of the field. Both are at the
// position of the group. outer is the features of the scope that declares
// the group, and member reports a group inside a oneof.
func (l *lowered) group(def ast.DeclDef, outer protobuf.Features, member bool) (*node.Field, *node.Struct) {
	g := def.AsGroup()
	name := g.Name.Text()
	_, label := labelled(def.Type())
	options := compactOptions(g.Options)
	scope := with(outer, options)
	first, _ := l.bounds(def)
	typ, required := fieldType(namedRef(name, l.at(first.LeafSpan())), label, scope.Presence, member)
	field := l.fieldDecl(def, strings.ToLower(name), g.Tag, typ, options)
	gb := l.unit.Graph()
	if required {
		gb.Stamp(field, meta.RawStamp{Key: protobuf.LabelKey, Value: labelRequired, Pos: field.Pos})
	}
	gb.Stamp(
		field,
		meta.RawStamp{Key: protobuf.DelimitedKey, Value: string(protobuf.EncodingDelimited), Pos: field.Pos},
	)
	message := &node.Struct{
		Pos:        field.Pos,
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Doc:        slices.Clone(field.Doc),
	}
	l.members(def, message, outer, true)
	return field, message
}

// extend lowers the fields of an extend block. Each field has the
// extended message as written as its Receives, and every stamp of a
// field. A group of the block is a field with Receives and a message in
// the scope of the block, as the group of a message is. scope is the
// features of the file or of the message that declares the block.
//
// A block is no declaration of the model, so its comments belong to its
// first field. The documentation above the block comes before the
// documentation of the field, and the comment after the opening brace of
// the block comes before the comment of the field. A carrier in them
// attaches to the field. A block without a field leaves its comments to
// the sweep of the file.
func (l *lowered) extend(def ast.DeclDef, scope protobuf.Features) []symbol.Symbol {
	extendee := def.AsExtend().Extendee
	decls := def.Body().Decls()
	out := make([]symbol.Symbol, 0, decls.Len())
	var head *node.Field
	for i := range decls.Len() {
		d := decls.At(i).AsDef()
		var f *node.Field
		switch d.Classify() {
		case ast.DefKindField:
			f = l.field(d, scope, false)
			out = append(out, f)
		case ast.DefKindGroup:
			var group *node.Struct
			f, group = l.group(d, scope, false)
			out = append(out, f, group)
		default:
			// No other definition is an extension, and the parser reports
			// one that the block cannot contain.
			continue
		}
		f.Receives = namedRef(extendee.Canonicalized(), l.at(extendee.Span()))
		if head == nil {
			head = f
		}
	}
	if head != nil {
		first, last := l.bounds(def)
		c := l.commentsOf(first, last, head)
		head.Doc = append(c.doc, head.Doc...)
		head.Annotations = append(c.annotations, head.Annotations...)
		switch {
		case head.Comment == "":
			head.Comment = c.comment
		case c.comment != "":
			head.Comment = c.comment + " " + head.Comment
		}
	}
	return out
}

// variantOf returns the variant of a member of a oneof. The variant has
// the field of the member, and repeats the documentation, the comment and
// the annotations of the field.
func variantOf(f *node.Field) *node.SumVariant {
	return &node.SumVariant{
		Pos:         f.Pos,
		Name:        f.Name,
		Fields:      []*node.Field{f},
		Doc:         slices.Clone(f.Doc),
		Comment:     f.Comment,
		Annotations: slices.Clone(f.Annotations),
	}
}

// oneof lowers a oneof into a sum whose variants are its members, which is
// the tagged shape and not an untagged union. Each member is one field
// with the comments and the carriers of the member, and its variant
// repeats the documentation, the comment and the annotations of the
// field. oneof also returns the messages of the groups among the members.
// The message of the oneof declares them, because a oneof is not a scope.
func (l *lowered) oneof(def ast.DeclDef, outer protobuf.Features) (*node.Sum, []symbol.Symbol) {
	first, last := l.bounds(def)
	s := &node.Sum{
		Pos:        l.at(first.LeafSpan()),
		Name:       def.Name().AsIdent().Text(),
		Visibility: symbol.VisibilityPublic,
	}
	c := l.commentsOf(first, last, s)
	s.Doc, s.Annotations, s.Comment = c.doc, c.annotations, c.comment
	decls := def.Body().Decls()
	options := statementOptions(decls)
	scope := with(outer, options)
	var groups []symbol.Symbol
	for i := range decls.Len() {
		d := decls.At(i).AsDef()
		switch d.Classify() {
		case ast.DefKindField:
			s.Variants = append(s.Variants, variantOf(l.field(d, scope, true)))
		case ast.DefKindGroup:
			field, group := l.group(d, scope, true)
			s.Variants = append(s.Variants, variantOf(field))
			groups = append(groups, group)
		default:
			// An option of the oneof loads as a stamp, and no other
			// definition is a variant of a oneof.
		}
	}
	l.unit.Graph().Stamp(s, meta.RawStamp{
		Key: protobuf.OneofKey, Value: s.Name, Pos: s.Pos,
	})
	l.stampOptions(s, s.Pos, options)
	return s, groups
}

// enum lowers an enum into the enum kind. Each value is a variant with its
// declared number. The reserved ranges, the options, the openness and the
// visibility of the enum are stamps. outer is the features of the scope
// that declares the enum, and nested reports an enum that a message
// declares.
func (l *lowered) enum(def ast.DeclDef, outer protobuf.Features, nested bool) *node.Enum {
	first, last := l.bounds(def)
	out := &node.Enum{
		Pos:        l.at(first.LeafSpan()),
		Name:       def.Name().AsIdent().Text(),
		Visibility: symbol.VisibilityPublic,
	}
	c := l.commentsOf(first, last, out)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	decls := def.Body().Decls()
	options := statementOptions(decls)
	scope := with(outer, options)

	var reserved []string
	for i := range decls.Len() {
		decl := decls.At(i)
		if rng := decl.AsRange(); rng.IsReserved() {
			if text := spelledReserved(rng); text != "" {
				reserved = append(reserved, text)
			}
			continue
		}
		d := decl.AsDef()
		if d.Classify() != ast.DefKindEnumValue {
			continue
		}
		v := d.AsEnumValue()
		vFirst, vLast := l.bounds(d)
		variant := &node.EnumVariant{
			Pos:   l.at(vFirst.LeafSpan()),
			Name:  v.Name.Text(),
			Value: v.Tag.Span().Text(),
		}
		vc := l.commentsOf(vFirst, vLast, variant)
		variant.Doc, variant.Annotations, variant.Comment = vc.doc, vc.annotations, vc.comment
		l.stampOptions(variant, variant.Pos, compactOptions(v.Options))
		if out.Variants == nil {
			// The declarations from this one on bound the values of the
			// enum, so the slice is allocated once.
			out.Variants = make([]*node.EnumVariant, 0, decls.Len()-i)
		}
		out.Variants = append(out.Variants, variant)
	}
	gb := l.unit.Graph()
	if len(reserved) > 0 {
		gb.Stamp(out, meta.RawStamp{
			Key: protobuf.ReservedKey, Value: strings.Join(reserved, listSep), Pos: out.Pos,
		})
	}
	l.stampOptions(out, out.Pos, options)
	if scope.EnumType == protobuf.EnumClosed {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.ClosedKey, Value: string(protobuf.EnumClosed), Pos: out.Pos})
	}
	l.stampLocal(out, def, outer, nested)
	return out
}

// stampLocal marks a message or an enum that another file cannot
// reference. The keyword local makes a declaration local. A declaration
// without the keyword local or export takes the default visibility of
// the scope that declares it. The keywords are prefixes of the type of the
// definition.
func (l *lowered) stampLocal(subject symbol.Symbol, def ast.DeclDef, outer protobuf.Features, nested bool) {
	stated := keyword.Unknown
	for ty := def.Type(); ty.Kind() == ast.TypeKindPrefixed; ty = ty.AsPrefixed().Type() {
		if k := ty.AsPrefixed().Prefix(); k == keyword.Local || k == keyword.Export {
			stated = k
		}
	}
	if local(outer, stated, nested) {
		first, _ := l.bounds(def)
		l.unit.Graph().Stamp(subject, meta.RawStamp{
			Key: protobuf.LocalKey, Value: localValue, Pos: l.at(first.LeafSpan()),
		})
	}
}

// service lowers a service into an interface. Each rpc is a method that
// takes its request and returns its response.
func (l *lowered) service(def ast.DeclDef) *node.Interface {
	first, last := l.bounds(def)
	out := &node.Interface{
		Pos:        l.at(first.LeafSpan()),
		Name:       def.Name().AsIdent().Text(),
		Visibility: symbol.VisibilityPublic,
	}
	c := l.commentsOf(first, last, out)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	decls := def.Body().Decls()
	for i := range decls.Len() {
		if d := decls.At(i).AsDef(); d.Classify() == ast.DefKindMethod {
			out.Methods = append(out.Methods, l.rpc(d))
		}
	}
	l.stampOptions(out, out.Pos, statementOptions(decls))
	return out
}

// rpc lowers one rpc into an abstract method with one parameter for the
// request and one return for the response. Where a side streams, a stamp
// records which side does. An rpc whose response is not a stream is
// asynchronous, because the caller awaits the one response.
func (l *lowered) rpc(def ast.DeclDef) *node.Method {
	first, last := l.bounds(def)
	m := &node.Method{
		Pos:        l.at(first.LeafSpan()),
		Name:       def.Name().AsIdent().Text(),
		Visibility: symbol.VisibilityPublic,
		Level:      symbol.LevelInstance,
		Abstract:   true,
	}
	c := l.commentsOf(first, last, m)
	m.Doc, m.Annotations, m.Comment = c.doc, c.annotations, c.comment
	sig := def.Signature()
	if inputs := sig.Inputs(); inputs.Len() > 0 {
		in := inputs.At(0)
		m.Params = []*node.Param{{Pos: l.at(in.Span()), Name: requestParam, Type: l.rpcType(in)}}
	}
	if outputs := sig.Outputs(); outputs.Len() > 0 {
		out := outputs.At(0)
		m.Returns = []*node.Return{{Pos: l.at(out.Span()), Type: l.rpcType(out)}}
		m.Async = m.Returns[0].Type.Form != symbol.FormStream
	}
	if side := streamSide(m); side != "" {
		l.unit.Graph().Stamp(m, meta.RawStamp{Key: protobuf.StreamKey, Value: side, Pos: m.Pos})
	}
	if body := def.Body(); !body.IsZero() {
		l.unit.Graph().Stamp(m, meta.RawStamp{Key: protobuf.BodyKey, Value: bodyValue, Pos: m.Pos})
		l.stampOptions(m, m.Pos, statementOptions(body.Decls()))
	}
	return m
}

// rpcType lowers one side of an rpc to the message that it refers to.
// Where that side streams, the message is under the asynchronous stream
// form, because the messages of a proto stream arrive over a connection.
func (l *lowered) rpcType(t ast.TypeAny) *node.TypeRef {
	ty, mark := labelled(t)
	inner := namedRef(spelled(ty), l.at(ty.Span()))
	if mark != keyword.Stream {
		return inner
	}
	ref := plugin.Composite(streamKeyword+" "+inner.Spelling, l.at(t.Span()), symbol.FormStream, inner)
	ref.Async = true
	return ref
}

// streamSide returns the value of the stream stamp for the side of an rpc
// that streams, and the empty string for an rpc where neither side does.
func streamSide(m *node.Method) string {
	in := len(m.Params) > 0 && m.Params[0].Type.Form == symbol.FormStream
	out := len(m.Returns) > 0 && m.Returns[0].Type.Form == symbol.FormStream
	switch {
	case in && out:
		return streamBoth
	case in:
		return streamRequest
	case out:
		return streamResponse
	default:
		return ""
	}
}

// spelledReserved returns the text of one reserved declaration as the
// schema wrote it, with its entries comma-joined in source order. An
// entry is a range or a name. proto2 and proto3 write a name as a
// string, and an edition writes it as an identifier. It returns the
// empty string for a declaration without an entry.
func spelledReserved(r ast.DeclRange) string {
	ranges := r.Ranges()
	parts := make([]string, 0, ranges.Len())
	for i := range ranges.Len() {
		e := ranges.At(i)
		if name := e.AsLiteral().AsString(); !name.IsZero() {
			parts = append(parts, name.Text())
			continue
		}
		parts = append(parts, e.Span().Text())
	}
	return strings.Join(parts, listSep)
}

// spelledExtensions returns the text of one extension range declaration
// as the schema wrote it: its ranges, comma-joined, and its options in
// brackets.
func spelledExtensions(r ast.DeclRange) string {
	ranges := r.Ranges()
	parts := make([]string, 0, ranges.Len())
	for i := range ranges.Len() {
		parts = append(parts, ranges.At(i).Span().Text())
	}
	text := strings.Join(parts, listSep)
	if options := r.Options(); !options.IsZero() {
		text += " " + options.Span().Text()
	}
	return text
}
