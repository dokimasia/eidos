// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/plugin/shape/tools

go 1.27.3

require (
	go.dokimi.dev/assert v0.7.0
	go.dokimi.dev/eidos/core v0.8.0
	go.dokimi.dev/eidos/lang v0.3.1
	go.dokimi.dev/eidos/lang/go v0.4.0
	go.dokimi.dev/eidos/sdk v0.6.0
	go.yaml.in/yaml/v3 v3.0.5
)

require (
	github.com/dave/dst v0.28.0 // indirect
	github.com/golangci/golines v0.16.0 // indirect
	github.com/ldez/structtags v0.6.1 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	mvdan.cc/gofumpt v0.12.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../../eidos-core

replace go.dokimi.dev/eidos/lang => ../../eidos-lang

replace go.dokimi.dev/eidos/lang/go => ../../eidos-lang-go

replace go.dokimi.dev/eidos/sdk => ../../eidos-sdk
