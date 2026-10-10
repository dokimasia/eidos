// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
)

// ComposeShape returns the composition of the shape catalog over Go
// source. It composes the Go frontend, which registers the Go keys, the
// Go rules, and the annotators of the catalog under [Brand], so an author
// writes a directive of the catalog as //+acme:shape writer. The caller
// adds the plans that consume the catalog, and the output. Each call
// returns new plugins, because a plugin instance belongs to one
// workspace.
func ComposeShape() *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target).
		Annotators(catalog.Annotators()...)
}
