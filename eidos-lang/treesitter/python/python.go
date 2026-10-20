// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	tspython "github.com/dokimasia/tree-sitter-python/bindings/go"

	"go.dokimi.dev/eidos/lang/treesitter"
)

// module is the path of the grammar binding's module, whose build
// version the grammar's version names.
const module = "github.com/dokimasia/tree-sitter-python"

// Grammar parses .py and .pyi files.
var Grammar = treesitter.Load("python", module, tspython.Language())
