// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/lang/go

go 1.27.2

require (
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/eidos/lang v0.2.1
	go.dokimi.dev/eidos/sdk v0.3.0
	golang.org/x/mod v0.42.0
)

require (
	go.dokimi.dev/eidos/core v0.4.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
