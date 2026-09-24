// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"bytes"
	"context"
	"slices"
	"strings"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
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
	// mapSpelling opens protobuf's map type, which the model
	// projects as the Map form over its key and value.
	mapSpelling = "map"
	// labelRepeated and labelOptional are the field labels the
	// projection has a form for: a list, and presence.
	labelRepeated = "repeated"
	labelOptional = "optional"
	// labelRequired is proto2's required, which the projection has
	// no form for and the residue stamps.
	labelRequired = "required"
	// importPublic and importWeak mark how an import is written.
	importPublic = "public"
	importWeak   = "weak"
)

// maxSyntaxFindings caps the syntax errors one file reports, and one
// more finding counts the errors past the cap.
const maxSyntaxFindings = 10

// Parse loads one proto file: the syntax through protocompile, the
// tree lowered into the node model, and the residue stamped.
//
// A syntax error reports positioned, and the file still contributes
// every declaration the parser recovered.
//
// Depth changes nothing. A schema states no bodies and no unexported
// names, so [plugin.DepthSignatures] and [plugin.DepthFull] load the
// same declarations under the same identities.
func (protoFrontend) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref := u.Files()[0]
	src, err := u.Read(ref.Path)
	if err != nil {
		return err
	}
	var syntaxErrs []reporter.ErrorWithPos
	handler := reporter.NewHandler(reporter.NewReporter(
		func(e reporter.ErrorWithPos) error {
			syntaxErrs = append(syntaxErrs, e)
			return nil // collect and keep going: the tree still has what parsed
		},
		nil,
	))
	tree, parseErr := parser.Parse(ref.Path, bytes.NewReader(src), handler)
	for i, e := range syntaxErrs {
		at := e.GetPosition()
		pos := position.Pos{File: ref.Path, Line: at.Line, Col: at.Col}
		if i == maxSyntaxFindings {
			u.Errorf(UnparsedFile, pos, "and %d more syntax errors", len(syntaxErrs)-maxSyntaxFindings)
			break
		}
		u.Errorf(UnparsedFile, pos, "%s", e.Unwrap())
	}
	if tree == nil {
		if parseErr != nil && len(syntaxErrs) == 0 {
			u.Errorf(UnparsedFile, position.Pos{File: ref.Path, Line: 1, Col: 1}, "%v", parseErr)
		}
		return nil
	}

	l := &lowered{unit: u, tree: tree, path: ref.Path, consumed: map[ast.Item]bool{}}
	l.file()
	return nil
}

// lowered is one file's lowering: the unit it writes through, the
// parsed tree it reads, and the path every position names.
type lowered struct {
	unit *plugin.SourceUnit
	tree *ast.FileNode
	path string
	// pkg is the file's proto package, the namespace its
	// declarations load under.
	pkg string
	// edition reports that the file states an edition, whose
	// features decide a field's presence.
	edition bool
	// imports are the file's imports in source order, which the
	// import stamp spells.
	imports []string
	// consumed records every comment a declaration took, so the
	// sweep reads only the comments no declaration took.
	consumed map[ast.Item]bool
}

