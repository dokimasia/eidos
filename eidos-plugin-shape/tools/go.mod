// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/plugin/shape/tools

go 1.27.2

require (
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/go v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/sdk v0.0.0-00010101000000-000000000000
	go.yaml.in/yaml/v3 v3.0.5
)

replace go.dokimi.dev/eidos/core => ../../eidos-core

replace go.dokimi.dev/eidos/lang => ../../eidos-lang

replace go.dokimi.dev/eidos/lang/go => ../../eidos-lang-go

replace go.dokimi.dev/eidos/sdk => ../../eidos-sdk
