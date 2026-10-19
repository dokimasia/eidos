// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import "go.dokimi.dev/eidos/sdk/render"

// Imports renders the import block of a file, which GraphQL does not have:
// a schema has one namespace, and no helper of the backend records an import.
// Imports returns the empty block and allocates nothing.
func Imports(*render.ImportSet) string {
	return ""
}
