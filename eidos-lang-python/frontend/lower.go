// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"cmp"
	"context"
	"regexp"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The marks of the comments of the head of a module: the shebang of the
// first line, and the directives of mypy, pyright and ruff.
const (
	shebangMark = "#!"
	mypyMark    = "mypy:"
	pyrightMark = "pyright:"
	ruffMark    = "ruff:"
)

// encodingLines is the number of the first lines of a module whose comment
// can declare its encoding, as PEP 263 states.
const encodingLines = 2

// utf8Names are the spellings of UTF-8 in an encoding declaration, in lower
// case with the underscore as a dash.
var utf8Names = []string{"utf-8", "utf8"}

// encodingDeclaration matches the encoding declaration of PEP 263 in a
// comment, and its group is the name of the encoding.
var encodingDeclaration = regexp.MustCompile(`^[ \t\f]*#.*?coding[:=][ \t]*([-_.a-zA-Z0-9]+)`)

// The name of the list of the public names of a module, and the methods of
// a list that add names to it.
const (
	allName     = "__all__"
	extendName  = "extend"
	appendName  = "append"
	dunderMark  = "__"
	privateMark = "_"
)

// lowering is the state of the lowering of one file: the context of the
// parse, the unit, the vocabulary of the grammar, the tree of the file and
// its path, the place of its module, the File node and its package, the
// record of the scope of the module, and the comment walk of the file. It
// also has the target of the checks of conditions, the names of the __all__
// of the module where it states one, the type variables of the module, and
// the parts of the stamps of the file that the walk collects.
type lowering struct {
	ctx      context.Context
	u        *plugin.SourceUnit
	v        *vocabulary
	tree     *treesitter.Tree
	path     string
	at       place
	file     *node.File
	pkg      *node.Package
	scope    *scope
	comments treesitter.Comments
	minor    int
	platform string

	// all lists the names of __all__, and is nil for a module without it.
	// allNames are the bindings of the names in the order of the module.
	all      map[string]bool
	allNames []*node.Binding
	// declaring is the number of the statements of the module that can
	// declare a name, which the prescan counts.
	declaring int
	// typeVars are the type variables that the module declares, by name,
	// classKinds the kinds of the classes that it declares so far, and
	// aliases the names of the aliases that it declares so far. Each is nil
	// until its first entry.
	typeVars   map[string]*typeVar
	classKinds map[string]classKind
	aliases    map[string]bool
	// overloads are the names of the module whose declarations so far are
	// overloads, which the implementation of the name may follow. It is nil
	// until the first overload.
	overloads map[string]bool

	// The parts of the stamps of the file.
	conditions, lazy, future, head []string

	// shift maps the positions of the tree of a string annotation under
	// lowering to positions of the file, and is nil outside one.
	shift *shift
	// failed is the error of the first parse that found the context of the
	// lowering ended, which the parse of the unit returns.
	failed error
	// signatures reports that the unit loads at signature depth, which
	// leaves out every declaration that is not public.
	signatures bool
	// annotated reports that the type under lowering has an Annotated
	// inside another type.
	annotated bool
}

// newLowering returns the lowering of one file of a unit at its place.
func newLowering(ctx context.Context, u *plugin.SourceUnit, v *vocabulary, file string, at place, minor int,
	platform string,
) *lowering {
	return &lowering{
		ctx: ctx, u: u, v: v, path: file, at: at, scope: newScope(at.pkg), comments: v.comments,
		minor: minor, platform: platform, signatures: u.Depth() == plugin.DepthSignatures,
	}
}

// parsed parses source under the context of the lowering, calls lower with
// its tree, and closes the tree after the call. It parses the module of the
// unit and each string annotation of the module. A parse that finds the
// context ended calls nothing and records the error in failed.
func (l *lowering) parsed(source []byte, lower func(tree *treesitter.Tree)) {
	tree, err := l.v.grammar.Parse(l.ctx, l.path, source)
	if err != nil {
		l.failed = cmp.Or(l.failed, err)
		return
	}
	defer tree.Close()
	lower(tree)
}

