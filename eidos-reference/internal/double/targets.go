// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package double

import "go.dokimi.dev/eidos/sdk/plugin"

// GoTarget and TypeScriptTarget are the targets that a double plugin
// declares a template tree for. They have the spellings of the targets of
// the Go and the TypeScript satellites. A plugin of both languages imports
// neither satellite.
const (
	GoTarget         plugin.Target = "golang"
	TypeScriptTarget plugin.Target = "typescript"
)
