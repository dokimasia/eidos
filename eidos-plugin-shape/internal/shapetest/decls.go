// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapetest

import (
	"strconv"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package path of the declarations of a fixture, and its one file.
const (
	Path = "acme/store"
	File = "store.go"
)

// The prefixes of the names of the parameters and the returns of a
// callable, which end in the position of each.
const (
	paramPrefix  = "p"
	returnPrefix = "r"
)

// Method returns a method of the struct host, without an identity. It has
// one parameter for each reference in params, named p0, p1 and on, and one
// return for each reference in returns, named r0, r1 and on. Its Receives
// names host until [Package] places the method in the struct of that name.
func Method(host, name string, params []*node.TypeRef, returns ...*node.TypeRef) *node.Method {
	m := &node.Method{
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Level:      symbol.LevelInstance,
		Receives:   nodetest.Named(host),
	}
	m.Params, m.Returns = signature(params, returns)
	return m
}

// Function returns a function of the package, without an identity. It has
// the parameters and the returns that [Method] states.
func Function(name string, params []*node.TypeRef, returns ...*node.TypeRef) *node.Function {
	fn := &node.Function{Name: name, Visibility: symbol.VisibilityPublic}
	fn.Params, fn.Returns = signature(params, returns)
	return fn
}

// signature returns the parameters and the returns of a callable, one for
// each reference, named by their positions.
func signature(params, returns []*node.TypeRef) ([]*node.Param, []*node.Return) {
	ps := make([]*node.Param, 0, len(params))
	for i, ref := range params {
		ps = append(ps, &node.Param{Name: paramPrefix + strconv.Itoa(i), Type: ref})
	}
	rs := make([]*node.Return, 0, len(returns))
	for i, ref := range returns {
		rs = append(rs, &node.Return{Name: returnPrefix + strconv.Itoa(i), Type: ref})
	}
	return ps, rs
}

// Package builds the package [Path] through nodetest, with one file,
// [File], that declares the declarations in order, and returns the package
// that the load builds. It declares one struct for each host of a method,
// at the first method of the host, with the methods of the host in order,
// and clears the Receives of each such method. After Package, every
// declaration has the identity, the host and the position that the load of
// the language [Lang] gives it.
//
// Package replaces the type of each parameter, return and field of the
// declarations with a copy, children and arguments included, so a test can
// pass one reference to several places and several tests can share one,
// while each place of the built fixture has a reference of its own. An
// inline reference keeps its members.
func Package(tb assert.TB, decls ...node.Declaration) *node.Package {
	tb.Helper()

	for _, decl := range decls {
		node.Walk(decl, func(s symbol.Symbol) bool {
			switch x := s.(type) {
			case *node.Param:
				x.Type = copied(x.Type)
			case *node.Return:
				x.Type = copied(x.Type)
			case *node.Field:
				x.Type = copied(x.Type)
			}
			return true
		})
	}
	var file []symbol.Symbol
	hosts := map[string]*node.Struct{}
	for _, decl := range decls {
		m, isMethod := decl.(*node.Method)
		if !isMethod || m.Receives == nil {
			file = append(file, decl)
			continue
		}
		host, declared := hosts[m.Receives.Spelling]
		if !declared {
			host = &node.Struct{Name: m.Receives.Spelling, Visibility: symbol.VisibilityPublic}
			hosts[m.Receives.Spelling] = host
			file = append(file, host)
		}
		m.Receives = nil
		host.Methods = append(host.Methods, m)
	}
	b := nodetest.New(nodetest.Neutral{Language: Lang})
	b.File(Path, File, file...)
	pkgs := b.Build(tb)
	assert.Length(tb, pkgs, 1, "the fixture builds one package")
	return pkgs[0]
}

// copied returns a copy of ref, with a copy of each of its children and
// its arguments, and nil for a nil ref. An inline reference keeps its
// members.
func copied(ref *node.TypeRef) *node.TypeRef {
	if ref == nil {
		return nil
	}
	c := *ref
	if len(ref.Elems) > 0 {
		c.Elems = make([]*node.TypeRef, 0, len(ref.Elems))
		for _, e := range ref.Elems {
			c.Elems = append(c.Elems, copied(e))
		}
	}
	if len(ref.Args) > 0 {
		c.Args = make([]*node.TypeRef, 0, len(ref.Args))
		for _, a := range ref.Args {
			c.Args = append(c.Args, copied(a))
		}
	}
	return &c
}