// file lowers the whole tree: the package clause and the syntax or
// edition first, since every declaration loads under them, then the
// declarations in source order.
//
// The syntax or edition statement's comments are the file's, and
// the package statement's comments are the package's: a licence
// header that a blank line separates from the first statement is
// neither, which is how protoc attributes it.
func (l *lowered) file() {
	l.pkg = l.packageClause()
	gb := l.unit.Graph()
	pkg := gb.Package(l.pkg)
	if l.pkg != "" {
		pkg.Path = strings.Split(l.pkg, protobuf.NameSep)
		pkg.Name = pkg.Path[len(pkg.Path)-1]
	}
	file := &node.File{Path: l.path, Pos: l.at(l.tree)}
	pkg.Files = append(pkg.Files, file)

	switch {
	case l.tree.Syntax != nil:
		c := l.commentsOf(l.tree.Syntax, nil, file)
		file.Doc, file.Annotations = c.doc, append(file.Annotations, c.annotations...)
	case l.tree.Edition != nil:
		l.edition = true
		c := l.commentsOf(l.tree.Edition, nil, file)
		file.Doc, file.Annotations = c.doc, append(file.Annotations, c.annotations...)
	}
	fileOptions := l.optionList(statementOptions(l.tree.Decls))
	presence := ""
	if l.edition {
		presence = presenceOf(fileOptions, presenceExplicit)
	}

	for _, decl := range l.tree.Decls {
		switch d := decl.(type) {
		case *ast.PackageNode:
			c := l.commentsOf(d, nil, pkg)
			pkg.Doc = c.doc
			file.Annotations = append(file.Annotations, c.annotations...)
		case *ast.MessageNode:
			file.Decls = append(file.Decls, l.message(d, presence))
		case *ast.EnumNode:
			file.Decls = append(file.Decls, l.enum(d))
		case *ast.ServiceNode:
			file.Decls = append(file.Decls, l.service(d))
		case *ast.ImportNode:
			file.Imports = append(file.Imports, l.importOf(d, file))
		case *ast.ExtendNode:
			// An extension adds a field to a message another file
			// declares, so the model has no shape for it.
			l.unit.Errorf(RefusedExtension, l.at(d),
				"%s extends %s, which it does not declare, and the block loads as nothing",
				l.path, identOf(d.Extendee))
		}
	}

	gb.Scope(file, &bindings{pkg: l.pkg})
	if l.pkg != "" {
		gb.Stamp(file, meta.RawStamp{Key: protobuf.PackageKey, Value: l.pkg, Pos: file.Pos})
	}
	if syntax := l.syntax(); syntax != "" {
		gb.Stamp(file, meta.RawStamp{Key: protobuf.SyntaxKey, Value: syntax, Pos: file.Pos})
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
// import stamp. An import has no identity, so a carrier on it
// reports as unaddressed, and its annotations are the file's.
func (l *lowered) importOf(d *ast.ImportNode, file *node.File) *node.Import {
	path := d.Name.AsString()
	switch {
	case d.Public != nil:
		l.imports = append(l.imports, importPublic+" "+path)
	case d.Weak != nil:
		l.imports = append(l.imports, importWeak+" "+path)
	default:
		l.imports = append(l.imports, path)
	}
	c := l.commentsOf(d, nil, nil)
	file.Annotations = append(file.Annotations, c.annotations...)
	return &node.Import{Pos: l.at(d), Path: path, Doc: c.doc, Comment: c.comment}
}

// sweep reads the comments no declaration took: their tool
// directives become the file's annotations, and their carriers
// report as unaddressed, so an authored directive above nothing is
// reported and never dropped.
func (l *lowered) sweep(file *node.File) {
	items := l.tree.Items()
	for item, held := items.First(); held; item, held = items.Next(item) {
		if l.consumed[item] {
			continue
		}
		comment, is := l.commentAt(item)
		if !is {
			continue
		}
		parts := l.unit.Comment(comment.RawText(),
			position.Pos{File: l.path, Line: comment.Start().Line, Col: comment.Start().Col})
		file.Annotations = append(file.Annotations, parts.Annotations...)
		for _, carrier := range parts.Carriers {
			l.unit.Errorf(UnaddressedCarrier, carrier.Pos,
				"%q is in a comment protoc attributes to no declaration; move it above the declaration",
				plugin.CarrierMark+carrier.Payload)
		}
	}
}

// packageClause returns the file's proto package. A file stating
// none returns empty, which the graph keeps as the unnamed
// namespace.
func (l *lowered) packageClause() string {
	for _, decl := range l.tree.Decls {
		if p, is := decl.(*ast.PackageNode); is {
			return identOf(p.Name)
		}
	}
	return ""
}

// syntax returns the file's syntax or edition as written.
func (l *lowered) syntax() string {
	switch {
	case l.tree.Syntax != nil:
		return l.tree.Syntax.Syntax.AsString()
	case l.tree.Edition != nil:
		return l.tree.Edition.Edition.AsString()
	default:
		return ""
	}
}

// brace returns a block's opening brace as a node, and nil for a
// declaration without one, so no nil pointer becomes a non-nil
// interface.
func brace(r *ast.RuneNode) ast.Node {
	if r == nil {
		return nil
	}
	return r
}

// message lowers a message into a struct: its fields, its oneofs as
// nested sums, its nested messages and enums as nested types, and
// its reserved and extension ranges as stamps.
//
// A nested declaration is a nested type, not a file-level one, which
// is how protobuf addresses it. presence is the field presence the
// enclosing scope states, which the message's own features override
// for its fields and its nested messages.
func (l *lowered) message(m *ast.MessageNode, presence string) *node.Struct {
	s := &node.Struct{
		Pos:        l.at(m),
		Name:       m.Name.Val,
		Visibility: symbol.VisibilityPublic,
	}
	c := l.commentsOf(m, brace(m.OpenBrace), s)
	s.Doc, s.Annotations, s.Comment = c.doc, c.annotations, c.comment
	options := l.optionList(statementOptions(m.Decls))
	if l.edition {
		presence = presenceOf(options, presence)
	}

	var reserved, extensions []string
	for _, decl := range m.Decls {
		switch d := decl.(type) {
		case *ast.FieldNode:
			s.Fields = append(s.Fields, l.field(d, presence))
		case *ast.MapFieldNode:
			s.Fields = append(s.Fields, l.mapField(d))
		case *ast.OneofNode:
			s.Types = append(s.Types, l.oneof(d))
		case *ast.MessageNode:
			s.Types = append(s.Types, l.message(d, presence))
		case *ast.EnumNode:
			s.Types = append(s.Types, l.enum(d))
		case *ast.ReservedNode:
			if spelled := l.reserved(d); spelled != "" {
				reserved = append(reserved, spelled)
			}
		case *ast.ExtensionRangeNode:
			extensions = append(extensions, l.extensionRange(d))
		case *ast.GroupNode:
			// A group is proto2's inlined message and field in one
			// declaration, and the model has a field or a type, never
			// both at once.
			l.unit.Errorf(RefusedGroup, l.at(d),
				"%s declares group %s inside %s, which is one declaration the model represents as two",
				l.path, d.Name.Val, s.Name)
		case *ast.ExtendNode:
			l.unit.Errorf(RefusedExtension, l.at(d),
				"%s extends %s inside %s, and the block loads as nothing",
				l.path, identOf(d.Extendee), s.Name)
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
	return s
}

// field lowers one plain field: its type under its label and its
// presence, then the parts every field form shares. presence is the
// edition's resolved field presence before the field's own
// features, and empty outside an edition and inside a oneof, where
// the label or the oneof decides.
func (l *lowered) field(f *ast.FieldNode, presence string) *node.Field {
	options := l.optionList(compact(f.Options))
	label := ""
	if f.Label.KeywordNode != nil {
		label = f.Label.Val
	}
	if presence != "" {
		presence = presenceOf(options, presence)
	}
	typ, required := l.fieldType(f, label, presence)
	out := l.fieldDecl(f, f.Name.Val, f.Tag, typ, options)
	if required {
		// A required field has no form in the projection: every
		// projected field is required unless its form states
		// otherwise, so the label is stamped.
		l.unit.Graph().Stamp(out, meta.RawStamp{Key: protobuf.LabelKey, Value: labelRequired, Pos: out.Pos})
	}
	return out
}

// fieldType lowers a field's type under its label and its presence,
// and reports whether the field is required. A repeated field is a
// list of its element. A field with presence, which proto2 and
// proto3 state with the optional label and an edition states with
// explicit presence, is the optional form over its type. A required
// field, which proto2 labels and an edition states with legacy
// required presence, and a field without presence are the type
// itself.
func (l *lowered) fieldType(f *ast.FieldNode, label, presence string) (*node.TypeRef, bool) {
	inner := &node.TypeRef{Spelling: identOf(f.FldType), Pos: l.at(f.FldType)}
	switch {
	case label == labelRepeated:
		return &node.TypeRef{
			Spelling: labelRepeated + " " + inner.Spelling, Pos: inner.Pos,
			Form: symbol.FormList, Elems: []*node.TypeRef{inner},
		}, false
	case label == labelOptional:
		return &node.TypeRef{
			Spelling: labelOptional + " " + inner.Spelling, Pos: inner.Pos,
			Form: symbol.FormOptional, Elems: []*node.TypeRef{inner},
		}, false
	case presence == presenceExplicit:
		// An edition states presence with a feature, not a label, so
		// the spelling is the type as written.
		return &node.TypeRef{
			Spelling: inner.Spelling, Pos: inner.Pos,
			Form: symbol.FormOptional, Elems: []*node.TypeRef{inner},
		}, false
	default:
		return inner, label == labelRequired || presence == presenceLegacyRequired
	}
}

// mapField lowers a map field into the Map form over its key and its
// value, with the spelling the schema wrote, and marks it as a map.
func (l *lowered) mapField(f *ast.MapFieldNode) *node.Field {
	key := &node.TypeRef{Spelling: f.MapType.KeyType.Val, Pos: l.at(f.MapType.KeyType)}
	value := &node.TypeRef{Spelling: identOf(f.MapType.ValueType), Pos: l.at(f.MapType.ValueType)}
	typ := &node.TypeRef{
		Spelling: l.text(f.MapType),
		Pos:      l.at(f.MapType),
		Form:     symbol.FormMap,
		Elems:    []*node.TypeRef{key, value},
	}
	out := l.fieldDecl(f, f.Name.Val, f.Tag, typ, l.optionList(compact(f.Options)))
	l.unit.Graph().Stamp(out, meta.RawStamp{Key: protobuf.MapEntryKey, Value: mapSpelling, Pos: out.Pos})
	return out
}

// fieldDecl lowers what every field form states: the name, the
// comments, the wire number, a default, a json_name and the other
// options. A plain field and a map field differ in their type
// alone.
func (l *lowered) fieldDecl(
	n ast.Node, name string, tag *ast.UintLiteralNode, typ *node.TypeRef, options []option,
) *node.Field {
	out := &node.Field{
		Pos:        l.at(n),
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Mutability: symbol.MutabilityMutable,
		Level:      symbol.LevelInstance,
		Type:       typ,
	}
	c := l.commentsOf(n, nil, out)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	gb := l.unit.Graph()
	if tag != nil {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.FieldKey, Value: uintText(tag), Pos: out.Pos})
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

// oneof lowers a oneof into a sum whose variants are its members:
// the tagged shape, never an untagged union. Each member is one
// field, which takes the member's comments and carriers, and its
// variant repeats the field's documentation, comment and
// annotations.
func (l *lowered) oneof(o *ast.OneofNode) *node.Sum {
	s := &node.Sum{
		Pos:        l.at(o),
		Name:       o.Name.Val,
		Visibility: symbol.VisibilityPublic,
	}
	c := l.commentsOf(o, brace(o.OpenBrace), s)
	s.Doc, s.Annotations, s.Comment = c.doc, c.annotations, c.comment
	for _, decl := range o.Decls {
		switch d := decl.(type) {
		case *ast.FieldNode:
			// A oneof member has presence through the oneof, so no
			// label and no edition feature decides its form.
			f := l.field(d, "")
			s.Variants = append(s.Variants, &node.SumVariant{
				Pos:         f.Pos,
				Name:        f.Name,
				Fields:      []*node.Field{f},
				Doc:         slices.Clone(f.Doc),
				Comment:     f.Comment,
				Annotations: slices.Clone(f.Annotations),
			})
		case *ast.GroupNode:
			l.unit.Errorf(RefusedGroup, l.at(d),
				"%s declares group %s inside oneof %s, which is one declaration the model represents as two",
				l.path, d.Name.Val, s.Name)
		}
	}
	l.unit.Graph().Stamp(s, meta.RawStamp{
		Key: protobuf.OneofKey, Value: s.Name, Pos: s.Pos,
	})
	l.stampOptions(s, s.Pos, l.optionList(statementOptions(o.Decls)))
	return s
}

// enum lowers an enum into the enum kind: each value a variant with
// its declared number, and its reserved ranges and options as
// stamps.
func (l *lowered) enum(e *ast.EnumNode) *node.Enum {
	out := &node.Enum{
		Pos:        l.at(e),
		Name:       e.Name.Val,
		Visibility: symbol.VisibilityPublic,
	}
	c := l.commentsOf(e, brace(e.OpenBrace), out)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment

	var reserved []string
	for _, decl := range e.Decls {
		switch d := decl.(type) {
		case *ast.EnumValueNode:
			variant := &node.EnumVariant{
				Pos:   l.at(d),
				Name:  d.Name.Val,
				Value: l.text(d.Number),
			}
			vc := l.commentsOf(d, nil, variant)
			variant.Doc, variant.Annotations, variant.Comment = vc.doc, vc.annotations, vc.comment
			l.stampOptions(variant, variant.Pos, l.optionList(compact(d.Options)))
			out.Variants = append(out.Variants, variant)
		case *ast.ReservedNode:
			if spelled := l.reserved(d); spelled != "" {
				reserved = append(reserved, spelled)
			}
		}
	}
	if len(reserved) > 0 {
		l.unit.Graph().Stamp(out, meta.RawStamp{
			Key: protobuf.ReservedKey, Value: strings.Join(reserved, listSep), Pos: out.Pos,
		})
	}
	l.stampOptions(out, out.Pos, l.optionList(statementOptions(e.Decls)))
	return out
}

// service lowers a service into an interface, each rpc a method
// taking its request and returning its response.
func (l *lowered) service(s *ast.ServiceNode) *node.Interface {
	out := &node.Interface{
		Pos:        l.at(s),
		Name:       s.Name.Val,
		Visibility: symbol.VisibilityPublic,
	}
	c := l.commentsOf(s, brace(s.OpenBrace), out)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	for _, decl := range s.Decls {
		if rpc, is := decl.(*ast.RPCNode); is {
			out.Methods = append(out.Methods, l.rpc(rpc))
		}
	}
	l.stampOptions(out, out.Pos, l.optionList(statementOptions(s.Decls)))
	return out
}

// rpc lowers one rpc into an abstract method: one parameter for the
// request, one return for the response, and a stamp naming which
// side streams where either does.
func (l *lowered) rpc(r *ast.RPCNode) *node.Method {
	m := &node.Method{
		Pos:        l.at(r),
		Name:       r.Name.Val,
		Visibility: symbol.VisibilityPublic,
		Level:      symbol.LevelInstance,
		Abstract:   true,
	}
	c := l.commentsOf(r, brace(r.OpenBrace), m)
	m.Doc, m.Annotations, m.Comment = c.doc, c.annotations, c.comment
	if r.Input != nil {
		m.Params = []*node.Param{{
			Pos:  l.at(r.Input),
			Name: requestParam,
			Type: l.rpcType(r.Input),
		}}
	}
	if r.Output != nil {
		m.Returns = []*node.Return{{Pos: l.at(r.Output), Type: l.rpcType(r.Output)}}
	}
	if side := streamSide(r); side != "" {
		l.unit.Graph().Stamp(m, meta.RawStamp{Key: protobuf.StreamKey, Value: side, Pos: m.Pos})
	}
	l.stampOptions(m, m.Pos, l.optionList(statementOptions(r.Decls)))
	return m
}

// rpcType lowers one side of an rpc: the message it names, under the
// stream form where that side streams.
func (l *lowered) rpcType(t *ast.RPCTypeNode) *node.TypeRef {
	inner := &node.TypeRef{Spelling: identOf(t.MessageType), Pos: l.at(t.MessageType)}
	if t.Stream == nil {
		return inner
	}
	return &node.TypeRef{
		Spelling: streamKeyword + " " + inner.Spelling, Pos: l.at(t),
		Form: symbol.FormStream, Elems: []*node.TypeRef{inner},
	}
}

// streamSide names which side of an rpc streams, and empty for one
// where neither does.
func streamSide(r *ast.RPCNode) string {
	in := r.Input != nil && r.Input.Stream != nil
	out := r.Output != nil && r.Output.Stream != nil
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

// reserved spells one reserved declaration as written: its ranges,
// or its names, which proto2 and proto3 write as strings and an
// edition writes as identifiers, comma-joined. A declaration naming
// nothing spells empty.
func (l *lowered) reserved(r *ast.ReservedNode) string {
	parts := make([]string, 0, len(r.Ranges)+len(r.Names)+len(r.Identifiers))
	for _, rng := range r.Ranges {
		parts = append(parts, l.text(rng))
	}
	for _, name := range r.Names {
		parts = append(parts, name.AsString())
	}
	for _, ident := range r.Identifiers {
		parts = append(parts, ident.Val)
	}
	return strings.Join(parts, listSep)
}

// extensionRange spells one extension range declaration as written:
// its ranges, comma-joined, and the options it states in brackets.
func (l *lowered) extensionRange(e *ast.ExtensionRangeNode) string {
	parts := make([]string, 0, len(e.Ranges))
	for _, r := range e.Ranges {
		parts = append(parts, l.text(r))
	}
	spelled := strings.Join(parts, listSep)
	if e.Options != nil {
		spelled += " " + l.text(e.Options)
	}
	return spelled
}
