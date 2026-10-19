// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	graphqllang "go.dokimi.dev/eidos/lang/graphql"
	graphqlfrontend "go.dokimi.dev/eidos/lang/graphql/frontend"
	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
	"go.dokimi.dev/eidos/lang/typescript"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Brand is the brand under which the catalog fixture stamps its files. A
// carrier of the fixture starts with it, as in #+acme:typescript.
const Brand output.Brand = "acme"

// The schema that the edit of the catalog fixture changes, the field of
// Review before and after the edit, and the mode of the file that the edit
// writes.
const (
	reviewsFile  = "catalog/reviews/reviews.graphql"
	strictRating = "rating: Int!"
	looseRating  = "rating: Int"
	reviewsMode  = 0o644
)

// ComposeCatalog returns the composition of the catalog fixture over root
// without its plans. The composition has the GraphQL frontend and rules, the
// Go, TypeScript and GraphQL targets, a disk sink at root and the ledger of
// the state directory under root. [CatalogPlans] returns the plans that it
// runs.
func ComposeCatalog(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(graphqlfrontend.New()).
		Rules(graphqlrules.New()).
		Targets(golang.Target, typescript.Target, graphqllang.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// CatalogPlans returns the plans of the catalog fixture over one graph, in
// the order go-model, ts-client and graphql-mirror: [ModelPlan],
// [ClientPlan] and [MirrorPlan]. Each call returns new generators and
// backends.
func CatalogPlans() []workspace.Plan {
	return []workspace.Plan{ModelPlan(), ClientPlan(), MirrorPlan()}
}

// EditCatalog is the edit of the catalog fixture. It makes the field rating
// of the type Review in catalog/reviews/reviews.graphql under root nullable.
// The struct of go-model changes, and the export of go-model does not,
// because the export lists the names of the declarations and of their
// members and not their types. It reads and writes the one file through an
// [os.Root] of root.
//
// Error modes: the error of a root or a file that does not open, read or
// write, and an error for a schema that does not declare the field rating
// as a non-null Int.
func EditCatalog(root string) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("graphql: %w", err)
	}
	defer r.Close()
	name := filepath.FromSlash(reviewsFile)
	b, err := r.ReadFile(name)
	if err != nil {
		return fmt.Errorf("graphql: %w", err)
	}
	src := string(b)
	if !strings.Contains(src, strictRating) {
		return fmt.Errorf("graphql: %s declares no field %q to make nullable", reviewsFile, strictRating)
	}
	if err := r.WriteFile(name, []byte(strings.Replace(src, strictRating, looseRating, 1)), reviewsMode); err != nil {
		return fmt.Errorf("graphql: %w", err)
	}
	return nil
}

// directiveDefinition reports whether the interface id is the definition of
// a directive, which the frontend stamps with its locations. Each read
// records on the invocation of m.
func directiveDefinition(m eidos.Matcher, id symbol.Identity) bool {
	_, stated := eidos.FactOf(m, id, meta.Named[[]string](graphqllang.DirectiveKey))
	return stated
}

// rootOperation reports whether the struct id is a root operation type of
// its schema, which the frontend stamps with the kind of its operations.
// Each read records on the invocation of m.
func rootOperation(m eidos.Matcher, id symbol.Identity) bool {
	_, stated := eidos.FactOf(m, id, meta.Named[string](graphqllang.OperationKey))
	return stated
}

// tableScalar reports whether the scalar a, an alias without a target, is a
// custom scalar of the table of the satellite. A reference to such a scalar
// folds to the shape of the table, so a generator into another language
// declares no type for it.
func tableScalar(a *node.Alias) bool {
	_, held := graphqllang.ScalarShape(a.Annotations)
	return held
}
