// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"runtime"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/lang/graphql"
	graphqlbackend "go.dokimi.dev/eidos/lang/graphql/backend"
	graphqlfrontend "go.dokimi.dev/eidos/lang/graphql/frontend"
	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
	"go.dokimi.dev/eidos/lang/java"
	javabackend "go.dokimi.dev/eidos/lang/java/backend"
	javafrontend "go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/lang/protobuf"
	protobackend "go.dokimi.dev/eidos/lang/protobuf/backend"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/lang/rust"
	rustbackend "go.dokimi.dev/eidos/lang/rust/backend"
	rustfrontend "go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/lang/typescript"
	tsbackend "go.dokimi.dev/eidos/lang/typescript/backend"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
)

// brand is the brand of the binary. A carrier is +eidos:<directive>, the
// config file is .eidos.yaml, and the state directory is .eidos/.
const brand output.Brand = "eidos"

// help is the help text of the app.
const help = "Generates code with the declarative plugins of a workspace."

// treeSources is the pattern of every package below the workspace root. A
// package that a store provides is in no directory of the tree, so the
// pattern does not admit it.
const treeSources = "./..."

// main runs the app of the binary and exits with the status of the run.
func main() {
	cli.Main(cli.New(help, compose), compose)
}

// compose returns the composition of the binary: the frontends of Go,
// TypeScript, Java, Rust, protobuf and GraphQL, the rules of Go, TypeScript,
// protobuf and GraphQL, and one extensible plan without generators for each
// target.
// Each plan has the name of its target and reads the packages of the
// language of the target below the workspace root. A phase call runs on
// as many workers as the process has processors. Each call returns new
// plugin instances, because a plugin instance belongs to one workspace.
func compose() *workspace.Builder {
	return workspace.New().
		Brand(brand).
		Frontends(gofrontend.New(nil), tsfrontend.New(), javafrontend.New(nil), rustfrontend.New(nil),
			protofrontend.New(), graphqlfrontend.New()).
		Rules(gorules.New(), tsrules.New(), protorules.New(), graphqlrules.New()).
		Targets(golang.Target, typescript.Target, java.Target, rust.Target, protobuf.Target, graphql.Target).
		Plans(
			workspace.Plan{
				Name:       string(golang.Target),
				Sources:    workspace.Sources{Lang: golang.Lang, Packages: []string{treeSources}},
				Backend:    gobackend.New(),
				Extensible: true,
			},
			workspace.Plan{
				Name:       string(typescript.Target),
				Sources:    workspace.Sources{Lang: typescript.Lang, Packages: []string{treeSources}},
				Backend:    tsbackend.New(),
				Extensible: true,
			},
			workspace.Plan{
				Name:       string(java.Target),
				Sources:    workspace.Sources{Lang: java.Lang, Packages: []string{treeSources}},
				Backend:    javabackend.New(),
				Extensible: true,
			},
			workspace.Plan{
				Name:       string(rust.Target),
				Sources:    workspace.Sources{Lang: rust.Lang, Packages: []string{treeSources}},
				Backend:    rustbackend.New(),
				Extensible: true,
			},
			workspace.Plan{
				Name:       string(protobuf.Target),
				Sources:    workspace.Sources{Lang: protobuf.Lang, Packages: []string{treeSources}},
				Backend:    protobackend.New(),
				Extensible: true,
			},
			workspace.Plan{
				Name:       string(graphql.Target),
				Sources:    workspace.Sources{Lang: graphql.Lang, Packages: []string{treeSources}},
				Backend:    graphqlbackend.New(),
				Extensible: true,
			},
		).
		Parallel(runtime.GOMAXPROCS(0))
}
