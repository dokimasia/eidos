// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/lang/typescript"
	tsbackend "go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/output"
)

// brand is the brand of the binary of the fixture, so a source file of the
// tree sets the directive of the plugin as +acme:audit.
const brand output.Brand = "acme"

// The plans of the fixture, and the package patterns that they read.
const (
	goPlan     = "go-audit"
	tsPlan     = "ts-audit"
	svcSources = "./svc"
	opsSources = "./ops"
)

// Compose returns the composition of the binary of the fixture, without the
// sink and the ledger that cli.Open adds for each workspace. The composition
// has the Go frontend and rules, the Go and TypeScript targets, and two
// extensible plans without generators of their own:
//
//   - The plan go-audit reads svc and ops toward the Go backend.
//   - The plan ts-audit reads ops toward the TypeScript backend.
//
// The config file of the tree adds the declarative plugin acme-audit to both
// plans. Without it, each plan has no generator and writes no file. Each
// call returns new frontends, rules and backends.
func Compose() *workspace.Builder {
	return workspace.New().
		Brand(brand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target, typescript.Target).
		Plans(
			workspace.Plan{
				Name:       goPlan,
				Sources:    workspace.Sources{Packages: []string{svcSources, opsSources}},
				Backend:    gobackend.New(),
				Extensible: true,
			},
			workspace.Plan{
				Name:       tsPlan,
				Sources:    workspace.Sources{Packages: []string{opsSources}},
				Backend:    tsbackend.New(),
				Extensible: true,
			},
		)
}
