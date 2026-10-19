// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// errStatement is the refusal of a statement of the neutral vocabulary.
var errStatement = errors.New(refusalPrefix + "a document declares no statement, so GraphQL writes none")

// Scaffold is the statement printer of GraphQL. A GraphQL document declares
// no statement: the body of an executable definition is its selection set,
// which a generator states as a verbatim body. Scaffold refuses every
// statement, and the render skips a declaration whose template places one.
// The templates of the backend place no body.
func Scaffold(emit.Stmt, *render.ImportSet) ([]byte, error) {
	return nil, errStatement
}
