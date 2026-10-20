// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/dokimasia/gqlparser/v2/ast"
	"github.com/dokimasia/gqlparser/v2/lexer"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/internal/document"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// descriptionSep splits a description into the lines of the documentation
// of its declaration.
const descriptionSep = "\n"

// declared is the declaration of one type of the unit and the kind of the
// GraphQL definition that it lowers from. An object type and an input object
// both lower into a struct, so the kind tells their extensions apart.
type declared struct {
	decl symbol.Symbol
	kind ast.DefinitionKind
}

// lowering is the lowering of one unit. It writes the declarations of the
// own files of the unit into the graph builder of the unit: the type system
// into the package of the schema, and the executable definitions into the
// package of the documents. types are the declarations of the type system
// of the unit by name, which an extension of the unit merges into, and
// directives are the interfaces of the directive definitions of the unit by
// name, which an extension of a directive merges into.
type lowering struct {
	unit       *plugin.SourceUnit
	gb         *plugin.GraphBuilder
	pkg        *node.Package
	docs       *node.Package
	types      map[string]declared
	directives map[string]*node.Interface
	// bindings are the resolution records of the files of the schema and of
	// the documents, which every file of the unit shares.
	schemaScope, docsScope *bindings
}

// newLowering returns the lowering of a unit whose first own file is at
// first. The package of the unit is the package of the schema that claims
// the file, and the package of its directory where no schema claims it. The
// shared packages are the packages of the units of the shared files, in the
// order of the files.
func newLowering(u *plugin.SourceUnit, w workspace, first string, sharedFiles []string) *lowering {
	pkgPath := w.packageOf(first)
	var shared []string
	for _, f := range sharedFiles {
		if pkg := w.packageOf(f); pkg != pkgPath && !slices.Contains(shared, pkg) {
			shared = append(shared, pkg)
		}
	}
	gb := u.Graph()
	l := &lowering{unit: u, gb: gb, types: map[string]declared{}, directives: map[string]*node.Interface{}}
	l.pkg = gb.Package(pkgPath)
	if pkgPath != "" {
		l.pkg.Name = path.Base(pkgPath)
	}
	extended := map[string]bool{}
	l.schemaScope = &bindings{schema: pkgPath, shared: shared, extended: extended}
	l.docsScope = &bindings{
		documents: path.Join(pkgPath, graphql.DocumentsSegment), schema: pkgPath, shared: shared, extended: extended,
	}
	return l
}

// newSource returns the lowering of the own file at p of the unit, with its
// tokens and its definitions.
func (l *lowering) newSource(p string, lx *document.Lexed, defs []document.Definition) *source {
	return &source{
		lowering: l, path: p, lexed: lx, defs: defs,
		notes: &comments{l: lx, unit: l.unit, path: p, taken: map[int]bool{}},
	}
}

// member is the first and the last token of one member of a body, as
// indices in Sig.
type member struct{ first, last int }

// source is one own file of a unit while it lowers: its path, its tokens,
// its top-level definitions and the attribution of its comments. file is
// the node of the file in the package of the schema, and docFile its node
// in the package of the documents. A file has the second node where it has
// an executable definition.
type source struct {
	*lowering
	path    string
	lexed   *document.Lexed
	defs    []document.Definition
	notes   *comments
	file    *node.File
	docFile *node.File
}

// at returns the position of a node of gqlparser in the file. The line and
// the column count from 1, and the column counts runes. The parser gives
// every node that it parses a position.
func (s *source) at(p *ast.Position) position.Pos {
	return position.Pos{File: s.path, Line: p.Line, Col: p.Column}
}

// head returns the position of the first token of the file, and the start
// of the file for a file without a token.
func (s *source) head() position.Pos {
	if len(s.lexed.Sig) == 0 {
		return position.Pos{File: s.path, Line: 1, Col: 1}
	}
	tok := s.lexed.At(0)
	return position.Pos{File: s.path, Line: tok.Pos.Line, Col: tok.Pos.Column}
}

