// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import (
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language in the [symbol.Identity] of every
// declaration this satellite loads, and the key a composition
// registers the projection rules under.
const Lang symbol.Lang = "protobuf"

// Name is the satellite's one identity: the origin its findings
// report under, the plugin its classification stamps are
// attributed to, and the owner of the protobuf metadata
// namespace.
const Name plugin.ID = "protobuf"

// Extension is the suffix of every proto schema file, and what the
// frontend's file claim matches on.
const Extension = ".proto"

// Version is folded into every unit key and into the composition
// fingerprint.
//
// It versions what the frontend produces, not the module's release:
// bump it when a load would put something different in the graph,
// which invalidates every unit loaded under the old value and keeps
// a warm run equal to a cold one.
const Version = "0.1.0"

// Syntax returns protobuf's comment forms, which the frontend
// strips documentation with and the render side would spell
// comments through.
//
// They are C's: a line comment, a block comment whose continuation
// lines open with a star, and the directive convention, so a
// carrier is written on a line comment the way it is in every other
// language here. A call returns a fresh value, which the caller may
// keep.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line:       []string{"//"},
		Blocks:     []plugin.CommentBlock{{Open: "/*", Close: "*/", Gutter: "*"}},
		Directives: true,
	}
}
