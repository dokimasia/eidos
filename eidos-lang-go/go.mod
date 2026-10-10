// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/lang/go

go 1.27.2

require (
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/sdk v0.0.0-00010101000000-000000000000
	golang.org/x/mod v0.41.0
)

require go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000 // indirect

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
