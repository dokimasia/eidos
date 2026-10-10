// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/plugin/shape/tools/registry"
	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Brand is the brand of the generator. The Go backend writes it into the
// frame of each generated file.
const Brand output.Brand = "shapegen"

// Plan is the name of the composition's one plan.
const Plan = "catalog"

// The directories and the files of the layout of the plan. The root of the
// catalog module is the workspace root of a run, and the directory of
// package catalog is below it. The files have the suffix .gen.go, which
// marks a generated Go file in this repository.
const (
	rootDir      = "."
	catalogDir   = "catalog"
	registryFile = "registry.gen.go"
	wiringFile   = "registry_wiring.gen.go"
)

// Compose returns the composition that generates the catalog's registry
// from its specs. It composes the spec frontend, which registers its keys,
// the absent rules of the language of the specs, and the plan [Plan] of
// the registry generator toward the Go backend. The plan writes
// registry.gen.go into the root of the catalog module and
// registry_wiring.gen.go into its directory catalog. The run loads no Go
// file, so the Go backend takes the path of each package from the import
// base [registry.CatalogPath].
//
// The caller sets the output and runs the composition over the catalog
// module's directory. Each call returns new plugins, because a plugin
// instance belongs to one workspace.
func Compose() *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(specfront.New()).
		Rules(rules.Absent(specfront.Lang)).
		Targets(golang.Target).
		Plans(workspace.Plan{
			Name:       Plan,
			Generators: []plugin.Generator{registry.New()},
			Backend:    gobackend.New(),
			Layout: layout.Config{
				Dir:        rootDir,
				ImportBase: registry.CatalogPath,
				Families: map[layout.Family]layout.Refinement{
					{Plugin: registry.ID}: {File: registryFile},
					{Plugin: registry.ID, Tag: string(registry.WiringTag)}: {Dir: catalogDir, File: wiringFile},
				},
			},
		})
}
