// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

module go.dokimi.dev/eidos/protoc

go 1.27.3

require (
	github.com/bufbuild/protocompile v0.14.2-0.20260917202354-386f9fcfc7b9
	go.dokimi.dev/assert v0.10.0
	go.dokimi.dev/eidos/core v0.12.0
	go.dokimi.dev/eidos/lang/go v0.6.3
	go.dokimi.dev/eidos/lang/protobuf v0.3.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/dave/dst v0.28.0 // indirect
	github.com/golangci/golines v0.16.0 // indirect
	github.com/ldez/structtags v0.6.1 // indirect
	github.com/petermattis/goid v0.0.0-20260918085751-abfca077860b // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/tidwall/btree v1.8.2 // indirect
	go.dokimi.dev/eidos/lang v0.4.2 // indirect
	go.dokimi.dev/eidos/sdk v0.8.0 // indirect
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	mvdan.cc/gofumpt v0.12.0 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/lang/protobuf => ../eidos-lang-protobuf

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
