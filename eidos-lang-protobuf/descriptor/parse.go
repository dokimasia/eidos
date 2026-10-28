// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of the descriptor that the lowering reads.
const (
	// syntaxEditions is the syntax of a file of an edition, whose edition
	// field states the edition.
	syntaxEditions = "editions"
	// editionPrefix opens the name of a value of the Edition enum.
	editionPrefix = "EDITION_"
	// The modifiers of an import, as the import stamp of the satellite
	// spells them.
	importPublic = "public"
	importWeak   = "weak"
	importOption = "option"
)

// The descriptions of a subject in the refusal of a carrier.
const (
	unattributed  = "a comment that protoc attributes to no declaration"
	importSubject = "an import"
)

// parse loads one file of a request: a serialized FileDescriptorProto at
// the proto path of the file. It decodes the descriptor, resolves the
// custom options of its declarations through [Registry], reads the version
// of the file, and lowers it into the node model with the projection of the
// source frontend.
//
// parse reports a version outside the table of the satellite under
// [frontend.UnknownEdition], and the file loads no declaration. It reports
// a file of the workspace without source code info under
// [MissingSourceInfo], and the file loads every declaration.
//
// Error modes: parse returns the error of a context that is done, the
// error of a member that does not read, and an error for bytes that are no
// FileDescriptorProto.
func parse(ctx context.Context, u *plugin.SourceUnit, reg *registry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref := u.Files()[0]
	b, err := u.Read(ref.Path)
	if err != nil {
		return err
	}
	fd, err := decode(u, b, reg)
	if err != nil {
		return fmt.Errorf("descriptor: decode %s: %w", ref.Path, err)
	}
	l := &lowered{unit: u, fd: fd, path: ref.Path, loc: newLocations(ref.Path, fd.GetSourceCodeInfo())}
	version, known := versionOf(fd)
	if !known {
		u.Errorf(
			frontend.UnknownEdition,
			l.loc.start(l.statement()),
			"the edition %s is outside the frontend's table of protobuf versions, so the frontend does not load the file",
			strings.TrimPrefix(fd.GetEdition().String(), editionPrefix),
		)
		return nil
	}
	l.version = version
	if fd.SourceCodeInfo == nil && !u.Dependency() {
		u.Errorf(MissingSourceInfo, l.loc.start(nil),
			"%s has no source code info, so its comments and directives do not load: "+
				"generate with source code info, which buf includes unless --exclude-source-info is set", fd.GetName())
	}
	l.file()
	return nil
}

// decode decodes a descriptor. Where an options message of the descriptor
// has fields that the decoding did not know, which are its custom options,
// decode decodes the descriptor again with the extensions that the files
// of [Registry] declare, which reg resolves.
//
// Error modes: the error of the decoding of the descriptor, of the read
// of the registry, of the decoding of the registry, and of the building of
// its files.
func decode(u *plugin.SourceUnit, b []byte, reg *registry) (*descriptorpb.FileDescriptorProto, error) {
	fd := &descriptorpb.FileDescriptorProto{}
	if err := proto.Unmarshal(b, fd); err != nil {
		return nil, fmt.Errorf("the descriptor: %w", err)
	}
	if !unknownOptions(fd.ProtoReflect()) {
		return fd, nil
	}
	content, err := registryFiles(u)
	if err != nil || content == nil {
		return fd, err
	}
	types, err := reg.typesOf(content)
	if err != nil {
		return nil, err
	}
	resolved := &descriptorpb.FileDescriptorProto{}
	if err = (proto.UnmarshalOptions{Resolver: types}).Unmarshal(b, resolved); err != nil {
		return nil, fmt.Errorf("the descriptor with the extensions of the registry: %w", err)
	}
	return resolved, nil
}

// unknownOptions reports whether an options message under m has fields
// that the decoding did not know.
func unknownOptions(m protoreflect.Message) bool {
	if strings.HasSuffix(string(m.Descriptor().Name()), "Options") && len(m.GetUnknown()) > 0 {
		return true
	}
	found := false
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind:
		case fd.IsList():
			list := v.List()
			for i := 0; i < list.Len() && !found; i++ {
				found = unknownOptions(list.Get(i).Message())
			}
		case !fd.IsMap():
			found = unknownOptions(v.Message())
		}
		return !found
	})
	return found
}

