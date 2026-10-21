// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/reference

go 1.27.2

require (
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/eidos/cli v0.2.3
	go.dokimi.dev/eidos/core v0.5.0
	go.dokimi.dev/eidos/lang v0.2.2
	go.dokimi.dev/eidos/lang/go v0.2.2
	go.dokimi.dev/eidos/lang/typescript v0.3.1
	go.dokimi.dev/eidos/sdk v0.3.1
	golang.org/x/mod v0.42.0
)

require (
	github.com/dokimasia/tree-sitter-typescript v0.0.0-20261020225451-839d520b3f2a // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	go.dokimi.dev/eidos/declarative v0.1.4 // indirect
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
