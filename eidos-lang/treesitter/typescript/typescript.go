// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"go.dokimi.dev/eidos/lang/treesitter"
)

// module is the path of the grammar binding's module, whose build
// version both grammars' versions name.
const module = "github.com/tree-sitter/tree-sitter-typescript"

// The two grammars: TypeScript parses .ts, .mts and .cts files, and
// TSX parses .tsx files, where JSX elements are expressions.
var (
	TypeScript = treesitter.Load("typescript", module, tsts.LanguageTypescript())
	TSX        = treesitter.Load("tsx", module, tsts.LanguageTSX())
)