// versionOf returns the version of a file: the edition of a file of an
// edition, the syntax of a file that states proto2 or proto3, and proto2
// for a file without a syntax. It reports false for a version outside the
// table of the satellite.
func versionOf(fd *descriptorpb.FileDescriptorProto) (protobuf.Edition, bool) {
	switch syntax := fd.GetSyntax(); syntax {
	case "":
		return protobuf.EditionNamed(protobuf.Proto2)
	case syntaxEditions:
		e, known := protobuf.EditionNamed(strings.TrimPrefix(fd.GetEdition().String(), editionPrefix))
		return e, known && !e.Syntax
	default:
		e, known := protobuf.EditionNamed(syntax)
		return e, known && e.Syntax
	}
}

// lowered is the lowering of one file. It has the unit that it writes
// through, the descriptor that it reads, and the source code info of the
// descriptor.
type lowered struct {
	unit    *plugin.SourceUnit
	fd      *descriptorpb.FileDescriptorProto
	path    string
	loc     *locations
	version protobuf.Edition
	// pkg is the proto package of the file.
	pkg string
	// imports are the entries of the import stamp, one for each import.
	// paths are the paths of the imports that import types.
	imports []string
	paths   []string
	// consumed records the locations whose comments a declaration took.
	consumed map[string]bool
}

// statement returns the location of the syntax or edition statement of
// the file, and nil for a file without one.
func (l *lowered) statement() *descriptorpb.SourceCodeInfo_Location {
	if loc := l.loc.at([]int32{fileEdition}); loc != nil {
		return loc
	}
	return l.loc.at([]int32{fileSyntax})
}

// file lowers the whole descriptor: the package, the file, its imports,
// and then its declarations in source order. The comments of the syntax or
// edition statement are the comments of the file, and the comments of the
// package statement are the comments of the package.
func (l *lowered) file() {
	l.pkg = l.fd.GetPackage()
	gb := l.unit.Graph()
	pkg := gb.Package(l.pkg)
	if l.pkg != "" {
		pkg.Path = strings.Split(l.pkg, protobuf.NameSep)
		pkg.Name = pkg.Path[len(pkg.Path)-1]
	}
	file := &node.File{Path: l.path, Pos: l.loc.start(l.loc.at(nil))}
	pkg.Files = append(pkg.Files, file)

	// A descriptor of proto2 can leave its syntax empty, so the location
	// of the statement states that the file has one.
	stated := l.fd.GetSyntax() != "" || l.statement() != nil
	if loc := l.statement(); loc != nil {
		c := l.commentsOf(loc, file, false)
		file.Doc, file.Annotations = c.doc, append(file.Annotations, c.annotations...)
	}
	options := l.optionsAt(l.fd.GetOptions(), []int32{fileOptions})
	scope := with(l.version.Defaults, options)
	if loc := l.loc.at([]int32{filePackage}); loc != nil {
		c := l.commentsOf(loc, pkg, false)
		pkg.Doc = c.doc
		file.Annotations = append(file.Annotations, c.annotations...)
	}
	l.importsOf(file)
	file.Decls = l.decls(scope)

	gb.Scope(file, frontend.BindingsFor(l.pkg, l.paths...))
	if l.pkg != "" {
		gb.Stamp(file, meta.RawStamp{Key: protobuf.PackageKey, Value: l.pkg, Pos: file.Pos})
	}
	if stated {
		gb.Stamp(file, meta.RawStamp{Key: protobuf.SyntaxKey, Value: l.version.Name, Pos: file.Pos})
	}
	l.stampOptions(file, file.Pos, options)
	if len(l.imports) > 0 {
		gb.Stamp(file, meta.RawStamp{Key: protobuf.ImportKey, Value: l.imports, Pos: file.Pos})
	}
	l.sweep(file)
}

