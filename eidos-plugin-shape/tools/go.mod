// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/plugin/shape/tools

go 1.27.2

require (
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/eidos/core v0.5.0
	go.dokimi.dev/eidos/lang v0.2.2
	go.dokimi.dev/eidos/lang/go v0.2.2
	go.dokimi.dev/eidos/sdk v0.3.1
	go.yaml.in/yaml/v3 v3.0.5
)

require (
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../../eidos-core

replace go.dokimi.dev/eidos/lang => ../../eidos-lang

replace go.dokimi.dev/eidos/lang/go => ../../eidos-lang-go

replace go.dokimi.dev/eidos/sdk => ../../eidos-sdk
