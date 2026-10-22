// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/lang/typescript

go 1.27.3

require (
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/eidos/lang v0.3.0
	go.dokimi.dev/eidos/sdk v0.5.0
)

require (
	github.com/dokimasia/tree-sitter-typescript v0.0.0-20261020225451-839d520b3f2a // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	go.dokimi.dev/eidos/core v0.7.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
