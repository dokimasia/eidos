// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"runtime"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// manifest is the part of a package.json that the frontend reads: the name
// of the package.
type manifest struct {
	Name string `json:"name"`
}

// workspaceGlobals is the global scope of a source file without a library:
// the global package of the workspace alone. Every such file shares the
// list, which no reader writes.
var workspaceGlobals = []string{""}

// parse loads one unit into the builder of the unit. A unit of the library
// store is a library, which [tsFrontend.parseLibrary] loads. Any other unit
// is a source file, which parses through its grammar. A syntax error is a
// problem of the source: every ERROR and MISSING node reports at its
// position, and every declaration that the parser recovered loads. The
// tsconfig chain of the member, among its shared inputs, states how its
// non-relative specifiers resolve. A chain that does not read whole reports
// under [BadConfig], and the files that it governs resolve through neither a
// baseUrl nor paths. Every package of the unit has the module of the
// package.json of the member. The global scope of the file is the global
// package of the workspace and the library of the file, as [globalsOf]
// states it. After a member with a library lowers, the unit records its
// uses, as [lowering.recordUses] states them.
func (f *tsFrontend) parse(ctx context.Context, u *plugin.SourceUnit) error {
	if key, _, isLibrary := libraryPath(u.Files()[0].Path); isLibrary {
		return f.parseLibrary(ctx, u, key)
	}
	for _, ref := range u.Files() {
		src, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		v := f.vocabularyOf(ref.Path)
		tree, err := v.grammar.Parse(ctx, ref.Path, src)
		if err != nil {
			return err
		}
		globals := globalsOf(u, ref, src)
		l := &lowering{
			u: u, v: v, tree: tree, path: ref.Path,
			module:   newModule(ref.Path, resolutionOf(u, ref), globals),
			files:    map[string]fileScope{},
			comments: v.comments,
			exported: map[string]bool{},
		}
		if typescript.Library(globals[len(globals)-1]) {
			l.declared = map[symbol.Identity]bool{}
		}
		l.lower(tree.Root())
		tree.Close()
		l.recordUses()
	}
	name, root := moduleOf(u)
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		gb.StampModule(pkg, name, root)
	}
	return nil
}

// parseLibrary loads the members of a library unit into the package of its
// library, the qualified path of key. A member is a file of the library or
// the fragment of a global name of a file. It parses the trees of the
// members concurrently, at most one parse for each processor, and lowers
// them one after another in the order of the unit, so an interface of more
// than one member merges in that order. It does not report a syntax error,
// because the author of the workspace cannot change the library. After
// every member lowers, the unit records the uses of each member, as
// [lowering.recordUses] states them.
//
// Error modes: parseLibrary returns the error of a member that does not
// read, and the error of the context.
func (f *tsFrontend) parseLibrary(ctx context.Context, u *plugin.SourceUnit, key string) error {
	files := u.Files()
	sources := make([][]byte, len(files))
	for i, ref := range files {
		src, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		sources[i] = src
	}
	trees := make([]*treesitter.Tree, len(files))
	errs := make([]error, len(files))
	slots := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i, ref := range files {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			trees[i], errs[i] = f.ts.grammar.Parse(ctx, ref.Path, sources[i])
		})
	}
	wg.Wait()
	defer func() {
		for _, tree := range trees {
			if tree != nil {
				tree.Close()
			}
		}
	}()
	if err := errors.Join(errs...); err != nil {
		return err
	}
	pkg := plugin.StorePath(typescript.LibraryStore, key)
	interfaces := map[mergeKey]*node.Interface{}
	declared := map[symbol.Identity]bool{}
	lowerings := make([]*lowering, 0, len(files))
	for i, ref := range files {
		l := &lowering{
			u: u, v: f.ts, tree: trees[i], path: ref.Path,
			module:     newModule(ref.Path, resolution{}, []string{pkg}),
			files:      map[string]fileScope{},
			comments:   f.ts.comments,
			exported:   map[string]bool{},
			global:     pkg,
			library:    true,
			interfaces: interfaces,
			declared:   declared,
		}
		l.lower(trees[i].Root())
		trees[i].Close()
		trees[i] = nil
		lowerings = append(lowerings, l)
	}
	for _, l := range lowerings {
		l.recordUses()
	}
	return nil
}

