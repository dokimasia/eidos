// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/lang/graphql

go 1.27.3

require (
	github.com/bmatcuk/doublestar/v4 v4.10.3
	github.com/dokimasia/gqlparser/v2 v2.0.0-20261020103051-349414ad49ed
	github.com/pelletier/go-toml/v2 v2.4.3
	go.dokimi.dev/assert v0.7.0
	go.dokimi.dev/eidos/lang v0.3.2
	go.dokimi.dev/eidos/sdk v0.7.0
	go.yaml.in/yaml/v3 v3.0.5
)

require (
	github.com/agnivade/levenshtein v1.2.1 // indirect
	go.dokimi.dev/eidos/core v0.9.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
