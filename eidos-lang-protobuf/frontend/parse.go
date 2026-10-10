// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/bufbuild/protocompile/experimental/ast"
	"github.com/bufbuild/protocompile/experimental/parser"
	"github.com/bufbuild/protocompile/experimental/report"
	"github.com/bufbuild/protocompile/experimental/source"
	"github.com/bufbuild/protocompile/experimental/token"
	"github.com/bufbuild/protocompile/experimental/token/keyword"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The proto spellings the lowering reads.
const (
	// listSep joins the entries of a stamped list, a reserved range
	// set or a file's options.
	listSep = ", "
	// streamRequest, streamResponse and streamBoth name which side
	// of an rpc streams.
	streamRequest  = "request"
	streamResponse = "response"
	streamBoth     = "both"
	// streamKeyword marks a streaming side of an rpc.
	streamKeyword = "stream"
	// requestParam names the one parameter an rpc takes.
	requestParam = "request"
	// mapSpelling marks a field the schema wrote as a map, which the
	// model projects as the Map form over its key and value.
	mapSpelling = "map"
	// labelRepeated and labelOptional are the field labels the
	// projection has a form for: a list, and presence.
	labelRepeated = "repeated"
	labelOptional = "optional"
	// labelRequired is proto2's required, which the projection has
	// no form for and the residue stamps.
	labelRequired = "required"
	// importPublic, importWeak and importOption mark how an import is
	// written.
	importPublic = "public"
	importWeak   = "weak"
	importOption = "option"
	// localValue is the value of the marker of a message or an enum that
	// another file cannot reference.
	localValue = "local"
)

// maxSyntaxFindings caps the syntax errors one file reports, and one
// more finding counts the errors past the cap.
const maxSyntaxFindings = 10

// Parse loads one proto file. It parses the syntax with protocompile's
// experimental parser, reads the version from the file's syntax or
// edition statement, lowers the tree into the node model and stamps the
// residue.
//
// Parse reports a syntax error under [UnparsedFile] at its position, and
// the file still contributes every declaration that the parser
// recovered. Parse reports a version outside the frontend's table under
// [UnknownEdition], and the file does not contribute any declaration,
// because the defaults that define presence and the openness of enums
// are unknown.
//
// The depth does not change the result. A schema has no bodies and no
// unexported names, so [plugin.DepthSignatures] and [plugin.DepthFull]
// load the same declarations under the same identities.
func (protoFrontend) Parse(ctx context.Context, u *plugin.SourceUnit) error {
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
	l.reportErrors(r, statement)
	l.attribute()
	l.file(statement)
	return nil
}

// parseSource parses a file's source with protocompile's experimental
// parser. It returns the tree, the parser's report, the syntax or
// edition statement and the version that the statement declares. The
// report has no warnings, because each of them is about the style of a
// schema. It reports false for a version outside the frontend's table.
//
// The parser does not know Edition 2026, which has the grammar of
// Edition 2024. It parses a 2026 file a second time, from a copy whose
// edition value is 2024. Both values have four bytes, so every position in
// the copy is the position in the file.
func parseSource(path string, src []byte) (*ast.File, *report.Report, ast.DeclSyntax, version, bool) {
	text := string(src)
	r := &report.Report{SuppressWarnings: true}
	tree, _ := parser.Parse(path, source.NewFile(path, text), r)
	statement := tree.Syntax()
	v, known := versionOf(statement)
	content := statement.Value().AsLiteral().AsString().RawContent()
	if v.name != edition2026 || content.Text() != edition2026 {
		return tree, r, statement, v, known
	}
	patched := text[:content.Start] + edition2024 + text[content.End:]
	r = &report.Report{SuppressWarnings: true}
	tree, _ = parser.Parse(path, source.NewFile(path, patched), r)
	statement = tree.Syntax()
	return tree, r, statement, v, known
}

