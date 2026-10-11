// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/sdk

go 1.27.2

require (
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000
)

require golang.org/x/sys v0.49.0 // indirect

replace go.dokimi.dev/eidos/core => ../eidos-core
