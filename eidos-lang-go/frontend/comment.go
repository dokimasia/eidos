// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go/ast"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// goBuild is the constraint directive. It is configuration and not an
// annotation, so only the exclusion scan reads it.
const goBuild = "go:build"

// split takes one comment group apart through the pipeline of the unit,
// and drops the go:build directive from the annotations. The pipeline
// reads consecutive line comments as one text, so the continuation of a
// carrier folds across them as the author reads it.
func (l *lowered) split(u *plugin.SourceUnit, group *ast.CommentGroup) plugin.CommentParts {
	var parts plugin.CommentParts
	if group == nil {
		return parts
	}
	l.consumed[group] = true
	take := func(one plugin.CommentParts) {
		parts.Carriers = append(parts.Carriers, one.Carriers...)
		for _, a := range one.Annotations {
			if a.Name == goBuild {
				continue
			}
			parts.Annotations = append(parts.Annotations, a)
		}
		parts.Docs = append(parts.Docs, one.Docs...)
	}
	var run []string
	var runAt position.Pos
	flush := func() {
		if len(run) > 0 {
			take(u.Comment(strings.Join(run, "\n"), runAt))
			run = run[:0]
		}
	}
	for _, c := range group.List {
		if strings.HasPrefix(c.Text, "//") {
			if len(run) == 0 {
				runAt = l.at(c.Pos())
			}
			run = append(run, c.Text)
			continue
		}
		flush()
		take(u.Comment(c.Text, l.at(c.Pos())))
	}
	flush()
	return parts
}

// skip marks comment groups as taken without reading them. They are the
// comments of a declaration that the load leaves out, and their carriers
// and tool directives leave with the declaration. skip passes over a nil
// group.
func (l *lowered) skip(groups ...*ast.CommentGroup) {
	for _, group := range groups {
		if group != nil {
			l.consumed[group] = true
		}
	}
}

// merge joins the parts of a declaration group with the parts of one of
// its specs. The carriers and the annotations of both apply to the spec.
// The documentation of the spec is its own text, and the text of the
// group where the spec has none.
func merge(own, group plugin.CommentParts) plugin.CommentParts {
	if len(own.Docs) == 0 {
		own.Docs = group.Docs
	}
	own.Carriers = append(own.Carriers, group.Carriers...)
	own.Annotations = append(own.Annotations, group.Annotations...)
	return own
}
