module go.dokimi.dev/eidos/conformance

go 1.27.0

require (
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/eidos/cli v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/go v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/java v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/protobuf v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/rust v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/typescript v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/plugin/shape v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/sdk v0.0.0-00010101000000-000000000000
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/bufbuild/protocompile v0.14.2-0.20260917202354-386f9fcfc7b9 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/petermattis/goid v0.0.0-20260716134002-a9b348f0a2b9 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f // indirect
	github.com/tidwall/btree v1.8.1 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-java v0.23.5 // indirect
	github.com/tree-sitter/tree-sitter-rust v0.24.2 // indirect
	github.com/tree-sitter/tree-sitter-typescript v0.23.2 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/exp v0.0.0-20260709172345-9ea1abe57597 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace go.dokimi.dev/eidos/cli => ../eidos-cli

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/lang/go => ../eidos-lang-go

replace go.dokimi.dev/eidos/lang/java => ../eidos-lang-java

replace go.dokimi.dev/eidos/lang/protobuf => ../eidos-lang-protobuf

replace go.dokimi.dev/eidos/lang/rust => ../eidos-lang-rust

replace go.dokimi.dev/eidos/lang/typescript => ../eidos-lang-typescript

replace go.dokimi.dev/eidos/plugin/shape => ../eidos-plugin-shape

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
