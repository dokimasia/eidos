// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
)

// parentDir is the path element that names a directory's parent.
const parentDir = ".."

// override is the routing an author wrote on one origin for one
// plugin's output: a tag that moves the primary family's declarations
// into a companion family, and a path that redirects them. Each field
// is positioned at the carrier line that wrote it, and the zero
// override routes nothing.
type override struct {
	tag    string
	tagAt  position.Pos
	path   string
	pathAt position.Pos
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
			kernel.take(d, directive.OutPath, directive.OutTag)
		case ownedBy(schemas, d.Name, p):
			own.take(d, directive.ReservedOut, directive.ReservedTag)
		}
	}
	if own.path == "" {
		own.path, own.pathAt = kernel.path, kernel.pathAt
	}
	if own.tag == "" {
		own.tag, own.tagAt = kernel.tag, kernel.tagAt
	}
	return own
}

// take fills the fields an instance writes and an earlier instance
// left empty.
func (o *override) take(d *directive.Directive, pathKey, tagKey directive.ParamKey) {
	if v, written := d.Param(pathKey); written && o.path == "" && v.Str != "" {
		o.path, o.pathAt = v.Str, d.Pos
	}
	if v, written := d.Param(tagKey); written && o.tag == "" && v.Str != "" {
		o.tag, o.tagAt = v.Str, d.Pos
	}
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
