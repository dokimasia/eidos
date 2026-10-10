// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust

import (
	tsrust "github.com/tree-sitter/tree-sitter-rust/bindings/go"

	"go.dokimi.dev/eidos/lang/treesitter"
)

// module is the path of the grammar binding's module, whose build
// version the grammar's version names.
const module = "github.com/tree-sitter/tree-sitter-rust"

// Grammar parses .rs files.
var Grammar = treesitter.Load("rust", module, tsrust.Language())
