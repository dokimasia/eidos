// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package nodetest

import (
	"context"
	"fmt"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// frontendName is the name of the fixture frontend.
const frontendName plugin.ID = "nodetest"

// The version that every unit key of the fixture frontend folds, the glob
// that claims every file of the in-memory tree, and the comment form of the
// frontend. No file of the tree contains a comment.
const (
	frontendVersion = "1"
	claimAll        = "**"
	lineComment     = "//"
)

// brand is the brand that Build loads a fixture under.
const brand output.Brand = "nodetest"

// The names of the fields that the model generator writes into the kinds
// of the node model, which Build reads and writes through reflection, so a
// kind that the schema adds needs no change here.
const (
	posField  = "Pos"
	nameField = "Name"
)

// pathSep separates the segments of a package path.
const pathSep = "/"

// Builder collects the packages and the files of one fixture in one
// language, and builds them through the kernel's load. The zero Builder is
// not usable, so a test calls [New].
//
// # Concurrency
//
// A Builder is not safe for concurrent use. A test that builds two
// fixtures in parallel uses one Builder for each.
type Builder struct {
	lang Language
	// packages are the packages of the fixture in the order of the first
	// call of Package for each, paths are their paths in the same order,
	// and byPath maps each path to its package.
	packages []*node.Package
	paths    []string
	byPath   map[string]*node.Package
	// files maps each path that File created to its file, and homes maps
	// it to the path of the package that File placed it in.
	files map[string]*node.File
	homes map[string]string
	// claimed lists each file path that a call of File asked for in a
	// second package, once.
	claimed []string
	// targets maps each reference that Ref returned to its target.
	targets map[*node.TypeRef]node.Declaration
	// built is what the first call of Build returned, and done reports
	// that Build ran.
	built []*node.Package
	done  bool
}

// New returns an empty builder for a fixture in the language of l.
func New(l Language) *Builder {
	return &Builder{
		lang:    l,
		byPath:  map[string]*node.Package{},
		files:   map[string]*node.File{},
		homes:   map[string]string{},
		targets: map[*node.TypeRef]node.Declaration{},
	}
}

// Package returns the package at the slash-separated path, and creates it
// on the first call for the path. A created package has the segments of the
// path and the last segment as its name. The empty path is the package of
// the global scope, without segments and without a name. A test can change
// any field of the package before Build.
func (b *Builder) Package(path string) *node.Package {
	if p, held := b.byPath[path]; held {
		return p
	}
	p := &node.Package{}
	if path != "" {
		p.Path = strings.Split(path, pathSep)
		p.Name = p.Path[len(p.Path)-1]
	}
	b.byPath[path] = p
	b.packages = append(b.packages, p)
	b.paths = append(b.paths, path)
	return p
}

// File returns the file at the slash-separated workspace path in the
// package at pkg, and creates the file and the package on the first call.
// It appends decls to the declarations of the file. A call for a path that
// an earlier call placed in another package returns that file, and Build
// then fails. A test can change any field of the file before Build.
func (b *Builder) File(pkg, path string, decls ...symbol.Symbol) *node.File {
	f, held := b.files[path]
	if !held {
		f = &node.File{Path: path}
		p := b.Package(pkg)
		p.Files = append(p.Files, f)
		b.files[path] = f
		b.homes[path] = pkg
	} else if b.homes[path] != pkg && !slices.Contains(b.claimed, path) {
		b.claimed = append(b.claimed, path)
	}
	f.Decls = append(f.Decls, decls...)
	return f
}

// Ref returns a named reference to target, with args as its type
// arguments. Build spells it, and the load gives it the identity of target
// as its target. target is a type declaration or a variant of the fixture,
// and Build fails where the load does not resolve the reference to it. The
// reference keeps args as its arguments.
func (b *Builder) Ref(target node.Declaration, args ...*node.TypeRef) *node.TypeRef {
	ref := &node.TypeRef{Args: args}
	b.targets[ref] = target
	return ref
}

// Build loads the fixture through the kernel's load, and returns the
// packages of the load's graph in identity order. It takes these steps:
//
//   - It positions each file at line 1 of its path, and each declaration
//     that [node.Walk] visits in the file at the next line, column 1,
//     where the test left the position zero. A type reference has the
//     position of the declaration that contains it, where the test left it
//     zero.
//   - It spells each reference that [Builder.Ref] returned as the name of
//     its target, after the names of the target's enclosing types, each
//     followed by a dot. A reference from another package gets that name
//     through [Language.Qualify], and the path of the target's package as
//     its Package. A reference whose spelling the test set keeps it.
//   - It spells each structural reference with an empty spelling through
//     [Language.Spell], children before parents.
//   - It loads the files through the kernel's load, under the context of
//     tb, with an in-memory tree that has an empty file at the path of each
//     file, and the fixture frontend as the one frontend.
//
// After Build, every file and declaration of the fixture has the identity
// that the load assigned to it, every member has its host, and every
// reference that Ref returned has its target. Each package that
// [Builder.Package] returned has the identity of its returned copy. The
// returned packages are the load's copies, so a change to a declaration
// after Build changes nothing that Build returned. A second call returns
// the same packages, and does not load again.
//
// Build fails the test, through an assertion, for each of these causes:
//
//   - The language returns an empty identity.
//   - A file path belongs to two packages, two files have one path, or a
//     path is not a valid path of an [fs.FS].
//   - One declaration or one reference appears in two places.
//   - A reference from Ref targets a declaration that no file of the
//     fixture contains, or the load does not resolve the reference to its
//     target, such as for a target of a kind that a reference cannot name.
//   - One file spells two targets alike.
//   - The language returns an error for a form.
//   - The load returns an error, reports a finding or panics, such as on a
//     declaration without a name.
func (b *Builder) Build(tb assert.TB) []*node.Package {
	tb.Helper()

	if b.done {
		return b.built
	}
	b.done = true
	assert.NotEmpty(tb, b.lang.Lang(), "the language of the fixture has an identity")
	assert.Empty(tb, b.claimed, "no file path belongs to two packages")
	s := b.survey(tb)
	s.position()
	b.link(tb, s)
	b.spell(tb, s)
	pkgs := slices.Collect(b.load(tb, s).Packages())
	for _, p := range pkgs {
		if mine, held := b.byPath[strings.Join(p.Path, pathSep)]; held {
			mine.ID = p.ID
		}
	}
	for _, f := range s.files {
		for _, ref := range s.refs[f] {
			if target, linked := b.targets[ref]; linked {
				assert.Equal(tb, ref.Target, target.Identity(), "the load resolves the reference to "+
					target.Kind().String()+" "+nameOf(target)+" to the identity of the declaration")
			}
		}
	}
	b.built = pkgs
	return pkgs
}

// survey is what one walk of a fixture finds: its files in order, the
// package of each file, the file of each declaration and each reference,
// the references of each file in walk order, the enclosing type names of
// each type declaration and variant, and the targets of the spellings of
// each file.
type survey struct {
	files    []*node.File
	home     map[*node.File]int
	fileOf   map[symbol.Symbol]*node.File
	refs     map[*node.File][]*node.TypeRef
	owners   map[symbol.Symbol]string
	bindings map[*node.File]map[string]node.Declaration
}

// survey walks every file of the fixture, in the order of the packages and
// of the files of each, and fails the test for a path that two files have,
// for a path that is not valid in an [fs.FS], and for a declaration or a
// reference that appears in two places.
func (b *Builder) survey(tb assert.TB) *survey {
	tb.Helper()

	s := &survey{
		home:     map[*node.File]int{},
		fileOf:   map[symbol.Symbol]*node.File{},
		refs:     map[*node.File][]*node.TypeRef{},
		owners:   map[symbol.Symbol]string{},
		bindings: map[*node.File]map[string]node.Declaration{},
	}
	var repeated, invalid, twice []string
	paths := map[string]bool{}
	for i, p := range b.packages {
		for _, f := range p.Files {
			if paths[f.Path] {
				repeated = append(repeated, f.Path)
			}
			if !fs.ValidPath(f.Path) {
				invalid = append(invalid, f.Path)
			}
			paths[f.Path] = true
			s.home[f] = i
			s.files = append(s.files, f)
			for _, d := range f.Decls {
				s.enclose(d, "")
			}
			node.Walk(f, func(n symbol.Symbol) bool {
				if n == symbol.Symbol(f) {
					return true
				}
				ref, isRef := n.(*node.TypeRef)
				if _, met := s.fileOf[n]; met {
					label := nameOf(n)
					if isRef {
						label = ref.Spelling
					}
					twice = append(twice, n.Kind().String()+" "+label)
					return false
				}
				s.fileOf[n] = f
				if isRef {
					s.refs[f] = append(s.refs[f], ref)
				}
				return true
			})
		}
	}
	assert.Empty(tb, repeated, "no two files of the fixture have one path")
	assert.Empty(tb, invalid, "every file path of the fixture is a valid path of a file system")
	assert.Empty(tb, twice, "no declaration or reference appears in two places of the fixture")
	return s
}

// enclose records owner as the enclosing type names of d where d is a type
// declaration or a variant, and descends into the types and the variants
// that d declares, which d encloses.
func (s *survey) enclose(d symbol.Symbol, owner string) {
	switch x := d.(type) {
	case *node.Struct:
		s.owners[x] = owner
		for _, inner := range x.Types {
			s.enclose(inner, chained(owner, x.Name))
		}
	case *node.Interface:
		s.owners[x] = owner
		for _, inner := range x.Types {
			s.enclose(inner, chained(owner, x.Name))
		}
	case *node.Enum:
		s.owners[x] = owner
		for _, v := range x.Variants {
			s.owners[v] = chained(owner, x.Name)
		}
	case *node.Sum:
		s.owners[x] = owner
		for _, v := range x.Variants {
			s.owners[v] = chained(owner, x.Name)
		}
	case *node.Alias:
		s.owners[x] = owner
	}
}

// position gives each file its position at line 1, each declaration the
// next line of its file, and each type reference the position of the
// declaration that contains it, wherever the test left a position zero.
// The walk of a declaration positions its own references and stops at a
// nested declaration, which positions its own.
func (s *survey) position() {
	for _, f := range s.files {
		placed(f, position.Pos{File: f.Path, Line: 1, Col: 1})
		line := 1
		node.Walk(f, func(n symbol.Symbol) bool {
			if _, isRef := n.(*node.TypeRef); isRef || n == symbol.Symbol(f) {
				return true
			}
			line++
			at := placed(n, position.Pos{File: f.Path, Line: line, Col: 1})
			node.Walk(n, func(inner symbol.Symbol) bool {
				if inner == n {
					return true
				}
				ref, isRef := inner.(*node.TypeRef)
				if isRef && ref.Pos.IsZero() {
					ref.Pos = at
				}
				return isRef
			})
			return true
		})
	}
}

// link spells each reference that [Builder.Ref] returned and records its
// target under its spelling in the bindings of its file. It fails the test
// for a target outside the fixture and for one spelling of two targets in
// one file.
func (b *Builder) link(tb assert.TB, s *survey) {
	tb.Helper()

	for _, f := range s.files {
		for _, ref := range s.refs[f] {
			target, linked := b.targets[ref]
			if !linked {
				continue
			}
			assert.Contains(tb, s.fileOf, symbol.Symbol(target), "the reference to "+target.Kind().String()+" "+
				nameOf(target)+" targets a declaration of the fixture", assert.ByIdentity())
			home := s.fileOf[target]
			if ref.Spelling == "" {
				name := chained(s.owners[target], nameOf(target))
				if s.home[home] != s.home[f] {
					name = b.lang.Qualify(b.packages[s.home[home]], name)
					ref.Package = b.paths[s.home[home]]
				}
				ref.Spelling = name
			}
			bindings := s.bindings[f]
			if bindings == nil {
				bindings = map[string]node.Declaration{}
				s.bindings[f] = bindings
			}
			earlier, met := bindings[ref.Spelling]
			assert.False(tb, met && earlier != target,
				"the spelling "+ref.Spelling+" names one target in "+f.Path)
			bindings[ref.Spelling] = target
		}
	}
}

// spell gives each structural reference with an empty spelling the
// spelling that the language returns. It visits the references of a file
// in the reverse of their walk order, so each child has its spelling before
// its parent. It fails the test for a form that the language refuses.
func (b *Builder) spell(tb assert.TB, s *survey) {
	tb.Helper()

	for _, f := range s.files {
		for _, ref := range slices.Backward(s.refs[f]) {
			if ref.Spelling != "" || ref.Form == symbol.FormNamed {
				continue
			}
			spelling, err := b.lang.Spell(ref)
			assert.NoError(tb, err, "the language spells the reference at "+ref.Pos.String())
			ref.Spelling = spelling
		}
	}
}

// load runs the kernel's load over the files of the fixture, through the
// fixture frontend, and returns the sealed graph. It fails the test for an
// error, a finding and a panic of the load.
func (b *Builder) load(tb assert.TB, s *survey) *store.Graph {
	tb.Helper()

	tree := make(fstest.MapFS, len(s.files))
	homes := make(map[string]int, len(s.files))
	for _, f := range s.files {
		tree[f.Path] = &fstest.MapFile{}
		homes[f.Path] = s.home[f]
	}
	front := &frontend{lang: b.lang, packages: b.packages, paths: b.paths, homes: homes, bindings: s.bindings}
	sink := diag.NewSink()
	g, err := loaded(assert.Context(tb), tree, front, sink)
	assert.NoError(tb, err, "the load builds the fixture")
	assert.Empty(tb, slices.Collect(sink.All()), "the load of the fixture reports no finding")
	return g
}

// frontend serves the files of a fixture to the load. Each package of the
// fixture is one unit, its parse hands the load the files that the test
// built, and its resolution returns the target that a reference from
// [Builder.Ref] names.
//
// # Concurrency
//
// A frontend is safe for concurrent use. The load calls Parse and Resolve
// from its workers, and both read state that Build wrote before the load.
type frontend struct {
	lang Language
	// packages and paths are the packages of the fixture and their paths,
	// in the order of the builder, and homes maps each file path to the
	// index of its package.
	packages []*node.Package
	paths    []string
	homes    map[string]int
	// bindings maps each file to the targets of its spellings.
	bindings map[*node.File]map[string]node.Declaration
}

// frontend is the frontend of a load of a fixture.
var _ plugin.Frontend = (*frontend)(nil)

// Version returns the version of the fixture frontend.
func (*frontend) Version() string { return frontendVersion }

// Name returns the name of the fixture frontend.
func (*frontend) Name() plugin.ID { return frontendName }

// Lang returns the language of the fixture.
func (f *frontend) Lang() symbol.Lang { return f.lang.Lang() }

// Syntax returns the one line comment form, which no file of the fixture
// contains.
func (*frontend) Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{lineComment}}
}

