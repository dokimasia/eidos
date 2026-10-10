// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java

import (
	tsjava "github.com/tree-sitter/tree-sitter-java/bindings/go"

	"go.dokimi.dev/eidos/lang/treesitter"
)

// module is the path of the grammar binding's module, whose build
// version the grammar's version names.
const module = "github.com/tree-sitter/tree-sitter-java"

// Grammar parses .java files.
var Grammar = treesitter.Load("java", module, tsjava.Language())
