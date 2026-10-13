// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/reference

go 1.27.2

require (
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/eidos/cli v0.1.0
	go.dokimi.dev/eidos/core v0.1.0
	go.dokimi.dev/eidos/lang v0.1.0
	go.dokimi.dev/eidos/lang/go v0.1.0
	go.dokimi.dev/eidos/lang/typescript v0.1.0
	go.dokimi.dev/eidos/sdk v0.1.0
	golang.org/x/mod v0.41.0
)

require (
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-typescript v0.23.2 // indirect
	go.dokimi.dev/eidos/declarative v0.1.0 // indirect
	go.stealthscale.io/cli v0.4.0 // indirect
	go.stealthscale.io/cli/yaml v0.1.3 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace go.dokimi.dev/eidos/cli => ../eidos-cli

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/declarative => ../eidos-declarative

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/lang/go => ../eidos-lang-go

replace go.dokimi.dev/eidos/lang/typescript => ../eidos-lang-typescript

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
