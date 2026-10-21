// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/cli

go 1.27.2

require (
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/eidos/core v0.3.0
	go.dokimi.dev/eidos/declarative v0.1.2
	go.stealthscale.io/cli v0.4.0
	go.stealthscale.io/cli/yaml v0.1.3
	go.yaml.in/yaml/v3 v3.0.5
)

require golang.org/x/sys v0.49.0 // indirect

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/declarative => ../eidos-declarative