// versionOf returns the version that a file's syntax or edition statement
// declares, and proto2 for a file without a statement. It reports false
// for a statement whose value is not a string of the frontend's table.
func versionOf(statement ast.DeclSyntax) (version, bool) {
	if statement.IsZero() {
		return versionNamed(keyword.Syntax, versionProto2)
	}
	value := statement.Value().AsLiteral().AsString()
	if value.IsZero() {
		return version{}, false
	}
	return versionNamed(statement.Keyword(), value.Text())
}

// lowered is one file's lowering: the unit it writes through, the
// parsed tree it reads, and the path every position names.
type lowered struct {
	unit   *plugin.SourceUnit
	tree   *ast.File
	stream *token.Stream
	path   string
	// version is the file's protobuf language version, whose defaults the
	// resolution of every feature starts from.
	version version
	// head is the file's first token other than white space and
	// comments. The position of head is the position of the file.
	head token.Token
	// pkg is the file's proto package, the namespace its
	// declarations load under.
	pkg string
	// imports are the file's imports in source order, which the
	// import stamp spells, and paths the paths of the imports that
	// import types, which the resolution step's
	// [protoFrontend.ImportOf] matches a declaring file against.
	imports []string
	paths   []string
	// leading and trailing are the comment groups that lead and trail a
	// token, by the token, as [lowered.attribute] attributed them.
	leading, trailing map[token.ID]commentGroup
	// consumed records every comment a declaration took, so the
	// sweep reads only the comments no declaration took.
	consumed map[token.ID]bool
}

// reportErrors reports each error of the parser's report under
// [UnparsedFile] at its position: the first maxSyntaxFindings, then one
// finding that counts the rest. It drops every diagnostic at the version
// statement, because the frontend reports its own verdict there.
func (l *lowered) reportErrors(r *report.Report, statement ast.DeclSyntax) {
	if len(r.Diagnostics) == 0 {
		return
	}
	stated := statement.Span()
	var (
		reported, more int
		moreAt         position.Pos
	)
	for i := range r.Diagnostics {
		d := &r.Diagnostics[i]
		at := d.Primary()
		if d.Level() != report.ICE && d.Level() != report.Error ||
			!stated.IsZero() && at.Start >= stated.Start && at.End <= stated.End {

			continue
		}
		if reported == maxSyntaxFindings {
			if more == 0 {
				moreAt = l.at(at)
			}
			more++
			continue
		}
		l.unit.Errorf(UnparsedFile, l.at(at), "%s", d.Message())
		reported++
	}
	if more > 0 {
		l.unit.Errorf(UnparsedFile, moreAt, "and %d more syntax errors", more)
	}
}

// file lowers the whole tree: the package clause and the syntax or
// edition first, since every declaration loads under them, then the
// declarations in source order.
//
// The syntax or edition statement's comments are the file's, and
// the package statement's comments are the package's. A licence header
// that a blank line separates from the first statement belongs to
// neither, as protoc attributes it.
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
	scope := l.version.defaults.with(fileOptions)

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
				// An extension adds a field to a message another file
				// declares, so the model cannot represent it.
				first, _ := l.bounds(def)
				l.unit.Errorf(RefusedExtension, l.at(first.LeafSpan()),
					"%s extends %s, which it does not declare, so the frontend does not load the block",
					l.path, def.AsExtend().Extendee.Canonicalized())
				continue
			default:
				continue
			}
			if file.Decls == nil {
				// The declarations from this one on bound the file's
				// declarations, so the slice is allocated once.
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
		gb.Stamp(file, meta.RawStamp{Key: protobuf.SyntaxKey, Value: l.version.name, Pos: file.Pos})
	}
	l.stampOptions(file, file.Pos, fileOptions)
	if len(l.imports) > 0 {
		gb.Stamp(file, meta.RawStamp{
			Key: protobuf.ImportKey, Value: strings.Join(l.imports, listSep), Pos: file.Pos,
		})
	}
	l.sweep(file)
}

// importOf lowers one import and records how it is written for the
// import stamp. An import option imports the custom options of a file
// and none of its types, so the resolution step does not read it. An
// import has no identity, so a carrier on it reports as unaddressed,
// and its annotations are the file's. An import with more than one
// modifier takes the form of its first modifier, and the parser reports
// such an import.
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