// Overloads reports whether the language of the fixture overloads.
func (f *frontend) Overloads() bool { return f.lang.Overloads() }

// Selection claims every file of the in-memory tree, which contains the
// files of the fixture alone.
func (*frontend) Selection() []string { return []string{claimAll} }

// Partition returns one unit for each package of the fixture that has a
// file, in the order of the packages, with the selected files of the
// package.
//
// Error modes: a selected file that the fixture does not contain returns
// an error, because the tree of the load has no other file.
func (f *frontend) Partition(
	_ context.Context, files []plugin.SourceRef, _ plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	units := make([][]plugin.SourceRef, len(f.packages))
	for _, ref := range files {
		i, held := f.homes[ref.Path]
		if !held {
			return nil, fmt.Errorf("nodetest: %s is not a file of the fixture", ref.Path)
		}
		units[i] = append(units[i], ref)
	}
	return slices.DeleteFunc(units, func(u []plugin.SourceRef) bool { return len(u) == 0 }), nil
}

// Parse hands the load the package of the unit: its name, documentation
// and position, and every file of the package as the test built it. It
// records the targets of the spellings of each file as the scope of the
// file. It returns no error.
func (f *frontend) Parse(_ context.Context, u *plugin.SourceUnit) error {
	i := f.homes[u.Files()[0].Path]
	src, gb := f.packages[i], u.Graph()
	p := gb.Package(f.paths[i])
	p.Name, p.Doc, p.Pos = src.Name, src.Doc, src.Pos
	p.Files = append(p.Files, src.Files...)
	for _, file := range src.Files {
		gb.Scope(file, f.bindings[file])
	}
	return nil
}

