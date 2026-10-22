// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/declarative

go 1.27.3

require (
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/eidos/core v0.6.0
	go.yaml.in/yaml/v3 v3.0.5
)

require golang.org/x/sys v0.49.0 // indirect

replace go.dokimi.dev/eidos/core => ../eidos-core