// sweep reads the comments no declaration took: their tool
// directives become the file's annotations, and their carriers
// report as unaddressed, so an authored directive above nothing is
// reported and never dropped.
func (l *lowered) sweep(file *node.File) {
	for tok := range l.stream.All() {
		if tok.IsSynthetic() {
			break
		}
		if tok.Kind() != token.Comment || l.consumed[tok.ID()] {
			continue
		}
		parts := l.unit.Comment(tok.Text(), l.at(tok.LeafSpan()))
		file.Annotations = append(file.Annotations, parts.Annotations...)
		for _, carrier := range parts.Carriers {
			l.unit.Errorf(UnaddressedCarrier, carrier.Pos,
				"%q is in a comment protoc attributes to no declaration. Move it directly above a declaration",
				carrier.Mark+carrier.Payload)
		}
	}
}

// message lowers a message into a struct, with the comments that
// protoc attributes to it, and its members through [lowered.members].
// outer is the features of the scope that declares the message, and
// nested reports a message that another message declares.
func (l *lowered) message(def ast.DeclDef, outer features, nested bool) *node.Struct {
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

// members lowers a message's body into a struct: its fields, its oneofs
// as nested sums, its groups, nested messages and nested enums as nested
// types, and its reserved and extension ranges, its options and its
// visibility as stamps.
//
// A nested declaration is a nested type, not a file-level one, which
// is how protobuf addresses it. The message's own features override
// outer for its members and its nested declarations.
func (l *lowered) members(def ast.DeclDef, s *node.Struct, outer features, nested bool) {
	decls := def.Body().Decls()
	options := statementOptions(decls)
	scope := outer.with(options)

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
				// The declarations from this one on bound the message's
				// fields, so the slice is allocated once.
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
			first, _ := l.bounds(d)
			l.unit.Errorf(RefusedExtension, l.at(first.LeafSpan()),
				"%s extends %s inside %s, so the frontend does not load the block",
				l.path, d.AsExtend().Extendee.Canonicalized(), s.Name)
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
		gb.Stamp(s, meta.RawStamp{
			Key: protobuf.ExtensionsKey, Value: strings.Join(extensions, listSep), Pos: s.Pos,
		})
	}
	l.stampOptions(s, s.Pos, options)
	l.stampLocal(s, def, outer, nested)
}

// field lowers one plain field: its type under its label and its
// presence, then the parts every field form shares. A field the schema
// wrote as a map lowers through [lowered.mapField]. outer is the features
// of the message or the oneof that declares the field, and member
// reports a member of a oneof, which has presence through its oneof.
//
// A field of a type other than a scalar has the delimited mark where its
// resolved message_encoding is DELIMITED. The parse does not resolve the
// type, so a field of an enum type has the mark under an inherited
// DELIMITED as well. protoc ignores the encoding of an enum field.
func (l *lowered) field(def ast.DeclDef, outer features, member bool) *node.Field {
	f := def.AsField()
	options := compactOptions(f.Options)
	ty, label := labelled(f.Type)
	if key, value := ty.AsGeneric().AsMap(); !key.IsZero() {
		return l.mapField(def, ty, key, value, options)
	}
	scope := outer.with(options)
	inner := namedRef(spelled(ty), l.at(ty.Span()))
	typ, required := fieldType(inner, label, scope.presence, member)
	out := l.fieldDecl(def, f.Name.Text(), f.Tag, typ, options)
	gb := l.unit.Graph()
	if required {
		// The projection has no form for a required field, because
		// every projected field is required unless its form is an
		// optional. The frontend stamps the label instead.
		gb.Stamp(out, meta.RawStamp{Key: protobuf.LabelKey, Value: labelRequired, Pos: out.Pos})
	}
	if scope.encoding == encodingDelimited && !protobuf.IsScalar(inner.Spelling) {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.DelimitedKey, Value: encodingDelimited, Pos: out.Pos})
	}
	return out
}

