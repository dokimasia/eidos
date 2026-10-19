// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"slices"
	"sync"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

// prelude returns the parse of the prelude of gqlparser: the built-in
// scalars, the built-in directives and the types of introspection, which
// every schema loads with. Every load shares the one parse. The validator
// writes to a definition that an extension extends and to a root type, and
// the prelude applies no directive, so [Load] gives each load a copy of each
// definition of the prelude and shares the rest.
var prelude = sync.OnceValues(func() (*ast.SchemaDocument, error) {
	return parser.ParseSchema(validator.Prelude)
})

// Load loads a schema through the validator of gqlparser: the prelude of
// gqlparser and the documents docs of the type system. The validator merges
// each extension into its type and adds the fields of introspection to the
// root of queries, so it writes into the definitions of docs, and a document
// loads once. Each definition of the prelude is a copy of the load, whose
// lists append into arrays of their own, so two loads can extend one
// definition of the prelude at once. Load returns an error that wraps the
// error of the validator for a schema that does not load.
//
// Load allocates a copy of each definition of the prelude, the merged
// document and what the validator allocates.
func Load(docs []*ast.SchemaDocument) (*ast.Schema, error) {
	base, err := prelude()
	var schema *ast.Schema
	if err == nil {
		size := len(base.Definitions)
		for _, d := range docs {
			size += len(d.Definitions)
		}
		merged := &ast.SchemaDocument{
			Directives: slices.Clip(base.Directives), Definitions: make(ast.DefinitionList, 0, size),
		}
		for _, d := range base.Definitions {
			c := *d
			c.Directives, c.Interfaces = slices.Clip(c.Directives), slices.Clip(c.Interfaces)
			c.Fields, c.EnumValues = slices.Clip(c.Fields), slices.Clip(c.EnumValues)
			c.Types, c.TypePositions = slices.Clip(c.Types), slices.Clip(c.TypePositions)
			merged.Definitions = append(merged.Definitions, &c)
		}
		for _, d := range docs {
			merged.Merge(d)
		}
		schema, err = validator.ValidateSchemaDocument(merged)
	}
	if err != nil {
		return nil, fmt.Errorf("document: %w", err)
	}
	return schema, nil
}
