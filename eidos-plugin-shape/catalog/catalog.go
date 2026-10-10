// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Annotators returns the two annotators of the catalog. The first is the
// plugin shape, which declares the directives and stamps the facts of
// every classification. The second is the plugin shapecheck, which
// validates the contract instances. Each call returns new plugins, because
// a plugin instance belongs to one workspace. A composition adds the
// catalog with Annotators(catalog.Annotators()...).
func Annotators() []plugin.Annotator {
	specs := shape.Specs()
	return []plugin.Annotator{newClassifier(specs, Detections()), newChecker(specs)}
}