// fieldType lowers a field's type under its label and its resolved
// presence, and reports whether the field is required. A repeated field
// is a list of its element. A member of a oneof is its type, because the
// oneof's sum contains its presence. A field with presence is the
// optional form over its type. proto2 and proto3 give a field presence
// with the optional label, and an edition gives it with explicit
// presence. A required field and a field without presence are the type
// itself. proto2 marks a required field with its label, and an edition
// marks it with legacy required presence.
func fieldType(inner *node.TypeRef, label keyword.Keyword, presence string, member bool) (*node.TypeRef, bool) {
	switch {
	case label == keyword.Repeated:
		return &node.TypeRef{
			Spelling: labelRepeated + " " + inner.Spelling, Pos: inner.Pos,
			Form: symbol.FormList, Elems: []*node.TypeRef{inner},
		}, false
	case member:
		return inner, false
	case label == keyword.Optional:
		return &node.TypeRef{
			Spelling: labelOptional + " " + inner.Spelling, Pos: inner.Pos,
			Form: symbol.FormOptional, Elems: []*node.TypeRef{inner},
		}, false
	case label == keyword.Required:
		return inner, true
	case presence == presenceExplicit:
		// An edition sets presence with a feature and not with a label,
		// so the spelling is the type as written.
		return &node.TypeRef{
			Spelling: inner.Spelling, Pos: inner.Pos,
			Form: symbol.FormOptional, Elems: []*node.TypeRef{inner},
		}, false
	default:
		return inner, presence == presenceLegacyRequired
	}
}

// labelled returns a type under its outermost prefix, and the prefix's
// keyword: a field's label, or the stream mark of an rpc's side. A type
// without a prefix returns itself and the unknown keyword.
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
// because the workspace does not load that file and no resolution
// names the import.
func namedRef(spelling string, at position.Pos) *node.TypeRef {
	ref := &node.TypeRef{Spelling: spelling, Pos: at}
	if file, known := protobuf.WellKnownImport(spelling); known {
		ref.Package = file
	}
	return ref
}

// mapField lowers a map field into the Map form over its key and its
// value, with the spelling the schema wrote, and marks it as a map.
func (l *lowered) mapField(def ast.DeclDef, generic, key, value ast.TypeAny, options []option) *node.Field {
	typ := &node.TypeRef{
		Spelling: generic.Span().Text(),
		Pos:      l.at(generic.Span()),
		Form:     symbol.FormMap,
		Elems: []*node.TypeRef{
			{Spelling: spelled(key), Pos: l.at(key.Span())},
			namedRef(spelled(value), l.at(value.Span())),
		},
	}
	out := l.fieldDecl(def, def.Name().AsIdent().Text(), def.Value(), typ, options)
	l.unit.Graph().Stamp(out, meta.RawStamp{Key: protobuf.MapEntryKey, Value: mapSpelling, Pos: out.Pos})
	return out
}

// fieldDecl lowers what every field form states: the name, the
// comments, the wire number, a default, a json_name and the other
// options. A plain field, a map field and a group's field differ in
// their type alone. A group's field takes its trailing comment after
// the opening brace of the group's body, as protoc does.
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
		// proto2's default is the value an absent field reads as,
		// which the model stores on the field.
		out.Value = o.value
	}
	if o, stated := named(options, jsonNameOption); stated {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.JSONNameKey, Value: o.stringValue(), Pos: out.Pos})
	}
	l.stampOptions(out, out.Pos, withoutCarried(options))
	return out
}

