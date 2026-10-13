// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package reference

import (
	"go.dokimi.dev/eidos/core/authored"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	goannotate "go.dokimi.dev/eidos/lang/go/annotate"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/lang/typescript"
	tsbackend "go.dokimi.dev/eidos/lang/typescript/backend"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/reference/auditgen"
	"go.dokimi.dev/eidos/reference/auditweaver"
	"go.dokimi.dev/eidos/reference/authgen"
	"go.dokimi.dev/eidos/reference/builder"
	"go.dokimi.dev/eidos/reference/debugweaver"
	"go.dokimi.dev/eidos/reference/defaults"
	"go.dokimi.dev/eidos/reference/enum"
	"go.dokimi.dev/eidos/reference/errorgen"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/metricgen"
	"go.dokimi.dev/eidos/reference/middlewaregen"
	"go.dokimi.dev/eidos/reference/mockgen"
	"go.dokimi.dev/eidos/reference/registrygen"
	"go.dokimi.dev/eidos/reference/repogen"
	"go.dokimi.dev/eidos/reference/sentinel"
	"go.dokimi.dev/eidos/reference/shapewriter"
	"go.dokimi.dev/eidos/reference/stubgen"
	"go.dokimi.dev/eidos/reference/tracegen"
	"go.dokimi.dev/eidos/reference/validategen"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Brand is the brand of the composition. A carrier is //+reference:<name>,
// the config file is .reference.yaml, and the state directory is
// .reference/.
const Brand output.Brand = "reference"

// GoPlan and TypeScriptPlan are the names of the two plans.
const (
	GoPlan         = "go"
	TypeScriptPlan = "typescript"
)

// Workers is the number of workers that each phase call dispatches its
// invocations on.
const Workers = 4

// The scopes of the two plans: every package of the workspace tree, and the
// sources under web. A package that a store provides lies in no directory
// of the tree, so neither scope admits it.
const (
	treeSources = "./..."
	webSources  = "web/..."
)

// Base returns the composition of eidos-reference without its plans: the
// brand reference, the Go and the TypeScript frontends and rules, the
// targets golang and typescript, the annotators of the authored samples
// and witnesses, the annotator of the Go satellite, shapewriter and
// defaults, and parallel dispatch on four workers. Each call returns new
// plugin instances.
func Base() *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(gofrontend.New(nil), tsfrontend.New()).
		Rules(gorules.New(), tsrules.New()).
		Targets(golang.Target, typescript.Target).
		Annotators(authored.Sample(), authored.Witness(), goannotate.New(), shapewriter.New(), defaults.New()).
		Parallel(Workers)
}

// Plans returns the two plans. The plan go reads the Go packages of the
// tree and runs every generator of the ensemble toward the Go backend. The
// plan typescript reads the TypeScript sources under web and runs mockgen
// and stubgen toward the TypeScript backend. The plans do not read a
// package of a store, such as the standard library of Go. Each call returns
// new plugin instances, and one instance of mockgen and of stubgen serves
// both plans.
func Plans() []workspace.Plan {
	mock, stub := mockgen.New(), stubgen.New()
	return []workspace.Plan{
		{
			Name:    GoPlan,
			Sources: workspace.Sources{Lang: golang.Lang, Packages: []string{treeSources}},
			Generators: []plugin.Generator{
				handlergen.New(), middlewaregen.New(), authgen.New(), metricgen.New(), tracegen.New(),
				validategen.New(), errorgen.New(), auditgen.New(),
				repogen.New(), mock, stub, registrygen.New(), debugweaver.New(), auditweaver.New(),
				builder.New(), enum.New(), sentinel.New(),
			},
			Backend: gobackend.New(),
		},
		{
			Name:       TypeScriptPlan,
			Sources:    workspace.Sources{Lang: typescript.Lang, Packages: []string{webSources}},
			Generators: []plugin.Generator{mock, stub},
			Backend:    tsbackend.New(),
		},
	}
}

// Compose returns Base with Plans: the composition of the binary.
func Compose() *workspace.Builder {
	return Base().Plans(Plans()...)
}
