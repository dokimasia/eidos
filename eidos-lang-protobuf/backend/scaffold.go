// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// errStatement is the refusal of a statement of the neutral vocabulary.
var errStatement = errors.New(refusalPrefix + "a schema declares no body, so protobuf writes no statement")

// Scaffold is the statement printer of protobuf. A protobuf schema declares
// no body, so Scaffold refuses every statement, and the render skips a
// declaration whose template places one. The templates of the backend place
// no body. Scaffold allocates nothing.
func Scaffold(emit.Stmt, *render.ImportSet) ([]byte, error) {
	return nil, errStatement
}