// group lowers a proto2 group into a nested message and a field, because
// a group declares both in one statement. The message has the group's
// name and members. The field has the message's type, and the group's
// name in lower case, as protoc names it. The field also has the group's
// label, number, options and comments, and the marker of a delimited
// field, because a group is encoded delimited. The message repeats the
// field's documentation. Both have the group's position. outer is the
// features of the scope that declares the group, and member reports a
// group inside a oneof.
func (l *lowered) group(def ast.DeclDef, outer features, member bool) (*node.Field, *node.Struct) {
	g := def.AsGroup()
	name := g.Name.Text()
	_, label := labelled(def.Type())
	options := compactOptions(g.Options)
	scope := outer.with(options)
	first, _ := l.bounds(def)
	typ, required := fieldType(namedRef(name, l.at(first.LeafSpan())), label, scope.presence, member)
	field := l.fieldDecl(def, strings.ToLower(name), g.Tag, typ, options)
	gb := l.unit.Graph()
	if required {
		gb.Stamp(field, meta.RawStamp{Key: protobuf.LabelKey, Value: labelRequired, Pos: field.Pos})
	}
	gb.Stamp(field, meta.RawStamp{Key: protobuf.DelimitedKey, Value: encodingDelimited, Pos: field.Pos})
	message := &node.Struct{
		Pos:        field.Pos,
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Doc:        slices.Clone(field.Doc),
	}
	l.members(def, message, outer, true)
	return field, message
}

// variantOf returns the variant of a oneof member. The variant has the
// member's field, and it repeats the field's documentation, comment and
// annotations.
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

// oneof lowers a oneof into a sum whose variants are its members:
// the tagged shape, never an untagged union. Each member is one
// field, which takes the member's comments and carriers, and its
// variant repeats the field's documentation, comment and
// annotations. It also returns the messages of the groups among the
// members, which the oneof's message declares, because a oneof is no
// scope.
func (l *lowered) oneof(def ast.DeclDef, outer features) (*node.Sum, []symbol.Symbol) {
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
	scope := outer.with(options)
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

// enum lowers an enum into the enum kind: each value a variant with
// its declared number, and its reserved ranges, its options, its
// openness and its visibility as stamps. outer is the features of the
// scope that declares the enum, and nested reports an enum that a
// message declares.
func (l *lowered) enum(def ast.DeclDef, outer features, nested bool) *node.Enum {
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
	scope := outer.with(options)

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
			// The declarations from this one on bound the enum's values,
			// so the slice is allocated once.
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
	if scope.enumType == enumClosed {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.ClosedKey, Value: enumClosed, Pos: out.Pos})
	}
	l.stampLocal(out, def, outer, nested)
	return out
}

// stampLocal marks a message or an enum that another file cannot
// reference. The keyword local makes a declaration local. A declaration
// without the keyword local or export takes the default visibility of
// the scope that declares it. The keywords are prefixes of the
// definition's type.
func (l *lowered) stampLocal(subject symbol.Symbol, def ast.DeclDef, outer features, nested bool) {
	stated := keyword.Unknown
	for ty := def.Type(); ty.Kind() == ast.TypeKindPrefixed; ty = ty.AsPrefixed().Type() {
		if k := ty.AsPrefixed().Prefix(); k == keyword.Local || k == keyword.Export {
			stated = k
		}
	}
	if outer.local(stated, nested) {
		first, _ := l.bounds(def)
		l.unit.Graph().Stamp(subject, meta.RawStamp{
			Key: protobuf.LocalKey, Value: localValue, Pos: l.at(first.LeafSpan()),
		})
	}
}

// service lowers a service into an interface, each rpc a method
// taking its request and returning its response.
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

// rpc lowers one rpc into an abstract method: one parameter for the
// request, one return for the response, and a stamp naming which
// side streams where either does. An rpc whose response is not a
// stream is asynchronous, because the caller awaits the one response.
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
		l.stampOptions(m, m.Pos, statementOptions(body.Decls()))
	}
	return m
}

// rpcType lowers one side of an rpc: the message it names, under the
// asynchronous stream form where that side streams, because the
// messages of a proto stream arrive over a connection.
func (l *lowered) rpcType(t ast.TypeAny) *node.TypeRef {
	ty, mark := labelled(t)
	inner := namedRef(spelled(ty), l.at(ty.Span()))
	if mark != keyword.Stream {
		return inner
	}
	return &node.TypeRef{
		Spelling: streamKeyword + " " + inner.Spelling, Pos: l.at(t.Span()),
		Form: symbol.FormStream, Async: true, Elems: []*node.TypeRef{inner},
	}
}

// streamSide names which side of an rpc streams, and empty for one
// where neither does.
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
