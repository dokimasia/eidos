// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go/ast"
	"strings"

	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// legacyBuild opens the legacy constraint form, which this
// frontend does not read: without the filter it would parse as a
// carrier named "build" and attach a directive nobody wrote.
const legacyBuild = "build"

// goBuild is the constraint directive, configuration rather than
// an annotation: the exclusion scan reads it, nothing else does.
const goBuild = "go:build"

// split takes one comment group apart through the unit's own
// pipeline, then filters what is Go's alone: the legacy +build
// form out of the carriers, the go:build directive out of the
// annotations.
func (l *lowered) split(u *plugin.SourceUnit, group *ast.CommentGroup) plugin.CommentParts {
	var parts plugin.CommentParts
	if group == nil {
		return parts
	}
	l.consumed[group] = true
	for _, c := range group.List {
		one := u.Comment(c.Text, l.at(c.Pos()))
		for _, carried := range one.Carriers {
			if carried.Payload == legacyBuild ||
				strings.HasPrefix(carried.Payload, legacyBuild+" ") {
				continue
			}
			parts.Carriers = append(parts.Carriers, carried)
		}
		for _, a := range one.Annotations {
			if a.Name == goBuild {
				continue
			}
			parts.Annotations = append(parts.Annotations, a)
		}
		parts.Docs = append(parts.Docs, one.Docs...)
	}
	return parts
}

// merge folds a group's parts under a spec's own: carriers and
// annotations union — a group's directives apply beside a spec's —
// while the documentation text keeps the nearer comment's.
func merge(own, group plugin.CommentParts) plugin.CommentParts {
	if len(own.Docs) == 0 {
		own.Docs = group.Docs
	}
	own.Carriers = append(own.Carriers, group.Carriers...)
	own.Annotations = append(own.Annotations, group.Annotations...)
	return own
}

// attachCarriers parses each carrier under the kernel grammar and
// records it on its subject, a grammar refusal reported at the
// carrier's own line.
func attachCarriers(u *plugin.SourceUnit, gb *plugin.GraphBuilder, subject symbol.Symbol, cs []plugin.Carrier) {
	for _, c := range cs {
		raw, err := directive.Parse(c.Payload)
		if err != nil {
			u.Errorf(BadCarrier, c.Pos, "%q: %v", plugin.CarrierMark+c.Payload, err)
			continue
		}
		raw.Pos = c.Pos
		gb.Attach(subject, raw)
	}
}

// refuseCarriers reports every carrier on a subject the model
// cannot address, so an authored directive never vanishes into
// silence.
func refuseCarriers(u *plugin.SourceUnit, cs []plugin.Carrier, what string) {
	for _, c := range cs {
		u.Errorf(UnaddressedCarrier, c.Pos,
			"%q sits on %s, which the model cannot address; move it to the declaration",
			plugin.CarrierMark+c.Payload, what)
	}
}
