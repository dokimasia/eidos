// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// parentDir is the path element that names a directory's parent.
const parentDir = ".."

// override is the routing an author wrote on one origin for one
// plugin's output: a tag that moves the primary family's declarations
// into a companion family, and a path that redirects them. Each field
// is positioned at the carrier line that wrote it, and tagFrom and
// pathFrom are the inputs that wrote it. The zero override routes
// nothing.
type override struct {
	tag      string
	tagAt    position.Pos
	path     string
	pathAt   position.Pos
	tagFrom  plugin.RouteInput
	pathFrom plugin.RouteInput
	// origin is the declaration that states the routing: the origin of the
	// declaration that the override routes, or the subject that placed the
	// declaration where that origin states none. The router sets it, and
	// it derives the key, the directory and the position of a finding.
	origin symbol.Identity
}

// overrideOf reads the routing on one origin for one plugin's output.
// The reserved out= and tag= keys on the plugin's own directives take
// precedence over the kernel out directive's path and tag, field by
// field, and within one spelling the first instance in source order
// takes precedence. A negated directive routes nothing, and a
// directive of another plugin routes nothing for this one.
func overrideOf(ds []directive.Directive, p plugin.ID, schemas *directive.Registry) override {
	var own, kernel override
	for i := range ds {
		d := &ds[i]
		switch {
		case d.Negated:
		case d.Name == directive.KernelOut:
			kernel.take(d, directive.OutPath, directive.OutTag, plugin.RouteOut)
		case ownedBy(schemas, d.Name, p):
			own.take(d, directive.ReservedOut, directive.ReservedTag, plugin.RouteDirective)
		}
	}
	if own.path == "" {
		own.path, own.pathAt, own.pathFrom = kernel.path, kernel.pathAt, kernel.pathFrom
	}
	if own.tag == "" {
		own.tag, own.tagAt, own.tagFrom = kernel.tag, kernel.tagAt, kernel.tagFrom
	}
	return own
}

// take fills the fields an instance writes and an earlier instance
// left empty, and records from as the input of each field that it fills.
func (o *override) take(d *directive.Directive, pathKey, tagKey directive.ParamKey, from plugin.RouteInput) {
	if v, written := d.Param(pathKey); written && o.path == "" && v.Str != "" {
		o.path, o.pathAt, o.pathFrom = v.Str, d.Pos, from
	}
	if v, written := d.Param(tagKey); written && o.tag == "" && v.Str != "" {
		o.tag, o.tagAt, o.tagFrom = v.Str, d.Pos, from
	}
}

// packagePaths returns the path of the kernel out directive on each package
// that has one, keyed by the identity of the package. The path applies to
// each declaration of the package that does not state a path of its own,
// and the tag of the directive does not apply. The first instance in source
// order with a path takes precedence, and a negated instance routes nothing.
// packagePaths returns nil where no package has a path, and it allocates
// only the map that it returns.
func packagePaths(ix *plugin.Index) map[symbol.Identity]override {
	var out map[symbol.Identity]override
	for s := range ix.ByDirective(directive.KernelOut) {
		pkg, declares := s.(node.Declaration)
		if !declares || s.Kind() != symbol.KindPackage {
			continue
		}
		id := pkg.Identity().PackageIdentity()
		ds := ix.DirectivesOf(id)
		for i := range ds {
			d := &ds[i]
			if d.Negated || d.Name != directive.KernelOut {
				continue
			}
			if v, written := d.Param(directive.OutPath); written && v.Str != "" {
				if out == nil {
					out = map[symbol.Identity]override{}
				}
				out[id] = override{path: v.Str, pathAt: d.Pos, pathFrom: plugin.RoutePackage}
				break
			}
		}
	}
	return out
}

// ownedBy reports whether a plugin registered a directive's schema.
func ownedBy(schemas *directive.Registry, name directive.Name, p plugin.ID) bool {
	if schemas == nil {
		return false
	}
	s, registered := schemas.ResolveName(name)
	return registered && plugin.ID(s.Plugin) == p
}

// redirect resolves a path override against a source directory. A
// path ending in a slash, a dot or two dots names a directory and
// leaves the filename to the target, and otherwise the last element
// is the filename. It reports false for an absolute path, a path with
// a backslash and a path whose elements leave the workspace root.
func redirect(base, override string) (dir, file string, inside bool) {
	if strings.HasPrefix(override, "/") || strings.Contains(override, `\`) {
		return "", "", false
	}
	joined := path.Join(base, override)
	if joined == parentDir || strings.HasPrefix(joined, parentDir+"/") {
		return "", "", false
	}
	last := path.Base(override)
	if strings.HasSuffix(override, "/") || last == "." || last == parentDir {
		return joined, "", true
	}
	return path.Dir(joined), path.Base(joined), true
}