// schemaFile returns the node of the file in the package of the schema, and
// creates it on the first call.
func (s *source) schemaFile() *node.File {
	if s.file == nil {
		s.file = &node.File{Path: s.path, Pos: s.head()}
		s.pkg.Files = append(s.pkg.Files, s.file)
		s.gb.Scope(s.file, s.schemaScope)
	}
	return s.file
}

// documentsFile returns the node of the file in the package of the
// documents, and creates the node and the package on the first call.
func (s *source) documentsFile() *node.File {
	if s.docFile == nil {
		if s.docs == nil {
			s.docs = s.gb.Package(s.docsScope.documents)
			s.docs.Name = path.Base(s.docsScope.documents)
		}
		s.docFile = &node.File{Path: s.path, Pos: s.head()}
		s.docs.Files = append(s.docs.Files, s.docFile)
		s.gb.Scope(s.docFile, s.docsScope)
	}
	return s.docFile
}

// definitionAt returns the top-level definition that contains the token at
// the rune offset of p. A node of the parser is at a token of the file, so
// the file has a definition for every node.
func (s *source) definitionAt(p *ast.Position) document.Definition {
	k := s.tokenAt(p)
	i, _ := slices.BinarySearchFunc(s.defs, k, func(d document.Definition, k int) int {
		return cmp.Compare(d.First, k+1)
	})
	return s.defs[max(i-1, 0)]
}

// tokenAt returns the index in [document.Lexed.Sig] of the token that starts
// at the rune offset of p, or of the first token after it where none starts
// there.
func (s *source) tokenAt(p *ast.Position) int {
	k, _ := s.lexed.Find(p.Start)
	return k
}

// body returns the index in Sig of the token whose trailing comment is the
// comment of the definition d: the opening brace of its body, and its last
// token where it has no body.
func (s *source) body(d document.Definition) int {
	if s.lexed.At(d.Last).Kind == lexer.BraceR {
		if open := s.lexed.Pair[d.Last]; open >= 0 {
			return open
		}
	}
	return d.Last
}

// memberAt returns the bounds of the member at i of the n members of a body.
// nameAt returns the position of the name of a member, and described reports
// whether a member has a description before its name. end is the index in
// Sig of the closing bracket of the body. A member ends before the first
// token of the next member, and the last member ends before the closing
// bracket. memberAt allocates nothing.
func (s *source) memberAt(i, n int, nameAt func(int) *ast.Position, described func(int) bool, end int) member {
	first := s.memberStart(i, nameAt, described)
	next := end
	if i+1 < n {
		next = s.memberStart(i+1, nameAt, described)
	}
	return member{first: first, last: max(next-1, first)}
}

// memberStart returns the index in Sig of the first token of the member at
// i: its description, or its name where it has none.
func (s *source) memberStart(i int, nameAt func(int) *ast.Position, described func(int) bool) int {
	k := s.tokenAt(nameAt(i))
	if described(i) && k > 0 {
		if kind := s.lexed.At(k - 1).Kind; kind == lexer.String || kind == lexer.BlockString {
			k--
		}
	}
	return k
}

// argumentsEnd returns the index in Sig of the closing parenthesis of the
// arguments after the name at p, and the index of the token after the name
// where the name has no arguments.
func (s *source) argumentsEnd(p *ast.Position) int {
	k := s.tokenAt(p) + 1
	if s.lexed.At(k).Kind != lexer.ParenL {
		return k
	}
	return s.lexed.Pair[k]
}

// docOf returns the documentation lines of a description through the kit.
func (s *source) docOf(description string) []string {
	if description == "" {
		return nil
	}
	return s.unit.DocLines(strings.Split(description, descriptionSep))
}

// sortDecls orders the declarations of each file of the unit by their
// positions, because gqlparser keeps the definitions of a file in lists by
// their kind.
func sortDecls(files []*node.File) {
	for _, f := range files {
		slices.SortStableFunc(f.Decls, func(a, b symbol.Symbol) int {
			pa, pb := a.Position(), b.Position()
			return cmp.Or(cmp.Compare(pa.Line, pb.Line), cmp.Compare(pa.Col, pb.Col))
		})
	}
}
