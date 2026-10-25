// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/plugin/shape

go 1.27.3

require (
	go.dokimi.dev/assert v0.7.0
	go.dokimi.dev/eidos/sdk v0.6.0
)

require (
	go.dokimi.dev/eidos/core v0.8.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