// lower lowers the parsed tree of a module into the builder of the unit:
// the comments of its head, its docstring, the carriers of its header,
// every declaration of the branches that the checks of its conditions
// select, its __all__, the stamps of the file, the carriers that no
// declaration took, and the syntax errors.
func (l *lowering) lower(tree *treesitter.Tree) {
	l.tree = tree
	root := tree.Root()
	gb := l.u.Graph()
	l.pkg = gb.Package(l.at.pkg)
	l.file = &node.File{Path: l.path, Pos: root.Pos()}
	l.pkg.Files = append(l.pkg.Files, l.file)
	gb.Scope(l.file, l.scope)
	l.headComments(root)
	doc, docNode := l.docstring(root)
	l.file.Doc = doc
	if len(l.pkg.Doc) == 0 {
		l.pkg.Doc = doc
	}
	l.header(root, docNode)
	l.prescan(root)
	l.file.Decls = make([]symbol.Symbol, 0, l.declaring)
	l.scope.declared = make(map[string]bool, l.declaring)
	l.statements(root)
	if l.all != nil {
		l.file.Exports = append(l.file.Exports, l.allExport(root))
	}
	for _, s := range [...]struct {
		key  meta.KeyName
		list []string
	}{
		{python.ConditionKey, l.conditions},
		{python.LazyKey, l.lazy},
		{python.FutureKey, l.future},
		{python.HeadKey, l.head},
	} {
		if len(s.list) > 0 {
			gb.Stamp(l.file, meta.RawStamp{Key: s.key, Value: s.list, Pos: l.file.Pos})
		}
	}
	l.comments.Sweep(root, func(n treesitter.Node) {
		l.u.RefuseCarriers(UnaddressedCarrier, l.u.Comment(n.Text(), n.Pos()).Carriers, unattached)
	})
	if problems := l.tree.Problems(); problems != nil {
		l.u.ReportSyntax(UnparsedFile, problems)
	}
}

// headComments takes the comments of the head of a module before its first
// statement into the stamp python.head, each without its #: the shebang of
// the first line, the encoding declaration of the first two lines, the
// comment # type: ignore, and the directives of mypy, pyright and ruff. It
// reports an encoding declaration of an encoding other than UTF-8 under
// [ForeignEncoding].
func (l *lowering) headComments(root treesitter.Node) {
	for n := range root.NamedChildren() {
		if n.Kind() != l.v.comment {
			return
		}
		text := n.Text()
		body := strings.TrimLeft(strings.TrimPrefix(text, commentMark), blanks)
		line := n.Pos().Line
		match := encodingDeclaration.FindStringSubmatch(text)
		switch {
		case line == 1 && strings.HasPrefix(text, shebangMark):
		case line <= encodingLines && match != nil:
			name := strings.ReplaceAll(strings.ToLower(match[1]), "_", "-")
			if !strings.HasPrefix(name, utf8Names[0]) && name != utf8Names[1] {
				l.u.Warnf(
					ForeignEncoding,
					n.Pos(),
					"the file declares the encoding %s, and the frontend reads it as UTF-8",
					match[1],
				)
			}
		case strings.HasPrefix(body, typeCommentMark) &&
			strings.HasPrefix(strings.TrimLeft(strings.TrimPrefix(body, typeCommentMark), blanks), ignoreDirective),
			strings.HasPrefix(body, mypyMark), strings.HasPrefix(body, pyrightMark), strings.HasPrefix(body, ruffMark):
		default:
			continue
		}
		l.head = append(l.head, strings.TrimPrefix(text, commentMark))
		l.comments.Take(n)
	}
}

// header attaches the carriers of the header of a module to its package.
// The header is every comment above the first statement of the module that
// is no comment of the head. The run directly above a first statement that
// declares belongs to that statement, so the header leaves it out. A
// carrier above the docstring or an import is part of the header.
func (l *lowering) header(root, doc treesitter.Node) {
	var comments, run []treesitter.Node
	for n := range root.NamedChildren() {
		if n.Kind() != l.v.comment {
			if n != doc && !l.importing(n) {
				run = l.comments.Leading(n)
			}
			break
		}
		comments = append(comments, n)
	}
	comments = comments[:len(comments)-len(run)]
	l.u.AttachCarriers(l.pkg, l.parts(comments).Carriers, BadCarrier)
}

// importing reports whether a statement imports, which no rule takes as its
// subject.
func (l *lowering) importing(n treesitter.Node) bool {
	k := n.Kind()
	return k == l.v.importStatement || k == l.v.importFromStatement || k == l.v.futureImportStatement ||
		k == l.v.lazyImportStatement
}

