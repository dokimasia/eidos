// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"sync"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// packageScopes are the scopes of the packages of the routed files of one
// render, for a language with a [Language.PackageScope]. The scope of a
// package reports the names that the files of the package declare:
//
//   - each name that the language yields for a file-level declaration that
//     the context's Declarations yields for the package, except a
//     declaration of a source file at the path of a routed file, which the
//     run writes again;
//   - the declared name of each declaration of a routed file of a package
//     of two or more routed files, except a record of a family that claims
//     its files, whose template declares names that the render does not
//     know;
//   - the settled name of each file-level declaration that the context's
//     Others has in the namespace of the package.
//
// The one routed file of a package reserves its own names, so its names
// take no place in the scope.
//
// # Concurrency
//
// The workers compute the scope of a package once, on the first file of the
// package that a worker renders, so the scopes of a render compute in
// parallel. [newPackageScopes] reads the kept names before the workers
// start, because the names of a warm run are not safe for concurrent use.
// The context's Declarations is safe for concurrent use.
type packageScopes struct {
	ctx      *plugin.RenderContext
	declares func(d symbol.Symbol, yield func(name string))
	// routed returns the set of the paths of the routed files, which the
	// first scope that reads a source file computes.
	routed   func() map[string]struct{}
	packages map[symbol.Identity]*packageScope
}

// packageScope is the scope of one package: the positions of its routed
// files among the files of the context, the kept names of its namespace,
// and the function that reports a name of the scope, which once sets on
// the first read and which is nil for a scope without a name.
type packageScope struct {
	files []int
	kept  []plugin.NameEntry
	once  sync.Once
	names func(name string) bool
}

// newPackageScopes returns the scopes of the packages of the routed files
// of ctx for a language that declares the names of a source declaration
// through declares, and nil for a language whose declares is nil. A file
// without a package belongs to no scope. It lists the routed files of each
// package and reads the kept names of each namespace, and computes no
// scope.
//
// # Allocation contract
//
// newPackageScopes allocates the scopes, the map of the packages, a scope
// and the list of its files for each package, and the map that counts the
// files of each package.
func newPackageScopes(
	ctx *plugin.RenderContext, declares func(d symbol.Symbol, yield func(name string)),
) *packageScopes {
	if declares == nil {
		return nil
	}
	counts := map[symbol.Identity]int{}
	for i := range ctx.Files {
		if pkg := ctx.Files[i].Pkg; !pkg.IsZero() {
			counts[pkg]++
		}
	}
	packages := make(map[symbol.Identity]*packageScope, len(counts))
	for i := range ctx.Files {
		g := &ctx.Files[i]
		if g.Pkg.IsZero() {
			continue
		}
		ps := packages[g.Pkg]
		if ps == nil {
			ps = &packageScope{files: make([]int, 0, counts[g.Pkg])}
			if ctx.Others != nil {
				ps.kept = ctx.Others.InFileScope(g.Scope, "")
			}
			packages[g.Pkg] = ps
		}
		ps.files = append(ps.files, i)
	}
	return &packageScopes{
		ctx: ctx, declares: declares, packages: packages,
		routed: sync.OnceValue(func() map[string]struct{} {
			paths := make(map[string]struct{}, len(ctx.Files))
			for i := range ctx.Files {
				paths[ctx.Files[i].Path] = struct{}{}
			}
			return paths
		}),
	}
}

// of returns the function that reports a name of the scope of the package
// pkg. It returns nil on nil scopes and for a package that no routed file
// declares or whose scope has no name. The first call for a package
// computes its scope, and every worker reads the same function after it.
func (s *packageScopes) of(pkg symbol.Identity) func(name string) bool {
	if s == nil {
		return nil
	}
	ps := s.packages[pkg]
	if ps == nil {
		return nil
	}
	ps.once.Do(func() { ps.names = s.compute(pkg, ps) })
	return ps.names
}

// compute returns the function that reports a name of the scope of the
// package pkg, and nil for a scope without a name. The set of the names is
// sized for the declarations of the routed files of a package of two or
// more routed files.
func (s *packageScopes) compute(pkg symbol.Identity, ps *packageScope) func(name string) bool {
	decls := 0
	if len(ps.files) > 1 {
		for _, i := range ps.files {
			for _, u := range s.ctx.Files[i].Units {
				if u.Template == "" {
					decls += len(u.Decls)
				}
			}
		}
	}
	set := make(map[string]struct{}, decls)
	add := func(name string) {
		if name != "" {
			set[name] = struct{}{}
		}
	}
	s.sources(pkg, add)
	for _, e := range ps.kept {
		add(e.Settled)
	}
	if decls > 0 {
		for _, i := range ps.files {
			for _, u := range s.ctx.Files[i].Units {
				if u.Template != "" {
					continue
				}
				for _, d := range u.Decls {
					add(emit.DeclaredName(d))
				}
			}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return func(name string) bool {
		_, held := set[name]
		return held
	}
}

// sources yields each name that the language declares for a file-level
// declaration of a source file of the loaded package at the path of pkg. It
// skips a source file at a routed path, and it yields nothing for a context
// without Declarations.
func (s *packageScopes) sources(pkg symbol.Identity, yield func(name string)) {
	if s.ctx.Declarations == nil {
		return
	}
	routed := s.routed()
	s.ctx.Declarations(pkg.Package, func(file string, decls []symbol.Symbol) {
		if _, again := routed[file]; again {
			return
		}
		for _, d := range decls {
			s.declares(d, yield)
		}
	})
}
