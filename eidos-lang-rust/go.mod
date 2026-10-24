// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/lang/rust

go 1.27.3

require (
	github.com/BurntSushi/toml v1.6.0
	go.dokimi.dev/assert v0.3.1
	go.dokimi.dev/eidos/lang v0.3.0
	go.dokimi.dev/eidos/sdk v0.5.0
)

require (
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-rust v0.24.2 // indirect
	go.dokimi.dev/eidos/core v0.7.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