// Resolve returns the identity that the load assigned to the target of
// the spelling in the file of the scope, and nil for a spelling without a
// target and for a target that the load did not identify.
func (*frontend) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	bindings, _ := scope.Bindings.(map[string]node.Declaration)
	target, held := bindings[spelling]
	if !held {
		return nil
	}
	id := target.Identity()
	if id.IsZero() {
		return nil
	}
	return plugin.Candidates{{id}}
}

// loaded runs the kernel's load and returns a panic of the load as an
// error. The assignment panics on a structural defect, such as a
// declaration without a name, on the goroutine of the load.
func loaded(ctx context.Context, tree fs.FS, front plugin.Frontend, sink *diag.Sink) (g *store.Graph, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("nodetest: the load panics: %v", r)
		}
	}()
	g, _, err = load.Load(ctx, load.Config{
		FS:        tree,
		Frontends: []plugin.Frontend{front},
		Sink:      sink,
		Brand:     brand,
	})
	return g, err
}

// placed sets the position of a node of the model to at where the node has
// none, and returns the position of the node. A symbol without a position
// field, such as a tag entry or a symbol of another model, returns at.
func placed(s symbol.Symbol, at position.Pos) position.Pos {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return at
	}
	field := v.Elem().FieldByName(posField)
	if !field.IsValid() || !field.CanSet() {
		return at
	}
	pos, is := reflect.TypeAssert[position.Pos](field)
	if !is {
		return at
	}
	if !pos.IsZero() {
		return pos
	}
	field.Set(reflect.ValueOf(at))
	return at
}

// nameOf returns the name of a declaration of the model, and the empty
// string for a kind without a name, such as an embed.
func nameOf(s symbol.Symbol) string {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return ""
	}
	field := v.Elem().FieldByName(nameField)
	if field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

// chained returns name after the enclosing type names owner and a dot, and
// name alone at the top level.
func chained(owner, name string) string {
	if owner == "" {
		return name
	}
	return owner + symbol.OwnerSep + name
}
