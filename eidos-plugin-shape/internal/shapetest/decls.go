// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapetest

import (
	"path"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package path of the declarations of a fixture, and its one file.
const (
	Path = "acme/store"
	File = "store.go"
)

// The prefixes of the names of the parameters and the returns of a
// callable, which end in the position of each, and the separators of an
// owner and of a package path.
const (
	paramPrefix  = "p"
	returnPrefix = "r"
	ownerSep     = "."
	pathSep      = "/"
)

// Method returns a method of the struct host. It has one parameter for
// each reference in params, named p0, p1 and on, and one return for each
// reference in returns, named r0, r1 and on. Each declaration has the
// identity that the resolution step assigns it, and [Package] gives the
// method its position and its struct.
func Method(host, name string, params []*node.TypeRef, returns ...*node.TypeRef) *node.Method {
	m := &node.Method{
		ID:         symbol.Identity{Lang: Lang, Package: Path, Owner: host, Name: name, Kind: symbol.KindMethod},
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Level:      symbol.LevelInstance,
		Host:       symbol.Identity{Lang: Lang, Package: Path, Name: host, Kind: symbol.KindStruct},
	}
	m.Params, m.Returns = signature(host+ownerSep+name, params, returns)
	return m
}

// Function returns a function of the package. It has the parameters and
// the returns that [Method] states, and [Package] gives it its position.
func Function(name string, params []*node.TypeRef, returns ...*node.TypeRef) *node.Function {
	fn := &node.Function{
		ID:         symbol.Identity{Lang: Lang, Package: Path, Name: name, Kind: symbol.KindFunction},
		Name:       name,
		Visibility: symbol.VisibilityPublic,
	}
	fn.Params, fn.Returns = signature(name, params, returns)
	return fn
}

// signature returns the parameters and the returns of a callable whose
// members have the owner, one for each reference, named by their
// positions.
func signature(owner string, params, returns []*node.TypeRef) ([]*node.Param, []*node.Return) {
	var ps []*node.Param
	for i, ref := range params {
		name := paramPrefix + strconv.Itoa(i)
		ps = append(ps, &node.Param{
			ID:   symbol.Identity{Lang: Lang, Package: Path, Owner: owner, Name: name, Kind: symbol.KindParam},
			Name: name,
			Type: ref,
		})
	}
	var rs []*node.Return
	for i, ref := range returns {
		name := returnPrefix + strconv.Itoa(i)
		rs = append(rs, &node.Return{
			ID:   symbol.Identity{Lang: Lang, Package: Path, Owner: owner, Name: name, Kind: symbol.KindReturn},
			Name: name,
			Type: ref,
		})
	}
	return ps, rs
}

// Package returns the package [Path] with one file, [File], that declares
// the declarations in order. It declares one struct for each host of a
// method, at the first method of the host, with the methods of the host
// in order. A method and a function have their line in the file, counted
// from 1, and every other declaration keeps its position.
func Package(decls ...node.Declaration) *node.Package {
	var file node.Symbols
	hosts := map[symbol.Identity]*node.Struct{}
	for i, decl := range decls {
		at := position.Pos{File: File, Line: i + 1}
		switch d := decl.(type) {
		case *node.Method:
			d.Pos = at
			host, declared := hosts[d.Host]
			if !declared {
				host = &node.Struct{ID: d.Host, Pos: at, Name: d.Host.Name, Visibility: symbol.VisibilityPublic}
				hosts[d.Host] = host
				file = append(file, host)
			}
			host.Methods = append(host.Methods, d)
		case *node.Function:
			d.Pos = at
			file = append(file, d)
		default:
			file = append(file, decl)
		}
	}
	return &node.Package{
		ID:   symbol.Identity{Lang: Lang, Package: Path, Kind: symbol.KindPackage},
		Path: strings.Split(Path, pathSep),
		Name: path.Base(Path),
		Files: []*node.File{{
			ID:    symbol.Identity{Lang: Lang, Package: Path, Name: File, Kind: symbol.KindFile},
			Path:  File,
			Decls: file,
		}},
	}
}