// importsOf lowers the imports of the file and records them for the import
// stamp. An import option imports the custom options of a file and none of
// its types, so the resolution step does not read it. An import has no
// identity, so a carrier on it reports as unaddressed.
func (l *lowered) importsOf(file *node.File) {
	public, weak := map[int32]bool{}, map[int32]bool{}
	for _, i := range l.fd.GetPublicDependency() {
		public[i] = true
	}
	for _, i := range l.fd.GetWeakDependency() {
		weak[i] = true
	}
	for i, dep := range l.fd.GetDependency() {
		switch n := int32(i); {
		case public[n]:
			l.imports = append(l.imports, importPublic+" "+dep)
		case weak[n]:
			l.imports = append(l.imports, importWeak+" "+dep)
		default:
			l.imports = append(l.imports, dep)
		}
		l.paths = append(l.paths, dep)
		file.Imports = append(file.Imports, l.importOf(dep, []int32{fileDependency, int32(i)}, file))
	}
	for i, dep := range l.fd.GetOptionDependency() {
		l.imports = append(l.imports, importOption+" "+dep)
		file.Imports = append(file.Imports, l.importOf(dep, []int32{fileOptionDependency, int32(i)}, file))
	}
}

// importOf lowers one import at its location. Its annotations are the
// annotations of the file.
func (l *lowered) importOf(dep string, path []int32, file *node.File) *node.Import {
	loc := l.loc.at(path)
	c := l.commentsOf(loc, nil, false)
	file.Annotations = append(file.Annotations, c.annotations...)
	return &node.Import{Pos: l.loc.start(loc), Path: dep, Doc: c.doc, Comment: c.comment}
}

// sweep reads the comments that no declaration took: the detached comments
// of every location, and the comments of the locations that the lowering
// does not read, such as an option statement. Their tool directives are
// annotations of the file. Their carriers are refused, so an authored
// directive above nothing is reported and not dropped.
func (l *lowered) sweep(file *node.File) {
	for _, loc := range l.fd.GetSourceCodeInfo().GetLocation() {
		at := l.loc.start(loc)
		texts := loc.GetLeadingDetachedComments()
		if !l.consumed[pathKey(loc.GetPath())] {
			texts = append(texts, loc.GetLeadingComments(), loc.GetTrailingComments())
		}
		for _, text := range texts {
			raw := rawComment(text)
			if raw == "" {
				continue
			}
			parts := l.unit.Comment(raw, at)
			file.Annotations = append(file.Annotations, parts.Annotations...)
			l.unit.RefuseCarriers(frontend.UnaddressedCarrier, parts.Carriers, unattributed)
		}
	}
}

// commented is the comments of one declaration: the documentation above
// it, the annotations above and beside it, and the comment beside it.
type commented struct {
	doc         []string
	annotations symbol.Annotations
	comment     string
}

// commentsOf splits the leading and the trailing comment of a location
// through the unit, and attaches their carriers to the subject. A nil
// subject is an import, whose carriers are refused. The leading comment is
// at the lines above the location. The trailing comment of a block, which
// protoc takes after the opening brace, is at the first line of the
// location, and the trailing comment of any other location is at its last
// line. A nil location has no comment.
func (l *lowered) commentsOf(loc *descriptorpb.SourceCodeInfo_Location, subject symbol.Symbol, block bool) commented {
	if loc == nil {
		return commented{}
	}
	if l.consumed == nil {
		l.consumed = map[string]bool{}
	}
	l.consumed[pathKey(loc.GetPath())] = true
	start := l.loc.start(loc)
	var lead, trail plugin.CommentParts
	if text := loc.GetLeadingComments(); text != "" {
		at := start
		at.Line, at.Col = max(start.Line-lineCount(text), 1), 1
		lead = l.unit.Comment(rawComment(text), at)
	}
	if text := loc.GetTrailingComments(); text != "" {
		at := start
		if !block {
			at.Line = endLine(loc)
		}
		trail = l.unit.Comment(rawComment(text), at)
	}
	l.carriers(subject, lead.Carriers)
	l.carriers(subject, trail.Carriers)
	return commented{
		doc:         lead.Docs,
		annotations: append(lead.Annotations, trail.Annotations...),
		comment:     strings.TrimSpace(strings.Join(trail.Docs, " ")),
	}
}

// carriers attaches every carrier to its subject, and reports a carrier
// that the grammar refuses under [frontend.BadCarrier]. A nil subject is
// an import, which has no identity, so carriers refuses every carrier on it
// under [frontend.UnaddressedCarrier].
func (l *lowered) carriers(subject symbol.Symbol, carriers []plugin.Carrier) {
	if subject == nil {
		l.unit.RefuseCarriers(frontend.UnaddressedCarrier, carriers, importSubject)
		return
	}
	l.unit.AttachCarriers(subject, carriers, frontend.BadCarrier)
}