// globalsOf returns the packages of the global scope of a source file: the
// global package of the workspace, and then the package of the library of
// the file where the load provides the library. The library of the file is
// the key of the shared input of the library that the partition declared.
// A lib directive at the head of the file adds the files of its library to
// the library of the file, and the root files of the extended library form
// its key, sorted. A directive with a name that TypeScript 7.0.2 does not
// know reports under [UnknownLibrary]. A file without a library reads no
// directive, because the options of its project select none.
func globalsOf(u *plugin.SourceUnit, ref plugin.SourceRef, src []byte) []string {
	key := ""
	for _, s := range ref.Shared {
		if k, _, isLibrary := libraryPath(s); isLibrary {
			key = k
		}
	}
	if key == "" {
		return workspaceGlobals
	}
	roots := strings.Split(key, typescript.LibraryKeySep)
	for _, d := range libReferences(ref.Path, src) {
		file, known := fileOfLibrary(d.name)
		switch {
		case !known:
			u.Warnf(UnknownLibrary, d.pos,
				"the directive refers to %q, which is no library of TypeScript %s, so it adds nothing", d.name,
				library.Release)
		case !slices.Contains(roots, file):
			roots = append(roots, file)
		}
	}
	slices.Sort(roots)
	return []string{"", plugin.StorePath(typescript.LibraryStore, strings.Join(roots, typescript.LibraryKeySep))}
}

// resolutionOf reads the tsconfig chain of a member through the door of
// the unit and returns its module resolution. It reports a chain that does
// not read whole at its governing configuration, and returns the empty
// resolution for it. It reports each entry of lib and a target that
// TypeScript 7.0.2 does not know under [UnknownLibrary]. A member without a
// tsconfig.json among its shared inputs resolves through neither a baseUrl
// nor paths.
func resolutionOf(u *plugin.SourceUnit, ref plugin.SourceRef) resolution {
	if len(ref.Shared) == 0 || path.Base(ref.Shared[0]) != tsconfigName {
		return resolution{}
	}
	config := ref.Shared[0]
	at := position.Pos{File: config, Line: 1, Col: 1}
	_, s, err := readChain(u.Read, config)
	_, unknownLib, unknownTarget := rootsOf(s.library)
	for _, entry := range unknownLib {
		u.Warnf(UnknownLibrary, at, "lib lists %q, which is no library of TypeScript %s, so it selects no file",
			entry, library.Release)
	}
	if unknownTarget != "" {
		u.Warnf(UnknownLibrary, at,
			"target %q is no target of TypeScript %s, so the default library of %s applies",
			unknownTarget, library.Release, defaultTarget)
	}
	if err != nil {
		u.Warnf(BadConfig, at, "%v. The files it governs resolve without its baseUrl and paths", err)
		return resolution{}
	}
	return s.resolution
}

// moduleOf returns the module of the member of a unit: the name of the
// package.json among its shared inputs, and the directory of that file. A
// member without a package.json is in no module, and so is a member whose
// package.json has no name. moduleOf reports a package.json that does not
// read or parse under [BadPackage].
func moduleOf(u *plugin.SourceUnit) (name, root string) {
	shared := u.Files()[0].Shared
	if len(shared) == 0 || path.Base(shared[len(shared)-1]) != packageJSON {
		return "", ""
	}
	file := shared[len(shared)-1]
	data, err := u.Read(file)
	var m manifest
	if err == nil {
		err = json.Unmarshal(data, &m)
	}
	if err != nil {
		u.Warnf(BadPackage, position.Pos{File: file, Line: 1, Col: 1},
			"%v. The files it governs load without a module", err)
		return "", ""
	}
	return m.Name, path.Dir(file)
}
