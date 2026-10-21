// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/lang/python

go 1.27.2

require (
	github.com/pelletier/go-toml/v2 v2.4.3
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/eidos/lang v0.2.3
	go.dokimi.dev/eidos/sdk v0.4.0
)

require (
	github.com/dokimasia/tree-sitter-python v0.0.0-20261020093147-bab31bd04b20 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	go.dokimi.dev/eidos/core v0.6.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
