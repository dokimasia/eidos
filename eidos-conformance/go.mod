// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/conformance

go 1.27.3

require (
	go.dokimi.dev/assert v0.3.1
	go.dokimi.dev/eidos/cli v0.2.5
	go.dokimi.dev/eidos/core v0.7.0
	go.dokimi.dev/eidos/declarative v0.1.6
	go.dokimi.dev/eidos/lang v0.3.0
	go.dokimi.dev/eidos/lang/go v0.3.0
	go.dokimi.dev/eidos/lang/graphql v0.1.4
	go.dokimi.dev/eidos/lang/java v0.2.4
	go.dokimi.dev/eidos/lang/protobuf v0.2.4
	go.dokimi.dev/eidos/lang/python v0.1.4
	go.dokimi.dev/eidos/lang/rust v0.2.4
	go.dokimi.dev/eidos/lang/typescript v0.4.0
	go.dokimi.dev/eidos/plugin/shape v0.0.7
	go.dokimi.dev/eidos/sdk v0.5.0
	go.stealthscale.io/cli v0.6.0
)

require (
	buf.build/gen/go/bufbuild/protodescriptor/protocolbuffers/go v1.36.12-20250109164928-1da0de137947.1 // indirect
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260709200747-435963d16310.1 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/agnivade/levenshtein v1.2.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.3 // indirect
	github.com/bufbuild/protocompile v0.14.2-0.20260917202354-386f9fcfc7b9 // indirect
	github.com/dave/dst v0.28.0 // indirect
	github.com/dokimasia/gqlparser/v2 v2.0.0-20261020103051-349414ad49ed // indirect
	github.com/dokimasia/tree-sitter-python v0.0.0-20261020093147-bab31bd04b20 // indirect
	github.com/dokimasia/tree-sitter-typescript v0.0.0-20261020225451-839d520b3f2a // indirect
	github.com/golangci/golines v0.16.0 // indirect
	github.com/ldez/structtags v0.6.1 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/petermattis/goid v0.0.0-20260716134002-a9b348f0a2b9 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f // indirect
	github.com/tidwall/btree v1.8.1 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-java v0.23.5 // indirect
	github.com/tree-sitter/tree-sitter-rust v0.24.2 // indirect
	go.stealthscale.io/cli/yaml v0.1.6 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/exp v0.0.0-20260709172345-9ea1abe57597 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	mvdan.cc/gofumpt v0.12.0 // indirect
)

replace go.dokimi.dev/eidos/cli => ../eidos-cli

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/declarative => ../eidos-declarative

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/lang/go => ../eidos-lang-go

replace go.dokimi.dev/eidos/lang/graphql => ../eidos-lang-graphql

replace go.dokimi.dev/eidos/lang/java => ../eidos-lang-java

replace go.dokimi.dev/eidos/lang/protobuf => ../eidos-lang-protobuf

replace go.dokimi.dev/eidos/lang/python => ../eidos-lang-python

replace go.dokimi.dev/eidos/lang/rust => ../eidos-lang-rust

replace go.dokimi.dev/eidos/lang/typescript => ../eidos-lang-typescript

replace go.dokimi.dev/eidos/plugin/shape => ../eidos-plugin-shape

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
