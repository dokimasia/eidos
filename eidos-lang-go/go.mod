// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/lang/go

go 1.27.3

require (
	github.com/golangci/golines v0.16.1
	go.dokimi.dev/assert v0.22.0
	go.dokimi.dev/eidos/lang v0.4.7
	go.dokimi.dev/eidos/sdk v0.11.1
	golang.org/x/mod v0.43.0
	mvdan.cc/gofumpt v0.12.0
)

require (
	github.com/dave/dst v0.28.0 // indirect
	github.com/ldez/structtags v0.6.2 // indirect
	go.dokimi.dev/eidos/core v0.16.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