// statements lowers the statements of one block of a module: the imports,
// the declarations, and the blocks of the compound statements that the
// checks of conditions select. A statement that declares nothing refuses
// its carriers.
func (l *lowering) statements(list treesitter.Node) {
	for stmt := range list.Content() {
		switch k := stmt.Kind(); k {
		case l.v.futureImportStatement:
			l.futureImport(stmt)
		case l.v.importStatement, l.v.importFromStatement:
			l.importStatement(stmt, stmt, false)
		case l.v.lazyImportStatement:
			l.importStatement(firstNamed(stmt), stmt, true)
		case l.v.classDefinition, l.v.functionDefinition, l.v.decoratedDefinition:
			if decl := l.definition(stmt, nil); decl != nil {
				l.file.Decls = append(l.file.Decls, decl)
			}
		case l.v.assignment:
			l.moduleAssignment(stmt)
		case l.v.typeAliasStatement:
			l.typeAlias(stmt)
		case l.v.ifStatement:
			l.branches(stmt, l.statements)
		case l.v.tryStatement, l.v.withStatement, l.v.forStatement, l.v.whileStatement, l.v.matchStatement:
			l.allBodies(stmt, l.statements)
		case l.v.stringNode, l.v.concatenatedString, l.v.passStatement, l.v.ellipsis:
		default:
			l.refuse(stmt, statementRefusal)
		}
	}
}

// declare records a name of a declaration at the top level of the module,
// and reports false with a finding under [DuplicateName] for a name that
// the module declares already. The first declaration of a name is its
// declaration. An overload of a name, and the implementation after its
// overloads, declare one name.
func (l *lowering) declare(name string, at position.Pos, overload bool) bool {
	switch {
	case !l.scope.declared[name]:
		l.scope.declared[name] = true
		setFlag(&l.overloads, name, overload)
		return true
	case l.overloads[name]:
		setFlag(&l.overloads, name, overload)
		return true
	default:
		l.u.Infof(DuplicateName, at, "the module declares %s a second time, and the first declaration is kept", name)
		return false
	}
}

// claim declares the name of a class or an alias in the module, or in the
// class body host where it is not nil, and returns the visibility of the
// name. It reports false for a second declaration of the name, which
// reports under [DuplicateName], and for a declaration that the signature
// depth leaves out.
func (l *lowering) claim(name string, at position.Pos, host *classBody) (symbol.Visibility, bool) {
	if host == nil {
		vis := l.moduleVisibility(name)
		return vis, l.declare(name, at, false) && (!l.signatures || vis == symbol.VisibilityPublic)
	}
	vis := memberVisibility(name)
	return vis, host.declare(l, name, at, false, false) && (!l.signatures || vis == symbol.VisibilityPublic)
}

// moduleVisibility returns the visibility of a name at the top level of a
// module: package for a name with one leading underscore and for a name
// that __all__ does not list where the module states __all__, and public
// otherwise. A name that starts and ends with two underscores is public.
func (l *lowering) moduleVisibility(name string) symbol.Visibility {
	switch {
	case l.all != nil && !l.all[name]:
		return symbol.VisibilityPackage
	case l.all != nil, dunder(name):
		return symbol.VisibilityPublic
	case strings.HasPrefix(name, privateMark):
		return symbol.VisibilityPackage
	default:
		return symbol.VisibilityPublic
	}
}

// memberVisibility returns the visibility of a member of a class: private
// for a name that starts with two underscores and does not end with two,
// which Python mangles, protected for a name with one leading underscore,
// and public otherwise.
func memberVisibility(name string) symbol.Visibility {
	switch {
	case dunder(name):
		return symbol.VisibilityPublic
	case strings.HasPrefix(name, dunderMark):
		return symbol.VisibilityPrivate
	case strings.HasPrefix(name, privateMark):
		return symbol.VisibilityProtected
	default:
		return symbol.VisibilityPublic
	}
}

// dunder reports whether a name starts and ends with two underscores, as
// __init__ does.
func dunder(name string) bool {
	return len(name) > 2*len(dunderMark) && strings.HasPrefix(name, dunderMark) && strings.HasSuffix(name, dunderMark)
}

// setFlag sets the flag of a name in a set of names that is nil until its
// first name: on adds the name, and makes the set where it is nil, and
// false removes the name. A set that no name enters allocates nothing.
func setFlag(set *map[string]bool, name string, on bool) {
	if !on {
		delete(*set, name)
		return
	}
	if *set == nil {
		*set = map[string]bool{}
	}
	(*set)[name] = true
}

// firstNamed returns the first named child of a node that is not a
// comment, and the zero Node where there is none.
func firstNamed(n treesitter.Node) treesitter.Node {
	for child := range n.Content() {
		return child
	}
	return treesitter.Node{}
}
